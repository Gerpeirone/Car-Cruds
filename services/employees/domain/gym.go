package domain

import (
	"errors"
	"net/mail"
	"strings"
	"unicode"
	"unicode/utf8"
)

var ErrGymConflict = errors.New("conflicto de solicitud de gimnasio")

// GymEnrollment is private administrative data. Never use it in the shared list.
type GymEnrollment struct {
	ID, EmployeeID, App, Key, Fingerprint     string
	DNI, Email, Name                          string
	State, ProviderCode, SocioID, SolicitudID string
}

type GymOutcome struct {
	State, ProviderCode, SocioID, SolicitudID string
}

func GymContact(dni, email string) (string, string, error) {
	dni, email = strings.TrimSpace(dni), strings.ToLower(strings.TrimSpace(email))
	if !GymText(dni, 20) || !GymText(email, 100) {
		return "", "", ErrInvalidInput
	}
	address, err := mail.ParseAddress(email)
	if err != nil || address.Address != email || address.Name != "" || !strings.Contains(email, "@") {
		return "", "", ErrInvalidInput
	}
	return dni, email, nil
}

func GymText(value string, max int) bool {
	if !utf8.ValidString(value) || strings.TrimSpace(value) == "" || utf8.RuneCountInString(value) > max {
		return false
	}
	for _, c := range value {
		if unicode.IsControl(c) {
			return false
		}
	}
	return true
}
