package data

import (
	"context"
	"encoding/json"
	amqp "github.com/rabbitmq/amqp091-go"
	"log/slog"
	"time"
)

func (r *Repository) ConsumeRentals(ctx context.Context, url string) {
	for ctx.Err() == nil {
		if err := r.consumeSession(ctx, url); err != nil && ctx.Err() == nil {
			slog.Warn("history_consumer_disconnected", "service", "customers", "result", "retrying")
		}
		if !pause(ctx, time.Second) {
			return
		}
	}
}
func pause(ctx context.Context, d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	}
}
func (r *Repository) consumeSession(ctx context.Context, url string) error {
	connection, err := amqp.DialConfig(url, amqp.Config{Dial: amqp.DefaultDial(2 * time.Second), Heartbeat: 10 * time.Second})
	if err != nil {
		return err
	}
	defer connection.Close()
	channel, err := connection.Channel()
	if err != nil {
		return err
	}
	defer channel.Close()
	if err = channel.ExchangeDeclare("carcruds.events", "topic", true, false, false, false, nil); err != nil {
		return err
	}
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
	if err = channel.Qos(16, 0, false); err != nil {
		return err
	}
	deliveries, err := channel.Consume(queue.Name, "", false, false, false, false, nil)
	if err != nil {
		return err
	}
	for {
		select {
		case <-ctx.Done():
			return nil
		case delivery, open := <-deliveries:
			if !open {
				return amqp.ErrClosed
			}
			var e struct {
				EventID       string `json:"eventId"`
				Type          string `json:"type"`
				ReservationID string `json:"reservationId"`
				CustomerID    string `json:"customerId"`
			}
			if json.Unmarshal(delivery.Body, &e) != nil || e.EventID == "" || e.Type != "RentalCompleted.v1" || e.ReservationID == "" || e.CustomerID == "" {
				_ = delivery.Nack(false, false)
				slog.Warn("history_message_failed", "service", "customers", "reason", "invalid_contract")
				continue
			}
			var applyErr error
			for attempt := 0; attempt < 3; attempt++ {
				work, cancel := context.WithTimeout(ctx, time.Second)
				applyErr = r.RecordRental(work, e.EventID, e.ReservationID, e.CustomerID, delivery.Body)
				cancel()
				if applyErr == nil {
					break
				}
				if !pause(ctx, time.Duration(attempt+1)*100*time.Millisecond) {
					return nil
				}
			}
			if applyErr != nil {
				_ = delivery.Nack(false, false)
				slog.Warn("history_message_failed", "service", "customers", "reason", "storage_failure", "event_id", e.EventID)
			} else {
				_ = delivery.Ack(false)
				slog.Info("history_message_applied", "service", "customers", "event_id", e.EventID)
			}
		}
	}
}
