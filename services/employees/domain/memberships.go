package domain

import (
	"errors"
	"math"
	"sort"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

var ErrEvidenceConflict = errors.New("evidencia incompatible con la referencia o versión registrada")
var ErrReportOverflow = errors.New("total monetario fuera de rango")

// A snapshot replaces the previous evidence for one employee and period.
// Nil means unknown; false and zero are explicit facts supplied by the gym.
type MembershipSnapshot struct {
	EmployeeID, Period, SourceReference string
	Version                             int64
	ObservedAt                          time.Time
	Member, Used                        *bool
	CostMinor, SpendingMinor            *int64
	Currency                            string
}

func ValidEvidenceKey(value string) bool {
	if value == "" || strings.TrimSpace(value) != value || !utf8.ValidString(value) || utf8.RuneCountInString(value) > 100 {
		return false
	}
	for _, c := range value {
		if unicode.IsControl(c) {
			return false
		}
	}
	return true
}

func (s MembershipSnapshot) Validate() error {
	if !ValidEvidenceKey(s.EmployeeID) || !ValidEvidenceKey(s.Period) || !ValidEvidenceKey(s.SourceReference) || s.Version < 1 || s.ObservedAt.IsZero() || s.ObservedAt.Year() < 1 || s.ObservedAt.Year() > 9999 {
		return ErrInvalidInput
	}
	for _, amount := range []*int64{s.CostMinor, s.SpendingMinor} {
		if amount != nil && *amount < 0 {
			return ErrInvalidInput
		}
	}
	if s.CostMinor != nil || s.SpendingMinor != nil || s.Currency != "" {
		if len(s.Currency) != 3 {
			return ErrInvalidInput
		}
		for _, c := range s.Currency {
			if c < 'A' || c > 'Z' {
				return ErrInvalidInput
			}
		}
	}
	return nil
}

type MembershipRow struct {
	Employee  Employee
	Evidence  *MembershipSnapshot
	SourceApp string
}

type CurrencyTotal struct {
	Currency                     string
	CostMinor, SpendingMinor     *int64
	CostRecords, SpendingRecords int64
}

type MembershipReport struct {
	Period                                     string
	Page                                       Page
	Total, MissingEvidence, IncompleteEvidence int64
	Items                                      []MembershipRow
	Totals                                     []CurrencyTotal
}

// Accumulate counts facts independently and separates currencies. Absence is
// never converted to non-membership, non-use or zero expenditure.
func (r *MembershipReport) Accumulate(row MembershipRow) error {
	r.Total++
	if row.Evidence == nil {
		r.MissingEvidence++
		r.IncompleteEvidence++
		return nil
	}
	s := row.Evidence
	if s.Member == nil || s.Used == nil || s.CostMinor == nil || s.SpendingMinor == nil {
		r.IncompleteEvidence++
	}
	if s.CostMinor == nil && s.SpendingMinor == nil {
		return nil
	}
	i := -1
	for n := range r.Totals {
		if r.Totals[n].Currency == s.Currency {
			i = n
			break
		}
	}
	if i < 0 {
		r.Totals = append(r.Totals, CurrencyTotal{Currency: s.Currency})
		i = len(r.Totals) - 1
	}
	t := &r.Totals[i]
	add := func(total **int64, amount *int64, count *int64) error {
		if amount == nil {
			return nil
		}
		if *total == nil {
			*total = new(int64)
		}
		if **total > math.MaxInt64-*amount {
			return ErrReportOverflow
		}
		**total += *amount
		(*count)++
		return nil
	}
	if err := add(&t.CostMinor, s.CostMinor, &t.CostRecords); err != nil {
		return err
	}
	return add(&t.SpendingMinor, s.SpendingMinor, &t.SpendingRecords)
}

func (r *MembershipReport) SortTotals() {
	sort.Slice(r.Totals, func(i, j int) bool { return r.Totals[i].Currency < r.Totals[j].Currency })
}
