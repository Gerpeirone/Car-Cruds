package domain_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/Gerpeirone/Car-Cruds/services/employees/domain"
)

func TestClinicIdempotencyKeyMatchesPartnerContract(t *testing.T) {
	for _, key := range []string{"a", "8c1c6de1-860a-4a9a-9a1e-c41b775bf8ab", "ABC_123:one.two", strings.Repeat("x", 128)} {
		if !domain.ValidClinicIdempotencyKey(key) {
			t.Errorf("valid key rejected: %q", key)
		}
	}
	for _, key := range []string{"", "á", "a b", "a\r\nX:secret", "a/b", strings.Repeat("x", 129)} {
		if domain.ValidClinicIdempotencyKey(key) {
			t.Errorf("invalid key accepted: %q", key)
		}
	}
}

func TestClinicTechnicalFailureIsUnavailableAndNeverClinicalRejection(t *testing.T) {
	err := &domain.ClinicFailure{Code: "CLINIC_UNAUTHORIZED"}
	if !errors.Is(err, domain.ErrClinicUnavailable) {
		t.Fatal("technical failure lost unavailable classification")
	}
	var rejection *domain.ClinicRejection
	if errors.As(err, &rejection) {
		t.Fatal("access failure classified as clinical rejection")
	}
}
