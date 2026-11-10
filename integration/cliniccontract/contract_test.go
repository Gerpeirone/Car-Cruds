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
	"sync"
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

// Uses the provider's unmodified public gateway, Turnos, MySQL and RabbitMQ.
// The fixture has fictitious keys and price, and never contacts a public clinic.
func TestCarCrudsConsumesUnmodifiedHospitalAppointments(t *testing.T) {
	base := strings.TrimRight(os.Getenv("CLINIC_TEST_BASE_URL"), "/")
	if base == "" {
		t.Skip("requiere el proveedor real aislado: integration/cliniccontract/compose.yaml")
	}
	u, err := url.Parse(base)
	if err != nil || u.Scheme != "http" || (u.Hostname() != "127.0.0.1" && u.Hostname() != "localhost") || u.Path != "/v1" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		t.Fatal("solo se admite la API pública /v1 de la instancia local de prueba")
	}
	key, otherKey := os.Getenv("CLINIC_TEST_API_KEY"), os.Getenv("CLINIC_TEST_OTHER_API_KEY")
	if key == "" || otherKey == "" || key == otherKey || os.Getenv("TEST_DATABASE_URL") == "" {
		t.Fatal("faltan las dos claves ficticias distintas y PostgreSQL propio para el contrato")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
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
		if _, err := admin.Exec(cleanup, "DROP SCHEMA "+quoted+" CASCADE"); err != nil {
			t.Error(err)
		}
	}()
	openPool := func() *pgxpool.Pool {
		t.Helper()
		config, err := pgxpool.ParseConfig(os.Getenv("TEST_DATABASE_URL"))
		if err != nil {
			t.Fatal(err)
		}
		config.ConnConfig.RuntimeParams["search_path"] = schema
		pool, err := pgxpool.NewWithConfig(ctx, config)
		if err != nil {
			t.Fatal(err)
		}
		return pool
	}
	pool := openPool()
	defer pool.Close()
	repo := postgres.New(pool)
	if err = repo.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	client := &http.Client{Timeout: 8 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	decodeCall := func(target, method string, body any, headers map[string]string, status int) (map[string]json.RawMessage, http.Header) {
		t.Helper()
		var data []byte
		if body != nil {
			data, err = json.Marshal(body)
			if err != nil {
				t.Fatal(err)
			}
		}
		r, err := http.NewRequestWithContext(ctx, method, target, bytes.NewReader(data))
		if err != nil {
			t.Fatal(err)
		}
		for name, value := range headers {
			r.Header.Set(name, value)
		}
		if body != nil {
			r.Header.Set("Content-Type", "application/json")
		}
		resp, err := client.Do(r)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		b, err := io.ReadAll(io.LimitReader(resp.Body, 65537))
		if err != nil || len(b) > 65536 {
			t.Fatal("respuesta de contrato incompleta o demasiado grande")
		}
		if resp.StatusCode != status {
			t.Fatalf("%s %s: %d %s", method, r.URL.Path, resp.StatusCode, b)
		}
		out := map[string]json.RawMessage{}
		if len(b) > 0 {
			if err = json.Unmarshal(b, &out); err != nil {
				t.Fatal(err)
			}
		}
		return out, resp.Header
	}
	providerCall := func(method, path, apiKey, idempotencyKey string, body any, status int) (map[string]json.RawMessage, http.Header) {
		t.Helper()
		return decodeCall(base+path, method, body, map[string]string{"X-API-Key": apiKey, "Idempotency-Key": idempotencyKey}, status)
	}
	field := func(m map[string]json.RawMessage, name string) string {
		t.Helper()
		var value string
		if err := json.Unmarshal(m[name], &value); err != nil {
			t.Fatal(err)
		}
		return value
	}
	expectError := func(m map[string]json.RawMessage, code string) {
		t.Helper()
		var envelope struct {
			Code string `json:"code"`
		}
		if err := json.Unmarshal(m["error"], &envelope); err != nil || envelope.Code != code {
			t.Fatalf("se esperaba error %s: %s", code, m)
		}
	}
	badKey, _ := providerCall("GET", "/doctors", "invalid-contract-key", "", nil, 401)
	expectError(badKey, "UNAUTHORIZED")
	// Only PUBLIC_HTTP_ADDR is published, not the legacy portal route.
	legacy, _ := decodeCall(strings.TrimSuffix(base, "/v1")+"/api/turnos/appointments", "POST", map[string]string{}, nil, 404)
	expectError(legacy, "NOT_FOUND")
	doctors, headers := providerCall("GET", "/doctors", key, "", nil, 200)
	var catalog []struct {
		ID string `json:"id"`
	}
	if err = json.Unmarshal(doctors["doctors"], &catalog); err != nil || len(catalog) == 0 || headers.Get("Cache-Control") != "no-store" {
		t.Fatal("catálogo o política de caché incompatibles")
	}
	doctorID := catalog[0].ID
	start := time.Now().UTC().Add(72 * time.Hour).Truncate(time.Second)
	booking := func(at time.Time) map[string]string {
		return map[string]string{"patientId": "pat-001", "doctorId": doctorID, "startAt": at.Format(time.RFC3339)}
	}
	missingIdem, _ := providerCall("POST", "/appointments", key, "", booking(start), 400)
	expectError(missingIdem, "INVALID_IDEMPOTENCY_KEY")
	type usage struct {
		Items []struct {
			ID       string `json:"appointmentId"`
			Price    int64  `json:"priceCents"`
			Currency string `json:"currency"`
			Status   string `json:"status"`
		} `json:"items"`
		Totals []struct {
			Currency string `json:"currency"`
			Bookings int    `json:"bookings"`
			Amount   int64  `json:"amountCents"`
		} `json:"totals"`
	}
	readUsage := func(apiKey string) usage {
		t.Helper()
		m, _ := providerCall("GET", "/usage?period="+time.Now().UTC().Format("2006-01"), apiKey, "", nil, 200)
		b, _ := json.Marshal(m)
		var result usage
		if err := json.Unmarshal(b, &result); err != nil {
			t.Fatal(err)
		}
		return result
	}
	before := readUsage(key)
	provider, err := clinic.New(base, key, true, 2*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	startService := func(repo *postgres.Repository, provider application.Clinic) *httptest.Server {
		service := application.New(repo, platform.UUID).WithClinic(repo, provider)
		return httptest.NewServer(platform.AuthMiddleware(httpapi.New(service), map[string]platform.Principal{"operator": {App: "contract-test", Role: "admin"}}))
	}
	server := startService(repo, provider)
	defer server.Close()
	call := func(method, path string, body any, status int) map[string]json.RawMessage {
		t.Helper()
		m, _ := decodeCall(server.URL+path, method, body, map[string]string{"Authorization": "Bearer operator"}, status)
		return m
	}
	employee := call("POST", "/v1/employees", map[string]any{"displayName": "Empleado ficticio del contrato", "active": true}, 201)
	personID := field(employee, "employeeId")
	call("POST", "/v1/employees/clinic/patient-links", map[string]any{"personKind": "employee", "personId": personID, "patientId": "pat-001"}, 204)
	create := func(at time.Time) string {
		r := call("POST", "/v1/employees/clinic/requests", map[string]any{"personKind": "employee", "personId": personID, "localReference": platform.UUID(), "doctorId": doctorID, "startAt": at.Format(time.RFC3339)}, 200)
		return field(r, "requestId")
	}
	requestID := create(start)
	first := call("POST", "/v1/employees/clinic/requests/"+requestID+"/submit", nil, 200)
	if field(first, "state") != "booked" {
		t.Fatalf("proveedor sin confirmación: %s", first)
	}
	repeated := call("POST", "/v1/employees/clinic/requests/"+requestID+"/submit", nil, 200)
	if field(repeated, "state") != "booked" || !bytes.Equal(first["lastKnownAppointment"], repeated["lastKnownAppointment"]) {
		t.Fatal("la repetición local cambió el turno confirmado")
	}
	type appointment struct {
		ID      string `json:"id"`
		Status  string `json:"status"`
		Version int64  `json:"version"`
	}
	var a appointment
	if err = json.Unmarshal(first["lastKnownAppointment"], &a); err != nil || a.ID == "" || a.Status != "booked" || a.Version != 1 {
		t.Fatalf("respuesta del proveedor incompatible: %+v", a)
	}
	foreign, _ := providerCall("GET", "/appointments/"+url.PathEscape(a.ID), otherKey, "", nil, 404)
	expectError(foreign, "NOT_FOUND")
	stored, err := repo.GetClinicRequest(ctx, requestID)
	if err != nil || stored.ProviderContract != "turnos-partner-v1" || stored.IdempotencyKey != requestID {
		t.Fatal("la intención no conserva contrato y clave antes del envío")
	}
	conflict, _ := providerCall("POST", "/appointments", key, stored.IdempotencyKey, booking(start.Add(time.Hour)), 409)
	expectError(conflict, "IDEMPOTENCY_CONFLICT")
	// Cancellation is a fixture action, not a new Car Cruds capability.
	providerCall("POST", "/appointments/"+url.PathEscape(a.ID)+"/cancel", key, "", nil, 200)
	refreshed := call("POST", "/v1/employees/clinic/requests/"+requestID+"/refresh", nil, 200)
	if field(refreshed, "state") != "cancelled" {
		t.Fatalf("cancelación no reflejada: %s", refreshed)
	}
	call("GET", "/v1/employees/reports/clinic?personKind=employee", nil, 200)

	// Hide a committed 201, then recover the same booking after a restart.
	var mu sync.Mutex
	var sentKeys []string
	var sentBodies [][]byte
	var recoveredID string
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Error(err)
			w.WriteHeader(503)
			return
		}
		upstream, err := http.NewRequestWithContext(r.Context(), r.Method, strings.TrimSuffix(base, "/v1")+r.URL.RequestURI(), bytes.NewReader(body))
		if err != nil {
			t.Error(err)
			w.WriteHeader(503)
			return
		}
		upstream.Header = r.Header.Clone()
		response, err := client.Do(upstream)
		if err != nil {
			t.Error(err)
			w.WriteHeader(503)
			return
		}
		defer response.Body.Close()
		b, err := io.ReadAll(response.Body)
		if err != nil {
			t.Error(err)
			w.WriteHeader(503)
			return
		}
		if r.Method == "POST" {
			mu.Lock()
			sentKeys = append(sentKeys, r.Header.Get("Idempotency-Key"))
			sentBodies = append(sentBodies, body)
			first := len(sentKeys) == 1
			var created appointment
			if err := json.Unmarshal(b, &created); err != nil || created.ID == "" || response.StatusCode != 201 {
				t.Errorf("el POST real no creó el turno: estado=%d cuerpo=%s", response.StatusCode, b)
			}
			recoveredID = created.ID
			mu.Unlock()
			if first {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(503)
				io.WriteString(w, `{"error":{"code":"UNAVAILABLE","message":"respuesta confirmada perdida en laboratorio"}}`)
				return
			}
			if response.Header.Get("Idempotency-Replayed") != "true" {
				t.Error("la recuperación no fue una repetición idempotente")
			}
		}
		for name, values := range response.Header {
			w.Header()[name] = values
		}
		w.WriteHeader(response.StatusCode)
		w.Write(b)
	}))
	defer proxy.Close()
	lossy, err := clinic.New(proxy.URL+"/v1", key, true, 2*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	server.Close()
	server = startService(repo, lossy)
	defer server.Close()
	lostID := create(start.Add(2 * time.Hour))
	unknown := call("POST", "/v1/employees/clinic/requests/"+lostID+"/submit", nil, 200)
	if field(unknown, "state") != "unknown" {
		t.Fatalf("respuesta perdida mal clasificada: %s", unknown)
	}
	persisted, err := repo.GetClinicRequest(ctx, lostID)
	if err != nil || persisted.IdempotencyKey != lostID || persisted.NextAttemptAt == nil {
		t.Fatal("la recuperación no conserva clave y espera durable")
	}
	// A new service and SQL pool only retain the durable intention.
	server.Close()
	pool.Close()
	pool = openPool()
	defer pool.Close()
	repo = postgres.New(pool)
	if err = repo.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	restarted, err := clinic.New(proxy.URL+"/v1", key, true, 2*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	server = startService(repo, restarted)
	defer server.Close()
	call("POST", "/v1/employees/clinic/requests/"+lostID+"/submit", nil, 200)
	mu.Lock()
	count := len(sentKeys)
	mu.Unlock()
	if count != 1 {
		t.Fatal("un submit durante la espera volvió a enviar el POST")
	}
	wait := time.Until(*persisted.NextAttemptAt) + 100*time.Millisecond
	if wait > 12*time.Second {
		t.Fatal("espera de recuperación inesperada")
	}
	if wait > 0 {
		timer := time.NewTimer(wait)
		defer timer.Stop()
		select {
		case <-timer.C:
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		}
	}
	recovered := call("POST", "/v1/employees/clinic/requests/"+lostID+"/submit", nil, 200)
	var recoveredAppointment appointment
	if err = json.Unmarshal(recovered["lastKnownAppointment"], &recoveredAppointment); err != nil || field(recovered, "state") != "booked" {
		t.Fatalf("no se recuperó el turno real: %s", recovered)
	}
	mu.Lock()
	if len(sentKeys) != 2 || sentKeys[0] != lostID || sentKeys[1] != lostID || !bytes.Equal(sentBodies[0], sentBodies[1]) || recoveredAppointment.ID != recoveredID {
		mu.Unlock()
		t.Fatal("la recuperación cambió clave, cuerpo o identificador del turno")
	}
	mu.Unlock()
	final := readUsage(key)
	if len(final.Items) != len(before.Items)+2 {
		t.Fatalf("se esperaban solo dos cargos nuevos: antes=%d después=%d", len(before.Items), len(final.Items))
	}
	arsTotal := func(u usage) (int, int64) {
		for _, total := range u.Totals {
			if total.Currency == "ARS" {
				return total.Bookings, total.Amount
			}
		}
		return 0, 0
	}
	beforeCount, beforeAmount := arsTotal(before)
	finalCount, finalAmount := arsTotal(final)
	if finalCount-beforeCount != 2 || finalAmount-beforeAmount != 2*12345 {
		t.Fatal("el total facturado no corresponde a solo dos reservas nuevas")
	}
	found := map[string]bool{}
	for _, item := range final.Items {
		if item.ID == a.ID || item.ID == recoveredID {
			if found[item.ID] || item.Price != 12345 || item.Currency != "ARS" {
				t.Fatal("cargo duplicado o incompatible con el precio ficticio de la prueba")
			}
			if item.ID == a.ID && item.Status != "cancelled" {
				t.Fatal("el estado de cuenta no conserva el cargo del turno cancelado")
			}
			found[item.ID] = true
		}
	}
	if len(found) != 2 {
		t.Fatal("faltan los cargos de los dos turnos verificados")
	}
	// Leave no active slot in the disposable provider, so the fixture can repeat.
	providerCall("POST", "/appointments/"+url.PathEscape(recoveredID)+"/cancel", key, "", nil, 200)
	if other := readUsage(otherKey); len(other.Items) != 0 || len(other.Totals) != 0 {
		t.Fatal("el segundo consumidor accedió a cargos ajenos")
	}
}
