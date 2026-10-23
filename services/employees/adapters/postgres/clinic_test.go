package postgres_test

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Gerpeirone/Car-Cruds/internal/platform"
	"github.com/Gerpeirone/Car-Cruds/services/employees/adapters/postgres"
	"github.com/Gerpeirone/Car-Cruds/services/employees/application"
	"github.com/Gerpeirone/Car-Cruds/services/employees/domain"
	"github.com/jackc/pgx/v5/pgxpool"
)

type clinicDouble struct {
	book   func(context.Context, domain.ClinicRequest) (domain.ClinicAppointment, error)
	lookup func(context.Context, string) (domain.ClinicAppointment, error)
}

func (c clinicDouble) Book(ctx context.Context, r domain.ClinicRequest) (domain.ClinicAppointment, error) {
	return c.book(ctx, r)
}
func (c clinicDouble) Lookup(ctx context.Context, id string) (domain.ClinicAppointment, error) {
	return c.lookup(ctx, id)
}

func TestClinicDurableIntentionUnknownRecoveryAndHistory(t *testing.T) {
	pool, config := isolatedDatabase(t)
	repo := postgres.New(pool)
	ctx := context.Background()
	if err := repo.CreateEmployee(ctx, domain.Employee{ID: "employee", DisplayName: "Persona ficticia", Active: true}); err != nil {
		t.Fatal(err)
	}
	if err := repo.CreateEmployee(ctx, domain.Employee{ID: "unlinked", DisplayName: "Sin evidencia", Active: true}); err != nil {
		t.Fatal(err)
	}
	link := domain.PatientLink{PersonKind: "employee", PersonID: "employee", PatientID: "patient", RegisteredBy: "operator"}
	if err := repo.LinkClinicPatient(ctx, link); err != nil {
		t.Fatal(err)
	}
	if err := repo.LinkClinicPatient(ctx, link); err != nil {
		t.Fatal(err)
	}
	badLink := link
	badLink.PatientID = "other"
	if err := repo.LinkClinicPatient(ctx, badLink); !errors.Is(err, domain.ErrEvidenceConflict) {
		t.Fatalf("remap=%v", err)
	}
	start := time.Now().UTC().Add(72 * time.Hour).Truncate(time.Second)
	var calls atomic.Int64
	provider := clinicDouble{book: func(_ context.Context, r domain.ClinicRequest) (domain.ClinicAppointment, error) {
		calls.Add(1)
		saved, err := repo.GetClinicRequest(ctx, r.ID)
		if err != nil || saved.AttemptedAt == nil || saved.State != "unknown" {
			t.Errorf("claim not durable: %+v %v", saved, err)
		}
		return domain.ClinicAppointment{}, domain.ErrClinicUnavailable
	}}
	svc := application.New(repo, platform.UUID).WithClinic(repo, provider)
	input := domain.ClinicRequest{PersonKind: "employee", PersonID: "employee", LocalReference: "review-1", DoctorID: "doctor", StartAt: start, RegisteredBy: "operator"}
	r, err := svc.CreateClinicRequest(ctx, input)
	if err != nil {
		t.Fatal(err)
	}
	repeated, err := svc.CreateClinicRequest(ctx, input)
	if err != nil || repeated.ID != r.ID {
		t.Fatalf("local idempotency: %+v %v", repeated, err)
	}
	changed := input
	changed.DoctorID = "another"
	if _, err = svc.CreateClinicRequest(ctx, changed); !errors.Is(err, domain.ErrEvidenceConflict) {
		t.Fatalf("changed intention=%v", err)
	}
	r, err = svc.SubmitClinicRequest(ctx, r.ID)
	if err != nil || r.State != "unknown" || r.Classification() != "pending_verification" {
		t.Fatalf("response lost: %+v %v", r, err)
	}
	if _, err = svc.SubmitClinicRequest(ctx, r.ID); err != nil || calls.Load() != 1 {
		t.Fatalf("blind POST replay: %d %v", calls.Load(), err)
	}
	a := domain.ClinicAppointment{ID: "appointment", PatientID: "patient", DoctorID: "doctor", StartAt: start, EndAt: start.Add(30 * time.Minute), Status: "booked", Version: 1}
	provider.lookup = func(context.Context, string) (domain.ClinicAppointment, error) { return a, nil }
	svc.WithClinic(repo, provider)
	r, err = svc.RefreshClinicRequest(ctx, r.ID, "appointment")
	if err != nil || r.State != "booked" || r.ObservedAt == nil {
		t.Fatalf("recovery: %+v %v", r, err)
	}
	oldRevision := r.Revision
	a.Version = 2
	a.Status = "cancelled"
	r, err = svc.RefreshClinicRequest(ctx, r.ID, "")
	if err != nil || r.Classification() != "without_confirmed_appointment" {
		t.Fatalf("cancel: %+v %v", r, err)
	}
	if r, err = repo.FailClinicRequest(ctx, r.ID, oldRevision, "LATE_FAILURE", false); err != nil || r.State != "cancelled" {
		t.Fatalf("late failure overwrote newer evidence: %+v %v", r, err)
	}
	a.Version = 1
	a.Status = "booked"
	r, err = svc.RefreshClinicRequest(ctx, r.ID, "")
	if err != nil || r.State != "cancelled" {
		t.Fatalf("late response: %+v %v", r, err)
	}
	provider.lookup = func(context.Context, string) (domain.ClinicAppointment, error) {
		return domain.ClinicAppointment{}, domain.ErrClinicUnavailable
	}
	svc.WithClinic(repo, provider)
	r, err = svc.RefreshClinicRequest(ctx, r.ID, "")
	if err != nil || r.State != "unknown" || r.Appointment.Status != "cancelled" {
		t.Fatalf("failed refresh lost last evidence: %+v %v", r, err)
	}
	rows, total, err := svc.ClinicReport(ctx, "employee", domain.Page{Number: 1, Size: 20})
	if err != nil || total != 2 || len(rows) != 2 || rows[1].Request != nil {
		t.Fatalf("missing people: %+v %d %v", rows, total, err)
	}
	var count int
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM clinic_appointment_history`).Scan(&count); err != nil || count != 2 {
		t.Fatalf("history=%d %v", count, err)
	}
	pool.Close()
	reopened, err := pgxpool.NewWithConfig(ctx, config.Copy())
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	repo = postgres.New(reopened)
	svc = application.New(repo, platform.UUID).WithClinic(repo, provider)
	if _, err = svc.SubmitClinicRequest(ctx, r.ID); err != nil || calls.Load() != 1 {
		t.Fatalf("restart replay: calls=%d err=%v", calls.Load(), err)
	}
}

func TestClinicConcurrentSubmitSendsOnceAndCandidateLinkIsProtected(t *testing.T) {
	pool, _ := isolatedDatabase(t)
	repo := postgres.New(pool)
	ctx := context.Background()
	if err := repo.CreateCandidate(ctx, domain.Candidate{ID: "candidate", DisplayName: "Candidato ficticio"}); err != nil {
		t.Fatal(err)
	}
	if err := repo.LinkClinicPatient(ctx, domain.PatientLink{PersonKind: "candidate", PersonID: "candidate", PatientID: "candidate-patient", RegisteredBy: "operator"}); err != nil {
		t.Fatal(err)
	}
	if err := repo.DeleteCandidate(ctx, "candidate"); !errors.Is(err, domain.ErrEvidenceConflict) {
		t.Fatalf("deleted referenced person: %v", err)
	}
	var calls atomic.Int64
	provider := clinicDouble{book: func(context.Context, domain.ClinicRequest) (domain.ClinicAppointment, error) {
		calls.Add(1)
		return domain.ClinicAppointment{}, &domain.ClinicRejection{Code: "PATIENT_NOT_ENABLED"}
	}}
	svc := application.New(repo, platform.UUID).WithClinic(repo, provider)
	r, err := svc.CreateClinicRequest(ctx, domain.ClinicRequest{PersonKind: "candidate", PersonID: "candidate", LocalReference: "candidate-review", DoctorID: "doctor", StartAt: time.Now().UTC().Add(72 * time.Hour).Truncate(time.Second), RegisteredBy: "operator"})
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := svc.SubmitClinicRequest(ctx, r.ID)
			if err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	r, err = svc.GetClinicRequest(ctx, r.ID)
	if err != nil || r.State != "rejected" || calls.Load() != 1 {
		t.Fatalf("state=%+v calls=%d err=%v", r, calls.Load(), err)
	}
	rows, total, err := svc.ClinicReport(ctx, "employee", domain.Page{Number: 1, Size: 20})
	if err != nil || total != 0 || len(rows) != 0 {
		t.Fatalf("candidate leaked: %+v %d %v", rows, total, err)
	}
}
