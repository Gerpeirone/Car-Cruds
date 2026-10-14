package platform

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAuthCannotSpoofPrincipal(t *testing.T) {
	h := AuthMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := PrincipalFrom(r.Context())
		if p.Role != "gym" || p.App != "gym-test" {
			t.Errorf("principal inesperado: %+v", p)
		}
		if RequireRole(w, r, "admin") {
			t.Error("se aceptó escalamiento de permisos")
		}
	}), map[string]Principal{"valid": {App: "gym-test", Role: "gym"}})
	req := httptest.NewRequest("GET", "/v1/employees", nil)
	req.Header.Set("Authorization", "Bearer valid")
	req.Header.Set("X-Role", "admin")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != 403 {
		t.Fatalf("status %d", w.Code)
	}
	w = httptest.NewRecorder()
	req.Header.Set("Authorization", "Bearer invalid")
	h.ServeHTTP(w, req)
	if w.Code != 401 {
		t.Fatalf("status %d", w.Code)
	}
}
func TestDecodeRejectsAmbiguousBodies(t *testing.T) {
	for _, body := range []string{`{"a":1} {"a":2}`, `{"unknown":1}`, `{"a":1,"a":2}`, `{"A":1}`, `null`, strings.Repeat("x", (1<<20)+1)} {
		var out struct {
			A int `json:"a"`
		}
		r := httptest.NewRequest("POST", "/", strings.NewReader(body))
		if Decode(httptest.NewRecorder(), r, &out) == nil {
			t.Fatal("se aceptó una entrada inválida")
		}
	}
}
