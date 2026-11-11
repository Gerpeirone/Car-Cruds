package clinic_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Gerpeirone/Car-Cruds/services/employees/adapters/clinic"
	"github.com/Gerpeirone/Car-Cruds/services/employees/domain"
)

const usageBody = `{"consumer":"carcruds","period":"2026-12","from":"2026-12-01T00:00:00Z","to":"2027-01-01T00:00:00Z","currentPrice":{"amountCents":2500000,"currency":"ARS"},"totals":[{"currency":"ARS","bookings":2,"amountCents":5000000}],"items":[{"appointmentId":"turn-1","bookedAt":"2026-12-01T13:05:12Z","priceCents":2500000,"currency":"ARS","status":"booked"},{"appointmentId":"turn-2","bookedAt":"2026-12-03T09:41:30Z","priceCents":2500000,"currency":"ARS","status":"cancelled"}]}`

func directoryClient(t *testing.T, handler http.HandlerFunc) *clinic.Client {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	client, err := clinic.New(server.URL+"/v1", clinicTestKey, true, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	return client
}
func TestDoctorsReadsPublishedCatalogWithConsumerKey(t *testing.T) {
	client := directoryClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" || r.URL.Path != "/v1/doctors" || r.Header.Get("X-API-Key") != clinicTestKey || r.Header.Get("Idempotency-Key") != "" {
			t.Errorf("unexpected %s %s", r.Method, r.URL)
		}
		_, _ = w.Write([]byte(`{"doctors":[{"id":"doc-001","durationMinutes":30},{"id":"doc-002","durationMinutes":45}]}`))
	})
	doctors, err := client.Doctors(context.Background())
	if err != nil || len(doctors) != 2 || doctors[1].ID != "doc-002" || doctors[1].DurationMinutes != 45 {
		t.Fatalf("doctors=%+v err=%v", doctors, err)
	}
}
func TestDoctorsRejectsIncompatibleCatalog(t *testing.T) {
	for _, body := range []string{`{"doctors":[{"id":"","durationMinutes":30}]}`, `{"doctors":[{"id":"doc-001","durationMinutes":0}]}`, `{"doctors":[]}{}`} {
		client := directoryClient(t, func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(body)) })
		if _, err := client.Doctors(context.Background()); !errors.Is(err, domain.ErrClinicUnavailable) {
			t.Fatalf("%s: err=%v", body, err)
		}
	}
}
func TestUsageMapsBillingInMinorUnits(t *testing.T) {
	client := directoryClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/usage" || r.URL.RawQuery != "period=2026-12" {
			t.Errorf("unexpected %s", r.URL)
		}
		_, _ = w.Write([]byte(usageBody))
	})
	u, err := client.Usage(context.Background(), "2026-12")
	if err != nil || u.CurrentPrice.AmountMinor != 2500000 || len(u.Totals) != 1 || u.Totals[0].AmountMinor != 5000000 || u.Totals[0].Bookings != 2 || len(u.Items) != 2 || u.Items[1].Status != "cancelled" {
		t.Fatalf("usage=%+v err=%v", u, err)
	}
}
func TestUsageFailuresNeverInventBilling(t *testing.T) {
	cases := []struct {
		status int
		body   string
		want   error
	}{
		{400, `{"error":{"code":"INVALID_PERIOD","message":"x"}}`, domain.ErrInvalidInput},
		{401, `{"error":{"code":"UNAUTHORIZED","message":"x"}}`, domain.ErrClinicUnavailable},
		{200, strings.Replace(usageBody, `"period":"2026-12"`, `"period":"2026-11"`, 1), domain.ErrClinicUnavailable},
		{200, strings.Replace(usageBody, `"amountCents":5000000`, `"amountCents":-1`, 1), domain.ErrClinicUnavailable},
		{200, strings.Repeat(" ", 1<<20+1), domain.ErrClinicUnavailable},
	}
	for _, tc := range cases {
		client := directoryClient(t, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(tc.status)
			_, _ = w.Write([]byte(tc.body))
		})
		if _, err := client.Usage(context.Background(), "2026-12"); !errors.Is(err, tc.want) {
			t.Fatalf("status %d: err=%v want %v", tc.status, err, tc.want)
		}
	}
	client := directoryClient(t, func(w http.ResponseWriter, r *http.Request) { t.Error("invalid period must not call the provider") })
	if _, err := client.Usage(context.Background(), "2026-13"); !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("err=%v", err)
	}
}
