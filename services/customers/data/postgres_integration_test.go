package data

import (
	"context"
	"github.com/Gerpeirone/Car-Cruds/internal/platform"
	"github.com/Gerpeirone/Car-Cruds/services/customers/business"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"os"
	"strings"
	"testing"
)

func TestRealPostgresCustomerPersistenceAndInbox(t *testing.T) {
	connection := os.Getenv("TEST_CUSTOMERS_DATABASE_URL")
	if connection == "" {
		connection = os.Getenv("TEST_DATABASE_URL")
	}
	if connection == "" {
		t.Skip("TEST_CUSTOMERS_DATABASE_URL o TEST_DATABASE_URL requiere PostgreSQL real")
	}
	ctx := context.Background()
	admin, err := pgxpool.New(ctx, connection)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	if err = admin.Ping(ctx); err != nil {
		t.Fatal("PostgreSQL real no disponible")
	}
	schema := "test_customers_" + strings.ReplaceAll(platform.UUID(), "-", "")
	quoted := pgx.Identifier{schema}.Sanitize()
	if _, err = admin.Exec(ctx, "CREATE SCHEMA "+quoted); err != nil {
		t.Fatal(err)
	}
	defer admin.Exec(ctx, "DROP SCHEMA "+quoted+" CASCADE")
	config, err := pgxpool.ParseConfig(connection)
	if err != nil {
		t.Fatal(err)
	}
	config.ConnConfig.RuntimeParams["search_path"] = schema + ",public"
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if err = Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	if err = Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	if err = SeedDemo(ctx, pool); err != nil {
		t.Fatal(err)
	}
	r := &Repository{Pool: pool}
	if _, err = r.Get(ctx, "cli-missing"); err == nil {
		t.Fatal("missing customer found")
	}
	c, err := r.Get(ctx, "cli-001")
	if err != nil || !c.Active || c.LicenseValidUntil != "2030-12-31" {
		t.Fatal(c, err)
	}
	if err = r.Create(ctx, business.Customer{ID: "cli-extra", Name: "Persistido", Active: true, LicenseValidUntil: "2031-01-01"}); err != nil {
		t.Fatal(err)
	}
	r = &Repository{Pool: pool}
	if c, err = r.Get(ctx, "cli-extra"); err != nil || c.Name != "Persistido" {
		t.Fatal(c, err)
	}
	for i := 0; i < 2; i++ {
		if err = r.RecordRental(ctx, "event-001", "res-001", "cli-001", []byte(`{"type":"RentalCompleted.v1"}`)); err != nil {
			t.Fatal(err)
		}
	}
	var history, inbox int
	pool.QueryRow(ctx, "SELECT count(*) FROM customer_rental_history").Scan(&history)
	pool.QueryRow(ctx, "SELECT count(*) FROM customer_inbox").Scan(&inbox)
	if history != 1 || inbox != 1 {
		t.Fatal("duplicate event effects", history, inbox)
	}
}
