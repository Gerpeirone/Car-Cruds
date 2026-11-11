package httpapi

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/Gerpeirone/Car-Cruds/internal/platform"
	"github.com/Gerpeirone/Car-Cruds/services/rentals/application"
	"github.com/Gerpeirone/Car-Cruds/services/rentals/domain"
)

// listRepo filters like the PostgreSQL query: by driver and, when given, by
// the application that created the reservation.
type listRepo struct {
	application.Repository
	rows []domain.Reservation
}

func (f *listRepo) List(_ context.Context, customerID, ownerApp string, page, pageSize int) ([]domain.Reservation, int64, error) {
	var items []domain.Reservation
	for _, r := range f.rows {
		if r.CustomerID == customerID && (ownerApp == "" || r.OwnerApp == ownerApp) {
			items = append(items, r)
		}
	}
	total := int64(len(items))
	from := min((page-1)*pageSize, len(items))
	return items[from:min(from+pageSize, len(items))], total, nil
}

// Regression of "Mis alquileres" empty in Azure: the access form was used with
// the operator credential, the browser asked GET /v1/reservations without
// customerId and Rentals answered 400. The five fictitious rows reproduce the
// case: driver cli-001, created by "operador", four RESERVED and one CANCELLED.
func TestListReservationsOperatorCredentialRegression(t *testing.T) {
	reservation := func(id string, status domain.Status, customer, app string) domain.Reservation {
		return domain.Reservation{ID: id, Status: status, VehicleID: "demo-" + id, CustomerID: customer, OwnerApp: app,
			Period: domain.Period{StartsOn: "2030-02-01", EndsOn: "2030-02-04", Days: 3}, DailyRateMinor: 5000000, TotalMinor: 15000000, Currency: "ARS"}
	}
	repo := &listRepo{rows: []domain.Reservation{
		reservation("res-1", domain.Reserved, "cli-001", "operador"),
		reservation("res-2", domain.Reserved, "cli-001", "operador"),
		reservation("res-3", domain.Reserved, "cli-001", "operador"),
		reservation("res-4", domain.Reserved, "cli-001", "operador"),
		reservation("res-5", domain.Cancelled, "cli-001", "operador"),
		reservation("res-ajena", domain.Reserved, "cli-002", "operador"),
		reservation("res-propia", domain.Reserved, "cli-001", "cliente-demo"),
	}}
	// Same principals as platform.TokensFromEnv.
	h := platform.AuthMiddleware(&Handler{Service: &application.Service{Repo: repo}}, map[string]platform.Principal{
		"operator-fixture": {App: "operador", Role: "admin"},
		"client-fixture":   {App: "cliente-demo", Role: "client", CustomerID: "cli-001"},
	})
	get := func(token, query string) (int, map[string]any) {
		t.Helper()
		r := httptest.NewRequest("GET", "/v1/reservations"+query, nil)
		r.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		var body map[string]any
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
			t.Fatalf("%s %s: %s", token, query, w.Body)
		}
		return w.Code, body
	}
	ids := func(body map[string]any) map[string]string {
		out := map[string]string{}
		for _, v := range body["items"].([]any) {
			item := v.(map[string]any)
			if item["customerId"] != "cli-001" {
				t.Fatalf("reserva de otro conductor: %v", item)
			}
			out[item["id"].(string)] = item["status"].(string)
		}
		return out
	}

	// The failing request, as the previous frontend sent it.
	if code, body := get("operator-fixture", "?page=1&pageSize=20"); code != 400 || body["code"] != "INVALID_INPUT" {
		t.Fatalf("sin customerId: %d %v", code, body)
	}
	// The corrected request: the driver of the session is explicit.
	code, body := get("operator-fixture", "?customerId=cli-001&page=1&pageSize=20")
	got := ids(body)
	if code != 200 || body["total"] != float64(6) || len(got) != 6 || got["res-5"] != "CANCELLED" || got["res-1"] != "RESERVED" {
		t.Fatalf("operador: %d %v", code, body)
	}
	// The driver credential may name itself, never another driver.
	code, body = get("client-fixture", "?customerId=cli-001&page=1&pageSize=20")
	if got := ids(body); code != 200 || len(got) != 1 || got["res-propia"] != "RESERVED" {
		t.Fatalf("conductor: %d %v", code, body)
	}
	if code, body := get("client-fixture", "?customerId=cli-002&page=1&pageSize=20"); code != 403 || body["code"] != "FORBIDDEN" {
		t.Fatalf("conductor ajeno: %d %v", code, body)
	}
}
