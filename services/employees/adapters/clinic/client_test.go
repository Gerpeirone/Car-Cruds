package clinic_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Gerpeirone/Car-Cruds/services/employees/adapters/clinic"
	"github.com/Gerpeirone/Car-Cruds/services/employees/domain"
)

const appointment = `{"id":"turn-1","patientId":"pat-001","doctorId":"doc-001","startAt":"2026-12-01T10:00:00Z","endAt":"2026-12-01T10:30:00Z","status":"booked","version":1}`

func TestObservedAppointmentContractAndNoInventedCredentials(t *testing.T) {
	var calls atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.Header.Get("Authorization") != "" || r.Header.Get("X-API-Key") != "" || r.Header.Get("Idempotency-Key") != "" {
			t.Error("invented provider authentication/idempotency")
		}
		if r.Method == "POST" {
			if r.URL.Path != "/api/turnos/appointments" || r.Header.Get("X-Correlation-ID") != "intention-1" {
				t.Error("wrong provider path/correlation")
			}
			var body map[string]string
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Error(err)
			}
			if len(body) != 3 || body["patientId"] != "pat-001" || body["doctorId"] != "doc-001" || body["startAt"] != "2026-12-01T10:00:00Z" {
				t.Errorf("body=%v", body)
			}
			w.WriteHeader(201)
		} else if r.URL.Path != "/api/turnos/appointments/turn-1" {
			t.Error("wrong lookup path")
		}
		_, _ = w.Write([]byte(appointment))
	}))
	defer server.Close()
	client, err := clinic.New(server.URL+"/api/turnos", true, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	start, _ := time.Parse(time.RFC3339, "2026-12-01T10:00:00Z")
	a, err := client.Book(context.Background(), domain.ClinicRequest{ID: "intention-1", PatientID: "pat-001", DoctorID: "doc-001", StartAt: start})
	if err != nil || a.Status != "booked" {
		t.Fatalf("book=%+v %v", a, err)
	}
	a, err = client.Lookup(context.Background(), a.ID)
	if err != nil || a.Version != 1 || calls.Load() != 2 {
		t.Fatalf("lookup=%+v %v", a, err)
	}
}
func TestProviderFailuresStayUnknownAndWritesAreNotRetried(t *testing.T) {
	for _, tc := range []struct {
		name     string
		status   int
		body     string
		rejected bool
	}{
		{"explicit overlap", 409, `{"error":{"code":"SLOT_UNAVAILABLE","message":"ignored"}}`, true},
		{"not enabled", 403, `{"error":{"code":"PATIENT_NOT_ENABLED"}}`, true},
		{"unknown patient", 404, `{"error":{"code":"UNKNOWN_PATIENT"}}`, true},
		{"short lead", 422, `{"error":{"code":"LEAD_TIME_TOO_SHORT"}}`, true},
		{"gateway timeout", 504, `{"error":{"code":"UPSTREAM_TIMEOUT"}}`, false},
		{"service failure", 503, `{"error":{"code":"UNAVAILABLE"}}`, false},
		{"wrong error envelope", 409, `{"error":{"code":"INTERNAL"}}`, false},
		{"malformed success", 201, `{"id":"some"}`, false},
		{"unknown status", 201, strings.Replace(appointment, "booked", "sent", 1), false},
		{"oversized", 201, strings.Repeat(" ", 65537), false},
		{"trailing json", 201, appointment + ` {}`, false},
		{"redirect", 307, `{}`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var calls atomic.Int64
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				w.Header().Set("Location", "/somewhere")
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer server.Close()
			client, _ := clinic.New(server.URL, true, time.Second)
			_, err := client.Book(context.Background(), domain.ClinicRequest{})
			var rejection *domain.ClinicRejection
			if errors.As(err, &rejection) != tc.rejected || err == nil || calls.Load() != 1 {
				t.Fatalf("err=%v calls=%d", err, calls.Load())
			}
		})
	}
	var calls atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(201)
		_, _ = w.Write([]byte(appointment))
	}))
	defer server.Close()
	client, _ := clinic.New(server.URL, true, 100*time.Millisecond)
	_, err := client.Book(context.Background(), domain.ClinicRequest{})
	if !errors.Is(err, domain.ErrClinicUnavailable) || calls.Load() != 1 {
		t.Fatalf("timeout=%v calls=%d", err, calls.Load())
	}
}
func TestClinicConfigurationRequiresExplicitTransport(t *testing.T) {
	for _, base := range []string{"", "http://hospital.example/api/turnos", "https://user:secret@hospital.example", "https://hospital.example?token=secret", "https://hospital.example/#fragment"} {
		if _, err := clinic.New(base, false, time.Second); err == nil {
			t.Fatalf("accepted %q", base)
		}
	}
	if _, err := clinic.New("https://hospital.example/api/turnos", false, time.Second); err != nil {
		t.Fatal(err)
	}
}
