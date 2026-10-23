package httpapi_test

import "testing"

func TestClinicRoutesNeverExposePersonalEvidenceToGymOrCustomer(t *testing.T) {
	handler := testHandler()
	for _, route := range []struct{ method, path string }{
		{"POST", "/v1/employees/clinic/patient-links"},
		{"POST", "/v1/employees/clinic/requests"},
		{"GET", "/v1/employees/clinic/requests/some-request"},
		{"POST", "/v1/employees/clinic/requests/some-request/submit"},
		{"POST", "/v1/employees/clinic/requests/some-request/refresh"},
		{"GET", "/v1/employees/reports/clinic"},
	} {
		for _, token := range []string{"", "test-gym", "test-client"} {
			want := 403
			if token == "" {
				want = 401
			}
			w := request(handler, route.method, route.path, "", token)
			if w.Code != want {
				t.Fatalf("%s %s token=%q: %d %s", route.method, route.path, token, w.Code, w.Body)
			}
		}
	}
	for _, tc := range []struct{ method, path, body string }{
		{"POST", "/v1/employees/clinic/requests", `{"personKind":"employee","personId":"e","localReference":"r","doctorId":"d","startAt":"2026-12-01T10:00:00.001Z"}`},
		{"POST", "/v1/employees/clinic/patient-links", `{"personKind":"client","personId":"e","patientId":"p"}`},
		{"POST", "/v1/employees/clinic/patient-links", `{"personKind":"employee","personId":"e","patientId":"p","diagnosis":"not allowed"}`},
		{"POST", "/v1/employees/clinic/requests?patientId=p", `{}`},
		{"POST", "/v1/employees/clinic/requests/r/submit", `{"patientId":"spoof"}`},
		{"GET", "/v1/employees/reports/clinic?personKind=employee&personKind=candidate", ""},
		{"GET", "/v1/employees/reports/clinic?personKind=client", ""},
		{"GET", "/v1/employees/reports/clinic?unknown=value", ""},
	} {
		w := request(handler, tc.method, tc.path, tc.body, "test-admin")
		if w.Code != 400 {
			t.Fatalf("%s %s: %d %s", tc.method, tc.path, w.Code, w.Body)
		}
	}
}
