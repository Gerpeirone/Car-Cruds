package fleet

import (
	"context"
	"errors"
	"github.com/Gerpeirone/Car-Cruds/internal/platform"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func integrationMongo(t *testing.T) (context.Context, *MongoRepository, string) {
	t.Helper()
	uri := os.Getenv("TEST_MONGODB_URI")
	if uri == "" {
		t.Skip("TEST_MONGODB_URI is required for real MongoDB integration")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	t.Cleanup(cancel)
	database := "fleet_test_" + strings.ReplaceAll(platform.UUID(), "-", "")
	repo, err := OpenMongo(ctx, uri, database)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = repo.Client.Database(database).Drop(cleanup)
		_ = repo.Close(cleanup)
	})
	return ctx, repo, database
}
func TestMongoAtomicVersionOutboxAndRestart(t *testing.T) {
	ctx, repo, database := integrationMongo(t)
	s := NewService(repo, nil, nil)
	created, err := s.Create(ctx, validInput("auto-001"))
	if err != nil {
		t.Fatal(err)
	}
	var successes, conflicts atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			input := validInput("auto-001")
			input.Version = created.Version
			input.DailyRateMinor = 6000000
			_, err := s.Update(ctx, "auto-001", input)
			if err == nil {
				successes.Add(1)
			} else if errors.Is(err, ErrConflict) {
				conflicts.Add(1)
			} else {
				t.Errorf("unexpected update: %v", err)
			}
		}()
	}
	wg.Wait()
	if successes.Load() != 1 || conflicts.Load() != 11 {
		t.Fatalf("updates=%d conflicts=%d", successes.Load(), conflicts.Load())
	}
	pending, err := repo.Pending(ctx, 100)
	if err != nil || len(pending) != 2 {
		t.Fatalf("pending events=%d err=%v", len(pending), err)
	}
	reopened, err := OpenMongo(ctx, os.Getenv("TEST_MONGODB_URI"), database)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close(context.Background())
	v, err := reopened.Get(ctx, "auto-001")
	if err != nil || v.Version != 2 || v.DailyRateMinor != 6000000 {
		t.Fatalf("persisted vehicle %+v err=%v", v, err)
	}
	if err = repo.MarkPublished(ctx, pending[0]); err != nil {
		t.Fatal(err)
	}
	pending, err = repo.Pending(ctx, 100)
	if err != nil || len(pending) != 1 || pending[0].Version != 2 {
		t.Fatalf("ack must preserve later event: %+v err=%v", pending, err)
	}
}
func TestMongoBoundedOutboxRejectsWithoutLosingChange(t *testing.T) {
	ctx, repo, _ := integrationMongo(t)
	s := NewService(repo, nil, nil)
	v, err := s.Create(ctx, validInput("auto-001"))
	if err != nil {
		t.Fatal(err)
	}
	for i := 1; i < MaxPendingEvents; i++ {
		if err = repo.EnqueueRebuild(ctx, v.VehicleID, newEvent(v)); err != nil {
			t.Fatal(err)
		}
	}
	input := validInput("auto-001")
	input.Version = 1
	input.DailyRateMinor = 7500000
	if _, err = s.Update(ctx, v.VehicleID, input); !errors.Is(err, ErrOutboxFull) {
		t.Fatalf("expected backpressure, got %v", err)
	}
	current, err := repo.Get(ctx, v.VehicleID)
	if err != nil || current.Version != 1 || current.DailyRateMinor != 5000000 {
		t.Fatalf("rejected mutation persisted: %+v err=%v", current, err)
	}
	pending, err := repo.Pending(ctx, MaxPendingEvents+1)
	if err != nil || len(pending) != MaxPendingEvents {
		t.Fatalf("events were dropped: %d err=%v", len(pending), err)
	}
	if err = repo.MarkPublished(ctx, pending[0]); err != nil {
		t.Fatal(err)
	}
	updated, err := s.Update(ctx, v.VehicleID, input)
	if err != nil || updated.Version != 2 {
		t.Fatalf("update after draining failed: %+v err=%v", updated, err)
	}
	pending, err = repo.Pending(ctx, MaxPendingEvents+1)
	if err != nil || len(pending) != MaxPendingEvents {
		t.Fatalf("atomic append failed: %d err=%v", len(pending), err)
	}
}
