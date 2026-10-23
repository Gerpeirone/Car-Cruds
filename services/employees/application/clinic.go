package application

import (
	"context"
	"errors"
	"time"

	"github.com/Gerpeirone/Car-Cruds/services/employees/domain"
)

type Clinic interface {
	Book(context.Context, domain.ClinicRequest) (domain.ClinicAppointment, error)
	Lookup(context.Context, string) (domain.ClinicAppointment, error)
}
type ClinicStore interface {
	LinkClinicPatient(context.Context, domain.PatientLink) error
	CreateClinicRequest(context.Context, domain.ClinicRequest) (domain.ClinicRequest, error)
	GetClinicRequest(context.Context, string) (domain.ClinicRequest, error)
	ClaimClinicRequest(context.Context, string) (domain.ClinicRequest, bool, error)
	ObserveClinicAppointment(context.Context, string, domain.ClinicAppointment) (domain.ClinicRequest, error)
	FailClinicRequest(context.Context, string, int64, string, bool) (domain.ClinicRequest, error)
	ClinicReport(context.Context, string, domain.Page) ([]domain.ClinicPersonReview, int64, error)
}

func (s *Service) WithClinic(store ClinicStore, clinic Clinic) *Service {
	s.clinicStore = store
	s.clinic = clinic
	s.clinicSlots = make(chan struct{}, 8)
	return s
}
func (s *Service) LinkClinicPatient(ctx context.Context, link domain.PatientLink) error {
	if !domain.ValidPersonKind(link.PersonKind) || !domain.ValidEvidenceKey(link.PersonID) || !domain.ValidEvidenceKey(link.PatientID) || !domain.ValidEvidenceKey(link.RegisteredBy) {
		return domain.ErrInvalidInput
	}
	if s.clinicStore == nil {
		return domain.ErrNotConfigured
	}
	return s.clinicStore.LinkClinicPatient(ctx, link)
}
func (s *Service) CreateClinicRequest(ctx context.Context, r domain.ClinicRequest) (domain.ClinicRequest, error) {
	if err := r.ValidateInput(); err != nil {
		return r, err
	}
	if !domain.ValidEvidenceKey(r.RegisteredBy) {
		return r, domain.ErrInvalidInput
	}
	if s.clinicStore == nil {
		return r, domain.ErrNotConfigured
	}
	r.ID = s.newID()
	r.StartAt = r.StartAt.UTC()
	return s.clinicStore.CreateClinicRequest(ctx, r)
}
func (s *Service) GetClinicRequest(ctx context.Context, id string) (domain.ClinicRequest, error) {
	if !domain.ValidEvidenceKey(id) {
		return domain.ClinicRequest{}, domain.ErrInvalidInput
	}
	if s.clinicStore == nil {
		return domain.ClinicRequest{}, domain.ErrNotConfigured
	}
	return s.clinicStore.GetClinicRequest(ctx, id)
}
func (s *Service) SubmitClinicRequest(ctx context.Context, id string) (domain.ClinicRequest, error) {
	if !domain.ValidEvidenceKey(id) {
		return domain.ClinicRequest{}, domain.ErrInvalidInput
	}
	if s.clinicStore == nil || s.clinic == nil {
		return domain.ClinicRequest{}, domain.ErrNotConfigured
	}
	select {
	case s.clinicSlots <- struct{}{}:
		defer func() { <-s.clinicSlots }()
	default:
		return domain.ClinicRequest{}, domain.ErrClinicUnavailable
	}
	r, claimed, err := s.clinicStore.ClaimClinicRequest(ctx, id)
	if err != nil || !claimed {
		return r, err
	}
	// The durable claim is committed before the network call. A restart or
	// response loss cannot trigger a second POST to a non-idempotent provider.
	a, callErr := s.clinic.Book(ctx, r)
	saveCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
	defer cancel()
	if callErr != nil {
		var rejection *domain.ClinicRejection
		known := errors.As(callErr, &rejection)
		code := "UNCONFIRMED"
		if known {
			code = rejection.Code
		}
		return s.clinicStore.FailClinicRequest(saveCtx, id, r.Revision, code, known)
	}
	if err = a.Validate(); err != nil || a.PatientID != r.PatientID || a.DoctorID != r.DoctorID || !a.StartAt.Equal(r.StartAt) {
		return s.clinicStore.FailClinicRequest(saveCtx, id, r.Revision, "INVALID_RESPONSE", false)
	}
	return s.clinicStore.ObserveClinicAppointment(saveCtx, id, a)
}
func (s *Service) RefreshClinicRequest(ctx context.Context, id, recoveryID string) (domain.ClinicRequest, error) {
	r, err := s.GetClinicRequest(ctx, id)
	if err != nil {
		return r, err
	}
	if s.clinic == nil {
		return r, domain.ErrNotConfigured
	}
	if r.AttemptedAt == nil {
		return r, domain.ErrEvidenceConflict
	}
	if r.Appointment != nil {
		if recoveryID != "" && recoveryID != r.Appointment.ID {
			return r, domain.ErrEvidenceConflict
		}
		recoveryID = r.Appointment.ID
	}
	if !domain.ValidEvidenceKey(recoveryID) {
		return r, domain.ErrInvalidInput
	}
	select {
	case s.clinicSlots <- struct{}{}:
		defer func() { <-s.clinicSlots }()
	default:
		return r, domain.ErrClinicUnavailable
	}
	a, callErr := s.clinic.Lookup(ctx, recoveryID)
	saveCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
	defer cancel()
	if callErr != nil {
		return s.clinicStore.FailClinicRequest(saveCtx, id, r.Revision, "LOOKUP_UNCONFIRMED", false)
	}
	if err = a.Validate(); err != nil || a.ID != recoveryID || a.PatientID != r.PatientID {
		return s.clinicStore.FailClinicRequest(saveCtx, id, r.Revision, "INVALID_RESPONSE", false)
	}
	return s.clinicStore.ObserveClinicAppointment(saveCtx, id, a)
}
func (s *Service) ClinicReport(ctx context.Context, kind string, page domain.Page) ([]domain.ClinicPersonReview, int64, error) {
	if !domain.ValidPersonKind(kind) {
		return nil, 0, domain.ErrInvalidInput
	}
	if err := page.Validate(); err != nil {
		return nil, 0, err
	}
	if s.clinicStore == nil {
		return nil, 0, domain.ErrNotConfigured
	}
	return s.clinicStore.ClinicReport(ctx, kind, page)
}
