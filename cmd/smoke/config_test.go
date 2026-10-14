package main

import (
	"strings"
	"testing"
)

func TestGatewayConfigurationCannotSendLocalTokensElsewhere(t *testing.T) {
	for _, address := range []string{"https://example.org", "http://user:private-token@localhost:8080", "http://127.0.0.1:8080?token=private-token", "http://127.0.0.1:8080/v1/customers", "ftp://127.0.0.1:8080"} {
		_, err := localURL(address, "http", "https")
		if err == nil || strings.Contains(err.Error(), "private-token") {
			t.Fatal("gateway configuration must fail closed without revealing credentials")
		}
	}
	for _, address := range []string{"http://127.0.0.1:8080", "http://localhost:8080/", "http://[::1]:8080"} {
		if _, err := localURL(address, "http", "https"); err != nil {
			t.Fatal("valid loopback gateway rejected")
		}
	}
}

func TestHistoryConfigurationCannotOverrideValidatedLocalHost(t *testing.T) {
	t.Setenv("SMOKE_CUSTOMERS_DATABASE_URL", "postgres://user:private-token@127.0.0.1:55432/customers?host=example.org")
	if _, err := localDatabaseURL("customers"); err == nil || strings.Contains(err.Error(), "private-token") {
		t.Fatal("PostgreSQL host override must be rejected without credential disclosure")
	}
	t.Setenv("SMOKE_CUSTOMERS_DATABASE_URL", "")
	t.Setenv("CUSTOMERS_DB_PASSWORD", "password-with-special-characters@/:")
	if _, err := localDatabaseURL("customers"); err != nil {
		t.Fatal("escaped local database credentials rejected")
	}
}
