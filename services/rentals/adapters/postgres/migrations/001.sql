CREATE EXTENSION IF NOT EXISTS btree_gist WITH SCHEMA public;
CREATE TABLE IF NOT EXISTS rental_reservations (
    id text PRIMARY KEY,
    owner_app text NOT NULL,
    vehicle_id text NOT NULL,
    customer_id text NOT NULL,
    starts_on date NOT NULL,
    ends_on date NOT NULL,
    status text NOT NULL CHECK (status IN ('RESERVED','ACTIVE','COMPLETED','CANCELLED')),
    daily_rate_minor bigint NOT NULL CHECK (daily_rate_minor > 0),
    total_minor bigint NOT NULL CHECK (total_minor > 0),
    currency text NOT NULL CHECK (currency = 'ARS'),
    CHECK (ends_on > starts_on AND ends_on - starts_on <= 30)
);
CREATE UNIQUE INDEX IF NOT EXISTS rental_one_active_vehicle ON rental_reservations(vehicle_id) WHERE status='ACTIVE';
CREATE TABLE IF NOT EXISTS rental_agenda (
    id text PRIMARY KEY,
    reservation_id text UNIQUE REFERENCES rental_reservations(id),
    vehicle_id text NOT NULL,
    starts_on date NOT NULL,
    ends_on date NOT NULL,
    occupied boolean NOT NULL DEFAULT true,
    kind text NOT NULL CHECK(kind IN ('RESERVATION','MAINTENANCE')),
    period daterange GENERATED ALWAYS AS (daterange(starts_on,ends_on,'[)')) STORED,
    CHECK (ends_on > starts_on),
    CONSTRAINT rental_no_overlap EXCLUDE USING gist (vehicle_id WITH =, period WITH &&) WHERE (occupied)
);
CREATE TABLE IF NOT EXISTS rental_idempotency (
    app text NOT NULL,
    operation text NOT NULL,
    key text NOT NULL,
    fingerprint text NOT NULL,
    response_status smallint NOT NULL,
    response_json jsonb NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY(app,operation,key)
);
CREATE TABLE IF NOT EXISTS rental_outbox (
    event_id text PRIMARY KEY,
    event_type text NOT NULL,
    payload jsonb NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    published_at timestamptz
);
