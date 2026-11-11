package data

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Gerpeirone/Car-Cruds/services/customers/business"
	"github.com/Gerpeirone/Car-Cruds/services/customers/presentation"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestAccountsHTTPAndPersistence(t *testing.T) {
	pool, config := isolatedCustomers(t)
	ctx := context.Background()
	accounts := business.Accounts{Store: &Repository{Pool: pool}, TTL: time.Hour}
	h := presentation.NewAccountHandler(accounts)
	request := func(method, path, body string, cookie *http.Cookie, status int) *httptest.ResponseRecorder {
		t.Helper()
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		if cookie != nil {
			r.AddCookie(cookie)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != status {
			t.Fatalf("%s %s: HTTP %d; esperado %d; cuerpo %s", method, path, w.Code, status, w.Body)
		}
		return w
	}
	const password = "Frase ficticia para pruebas #☕"
	const valid = `{"firstName":"Ana","lastName":"Demo","email":" ANA@EXAMPLE.COM ","password":"` + password + `","passwordConfirmation":"` + password + `"}`
	for _, invalid := range []string{
		strings.Replace(valid, "ANA@EXAMPLE.COM", "sin correo", 1),
		strings.ReplaceAll(valid, password, "corta"),
		strings.Replace(valid, password, "confirmacion diferente", 1),
		strings.TrimSuffix(valid, "}") + `,"role":"admin"}`,
	} {
		request("POST", "/v1/auth/register", invalid, nil, 400)
	}
	initial := request("POST", "/v1/auth/register", valid, nil, 202)
	repeated := request("POST", "/v1/auth/register", strings.Replace(valid, "ANA@EXAMPLE.COM", "ana@example.com", 1), nil, 202)
	if initial.Body.String() != repeated.Body.String() {
		t.Fatal("duplicate email revealed by response")
	}
	request("GET", "/v1/auth/me", "", nil, 401)
	bad := request("POST", "/v1/auth/login", `{"email":"ana@example.com","password":"incorrecta"}`, nil, 401)
	unknown := request("POST", "/v1/auth/login", `{"email":"no-existe@example.com","password":"incorrecta"}`, nil, 401)
	if bad.Body.String() != unknown.Body.String() {
		t.Fatal("unknown email revealed by login")
	}
	login := request("POST", "/v1/auth/login", `{"email":"ana@example.com","password":"`+password+`"}`, nil, 200)
	cookie := login.Result().Cookies()[0]
	if !cookie.HttpOnly || !cookie.Secure || cookie.Path != "/" || cookie.Domain != "" ||
		cookie.SameSite != http.SameSiteStrictMode || !cookie.Expires.After(time.Now()) {
		t.Fatal("insecure session cookie")
	}
	var a business.Account
	if err := json.Unmarshal(login.Body.Bytes(), &a); err != nil || a.Email != "ana@example.com" || a.Role != "client" || a.FirstName != "Ana" ||
		strings.Contains(login.Body.String(), "password") {
		t.Fatalf("unexpected account response: %s", login.Body)
	}
	reopened, err := pgxpool.NewWithConfig(ctx, config.Copy())
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	h = presentation.NewAccountHandler(business.Accounts{Store: &Repository{Pool: reopened}, TTL: time.Hour})
	request("GET", "/v1/auth/me", "", cookie, 200)
	request("POST", "/v1/auth/logout", "{}", cookie, 204)
	request("GET", "/v1/auth/me", "", cookie, 401)
	login = request("POST", "/v1/auth/login", `{"email":"ana@example.com","password":"`+password+`"}`, nil, 200)
	cookie = login.Result().Cookies()[0]
	if _, err = pool.Exec(ctx, "UPDATE customer_sessions SET expires_at=now() - interval '1 second' WHERE account_id=$1", a.ID); err != nil {
		t.Fatal(err)
	}
	request("GET", "/v1/auth/me", "", cookie, 401)
	for range 8 {
		request("POST", "/v1/auth/login", `{"email":"limite@example.com","password":"incorrecta"}`, nil, 401)
	}
	request("POST", "/v1/auth/login", `{"email":"LIMITE@example.com","password":"incorrecta"}`, nil, 429)
}

func TestConcurrentRegistrationAndPasswordHash(t *testing.T) {
	pool, _ := isolatedCustomers(t)
	ctx := context.Background()
	accounts := business.Accounts{Store: &Repository{Pool: pool}}
	password := strings.Repeat("frase ✦ ", 70) + "terminación"
	var jobs sync.WaitGroup
	for range 6 {
		jobs.Go(func() {
			if err := accounts.Register(ctx, "Ana", "Demo", "CONCURRENTE@example.com", password, password); err != nil {
				t.Error(err)
			}
		})
	}
	jobs.Wait()
	var count int
	var hash string
	if err := pool.QueryRow(ctx, "SELECT count(*),min(password_hash) FROM customer_accounts WHERE email=$1", "concurrente@example.com").Scan(&count, &hash); err != nil {
		t.Fatal(err)
	}
	if count != 1 || !strings.HasPrefix(hash, "$argon2id$v=19$") || strings.Contains(hash, password) {
		t.Fatal("non-unique or unprotected account")
	}
	if err := accounts.Register(ctx, "Ana", "Demo", "otra@example.com", password, password); err != nil {
		t.Fatal(err)
	}
	var otherHash string
	if err := pool.QueryRow(ctx, "SELECT password_hash FROM customer_accounts WHERE email=$1", "otra@example.com").Scan(&otherHash); err != nil || otherHash == hash {
		t.Fatal("salt not unique")
	}
	accounts.TTL = time.Hour
	if _, _, _, err := accounts.Login(ctx, "concurrente@example.com", password); err != nil {
		t.Fatal("long password could not be verified")
	}
	if _, _, _, err := accounts.Login(ctx, "concurrente@example.com", password+"!"); err != business.ErrAccountAuth {
		t.Fatal("long password was truncated")
	}
}

// A repeated registration answers like a new one (no enumeration) but never
// replaces the stored account: the original password keeps working, and the
// address is normalized identically when registering and logging in.
func TestDuplicateRegistrationKeepsOriginalPasswordAndNormalizesLogin(t *testing.T) {
	pool, _ := isolatedCustomers(t)
	ctx := context.Background()
	accounts := business.Accounts{Store: &Repository{Pool: pool}, TTL: time.Hour}
	const original = "  Frase original con espacios  "
	const other = "Otra frase ficticia distinta"
	if err := accounts.Register(ctx, "Ana", "Demo", "Ana.Demo@Example.TEST", original, original); err != nil {
		t.Fatal(err)
	}
	var id, hash string
	if err := pool.QueryRow(ctx, "SELECT id::text, password_hash FROM customer_accounts WHERE email='ana.demo@example.test'").Scan(&id, &hash); err != nil {
		t.Fatal(err)
	}
	if err := accounts.Register(ctx, "Otra", "Persona", " ana.demo@example.test ", other, other); err != nil {
		t.Fatalf("duplicate registration must look accepted: %v", err)
	}
	var count int
	var afterID, afterHash, first string
	if err := pool.QueryRow(ctx, "SELECT count(*), min(id::text), min(password_hash), min(first_name) FROM customer_accounts WHERE email='ana.demo@example.test'").Scan(&count, &afterID, &afterHash, &first); err != nil {
		t.Fatal(err)
	}
	if count != 1 || afterID != id || afterHash != hash || first != "Ana" {
		t.Fatal("duplicate registration modified the existing account")
	}
	for _, email := range []string{"ana.demo@example.test", "  ANA.DEMO@EXAMPLE.TEST\t"} {
		if _, _, _, err := accounts.Login(ctx, email, original); err != nil {
			t.Fatalf("login %q with the original password: %v", email, err)
		}
	}
	for _, wrong := range []string{other, strings.TrimSpace(original), original + " "} {
		if _, _, _, err := accounts.Login(ctx, "ana.demo@example.test", wrong); err != business.ErrAccountAuth {
			t.Fatalf("password %q must not match: %v", wrong, err)
		}
	}
}
