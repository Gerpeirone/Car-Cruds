package application

import (
	"context"
	"strings"

	"github.com/Gerpeirone/Car-Cruds/services/employees/domain"
)

type Service struct {
	repository  Repository
	newID       func() string
	memberships MembershipInbox
}

func New(repository Repository, newID func() string) *Service {
	return &Service{repository: repository, newID: newID}
}

func (s *Service) ListEmployees(ctx context.Context, page domain.Page, activeOnly bool) ([]domain.Employee, int64, error) {
	if err := page.Validate(); err != nil {
		return nil, 0, err
	}
	return s.repository.ListEmployees(ctx, page, activeOnly)
}

func (s *Service) GetEmployee(ctx context.Context, id string) (domain.Employee, error) {
	if strings.TrimSpace(id) == "" {
		return domain.Employee{}, domain.ErrInvalidInput
	}
	return s.repository.GetEmployee(ctx, id)
}

func (s *Service) CreateEmployee(ctx context.Context, name string, active bool) (domain.Employee, error) {
	name, err := domain.NormalizeName(name)
	if err != nil {
		return domain.Employee{}, err
	}
	employee := domain.Employee{ID: s.newID(), DisplayName: name, Active: active}
	if err := s.repository.CreateEmployee(ctx, employee); err != nil {
		return domain.Employee{}, err
	}
	return employee, nil
}

func (s *Service) UpdateEmployee(ctx context.Context, id, name string, active bool) (domain.Employee, error) {
	name, err := domain.NormalizeName(name)
	if err != nil || strings.TrimSpace(id) == "" {
		return domain.Employee{}, domain.ErrInvalidInput
	}
	return s.repository.UpdateEmployee(ctx, domain.Employee{ID: id, DisplayName: name, Active: active})
}

func (s *Service) DeactivateEmployee(ctx context.Context, id string) (domain.Employee, error) {
	if strings.TrimSpace(id) == "" {
		return domain.Employee{}, domain.ErrInvalidInput
	}
	return s.repository.DeactivateEmployee(ctx, id)
}

func (s *Service) ListCandidates(ctx context.Context, page domain.Page) ([]domain.Candidate, int64, error) {
	if err := page.Validate(); err != nil {
		return nil, 0, err
	}
	return s.repository.ListCandidates(ctx, page)
}

func (s *Service) GetCandidate(ctx context.Context, id string) (domain.Candidate, error) {
	if strings.TrimSpace(id) == "" {
		return domain.Candidate{}, domain.ErrInvalidInput
	}
	return s.repository.GetCandidate(ctx, id)
}

func (s *Service) CreateCandidate(ctx context.Context, name string) (domain.Candidate, error) {
	name, err := domain.NormalizeName(name)
	if err != nil {
		return domain.Candidate{}, err
	}
	candidate := domain.Candidate{ID: s.newID(), DisplayName: name}
	if err := s.repository.CreateCandidate(ctx, candidate); err != nil {
		return domain.Candidate{}, err
	}
	return candidate, nil
}

func (s *Service) UpdateCandidate(ctx context.Context, id, name string) (domain.Candidate, error) {
	name, err := domain.NormalizeName(name)
	if err != nil || strings.TrimSpace(id) == "" {
		return domain.Candidate{}, domain.ErrInvalidInput
	}
	return s.repository.UpdateCandidate(ctx, domain.Candidate{ID: id, DisplayName: name})
}

func (s *Service) DeleteCandidate(ctx context.Context, id string) error {
	if strings.TrimSpace(id) == "" {
		return domain.ErrInvalidInput
	}
	return s.repository.DeleteCandidate(ctx, id)
}

// External acceptance and the hospital adapter are still pending. The gym
// inbox reports only evidence actually received, never assumed supplier facts.
func (s *Service) IntegrationStatus(context.Context) error { return domain.ErrNotConfigured }
