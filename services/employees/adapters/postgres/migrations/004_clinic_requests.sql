CREATE TABLE clinic_patient_links (
 person_kind text NOT NULL CHECK(person_kind IN ('employee','candidate')),
 person_id text NOT NULL, patient_id text NOT NULL UNIQUE, registered_by text NOT NULL,
 employee_id text REFERENCES employees(employee_id), candidate_id text REFERENCES candidates(candidate_id),
 PRIMARY KEY(person_kind,person_id),
 CHECK ((person_kind='employee' AND employee_id=person_id AND employee_id IS NOT NULL AND candidate_id IS NULL)
 OR (person_kind='candidate' AND candidate_id=person_id AND candidate_id IS NOT NULL AND employee_id IS NULL))
);
CREATE TABLE clinic_review_requests (
 request_id text PRIMARY KEY, local_reference text NOT NULL UNIQUE,
 person_kind text NOT NULL, person_id text NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now(), external_id text UNIQUE,
 payload bytea NOT NULL,
 FOREIGN KEY(person_kind,person_id) REFERENCES clinic_patient_links(person_kind,person_id)
);
CREATE INDEX clinic_requests_person ON clinic_review_requests(person_kind,person_id,created_at DESC,request_id);
CREATE TABLE clinic_appointment_history (
 request_id text NOT NULL REFERENCES clinic_review_requests(request_id),
 version bigint NOT NULL CHECK(version>0), payload bytea NOT NULL,
 observed_at timestamptz NOT NULL DEFAULT now(), PRIMARY KEY(request_id,version)
);
