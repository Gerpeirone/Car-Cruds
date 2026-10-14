package postgres

import (
	"context"
	"errors"
	"github.com/Gerpeirone/Car-Cruds/internal/platform"
	"github.com/Gerpeirone/Car-Cruds/services/rentals/application"
	"github.com/Gerpeirone/Car-Cruds/services/rentals/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

type fixtureDeps struct{ unavailable bool }

func (d *fixtureDeps) Get(context.Context, string) (domain.Customer, error) {
	if d.unavailable {
		return domain.Customer{}, errors.New("offline")
	}
	return domain.Customer{ID: "cli-001", Active: true, LicenseValidUntil: "2099-12-31"}, nil
}
func (d *fixtureDeps) Quote(context.Context, string) (domain.Quote, error) {
	if d.unavailable {
		return domain.Quote{}, errors.New("offline")
	}
	return domain.Quote{VehicleID: "auto-001", Active: true, DailyRateMinor: 5000000, Currency: "ARS"}, nil
}
func testPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	connection := os.Getenv("TEST_RENTALS_DATABASE_URL")
	if connection == "" {
		connection = os.Getenv("TEST_DATABASE_URL")
	}
	if connection == "" {
		t.Skip("PostgreSQL real requerido: TEST_RENTALS_DATABASE_URL o TEST_DATABASE_URL")
	}
	ctx := context.Background()
	admin, err := pgxpool.New(ctx, connection)
	if err != nil {
		t.Fatal(err)
	}
	if err = admin.Ping(ctx); err != nil {
		admin.Close()
		t.Fatal("PostgreSQL real no disponible")
	}
	schema := "test_rentals_" + strings.ReplaceAll(platform.UUID(), "-", "")
	quoted := pgx.Identifier{schema}.Sanitize()
	if _, err = admin.Exec(ctx, "CREATE SCHEMA "+quoted); err != nil {
		admin.Close()
		t.Fatal(err)
	}
	config, err := pgxpool.ParseConfig(connection)
	if err != nil {
		t.Fatal(err)
	}
	config.ConnConfig.RuntimeParams["search_path"] = schema + ",public"
	config.MaxConns = 24
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		pool.Close()
		_, _ = admin.Exec(context.Background(), "DROP SCHEMA "+quoted+" CASCADE")
		admin.Close()
	})
	if err = Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	if err = Migrate(ctx, pool); err != nil {
		t.Fatal("migration not idempotent", err)
	}
	return pool
}
func service(pool *pgxpool.Pool, deps *fixtureDeps) *application.Service {
	return &application.Service{Repo: &Repository{Pool: pool, NewID: platform.UUID}, Customers: deps, Fleet: deps, Now: time.Now, NewID: platform.UUID}
}
func input() application.Create {
	today := time.Now().In(domain.Branch)
	return application.Create{VehicleID: "auto-001", CustomerID: "cli-001", StartsOn: today.AddDate(0, 0, 1).Format("2006-01-02"), EndsOn: today.AddDate(0, 0, 4).Format("2006-01-02"), ExpectedTotalMinor: 15000000}
}

var adminActor = domain.Actor{App: "integration-admin", Role: "admin"}

func TestRealPostgresConcurrentExclusionAndDurableReplay(t *testing.T) {
	pool := testPool(t)
	deps := &fixtureDeps{}
	services := []*application.Service{service(pool, deps), service(pool, deps)}
	ctx := context.Background()
	request := input()
	const count = 16
	start := make(chan struct{})
	results := make(chan application.Result, count)
	failures := make(chan error, count)
	var wg sync.WaitGroup
	for i := 0; i < count; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			<-start
			result, err := services[index%2].Create(ctx, adminActor, platform.UUID(), request)
			if err != nil {
				failures <- err
			} else {
				results <- result
			}
		}(i)
	}
	close(start)
	wg.Wait()
	close(results)
	close(failures)
	for err := range failures {
		t.Error(err)
	}
	winners, conflicts := 0, 0
	for result := range results {
		if result.Reservation != nil {
			winners++
		} else if result.Problem != nil && result.Problem.Code == "VEHICLE_UNAVAILABLE" {
			conflicts++
		} else {
			t.Errorf("unexpected result %+v", result)
		}
	}
	if winners != 1 || conflicts != count-1 {
		t.Fatalf("winners=%d conflicts=%d", winners, conflicts)
	}
	var reservations, keys, events int
	pool.QueryRow(ctx, "SELECT count(*) FROM rental_reservations").Scan(&reservations)
	pool.QueryRow(ctx, "SELECT count(*) FROM rental_idempotency").Scan(&keys)
	pool.QueryRow(ctx, "SELECT count(*) FROM rental_outbox").Scan(&events)
	if reservations != 1 || keys != count || events != 0 {
		t.Fatalf("reservation=%d keys=%d outbox=%d", reservations, keys, events)
	}
	// New service object models a process restart, backed by the actual database.
	var id string
	if err := pool.QueryRow(ctx, "SELECT id FROM rental_reservations").Scan(&id); err != nil {
		t.Fatal(err)
	}
	if _, err := services[0].Cancel(ctx, adminActor, id); err != nil {
		t.Fatal(err)
	}
	first, err := services[0].Create(ctx, adminActor, "durable", request)
	if err != nil || first.Reservation == nil {
		t.Fatal(first, err)
	}
	if _, err = services[0].Cancel(ctx, adminActor, first.Reservation.ID); err != nil {
		t.Fatal(err)
	}
	restarted := service(pool, &fixtureDeps{unavailable: true})
	restarted.Now = func() time.Time { return time.Now().AddDate(0, 1, 0) }
	replay, err := restarted.Create(ctx, adminActor, "durable", request)
	if err != nil || replay.Reservation == nil || replay.Reservation.ID != first.Reservation.ID || replay.Reservation.Status != domain.Reserved {
		t.Fatal("replay revalidated dependencies or current state", replay, err)
	}
	current, err := restarted.Get(ctx, adminActor, first.Reservation.ID)
	if err != nil || current.Status != domain.Cancelled {
		t.Fatal(current, err)
	}
	request.ExpectedTotalMinor++
	changed, err := restarted.Create(ctx, adminActor, "durable", request)
	if err != nil || changed.Problem == nil || changed.Problem.Code != "IDEMPOTENCY_KEY_REUSED" {
		t.Fatal(changed, err)
	}
}
func TestRealPostgresTransientFailureOwnershipAndMaintenance(t *testing.T) {
	pool := testPool(t)
	deps := &fixtureDeps{unavailable: true}
	s := service(pool, deps)
	ctx := context.Background()
	request := input()
	if _, err := s.Create(ctx, adminActor, "transient", request); err == nil {
		t.Fatal("dependency failure confirmed")
	}
	var keys int
	pool.QueryRow(ctx, "SELECT count(*) FROM rental_idempotency").Scan(&keys)
	if keys != 0 {
		t.Fatal("transient failure consumed key")
	}
	deps.unavailable = false
	created, err := s.Create(ctx, adminActor, "transient", request)
	if err != nil || created.Reservation == nil {
		t.Fatal(created, err)
	}
	other := domain.Actor{App: "other-client", Role: "client", CustomerID: "cli-001"}
	if _, err = s.Get(ctx, other, created.Reservation.ID); err == nil {
		t.Fatal("application ownership bypassed")
	}
	for i := 0; i < 2; i++ {
		if _, err = s.Cancel(ctx, adminActor, created.Reservation.ID); err != nil {
			t.Fatal(err)
		}
	}
	p, _ := domain.NewPeriod(request.StartsOn, request.EndsOn, time.Now())
	if _, err = s.Repo.Maintenance(ctx, platform.UUID(), request.VehicleID, p); err != nil {
		t.Fatal(err)
	}
	blocked, err := s.Create(ctx, adminActor, "blocked", request)
	if err != nil || blocked.Problem == nil || blocked.Problem.Code != "VEHICLE_UNAVAILABLE" {
		t.Fatal(blocked, err)
	}
}

func TestRealPostgresOverduePickupAndCompletedOutbox(t *testing.T) {
	pool := testPool(t)
	s := service(pool, &fixtureDeps{})
	ctx := context.Background()
	today := time.Now().In(domain.Branch)
	firstRequest := application.Create{VehicleID: "auto-001", CustomerID: "cli-001", StartsOn: today.Format("2006-01-02"), EndsOn: today.AddDate(0, 0, 1).Format("2006-01-02"), ExpectedTotalMinor: 5000000}
	first, err := s.Create(ctx, adminActor, "first-pickup", firstRequest)
	if err != nil || first.Reservation == nil {
		t.Fatal(first, err)
	}
	if _, err = s.Pickup(ctx, adminActor, first.Reservation.ID); err != nil {
		t.Fatal(err)
	}
	secondRequest := firstRequest
	secondRequest.StartsOn = firstRequest.EndsOn
	secondRequest.EndsOn = today.AddDate(0, 0, 2).Format("2006-01-02")
	second, err := s.Create(ctx, adminActor, "next-pickup", secondRequest)
	if err != nil || second.Reservation == nil {
		t.Fatal(second, err)
	}
	s.Now = func() time.Time { return today.AddDate(0, 0, 1) }
	if _, err = s.Pickup(ctx, adminActor, second.Reservation.ID); err == nil {
		t.Fatal("a delayed active rental allowed a second physical pickup")
	}
	current, err := s.Get(ctx, adminActor, second.Reservation.ID)
	if err != nil || current.Status != domain.Reserved {
		t.Fatal(current, err)
	}
	for i := 0; i < 2; i++ {
		v, err := s.Return(ctx, adminActor, first.Reservation.ID)
		if err != nil || v.Status != domain.Completed {
			t.Fatal(v, err)
		}
	}
	var events int
	if err = pool.QueryRow(ctx, "SELECT count(*) FROM rental_outbox WHERE event_type='RentalCompleted.v1'").Scan(&events); err != nil || events != 1 {
		t.Fatal("return duplicated or lost its transactional event", events, err)
	}
	if _, err = s.Pickup(ctx, adminActor, second.Reservation.ID); err != nil {
		t.Fatal("return did not allow next pickup", err)
	}
}
