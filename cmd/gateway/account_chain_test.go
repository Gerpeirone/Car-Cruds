package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Gerpeirone/Car-Cruds/internal/platform"
	"github.com/Gerpeirone/Car-Cruds/services/customers/business"
	"github.com/Gerpeirone/Car-Cruds/services/customers/presentation"
	"github.com/Gerpeirone/Car-Cruds/services/fleet"
)

// memoryAccounts is a disposable account store for the HTTP chain tests.
type memoryAccounts struct {
	mu       sync.Mutex
	accounts map[string]business.Account
	hashes   map[string]string
	sessions map[string]struct {
		account string
		expires time.Time
	}
}

func newMemoryAccounts() *memoryAccounts {
	return &memoryAccounts{accounts: map[string]business.Account{}, hashes: map[string]string{}, sessions: map[string]struct {
		account string
		expires time.Time
	}{}}
}
func (m *memoryAccounts) InsertAccount(_ context.Context, a business.Account, hash string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.accounts[a.Email]; !ok {
		m.accounts[a.Email], m.hashes[a.Email] = a, hash
	}
	return nil
}
func (m *memoryAccounts) AccountByEmail(_ context.Context, email string) (business.Account, string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	a, ok := m.accounts[email]
	if !ok {
		return business.Account{}, "", business.ErrAccountAuth
	}
	return a, m.hashes[email], nil
}
func (m *memoryAccounts) InsertSession(_ context.Context, id string, digest []byte, expires time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sessions[string(digest)] = struct {
		account string
		expires time.Time
	}{id, expires}
	return nil
}
func (m *memoryAccounts) AccountBySession(_ context.Context, digest []byte) (business.Account, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.sessions[string(digest)]
	if !ok || !time.Now().Before(s.expires) {
		return business.Account{}, business.ErrAccountAuth
	}
	for _, a := range m.accounts {
		if a.ID == s.account {
			return a, nil
		}
	}
	return business.Account{}, business.ErrAccountAuth
}
func (m *memoryAccounts) DeleteSession(_ context.Context, digest []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.sessions, string(digest))
	return nil
}

func (m *memoryAccounts) expireAll() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for k, s := range m.sessions {
		s.expires = time.Now().Add(-time.Second)
		m.sessions[k] = s
	}
}

// The browser path of a personal account through the real gateway, the real
// Customers account handler and the real Flota cookie check: the session
// cookie keeps working on the catalog and the vehicle detail, ends on logout
// or expiry, and never reaches or unlocks any other service.
func TestAccountSessionAcrossGatewayCustomersAndFleet(t *testing.T) {
	tokens := map[string]platform.Principal{"admin-fixture-token-not-real": {App: "operador", Role: "admin"}}
	store := newMemoryAccounts()
	customersMux := http.NewServeMux()
	customersMux.Handle("/v1/auth/", presentation.NewAccountHandler(business.Accounts{Store: store, TTL: time.Hour}))
	customers := httptest.NewServer(platform.AuthMiddleware(customersMux, tokens, platform.AccountRoute))
	defer customers.Close()
	vehicles := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !platform.RequireRole(w, r, "admin", "client", "service", "catalog") {
			return
		}
		platform.WriteJSON(w, 200, map[string]string{"path": r.URL.Path, "role": platform.PrincipalFrom(r.Context()).Role})
	})
	fleetServer := httptest.NewServer(platform.AuthMiddleware(fleet.WithAccountCatalog(vehicles, customers.URL), tokens, platform.AccountCatalogRoute))
	defer fleetServer.Close()
	var leaked atomic.Int64
	other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Cookie") != "" {
			leaked.Add(1)
		}
		platform.WriteJSON(w, 200, map[string]string{})
	}))
	defer other.Close()
	t.Setenv("CUSTOMERS_URL", customers.URL)
	t.Setenv("FLEET_URL", fleetServer.URL)
	t.Setenv("RENTALS_URL", other.URL)
	t.Setenv("EMPLOYEES_URL", other.URL)
	gw, _, err := handler()
	if err != nil {
		t.Fatal(err)
	}
	front := httptest.NewServer(platform.AuthMiddleware(gw, tokens, platform.AccountRoute, platform.AccountCatalogRoute))
	defer front.Close()
	do := func(method, path, body string, cookie *http.Cookie) (int, string, *http.Response) {
		t.Helper()
		r, _ := http.NewRequest(method, front.URL+path, bytes.NewBufferString(body))
		if body != "" {
			r.Header.Set("Content-Type", "application/json")
		}
		if cookie != nil {
			r.AddCookie(cookie)
		}
		res, err := http.DefaultClient.Do(r)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		var b bytes.Buffer
		_, _ = b.ReadFrom(res.Body)
		return res.StatusCode, b.String(), res
	}
	const password = "  Frase ficticia de laboratorio  "
	register := `{"firstName":"Ana","lastName":"Demo","email":" Ana@Example.TEST ","password":"` + password + `","passwordConfirmation":"` + password + `"}`
	if code, _, _ := do("POST", "/v1/auth/register", register, nil); code != 202 {
		t.Fatalf("registro=%d", code)
	}
	if code, _, _ := do("POST", "/v1/auth/login", `{"email":"ana@example.test","password":"incorrecta de prueba"}`, nil); code != 401 {
		t.Fatalf("contraseña incorrecta=%d", code)
	}
	code, body, res := do("POST", "/v1/auth/login", `{"email":"ANA@example.test ","password":"`+password+`"}`, nil)
	if code != 200 || strings.Contains(body, "password") || strings.Contains(body, "token") {
		t.Fatalf("login=%d %s", code, body)
	}
	var cookie *http.Cookie
	for _, c := range res.Cookies() {
		if c.Name == platform.AccountCookie {
			cookie = c
		}
	}
	if cookie == nil || !cookie.HttpOnly || !cookie.Secure || cookie.SameSite != http.SameSiteStrictMode || cookie.Path != "/" || cookie.Domain != "" || !cookie.Expires.After(time.Now().Add(50*time.Minute)) {
		t.Fatalf("cookie=%+v", cookie)
	}
	var me business.Account
	if code, body, _ := do("GET", "/v1/auth/me", "", cookie); code != 200 || json.Unmarshal([]byte(body), &me) != nil || me.Email != "ana@example.test" || me.Role != "client" {
		t.Fatalf("me=%d %s", code, body)
	}
	for range 3 {
		for _, path := range []string{"/v1/vehicles/search?q=toyota", "/v1/vehicles?page=1", "/v1/vehicles/auto-001", "/v1/vehicles/auto_002-b"} {
			if code, body, _ := do("GET", path, "", cookie); code != 200 || !strings.Contains(body, `"role":"catalog"`) {
				t.Fatalf("%s=%d %s", path, code, body)
			}
		}
	}
	for _, c := range []struct{ method, path string }{
		{"GET", "/v1/admin/employees"}, {"GET", "/v1/employees/clinic/doctors"}, {"GET", "/v1/vehicles/auto-001/quote"},
		{"POST", "/v1/vehicles"}, {"GET", "/v1/reservations/r-1"}, {"GET", "/v1/reservations"}, {"POST", "/v1/reservations"}, {"GET", "/v1/customers/cli-001"},
	} {
		body := ""
		if c.method == "POST" {
			body = "{}"
		}
		if code, _, _ := do(c.method, c.path, body, cookie); code != 401 {
			t.Fatalf("%s %s con cookie de cliente=%d", c.method, c.path, code)
		}
	}
	if leaked.Load() != 0 {
		t.Fatal("la cookie personal llegó a otro servicio")
	}
	if code, _, _ := do("POST", "/v1/auth/logout", "{}", cookie); code != 204 {
		t.Fatalf("logout=%d", code)
	}
	for _, path := range []string{"/v1/auth/me", "/v1/vehicles/auto-001", "/v1/vehicles/search?q=x"} {
		if code, _, _ := do("GET", path, "", cookie); code != 401 {
			t.Fatalf("%s con cookie revocada=%d", path, code)
		}
	}
	_, _, res = do("POST", "/v1/auth/login", `{"email":"ana@example.test","password":"`+password+`"}`, nil)
	cookie = res.Cookies()[0]
	store.expireAll()
	for _, path := range []string{"/v1/auth/me", "/v1/vehicles/auto-001"} {
		if code, _, _ := do("GET", path, "", cookie); code != 401 {
			t.Fatalf("%s con sesión vencida=%d", path, code)
		}
	}
}
