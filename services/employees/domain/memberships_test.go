package domain_test

import (
	"errors"
	"github.com/Gerpeirone/Car-Cruds/services/employees/domain"
	"math"
	"testing"
	"time"
)

func value[T any](v T) *T { return &v }

func TestMembershipFactsRemainIndependentAndTotalsSeparateCurrencies(t *testing.T) {
	r := domain.MembershipReport{}
	for _, s := range []*domain.MembershipSnapshot{
		nil,
		{Member: value(true), Used: value(false), CostMinor: value(int64(100)), SpendingMinor: value(int64(0)), Currency: "ARS"},
		{Member: value(false), Used: value(true), CostMinor: value(int64(20)), Currency: "USD"},
		{CostMinor: value(int64(40)), Currency: "ARS"},
	} {
		if err := r.Accumulate(domain.MembershipRow{Evidence: s}); err != nil {
			t.Fatal(err)
		}
	}
	r.SortTotals()
	if r.Total != 4 || r.MissingEvidence != 1 || r.IncompleteEvidence != 3 {
		t.Fatalf("counts: %+v", r)
	}
	if len(r.Totals) != 2 || r.Totals[0].Currency != "ARS" || *r.Totals[0].CostMinor != 140 || *r.Totals[0].SpendingMinor != 0 || r.Totals[0].SpendingRecords != 1 || r.Totals[1].SpendingMinor != nil {
		t.Fatalf("totals: %+v", r.Totals)
	}
	if err := r.Accumulate(domain.MembershipRow{Evidence: &domain.MembershipSnapshot{CostMinor: value(int64(math.MaxInt64)), Currency: "ARS"}}); !errors.Is(err, domain.ErrReportOverflow) {
		t.Fatalf("overflow=%v", err)
	}
}

func TestMembershipSnapshotValidation(t *testing.T) {
	valid := domain.MembershipSnapshot{EmployeeID: "e1", Period: "periodo-acordado", SourceReference: "gym-ref", Version: 1, ObservedAt: time.Now()}
	if err := valid.Validate(); err != nil {
		t.Fatal(err)
	}
	for name, change := range map[string]func(*domain.MembershipSnapshot){
		"missing employee":       func(s *domain.MembershipSnapshot) { s.EmployeeID = "" },
		"ambiguous period":       func(s *domain.MembershipSnapshot) { s.Period = " periodo " },
		"control character":      func(s *domain.MembershipSnapshot) { s.SourceReference = "r\x00" },
		"zero version":           func(s *domain.MembershipSnapshot) { s.Version = 0 },
		"missing observation":    func(s *domain.MembershipSnapshot) { s.ObservedAt = time.Time{} },
		"negative cost":          func(s *domain.MembershipSnapshot) { s.CostMinor = value(int64(-1)); s.Currency = "ARS" },
		"money without currency": func(s *domain.MembershipSnapshot) { s.SpendingMinor = value(int64(0)) },
		"invalid currency":       func(s *domain.MembershipSnapshot) { s.Currency = "ars" },
	} {
		t.Run(name, func(t *testing.T) {
			s := valid
			change(&s)
			if !errors.Is(s.Validate(), domain.ErrInvalidInput) {
				t.Fatal("accepted invalid evidence")
			}
		})
	}
}
