CREATE TABLE IF NOT EXISTS employees (
    employee_id text PRIMARY KEY,
    display_name text NOT NULL CHECK (char_length(btrim(display_name)) BETWEEN 1 AND 150),
    active boolean NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS employees_active_order ON employees (employee_id) WHERE active;

CREATE TABLE IF NOT EXISTS candidates (
    candidate_id text PRIMARY KEY,
    display_name text NOT NULL CHECK (char_length(btrim(display_name)) BETWEEN 1 AND 150),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
