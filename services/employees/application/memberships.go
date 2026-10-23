package application

import (
	"context"
	"github.com/Gerpeirone/Car-Cruds/services/employees/domain"
	"time"
)

// MembershipInbox is our persistence port. It does not prescribe a gym API.
type MembershipInbox interface {
	ReceiveMembership(context.Context, string, domain.MembershipSnapshot) (bool, error)
	MembershipReport(context.Context, string, domain.Page) (domain.MembershipReport, error)
}

func (s *Service) WithMembershipInbox(inbox MembershipInbox) *Service {
	s.memberships = inbox
	return s
}

func (s *Service) ReceiveMembership(ctx context.Context, source string, snapshot domain.MembershipSnapshot) (bool, error) {
	if !domain.ValidEvidenceKey(source) {
		return false, domain.ErrInvalidInput
	}
	if err := snapshot.Validate(); err != nil {
		return false, err
	}
	if s.memberships == nil {
		return false, domain.ErrNotConfigured
	}
	// PostgreSQL timestamps have microsecond precision. Canonicalize before
	// persistence and duplicate comparison, independent of producer timezone.
	snapshot.ObservedAt = snapshot.ObservedAt.UTC().Truncate(time.Microsecond)
	return s.memberships.ReceiveMembership(ctx, source, snapshot)
}

func (s *Service) MembershipReport(ctx context.Context, period string, page domain.Page) (domain.MembershipReport, error) {
	if !domain.ValidEvidenceKey(period) {
		return domain.MembershipReport{}, domain.ErrInvalidInput
	}
	if err := page.Validate(); err != nil {
		return domain.MembershipReport{}, err
	}
	if s.memberships == nil {
		return domain.MembershipReport{}, domain.ErrNotConfigured
	}
	return s.memberships.MembershipReport(ctx, period, page)
}
