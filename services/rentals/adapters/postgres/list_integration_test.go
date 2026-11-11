package postgres

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/Gerpeirone/Car-Cruds/services/rentals/application"
	"github.com/Gerpeirone/Car-Cruds/services/rentals/domain"
)

// "Mis alquileres": a driver lists only the reservations of its own validated
// identity and application, cannot widen the scope with another customerId,
// and cancels only what the domain allows, once, even under concurrency.
func TestRealPostgresDriverListAndCancellationRules(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	svc := service(pool, &fixtureDeps{})
	today := time.Now().In(domain.Branch)
	day := func(n int) string { return today.AddDate(0, 0, n).Format("2006-01-02") }
	create := func(customer, start, end string, total int64) domain.Reservation {
		t.Helper()
		in := application.Create{VehicleID: "auto-001", CustomerID: customer, StartsOn: start, EndsOn: end, ExpectedTotalMinor: total}
		res, err := svc.Create(ctx, adminActor, "list-"+customer+"-"+start, in)
		if err != nil || res.Reservation == nil {
			t.Fatalf("create %s %s: %+v %v", customer, start, res, err)
		}
		return *res.Reservation
	}
	// The admin actor owns these rows; a driver credential sees reservations
	// created through its own application, as with GET /v1/reservations/{id}.
	driver := domain.Actor{App: "cliente-demo", Role: "client", CustomerID: "cli-001"}
	owned := func(r domain.Reservation) {
		t.Helper()
		if _, err := pool.Exec(ctx, "UPDATE rental_reservations SET owner_app=$2 WHERE id=$1", r.ID, driver.App); err != nil {
			t.Fatal(err)
		}
	}
	future := create("cli-001", day(10), day(12), 10000000)
	active := create("cli-001", day(0), day(2), 10000000)
	other := create("cli-002", day(20), day(23), 15000000)
	foreignApp := create("cli-001", day(30), day(31), 5000000)
	owned(future)
	owned(active)
	owned(other)
	if _, err := svc.Pickup(ctx, adminActor, active.ID); err != nil {
		t.Fatal(err)
	}

	page, err := svc.List(ctx, driver, "", 1, 20)
	if err != nil || page.Total != 2 || len(page.Items) != 2 || page.Items[0].ID != future.ID || page.Items[1].ID != active.ID {
		t.Fatalf("propias=%+v %v", page, err)
	}
	for _, r := range page.Items {
		if r.CustomerID != "cli-001" || r.ID == other.ID || r.ID == foreignApp.ID || r.Period.Days < 1 {
			t.Fatalf("fila ajena o incompleta: %+v", r)
		}
	}
	if _, err := svc.List(ctx, driver, "cli-002", 1, 20); err == nil || err.(*domain.Problem).Code != "FORBIDDEN" {
		t.Fatalf("otro conductor=%v", err)
	}
	if p, err := svc.List(ctx, driver, "", 2, 1); err != nil || len(p.Items) != 1 || p.Items[0].ID != active.ID || p.Total != 2 {
		t.Fatalf("paginado=%+v %v", p, err)
	}
	if _, err := svc.List(ctx, driver, "", 1, 51); err == nil {
		t.Fatal("pageSize sin límite")
	}
	if p, err := svc.List(ctx, adminActor, "cli-002", 1, 20); err != nil || p.Total != 1 || p.Items[0].ID != other.ID {
		t.Fatalf("admin=%+v %v", p, err)
	}
	if _, err := svc.List(ctx, adminActor, "", 1, 20); err == nil {
		t.Fatal("admin sin customerId")
	}
	if _, err := svc.List(ctx, domain.Actor{App: "gym", Role: "gym"}, "cli-001", 1, 20); err == nil {
		t.Fatal("rol sin permiso")
	}

	// A driver cannot cancel another driver's reservation: it looks absent.
	if _, err := svc.Cancel(ctx, driver, other.ID); err == nil || err.(*domain.Problem).Code != "NOT_FOUND" {
		t.Fatalf("cancelar ajena=%v", err)
	}
	// An ACTIVE rental is not cancelled: it is returned by the operator.
	if _, err := svc.Cancel(ctx, driver, active.ID); err == nil || err.(*domain.Problem).Code != "INVALID_STATE" {
		t.Fatalf("cancelar activa=%v", err)
	}
	// Concurrent and repeated cancellations have one effect and the same answer.
	var wg sync.WaitGroup
	errs := make(chan error, 6)
	for range 6 {
		wg.Go(func() {
			r, err := svc.Cancel(ctx, driver, future.ID)
			if err == nil && r.Status != domain.Cancelled {
				err = &domain.Problem{Code: "UNEXPECTED", Message: string(r.Status)}
			}
			errs <- err
		})
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("cancelación concurrente=%v", err)
		}
	}
	var cancelled, free int
	if err := pool.QueryRow(ctx, "SELECT count(*) FILTER (WHERE status='CANCELLED'), (SELECT count(*) FROM rental_agenda WHERE reservation_id=$1 AND NOT occupied) FROM rental_reservations WHERE id=$1", future.ID).Scan(&cancelled, &free); err != nil || cancelled != 1 || free != 1 {
		t.Fatalf("estado=%d agenda liberada=%d %v", cancelled, free, err)
	}
	if again, err := svc.Cancel(ctx, driver, future.ID); err != nil || again.Status != domain.Cancelled {
		t.Fatalf("cancelación repetida=%+v %v", again, err)
	}
	if p, _ := svc.List(ctx, driver, "", 1, 20); p.Items[0].Status != domain.Cancelled || p.Items[1].Status != domain.Active {
		t.Fatalf("lista actualizada=%+v", p.Items)
	}
}
