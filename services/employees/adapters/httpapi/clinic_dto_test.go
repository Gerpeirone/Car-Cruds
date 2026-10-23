package httpapi

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/Gerpeirone/Car-Cruds/services/employees/domain"
)

func TestClinicResponsePublishesCooldownAndKeepsRecoveryMetadataInternal(t *testing.T) {
	next := time.Date(2026, 12, 1, 10, 0, 0, 0, time.UTC)
	raw, err := json.Marshal(clinicDTO(domain.ClinicRequest{ID: "request", State: "unknown", FailureCode: "CLINIC_RATE_LIMITED", NextAttemptAt: &next, ProviderContract: domain.ClinicProviderContract, IdempotencyKey: "private-recovery-key"}))
	if err != nil {
		t.Fatal(err)
	}
	var dto map[string]json.RawMessage
	if err = json.Unmarshal(raw, &dto); err != nil {
		t.Fatal(err)
	}
	if string(dto["nextAttemptAt"]) != `"2026-12-01T10:00:00Z"` || string(dto["classification"]) != `"pending_verification"` || strings.Contains(string(raw), "private-recovery-key") || strings.Contains(string(raw), "ProviderContract") {
		t.Fatalf("invalid administrative response: %s", raw)
	}
}
