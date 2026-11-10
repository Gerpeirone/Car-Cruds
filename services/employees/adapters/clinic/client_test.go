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
const clinicTestKey = "clinic-fixture-private-key"

func bookingInput() domain.ClinicRequest {
	start, _ := time.Parse(time.RFC3339, "2026-12-01T10:00:00Z")
	return domain.ClinicRequest{ID: "intention-1", ProviderContract: domain.ClinicProviderContract, IdempotencyKey: "durable-intention-1", PatientID: "pat-001", DoctorID: "doc-001", StartAt: start}
}

func TestPublishedAppointmentContractAuthenticatesAndUsesDurableKey(t *testing.T) {
	var calls atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.Header.Get("Authorization") != "" || r.Header.Get("X-API-Key") != clinicTestKey {
			t.Error("provider credential missing or caller credential forwarded")
		}
		if r.Method == "POST" {
			if r.URL.Path != "/v1/appointments" || r.Header.Get("X-Correlation-ID") != "intention-1" || r.Header.Get("Idempotency-Key") != "durable-intention-1" {
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
		} else if r.URL.Path != "/v1/appointments/turn-1" || r.Header.Get("Idempotency-Key") != "" {
			t.Error("wrong lookup path")
		}
		_, _ = w.Write([]byte(appointment))
	}))
	defer server.Close()
	client, err := clinic.New(server.URL+"/v1", clinicTestKey, true, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	a, err := client.Book(context.Background(), bookingInput())
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
			client, _ := clinic.New(server.URL+"/v1", clinicTestKey, true, time.Second)
			_, err := client.Book(context.Background(), bookingInput())
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
	client, _ := clinic.New(server.URL+"/v1", clinicTestKey, true, 100*time.Millisecond)
	_, err := client.Book(context.Background(), bookingInput())
	if !errors.Is(err, domain.ErrClinicUnavailable) || calls.Load() != 1 {
		t.Fatalf("timeout=%v calls=%d", err, calls.Load())
	}
}
func TestClinicConfigurationRequiresExplicitTransport(t *testing.T) {
	for _, base := range []string{"", "http://hospital.example/v1", "https://user:secret@hospital.example/v1", "https://hospital.example/v1?token=secret", "https://hospital.example/v1#fragment", "https://hospital.example/api/turnos", "https://hospital.example/a/../v1"} {
		if _, err := clinic.New(base, clinicTestKey, false, time.Second); err == nil {
			t.Fatalf("accepted %q", base)
		}
	}
	if _, err := clinic.New("https://hospital.example/v1", clinicTestKey, false, time.Second); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"", " ", " private-key", "private-key\r\nX-Other: yes"} {
		if _, err := clinic.New("https://hospital.example/v1", key, false, time.Second); err == nil || strings.Contains(err.Error(), "private-key") {
			t.Fatal("invalid credential accepted or leaked")
		}
	}
}

func TestOperationalErrorsPreserveCodesAndRetryAfter(t *testing.T) {
	for _, tc := range []struct {
		name, method, code, want, retry string
		status                          int
		delay                           time.Duration
	}{
		{"credential", "POST", "UNAUTHORIZED", "CLINIC_UNAUTHORIZED", "", 401, 0},
		{"rate", "POST", "RATE_LIMITED", "CLINIC_RATE_LIMITED", "7", 429, 7 * time.Second},
		{"rate lookup", "GET", "RATE_LIMITED", "CLINIC_RATE_LIMITED", "7", 429, 7 * time.Second},
		{"invalid retry header", "POST", "RATE_LIMITED", "CLINIC_RATE_LIMITED", "invalid", 429, time.Minute},
		{"idempotency", "POST", "IDEMPOTENCY_CONFLICT", "CLINIC_IDEMPOTENCY_CONFLICT", "", 409, 0},
		{"lookup missing or foreign", "GET", "NOT_FOUND", "CLINIC_NOT_FOUND", "", 404, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var calls atomic.Int64
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				w.Header().Set("Retry-After", tc.retry)
				w.WriteHeader(tc.status)
				_ = json.NewEncoder(w).Encode(map[string]any{"error": map[string]string{"code": tc.code, "message": "private-provider-detail"}})
			}))
			defer server.Close()
			client, _ := clinic.New(server.URL+"/v1", clinicTestKey, true, time.Second)
			var err error
			if tc.method == "POST" {
				_, err = client.Book(context.Background(), bookingInput())
			} else {
				_, err = client.Lookup(context.Background(), "turn-1")
			}
			var failure *domain.ClinicFailure
			if !errors.As(err, &failure) || failure.Code != tc.want || failure.RetryAfter != tc.delay || calls.Load() != 1 || strings.Contains(err.Error(), "private-provider-detail") {
				t.Fatalf("unexpected operational result: %v calls=%d", err, calls.Load())
			}
		})
	}
}

func TestBookingRequiresPersistedContractAndKey(t *testing.T) {
	var calls atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls.Add(1) }))
	defer server.Close()
	client, _ := clinic.New(server.URL+"/v1", clinicTestKey, true, time.Second)
	for _, input := range []domain.ClinicRequest{{}, {ProviderContract: domain.ClinicProviderContract}, {ProviderContract: domain.ClinicProviderContract, IdempotencyKey: "reference with spaces"}} {
		if _, err := client.Book(context.Background(), input); !errors.Is(err, domain.ErrInvalidInput) {
			t.Fatalf("unrecorded booking sent: %v", err)
		}
	}
	if calls.Load() != 0 {
		t.Fatal("invalid booking reached the provider")
	}
}

func TestRedirectNeverForwardsProviderKey(t *testing.T) {
	var forwarded atomic.Int64
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { forwarded.Add(1) }))
	defer target.Close()
	source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL+"/v1/appointments", http.StatusTemporaryRedirect)
	}))
	defer source.Close()
	client, _ := clinic.New(source.URL+"/v1", clinicTestKey, true, time.Second)
	if _, err := client.Book(context.Background(), bookingInput()); err == nil || forwarded.Load() != 0 {
		t.Fatal("provider redirect followed")
	}
}

func TestIdempotentReplayCanReturnCancelledAppointment(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Idempotency-Replayed", "true")
		w.WriteHeader(201)
		_, _ = w.Write([]byte(strings.Replace(appointment, `"booked"`, `"cancelled"`, 1)))
	}))
	defer server.Close()
	client, _ := clinic.New(server.URL+"/v1", clinicTestKey, true, time.Second)
	a, err := client.Book(context.Background(), bookingInput())
	if err != nil || a.Status != "cancelled" {
		t.Fatalf("replay misclassified: %+v %v", a, err)
	}
}
