package domain

import (
	"errors"
	"regexp"
	"time"
)

var ErrPatientLinkRequired = errors.New("falta vincular la persona con un paciente del hospital")
var ErrClinicUnavailable = errors.New("hospital no disponible o respuesta incompatible")

const ClinicProviderContract = "turnos-partner-v1"

var clinicIdempotencyKey = regexp.MustCompile(`^[A-Za-z0-9._:-]{1,128}$`)

func ValidClinicIdempotencyKey(key string) bool { return clinicIdempotencyKey.MatchString(key) }

type PatientLink struct{ PersonKind, PersonID, PatientID, RegisteredBy string }

// The legacy personKind field remains for employee request compatibility.
func ValidPersonKind(kind string) bool { return kind == "employee" }

type ClinicAppointment struct {
	ID, PatientID, DoctorID, Status string
	StartAt, EndAt                  time.Time
	Version                         int64
}

func (a ClinicAppointment) Validate() error {
	if !ValidEvidenceKey(a.ID) || !ValidEvidenceKey(a.PatientID) || !ValidEvidenceKey(a.DoctorID) || a.Version < 1 || a.StartAt.IsZero() || !a.StartAt.Before(a.EndAt) || (a.Status != "booked" && a.Status != "cancelled") {
		return ErrClinicUnavailable
	}
	return nil
}

// State is administrative: booking does not prove attendance or fitness.
type ClinicRequest struct {
	ID, LocalReference, PersonKind, PersonID, PatientID, DoctorID, RegisteredBy string
	StartAt, CreatedAt                                                          time.Time
	AttemptedAt, ObservedAt                                                     *time.Time
	State, FailureCode                                                          string
	ProviderContract, IdempotencyKey                                            string
	NextAttemptAt                                                               *time.Time
	Revision                                                                    int64
	Appointment                                                                 *ClinicAppointment
}

func (r ClinicRequest) ValidateInput() error {
	if !ValidPersonKind(r.PersonKind) || !ValidEvidenceKey(r.PersonID) || !ValidEvidenceKey(r.LocalReference) || !ValidEvidenceKey(r.DoctorID) || r.StartAt.IsZero() || r.StartAt.Year() < 1 || r.StartAt.Year() > 9999 || r.StartAt.Nanosecond() != 0 {
		return ErrInvalidInput
	}
	return nil
}
func (r ClinicRequest) Classification() string {
	switch r.State {
	case "booked":
		return "confirmed"
	case "cancelled", "rejected":
		return "without_confirmed_appointment"
	default:
		return "pending_verification"
	}
}

type ClinicPersonReview struct {
	PersonKind, PersonID, DisplayName string
	Active                            *bool
	Request                           *ClinicRequest
}

// A provider rejection is explicit evidence that this creation was refused;
// network failures and invalid replies never become this error.
type ClinicRejection struct{ Code string }

func (e *ClinicRejection) Error() string { return "solicitud rechazada por el hospital: " + e.Code }

// Failure identifies a technical or access problem, never a clinical refusal.
// RetryAfter is advisory evidence from the provider's rate limit response.
type ClinicFailure struct {
	Code       string
	RetryAfter time.Duration
}

func (e *ClinicFailure) Error() string { return "integración con el hospital: " + e.Code }
func (e *ClinicFailure) Unwrap() error { return ErrClinicUnavailable }

// ClinicDoctor is the provider catalog entry used to choose a valid doctor.
type ClinicDoctor struct {
	ID              string
	DurationMinutes int
}

func (d ClinicDoctor) Validate() error {
	if !ValidEvidenceKey(d.ID) || d.DurationMinutes < 1 {
		return ErrClinicUnavailable
	}
	return nil
}

var clinicPeriod = regexp.MustCompile(`^\d{4}-(0[1-9]|1[0-2])$`)

// ValidClinicPeriod accepts the provider billing month AAAA-MM (UTC).
func ValidClinicPeriod(period string) bool { return clinicPeriod.MatchString(period) }

// ClinicUsage is the provider billing statement for Car Cruds in one UTC
// month. Amounts are integer minor units; a cancellation is still billed.
type ClinicUsage struct {
	Period       string
	From, To     time.Time
	CurrentPrice ClinicMoney
	Totals       []ClinicUsageTotal
	Items        []ClinicUsageItem
}
type ClinicMoney struct {
	AmountMinor int64
	Currency    string
}
type ClinicUsageTotal struct {
	Currency              string
	Bookings, AmountMinor int64
}
type ClinicUsageItem struct {
	AppointmentID, Currency, Status string
	BookedAt                        time.Time
	PriceMinor                      int64
}

var currencyCode = regexp.MustCompile(`^[A-Z]{3}$`)

func (u ClinicUsage) Validate(period string) error {
	if u.Period != period || u.From.IsZero() || !u.From.Before(u.To) || u.CurrentPrice.AmountMinor < 0 || !currencyCode.MatchString(u.CurrentPrice.Currency) {
		return ErrClinicUnavailable
	}
	for _, t := range u.Totals {
		if !currencyCode.MatchString(t.Currency) || t.Bookings < 0 || t.AmountMinor < 0 {
			return ErrClinicUnavailable
		}
	}
	for _, i := range u.Items {
		if !ValidEvidenceKey(i.AppointmentID) || !currencyCode.MatchString(i.Currency) || i.PriceMinor < 0 || i.BookedAt.IsZero() || (i.Status != "booked" && i.Status != "cancelled") {
			return ErrClinicUnavailable
		}
	}
	return nil
}
