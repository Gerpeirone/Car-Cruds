package data

import (
	"context"
	_ "embed"
	"errors"
	"github.com/Gerpeirone/Car-Cruds/services/customers/business"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed migrations/001.sql
var migration string

func Migrate(ctx context.Context, pool *pgxpool.Pool) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, "SELECT pg_advisory_xact_lock(1448201254)"); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, migration); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
func SeedDemo(ctx context.Context, pool *pgxpool.Pool) error {
	_, err := pool.Exec(ctx, "INSERT INTO customers(id,name,active,license_valid_until) VALUES ('cli-001','Cliente demo',true,'2030-12-31'),('cli-blocked','Cliente bloqueado',false,'2030-12-31') ON CONFLICT(id) DO NOTHING")
	return err
}

type Repository struct{ Pool *pgxpool.Pool }

func unavailable() *business.Problem {
	return &business.Problem{Code: "DEPENDENCY_UNAVAILABLE", Message: "El almacenamiento no está disponible."}
}
func (r *Repository) Get(ctx context.Context, id string) (business.Customer, error) {
	var c business.Customer
	err := r.Pool.QueryRow(ctx, "SELECT id,name,active,license_valid_until::text FROM customers WHERE id=$1", id).Scan(&c.ID, &c.Name, &c.Active, &c.LicenseValidUntil)
	if errors.Is(err, pgx.ErrNoRows) {
		return c, &business.Problem{Code: "NOT_FOUND", Message: "Cliente inexistente."}
	}
	if err != nil {
		return c, unavailable()
	}
	return c, nil
}
func (r *Repository) List(ctx context.Context) ([]business.Customer, error) {
	rows, err := r.Pool.Query(ctx, "SELECT id,name,active,license_valid_until::text FROM customers ORDER BY id LIMIT 200")
	if err != nil {
		return nil, unavailable()
	}
	defer rows.Close()
	result := []business.Customer{}
	for rows.Next() {
		var c business.Customer
		if err = rows.Scan(&c.ID, &c.Name, &c.Active, &c.LicenseValidUntil); err != nil {
			return nil, unavailable()
		}
		result = append(result, c)
	}
	if rows.Err() != nil {
		return nil, unavailable()
	}
	return result, nil
}
func (r *Repository) Create(ctx context.Context, c business.Customer) error {
	_, err := r.Pool.Exec(ctx, "INSERT INTO customers(id,name,active,license_valid_until) VALUES($1,$2,$3,$4::date)", c.ID, c.Name, c.Active, c.LicenseValidUntil)
	if err != nil {
		var p *pgconn.PgError
		if errors.As(err, &p) && p.Code == "23505" {
			return &business.Problem{Code: "CONFLICT", Message: "El identificador de cliente ya existe."}
		}
		return unavailable()
	}
	return nil
}
func (r *Repository) RecordRental(ctx context.Context, eventID, reservationID, customerID string, payload []byte) error {
	tx, err := r.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	result, err := tx.Exec(ctx, "INSERT INTO customer_inbox(event_id) VALUES($1) ON CONFLICT DO NOTHING", eventID)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return tx.Commit(ctx)
	}
	_, err = tx.Exec(ctx, "INSERT INTO customer_rental_history(event_id,reservation_id,customer_id,payload) VALUES($1,$2,$3,$4) ON CONFLICT(reservation_id) DO NOTHING", eventID, reservationID, customerID, payload)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}
