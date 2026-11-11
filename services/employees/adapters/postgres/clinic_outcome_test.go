package postgres_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Gerpeirone/Car-Cruds/internal/platform"
	"github.com/Gerpeirone/Car-Cruds/services/employees/adapters/clinic"
	"github.com/Gerpeirone/Car-Cruds/services/employees/adapters/postgres"
	"github.com/Gerpeirone/Car-Cruds/services/employees/application"
	"github.com/Gerpeirone/Car-Cruds/services/employees/domain"
)

// Real service, repository and HTTP adapter against a simulated hospital:
// what the operator panel sees after each kind of answer, and whether a
// later action may POST again.
func TestClinicOutcomesThroughTheRealAdapter(t *testing.T) {
	pool, _ := isolatedDatabase(t)
	repo := postgres.New(pool)
	ctx := context.Background()
	var posts, gets atomic.Int64
	var mode atomic.Value
	hospital := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			gets.Add(1)
		} else {
			posts.Add(1)
		}
		switch mode.Load() {
		case "outside":
			w.WriteHeader(422)
			_, _ = w.Write([]byte(`{"error":{"code":"OUTSIDE_SCHEDULE","message":"El horario está fuera de la agenda del médico."}}`))
		case "directory":
			w.WriteHeader(503)
			_, _ = w.Write([]byte(`{"error":{"code":"DIRECTORY_UNAVAILABLE"}}`))
		case "timeout":
			select {
			case <-time.After(time.Second):
			case <-r.Context().Done():
			}
		}
	}))
	defer hospital.Close()
	provider, err := clinic.New(hospital.URL+"/v1", "fixture-clinic-key-not-real", true, 150*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	svc := application.New(repo, platform.UUID).WithClinic(repo, provider)
	start := time.Now().UTC().Add(72 * time.Hour).Truncate(time.Second)
	request := func(person string) domain.ClinicRequest {
		t.Helper()
		if err := repo.CreateEmployee(ctx, domain.Employee{ID: person, DisplayName: "Persona ficticia", Active: true}); err != nil {
			t.Fatal(err)
		}
		if err := repo.LinkClinicPatient(ctx, domain.PatientLink{PersonKind: "employee", PersonID: person, PatientID: "patient-" + person, RegisteredBy: "operator"}); err != nil {
			t.Fatal(err)
		}
		r, err := svc.CreateClinicRequest(ctx, domain.ClinicRequest{PersonKind: "employee", PersonID: person, LocalReference: "ref-" + person, DoctorID: "doc-001", StartAt: start, RegisteredBy: "operator"})
		if err != nil {
			t.Fatal(err)
		}
		return r
	}

	t.Run("fuera de agenda: rechazo definitivo sin reintento", func(t *testing.T) {
		mode.Store("outside")
		posts.Store(0)
		gets.Store(0)
		r := request("employee-outside")
		r, err := svc.SubmitClinicRequest(ctx, r.ID)
		if err != nil || r.State != "rejected" || r.FailureCode != "OUTSIDE_SCHEDULE" || r.NextAttemptAt != nil || r.Classification() != "without_confirmed_appointment" || r.Appointment != nil {
			t.Fatalf("solicitud=%+v err=%v", r, err)
		}
		again, err := svc.SubmitClinicRequest(ctx, r.ID)
		if err != nil || again.State != "rejected" || posts.Load() != 1 {
			t.Fatalf("reenvío=%+v posts=%d err=%v", again, posts.Load(), err)
		}
		if _, err := svc.RefreshClinicRequest(ctx, r.ID, ""); err == nil || gets.Load() != 0 {
			t.Fatalf("consulta sin turno=%v gets=%d", err, gets.Load())
		}
		rows, _, err := svc.ClinicReport(ctx, "employee", domain.Page{Number: 1, Size: 50})
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for _, row := range rows {
			if row.PersonID == "employee-outside" {
				found = row.Request != nil && row.Request.Classification() == "without_confirmed_appointment"
			}
		}
		if !found {
			t.Fatalf("informe=%+v", rows)
		}
	})

	for _, tc := range []struct{ name, mode, person string }{
		{"directorio del hospital caído (503)", "directory", "employee-directory"},
		{"timeout", "timeout", "employee-timeout"},
	} {
		t.Run(tc.name+": sigue incierto y no se reenvía solo", func(t *testing.T) {
			mode.Store(tc.mode)
			posts.Store(0)
			r := request(tc.person)
			before := time.Now().UTC()
			r, err := svc.SubmitClinicRequest(ctx, r.ID)
			if err != nil || r.State != "unknown" || r.FailureCode != "CLINIC_UNAVAILABLE" || r.Classification() != "pending_verification" {
				t.Fatalf("solicitud=%+v err=%v", r, err)
			}
			if r.NextAttemptAt == nil || r.NextAttemptAt.Before(before) || r.NextAttemptAt.After(before.Add(11*time.Second)) {
				t.Fatalf("nextAttemptAt=%v", r.NextAttemptAt)
			}
			// Within the cooldown an explicit submit returns the stored state
			// without a second POST; nothing retries in the background.
			again, err := svc.SubmitClinicRequest(ctx, r.ID)
			if err != nil || again.State != "unknown" || posts.Load() != 1 {
				t.Fatalf("reenvío=%+v posts=%d err=%v", again, posts.Load(), err)
			}
			time.Sleep(300 * time.Millisecond)
			if posts.Load() != 1 {
				t.Fatalf("reintento automático: posts=%d", posts.Load())
			}
		})
	}
}
