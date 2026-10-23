package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
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
	for _, leak := range []bool{false, true} {
		calls := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			calls++
			if r.Method != http.MethodGet {
				t.Error("verification modified data")
			}
			if r.Header.Get("Authorization") != "Bearer test-gym" {
				w.WriteHeader(401)
				return
			}
			if r.URL.Path != "/v1/employees" {
				if leak {
					w.WriteHeader(200)
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
		err := verify(context.Background(), server.URL, "test-gym")
		server.Close()
		if (err != nil) != leak || calls == 0 {
			t.Fatalf("leak=%v err=%v", leak, err)
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
	if err := verify(context.Background(), source.URL, "test-gym"); err == nil || leaked {
		t.Fatal("redirect followed or reported as success")
	}
}
