package data

import (
	"context"
	"errors"
	"time"

	"github.com/Gerpeirone/Car-Cruds/services/customers/business"
	"github.com/jackc/pgx/v5"
)

func (r *Repository) InsertAccount(ctx context.Context, a business.Account, hash string) error {
	_, err := r.Pool.Exec(ctx, `INSERT INTO customer_accounts (id, first_name, last_name, email, password_hash)
		VALUES ($1,$2,$3,$4,$5) ON CONFLICT (email) DO NOTHING`, a.ID, a.FirstName, a.LastName, a.Email, hash)
	return err
}

func (r *Repository) AccountByEmail(ctx context.Context, email string) (business.Account, string, error) {
	var a business.Account
	var hash string
	err := r.Pool.QueryRow(ctx, `SELECT id::text, first_name, last_name, email, role, password_hash
		FROM customer_accounts WHERE email=$1`, email).Scan(&a.ID, &a.FirstName, &a.LastName, &a.Email, &a.Role, &hash)
	if errors.Is(err, pgx.ErrNoRows) {
		err = business.ErrAccountAuth
	}
	return a, hash, err
}

func (r *Repository) InsertSession(ctx context.Context, id string, tokenHash []byte, expires time.Time) error {
	_, err := r.Pool.Exec(ctx, `WITH expired AS (
		SELECT token_hash FROM customer_sessions WHERE expires_at <= now()
		ORDER BY expires_at LIMIT 100 FOR UPDATE SKIP LOCKED
	), removed AS (
		DELETE FROM customer_sessions s USING expired WHERE s.token_hash=expired.token_hash
	)
	INSERT INTO customer_sessions (token_hash, account_id, expires_at) VALUES ($1,$2,$3)`, tokenHash, id, expires)
	return err
}

func (r *Repository) AccountBySession(ctx context.Context, tokenHash []byte) (business.Account, error) {
	var a business.Account
	err := r.Pool.QueryRow(ctx, `SELECT a.id::text, a.first_name, a.last_name, a.email, a.role
		FROM customer_sessions s JOIN customer_accounts a ON a.id=s.account_id
		WHERE s.token_hash=$1 AND s.expires_at > now()`, tokenHash).Scan(&a.ID, &a.FirstName, &a.LastName, &a.Email, &a.Role)
	if errors.Is(err, pgx.ErrNoRows) {
		err = business.ErrAccountAuth
	}
	return a, err
}

func (r *Repository) DeleteSession(ctx context.Context, tokenHash []byte) error {
	_, err := r.Pool.Exec(ctx, "DELETE FROM customer_sessions WHERE token_hash=$1", tokenHash)
	return err
}
