package dependencies

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestDependencyTimeoutAndInvalidBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer internal-test" {
			t.Error("service identity not forwarded")
		}
		<-r.Context().Done()
	}))
	defer server.Close()
	h := &HTTP{Client: &http.Client{}, CustomersURL: server.URL, Token: "internal-test"}
	start := time.Now()
	if _, err := h.Get(context.Background(), "cli-001"); err == nil {
		t.Fatal("timeout accepted")
	}
	if time.Since(start) > 1600*time.Millisecond {
		t.Fatal("one-second call budget exceeded")
	}
	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(`{"id":"cli-001","active":true}`)) }))
	defer bad.Close()
	h.CustomersURL = bad.URL
	if _, err := h.Get(context.Background(), "cli-001"); err == nil {
		t.Fatal("incomplete dependency response accepted")
	}
}
