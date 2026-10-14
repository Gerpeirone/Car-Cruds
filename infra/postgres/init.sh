#!/bin/bash
set -eu
psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" \
  -v customers_password="$CUSTOMERS_DB_PASSWORD" \
  -v rentals_password="$RENTALS_DB_PASSWORD" \
  -v employees_password="$EMPLOYEES_DB_PASSWORD" <<'SQL'
CREATE ROLE customers LOGIN PASSWORD :'customers_password';
CREATE ROLE rentals LOGIN PASSWORD :'rentals_password';
CREATE ROLE employees LOGIN PASSWORD :'employees_password';
CREATE DATABASE customers OWNER customers;
CREATE DATABASE rentals OWNER rentals;
CREATE DATABASE employees OWNER employees;
REVOKE CONNECT ON DATABASE customers FROM PUBLIC;
REVOKE CONNECT ON DATABASE rentals FROM PUBLIC;
REVOKE CONNECT ON DATABASE employees FROM PUBLIC;
GRANT CONNECT ON DATABASE customers TO customers;
GRANT CONNECT ON DATABASE rentals TO rentals;
GRANT CONNECT ON DATABASE employees TO employees;
\connect rentals
CREATE EXTENSION IF NOT EXISTS btree_gist;
SQL
