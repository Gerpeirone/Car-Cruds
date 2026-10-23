package cliniccontract_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Gerpeirone/Car-Cruds/internal/platform"
	"github.com/Gerpeirone/Car-Cruds/services/employees/adapters/clinic"
	"github.com/Gerpeirone/Car-Cruds/services/employees/adapters/httpapi"
	"github.com/Gerpeirone/Car-Cruds/services/employees/adapters/postgres"
	"github.com/Gerpeirone/Car-Cruds/services/employees/application"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestCarCrudsConsumesUnmodifiedHospitalAppointments(t *testing.T) {
	base := os.Getenv("CLINIC_TEST_BASE_URL")
	if base == "" {
		t.Skip("requiere el proveedor real aislado: integration/cliniccontract/compose.yaml")
	}
	u, err := url.Parse(base)
	if err != nil || u.Scheme != "http" || (u.Hostname() != "127.0.0.1" && u.Hostname() != "localhost") {
		t.Fatal("solo se admite la instancia local de prueba, nunca el hospital público")
	}
	if os.Getenv("TEST_DATABASE_URL") == "" {
		t.Fatal("falta PostgreSQL propio para el test de contrato")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	admin, err := pgxpool.New(ctx, os.Getenv("TEST_DATABASE_URL"))
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	schema := "clinic_contract_" + strings.ReplaceAll(platform.UUID(), "-", "")
	quoted := pgx.Identifier{schema}.Sanitize()
	if _, err = admin.Exec(ctx, "CREATE SCHEMA "+quoted); err != nil {
		t.Fatal(err)
	}
	defer func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_, err := admin.Exec(cleanup, "DROP SCHEMA "+quoted+" CASCADE")
		if err != nil {
			t.Error(err)
		}
	}()
	config, err := pgxpool.ParseConfig(os.Getenv("TEST_DATABASE_URL"))
	if err != nil {
		t.Fatal(err)
	}
	config.ConnConfig.RuntimeParams["search_path"] = schema
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	repo := postgres.New(pool)
	if err = repo.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	provider, err := clinic.New(base, true, 2*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	service := application.New(repo, platform.UUID).WithClinic(repo, provider)
	server := httptest.NewServer(platform.AuthMiddleware(httpapi.New(service), map[string]platform.Principal{"operator": {App: "contract-test", Role: "admin"}}))
	defer server.Close()
	client := &http.Client{Timeout: 8 * time.Second}
	call := func(method, path string, body any, status int) map[string]json.RawMessage {
		t.Helper()
		var data []byte
		if body != nil {
			data, err = json.Marshal(body)
			if err != nil {
				t.Fatal(err)
			}
		}
		r, err := http.NewRequestWithContext(ctx, method, server.URL+path, bytes.NewReader(data))
		if err != nil {
			t.Fatal(err)
		}
		r.Header.Set("Authorization", "Bearer operator")
		if body != nil {
			r.Header.Set("Content-Type", "application/json")
		}
		resp, err := client.Do(r)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		b, err := io.ReadAll(io.LimitReader(resp.Body, 65536))
		if err != nil {
			t.Fatal(err)
		}
		if resp.StatusCode != status {
			t.Fatalf("%s %s: %d %s", method, path, resp.StatusCode, b)
		}
		out := map[string]json.RawMessage{}
		if len(b) > 0 {
			if err = json.Unmarshal(b, &out); err != nil {
				t.Fatal(err)
			}
		}
		return out
	}
	field := func(m map[string]json.RawMessage, key string) string {
		t.Helper()
		var value string
		if err := json.Unmarshal(m[key], &value); err != nil {
			t.Fatal(err)
		}
		return value
	}
	employee := call("POST", "/v1/employees", map[string]any{"displayName": "Empleado ficticio del contrato", "active": true}, 201)
	personID := field(employee, "employeeId")
	call("POST", "/v1/employees/clinic/patient-links", map[string]any{"personKind": "employee", "personId": personID, "patientId": "pat-001"}, 204)
	start := time.Now().UTC().Add(72 * time.Hour).Truncate(time.Second)
	create := func(at time.Time) string {
		r := call("POST", "/v1/employees/clinic/requests", map[string]any{"personKind": "employee", "personId": personID, "localReference": platform.UUID(), "doctorId": "doc-001", "startAt": at.Format(time.RFC3339)}, 200)
		return field(r, "requestId")
	}
	requestID := create(start)
	first := call("POST", "/v1/employees/clinic/requests/"+requestID+"/submit", nil, 200)
	if field(first, "state") != "booked" {
		t.Fatalf("provider did not confirm: %s", first)
	}
	repeated := call("POST", "/v1/employees/clinic/requests/"+requestID+"/submit", nil, 200)
	if field(repeated, "state") != "booked" || !bytes.Equal(first["lastKnownAppointment"], repeated["lastKnownAppointment"]) {
		t.Fatal("repeat changed the confirmed booking")
	}
	var a struct {
		ID      string `json:"id"`
		Status  string `json:"status"`
		Version int64  `json:"version"`
	}
	if err = json.Unmarshal(first["lastKnownAppointment"], &a); err != nil {
		t.Fatal(err)
	}
	if a.ID == "" || a.Status != "booked" || a.Version != 1 {
		t.Fatalf("incompatible provider response: %+v", a)
	}
	// The hospital operator changes its own appointment in the disposable fixture.
	r, err := http.NewRequestWithContext(ctx, "POST", base+"/appointments/"+url.PathEscape(a.ID)+"/cancel", strings.NewReader(`{"reason":"contract_test"}`))
	if err != nil {
		t.Fatal(err)
	}
	r.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(r)
	if err != nil {
		t.Fatal(err)
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("hospital cancellation=%d", resp.StatusCode)
	}
	refreshed := call("POST", "/v1/employees/clinic/requests/"+requestID+"/refresh", nil, 200)
	if field(refreshed, "state") != "cancelled" {
		t.Fatalf("cancellation not reflected: %s", refreshed)
	}
	call("GET", "/v1/employees/reports/clinic?personKind=employee", nil, 200)
	// Forward the POST to the real provider, then lose its successful response.
	// This tests the case that a second POST would duplicate a business effect.
	recoveryIDs := make(chan string, 2)
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstream, err := http.NewRequestWithContext(r.Context(), r.Method, base+r.URL.Path, r.Body)
		if err != nil {
			t.Error(err)
			w.WriteHeader(503)
			return
		}
		upstream.Header.Set("Content-Type", "application/json")
		response, err := client.Do(upstream)
		if err != nil {
			t.Error(err)
			w.WriteHeader(503)
			return
		}
		defer response.Body.Close()
		if r.Method == "POST" {
			var created struct {
				ID string `json:"id"`
			}
			if err := json.NewDecoder(response.Body).Decode(&created); err != nil {
				t.Error(err)
			}
			recoveryIDs <- created.ID
			w.WriteHeader(503)
			return
		}
		w.WriteHeader(response.StatusCode)
		io.Copy(w, response.Body)
	}))
	defer proxy.Close()
	lossy, _ := clinic.New(proxy.URL, true, 2*time.Second)
	server.Close()
	service = application.New(repo, platform.UUID).WithClinic(repo, lossy)
	server = httptest.NewServer(platform.AuthMiddleware(httpapi.New(service), map[string]platform.Principal{"operator": {App: "contract-test", Role: "admin"}}))
	defer server.Close()
	lostID := create(start.Add(time.Hour))
	unknown := call("POST", "/v1/employees/clinic/requests/"+lostID+"/submit", nil, 200)
	var recoveryID string
	select {
	case recoveryID = <-recoveryIDs:
	case <-time.After(5 * time.Second):
		t.Fatal("el proveedor no devolvió un ID para conciliar")
	}
	if field(unknown, "state") != "unknown" || recoveryID == "" {
		t.Fatalf("lost response was misclassified: %s", unknown)
	}
	replayed := call("POST", "/v1/employees/clinic/requests/"+lostID+"/submit", nil, 200)
	if field(replayed, "state") != "unknown" || len(recoveryIDs) != 0 {
		t.Fatal("uncertain creation was sent again")
	}
	recovered := call("POST", "/v1/employees/clinic/requests/"+lostID+"/refresh", map[string]string{"appointmentId": recoveryID}, 200)
	if field(recovered, "state") != "booked" {
		t.Fatalf("could not reconcile actual booking: %s", recovered)
	}
}
