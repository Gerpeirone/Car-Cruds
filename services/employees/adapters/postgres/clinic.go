package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/Gerpeirone/Car-Cruds/services/employees/application"
	"github.com/Gerpeirone/Car-Cruds/services/employees/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

var _ application.ClinicStore = (*Repository)(nil)

func clinicStorageError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23505":
			return domain.ErrEvidenceConflict
		case "23503":
			return domain.ErrNotFound
		}
	}
	return err
}
func (r *Repository) LinkClinicPatient(ctx context.Context, l domain.PatientLink) error {
	if !domain.ValidPersonKind(l.PersonKind) || !domain.ValidEvidenceKey(l.PersonID) || !domain.ValidEvidenceKey(l.PatientID) || !domain.ValidEvidenceKey(l.RegisteredBy) {
		return domain.ErrInvalidInput
	}
	var employee, candidate *string
	if l.PersonKind == "employee" {
		employee = &l.PersonID
	} else {
		candidate = &l.PersonID
	}
	command, err := r.pool.Exec(ctx, `INSERT INTO clinic_patient_links(person_kind,person_id,patient_id,registered_by,employee_id,candidate_id) VALUES($1,$2,$3,$4,$5,$6)
	ON CONFLICT(person_kind,person_id) DO UPDATE SET patient_id=clinic_patient_links.patient_id WHERE clinic_patient_links.patient_id=EXCLUDED.patient_id`, l.PersonKind, l.PersonID, l.PatientID, l.RegisteredBy, employee, candidate)
	if err != nil {
		return clinicStorageError(err)
	}
	if command.RowsAffected() == 0 {
		return domain.ErrEvidenceConflict
	}
	return nil
}
func decodeClinic(row pgx.Row) (domain.ClinicRequest, error) {
	var b []byte
	var request domain.ClinicRequest
	if err := row.Scan(&b); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return request, domain.ErrNotFound
		}
		return request, err
	}
	err := json.Unmarshal(b, &request)
	return request, err
}
func (r *Repository) GetClinicRequest(ctx context.Context, id string) (domain.ClinicRequest, error) {
	return decodeClinic(r.pool.QueryRow(ctx, `SELECT payload FROM clinic_review_requests WHERE request_id=$1`, id))
}
func (r *Repository) CreateClinicRequest(ctx context.Context, s domain.ClinicRequest) (domain.ClinicRequest, error) {
	if err := s.ValidateInput(); err != nil {
		return s, err
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return s, err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,724105))`, s.LocalReference); err != nil {
		return s, err
	}
	old, err := decodeClinic(tx.QueryRow(ctx, `SELECT payload FROM clinic_review_requests WHERE local_reference=$1`, s.LocalReference))
	if err == nil {
		if old.PersonKind != s.PersonKind || old.PersonID != s.PersonID || old.DoctorID != s.DoctorID || !old.StartAt.Equal(s.StartAt) {
			return old, domain.ErrEvidenceConflict
		}
		return old, tx.Commit(ctx)
	}
	if !errors.Is(err, domain.ErrNotFound) {
		return s, err
	}
	err = tx.QueryRow(ctx, `SELECT patient_id FROM clinic_patient_links WHERE person_kind=$1 AND person_id=$2`, s.PersonKind, s.PersonID).Scan(&s.PatientID)
	if errors.Is(err, pgx.ErrNoRows) {
		return s, domain.ErrPatientLinkRequired
	}
	if err != nil {
		return s, err
	}
	s.State = "ready"
	s.Revision = 1
	s.CreatedAt = time.Now().UTC()
	s.StartAt = s.StartAt.UTC()
	payload, err := json.Marshal(s)
	if err != nil {
		return s, err
	}
	_, err = tx.Exec(ctx, `INSERT INTO clinic_review_requests(request_id,local_reference,person_kind,person_id,created_at,payload) VALUES($1,$2,$3,$4,$5,$6)`, s.ID, s.LocalReference, s.PersonKind, s.PersonID, s.CreatedAt, payload)
	if err != nil {
		return s, clinicStorageError(err)
	}
	return s, tx.Commit(ctx)
}

func (r *Repository) mutateClinic(ctx context.Context, id string, change func(*domain.ClinicRequest, pgx.Tx) error) (domain.ClinicRequest, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.ClinicRequest{}, err
	}
	defer tx.Rollback(ctx)
	s, err := decodeClinic(tx.QueryRow(ctx, `SELECT payload FROM clinic_review_requests WHERE request_id=$1 FOR UPDATE`, id))
	if err != nil {
		return s, err
	}
	if err = change(&s, tx); err != nil {
		return s, err
	}
	payload, err := json.Marshal(s)
	if err != nil {
		return s, err
	}
	var externalID *string
	if s.Appointment != nil {
		externalID = &s.Appointment.ID
	}
	if _, err = tx.Exec(ctx, `UPDATE clinic_review_requests SET payload=$2,external_id=$3 WHERE request_id=$1`, id, payload, externalID); err != nil {
		return s, clinicStorageError(err)
	}
	return s, tx.Commit(ctx)
}
func (r *Repository) ClaimClinicRequest(ctx context.Context, id string) (domain.ClinicRequest, bool, error) {
	claimed := false
	s, err := r.mutateClinic(ctx, id, func(s *domain.ClinicRequest, _ pgx.Tx) error {
		if s.AttemptedAt != nil {
			return nil
		}
		now := time.Now().UTC()
		s.AttemptedAt = &now
		s.State = "unknown"
		s.FailureCode = "AWAITING_CONFIRMATION"
		s.Revision++
		claimed = true
		return nil
	})
	return s, claimed, err
}
func (r *Repository) FailClinicRequest(ctx context.Context, id string, revision int64, code string, rejected bool) (domain.ClinicRequest, error) {
	return r.mutateClinic(ctx, id, func(s *domain.ClinicRequest, _ pgx.Tx) error {
		if s.Revision != revision {
			return nil
		}
		s.State = "unknown"
		if rejected {
			s.State = "rejected"
		}
		s.FailureCode = code
		s.Revision++
		return nil
	})
}
func (r *Repository) ObserveClinicAppointment(ctx context.Context, id string, a domain.ClinicAppointment) (domain.ClinicRequest, error) {
	if err := a.Validate(); err != nil {
		return domain.ClinicRequest{}, err
	}
	a.StartAt = a.StartAt.UTC()
	a.EndAt = a.EndAt.UTC()
	return r.mutateClinic(ctx, id, func(s *domain.ClinicRequest, tx pgx.Tx) error {
		if s.AttemptedAt == nil || s.PatientID != a.PatientID {
			return domain.ErrEvidenceConflict
		}
		if s.Appointment == nil {
			if s.DoctorID != a.DoctorID || !s.StartAt.Equal(a.StartAt) {
				return domain.ErrEvidenceConflict
			}
		} else {
			old := s.Appointment
			if old.ID != a.ID {
				return domain.ErrEvidenceConflict
			}
			if a.Version < old.Version {
				return nil
			}
			if a.Version == old.Version && (*old != a) {
				return domain.ErrEvidenceConflict
			}
		}
		b, err := json.Marshal(a)
		if err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `INSERT INTO clinic_appointment_history(request_id,version,payload) VALUES($1,$2,$3) ON CONFLICT DO NOTHING`, id, a.Version, b); err != nil {
			return err
		}
		now := time.Now().UTC()
		s.Appointment = &a
		s.State = a.Status
		s.ObservedAt = &now
		s.FailureCode = ""
		s.Revision++
		return nil
	})
}
func (r *Repository) ClinicReport(ctx context.Context, kind string, page domain.Page) ([]domain.ClinicPersonReview, int64, error) {
	if !domain.ValidPersonKind(kind) {
		return nil, 0, domain.ErrInvalidInput
	}
	if err := page.Validate(); err != nil {
		return nil, 0, err
	}
	// Kind is allowlisted above; SQL identifiers are never supplied by callers.
	population := `SELECT employee_id AS id,display_name,active FROM employees`
	if kind == "candidate" {
		population = `SELECT candidate_id AS id,display_name,NULL::boolean AS active FROM candidates`
	}
	rows, err := r.pool.Query(ctx, `SELECT p.id,p.display_name,p.active,r.payload FROM (`+population+`) p
	LEFT JOIN LATERAL (SELECT payload FROM clinic_review_requests WHERE person_kind=$1 AND person_id=p.id ORDER BY created_at DESC,request_id DESC LIMIT 1) r ON true ORDER BY p.id`, kind)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	items := []domain.ClinicPersonReview{}
	var total int64
	for rows.Next() {
		var item domain.ClinicPersonReview
		var b []byte
		item.PersonKind = kind
		if err = rows.Scan(&item.PersonID, &item.DisplayName, &item.Active, &b); err != nil {
			return nil, 0, err
		}
		if b != nil {
			item.Request = new(domain.ClinicRequest)
			if err = json.Unmarshal(b, item.Request); err != nil {
				return nil, 0, err
			}
		}
		if total >= page.Offset() && len(items) < page.Size {
			items = append(items, item)
		}
		total++
	}
	return items, total, rows.Err()
}
