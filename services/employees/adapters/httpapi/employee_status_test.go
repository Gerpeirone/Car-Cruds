package httpapi_test

import (
	"encoding/json"
	"testing"
)

func TestGymVerifiesInactiveEmployeeWithoutReusingIdentityOrLeakingDetails(t *testing.T) {
	h := testHandler()
	w := request(h, "POST", "/v1/employees", `{"displayName":"Persona ficticia","active":true}`, "test-admin")
	if w.Code != 201 {
		t.Fatal(w.Code, w.Body.String())
	}
	check := func(id string, active bool) {
		t.Helper()
		w := request(h, "GET", "/v1/employees/"+id+"/status", "", "test-gym")
		var result map[string]any
		if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		if w.Code != 200 || len(result) != 2 || result["employeeId"] != id || result["active"] != active {
			t.Fatalf("status leaks or changes identity: %d %s", w.Code, w.Body)
		}
	}
	check("person-001", true)
	w = request(h, "DELETE", "/v1/employees/person-001", "", "test-admin")
	if w.Code != 200 {
		t.Fatal(w.Code)
	}
	check("person-001", false)
	// Names are mutable, identities are not. Reactivation keeps the same ID.
	w = request(h, "PUT", "/v1/employees/person-001", `{"displayName":"Nombre actualizado","active":true}`, "test-admin")
	if w.Code != 200 {
		t.Fatal(w.Code)
	}
	check("person-001", true)
	w = request(h, "POST", "/v1/employees", `{"displayName":"Persona ficticia","active":true}`, "test-admin")
	if w.Code != 201 {
		t.Fatal(w.Code)
	}
	check("person-002", true)
	request(h, "POST", "/v1/candidates", `{"displayName":"Candidato ficticio"}`, "test-admin")
	for _, tc := range []struct {
		method, path, token string
		want                int
	}{
		{"GET", "/v1/employees/person-001/status", "", 401},
		{"GET", "/v1/employees/person-001/status", "test-client", 403},
		{"GET", "/v1/employees/person-001/status", "test-service", 200},
		{"POST", "/v1/employees/person-001/status", "test-gym", 405},
		{"GET", "/v1/employees/person-001/status?includePrivate=true", "test-gym", 400},
		{"GET", "/v1/employees/person-003/status", "test-gym", 404},
		{"GET", "/v1/employees/person-001", "test-gym", 403},
	} {
		w := request(h, tc.method, tc.path, "", tc.token)
		if w.Code != tc.want {
			t.Fatalf("%s %s: %d %s", tc.method, tc.path, w.Code, w.Body)
		}
	}
}
