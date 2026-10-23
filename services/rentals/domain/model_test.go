package domain

import (
	"math"
	"testing"
	"time"
)

func TestCalendarPeriodAndExclusiveLicense(t *testing.T) {
	now := time.Date(2026, 10, 16, 1, 0, 0, 0, time.UTC) // October 15 in Buenos Aires.
	p, err := NewPeriod("2026-10-15", "2026-10-18", now)
	if err != nil || p.Days != 3 {
		t.Fatalf("%+v %v", p, err)
	}
	if !(Customer{Active: true, LicenseValidUntil: "2026-10-17"}).Eligible(p) {
		t.Fatal("license includes last occupied day")
	}
	if (Customer{Active: true, LicenseValidUntil: "2026-10-16"}).Eligible(p) {
		t.Fatal("license does not cover full rental")
	}
	adjacent, _ := NewPeriod("2026-10-18", "2026-10-19", now)
	if p.Overlaps(adjacent) {
		t.Fatal("exclusive boundary overlapped")
	}
	for _, dates := range [][2]string{{"2026-10-14", "2026-10-18"}, {"2026-10-15", "2026-10-15"}, {"2026-10-15", "2026-11-15"}, {"2026-02-30", "2026-03-01"}, {"2026-10-15T00:00:00Z", "2026-10-18"}} {
		if _, err := NewPeriod(dates[0], dates[1], now); err == nil {
			t.Fatal("invalid period accepted", dates)
		}
	}
}
func TestQuoteOverflowAndStateTransitions(t *testing.T) {
	q := Quote{DailyRateMinor: 5000000, Currency: "ARS"}
	if value, err := q.Total(3); err != nil || value != 15000000 {
		t.Fatal(value, err)
	}
	q.DailyRateMinor = math.MaxInt64
	if _, err := q.Total(2); err == nil {
		t.Fatal("overflow accepted")
	}
	r := Reservation{Status: Reserved}
	v, p := r.Transition(Cancelled)
	if p != nil || v.Status != Cancelled {
		t.Fatal(v, p)
	}
	if _, p = v.Transition(Active); p == nil {
		t.Fatal("cancelled reactivated")
	}
	if _, p = v.Transition(Cancelled); p != nil {
		t.Fatal("cancel replay rejected")
	}
	r.Status = Active
	if _, p = r.Transition(Cancelled); p == nil {
		t.Fatal("active cancelled")
	}
}
