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
	clinicStore ClinicStore
	clinic      Clinic
	clinicSlots chan struct{}
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

// External acceptance is still pending. The clinic adapter and gym inbox report
// only evidence actually received, never assumed supplier facts.
func (s *Service) IntegrationStatus(context.Context) error { return domain.ErrNotConfigured }
