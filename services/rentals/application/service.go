package application

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/Gerpeirone/Car-Cruds/services/rentals/domain"
)

type Create struct {
	VehicleID, CustomerID, StartsOn, EndsOn string
	ExpectedTotalMinor                      int64
}
type Result struct {
	Reservation *domain.Reservation
	Problem     *domain.Problem
}
type UnitOfWork interface {
	Reserve(context.Context, domain.Reservation) *domain.Problem
}
type Repository interface {
	LookupResult(context.Context, string, string, string) (Result, bool, error)
	ExecuteOnce(context.Context, string, string, string, func(context.Context, UnitOfWork) (Result, error)) (Result, error)
	Available(context.Context, string, domain.Period) (bool, error)
	Get(context.Context, string) (domain.Reservation, error)
	Transition(context.Context, domain.Actor, string, domain.Status) (domain.Reservation, error)
	Maintenance(context.Context, string, string, domain.Period) (string, error)
}
type Customers interface {
	Get(context.Context, string) (domain.Customer, error)
}
type Fleet interface {
	Quote(context.Context, string) (domain.Quote, error)
}

type Service struct {
	Repo      Repository
	Customers Customers
	Fleet     Fleet
	Now       func() time.Time
	NewID     func() string
}

func (s *Service) Create(ctx context.Context, actor domain.Actor, key string, input Create) (Result, error) {
	if !actor.CanCreate(input.CustomerID) {
		return Result{Problem: domain.Fail("FORBIDDEN", "No tiene permiso para reservar para ese cliente.")}, nil
	}
	// Canonical values include length prefixes so field boundaries cannot collide.
	canonical := fmt.Sprintf("%d:%s%d:%s%d:%s%d:%s%d", len(input.VehicleID), input.VehicleID, len(input.CustomerID), input.CustomerID, len(input.StartsOn), input.StartsOn, len(input.EndsOn), input.EndsOn, input.ExpectedTotalMinor)
	hash := sha256.Sum256([]byte(canonical))
	fingerprint := hex.EncodeToString(hash[:])
	if saved, found, err := s.Repo.LookupResult(ctx, actor.App, key, fingerprint); err != nil || found {
		return saved, err
	}
	// Remote validation holds neither a database connection nor an idempotency
	// lock. ExecuteOnce rechecks the key under lock before committing the result.
	prepared, err := s.prepare(ctx, actor, input)
	if err != nil {
		// A competing request may have committed while validation was in flight.
		// Its durable response takes precedence over a transient provider failure.
		if saved, found, lookupErr := s.Repo.LookupResult(ctx, actor.App, key, fingerprint); lookupErr == nil && found {
			return saved, nil
		}
		return Result{}, err
	}
	return s.Repo.ExecuteOnce(ctx, actor.App, key, fingerprint, func(ctx context.Context, tx UnitOfWork) (Result, error) {
		if prepared.Problem != nil {
			return prepared, nil
		}
		if problem := tx.Reserve(ctx, *prepared.Reservation); problem != nil {
			if problem.Code == "DEPENDENCY_UNAVAILABLE" {
				return Result{}, problem
			}
			return Result{Problem: problem}, nil
		}
		return prepared, nil
	})
}

func (s *Service) prepare(ctx context.Context, actor domain.Actor, input Create) (Result, error) {
	period, problem := domain.NewPeriod(input.StartsOn, input.EndsOn, s.Now())
	if problem != nil {
		return Result{Problem: problem}, nil
	}
	customer, err := s.Customers.Get(ctx, input.CustomerID)
	if err != nil {
		return dependency(err)
	}
	if !customer.Eligible(period) {
		return Result{Problem: domain.Fail("CUSTOMER_NOT_ELIGIBLE", "Cliente no habilitado durante el período.")}, nil
	}
	quote, err := s.Fleet.Quote(ctx, input.VehicleID)
	if err != nil {
		return dependency(err)
	}
	if !quote.Active {
		return Result{Problem: domain.Fail("VEHICLE_UNAVAILABLE", "Vehículo no habilitado administrativamente.")}, nil
	}
	total, problem := quote.Total(period.Days)
	if problem != nil {
		return Result{}, problem
	}
	if total != input.ExpectedTotalMinor {
		return Result{Problem: domain.Fail("PRICE_CHANGED", "El importe esperado cambió; consulte precio y use otra clave.")}, nil
	}
	reservation := domain.Reservation{ID: s.NewID(), OwnerApp: actor.App, VehicleID: input.VehicleID, CustomerID: input.CustomerID, Status: domain.Reserved, Period: period, DailyRateMinor: quote.DailyRateMinor, TotalMinor: total, Currency: "ARS"}
	return Result{Reservation: &reservation}, nil
}

func dependency(err error) (Result, error) {
	if p, ok := err.(*domain.Problem); ok && p.Code == "NOT_FOUND" {
		return Result{Problem: p}, nil
	}
	return Result{}, domain.Fail("DEPENDENCY_UNAVAILABLE", "Una dependencia no está disponible; no se confirmó la reserva.")
}

type Availability struct {
	VehicleID                  string
	Period                     domain.Period
	Available                  bool
	DailyRateMinor, TotalMinor int64
	Currency, Reason           string
}

func (s *Service) Availability(ctx context.Context, vehicleID, start, end string) (Availability, error) {
	period, p := domain.NewPeriod(start, end, s.Now())
	if p != nil {
		return Availability{}, p
	}
	quote, err := s.Fleet.Quote(ctx, vehicleID)
	if err != nil {
		return Availability{}, err
	}
	total, p := quote.Total(period.Days)
	if p != nil {
		return Availability{}, p
	}
	free, err := s.Repo.Available(ctx, vehicleID, period)
	if err != nil {
		return Availability{}, err
	}
	result := Availability{VehicleID: vehicleID, Period: period, Available: free && quote.Active, DailyRateMinor: quote.DailyRateMinor, TotalMinor: total, Currency: "ARS"}
	if !quote.Active {
		result.Reason = "VEHICLE_INACTIVE"
	} else if !free {
		result.Reason = "OVERLAPPING_RESERVATION"
	}
	return result, nil
}

func (s *Service) Get(ctx context.Context, actor domain.Actor, id string) (domain.Reservation, error) {
	r, err := s.Repo.Get(ctx, id)
	if err != nil {
		return r, err
	}
	if !actor.CanRead(r) {
		return domain.Reservation{}, domain.Fail("NOT_FOUND", "Reserva inexistente.")
	}
	return r, nil
}
func (s *Service) Cancel(ctx context.Context, actor domain.Actor, id string) (domain.Reservation, error) {
	return s.Repo.Transition(ctx, actor, id, domain.Cancelled)
}
func (s *Service) Pickup(ctx context.Context, actor domain.Actor, id string) (domain.Reservation, error) {
	r, err := s.Get(ctx, actor, id)
	if err != nil {
		return r, err
	}
	if r.Status == domain.Active {
		return r, nil
	}
	if r.Status != domain.Reserved {
		return r, domain.Fail("INVALID_STATE", "Solo se retira una reserva RESERVED.")
	}
	today := s.Now().In(domain.Branch).Format("2006-01-02")
	if today < r.Period.StartsOn || today >= r.Period.EndsOn {
		return r, domain.Fail("INVALID_STATE", "La reserva no está vigente para retiro.")
	}
	c, err := s.Customers.Get(ctx, r.CustomerID)
	if err != nil {
		return r, domain.Fail("DEPENDENCY_UNAVAILABLE", "No se pudo revalidar al conductor.")
	}
	if !c.Eligible(r.Period) {
		return r, domain.Fail("CUSTOMER_NOT_ELIGIBLE", "Conductor no habilitado.")
	}
	return s.Repo.Transition(ctx, actor, id, domain.Active)
}
func (s *Service) Return(ctx context.Context, actor domain.Actor, id string) (domain.Reservation, error) {
	return s.Repo.Transition(ctx, actor, id, domain.Completed)
}
