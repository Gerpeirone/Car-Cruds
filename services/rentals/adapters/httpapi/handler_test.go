package httpapi

import (
	"encoding/json"
	"github.com/Gerpeirone/Car-Cruds/internal/platform"
	"github.com/Gerpeirone/Car-Cruds/services/rentals/application"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGuardedRequestsDoNotReachPersistence(t *testing.T) {
	h := platform.AuthMiddleware(&Handler{Service: &application.Service{}}, map[string]platform.Principal{"client": {App: "client", Role: "client", CustomerID: "cli-001"}, "gym": {App: "gym", Role: "gym"}})
	valid := `{"vehicleId":"auto-001","customerId":"cli-other","startsOn":"2030-01-01","endsOn":"2030-01-04","expectedTotalMinor":15000000}`
	for _, test := range []struct {
		token, body, content string
		status               int
		code                 string
	}{{"", valid, "application/json", 401, "UNAUTHORIZED"}, {"gym", valid, "application/json", 403, "FORBIDDEN"}, {"client", valid, "text/plain", 415, "UNSUPPORTED_MEDIA_TYPE"}, {"client", valid, "application/json", 403, "FORBIDDEN"}, {"client", `{"vehicleId":"a","vehicleId":"b"}`, "application/json", 400, "INVALID_INPUT"}} {
		r := httptest.NewRequest("POST", "/v1/reservations", strings.NewReader(test.body))
		r.Header.Set("Content-Type", test.content)
		r.Header.Set("Idempotency-Key", "guard")
		if test.token != "" {
			r.Header.Set("Authorization", "Bearer "+test.token)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		var body map[string]any
		json.Unmarshal(w.Body.Bytes(), &body)
		if w.Code != test.status || body["code"] != test.code {
			t.Fatal(w.Code, w.Body.String())
		}
	}
}

var _ http.Handler = (*Handler)(nil)

func TestKnownPatternsRemainStableAcrossResourceIDs(t *testing.T) {
	h := platform.AuthMiddleware(&Handler{Service: &application.Service{}}, map[string]platform.Principal{"gym": {App: "gym", Role: "gym"}})
	for _, test := range []struct{ method, path, want string }{{"GET", "/v1/reservations/res-a", "GET /v1/reservations/{id}"}, {"GET", "/v1/reservations/res-b", "GET /v1/reservations/{id}"}, {"POST", "/v1/reservations/res-a/pickup", "POST /v1/reservations/{id}/pickup"}, {"POST", "/v1/maintenance", "POST /v1/maintenance"}, {"GET", "/v1/availability", "GET /v1/availability"}} {
		r := httptest.NewRequest(test.method, test.path, nil)
		r.Header.Set("Authorization", "Bearer gym")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != 403 || r.Pattern != test.want {
			t.Fatalf("status=%d pattern=%q want=%q", w.Code, r.Pattern, test.want)
		}
	}
}
