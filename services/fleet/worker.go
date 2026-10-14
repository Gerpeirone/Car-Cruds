package fleet

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/prometheus/client_golang/prometheus"
	amqp "github.com/rabbitmq/amqp091-go"
	"log/slog"
	"math/rand/v2"
	"net"
	"sync/atomic"
	"time"
)

var publishedEvents = prometheus.NewCounter(prometheus.CounterOpts{Name: "fleet_outbox_published_total", Help: "Events acknowledged by RabbitMQ and removed from Mongo outbox."})
var indexedEvents = prometheus.NewCounterVec(prometheus.CounterOpts{Name: "fleet_index_events_total", Help: "Index event outcomes."}, []string{"result"})
var indexWriteDelay = prometheus.NewHistogram(prometheus.HistogramOpts{Name: "fleet_index_write_delay_seconds", Help: "Source update to successful index write, excluding searcher refresh.", Buckets: []float64{.1, .25, .5, 1, 2, 5, 10, 30}})

const EventExchange = "carcruds.events"
const IndexQueue = "fleet.indexer"
const IndexDLQ = "fleet.indexer.dlq"

type Worker struct {
	repo      *MongoRepository
	search    SearchEngine
	rabbitURL string
	connected atomic.Bool
}

func NewWorker(repo *MongoRepository, search SearchEngine, rabbitURL string) *Worker {
	return &Worker{repo: repo, search: search, rabbitURL: rabbitURL}
}
func (w *Worker) Ready(ctx context.Context) error {
	if !w.connected.Load() {
		return errors.New("indexer broker unavailable")
	}
	if err := w.repo.Ready(ctx); err != nil {
		return errors.New("indexer source unavailable")
	}
	return w.search.Ready(ctx)
}
func (w *Worker) Run(ctx context.Context) {
	delay := time.Second
	for ctx.Err() == nil {
		err := w.session(ctx)
		w.connected.Store(false)
		if ctx.Err() != nil {
			return
		}
		slog.Warn("indexer session unavailable; retrying", "retryDelaySeconds", delay.Seconds(), "dependency", "messaging_or_index")
		_ = err // Driver errors can contain credentials; log only the dependency classification.
		select {
		case <-ctx.Done():
			return
		case <-time.After(delay):
		}
		if delay < 8*time.Second {
			delay *= 2
		}
	}
}
func (w *Worker) session(ctx context.Context) error {
	var socket net.Conn
	connection, err := amqp.DialConfig(w.rabbitURL, amqp.Config{Heartbeat: 10 * time.Second, Dial: func(network, addr string) (net.Conn, error) {
		conn, dialErr := amqp.DefaultDial(3*time.Second)(network, addr)
		if dialErr == nil {
			socket = conn
		}
		return conn, dialErr
	}})
	if err != nil {
		if socket != nil {
			_ = socket.Close()
		}
		return errors.New("broker connection failed")
	}
	// Close the transport first: AMQP channel/connection close handshakes must
	// not wait indefinitely behind a blocked publish or an unresponsive peer.
	defer func() {
		_ = socket.Close()
		_ = connection.Close()
	}()
	stopSessionClose := context.AfterFunc(ctx, func() { _ = socket.Close() })
	defer stopSessionClose()
	if err = socket.SetDeadline(time.Now().Add(3 * time.Second)); err != nil {
		return err
	}
	pub, err := connection.Channel()
	if err != nil {
		return err
	}
	consumer, err := connection.Channel()
	if err != nil {
		return err
	}
	if err = declareTopology(consumer); err != nil {
		return err
	}
	if err = pub.Confirm(false); err != nil {
		return err
	}
	if err = consumer.Qos(8, 0, false); err != nil {
		return err
	}
	deliveries, err := consumer.Consume(IndexQueue, "", false, false, false, false, nil)
	if err != nil {
		return err
	}
	if err = socket.SetDeadline(time.Time{}); err != nil {
		return err
	}
	w.connected.Store(true)
	sessionCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	errorsCh := make(chan error, 1)
	go func() { errorsCh <- w.relay(sessionCtx, pub, socket) }()
	closed := connection.NotifyClose(make(chan *amqp.Error, 1))
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-closed:
			return errors.New("broker connection closed")
		case err := <-errorsCh:
			return err
		case delivery, ok := <-deliveries:
			if !ok {
				return errors.New("consumer closed")
			}
			if err = w.consume(sessionCtx, delivery); err != nil {
				return err
			}
		}
	}
}
func declareTopology(ch *amqp.Channel) error {
	if err := ch.ExchangeDeclare(EventExchange, "topic", true, false, false, false, nil); err != nil {
		return err
	}
	if err := ch.ExchangeDeclare("carcruds.dlx", "topic", true, false, false, false, nil); err != nil {
		return err
	}
	if _, err := ch.QueueDeclare(IndexDLQ, true, false, false, false, nil); err != nil {
		return err
	}
	if err := ch.QueueBind(IndexDLQ, "fleet.indexer", "carcruds.dlx", false, nil); err != nil {
		return err
	}
	if _, err := ch.QueueDeclare(IndexQueue, true, false, false, false, amqp.Table{"x-dead-letter-exchange": "carcruds.dlx", "x-dead-letter-routing-key": "fleet.indexer"}); err != nil {
		return err
	}
	return ch.QueueBind(IndexQueue, "VehicleChanged.v1", EventExchange, false, nil)
}
func (w *Worker) relay(ctx context.Context, ch *amqp.Channel, socket net.Conn) error {
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
		batchCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
		events, err := w.repo.Pending(batchCtx, 50)
		cancel()
		if err != nil {
			continue
		}
		for _, event := range events {
			body, _ := json.Marshal(event)
			err := withPublisherDeadline(ctx, socket, func(publishCtx context.Context) error {
				confirmation, publishErr := ch.PublishWithDeferredConfirmWithContext(publishCtx, EventExchange, event.Type, false, false, amqp.Publishing{ContentType: "application/json", DeliveryMode: amqp.Persistent, MessageId: event.EventID, Type: event.Type, Timestamp: event.OccurredAt, Body: body})
				if publishErr != nil {
					return publishErr
				}
				if confirmation == nil {
					return errors.New("publisher confirmation unavailable")
				}
				accepted, confirmErr := confirmation.WaitContext(publishCtx)
				if !accepted && confirmErr == nil {
					return errors.New("publisher negative confirmation")
				}
				return confirmErr
			})
			if err != nil {
				return errors.New("publisher unavailable")
			}
			saveCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
			err = w.repo.MarkPublished(saveCtx, event)
			cancel()
			if err != nil {
				return err
			}
			publishedEvents.Inc()
		}
	}
}

// The AMQP SDK's publish context cannot interrupt an in-flight socket write.
// One budget therefore covers the real write and the publisher confirmation;
// cancellation closes the session transport, keeping delivery uncertain and
// leaving the Mongo event pending for a safe replay.
func withPublisherDeadline(ctx context.Context, socket net.Conn, operation func(context.Context) error) error {
	callCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	deadline, _ := callCtx.Deadline()
	if err := socket.SetWriteDeadline(deadline); err != nil {
		_ = socket.Close()
		return err
	}
	stopClose := context.AfterFunc(callCtx, func() { _ = socket.Close() })
	err := operation(callCtx)
	stopped := stopClose()
	if callCtx.Err() != nil {
		err = callCtx.Err()
	}
	if err != nil || !stopped {
		_ = socket.Close()
		if err == nil {
			err = context.Canceled
		}
		return err
	}
	if err = socket.SetWriteDeadline(time.Time{}); err != nil {
		_ = socket.Close()
	}
	return err
}
func (w *Worker) consume(ctx context.Context, delivery amqp.Delivery) error {
	var event Event
	if len(delivery.Body) > 1<<20 || json.Unmarshal(delivery.Body, &event) != nil || event.Type != "VehicleChanged.v1" || event.EventID == "" || event.Version < 1 || ValidateID(event.VehicleID) != nil {
		indexedEvents.WithLabelValues("invalid").Inc()
		return delivery.Nack(false, false)
	}
	for attempt := 0; attempt < 3; attempt++ {
		callCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
		// Reload the owner rather than trusting a delayed event payload.
		vehicle, err := w.repo.Get(callCtx, event.VehicleID)
		if err == nil {
			err = w.search.Upsert(callCtx, vehicle)
		}
		cancel()
		if err == nil {
			indexedEvents.WithLabelValues("processed").Inc()
			if delay := time.Since(vehicle.UpdatedAt).Seconds(); delay >= 0 {
				indexWriteDelay.Observe(delay)
			}
			return delivery.Ack(false)
		}
		if errors.Is(err, ErrNotFound) {
			indexedEvents.WithLabelValues("not_found").Inc()
			return delivery.Nack(false, false)
		}
		if attempt < 2 {
			wait := time.Duration(200*(1<<attempt)+rand.IntN(100)) * time.Millisecond
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(wait):
			}
		}
	}
	indexedEvents.WithLabelValues("failed").Inc()
	return delivery.Nack(false, false)
}
