package httpapi_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/Gerpeirone/Car-Cruds/internal/platform"
	"github.com/Gerpeirone/Car-Cruds/services/employees/adapters/httpapi"
	"github.com/Gerpeirone/Car-Cruds/services/employees/application"
	"github.com/Gerpeirone/Car-Cruds/services/employees/domain"
)

type directoryClinic struct{ periods []string }

func (c *directoryClinic) Book(context.Context, domain.ClinicRequest) (domain.ClinicAppointment, error) {
	return domain.ClinicAppointment{}, domain.ErrClinicUnavailable
}
func (c *directoryClinic) Lookup(context.Context, string) (domain.ClinicAppointment, error) {
	return domain.ClinicAppointment{}, domain.ErrClinicUnavailable
}
func (c *directoryClinic) Doctors(context.Context) ([]domain.ClinicDoctor, error) {
	return []domain.ClinicDoctor{{ID: "doc-001", DurationMinutes: 30}}, nil
}
func (c *directoryClinic) Usage(_ context.Context, period string) (domain.ClinicUsage, error) {
	c.periods = append(c.periods, period)
	from := time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC)
	return domain.ClinicUsage{Period: period, From: from, To: from.AddDate(0, 1, 0), CurrentPrice: domain.ClinicMoney{AmountMinor: 2500000, Currency: "ARS"},
		Totals: []domain.ClinicUsageTotal{{Currency: "ARS", Bookings: 1, AmountMinor: 2500000}},
		Items:  []domain.ClinicUsageItem{{AppointmentID: "turn-1", BookedAt: from, PriceMinor: 2500000, Currency: "ARS", Status: "cancelled"}}}, nil
}

func directoryHandler(clinic application.Clinic) func(method, path, token string) (int, map[string]any) {
	repo := &memoryRepository{employees: map[string]domain.Employee{}}
	service := application.New(repo, func() string { return "id" })
	if clinic != nil {
		service.WithClinic(nil, clinic)
	}
	h := platform.AuthMiddleware(httpapi.New(service), map[string]platform.Principal{
		"test-admin": {App: "op", Role: "admin"}, "test-gym": {App: "gym", Role: "gym"},
		"test-client": {App: "c", Role: "client"}, "test-service": {App: "s", Role: "service"},
	})
	return func(method, path, token string) (int, map[string]any) {
		w := request(h, method, path, "", token)
		var body map[string]any
		_ = json.Unmarshal(w.Body.Bytes(), &body)
		return w.Code, body
	}
}
func TestClinicDirectoryRoutesAndPermissions(t *testing.T) {
	clinic := &directoryClinic{}
	call := directoryHandler(clinic)
	if code, body := call("GET", "/v1/employees/clinic/doctors", "test-admin"); code != 200 || fmt.Sprint(body["doctors"]) != "[map[durationMinutes:30 id:doc-001]]" {
		t.Fatalf("doctors: %d %v", code, body)
	}
	if code, body := call("GET", "/v1/employees/clinic/usage?period=2026-12", "test-service"); code != 200 || fmt.Sprint(body["totals"]) != "[map[amountMinor:2.5e+06 bookings:1 currency:ARS]]" {
		t.Fatalf("usage: %d %v", code, body)
	}
	for _, tc := range []struct {
		path, token string
		want        int
	}{
		{"/v1/employees/clinic/doctors", "", 401},
		{"/v1/employees/clinic/doctors", "test-gym", 403},
		{"/v1/employees/clinic/doctors", "test-service", 403},
		{"/v1/employees/clinic/doctors?x=1", "test-admin", 400},
		{"/v1/employees/clinic/usage?period=2026-12", "test-client", 403},
		{"/v1/employees/clinic/usage?period=2026-12", "test-gym", 403},
		{"/v1/employees/clinic/usage", "test-admin", 400},
		{"/v1/employees/clinic/usage?period=2026-13", "test-admin", 400},
		{"/v1/employees/clinic/usage?period=2026-12&period=2026-11", "test-admin", 400},
		{"/v1/employees/clinic/usage?period=2026-12&page=1", "test-admin", 400},
	} {
		if code, _ := call("GET", tc.path, tc.token); code != tc.want {
			t.Fatalf("%s token=%q: %d want %d", tc.path, tc.token, code, tc.want)
		}
	}
	if len(clinic.periods) != 1 {
		t.Fatalf("invalid requests reached the provider: %v", clinic.periods)
	}
	if code, _ := call("POST", "/v1/employees/clinic/doctors", "test-admin"); code != 405 {
		t.Fatalf("method: %d", code)
	}
}
func TestClinicDirectoryWithoutProviderIsNotConfigured(t *testing.T) {
	call := directoryHandler(nil)
	for _, path := range []string{"/v1/employees/clinic/doctors", "/v1/employees/clinic/usage?period=2026-12"} {
		if code, body := call("GET", path, "test-admin"); code != 503 || body["code"] != "NOT_CONFIGURED" {
			t.Fatalf("%s: %d %v", path, code, body)
		}
	}
}
