CREATE TABLE IF NOT EXISTS customers (
    id text PRIMARY KEY,
    name text NOT NULL CHECK(length(name) BETWEEN 1 AND 200),
    active boolean NOT NULL,
    license_valid_until date NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS customer_inbox (
    event_id text PRIMARY KEY,
    received_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS customer_rental_history (
    event_id text PRIMARY KEY REFERENCES customer_inbox(event_id),
    reservation_id text NOT NULL UNIQUE,
    customer_id text NOT NULL REFERENCES customers(id),
    payload jsonb NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);
