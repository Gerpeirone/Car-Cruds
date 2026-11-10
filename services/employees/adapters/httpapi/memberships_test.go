package httpapi_test

import (
	"context"
	"github.com/Gerpeirone/Car-Cruds/internal/platform"
	"github.com/Gerpeirone/Car-Cruds/services/employees/adapters/httpapi"
	"github.com/Gerpeirone/Car-Cruds/services/employees/application"
	"github.com/Gerpeirone/Car-Cruds/services/employees/domain"
	"net/http"
	"testing"
)

type inboxSpy struct {
	calls    int
	source   string
	snapshot domain.MembershipSnapshot
}

func (s *inboxSpy) ReceiveMembership(_ context.Context, source string, snapshot domain.MembershipSnapshot) (bool, error) {
	s.calls++
	s.source = source
	s.snapshot = snapshot
	return true, nil
}
func (s *inboxSpy) MembershipReport(_ context.Context, period string, page domain.Page) (domain.MembershipReport, error) {
	s.calls++
	return domain.MembershipReport{Period: period, Page: page}, nil
}

func TestGymInboxAuthorizationStrictInputAndAuthenticatedSource(t *testing.T) {
	spy := &inboxSpy{}
	handler := platform.AuthMiddleware(httpapi.New(application.New(nil, nil).WithMembershipInbox(spy)), map[string]platform.Principal{
		"gym": {App: "gimnasio-pablo", Role: "gym"}, "admin": {App: "operator", Role: "admin"}, "service": {App: "internal", Role: "service"},
	})
	valid := `{"employeeId":"e1","period":"2026-10","sourceReference":"gym-1","version":1,"observedAt":"2026-10-22T12:00:00Z","member":true,"used":false,"costMinor":100,"spendingMinor":0,"currency":"ARS"}`
	for _, tc := range []struct {
		method, path, body, token string
		status                    int
	}{
		{"POST", "/v1/employees/membership-evidence", valid, "", 401},
		{"POST", "/v1/employees/membership-evidence", valid, "admin", 403},
		{"POST", "/v1/employees/membership-evidence", valid, "service", 403},
		{"GET", "/v1/employees/membership-evidence", "", "gym", 405},
		{"POST", "/v1/employees/membership-evidence?x=1", valid, "gym", 400},
		{"POST", "/v1/employees/membership-evidence", `{"employeeId":"e1","sourceApp":"operator"}`, "gym", 400},
		{"POST", "/v1/employees/membership-evidence", `{"version":1,"version":2}`, "gym", 400},
		{"POST", "/v1/employees/membership-evidence", `{"employeeId":"e1"}`, "gym", 400},
		{"GET", "/v1/employees/reports/memberships?period=2026-10", "", "gym", 403},
		{"GET", "/v1/employees/reports/memberships", "", "admin", 400},
		{"GET", "/v1/employees/reports/memberships?period=a&period=b", "", "admin", 400},
		{"GET", "/v1/employees/reports/memberships?period=a&x=b", "", "admin", 400},
		{"GET", "/v1/employees/reports/memberships?period=a&pageSize=101", "", "admin", 400},
	} {
		w := request(handler, tc.method, tc.path, tc.body, tc.token)
		if w.Code != tc.status {
			t.Fatalf("%s %s: %d %s", tc.method, tc.path, w.Code, w.Body)
		}
	}
	if spy.calls != 0 {
		t.Fatal("rejected request reached storage")
	}
	w := request(handler, http.MethodPost, "/v1/employees/membership-evidence", valid, "gym")
	if w.Code != 200 || spy.source != "gimnasio-pablo" || spy.snapshot.Used == nil || *spy.snapshot.Used || spy.snapshot.SpendingMinor == nil || *spy.snapshot.SpendingMinor != 0 {
		t.Fatalf("receipt: %d %s %+v", w.Code, w.Body, spy)
	}
	w = request(handler, "GET", "/v1/employees/reports/memberships?period=2026-10&pageSize=1", "", "admin")
	if w.Code != 200 {
		t.Fatalf("report: %d %s", w.Code, w.Body)
	}
	partial := `{"employeeId":"e1","period":"2026-10","sourceReference":"gym-1","version":2,"observedAt":"2026-10-23T12:00:00Z","member":null,"used":null,"costMinor":null,"spendingMinor":null,"currency":"ARS"}`
	w = request(handler, "POST", "/v1/employees/membership-evidence", partial, "gym")
	if w.Code != 200 || spy.snapshot.Member != nil || spy.snapshot.Used != nil || spy.snapshot.CostMinor != nil || spy.snapshot.SpendingMinor != nil {
		t.Fatalf("partial facts coerced to false/zero: %d %s", w.Code, w.Body)
	}
}
