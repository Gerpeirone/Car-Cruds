package domain

import (
	"errors"
	"time"
)

var ErrPatientLinkRequired = errors.New("falta vincular la persona con un paciente del hospital")
var ErrClinicUnavailable = errors.New("hospital no disponible o respuesta incompatible")

type PatientLink struct{ PersonKind, PersonID, PatientID, RegisteredBy string }

func ValidPersonKind(kind string) bool { return kind == "employee" || kind == "candidate" }

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
