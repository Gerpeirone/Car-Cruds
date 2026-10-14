package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/Gerpeirone/Car-Cruds/internal/devconfig"
	"github.com/Gerpeirone/Car-Cruds/internal/platform"
	amqp "github.com/rabbitmq/amqp091-go"
	"net"
	"os"
	"time"
)

func run() error {
	if err := devconfig.Load(".env"); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	var socket net.Conn
	conn, err := amqp.DialConfig("amqp://carcruds:"+os.Getenv("RABBITMQ_PASSWORD")+"@127.0.0.1:55672/", amqp.Config{Dial: func(network, addr string) (net.Conn, error) {
		c, e := amqp.DefaultDial(3*time.Second)(network, addr)
		socket = c
		return c, e
	}, Heartbeat: 10 * time.Second})
	if err != nil {
		return errors.New("broker local no disponible")
	}
	defer func() { socket.Close(); conn.Close() }()
	stop := context.AfterFunc(ctx, func() { socket.Close() })
	defer stop()
	socket.SetDeadline(time.Now().Add(3 * time.Second))
	ch, err := conn.Channel()
	if err != nil {
		return errors.New("canal no disponible")
	}
	const dead = "customers.rental-history.failed"
	before, err := ch.QueueInspect(dead)
	if err != nil {
		return errors.New("DLQ no declarada")
	}
	if before.Messages != 0 {
		return errors.New("DLQ contiene otros datos: ensayo no ejecutado")
	}
	if err = ch.Confirm(false); err != nil {
		return errors.New("confirmaciones no disponibles")
	}
	confirms := ch.NotifyPublish(make(chan amqp.Confirmation, 1))
	id := "smoke-malformed-" + platform.UUID()
	err = ch.PublishWithContext(ctx, "carcruds.events", "RentalCompleted.v1", true, false, amqp.Publishing{MessageId: id, ContentType: "application/json", DeliveryMode: amqp.Persistent, Body: []byte(`{"smokeMalformed":true}`)})
	if err != nil {
		return errors.New("publicación no completada")
	}
	select {
	case c := <-confirms:
		if !c.Ack {
			return errors.New("publicación no confirmada")
		}
	case <-ctx.Done():
		return errors.New("confirmación excedió el plazo")
	}
	for ctx.Err() == nil {
		socket.SetDeadline(time.Now().Add(3 * time.Second))
		delivery, found, e := ch.Get(dead, false)
		if e != nil {
			return errors.New("lectura de DLQ falló")
		}
		if found {
			if delivery.MessageId != id {
				delivery.Nack(false, true)
				return errors.New("mensaje ajeno conservado; ensayo no acreditado")
			}
			reason := fmt.Sprint(delivery.Headers["x-first-death-reason"])
			queue := fmt.Sprint(delivery.Headers["x-first-death-queue"])
			if reason != "rejected" || queue != "customers.rental-history" {
				delivery.Nack(false, true)
				return errors.New("origen de rechazo inesperado")
			}
			if err = delivery.Ack(false); err != nil {
				return errors.New("limpieza de fixture no confirmada")
			}
			return json.NewEncoder(os.Stdout).Encode(map[string]any{"verifiedAtUtc": time.Now().UTC().Format(time.RFC3339), "result": "PASS", "messageId": id, "routingKey": "RentalCompleted.v1", "sourceQueue": queue, "deadLetterQueue": dead, "reason": reason, "beforeMessages": before.Messages, "malformedFixtureAcknowledgedAfterInspection": true, "unrelatedMessagesTouched": false, "scope": "Publicación real autenticada a broker local y rechazo por consumidor Clientes operativo"})
		}
		socket.SetDeadline(time.Time{})
		select {
		case <-time.After(100 * time.Millisecond):
		case <-ctx.Done():
		}
	}
	return errors.New("mensaje inválido no apareció en DLQ dentro del plazo")
}
func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
