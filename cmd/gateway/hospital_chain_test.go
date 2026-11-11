package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/Gerpeirone/Car-Cruds/internal/platform"
	"github.com/Gerpeirone/Car-Cruds/services/employees/adapters/clinic"
	"github.com/Gerpeirone/Car-Cruds/services/employees/adapters/httpapi"
	"github.com/Gerpeirone/Car-Cruds/services/employees/application"
)

// The operator panel reads doctors and billing through the whole chain:
// gateway -> Empleados -> clinic adapter -> hospital. Only GETs reach the
// hospital, with X-API-Key and never the administrative Bearer.
func TestHospitalReadsCrossGatewayAndEmployees(t *testing.T) {
	const clinicKey = "fixture-clinic-key-not-real"
	var mu sync.Mutex
	var seen []string
	hospital := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		seen = append(seen, r.Method+" "+r.URL.RequestURI())
		mu.Unlock()
		if r.Method != http.MethodGet || r.Header.Get("Authorization") != "" {
			t.Errorf("hospital recibió %s con Authorization=%q", r.Method, r.Header.Get("Authorization"))
		}
		if r.Header.Get("X-API-Key") != clinicKey {
			w.WriteHeader(401)
			_, _ = w.Write([]byte(`{"error":{"code":"UNAUTHORIZED"}}`))
			return
		}
		switch r.URL.RequestURI() {
		case "/v1/doctors":
			_, _ = w.Write([]byte(`{"doctors":[{"id":"doc-001","durationMinutes":30,"specialty":"cardiologia"},{"id":"doc-002","durationMinutes":20}]}`))
		case "/v1/usage?period=2026-11":
			_, _ = w.Write([]byte(`{"period":"2026-11","from":"2026-11-01T00:00:00Z","to":"2026-12-01T00:00:00Z","currentPrice":{"amountCents":2500000,"currency":"ARS"},"totals":[{"currency":"ARS","bookings":1,"amountCents":2500000}],"items":[{"appointmentId":"turn-1","bookedAt":"2026-11-10T22:00:00Z","priceCents":2500000,"currency":"ARS","status":"booked"}]}`))
		default:
			w.WriteHeader(404)
			_, _ = w.Write([]byte(`{"error":{"code":"NOT_FOUND"}}`))
		}
	}))
	defer hospital.Close()

	tokens := map[string]platform.Principal{"admin-fixture": {App: "operador", Role: "admin"}}
	chain := func(key string) http.Handler {
		provider, err := clinic.New(hospital.URL+"/v1", key, true, 2*time.Second)
		if err != nil {
			t.Fatal(err)
		}
		service := application.New(nil, func() string { return "id" })
		service.WithClinic(nil, provider)
		employees := httptest.NewServer(platform.AuthMiddleware(httpapi.New(service), tokens))
		t.Cleanup(employees.Close)
		t.Setenv("EMPLOYEES_URL", employees.URL)
		gateway, _, err := handler()
		if err != nil {
			t.Fatal(err)
		}
		return platform.AuthMiddleware(gateway, tokens)
	}
	get := func(h http.Handler, path string) (int, map[string]any) {
		r := httptest.NewRequest(http.MethodGet, path, nil)
		r.Header.Set("Authorization", "Bearer admin-fixture")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		var body map[string]any
		_ = json.Unmarshal(w.Body.Bytes(), &body)
		return w.Code, body
	}

	h := chain(clinicKey)
	if code, body := get(h, "/v1/employees/clinic/doctors"); code != 200 || len(body["doctors"].([]any)) != 2 {
		t.Fatalf("médicos=%d %v", code, body)
	}
	code, body := get(h, "/v1/employees/clinic/usage?period=2026-11")
	if code != 200 || body["period"] != "2026-11" {
		t.Fatalf("facturación=%d %v", code, body)
	}
	total := body["totals"].([]any)[0].(map[string]any)
	if total["amountMinor"] != float64(2500000) || total["bookings"] != float64(1) || total["currency"] != "ARS" {
		t.Fatalf("total=%v", total)
	}

	// A key rejected by the hospital is reported as unavailable, never as a
	// missing route (404), so the panel cannot confuse both causes.
	h = chain("wrong-key")
	for _, path := range []string{"/v1/employees/clinic/doctors", "/v1/employees/clinic/usage?period=2026-11"} {
		if code, body := get(h, path); code != 503 || body["code"] != "CLINIC_UNAVAILABLE" {
			t.Fatalf("%s con clave rechazada=%d %v", path, code, body)
		}
	}
	mu.Lock()
	defer mu.Unlock()
	want := []string{"GET /v1/doctors", "GET /v1/usage?period=2026-11", "GET /v1/doctors", "GET /v1/usage?period=2026-11"}
	if len(seen) != len(want) {
		t.Fatalf("hospital recibió %v", seen)
	}
	for i := range want {
		if seen[i] != want[i] {
			t.Fatalf("hospital recibió %v", seen)
		}
	}
}
