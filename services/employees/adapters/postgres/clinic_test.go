package postgres_test

import (
	"context"
	"encoding/json"
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
	expireClinicCooldown(t, pool, r.ID)
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
	if r, err = repo.FailClinicRequest(ctx, r.ID, oldRevision, "LATE_FAILURE", false, nil); err != nil || r.State != "cancelled" {
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

// Expire the durable deadline without sleeping, simulating a later explicit
// action while preserving all other payload evidence.
func expireClinicCooldown(t *testing.T, pool *pgxpool.Pool, id string) {
	t.Helper()
	ctx := context.Background()
	var payload []byte
	if err := pool.QueryRow(ctx, `SELECT payload FROM clinic_review_requests WHERE request_id=$1`, id).Scan(&payload); err != nil {
		t.Fatal(err)
	}
	var request domain.ClinicRequest
	if err := json.Unmarshal(payload, &request); err != nil {
		t.Fatal(err)
	}
	expired := time.Now().UTC().Add(-time.Minute)
	request.NextAttemptAt = &expired
	payload, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `UPDATE clinic_review_requests SET payload=$2 WHERE request_id=$1`, id, payload); err != nil {
		t.Fatal(err)
	}
}

func newClinicIntention(t *testing.T, repo *postgres.Repository) domain.ClinicRequest {
	t.Helper()
	ctx := context.Background()
	if err := repo.CreateEmployee(ctx, domain.Employee{ID: "employee", DisplayName: "Persona ficticia", Active: true}); err != nil {
		t.Fatal(err)
	}
	if err := repo.LinkClinicPatient(ctx, domain.PatientLink{PersonKind: "employee", PersonID: "employee", PatientID: "patient", RegisteredBy: "operator"}); err != nil {
		t.Fatal(err)
	}
	svc := application.New(repo, platform.UUID).WithClinic(repo, nil)
	r, err := svc.CreateClinicRequest(ctx, domain.ClinicRequest{PersonKind: "employee", PersonID: "employee", LocalReference: "review", DoctorID: "doctor", StartAt: time.Now().UTC().Add(72 * time.Hour).Truncate(time.Second), RegisteredBy: "operator"})
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func appointmentFor(r domain.ClinicRequest) domain.ClinicAppointment {
	return domain.ClinicAppointment{ID: "appointment", PatientID: r.PatientID, DoctorID: r.DoctorID, StartAt: r.StartAt, EndAt: r.StartAt.Add(30 * time.Minute), Status: "booked", Version: 1}
}

func TestClinicV1LostResponseRecoversSameKeyAfterRestartWithoutDuplicateEffects(t *testing.T) {
	pool, config := isolatedDatabase(t)
	repo := postgres.New(pool)
	ctx := context.Background()
	r := newClinicIntention(t, repo)
	if r.ProviderContract != domain.ClinicProviderContract || r.IdempotencyKey != r.ID || r.NextAttemptAt != nil {
		t.Fatalf("new intention provenance: %+v", r)
	}
	var calls, effects int
	var upstreamKey string
	a := appointmentFor(r)
	provider := clinicDouble{book: func(_ context.Context, request domain.ClinicRequest) (domain.ClinicAppointment, error) {
		calls++
		saved, err := repo.GetClinicRequest(ctx, request.ID)
		if err != nil || saved.IdempotencyKey != request.ID || saved.ProviderContract != domain.ClinicProviderContract || saved.AttemptedAt == nil || saved.NextAttemptAt == nil {
			t.Fatalf("network preceded durable key/claim: %+v %v", saved, err)
		}
		if upstreamKey == "" {
			upstreamKey = request.IdempotencyKey
			effects++
			return domain.ClinicAppointment{}, &domain.ClinicFailure{Code: "CLINIC_UNAVAILABLE"}
		}
		if request.IdempotencyKey != upstreamKey || request.PatientID != r.PatientID || request.DoctorID != r.DoctorID || !request.StartAt.Equal(r.StartAt) {
			t.Fatalf("replay changed key/body: %+v", request)
		}
		// The provider's replay returns current evidence, including cancellation.
		a.Status, a.Version = "cancelled", 2
		return a, nil
	}}
	svc := application.New(repo, platform.UUID).WithClinic(repo, provider)
	r, err := svc.SubmitClinicRequest(ctx, r.ID)
	if err != nil || r.State != "unknown" || r.FailureCode != "CLINIC_UNAVAILABLE" || r.NextAttemptAt == nil {
		t.Fatalf("lost response: %+v %v", r, err)
	}
	if _, err = svc.SubmitClinicRequest(ctx, r.ID); err != nil || calls != 1 {
		t.Fatalf("cooldown calls=%d err=%v", calls, err)
	}
	pool.Close()
	reopened, err := pgxpool.NewWithConfig(ctx, config.Copy())
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	repo = postgres.New(reopened)
	expireClinicCooldown(t, reopened, r.ID)
	svc = application.New(repo, platform.UUID).WithClinic(repo, provider)
	r, err = svc.SubmitClinicRequest(ctx, r.ID)
	if err != nil || r.State != "cancelled" || r.Appointment == nil || r.Appointment.Version != 2 || r.NextAttemptAt != nil || calls != 2 || effects != 1 {
		t.Fatalf("recovery: %+v calls=%d effects=%d err=%v", r, calls, effects, err)
	}
	if _, err = svc.SubmitClinicRequest(ctx, r.ID); err != nil || calls != 2 {
		t.Fatalf("known appointment replayed POST: %d %v", calls, err)
	}
}

func TestClinicV1ClaimLeaseSurvivesCrashAndSerializesConcurrentSubmit(t *testing.T) {
	pool, _ := isolatedDatabase(t)
	repo := postgres.New(pool)
	ctx := context.Background()
	r := newClinicIntention(t, repo)
	first, claimed, err := repo.ClaimClinicRequest(ctx, r.ID)
	if err != nil || !claimed || first.NextAttemptAt == nil {
		t.Fatalf("first claim: %+v %t %v", first, claimed, err)
	}
	// No failure write: the previous process stopped after its durable claim.
	if _, claimed, err = repo.ClaimClinicRequest(ctx, r.ID); err != nil || claimed {
		t.Fatalf("live lease claimed again: %t %v", claimed, err)
	}
	expireClinicCooldown(t, pool, r.ID)
	var calls atomic.Int64
	entered, release := make(chan struct{}), make(chan struct{})
	provider := clinicDouble{book: func(_ context.Context, request domain.ClinicRequest) (domain.ClinicAppointment, error) {
		if calls.Add(1) == 1 {
			close(entered)
		}
		<-release
		return appointmentFor(request), nil
	}}
	svc := application.New(repo, platform.UUID).WithClinic(repo, provider)
	done := make(chan error, 1)
	go func() { _, err := svc.SubmitClinicRequest(ctx, r.ID); done <- err }()
	<-entered
	var wg sync.WaitGroup
	for range 6 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := svc.SubmitClinicRequest(ctx, r.ID); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	close(release)
	if err := <-done; err != nil || calls.Load() != 1 {
		t.Fatalf("concurrent lease calls=%d err=%v", calls.Load(), err)
	}
	if r, err = repo.FailClinicRequest(ctx, r.ID, first.Revision, "CLINIC_UNAVAILABLE", false, nil); err != nil || r.State != "booked" {
		t.Fatalf("stale claim failure overwrote success: %+v %v", r, err)
	}
}

func TestClinicV1LegacyProvenanceAndActiveEmployeeGate(t *testing.T) {
	for _, scenario := range []string{"legacy-sent", "legacy-ready", "inactive-uncertain"} {
		t.Run(scenario, func(t *testing.T) {
			pool, _ := isolatedDatabase(t)
			repo := postgres.New(pool)
			ctx := context.Background()
			r := newClinicIntention(t, repo)
			if scenario == "legacy-sent" || scenario == "legacy-ready" {
				r.ProviderContract, r.IdempotencyKey = "", ""
			}
			if scenario != "legacy-ready" {
				now := time.Now().UTC()
				r.AttemptedAt, r.State = &now, "unknown"
			}
			payload, err := json.Marshal(r)
			if err != nil {
				t.Fatal(err)
			}
			if _, err = pool.Exec(ctx, `UPDATE clinic_review_requests SET payload=$2 WHERE request_id=$1`, r.ID, payload); err != nil {
				t.Fatal(err)
			}
			if scenario == "inactive-uncertain" {
				if _, err = repo.DeactivateEmployee(ctx, r.PersonID); err != nil {
					t.Fatal(err)
				}
			}
			var calls int
			provider := clinicDouble{book: func(_ context.Context, request domain.ClinicRequest) (domain.ClinicAppointment, error) {
				calls++
				if request.ProviderContract != domain.ClinicProviderContract || request.IdempotencyKey != request.ID {
					t.Fatalf("legacy ready did not adopt v1 durably: %+v", request)
				}
				return appointmentFor(request), nil
			}}
			svc := application.New(repo, platform.UUID).WithClinic(repo, provider)
			got, err := svc.SubmitClinicRequest(ctx, r.ID)
			if scenario == "inactive-uncertain" {
				if !errors.Is(err, domain.ErrEmployeeInactive) || calls != 0 {
					t.Fatalf("inactive uncertain POST calls=%d err=%v", calls, err)
				}
			} else if scenario == "legacy-sent" {
				if err != nil || calls != 0 || got.ProviderContract != "" || got.IdempotencyKey != "" || got.State != "unknown" {
					t.Fatalf("legacy uncertain replay: %+v calls=%d err=%v", got, calls, err)
				}
			} else if err != nil || calls != 1 || got.State != "booked" {
				t.Fatalf("legacy ready: %+v calls=%d err=%v", got, calls, err)
			}
		})
	}
}

func TestClinicV1TechnicalFailuresRemainUnknownAndRespectRetryPolicy(t *testing.T) {
	for _, code := range []string{"CLINIC_UNAUTHORIZED", "CLINIC_RATE_LIMITED", "CLINIC_IDEMPOTENCY_CONFLICT"} {
		t.Run(code, func(t *testing.T) {
			pool, _ := isolatedDatabase(t)
			repo := postgres.New(pool)
			ctx := context.Background()
			r := newClinicIntention(t, repo)
			var calls int
			provider := clinicDouble{book: func(_ context.Context, request domain.ClinicRequest) (domain.ClinicAppointment, error) {
				calls++
				if calls == 1 {
					delay := time.Duration(0)
					if code == "CLINIC_RATE_LIMITED" {
						delay = time.Minute
					}
					return domain.ClinicAppointment{}, &domain.ClinicFailure{Code: code, RetryAfter: delay}
				}
				return appointmentFor(request), nil
			}, lookup: func(context.Context, string) (domain.ClinicAppointment, error) {
				return domain.ClinicAppointment{}, &domain.ClinicFailure{Code: "CLINIC_NOT_FOUND"}
			}}
			svc := application.New(repo, platform.UUID).WithClinic(repo, provider)
			before := time.Now().UTC()
			r, err := svc.SubmitClinicRequest(ctx, r.ID)
			if err != nil || r.State != "unknown" || r.FailureCode != code {
				t.Fatalf("technical failure became clinical rejection: %+v %v", r, err)
			}
			if code == "CLINIC_IDEMPOTENCY_CONFLICT" {
				if r.NextAttemptAt != nil {
					t.Fatalf("conflict scheduled retry: %+v", r)
				}
				// Even an unsuccessful lookup cannot clear the POST conflict.
				if r, err = svc.RefreshClinicRequest(ctx, r.ID, "appointment"); err != nil || r.FailureCode != code {
					t.Fatalf("reconciliation weakened conflict: %+v %v", r, err)
				}
			} else {
				minimum := 9 * time.Second
				if code == "CLINIC_RATE_LIMITED" {
					minimum = 59 * time.Second
				}
				if r.NextAttemptAt == nil || !r.NextAttemptAt.After(before.Add(minimum)) {
					t.Fatalf("Retry-After lost: %+v", r)
				}
				if _, err = svc.SubmitClinicRequest(ctx, r.ID); err != nil || calls != 1 {
					t.Fatalf("cooldown ignored: calls=%d err=%v", calls, err)
				}
				expireClinicCooldown(t, pool, r.ID)
			}
			r, err = svc.SubmitClinicRequest(ctx, r.ID)
			if err != nil {
				t.Fatal(err)
			}
			if code == "CLINIC_IDEMPOTENCY_CONFLICT" {
				if calls != 1 || r.State != "unknown" || r.FailureCode != code {
					t.Fatalf("conflict POST retried: %+v calls=%d", r, calls)
				}
			} else if calls != 2 || r.State != "booked" || r.IdempotencyKey != r.ID {
				t.Fatalf("corrected access recovery: %+v calls=%d", r, calls)
			}
		})
	}
}

func TestClinicV1RefreshRateLimitPreservesHistoryAndDefersNetwork(t *testing.T) {
	pool, _ := isolatedDatabase(t)
	repo := postgres.New(pool)
	ctx := context.Background()
	r := newClinicIntention(t, repo)
	var lookups int
	provider := clinicDouble{book: func(_ context.Context, request domain.ClinicRequest) (domain.ClinicAppointment, error) {
		return appointmentFor(request), nil
	}, lookup: func(context.Context, string) (domain.ClinicAppointment, error) {
		lookups++
		return domain.ClinicAppointment{}, &domain.ClinicFailure{Code: "CLINIC_RATE_LIMITED", RetryAfter: time.Minute}
	}}
	svc := application.New(repo, platform.UUID).WithClinic(repo, provider)
	if _, err := svc.SubmitClinicRequest(ctx, r.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.DeactivateEmployee(ctx, r.PersonID); err != nil {
		t.Fatal(err)
	}
	r, err := svc.RefreshClinicRequest(ctx, r.ID, "")
	if err != nil || r.NextAttemptAt == nil || r.FailureCode != "CLINIC_RATE_LIMITED" || r.Appointment == nil || r.Appointment.Status != "booked" {
		t.Fatalf("rate limit lost prior evidence: %+v %v", r, err)
	}
	if _, err := svc.RefreshClinicRequest(ctx, r.ID, ""); err != nil || lookups != 1 {
		t.Fatalf("GET ignored rate cooldown: %d %v", lookups, err)
	}
}

func TestClinicConcurrentEmployeeSubmitSendsOnce(t *testing.T) {
	pool, _ := isolatedDatabase(t)
	repo := postgres.New(pool)
	ctx := context.Background()
	if err := repo.CreateEmployee(ctx, domain.Employee{ID: "employee", DisplayName: "Empleado ficticio", Active: true}); err != nil {
		t.Fatal(err)
	}
	if err := repo.LinkClinicPatient(ctx, domain.PatientLink{PersonKind: "employee", PersonID: "employee", PatientID: "employee-patient", RegisteredBy: "operator"}); err != nil {
		t.Fatal(err)
	}
	var calls atomic.Int64
	provider := clinicDouble{book: func(context.Context, domain.ClinicRequest) (domain.ClinicAppointment, error) {
		calls.Add(1)
		return domain.ClinicAppointment{}, &domain.ClinicRejection{Code: "PATIENT_NOT_ENABLED"}
	}}
	svc := application.New(repo, platform.UUID).WithClinic(repo, provider)
	r, err := svc.CreateClinicRequest(ctx, domain.ClinicRequest{PersonKind: "employee", PersonID: "employee", LocalReference: "employee-review", DoctorID: "doctor", StartAt: time.Now().UTC().Add(72 * time.Hour).Truncate(time.Second), RegisteredBy: "operator"})
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
	if err != nil || total != 1 || len(rows) != 1 || rows[0].Request == nil {
		t.Fatalf("employee missing: %+v %d %v", rows, total, err)
	}
}

func TestClinicInactiveEmployeesCannotStartNewActionsButKeepHistory(t *testing.T) {
	pool, _ := isolatedDatabase(t)
	repo := postgres.New(pool)
	ctx := context.Background()
	for _, id := range []string{"employee", "unlinked"} {
		if err := repo.CreateEmployee(ctx, domain.Employee{ID: id, DisplayName: "Persona ficticia", Active: true}); err != nil {
			t.Fatal(err)
		}
	}
	link := domain.PatientLink{PersonKind: "employee", PersonID: "employee", PatientID: "patient", RegisteredBy: "operator"}
	if err := repo.LinkClinicPatient(ctx, link); err != nil {
		t.Fatal(err)
	}
	start := time.Now().UTC().Add(72 * time.Hour).Truncate(time.Second)
	input := domain.ClinicRequest{PersonKind: "employee", PersonID: "employee", LocalReference: "ready-review", DoctorID: "doctor", StartAt: start, RegisteredBy: "operator"}
	var calls atomic.Int64
	a := domain.ClinicAppointment{ID: "appointment", PatientID: "patient", DoctorID: "doctor", StartAt: start, EndAt: start.Add(30 * time.Minute), Status: "booked", Version: 1}
	provider := clinicDouble{book: func(context.Context, domain.ClinicRequest) (domain.ClinicAppointment, error) {
		calls.Add(1)
		return a, nil
	}, lookup: func(context.Context, string) (domain.ClinicAppointment, error) { return a, nil }}
	svc := application.New(repo, platform.UUID).WithClinic(repo, provider)
	ready, err := svc.CreateClinicRequest(ctx, input)
	if err != nil {
		t.Fatal(err)
	}
	input.LocalReference = "sent-review"
	sent, err := svc.CreateClinicRequest(ctx, input)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = svc.SubmitClinicRequest(ctx, sent.ID); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"employee", "unlinked"} {
		if _, err = repo.DeactivateEmployee(ctx, id); err != nil {
			t.Fatal(err)
		}
	}
	link.PersonID, link.PatientID = "unlinked", "another-patient"
	if err = svc.LinkClinicPatient(ctx, link); !errors.Is(err, domain.ErrEmployeeInactive) {
		t.Fatalf("inactive link: %v", err)
	}
	input.LocalReference = "new-review"
	if _, err = svc.CreateClinicRequest(ctx, input); !errors.Is(err, domain.ErrEmployeeInactive) {
		t.Fatalf("inactive intention: %v", err)
	}
	if _, err = svc.SubmitClinicRequest(ctx, ready.ID); !errors.Is(err, domain.ErrEmployeeInactive) {
		t.Fatalf("inactive first send: %v", err)
	}
	unchanged, err := svc.GetClinicRequest(ctx, ready.ID)
	if err != nil || unchanged.State != "ready" || unchanged.AttemptedAt != nil {
		t.Fatalf("blocked action mutated intention: %+v %v", unchanged, err)
	}
	input.LocalReference = "sent-review"
	if repeated, err := svc.CreateClinicRequest(ctx, input); err != nil || repeated.ID != sent.ID {
		t.Fatalf("existing intention lost: %+v %v", repeated, err)
	}
	if repeated, err := svc.SubmitClinicRequest(ctx, sent.ID); err != nil || repeated.State != "booked" || calls.Load() != 1 {
		t.Fatalf("already sent replay: %+v calls=%d %v", repeated, calls.Load(), err)
	}
	a.Status, a.Version = "cancelled", 2
	if refreshed, err := svc.RefreshClinicRequest(ctx, sent.ID, ""); err != nil || refreshed.State != "cancelled" {
		t.Fatalf("inactive refresh: %+v %v", refreshed, err)
	}
	rows, total, err := svc.ClinicReport(ctx, "employee", domain.Page{Number: 1, Size: 20})
	if err != nil || total != 2 || len(rows) != 2 || rows[0].Active == nil || *rows[0].Active || rows[0].Request == nil {
		t.Fatalf("inactive history excluded: %+v %d %v", rows, total, err)
	}
	var history int
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM clinic_appointment_history WHERE request_id=$1`, sent.ID).Scan(&history); err != nil || history != 2 {
		t.Fatalf("inactive history=%d %v", history, err)
	}
}

func TestClinicLegacyCandidatesArePreservedAndInaccessible(t *testing.T) {
	pool, _ := isolatedDatabase(t)
	repo := postgres.New(pool)
	ctx := context.Background()
	if err := repo.CreateEmployee(ctx, domain.Employee{ID: "employee", DisplayName: "Persona ficticia", Active: true}); err != nil {
		t.Fatal(err)
	}
	link := domain.PatientLink{PersonKind: "employee", PersonID: "employee", PatientID: "employee-patient", RegisteredBy: "operator"}
	if err := repo.LinkClinicPatient(ctx, link); err != nil {
		t.Fatal(err)
	}
	start := time.Now().UTC().Add(72 * time.Hour).Truncate(time.Second)
	legacy := domain.ClinicRequest{ID: "legacy-request", PersonKind: "candidate", PersonID: "legacy-person", PatientID: "legacy-patient", LocalReference: "legacy-reference", DoctorID: "doctor", StartAt: start, CreatedAt: time.Now().UTC(), RegisteredBy: "operator", State: "ready", Revision: 1}
	payload, err := json.Marshal(legacy)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO candidates(candidate_id,display_name) VALUES('legacy-person','Registro historico')`); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO clinic_patient_links(person_kind,person_id,patient_id,registered_by,candidate_id) VALUES('candidate','legacy-person','legacy-patient','operator','legacy-person')`); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO clinic_review_requests(request_id,local_reference,person_kind,person_id,payload) VALUES($1,$2,'candidate','legacy-person',$3)`, legacy.ID, legacy.LocalReference, payload); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO clinic_appointment_history(request_id,version,payload) VALUES($1,1,$2)`, legacy.ID, []byte(`{"legacy":true}`)); err != nil {
		t.Fatal(err)
	}
	var calls atomic.Int64
	provider := clinicDouble{book: func(context.Context, domain.ClinicRequest) (domain.ClinicAppointment, error) {
		calls.Add(1)
		return domain.ClinicAppointment{}, nil
	}, lookup: func(context.Context, string) (domain.ClinicAppointment, error) {
		calls.Add(1)
		return domain.ClinicAppointment{}, nil
	}}
	svc := application.New(repo, platform.UUID).WithClinic(repo, provider)
	if _, err = svc.GetClinicRequest(ctx, legacy.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("legacy lookup: %v", err)
	}
	if _, err = svc.SubmitClinicRequest(ctx, legacy.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("legacy send: %v", err)
	}
	if _, err = svc.RefreshClinicRequest(ctx, legacy.ID, "appointment"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("legacy refresh: %v", err)
	}
	if _, _, err = repo.ClaimClinicRequest(ctx, legacy.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("legacy claim: %v", err)
	}
	if _, err = repo.FailClinicRequest(ctx, legacy.ID, 1, "FAIL", false, nil); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("legacy mutation: %v", err)
	}
	a := domain.ClinicAppointment{ID: "appointment", PatientID: "legacy-patient", DoctorID: "doctor", StartAt: start, EndAt: start.Add(30 * time.Minute), Status: "booked", Version: 1}
	if _, err = repo.ObserveClinicAppointment(ctx, legacy.ID, a); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("legacy observation: %v", err)
	}
	if _, err = repo.CreateClinicRequest(ctx, legacy); !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("new candidate intention: %v", err)
	}
	if err = repo.LinkClinicPatient(ctx, domain.PatientLink{PersonKind: "candidate", PersonID: "legacy-person", PatientID: "legacy-patient", RegisteredBy: "operator"}); !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("candidate link: %v", err)
	}
	input := legacy
	input.ID, input.PersonKind, input.PersonID = "new-employee-request", "employee", "employee"
	if found, err := svc.CreateClinicRequest(ctx, input); !errors.Is(err, domain.ErrEvidenceConflict) || found.PersonKind != "employee" || found.ID == legacy.ID {
		t.Fatalf("legacy reference exposed: %+v %v", found, err)
	}
	if _, _, err = svc.ClinicReport(ctx, "candidate", domain.Page{Number: 1, Size: 20}); !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("candidate report: %v", err)
	}
	rows, total, err := svc.ClinicReport(ctx, "employee", domain.Page{Number: 1, Size: 20})
	if err != nil || total != 1 || len(rows) != 1 || rows[0].Request != nil {
		t.Fatalf("legacy report leaked: %+v %d %v", rows, total, err)
	}
	if calls.Load() != 0 {
		t.Fatalf("legacy request reached provider: %d", calls.Load())
	}
	if err = repo.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	if err = repo.SeedDemo(ctx); err != nil {
		t.Fatal(err)
	}
	var stored []byte
	if err = pool.QueryRow(ctx, `SELECT payload FROM clinic_review_requests WHERE request_id=$1`, legacy.ID).Scan(&stored); err != nil || string(stored) != string(payload) {
		t.Fatalf("legacy request changed: %s %v", stored, err)
	}
	var history, people, links int
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM clinic_appointment_history WHERE request_id=$1`, legacy.ID).Scan(&history); err != nil || history != 1 {
		t.Fatalf("legacy history=%d %v", history, err)
	}
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM candidates WHERE candidate_id='legacy-person'`).Scan(&people); err != nil || people != 1 {
		t.Fatalf("legacy people=%d %v", people, err)
	}
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM clinic_patient_links WHERE person_kind='candidate'`).Scan(&links); err != nil || links != 1 {
		t.Fatalf("legacy links=%d %v", links, err)
	}
}
