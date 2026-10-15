package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Gerpeirone/Car-Cruds/services/rentals/domain"
)

type testRepository struct {
	Repository
	lookup  func() (Result, bool, error)
	execute func(func(context.Context, UnitOfWork) (Result, error)) (Result, error)
}

func (r testRepository) LookupResult(context.Context, string, string, string) (Result, bool, error) {
	return r.lookup()
}
func (r testRepository) ExecuteOnce(_ context.Context, _, _, _ string, fn func(context.Context, UnitOfWork) (Result, error)) (Result, error) {
	return r.execute(fn)
}

type testUnit struct {
	reserve func(domain.Reservation) *domain.Problem
}

func (u testUnit) Reserve(_ context.Context, r domain.Reservation) *domain.Problem {
	return u.reserve(r)
}

type testCustomers func() (domain.Customer, error)

func (f testCustomers) Get(context.Context, string) (domain.Customer, error) { return f() }

type testFleet func() (domain.Quote, error)

func (f testFleet) Quote(context.Context, string) (domain.Quote, error) { return f() }

func request() Create {
	return Create{VehicleID: "auto-001", CustomerID: "cli-001", StartsOn: "2030-01-02", EndsOn: "2030-01-05", ExpectedTotalMinor: 300}
}

var actor = domain.Actor{App: "test-admin", Role: "admin"}

func TestCreateValidatesBeforeTransactionAndFreezesAuthoritativePrice(t *testing.T) {
	inside := false
	validations, writes := 0, 0
	s := &Service{Now: func() time.Time { return time.Date(2030, 1, 1, 12, 0, 0, 0, time.UTC) }, NewID: func() string { return "reservation" }}
	s.Repo = testRepository{
		lookup: func() (Result, bool, error) { return Result{}, false, nil },
		execute: func(fn func(context.Context, UnitOfWork) (Result, error)) (Result, error) {
			if validations != 2 {
				t.Fatal("transaction opened before remote validation")
			}
			inside = true
			defer func() { inside = false }()
			return fn(context.Background(), testUnit{reserve: func(r domain.Reservation) *domain.Problem {
				writes++
				if !inside || r.TotalMinor != 300 || r.DailyRateMinor != 100 {
					t.Fatal("incorrect atomic write or price")
				}
				return nil
			}})
		},
	}
	s.Customers = testCustomers(func() (domain.Customer, error) {
		if inside {
			t.Fatal("customer call retained transaction")
		}
		validations++
		return domain.Customer{Active: true, LicenseValidUntil: "2031-01-01"}, nil
	})
	s.Fleet = testFleet(func() (domain.Quote, error) {
		if inside {
			t.Fatal("fleet call retained transaction")
		}
		validations++
		return domain.Quote{Active: true, DailyRateMinor: 100, Currency: "ARS"}, nil
	})
	result, err := s.Create(context.Background(), actor, "key", request())
	if err != nil || result.Reservation == nil || writes != 1 {
		t.Fatal(result, err, writes)
	}
}

func TestCreateReturnsDurableResultBeforeRevalidation(t *testing.T) {
	for _, saved := range []Result{
		{Reservation: &domain.Reservation{ID: "original"}},
		{Problem: domain.Fail("IDEMPOTENCY_KEY_REUSED", "conflict")},
		{Problem: domain.Fail("PRICE_CHANGED", "original rejection")},
	} {
		s := &Service{Repo: testRepository{lookup: func() (Result, bool, error) { return saved, true, nil }}}
		// Nil clock/dependencies would panic if a replay were revalidated.
		result, err := s.Create(context.Background(), actor, "key", request())
		if err != nil || result != saved {
			t.Fatal("durable response changed", result, err)
		}
	}
}

func TestCreateTransientFailureDoesNotConsumeKeyAndObservesConcurrentCommit(t *testing.T) {
	for _, committed := range []bool{false, true} {
		reads := 0
		saved := Result{Reservation: &domain.Reservation{ID: "committed-by-other-request"}}
		s := &Service{
			Now:       func() time.Time { return time.Date(2030, 1, 1, 12, 0, 0, 0, time.UTC) },
			Customers: testCustomers(func() (domain.Customer, error) { return domain.Customer{}, errors.New("offline") }),
			Repo: testRepository{lookup: func() (Result, bool, error) {
				reads++
				if reads == 2 && committed {
					return saved, true, nil
				}
				return Result{}, false, nil
			}, execute: func(func(context.Context, UnitOfWork) (Result, error)) (Result, error) {
				t.Fatal("transient failure must not persist an idempotency result")
				return Result{}, nil
			}},
		}
		result, err := s.Create(context.Background(), actor, "key", request())
		if committed && (err != nil || result != saved) {
			t.Fatal("concurrent commit lost", result, err)
		}
		if !committed && (err == nil || result.Reservation != nil) {
			t.Fatal("transient failure confirmed")
		}
	}
}

func TestCreateRechecksIdempotencyAfterValidation(t *testing.T) {
	saved := Result{Reservation: &domain.Reservation{ID: "first-writer"}}
	s := &Service{
		Now: func() time.Time { return time.Date(2030, 1, 1, 12, 0, 0, 0, time.UTC) },
		Repo: testRepository{
			lookup:  func() (Result, bool, error) { return Result{}, false, nil },
			execute: func(func(context.Context, UnitOfWork) (Result, error)) (Result, error) { return saved, nil },
		},
	}
	invalid := request()
	invalid.StartsOn = "invalid"
	result, err := s.Create(context.Background(), actor, "key", invalid)
	if err != nil || result != saved {
		t.Fatal("first durable result must win", result, err)
	}
}
