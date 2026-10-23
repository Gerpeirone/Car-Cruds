package postgres

import (
	"context"
	"embed"
	"errors"
	"fmt"

	"github.com/Gerpeirone/Car-Cruds/services/employees/application"
	"github.com/Gerpeirone/Car-Cruds/services/employees/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed migrations/*.sql
var migrations embed.FS

type Repository struct{ pool *pgxpool.Pool }

var _ application.Repository = (*Repository)(nil)

func New(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool} }

func (r *Repository) Migrate(ctx context.Context) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(724103)`); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `CREATE TABLE IF NOT EXISTS employees_schema_migrations (version text PRIMARY KEY, applied_at timestamptz NOT NULL DEFAULT now())`); err != nil {
		return err
	}
	entries, err := migrations.ReadDir("migrations")
	if err != nil {
		return err
	}
	for _, entry := range entries {
		var applied bool
		if err = tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM employees_schema_migrations WHERE version=$1)`, entry.Name()).Scan(&applied); err != nil {
			return err
		}
		if applied {
			continue
		}
		contents, readErr := migrations.ReadFile("migrations/" + entry.Name())
		if readErr != nil {
			return readErr
		}
		if _, err = tx.Exec(ctx, string(contents)); err != nil {
			return fmt.Errorf("migración %s: %w", entry.Name(), err)
		}
		if _, err = tx.Exec(ctx, `INSERT INTO employees_schema_migrations(version) VALUES ($1)`, entry.Name()); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func (r *Repository) SeedDemo(ctx context.Context) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	_, err = tx.Exec(ctx, `INSERT INTO employees(employee_id,display_name,active) VALUES
		('employee-001','Persona ficticia de demostración Uno',true),
		('employee-002','Persona ficticia de demostración Dos',true)
		ON CONFLICT (employee_id) DO NOTHING`)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO candidates(candidate_id,display_name) VALUES
		('candidate-001','Candidato ficticio de demostración') ON CONFLICT (candidate_id) DO NOTHING`)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *Repository) ListEmployees(ctx context.Context, page domain.Page, activeOnly bool) ([]domain.Employee, int64, error) {
	if err := page.Validate(); err != nil {
		return nil, 0, err
	}
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return nil, 0, err
	}
	defer tx.Rollback(ctx)
	var total int64
	if err = tx.QueryRow(ctx, `SELECT count(*) FROM employees WHERE (NOT $1::boolean OR active)`, activeOnly).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := tx.Query(ctx, `SELECT employee_id,display_name,active FROM employees WHERE (NOT $1::boolean OR active) ORDER BY employee_id LIMIT $2 OFFSET $3`, activeOnly, page.Size, page.Offset())
	if err != nil {
		return nil, 0, err
	}
	items := make([]domain.Employee, 0, page.Size)
	for rows.Next() {
		var item domain.Employee
		if err = rows.Scan(&item.ID, &item.DisplayName, &item.Active); err != nil {
			rows.Close()
			return nil, 0, err
		}
		items = append(items, item)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, 0, err
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r *Repository) GetEmployee(ctx context.Context, id string) (domain.Employee, error) {
	return scanEmployee(r.pool.QueryRow(ctx, `SELECT employee_id,display_name,active FROM employees WHERE employee_id=$1`, id))
}

func (r *Repository) CreateEmployee(ctx context.Context, item domain.Employee) error {
	_, err := r.pool.Exec(ctx, `INSERT INTO employees(employee_id,display_name,active) VALUES($1,$2,$3)`, item.ID, item.DisplayName, item.Active)
	return err
}

func (r *Repository) UpdateEmployee(ctx context.Context, item domain.Employee) (domain.Employee, error) {
	return scanEmployee(r.pool.QueryRow(ctx, `UPDATE employees SET display_name=$2, active=$3, updated_at=now() WHERE employee_id=$1 RETURNING employee_id,display_name,active`, item.ID, item.DisplayName, item.Active))
}

func (r *Repository) DeactivateEmployee(ctx context.Context, id string) (domain.Employee, error) {
	return scanEmployee(r.pool.QueryRow(ctx, `UPDATE employees SET active=false, updated_at=CASE WHEN active THEN now() ELSE updated_at END WHERE employee_id=$1 RETURNING employee_id,display_name,active`, id))
}

func (r *Repository) ListCandidates(ctx context.Context, page domain.Page) ([]domain.Candidate, int64, error) {
	if err := page.Validate(); err != nil {
		return nil, 0, err
	}
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return nil, 0, err
	}
	defer tx.Rollback(ctx)
	var total int64
	if err = tx.QueryRow(ctx, `SELECT count(*) FROM candidates`).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := tx.Query(ctx, `SELECT candidate_id,display_name FROM candidates ORDER BY candidate_id LIMIT $1 OFFSET $2`, page.Size, page.Offset())
	if err != nil {
		return nil, 0, err
	}
	items := make([]domain.Candidate, 0, page.Size)
	for rows.Next() {
		var item domain.Candidate
		if err = rows.Scan(&item.ID, &item.DisplayName); err != nil {
			rows.Close()
			return nil, 0, err
		}
		items = append(items, item)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, 0, err
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r *Repository) GetCandidate(ctx context.Context, id string) (domain.Candidate, error) {
	return scanCandidate(r.pool.QueryRow(ctx, `SELECT candidate_id,display_name FROM candidates WHERE candidate_id=$1`, id))
}

func (r *Repository) CreateCandidate(ctx context.Context, item domain.Candidate) error {
	_, err := r.pool.Exec(ctx, `INSERT INTO candidates(candidate_id,display_name) VALUES($1,$2)`, item.ID, item.DisplayName)
	return err
}

func (r *Repository) UpdateCandidate(ctx context.Context, item domain.Candidate) (domain.Candidate, error) {
	return scanCandidate(r.pool.QueryRow(ctx, `UPDATE candidates SET display_name=$2, updated_at=now() WHERE candidate_id=$1 RETURNING candidate_id,display_name`, item.ID, item.DisplayName))
}

func (r *Repository) DeleteCandidate(ctx context.Context, id string) error {
	command, err := r.pool.Exec(ctx, `DELETE FROM candidates WHERE candidate_id=$1`, id)
	if err != nil {
		var constraint *pgconn.PgError
		if errors.As(err, &constraint) && constraint.Code == "23503" {
			return domain.ErrEvidenceConflict
		}
		return err
	}
	if command.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func scanEmployee(row pgx.Row) (domain.Employee, error) {
	var item domain.Employee
	err := row.Scan(&item.ID, &item.DisplayName, &item.Active)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Employee{}, domain.ErrNotFound
	}
	return item, err
}

func scanCandidate(row pgx.Row) (domain.Candidate, error) {
	var item domain.Candidate
	err := row.Scan(&item.ID, &item.DisplayName)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Candidate{}, domain.ErrNotFound
	}
	return item, err
}
