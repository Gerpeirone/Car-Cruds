package application

import (
	"context"
	"strings"
	"time"

	"github.com/Gerpeirone/Car-Cruds/services/employees/domain"
)

type Gym interface {
	Enroll(context.Context, domain.GymEnrollment) domain.GymOutcome
}
type GymStore interface {
	PrepareGym(context.Context, string, string, string, string, string, string) (domain.GymEnrollment, error)
	GetGym(context.Context, string, string) (domain.GymEnrollment, error)
	ClaimGym(context.Context, string, string) (bool, error)
	CompleteGym(context.Context, string, string, domain.GymOutcome) (domain.GymEnrollment, error)
}

func (s *Service) WithGym(store GymStore, gym Gym) *Service { s.gymStore, s.gym = store, gym; return s }

func (s *Service) EnrollGym(ctx context.Context, app, employeeID, key, dni, email string) (domain.GymEnrollment, error) {
	if s.gym == nil || s.gymStore == nil {
		return domain.GymEnrollment{}, domain.ErrNotConfigured
	}
	if !domain.GymText(key, 128) || strings.TrimSpace(employeeID) == "" {
		return domain.GymEnrollment{}, domain.ErrInvalidInput
	}
	dni, email, err := domain.GymContact(dni, email)
	if err != nil {
		return domain.GymEnrollment{}, err
	}
	item, err := s.gymStore.PrepareGym(ctx, app, key, employeeID, dni, email, s.newID())
	if err != nil || item.State == "ACCEPTED" || item.State == "REJECTED" {
		return item, err
	}
	lease := s.newID()
	claimed, err := s.gymStore.ClaimGym(ctx, item.ID, lease)
	if err != nil {
		return item, err
	}
	if !claimed {
		return s.gymStore.GetGym(ctx, employeeID, item.ID)
	}
	outcome := s.gym.Enroll(ctx, item)
	// Persist even if the caller disconnected after the provider accepted.
	finish, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
	defer cancel()
	return s.gymStore.CompleteGym(finish, item.ID, lease, outcome)
}

func (s *Service) GetGym(ctx context.Context, employeeID, id string) (domain.GymEnrollment, error) {
	if s.gymStore == nil {
		return domain.GymEnrollment{}, domain.ErrNotConfigured
	}
	return s.gymStore.GetGym(ctx, employeeID, id)
}
