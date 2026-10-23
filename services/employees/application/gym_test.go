package application

import (
	"context"
	"errors"
	"testing"

	"github.com/Gerpeirone/Car-Cruds/services/employees/domain"
)

type memoryGymStore struct {
	item       domain.GymEnrollment
	prepareErr error
	claimed    bool
	persisted  bool
}

func (m *memoryGymStore) PrepareGym(_ context.Context, app, key, id, dni, email, newID string) (domain.GymEnrollment, error) {
	if m.prepareErr != nil {
		return domain.GymEnrollment{}, m.prepareErr
	}
	if m.persisted {
		if m.item.EmployeeID != id || m.item.DNI != dni || m.item.Email != email || m.item.Key != key {
			return domain.GymEnrollment{}, domain.ErrGymConflict
		}
		return m.item, nil
	}
	m.item = domain.GymEnrollment{ID: newID, EmployeeID: id, App: app, Key: key, DNI: dni, Email: email, Name: "Fictitious", State: "PENDING"}
	m.persisted = true
	return m.item, nil
}
func (m *memoryGymStore) GetGym(context.Context, string, string) (domain.GymEnrollment, error) {
	return m.item, nil
}
func (m *memoryGymStore) ClaimGym(context.Context, string, string) (bool, error) {
	return m.claimed, nil
}
func (m *memoryGymStore) CompleteGym(ctx context.Context, _, _ string, o domain.GymOutcome) (domain.GymEnrollment, error) {
	if ctx.Err() != nil {
		return domain.GymEnrollment{}, ctx.Err()
	}
	m.item.State = o.State
	m.item.ProviderCode = o.ProviderCode
	m.item.SocioID = o.SocioID
	m.item.SolicitudID = o.SolicitudID
	return m.item, nil
}

type fakeGym func(context.Context, domain.GymEnrollment) domain.GymOutcome

func (f fakeGym) Enroll(ctx context.Context, e domain.GymEnrollment) domain.GymOutcome {
	return f(ctx, e)
}

func TestGymIntentionMustBeDurableBeforeExternalEffect(t *testing.T) {
	store := &memoryGymStore{claimed: true}
	calls := 0
	s := New(nil, func() string { return "stable-id" }).WithGym(store, fakeGym(func(_ context.Context, e domain.GymEnrollment) domain.GymOutcome {
		calls++
		if !store.persisted {
			t.Fatal("HTTP before persistence")
		}
		return domain.GymOutcome{State: "ACCEPTED", SocioID: "socio", SolicitudID: "request"}
	}))
	store.prepareErr = errors.New("database down")
	if _, err := s.EnrollGym(context.Background(), "app", "employee", "key", "99900001", "person@example.invalid"); err == nil || calls != 0 {
		t.Fatal("sent without durable intention")
	}
	store.prepareErr = nil
	for i := 0; i < 2; i++ {
		result, err := s.EnrollGym(context.Background(), "app", "employee", "key", " 99900001 ", " PERSON@example.invalid ")
		if err != nil || result.State != "ACCEPTED" {
			t.Fatal(result, err)
		}
	}
	if calls != 1 {
		t.Fatal("replayed terminal result sent again", calls)
	}
	if _, err := s.EnrollGym(context.Background(), "app", "employee", "key", "different", "person@example.invalid"); !errors.Is(err, domain.ErrGymConflict) {
		t.Fatal("changed body accepted")
	}
}

func TestGymUnknownCanReplaySameSnapshotAndSurviveCallerCancellation(t *testing.T) {
	store := &memoryGymStore{claimed: true}
	calls := 0
	ctx, cancel := context.WithCancel(context.Background())
	s := New(nil, func() string { return "stable-id" }).WithGym(store, fakeGym(func(_ context.Context, e domain.GymEnrollment) domain.GymOutcome {
		calls++
		if calls == 1 {
			cancel()
			return domain.GymOutcome{State: "UNKNOWN", ProviderCode: "GYM_UNAVAILABLE"}
		}
		if e.ID != "stable-id" {
			t.Fatal("provider intention changed")
		}
		return domain.GymOutcome{State: "ACCEPTED", SocioID: "socio", SolicitudID: "request"}
	}))
	result, err := s.EnrollGym(ctx, "app", "employee", "key", "99900001", "person@example.invalid")
	if err != nil || result.State != "UNKNOWN" {
		t.Fatal("lost status when caller disconnected", result, err)
	}
	result, err = s.EnrollGym(context.Background(), "app", "employee", "key", "99900001", "person@example.invalid")
	if err != nil || result.State != "ACCEPTED" || calls != 2 {
		t.Fatal(result, err, calls)
	}
}

func TestGymConcurrentClaimAndInvalidContactDoNotSend(t *testing.T) {
	store := &memoryGymStore{claimed: false}
	s := New(nil, func() string { return "stable-id" }).WithGym(store, fakeGym(func(context.Context, domain.GymEnrollment) domain.GymOutcome {
		t.Fatal("sent without a lease")
		return domain.GymOutcome{}
	}))
	result, err := s.EnrollGym(context.Background(), "app", "employee", "key", "99900001", "person@example.invalid")
	if err != nil || result.State != "PENDING" {
		t.Fatal(result, err)
	}
	for _, email := range []string{"", "not-an-email", "Display Name <person@example.invalid>"} {
		if _, err := s.EnrollGym(context.Background(), "app", "employee", "key", "99900001", email); !errors.Is(err, domain.ErrInvalidInput) {
			t.Fatal("invalid email accepted")
		}
	}
}
