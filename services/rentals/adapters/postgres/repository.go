package postgres

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"github.com/Gerpeirone/Car-Cruds/services/rentals/adapters/wire"
	"github.com/Gerpeirone/Car-Cruds/services/rentals/application"
	"github.com/Gerpeirone/Car-Cruds/services/rentals/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"time"
)

//go:embed migrations/001.sql
var migration string

func Migrate(ctx context.Context, pool *pgxpool.Pool) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, "SELECT pg_advisory_xact_lock(1448201253)"); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, migration); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

type Repository struct {
	Pool  *pgxpool.Pool
	NewID func() string
}

func unavailable() *domain.Problem {
	return domain.Fail("DEPENDENCY_UNAVAILABLE", "El almacenamiento no está disponible.")
}

// LookupResult performs one short read and releases the connection before any
// remote validation. ExecuteOnce remains the authority for concurrent commits.
func (r *Repository) LookupResult(ctx context.Context, app, key, fingerprint string) (application.Result, bool, error) {
	return lookupResult(ctx, r.Pool, app, key, fingerprint)
}

type resultReader interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}

func lookupResult(ctx context.Context, reader resultReader, app, key, fingerprint string) (application.Result, bool, error) {
	var savedHash string
	var status int
	var body []byte
	err := reader.QueryRow(ctx, "SELECT fingerprint,response_status,response_json FROM rental_idempotency WHERE app=$1 AND operation='create-reservation' AND key=$2", app, key).Scan(&savedHash, &status, &body)
	if errors.Is(err, pgx.ErrNoRows) {
		return application.Result{}, false, nil
	}
	if err != nil {
		return application.Result{}, false, unavailable()
	}
	if savedHash != fingerprint {
		return application.Result{Problem: domain.Fail("IDEMPOTENCY_KEY_REUSED", "La clave se utilizó con otra carga.")}, true, nil
	}
	result, err := wire.Decode(status, body, app)
	return result, true, err
}

func (r *Repository) ExecuteOnce(ctx context.Context, app, key, fingerprint string, fn func(context.Context, application.UnitOfWork) (application.Result, error)) (application.Result, error) {
	tx, err := r.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return application.Result{}, unavailable()
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, "SELECT pg_advisory_xact_lock(hashtextextended($1,0))", app+"\nPOST /v1/reservations\n"+key); err != nil {
		return application.Result{}, unavailable()
	}
	if saved, found, err := lookupResult(ctx, tx, app, key, fingerprint); err != nil || found {
		return saved, err
	}
	result, err := fn(ctx, &unit{tx: tx, newID: r.NewID})
	if err != nil {
		return application.Result{}, err
	}
	status, body, err := wire.Encode(result)
	if err != nil {
		return application.Result{}, unavailable()
	}
	if _, err = tx.Exec(ctx, "INSERT INTO rental_idempotency(app,operation,key,fingerprint,response_status,response_json) VALUES($1,'create-reservation',$2,$3,$4,$5)", app, key, fingerprint, status, body); err != nil {
		return application.Result{}, unavailable()
	}
	if err = tx.Commit(ctx); err != nil {
		return application.Result{}, unavailable()
	}
	return result, nil
}

type unit struct {
	tx    pgx.Tx
	newID func() string
}

func (u *unit) Reserve(ctx context.Context, r domain.Reservation) *domain.Problem {
	if _, err := u.tx.Exec(ctx, "SAVEPOINT create_reservation"); err != nil {
		return unavailable()
	}
	_, err := u.tx.Exec(ctx, "INSERT INTO rental_reservations(id,owner_app,vehicle_id,customer_id,starts_on,ends_on,status,daily_rate_minor,total_minor,currency) VALUES($1,$2,$3,$4,$5::date,$6::date,$7,$8,$9,$10)", r.ID, r.OwnerApp, r.VehicleID, r.CustomerID, r.Period.StartsOn, r.Period.EndsOn, r.Status, r.DailyRateMinor, r.TotalMinor, r.Currency)
	if err == nil {
		_, err = u.tx.Exec(ctx, "INSERT INTO rental_agenda(id,reservation_id,vehicle_id,starts_on,ends_on,kind) VALUES($1,$1,$2,$3::date,$4::date,'RESERVATION')", r.ID, r.VehicleID, r.Period.StartsOn, r.Period.EndsOn)
	}
	if err != nil {
		_, rollbackErr := u.tx.Exec(ctx, "ROLLBACK TO SAVEPOINT create_reservation")
		if rollbackErr != nil {
			return unavailable()
		}
		var p *pgconn.PgError
		if errors.As(err, &p) && p.Code == "23P01" {
			return domain.Fail("VEHICLE_UNAVAILABLE", "El auto ya tiene una reserva o bloqueo en el período.")
		}
		return unavailable()
	}
	if _, err = u.tx.Exec(ctx, "RELEASE SAVEPOINT create_reservation"); err != nil {
		return unavailable()
	}
	return nil
}
func appendEvent(ctx context.Context, tx pgx.Tx, eventID, kind string, r domain.Reservation) error {
	payload, err := json.Marshal(map[string]any{"eventId": eventID, "type": kind, "reservationId": r.ID, "vehicleId": r.VehicleID, "customerId": r.CustomerID, "status": r.Status, "startsOn": r.Period.StartsOn, "endsOn": r.Period.EndsOn})
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, "INSERT INTO rental_outbox(event_id,event_type,payload) VALUES($1,$2,$3)", eventID, kind, payload)
	return err
}

func (r *Repository) Available(ctx context.Context, vehicleID string, period domain.Period) (bool, error) {
	var occupied bool
	err := r.Pool.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM rental_agenda WHERE vehicle_id=$1 AND occupied AND period && daterange($2::date,$3::date,'[)'))", vehicleID, period.StartsOn, period.EndsOn).Scan(&occupied)
	if err != nil {
		return false, unavailable()
	}
	return !occupied, nil
}

const columns = "id,owner_app,vehicle_id,customer_id,starts_on::text,ends_on::text,status,daily_rate_minor,total_minor,currency"

func read(row pgx.Row) (domain.Reservation, error) {
	var r domain.Reservation
	err := row.Scan(&r.ID, &r.OwnerApp, &r.VehicleID, &r.CustomerID, &r.Period.StartsOn, &r.Period.EndsOn, &r.Status, &r.DailyRateMinor, &r.TotalMinor, &r.Currency)
	if errors.Is(err, pgx.ErrNoRows) {
		return r, domain.Fail("NOT_FOUND", "Reserva inexistente.")
	}
	if err != nil {
		return r, unavailable()
	}
	// Historical rows must not be revalidated against today's date.
	return r, nil
}
func duration(start, end string) int {
	s, _ := time.Parse("2006-01-02", start)
	e, _ := time.Parse("2006-01-02", end)
	return int(e.Sub(s) / (24 * time.Hour))
}
func (r *Repository) Get(ctx context.Context, id string) (domain.Reservation, error) {
	v, err := read(r.Pool.QueryRow(ctx, "SELECT "+columns+" FROM rental_reservations WHERE id=$1", id))
	if err == nil {
		v.Period.Days = duration(v.Period.StartsOn, v.Period.EndsOn)
	}
	return v, err
}

func (r *Repository) Transition(ctx context.Context, actor domain.Actor, id string, next domain.Status) (domain.Reservation, error) {
	tx, err := r.Pool.Begin(ctx)
	if err != nil {
		return domain.Reservation{}, unavailable()
	}
	defer tx.Rollback(ctx)
	v, err := read(tx.QueryRow(ctx, "SELECT "+columns+" FROM rental_reservations WHERE id=$1 FOR UPDATE", id))
	if err != nil {
		return v, err
	}
	v.Period.Days = duration(v.Period.StartsOn, v.Period.EndsOn)
	if !actor.CanRead(v) {
		return domain.Reservation{}, domain.Fail("NOT_FOUND", "Reserva inexistente.")
	}
	changed, p := v.Transition(next)
	if p != nil {
		return v, p
	}
	if changed.Status == v.Status {
		return v, nil
	}
	if _, err = tx.Exec(ctx, "UPDATE rental_reservations SET status=$2 WHERE id=$1 AND status=$3", id, next, v.Status); err != nil {
		var e *pgconn.PgError
		if errors.As(err, &e) && e.Code == "23505" {
			return v, domain.Fail("VEHICLE_UNAVAILABLE", "El vehículo conserva otro alquiler activo.")
		}
		return v, unavailable()
	}
	if next == domain.Cancelled || next == domain.Completed {
		if _, err = tx.Exec(ctx, "UPDATE rental_agenda SET occupied=false WHERE reservation_id=$1", id); err != nil {
			return v, unavailable()
		}
	}
	if next == domain.Completed {
		if err = appendEvent(ctx, tx, r.NewID(), "RentalCompleted.v1", changed); err != nil {
			return v, unavailable()
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return v, unavailable()
	}
	return changed, nil
}

func (r *Repository) Maintenance(ctx context.Context, id, vehicleID string, p domain.Period) (string, error) {
	_, err := r.Pool.Exec(ctx, "INSERT INTO rental_agenda(id,vehicle_id,starts_on,ends_on,kind) VALUES($1,$2,$3::date,$4::date,'MAINTENANCE')", id, vehicleID, p.StartsOn, p.EndsOn)
	if err != nil {
		var e *pgconn.PgError
		if errors.As(err, &e) && e.Code == "23P01" {
			return "", domain.Fail("VEHICLE_UNAVAILABLE", "Mantenimiento en conflicto con la agenda.")
		}
		return "", unavailable()
	}
	return id, nil
}
