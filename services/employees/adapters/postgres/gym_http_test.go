package postgres_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Gerpeirone/Car-Cruds/internal/platform"
	"github.com/Gerpeirone/Car-Cruds/services/employees/adapters/httpapi"
	"github.com/Gerpeirone/Car-Cruds/services/employees/adapters/postgres"
	"github.com/Gerpeirone/Car-Cruds/services/employees/application"
)

func TestGymHTTPContractWithPostgres(t *testing.T) {
	pool, _ := isolatedDatabase(t)
	repo := postgres.New(pool)
	if err := repo.SeedDemo(context.Background()); err != nil {
		t.Fatal(err)
	}
	const gym = "fictitious-gym-token-for-test"
	const admin = "fictitious-admin-token-for-test"
	t.Setenv("AUTH_GYM_TOKEN", gym)
	t.Setenv("AUTH_ADMIN_TOKEN", admin)
	t.Setenv("AUTH_CLIENT_TOKEN", "")
	t.Setenv("AUTH_SERVICE_TOKEN", "fictitious-service-token-for-test")
	t.Setenv("AUTH_GYM_TOKEN_EXPIRES_AT", time.Now().Add(time.Hour).UTC().Format(time.RFC3339))

	newServer := func() *httptest.Server {
		tokens, err := platform.TokensFromEnv()
		if err != nil {
			t.Fatal(err)
		}
		service := application.New(repo, platform.UUID).WithMembershipInbox(repo)
		server := httptest.NewServer(platform.AuthMiddleware(httpapi.New(service), tokens))
		t.Cleanup(server.Close)
		return server
	}
	server := newServer()
	client := &http.Client{Timeout: 5 * time.Second}
	defer client.CloseIdleConnections()
	request := func(base, method, path, token string, body []byte) (int, []byte, error) {
		req, err := http.NewRequest(method, base+path, bytes.NewReader(body))
		if err != nil {
			return 0, nil, err
		}
		req.Header.Set("Content-Type", "application/json")
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		resp, err := client.Do(req)
		if err != nil {
			return 0, nil, err
		}
		defer resp.Body.Close()
		data, err := io.ReadAll(resp.Body)
		return resp.StatusCode, data, err
	}
	assert := func(base, method, path, token string, body []byte, want int) []byte {
		t.Helper()
		status, data, err := request(base, method, path, token, body)
		if err != nil || status != want {
			t.Fatalf("%s %s: HTTP %d, expected %d, body %s, error %v", method, path, status, want, data, err)
		}
		return data
	}
	const listPath = "/v1/employees"
	const statusPath = "/v1/employees/employee-001/status"
	const evidencePath = "/v1/employees/membership-evidence"
	const reportPath = "/v1/employees/reports/memberships?period=2026-10"
	assert(server.URL, "GET", listPath, "", nil, 401)
	assert(server.URL, "GET", listPath, "invalid", nil, 401)
	assert(server.URL, "GET", reportPath, gym, nil, 403)
	assert(server.URL, "POST", evidencePath, admin, nil, 403)
	list := assert(server.URL, "GET", listPath, gym, nil, 200)
	if !bytes.Contains(list, []byte(`"employeeId":"employee-001"`)) {
		t.Fatalf("fictitious employee absent: %s", list)
	}
	status := assert(server.URL, "GET", statusPath, gym, nil, 200)
	if string(bytes.TrimSpace(status)) != `{"employeeId":"employee-001","active":true}` {
		t.Fatalf("unexpected status: %s", status)
	}

	raw, err := os.ReadFile("../../../../docs/contracts/employees-openapi.json")
	if err != nil {
		t.Fatal(err)
	}
	var contract struct {
		Info  struct{ Version string }
		Paths map[string]struct {
			Post struct {
				RequestBody struct {
					Content map[string]struct{ Example json.RawMessage }
				}
			}
		}
	}
	if err := json.Unmarshal(raw, &contract); err != nil || contract.Info.Version != "0.4.0" {
		t.Fatalf("unexpected contract version: %q, %v", contract.Info.Version, err)
	}
	evidence := contract.Paths[evidencePath].Post.RequestBody.Content["application/json"].Example
	assert(server.URL, "POST", evidencePath, gym, []byte(`{}`), 400)
	assert(server.URL, "POST", evidencePath, gym, bytes.Replace(evidence, []byte("employee-001"), []byte("unknown-001"), 1), 404)

	type result struct {
		code int
		body []byte
		err  error
	}
	const retries = 8
	results := make(chan result, retries)
	for range retries {
		go func() {
			code, body, err := request(server.URL, "POST", evidencePath, gym, evidence)
			results <- result{code, body, err}
		}()
	}
	applied := 0
	for range retries {
		response := <-results
		switch string(bytes.TrimSpace(response.body)) {
		case `{"applied":true}`:
			applied++
		case `{"applied":false}`:
		default:
			t.Errorf("unexpected receipt: %s", response.body)
		}
		if response.err != nil || response.code != 200 {
			t.Errorf("HTTP %d, error %v", response.code, response.err)
		}
	}
	if applied != 1 {
		t.Fatalf("expected one applied snapshot, got %d", applied)
	}
	var snapshot map[string]json.RawMessage
	if err := json.Unmarshal(evidence, &snapshot); err != nil {
		t.Fatal(err)
	}
	snapshot["member"] = json.RawMessage(`false`)
	changed, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	assert(server.URL, "POST", evidencePath, gym, changed, 409)
	receipt := assert(server.URL, "POST", evidencePath, gym, evidence, 200)
	if !strings.Contains(string(receipt), `"applied":false`) {
		t.Fatalf("retry applied again: %s", receipt)
	}
	var count int
	if err := pool.QueryRow(context.Background(), "SELECT count(*) FROM membership_evidence_history").Scan(&count); err != nil || count != 1 {
		t.Fatalf("history rows=%d, err=%v", count, err)
	}
	report := assert(server.URL, "GET", reportPath, admin, nil, 200)
	if !bytes.Contains(report, []byte(`"sourceApp":"gimnasio-pablo"`)) {
		t.Fatalf("report missing authenticated provenance: %s", report)
	}

	t.Setenv("AUTH_GYM_TOKEN_EXPIRES_AT", "2020-01-01T00:00:00Z")
	expired := newServer()
	assert(expired.URL, "GET", listPath, gym, nil, 401)
	assert(expired.URL, "GET", reportPath, admin, nil, 200)
	pool.Close()
	assert(server.URL, "POST", evidencePath, gym, evidence, 503)
}
