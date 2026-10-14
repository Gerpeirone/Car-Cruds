package domain

import (
	"errors"
	"strings"
	"unicode"
	"unicode/utf8"
)

var (
	ErrInvalidInput  = errors.New("datos de personal inválidos")
	ErrNotFound      = errors.New("registro de personal inexistente")
	ErrNotConfigured = errors.New("integración pendiente de contrato y configuración")
)

// Employee and Candidate have different identities and lifecycles. Registering
// a candidate never grants employee status or adds that person to the gym list.
type Employee struct {
	ID          string
	DisplayName string
	Active      bool
}

type Candidate struct {
	ID          string
	DisplayName string
}

type Page struct {
	Number int
	Size   int
}

func (p Page) Validate() error {
	if p.Number < 1 || p.Size < 1 || p.Size > 100 {
		return ErrInvalidInput
	}
	// PostgreSQL uses a signed 64-bit OFFSET. Check before multiplication.
	if int64(p.Number-1) > (1<<63-1)/int64(p.Size) {
		return ErrInvalidInput
	}
	return nil
}

func (p Page) Offset() int64 { return int64(p.Number-1) * int64(p.Size) }

func NormalizeName(name string) (string, error) {
	if !utf8.ValidString(name) || utf8.RuneCountInString(name) > 150 {
		return "", ErrInvalidInput
	}
	for _, char := range name {
		if unicode.IsControl(char) {
			return "", ErrInvalidInput
		}
	}
	name = strings.TrimSpace(name)
	if length := utf8.RuneCountInString(name); length < 1 || length > 150 {
		return "", ErrInvalidInput
	}
	return name, nil
}
