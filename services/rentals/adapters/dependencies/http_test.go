package dependencies

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
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

func TestCustomerResponseRequiresOneBoundedObjectAndAllowsAdditionalFields(t *testing.T) {
	const valid = `{"id":"cli-001","name":"Nombre adicional legítimo","active":true,"licenseValidUntil":"2030-12-31"}`
	const limit = 1 << 20
	for _, test := range []struct {
		name, body string
		accept     bool
	}{
		{"additional-fields", valid, true},
		{"whitespace", valid + " \n\t", true},
		{"exact-limit", valid + strings.Repeat(" ", limit-len(valid)), true},
		{"one-byte-over-limit", valid + strings.Repeat(" ", limit+1-len(valid)), false},
		{"large-valid-extra-field", `{"id":"cli-001","active":true,"licenseValidUntil":"2030-12-31","name":"` + strings.Repeat("a", limit) + `"}`, false},
		{"trailing-garbage", valid + " invalid", false},
		{"second-object", valid + ` {"id":"another"}`, false},
		{"second-null", valid + " null", false},
		{"null", "null", false},
		{"array", "[" + valid + "]", false},
		{"string", `"value"`, false},
		{"empty", "", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(test.body))
			}))
			defer server.Close()
			client := &HTTP{Client: server.Client(), CustomersURL: server.URL}
			customer, err := client.Get(context.Background(), "cli-001")
			if test.accept {
				if err != nil || customer.ID != "cli-001" || !customer.Active {
					t.Fatal("valid additive response rejected", customer, err)
				}
			} else if err == nil {
				t.Fatal("invalid dependency response accepted")
			}
		})
	}
}

func TestQuoteRejectsTrailingPayloadAndKeepsAdditiveCompatibility(t *testing.T) {
	const valid = `{"vehicleId":"auto-001","active":true,"dailyRateMinor":5000000,"currency":"ARS","version":2,"name":"Auto"}`
	for _, suffix := range []string{"", ` {"currency":"USD"}`, " broken"} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(valid + suffix)) }))
		client := &HTTP{Client: server.Client(), FleetURL: server.URL}
		quote, err := client.Quote(context.Background(), "auto-001")
		server.Close()
		if suffix == "" {
			if err != nil || quote.DailyRateMinor != 5000000 || quote.Currency != "ARS" {
				t.Fatal(quote, err)
			}
		} else if err == nil {
			t.Fatal("quote accepted trailing payload")
		}
	}
}
