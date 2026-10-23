package postgres

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"

	"github.com/Gerpeirone/Car-Cruds/services/employees/application"
	"github.com/Gerpeirone/Car-Cruds/services/employees/domain"
	"github.com/jackc/pgx/v5"
)

var _ application.GymStore = (*Repository)(nil)

const gymColumns = `enrollment_id,employee_id,app,idempotency_key,fingerprint,dni,email,employee_name,state,provider_code,socio_id,solicitud_id`

func scanGym(row pgx.Row) (domain.GymEnrollment, error) {
	var e domain.GymEnrollment
	err := row.Scan(&e.ID, &e.EmployeeID, &e.App, &e.Key, &e.Fingerprint, &e.DNI, &e.Email, &e.Name, &e.State, &e.ProviderCode, &e.SocioID, &e.SolicitudID)
	if errors.Is(err, pgx.ErrNoRows) {
		err = domain.ErrNotFound
	}
	return e, err
}

func (r *Repository) PrepareGym(ctx context.Context, app, key, employeeID, dni, email, id string) (domain.GymEnrollment, error) {
	canonical, _ := json.Marshal([]string{employeeID, dni, email})
	hash := sha256.Sum256(canonical)
	fingerprint := hex.EncodeToString(hash[:])
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.GymEnrollment{}, err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, "gym-key:"+app+":"+key); err != nil {
		return domain.GymEnrollment{}, err
	}
	saved, err := scanGym(tx.QueryRow(ctx, `SELECT `+gymColumns+` FROM gym_enrollments WHERE app=$1 AND idempotency_key=$2`, app, key))
	if err == nil {
		if saved.Fingerprint != fingerprint {
			return domain.GymEnrollment{}, domain.ErrGymConflict
		}
		return saved, nil
	}
	if !errors.Is(err, domain.ErrNotFound) {
		return domain.GymEnrollment{}, err
	}
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, "gym-employee:"+employeeID); err != nil {
		return domain.GymEnrollment{}, err
	}
	var name string
	var active bool
	err = tx.QueryRow(ctx, `SELECT display_name,active FROM employees WHERE employee_id=$1 FOR SHARE`, employeeID).Scan(&name, &active)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.GymEnrollment{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.GymEnrollment{}, err
	}
	if !active || !domain.GymText(name, 100) || !domain.GymText(employeeID, 100) {
		return domain.GymEnrollment{}, domain.ErrInvalidInput
	}
	var exists bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM gym_enrollments WHERE employee_id=$1 AND state<>'REJECTED')`, employeeID).Scan(&exists); err != nil {
		return domain.GymEnrollment{}, err
	}
	if exists {
		return domain.GymEnrollment{}, domain.ErrGymConflict
	}
	item, err := scanGym(tx.QueryRow(ctx, `INSERT INTO gym_enrollments(enrollment_id,employee_id,app,idempotency_key,fingerprint,dni,email,employee_name,state) VALUES($1,$2,$3,$4,$5,$6,$7,$8,'PENDING') RETURNING `+gymColumns, id, employeeID, app, key, fingerprint, dni, email, name))
	if err != nil {
		return domain.GymEnrollment{}, err
	}
	return item, tx.Commit(ctx)
}

func (r *Repository) GetGym(ctx context.Context, employeeID, id string) (domain.GymEnrollment, error) {
	return scanGym(r.pool.QueryRow(ctx, `SELECT `+gymColumns+` FROM gym_enrollments WHERE employee_id=$1 AND enrollment_id=$2`, employeeID, id))
}
func (r *Repository) ClaimGym(ctx context.Context, id, token string) (bool, error) {
	result, err := r.pool.Exec(ctx, `UPDATE gym_enrollments SET lease_token=$2,lease_until=now()+interval '15 seconds',updated_at=now() WHERE enrollment_id=$1 AND state IN ('PENDING','UNKNOWN') AND (lease_until IS NULL OR lease_until<now())`, id, token)
	return result.RowsAffected() == 1, err
}
func (r *Repository) CompleteGym(ctx context.Context, id, token string, o domain.GymOutcome) (domain.GymEnrollment, error) {
	if o.State != "ACCEPTED" && o.State != "REJECTED" && o.State != "UNKNOWN" {
		return domain.GymEnrollment{}, domain.ErrInvalidInput
	}
	e, err := scanGym(r.pool.QueryRow(ctx, `UPDATE gym_enrollments SET state=$3,provider_code=$4,socio_id=$5,solicitud_id=$6,lease_token='',lease_until=NULL,updated_at=now() WHERE enrollment_id=$1 AND lease_token=$2 RETURNING `+gymColumns, id, token, o.State, o.ProviderCode, o.SocioID, o.SolicitudID))
	if errors.Is(err, domain.ErrNotFound) {
		return scanGym(r.pool.QueryRow(ctx, `SELECT `+gymColumns+` FROM gym_enrollments WHERE enrollment_id=$1`, id))
	}
	return e, err
}
