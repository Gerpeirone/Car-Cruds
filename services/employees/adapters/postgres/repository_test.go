package postgres_test

import (
	"context"
	"errors"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Gerpeirone/Car-Cruds/internal/platform"
	"github.com/Gerpeirone/Car-Cruds/services/employees/adapters/postgres"
	"github.com/Gerpeirone/Car-Cruds/services/employees/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func isolatedDatabase(t *testing.T) (*pgxpool.Pool, *pgxpool.Config) {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("requiere PostgreSQL real mediante TEST_DATABASE_URL")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	admin, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	if err = admin.Ping(ctx); err != nil {
		admin.Close()
		t.Fatal(err)
	}
	schema := "employees_test_" + strings.ReplaceAll(platform.UUID(), "-", "")
	quoted := pgx.Identifier{schema}.Sanitize()
	if _, err = admin.Exec(ctx, "CREATE SCHEMA "+quoted); err != nil {
		admin.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if _, err := admin.Exec(cleanup, "DROP SCHEMA "+quoted+" CASCADE"); err != nil {
			t.Errorf("limpieza del esquema propio: %v", err)
		}
		admin.Close()
	})
	config, err := pgxpool.ParseConfig(url)
	if err != nil {
		t.Fatal(err)
	}
	config.ConnConfig.RuntimeParams["search_path"] = schema
	config.MaxConns = 5
	pool, err := pgxpool.NewWithConfig(ctx, config.Copy())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if err = postgres.New(pool).Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	return pool, config
}

func TestPostgresPersistsSeparatePopulationsAndSeedDoesNotOverwriteLeaves(t *testing.T) {
	pool, config := isolatedDatabase(t)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	repository := postgres.New(pool)
	if err := repository.SeedDemo(ctx); err != nil {
		t.Fatal(err)
	}
	if err := repository.Migrate(ctx); err != nil {
		t.Fatal("repeat migration:", err)
	}
	if _, err := repository.DeactivateEmployee(ctx, "employee-001"); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.UpdateEmployee(ctx, domain.Employee{ID: "employee-002", DisplayName: "Nombre ficticio modificado", Active: true}); err != nil {
		t.Fatal(err)
	}
	if err := repository.CreateEmployee(ctx, domain.Employee{ID: "employee-003", DisplayName: "Persona inactiva", Active: false}); err != nil {
		t.Fatal(err)
	}
	if err := repository.CreateCandidate(ctx, domain.Candidate{ID: "candidate-002", DisplayName: "Otra posible incorporación"}); err != nil {
		t.Fatal(err)
	}
	if err := repository.SeedDemo(ctx); err != nil {
		t.Fatal(err)
	}
	items, total, err := repository.ListEmployees(ctx, domain.Page{Number: 1, Size: 1}, true)
	if err != nil || total != 1 || len(items) != 1 || items[0].ID != "employee-002" || items[0].DisplayName != "Nombre ficticio modificado" {
		t.Fatalf("active population: %+v total=%d err=%v", items, total, err)
	}
	_, all, err := repository.ListEmployees(ctx, domain.Page{Number: 1, Size: 20}, false)
	if err != nil || all != 3 {
		t.Fatalf("administrative population=%d, err=%v", all, err)
	}
	_, candidates, err := repository.ListCandidates(ctx, domain.Page{Number: 1, Size: 20})
	if err != nil || candidates != 2 {
		t.Fatalf("candidate population=%d, err=%v", candidates, err)
	}
	pool.Close()
	reconnected, err := pgxpool.NewWithConfig(ctx, config.Copy())
	if err != nil {
		t.Fatal(err)
	}
	defer reconnected.Close()
	repository = postgres.New(reconnected)
	employee, err := repository.GetEmployee(ctx, "employee-001")
	if err != nil || employee.Active {
		t.Fatalf("leave not persisted after reconnect: %+v %v", employee, err)
	}
	if _, err = repository.UpdateCandidate(ctx, domain.Candidate{ID: "candidate-002", DisplayName: "Nombre administrativo revisado"}); err != nil {
		t.Fatal(err)
	}
	if err = repository.DeleteCandidate(ctx, "candidate-002"); err != nil {
		t.Fatal(err)
	}
	if _, err = repository.GetCandidate(ctx, "candidate-002"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("deleted candidate err=%v", err)
	}
	if _, err = repository.GetEmployee(ctx, "candidate-001"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("candidate became employee err=%v", err)
	}
}

type queryKey struct{}
type afterCount struct {
	once  sync.Once
	write func()
}

func (t *afterCount) TraceQueryStart(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	return context.WithValue(ctx, queryKey{}, data.SQL)
}
func (t *afterCount) TraceQueryEnd(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryEndData) {
	query, _ := ctx.Value(queryKey{}).(string)
	if data.Err == nil && strings.HasPrefix(query, "SELECT count(*) FROM employees") {
		t.once.Do(t.write)
	}
}

func TestPostgresPageAndCountUseSameSnapshotDuringConcurrentWrite(t *testing.T) {
	pool, config := isolatedDatabase(t)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	repository := postgres.New(pool)
	if err := repository.CreateEmployee(ctx, domain.Employee{ID: "employee-001", DisplayName: "Primera persona ficticia", Active: true}); err != nil {
		t.Fatal(err)
	}
	var writeErr error
	config.ConnConfig.Tracer = &afterCount{write: func() {
		// Commit on another connection precisely between COUNT and SELECT.
		writeErr = repository.CreateEmployee(ctx, domain.Employee{ID: "employee-002", DisplayName: "Cambio concurrente", Active: true})
	}}
	reader, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	items, total, err := postgres.New(reader).ListEmployees(ctx, domain.Page{Number: 1, Size: 20}, true)
	if writeErr != nil {
		t.Fatal("concurrent writer:", writeErr)
	}
	if err != nil || total != 1 || len(items) != 1 || items[0].ID != "employee-001" {
		t.Fatalf("count/list read different snapshots: %+v total=%d err=%v", items, total, err)
	}
	items, total, err = repository.ListEmployees(ctx, domain.Page{Number: 2, Size: 1}, true)
	if err != nil || total != 2 || len(items) != 1 || items[0].ID != "employee-002" {
		t.Fatalf("next request cannot see committed write: %+v total=%d err=%v", items, total, err)
	}
}
