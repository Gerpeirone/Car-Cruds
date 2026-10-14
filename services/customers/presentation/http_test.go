package presentation

import (
	"github.com/Gerpeirone/Car-Cruds/internal/platform"
	"github.com/Gerpeirone/Car-Cruds/services/customers/business"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCustomerOwnershipAndRoleGuards(t *testing.T) {
	h := platform.AuthMiddleware(&Handler{Service: &business.Service{}}, map[string]platform.Principal{"client": {App: "client", Role: "client", CustomerID: "cli-001"}, "gym": {App: "gym", Role: "gym"}})
	for _, test := range []struct {
		method, path, token string
		status              int
	}{{"GET", "/v1/customers/cli-other", "client", 404}, {"GET", "/v1/customers/cli-001", "gym", 403}, {"GET", "/v1/customers", "client", 403}, {"POST", "/v1/customers", "client", 403}, {"GET", "/v1/customers/cli-001", "", 401}} {
		r := httptest.NewRequest(test.method, test.path, strings.NewReader(`{}`))
		if test.token != "" {
			r.Header.Set("Authorization", "Bearer "+test.token)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != test.status {
			t.Fatal(test, w.Code, w.Body.String())
		}
	}
}

func TestCustomerPatternDoesNotExposeIdentifier(t *testing.T) {
	h := platform.AuthMiddleware(&Handler{Service: &business.Service{}}, map[string]platform.Principal{"gym": {App: "gym", Role: "gym"}})
	for _, id := range []string{"cli-first", "cli-second"} {
		r := httptest.NewRequest("GET", "/v1/customers/"+id, nil)
		r.Header.Set("Authorization", "Bearer gym")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != 403 || r.Pattern != "GET /v1/customers/{id}" {
			t.Fatal(w.Code, r.Pattern)
		}
	}
}
