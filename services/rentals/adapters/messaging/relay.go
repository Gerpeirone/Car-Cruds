package messaging

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5/pgxpool"
	amqp "github.com/rabbitmq/amqp091-go"
	"log/slog"
	"net"
	"time"
)

// Run only publishes events with a consumer: RentalCompleted.v1 feeds Customers.
// The database remains authoritative; a failed publication keeps its outbox row.
func Run(ctx context.Context, pool *pgxpool.Pool, url string) {
	for ctx.Err() == nil {
		if err := session(ctx, pool, url); err != nil && ctx.Err() == nil {
			slog.Warn("outbox_disconnected", "service", "rentals", "result", "retrying")
		}
		if !wait(ctx, time.Second) {
			return
		}
	}
}
func wait(ctx context.Context, d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	}
}
func session(ctx context.Context, pool *pgxpool.Pool, url string) error {
	var socket net.Conn
	dial := func(network, address string) (net.Conn, error) {
		connection, err := amqp.DefaultDial(2*time.Second)(network, address)
		if err == nil {
			socket = connection
		}
		return connection, err
	}
	connection, err := amqp.DialConfig(url, amqp.Config{Dial: dial, Heartbeat: 10 * time.Second})
	if err != nil {
		if socket != nil {
			_ = socket.Close()
		}
		return err
	}
	// Closing the transport first also unblocks library writes and close RPCs.
	defer func() { _ = socket.Close(); _ = connection.Close() }()
	stopSessionClose := context.AfterFunc(ctx, func() { _ = socket.Close() })
	defer stopSessionClose()
	channel, err := connection.Channel()
	if err != nil {
		return err
	}
	if err = channel.ExchangeDeclare("carcruds.events", "topic", true, false, false, false, nil); err != nil {
		return err
	}
	// Declare the durable destination even when the consumer is stopped. A
	// publisher confirmation alone does not prove that an event was routed.
	if err = channel.ExchangeDeclare("carcruds.failed", "direct", true, false, false, false, nil); err != nil {
		return err
	}
	dead, err := channel.QueueDeclare("customers.rental-history.failed", true, false, false, false, nil)
	if err != nil {
		return err
	}
	if err = channel.QueueBind(dead.Name, "customers.rental-history", "carcruds.failed", false, nil); err != nil {
		return err
	}
	queue, err := channel.QueueDeclare("customers.rental-history", true, false, false, false, amqp.Table{"x-dead-letter-exchange": "carcruds.failed", "x-dead-letter-routing-key": "customers.rental-history"})
	if err != nil {
		return err
	}
	if err = channel.QueueBind(queue.Name, "RentalCompleted.v1", "carcruds.events", false, nil); err != nil {
		return err
	}
	if err = channel.Confirm(false); err != nil {
		return err
	}
	returns := channel.NotifyReturn(make(chan amqp.Return, 1))
	for ctx.Err() == nil {
		sent, err := publishOne(ctx, pool, channel, returns, socket)
		if err != nil {
			return err
		}
		if !sent && !wait(ctx, 250*time.Millisecond) {
			return nil
		}
	}
	return nil
}
func publishOne(ctx context.Context, pool *pgxpool.Pool, ch *amqp.Channel, returns <-chan amqp.Return, socket net.Conn) (bool, error) {
	work, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	tx, err := pool.Begin(work)
	if err != nil {
		return false, err
	}
	defer tx.Rollback(work)
	var id, kind string
	var body []byte
	rows, err := tx.Query(work, "SELECT event_id,event_type,payload FROM rental_outbox WHERE published_at IS NULL AND event_type='RentalCompleted.v1' ORDER BY created_at FOR UPDATE SKIP LOCKED LIMIT 1")
	if err != nil {
		return false, err
	}
	if !rows.Next() {
		rows.Close()
		return false, rows.Err()
	}
	err = rows.Scan(&id, &kind, &body)
	rows.Close()
	if err != nil {
		return false, err
	}
	err = withWriteDeadline(work, socket, func() error {
		confirmation, err := ch.PublishWithDeferredConfirmWithContext(work, "carcruds.events", kind, true, false, amqp.Publishing{ContentType: "application/json", DeliveryMode: amqp.Persistent, MessageId: id, Type: kind, Timestamp: time.Now(), Body: body})
		if err != nil {
			return err
		}
		confirmed, err := confirmation.WaitContext(work)
		if err != nil {
			return err
		}
		if !confirmed {
			return amqp.ErrClosed
		}
		// A mandatory basic.return precedes its confirmation: do not persist success without a route.
		select {
		case <-returns:
			return amqp.ErrClosed
		default:
			return nil
		}
	})
	if err != nil {
		return false, err
	}
	if _, err = tx.Exec(work, "UPDATE rental_outbox SET published_at=now() WHERE event_id=$1", id); err != nil {
		return false, err
	}
	if err = tx.Commit(work); err != nil {
		return false, err
	}
	slog.Info("outbox_published", "service", "rentals", "event_id", id, "event_type", kind)
	return true, nil
}

// The AMQP library checks ctx before Publish but does not cancel an in-flight
// socket write. Bound that write at the transport, then wait for its confirmation
// under the same deadline. On error the caller closes the connection and retries.
func withWriteDeadline(ctx context.Context, socket net.Conn, fn func() error) (err error) {
	deadline, ok := ctx.Deadline()
	if !ok {
		return errors.New("se requiere deadline de publicación")
	}
	if err = ctx.Err(); err != nil {
		return err
	}
	if err = socket.SetWriteDeadline(deadline); err != nil {
		return err
	}
	stopClose := context.AfterFunc(ctx, func() { _ = socket.Close() })
	defer stopClose()
	defer func() {
		if err == nil {
			err = socket.SetWriteDeadline(time.Time{})
		}
	}()
	err = fn()
	if err == nil {
		err = ctx.Err()
	}
	return err
}
