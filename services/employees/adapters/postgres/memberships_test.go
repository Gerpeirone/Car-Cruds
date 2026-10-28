package postgres_test

import (
	"context"
	"errors"
	"github.com/Gerpeirone/Car-Cruds/services/employees/adapters/postgres"
	"github.com/Gerpeirone/Car-Cruds/services/employees/domain"
	"github.com/jackc/pgx/v5/pgxpool"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func ptr[T any](v T) *T { return &v }

func TestMembershipPersistenceCorrectionsAndReport(t *testing.T) {
	pool, config := isolatedDatabase(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	repo := postgres.New(pool)
	for _, e := range []domain.Employee{{ID: "a", DisplayName: "Ficticio A", Active: true}, {ID: "b", DisplayName: "Ficticio B", Active: false}, {ID: "c", DisplayName: "Ficticio C", Active: true}} {
		if err := repo.CreateEmployee(ctx, e); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := pool.Exec(ctx, `INSERT INTO candidates(candidate_id,display_name) VALUES('candidate','Registro historico')`); err != nil {
		t.Fatal(err)
	}
	s := domain.MembershipSnapshot{EmployeeID: "a", Period: "2026-10", SourceReference: "ref-a", Version: 2, ObservedAt: time.Date(2026, 10, 22, 12, 0, 0, 0, time.UTC), Member: ptr(true), Used: ptr(false), CostMinor: ptr(int64(100)), SpendingMinor: ptr(int64(0)), Currency: "ARS"}
	receive := func(s domain.MembershipSnapshot, want bool) {
		t.Helper()
		applied, err := repo.ReceiveMembership(ctx, "gimnasio-pablo", s)
		if err != nil || applied != want {
			t.Fatalf("receipt applied=%v err=%v", applied, err)
		}
	}
	receive(s, true)
	receive(s, false)
	conflict := s
	conflict.CostMinor = ptr(int64(200))
	if _, err := repo.ReceiveMembership(ctx, "gimnasio-pablo", conflict); !errors.Is(err, domain.ErrEvidenceConflict) {
		t.Fatalf("same version conflict=%v", err)
	}
	conflict = s
	conflict.SourceReference = "another-reference"
	conflict.Version = 3
	if _, err := repo.ReceiveMembership(ctx, "gimnasio-pablo", conflict); !errors.Is(err, domain.ErrEvidenceConflict) {
		t.Fatalf("identity conflict=%v", err)
	}
	if _, err := repo.ReceiveMembership(ctx, "another-gym", s); !errors.Is(err, domain.ErrEvidenceConflict) {
		t.Fatalf("source conflict=%v", err)
	}
	stale := s
	stale.Version = 1
	stale.CostMinor = ptr(int64(999))
	receive(stale, false)
	corrected := s
	corrected.Version = 3
	corrected.CostMinor = ptr(int64(120))
	receive(corrected, true)
	foreign := s
	foreign.EmployeeID = "candidate"
	if _, err := repo.ReceiveMembership(ctx, "gimnasio-pablo", foreign); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("candidate receipt=%v", err)
	}
	usd := s
	usd.EmployeeID = "b"
	usd.SourceReference = "ref-b"
	usd.Currency = "USD"
	usd.CostMinor = ptr(int64(50))
	usd.SpendingMinor = nil
	receive(usd, true)
	report, err := repo.MembershipReport(ctx, "2026-10", domain.Page{Number: 1, Size: 1})
	if err != nil || report.Total != 3 || report.MissingEvidence != 1 || report.IncompleteEvidence != 2 || len(report.Items) != 1 || len(report.Totals) != 2 || *report.Totals[0].CostMinor != 120 || *report.Totals[0].SpendingMinor != 0 || report.Totals[1].SpendingMinor != nil {
		t.Fatalf("report: %+v %v", report, err)
	}
	if report.Items[0].Evidence.Version != 3 || report.Items[0].SourceApp != "gimnasio-pablo" {
		t.Fatalf("provenance: %+v", report.Items[0])
	}
	next, err := repo.MembershipReport(ctx, "2026-10", domain.Page{Number: 2, Size: 1})
	if err != nil || next.Items[0].Employee.Active || next.Items[0].Employee.ID != "b" || *next.Totals[0].CostMinor != 120 {
		t.Fatalf("inactive history/page: %+v %v", next, err)
	}
	empty, err := repo.MembershipReport(ctx, "2026-11", domain.Page{Number: 1, Size: 20})
	if err != nil || empty.MissingEvidence != 3 || len(empty.Totals) != 0 {
		t.Fatalf("period leakage: %+v %v", empty, err)
	}
	var history int
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM membership_evidence_history WHERE employee_id='a'`).Scan(&history); err != nil || history != 3 {
		t.Fatalf("history=%d err=%v", history, err)
	}
	pool.Close()
	reopened, err := pgxpool.NewWithConfig(ctx, config.Copy())
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	repo = postgres.New(reopened)
	if err = repo.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	receive(corrected, false)
	report, err = repo.MembershipReport(ctx, "2026-10", domain.Page{Number: 1, Size: 20})
	if err != nil || *report.Totals[0].CostMinor != 120 {
		t.Fatalf("restart: %+v %v", report, err)
	}
}

func TestConcurrentGymRetriesHaveOneEffect(t *testing.T) {
	pool, _ := isolatedDatabase(t)
	repo := postgres.New(pool)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := repo.CreateEmployee(ctx, domain.Employee{ID: "one", DisplayName: "Ficticio", Active: true}); err != nil {
		t.Fatal(err)
	}
	s := domain.MembershipSnapshot{EmployeeID: "one", Period: "P", SourceReference: "r", Version: 1, ObservedAt: time.Now(), SpendingMinor: ptr(int64(100)), Currency: "ARS"}
	var applied atomic.Int64
	var wg sync.WaitGroup
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ok, err := postgres.New(pool).ReceiveMembership(ctx, "gym", s)
			if err != nil {
				t.Error(err)
			}
			if ok {
				applied.Add(1)
			}
		}()
	}
	wg.Wait()
	if applied.Load() != 1 {
		t.Fatalf("effects=%d", applied.Load())
	}
	report, err := repo.MembershipReport(ctx, "P", domain.Page{Number: 1, Size: 20})
	if err != nil || *report.Totals[0].SpendingMinor != 100 {
		t.Fatalf("duplicate spending: %+v %v", report, err)
	}
}
