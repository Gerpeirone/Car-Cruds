package application

import (
	"context"
	"time"

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

// Clinic is a local business port, not the hospital's API contract. Its adapter
// remains absent until the provider publishes the actual operations and rules.
// An implementation must persist an intention before a request with effects.
type Clinic interface {
	RequestReview(context.Context, ReviewIntention) (ReviewEvidence, error)
	LookupReview(context.Context, string) (ReviewEvidence, error)
}

type ReviewIntention struct {
	LocalReference string
	PersonID       string
	PersonKind     string
}

type ReviewEvidence struct {
	ExternalReference string
	Confirmed         bool
	Known             bool
	ObservedAt        time.Time
}

// Memberships expresses the evidence needed by our administrative report. It
// does not choose polling, notifications or events for the gym integration.
// No adapter is supplied until identity, period, use and costs are agreed.
type Memberships interface {
	EvidenceFor(context.Context, string, string) (MembershipEvidence, error)
}

type MembershipEvidence struct {
	EmployeeID      string
	Period          string
	SourceReference string
	ObservedAt      time.Time
	Member          *bool
	Used            *bool
	CostMinor       *int64
	Currency        string
}
