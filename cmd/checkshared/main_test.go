package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDestinationRequiresExplicitOriginAndTransportSecurity(t *testing.T) {
	for _, raw := range []string{"", "http://example.com", "https://user:secret@example.com", "https://example.com/v1", "https://example.com?secret=x", "https://example.com?", "https://example.com/#token", "ftp://localhost"} {
		if _, err := validateURL(raw); err == nil {
			t.Errorf("accepted unsafe origin %q", raw)
		}
	}
	for _, raw := range []string{"https://example.com", "http://localhost:8080", "http://127.0.0.1:8080/", "http://[::1]:8080"} {
		if _, err := validateURL(raw); err != nil {
			t.Errorf("rejected valid origin %q", raw)
		}
	}
}

func TestContractRejectsExtraFieldsAndInconsistentPages(t *testing.T) {
	for _, body := range []string{
		`{"items":[{"employeeId":"1","displayName":"Demo","medical":"private"}],"page":1,"pageSize":1,"total":1}`,
		`{"items":[],"page":1,"pageSize":1,"total":1}`,
		`{"items":null,"page":1,"pageSize":1,"total":0}`,
		`{"items":[],"page":1,"pageSize":1}`,
		`{"items":[],"page":1,"pageSize":1,"total":0,"private":"data"}`,
		`{"items":[],"page":1,"pageSize":1,"total":0} {}`,
	} {
		if err := validatePage([]byte(body), 1, 1); err == nil {
			t.Error("invalid or excessive data accepted")
		}
	}
}

func TestVerificationOnlyReadsAndDetectsAuthorizationRegression(t *testing.T) {
	contract := filepath.Join(t.TempDir(), "employees-openapi.json")
	if err := os.WriteFile(contract, []byte("{\r\n  \"info\":{\"version\":\"0.4.0\"}\r\n}"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, scenario := range []string{"valid", "privilege leak", "stale contract", "inactive employee"} {
		calls := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			calls++
			if r.Method != http.MethodGet {
				t.Error("verification modified data")
			}
			if r.URL.Path == "/contracts/employees-openapi.json" {
				if r.Header.Get("Authorization") != "" {
					t.Error("credential sent to public contract")
				}
				version := "0.4.0"
				if scenario == "stale contract" {
					version = "0.3.0"
				}
				fmt.Fprintf(w, `{"info":{"version":%q}}`, version)
				return
			}
			if r.Header.Get("Authorization") != "Bearer test-gym" {
				w.WriteHeader(401)
				return
			}
			if r.URL.Path == "/v1/employees/opaque-id/status" {
				fmt.Fprintf(w, `{"employeeId":"opaque-id","active":%t}`, scenario != "inactive employee")
				return
			}
			if r.URL.Path != "/v1/employees" {
				if scenario == "privilege leak" {
					w.WriteHeader(200)
				} else if r.URL.Path == "/v1/candidates" {
					w.WriteHeader(404)
				} else {
					w.WriteHeader(403)
				}
				return
			}
			page := r.URL.Query().Get("page")
			if page == "0" {
				w.WriteHeader(400)
				return
			}
			items := `[{"employeeId":"opaque-id","displayName":"Fictitious"}]`
			if page == "2" {
				items = `[]`
			}
			fmt.Fprintf(w, `{"items":%s,"page":%s,"pageSize":1,"total":1}`, items, page)
		}))
		err := verify(context.Background(), server.URL, "test-gym", contract, "opaque-id")
		server.Close()
		if (err != nil) != (scenario != "valid") || calls == 0 {
			t.Fatalf("%s: %v", scenario, err)
		}
	}
}

func TestRedirectNeverForwardsCredentialToAnotherOrigin(t *testing.T) {
	leaked := false
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { leaked = true }))
	defer target.Close()
	source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "" || strings.Contains(r.Header.Get("Authorization"), "invalid-") {
			w.WriteHeader(401)
			return
		}
		http.Redirect(w, r, target.URL, 302)
	}))
	defer source.Close()
	if err := verify(context.Background(), source.URL, "test-gym", "", ""); err == nil || leaked {
		t.Fatal("redirect followed or reported as success")
	}
}
