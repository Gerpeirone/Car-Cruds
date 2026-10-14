package main

import (
	"context"
	"errors"
	"net"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

func localDatabaseURL(service string) (string, error) {
	value := os.Getenv("SMOKE_" + strings.ToUpper(service) + "_DATABASE_URL")
	if value == "" {
		password := os.Getenv(strings.ToUpper(service) + "_DB_PASSWORD")
		if password == "" {
			return "", errors.New("falta configuración local de PostgreSQL para -history")
		}
		value = (&url.URL{Scheme: "postgres", User: url.UserPassword(service, password), Host: "127.0.0.1:55432", Path: "/" + service, RawQuery: "sslmode=disable"}).String()
	}
	u, err := url.Parse(value)
	if err != nil || (u.Scheme != "postgres" && u.Scheme != "postgresql") || u.Host == "" || u.Fragment != "" || u.Path == "" {
		return "", errors.New("URL local de PostgreSQL inválida")
	}
	ip := net.ParseIP(u.Hostname())
	if u.Hostname() != "localhost" && (ip == nil || !ip.IsLoopback()) {
		return "", errors.New("la comprobación de historial admite únicamente PostgreSQL local")
	}
	// A URI host query parameter can override the address parsed above.
	for key := range u.Query() {
		if key != "sslmode" {
			return "", errors.New("parámetro no permitido en URL local de PostgreSQL")
		}
	}
	return value, nil
}

func openReadOnly(ctx context.Context, service string) (*pgx.Conn, error) {
	address, err := localDatabaseURL(service)
	if err != nil {
		return nil, err
	}
	cfg, err := pgx.ParseConfig(address)
	if err != nil {
		return nil, errors.New("configuración de PostgreSQL inválida")
	}
	cfg.ConnectTimeout = 3 * time.Second
	cfg.RuntimeParams["default_transaction_read_only"] = "on"
	conn, err := pgx.ConnectConfig(ctx, cfg)
	if err != nil {
		return nil, errors.New("PostgreSQL local no disponible para historial")
	}
	return conn, nil
}

func closeDatabase(conn *pgx.Conn) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = conn.Close(ctx)
}

func (h *harness) checkHistory(ctx context.Context) error {
	customers, err := openReadOnly(ctx, "customers")
	if err != nil {
		return err
	}
	defer closeDatabase(customers)
	rentals, err := openReadOnly(ctx, "rentals")
	if err != nil {
		return err
	}
	defer closeDatabase(rentals)
	pollCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	started := time.Now()
	for {
		queryCtx, queryCancel := context.WithTimeout(pollCtx, 2*time.Second)
		ready, sampleErr := h.historySample(queryCtx, customers, rentals)
		queryCancel()
		if sampleErr != nil {
			return sampleErr
		}
		if ready {
			h.log("PASS RentalCompleted: outbox confirmado e inbox/historial únicos reservationID=%s recepciónMs=%d consultas=read-only", h.completedID, time.Since(started).Milliseconds())
			return nil
		}
		if err := pause(pollCtx, 250*time.Millisecond); err != nil {
			return errors.New("RentalCompleted no alcanzó outbox publicado e historial único dentro de quince segundos")
		}
	}
}

func (h *harness) historySample(ctx context.Context, customers, rentals *pgx.Conn) (bool, error) {
	customerTx, err := customers.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
	if err != nil {
		return false, errors.New("no se pudo iniciar lectura del historial")
	}
	defer customerTx.Rollback(ctx)
	var eventID, eventType, customerID, reservationID string
	err = customerTx.QueryRow(ctx, `SELECT h.event_id, h.payload->>'type', h.payload->>'customerId', h.payload->>'reservationId'
		FROM customer_rental_history h JOIN customer_inbox i ON i.event_id=h.event_id
		WHERE h.reservation_id=$1 AND h.customer_id=$2`, h.completedID, h.customerID).Scan(&eventID, &eventType, &customerID, &reservationID)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, errors.New("consulta del historial falló")
	}
	if eventID == "" || eventType != "RentalCompleted.v1" || customerID != h.customerID || reservationID != h.completedID {
		return false, errors.New("evento recibido no coincide con la devolución de esta ejecución")
	}
	var historyCount int
	if err := customerTx.QueryRow(ctx, "SELECT count(*) FROM customer_rental_history WHERE reservation_id=$1", h.completedID).Scan(&historyCount); err != nil || historyCount != 1 {
		return false, errors.New("historial no conservó una sola entrada por devolución")
	}
	rentalTx, err := rentals.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
	if err != nil {
		return false, errors.New("no se pudo iniciar lectura del outbox")
	}
	defer rentalTx.Rollback(ctx)
	var outboxCount, publishedCount int
	err = rentalTx.QueryRow(ctx, `SELECT count(*), count(*) FILTER (WHERE published_at IS NOT NULL AND event_id=$2)
		FROM rental_outbox WHERE event_type='RentalCompleted.v1' AND payload->>'reservationId'=$1`, h.completedID, eventID).Scan(&outboxCount, &publishedCount)
	if err != nil {
		return false, errors.New("consulta del outbox falló")
	}
	if outboxCount > 1 {
		return false, errors.New("devolución repetida generó eventos duplicados en outbox")
	}
	return outboxCount == 1 && publishedCount == 1, nil
}
