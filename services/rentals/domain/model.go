// Package domain contains rental rules without transport or persistence types.
package domain

import (
	"fmt"
	"regexp"
	"time"
	_ "time/tzdata"
)

type Status string

const (
	Reserved  Status = "RESERVED"
	Active    Status = "ACTIVE"
	Completed Status = "COMPLETED"
	Cancelled Status = "CANCELLED"
)

type Problem struct{ Code, Message string }

func (p *Problem) Error() string         { return p.Code + ": " + p.Message }
func Fail(code, message string) *Problem { return &Problem{Code: code, Message: message} }

type Period struct {
	StartsOn, EndsOn string
	Days             int
}

var Identifier = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,63}$`)
var Branch, _ = time.LoadLocation("America/Buenos_Aires")

func NewPeriod(start, end string, now time.Time) (Period, *Problem) {
	s, e := parseDate(start), parseDate(end)
	if s.IsZero() || e.IsZero() {
		return Period{}, Fail("INVALID_INPUT", "Fechas válidas YYYY-MM-DD requeridas.")
	}
	today := now.In(Branch).Format("2006-01-02")
	if start < today {
		return Period{}, Fail("INVALID_INPUT", "El inicio no puede ser anterior al día actual de la sucursal.")
	}
	days := int(e.Sub(s) / (24 * time.Hour))
	if days < 1 || days > 30 {
		return Period{}, Fail("INVALID_INPUT", "El período [inicio, fin) debe durar entre 1 y 30 días.")
	}
	return Period{start, end, days}, nil
}

func parseDate(value string) time.Time {
	d, err := time.Parse("2006-01-02", value)
	if err != nil || d.Format("2006-01-02") != value {
		return time.Time{}
	}
	return d
}

func (p Period) Overlaps(other Period) bool {
	return p.StartsOn < other.EndsOn && other.StartsOn < p.EndsOn
}

type Customer struct {
	ID                string
	Active            bool
	LicenseValidUntil string
}

func (c Customer) Eligible(period Period) bool {
	// The last occupied day is endsOn - one calendar day.
	last := parseDate(period.EndsOn).AddDate(0, 0, -1).Format("2006-01-02")
	return c.Active && !parseDate(c.LicenseValidUntil).IsZero() && c.LicenseValidUntil >= last
}

type Quote struct {
	VehicleID      string
	Active         bool
	DailyRateMinor int64
	Currency       string
}

func (q Quote) Total(days int) (int64, *Problem) {
	if q.Currency != "ARS" || q.DailyRateMinor <= 0 || days < 1 || days > 30 || q.DailyRateMinor > (int64(1<<63-1)/int64(days)) {
		return 0, Fail("DEPENDENCY_UNAVAILABLE", "La tarifa autoritativa no es válida.")
	}
	return q.DailyRateMinor * int64(days), nil
}

type Reservation struct {
	ID, OwnerApp, VehicleID, CustomerID string
	Status                              Status
	Period                              Period
	DailyRateMinor, TotalMinor          int64
	Currency                            string
}

type Actor struct{ App, Role, CustomerID string }

func (a Actor) CanRead(r Reservation) bool {
	if a.Role == "admin" || a.Role == "service" {
		return true
	}
	return a.Role == "client" && a.App == r.OwnerApp && a.CustomerID == r.CustomerID
}
func (a Actor) CanCreate(customerID string) bool {
	return a.App != "" && (a.Role == "admin" || a.Role == "service" || (a.Role == "client" && a.CustomerID == customerID))
}

func (r Reservation) Transition(next Status) (Reservation, *Problem) {
	if r.Status == next {
		return r, nil
	}
	valid := (r.Status == Reserved && (next == Cancelled || next == Active)) || (r.Status == Active && next == Completed)
	if !valid {
		return r, Fail("INVALID_STATE", fmt.Sprintf("No se permite %s → %s.", r.Status, next))
	}
	r.Status = next
	return r, nil
}
