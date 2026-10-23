CREATE TABLE gym_enrollments (
    enrollment_id text PRIMARY KEY,
    employee_id text NOT NULL REFERENCES employees(employee_id),
    app text NOT NULL,
    idempotency_key text NOT NULL,
    fingerprint text NOT NULL,
    dni text NOT NULL,
    email text NOT NULL,
    employee_name text NOT NULL,
    state text NOT NULL CHECK (state IN ('PENDING','UNKNOWN','ACCEPTED','REJECTED')),
    provider_code text NOT NULL DEFAULT '',
    socio_id text NOT NULL DEFAULT '',
    solicitud_id text NOT NULL DEFAULT '',
    lease_token text NOT NULL DEFAULT '',
    lease_until timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE(app, idempotency_key)
);
CREATE UNIQUE INDEX gym_one_unresolved_or_accepted ON gym_enrollments(employee_id) WHERE state <> 'REJECTED';
