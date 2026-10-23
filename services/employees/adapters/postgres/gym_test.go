package postgres_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Gerpeirone/Car-Cruds/internal/platform"
	"github.com/Gerpeirone/Car-Cruds/services/employees/adapters/gym"
	"github.com/Gerpeirone/Car-Cruds/services/employees/adapters/httpapi"
	"github.com/Gerpeirone/Car-Cruds/services/employees/adapters/postgres"
	"github.com/Gerpeirone/Car-Cruds/services/employees/application"
	"github.com/Gerpeirone/Car-Cruds/services/employees/domain"
)

func TestPostgresGymLostResponseDurableReplayAndPrivateHTTPProjection(t *testing.T) {
	pool, _ := isolatedDatabase(t)
	repo := postgres.New(pool)
	ctx := context.Background()
	if err := repo.CreateEmployee(ctx, domain.Employee{ID: "gym-fixture", DisplayName: "Persona Ficticia", Active: true}); err != nil {
		t.Fatal(err)
	}
	var calls atomic.Int32
	var providerKey atomic.Value
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]string
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		probe, cancel := context.WithTimeout(ctx, time.Second)
		defer cancel()
		var savedID string
		if err := pool.QueryRow(probe, `SELECT enrollment_id FROM gym_enrollments WHERE employee_id='gym-fixture'`).Scan(&savedID); err != nil {
			t.Error("intention not persisted before provider call", err)
		}
		if r.Header.Get("X-API-Key") != "private-fixture-key" || r.Header.Get("Idempotency-Key") != "carcruds-"+savedID {
			t.Error("wrong credential or stable provider key")
		}
		if body["nombre"] != "Persona Ficticia" || body["email"] != "persona@example.invalid" {
			t.Error("retry changed snapshot")
		}
		if calls.Add(1) == 1 {
			providerKey.Store(r.Header.Get("Idempotency-Key"))
			connection, _, err := w.(http.Hijacker).Hijack()
			if err != nil {
				t.Error(err)
				return
			}
			_ = connection.Close()
			return
		}
		if providerKey.Load() != r.Header.Get("Idempotency-Key") {
			t.Error("provider key changed after lost response")
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(201)
		fmt.Fprint(w, `{"socioId":"socio-fixture","solicitudId":"request-fixture","estadoSolicitud":"PENDIENTE","membresiaHabilitada":false}`)
	}))
	defer provider.Close()
	client, err := gym.New(provider.URL, "private-fixture-key", false)
	if err != nil {
		t.Fatal(err)
	}
	makeHandler := func() http.Handler {
		return platform.AuthMiddleware(httpapi.New(application.New(postgres.New(pool), platform.UUID).WithGym(postgres.New(pool), client)), map[string]platform.Principal{"admin-fixture": {App: "operator", Role: "admin"}, "gym-fixture": {App: "gym", Role: "gym"}})
	}
	request := func(handler http.Handler, method, path, body, key, token string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Authorization", "Bearer "+token)
		if key != "" {
			r.Header.Set("Idempotency-Key", key)
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w
	}
	path := "/v1/employees/gym-fixture/gym-enrollments"
	body := `{"dni":"99900001","email":"persona@example.invalid"}`
	first := request(makeHandler(), "POST", path, body, "same-key", "admin-fixture")
	if first.Code != 503 || !strings.Contains(first.Body.String(), `"UNKNOWN"`) {
		t.Fatal("lost response must remain unknown", first.Code, first.Body.String())
	}
	if _, err := repo.UpdateEmployee(ctx, domain.Employee{ID: "gym-fixture", DisplayName: "Nombre posterior", Active: false}); err != nil {
		t.Fatal(err)
	}
	// Recreate application/repository to demonstrate durable retry after restart.
	replay := request(makeHandler(), "POST", path, body, "same-key", "admin-fixture")
	if replay.Code != 201 || !strings.Contains(replay.Body.String(), `"membershipEnabled":false`) {
		t.Fatal(replay.Code, replay.Body.String())
	}
	for _, private := range []string{"99900001", "persona@example.invalid", "Persona Ficticia", "private-fixture-key"} {
		if strings.Contains(replay.Body.String(), private) {
			t.Fatal("private data exposed")
		}
	}
	third := request(makeHandler(), "POST", path, body, "same-key", "admin-fixture")
	if third.Code != 201 || third.Body.String() != replay.Body.String() || calls.Load() != 2 {
		t.Fatal("terminal replay sent or changed", calls.Load())
	}
	changed := request(makeHandler(), "POST", path, strings.Replace(body, "99900001", "99900002", 1), "same-key", "admin-fixture")
	if changed.Code != 409 {
		t.Fatal("changed payload reused key")
	}
	denied := request(makeHandler(), "POST", path, body, "same-key", "gym-fixture")
	if denied.Code != 403 {
		t.Fatal("gym could administer enrollments")
	}
	location := replay.Header().Get("Location")
	if got := request(makeHandler(), "GET", location, "", "", "admin-fixture"); got.Code != 200 {
		t.Fatal("cannot recover enrollment")
	}
	if got := request(makeHandler(), "GET", location, "", "", "gym-fixture"); got.Code != 403 {
		t.Fatal("private enrollment readable by gym")
	}
}

func TestPostgresGymSerializesKeysLeasesAndEmployeeIntentions(t *testing.T) {
	pool, _ := isolatedDatabase(t)
	repo := postgres.New(pool)
	ctx := context.Background()
	if err := repo.CreateEmployee(ctx, domain.Employee{ID: "gym-concurrent", DisplayName: "Persona Ficticia", Active: true}); err != nil {
		t.Fatal(err)
	}
	const n = 12
	results := make(chan domain.GymEnrollment, n)
	failures := make(chan error, n)
	for i := 0; i < n; i++ {
		go func() {
			e, err := repo.PrepareGym(ctx, "operator", "same-key", "gym-concurrent", "99900001", "persona@example.invalid", platform.UUID())
			if err != nil {
				failures <- err
			} else {
				results <- e
			}
		}()
	}
	id := ""
	for i := 0; i < n; i++ {
		select {
		case err := <-failures:
			t.Fatal(err)
		case e := <-results:
			if id != "" && id != e.ID {
				t.Fatal("duplicate intention")
			}
			id = e.ID
		}
	}
	if _, err := repo.PrepareGym(ctx, "other-app", "new-key", "gym-concurrent", "99900001", "persona@example.invalid", platform.UUID()); err != domain.ErrGymConflict {
		t.Fatal("new key bypassed unresolved enrollment", err)
	}
	claimed, err := repo.ClaimGym(ctx, id, "lease-one")
	if err != nil || !claimed {
		t.Fatal(claimed, err)
	}
	if other, err := repo.ClaimGym(ctx, id, "lease-two"); err != nil || other {
		t.Fatal("concurrent lease admitted")
	}
	if _, err := repo.CompleteGym(ctx, id, "stale-lease", domain.GymOutcome{State: "REJECTED", ProviderCode: "DNI_EXISTENTE"}); err != nil {
		t.Fatal(err)
	}
	e, err := repo.GetGym(ctx, "gym-concurrent", id)
	if err != nil || e.State != "PENDING" {
		t.Fatal("stale completion overwrote current lease")
	}
	if _, err := pool.Exec(ctx, `UPDATE gym_enrollments SET lease_until=now()-interval '1 second' WHERE enrollment_id=$1`, id); err != nil {
		t.Fatal(err)
	}
	if claimed, err := repo.ClaimGym(ctx, id, "lease-recovery"); err != nil || !claimed {
		t.Fatal("crashed sender not recoverable")
	}
	if _, err := repo.CompleteGym(ctx, id, "lease-recovery", domain.GymOutcome{State: "ACCEPTED", SocioID: "socio", SolicitudID: "request"}); err != nil {
		t.Fatal(err)
	}
}
