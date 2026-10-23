package application

import (
	"context"

	"github.com/Gerpeirone/Car-Cruds/services/employees/domain"
)

// Repository is an output port owned by the application. It contains no SQL or
// driver types; the adapter supplies persistence and snapshot pagination.
type Repository interface {
	ListEmployees(context.Context, domain.Page, bool) ([]domain.Employee, int64, error)
	GetEmployee(context.Context, string) (domain.Employee, error)
	CreateEmployee(context.Context, domain.Employee) error
	UpdateEmployee(context.Context, domain.Employee) (domain.Employee, error)
	DeactivateEmployee(context.Context, string) (domain.Employee, error)
	ListCandidates(context.Context, domain.Page) ([]domain.Candidate, int64, error)
	GetCandidate(context.Context, string) (domain.Candidate, error)
	CreateCandidate(context.Context, domain.Candidate) error
	UpdateCandidate(context.Context, domain.Candidate) (domain.Candidate, error)
	DeleteCandidate(context.Context, string) error
}
