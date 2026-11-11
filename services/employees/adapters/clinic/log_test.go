package clinic_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Gerpeirone/Car-Cruds/services/employees/adapters/clinic"
	"github.com/Gerpeirone/Car-Cruds/services/employees/domain"
)

// captureLogs routes slog to a buffer for one test and returns the decoded
// records plus the raw text, so tests can also search for leaked secrets.
func captureLogs(t *testing.T) func() ([]map[string]any, string) {
	t.Helper()
	var buffer bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buffer, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(previous) })
	return func() ([]map[string]any, string) {
		var records []map[string]any
		for _, line := range strings.Split(strings.TrimSpace(buffer.String()), "\n") {
			if line == "" {
				continue
			}
			var record map[string]any
			if err := json.Unmarshal([]byte(line), &record); err != nil {
				t.Fatalf("log no estructurado: %q", line)
			}
			records = append(records, record)
		}
		return records, buffer.String()
	}
}

// assertNoSecrets fails if the API key, the patient or the provider's
// free-text message reached the log.
func assertNoSecrets(t *testing.T, raw string) {
	t.Helper()
	for _, secret := range []string{clinicTestKey, "X-API-Key", "x-api-key", "pat-001", "texto-libre-del-proveedor"} {
		if strings.Contains(raw, secret) {
			t.Fatalf("el log contiene %q: %s", secret, raw)
		}
	}
}

func onlyClinicRecord(t *testing.T, records []map[string]any) map[string]any {
	t.Helper()
	var found []map[string]any
	for _, r := range records {
		if r["event"] == "clinic.call" {
			found = append(found, r)
		}
	}
	if len(found) != 1 {
		t.Fatalf("se esperaba un registro clinic.call: %v", records)
	}
	return found[0]
}

func TestOutsideScheduleIsAFinalRejectionAndIsLogged(t *testing.T) {
	logs := captureLogs(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(422)
		// A provider message echoing the key must never be logged.
		_, _ = w.Write([]byte(`{"error":{"code":"OUTSIDE_SCHEDULE","message":"texto-libre-del-proveedor ` + clinicTestKey + `"}}`))
	}))
	defer server.Close()
	client, _ := clinic.New(server.URL+"/v1", clinicTestKey, true, time.Second)
	_, err := client.Book(context.Background(), bookingInput())
	var rejection *domain.ClinicRejection
	if !errors.As(err, &rejection) || rejection.Code != "OUTSIDE_SCHEDULE" {
		t.Fatalf("err=%v", err)
	}
	records, raw := logs()
	record := onlyClinicRecord(t, records)
	want := map[string]any{"level": "WARN", "operation": "book", "request_id": "intention-1", "http_status": float64(422), "provider_code": "OUTSIDE_SCHEDULE", "outcome": "rejected"}
	for key, value := range want {
		if record[key] != value {
			t.Fatalf("%s=%v, registro=%v", key, record[key], record)
		}
	}
	assertNoSecrets(t, raw)
}

// Everything that is not a documented answer stays uncertain (the caller keeps
// CLINIC_UNAVAILABLE and only an explicit action may retry), and the log says
// why without exposing credentials.
func TestUncertainOutcomesStayUnavailableAndExplainTheCause(t *testing.T) {
	for _, tc := range []struct {
		name, body, code, cause string
		status, sleepMs         int
		wantStatus              float64
	}{
		{name: "directorio del hospital caído", status: 503, body: `{"error":{"code":"DIRECTORY_UNAVAILABLE","message":"texto-libre-del-proveedor"}}`, code: "DIRECTORY_UNAVAILABLE", cause: "unexpected_status", wantStatus: 503},
		{name: "error interno", status: 500, body: `{"error":{"code":"INTERNAL"}}`, code: "INTERNAL", cause: "unexpected_status", wantStatus: 500},
		{name: "proxy sin JSON", status: 502, body: `<html>bad gateway</html>`, cause: "unexpected_status", wantStatus: 502},
		{name: "agenda con estado inesperado", status: 409, body: `{"error":{"code":"OUTSIDE_SCHEDULE"}}`, code: "OUTSIDE_SCHEDULE", cause: "unexpected_status", wantStatus: 409},
		{name: "código con forma de credencial", status: 418, body: `{"error":{"code":"` + clinicTestKey + `"}}`, code: "UNRECOGNIZED", cause: "unexpected_status", wantStatus: 418},
		{name: "201 ilegible", status: 201, body: `{"id":`, cause: "invalid_body", wantStatus: 201},
		{name: "timeout", status: 201, body: appointment, sleepMs: 400, cause: "timeout", wantStatus: 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			logs := captureLogs(t)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if tc.sleepMs > 0 {
					select {
					case <-time.After(time.Duration(tc.sleepMs) * time.Millisecond):
					case <-r.Context().Done():
						return
					}
				}
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer server.Close()
			client, _ := clinic.New(server.URL+"/v1", clinicTestKey, true, 150*time.Millisecond)
			_, err := client.Book(context.Background(), bookingInput())
			var rejection *domain.ClinicRejection
			if !errors.Is(err, domain.ErrClinicUnavailable) || errors.As(err, &rejection) {
				t.Fatalf("err=%v", err)
			}
			records, raw := logs()
			record := onlyClinicRecord(t, records)
			if record["outcome"] != "unavailable" || record["result_code"] != "CLINIC_UNAVAILABLE" || record["cause"] != tc.cause || record["http_status"] != tc.wantStatus || record["request_id"] != "intention-1" {
				t.Fatalf("registro=%v", record)
			}
			if tc.code == "" && record["provider_code"] != nil || tc.code != "" && record["provider_code"] != tc.code {
				t.Fatalf("provider_code=%v", record["provider_code"])
			}
			assertNoSecrets(t, raw)
		})
	}
}

func TestUnreachableHospitalIsLoggedAsTransport(t *testing.T) {
	logs := captureLogs(t)
	server := httptest.NewServer(http.NotFoundHandler())
	base := server.URL
	server.Close()
	client, _ := clinic.New(base+"/v1", clinicTestKey, true, time.Second)
	if _, err := client.Book(context.Background(), bookingInput()); !errors.Is(err, domain.ErrClinicUnavailable) {
		t.Fatalf("err=%v", err)
	}
	records, raw := logs()
	if record := onlyClinicRecord(t, records); record["cause"] != "transport" || record["http_status"] != float64(0) {
		t.Fatalf("registro=%v", record)
	}
	assertNoSecrets(t, raw)
}

func TestKnownFailuresAndReadsAreLoggedWithoutSecrets(t *testing.T) {
	for _, tc := range []struct {
		name, path, body string
		status           int
		call             func(*clinic.Client) error
		want             map[string]any
	}{
		{"clave rechazada", "/v1/appointments", `{"error":{"code":"UNAUTHORIZED"}}`, 401,
			func(c *clinic.Client) error { _, err := c.Book(context.Background(), bookingInput()); return err },
			map[string]any{"operation": "book", "outcome": "failure", "result_code": "CLINIC_UNAUTHORIZED", "http_status": float64(401)}},
		{"turno confirmado", "/v1/appointments", appointment, 201,
			func(c *clinic.Client) error { _, err := c.Book(context.Background(), bookingInput()); return err },
			map[string]any{"operation": "book", "outcome": "ok", "appointment_id": "turn-1", "appointment_status": "booked", "level": "INFO"}},
		{"consulta inexistente", "/v1/appointments/turn-1", `{"error":{"code":"NOT_FOUND"}}`, 404,
			func(c *clinic.Client) error { _, err := c.Lookup(context.Background(), "turn-1"); return err },
			map[string]any{"operation": "lookup", "appointment_id": "turn-1", "outcome": "failure", "result_code": "CLINIC_NOT_FOUND"}},
		{"médicos", "/v1/doctors", `{"doctors":[{"id":"doc-001","durationMinutes":30}]}`, 200,
			func(c *clinic.Client) error { _, err := c.Doctors(context.Background()); return err },
			map[string]any{"operation": "doctors", "outcome": "ok", "http_status": float64(200)}},
		{"médicos inválidos", "/v1/doctors", `{"doctors":[{"id":"doc-001","durationMinutes":0}]}`, 200,
			func(c *clinic.Client) error { _, err := c.Doctors(context.Background()); return err },
			map[string]any{"operation": "doctors", "outcome": "unavailable", "cause": "invalid_body"}},
		{"facturación caída", "/v1/usage", `{"error":{"code":"UNAVAILABLE"}}`, 503,
			func(c *clinic.Client) error { _, err := c.Usage(context.Background(), "2026-11"); return err },
			map[string]any{"operation": "usage", "period": "2026-11", "outcome": "unavailable", "provider_code": "UNAVAILABLE"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			logs := captureLogs(t)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != tc.path {
					t.Errorf("ruta=%s", r.URL.Path)
				}
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer server.Close()
			client, _ := clinic.New(server.URL+"/v1", clinicTestKey, true, time.Second)
			_ = tc.call(client)
			records, raw := logs()
			record := onlyClinicRecord(t, records)
			for key, value := range tc.want {
				if record[key] != value {
					t.Fatalf("%s=%v, registro=%v", key, record[key], record)
				}
			}
			assertNoSecrets(t, raw)
		})
	}
}

func TestInvalidInputIsNotSentNorLogged(t *testing.T) {
	logs := captureLogs(t)
	client, _ := clinic.New("https://hospital.invalid/v1", clinicTestKey, false, time.Second)
	input := bookingInput()
	input.IdempotencyKey = ""
	if _, err := client.Book(context.Background(), input); !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("err=%v", err)
	}
	if records, _ := logs(); len(records) != 0 {
		t.Fatalf("registros=%v", records)
	}
}
