package main

import (
	"strings"
	"testing"
)

func TestClinicConfigurationRequiresProviderCredentialWhenEnabled(t *testing.T) {
	for _, key := range []string{"CLINIC_BASE_URL", "CLINIC_API_KEY", "CLINIC_ALLOW_HTTP", "CLINIC_TIMEOUT"} {
		t.Setenv(key, "")
	}
	if provider, err := configuredClinic(); err != nil || provider != nil {
		t.Fatal("disabled provider prevents local operation")
	}
	t.Setenv("CLINIC_BASE_URL", "https://hospital.example/v1")
	if _, err := configuredClinic(); err == nil {
		t.Fatal("provider enabled without its credential")
	}
	t.Setenv("CLINIC_API_KEY", "private-fixture-key")
	t.Setenv("CLINIC_ALLOW_HTTP", "false")
	t.Setenv("CLINIC_TIMEOUT", "2s")
	if provider, err := configuredClinic(); err != nil || provider == nil {
		t.Fatalf("valid configuration: %v", err)
	}
	for _, tc := range []struct{ key, value string }{{"CLINIC_ALLOW_HTTP", "invalid"}, {"CLINIC_TIMEOUT", "invalid"}, {"CLINIC_TIMEOUT", "10s"}, {"CLINIC_BASE_URL", "https://hospital.example/api/turnos"}} {
		t.Run(tc.key+tc.value, func(t *testing.T) {
			t.Setenv(tc.key, tc.value)
			if _, err := configuredClinic(); err == nil || strings.Contains(err.Error(), "private-fixture-key") {
				t.Fatal("invalid configuration accepted or credential leaked")
			}
		})
	}
}
