-- 002 belonged to the withdrawn outbound enrollment implementation. Do not
-- reuse its version or delete data from databases that had already applied it.
CREATE TABLE membership_evidence (
 employee_id text NOT NULL REFERENCES employees(employee_id), period text NOT NULL,
 source_app text NOT NULL, source_reference text NOT NULL,
 version bigint NOT NULL CHECK(version>0), observed_at timestamptz NOT NULL,
 member boolean, used boolean, cost_minor bigint CHECK(cost_minor>=0),
 spending_minor bigint CHECK(spending_minor>=0), currency text NOT NULL,
 PRIMARY KEY(employee_id,period),
 CHECK ((cost_minor IS NULL AND spending_minor IS NULL AND currency='') OR currency ~ '^[A-Z]{3}$')
);
CREATE TABLE membership_evidence_history (
 employee_id text NOT NULL REFERENCES employees(employee_id), period text NOT NULL,
 version bigint NOT NULL CHECK(version>0), source_app text NOT NULL, payload bytea NOT NULL,
 received_at timestamptz NOT NULL DEFAULT now(), PRIMARY KEY(employee_id,period,version)
);
CREATE INDEX membership_evidence_period ON membership_evidence(period,employee_id);
