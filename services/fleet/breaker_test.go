package fleet

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func testBreaker(cache Cache, clock *time.Time) *BreakerCache {
	b := NewBreakerCache(cache)
	b.now = func() time.Time { return *clock }
	return b
}
func TestBreakerOpensAfterConsecutiveFailuresAndBypassesRedis(t *testing.T) {
	clock := time.Unix(0, 0)
	var calls atomic.Int32
	b := testBreaker(stubCache{get: func(context.Context, string) (Vehicle, bool, error) {
		calls.Add(1)
		return Vehicle{}, false, context.DeadlineExceeded
	}}, &clock)
	for range b.Threshold {
		if _, _, err := b.Get(context.Background(), "auto-001"); !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("closed breaker must report the Redis error, got %v", err)
		}
	}
	if _, _, err := b.Get(context.Background(), "auto-001"); !errors.Is(err, ErrCacheBypassed) {
		t.Fatalf("open breaker must bypass, got %v", err)
	}
	if err := b.Put(context.Background(), Vehicle{VehicleID: "auto-001"}); !errors.Is(err, ErrCacheBypassed) {
		t.Fatalf("open breaker must bypass writes, got %v", err)
	}
	if calls.Load() != int32(b.Threshold) {
		t.Fatalf("Redis calls while open: %d", calls.Load())
	}
}
func TestBreakerSuccessResetsConsecutiveFailures(t *testing.T) {
	clock := time.Unix(0, 0)
	fail := true
	b := testBreaker(stubCache{get: func(context.Context, string) (Vehicle, bool, error) {
		if fail {
			return Vehicle{}, false, errors.New("unavailable")
		}
		return Vehicle{}, false, nil
	}}, &clock)
	for range b.Threshold - 1 {
		b.Get(context.Background(), "auto-001")
	}
	fail = false
	b.Get(context.Background(), "auto-001")
	fail = true
	for range b.Threshold - 1 {
		b.Get(context.Background(), "auto-001")
	}
	if _, _, err := b.Get(context.Background(), "auto-001"); errors.Is(err, ErrCacheBypassed) {
		t.Fatal("non-consecutive failures must not open the breaker")
	}
}
func TestBreakerHalfOpenAllowsSingleProbeAndCloses(t *testing.T) {
	clock := time.Unix(0, 0)
	healthy := false
	release := make(chan struct{})
	var inFlight sync.WaitGroup
	b := testBreaker(stubCache{get: func(context.Context, string) (Vehicle, bool, error) {
		if !healthy {
			return Vehicle{}, false, errors.New("unavailable")
		}
		<-release
		return Vehicle{VehicleID: "auto-001"}, true, nil
	}}, &clock)
	for range b.Threshold {
		b.Get(context.Background(), "auto-001")
	}
	clock = clock.Add(b.Cooldown)
	healthy = true
	inFlight.Add(1)
	go func() {
		defer inFlight.Done()
		if _, hit, err := b.Get(context.Background(), "auto-001"); err != nil || !hit {
			t.Errorf("probe: hit=%v err=%v", hit, err)
		}
	}()
	deadline := time.Now().Add(2 * time.Second)
	for {
		b.mu.Lock()
		probing := b.probing
		b.mu.Unlock()
		if probing || time.Now().After(deadline) {
			break
		}
		time.Sleep(time.Millisecond)
	}
	if _, _, err := b.Get(context.Background(), "auto-001"); !errors.Is(err, ErrCacheBypassed) {
		t.Fatalf("only one probe may reach Redis while half-open, got %v", err)
	}
	close(release)
	inFlight.Wait()
	if _, hit, err := b.Get(context.Background(), "auto-001"); err != nil || !hit {
		t.Fatalf("successful probe must close the breaker: hit=%v err=%v", hit, err)
	}
}
func TestBreakerFailedProbeReopensForAnotherCooldown(t *testing.T) {
	clock := time.Unix(0, 0)
	b := testBreaker(stubCache{get: func(context.Context, string) (Vehicle, bool, error) {
		return Vehicle{}, false, errors.New("unavailable")
	}}, &clock)
	for range b.Threshold {
		b.Get(context.Background(), "auto-001")
	}
	clock = clock.Add(b.Cooldown)
	if _, _, err := b.Get(context.Background(), "auto-001"); errors.Is(err, ErrCacheBypassed) {
		t.Fatal("expired cooldown must let a probe through")
	}
	if _, _, err := b.Get(context.Background(), "auto-001"); !errors.Is(err, ErrCacheBypassed) {
		t.Fatalf("failed probe must reopen, got %v", err)
	}
}
func TestBreakerAlwaysAttemptsInvalidation(t *testing.T) {
	clock := time.Unix(0, 0)
	invalidations := 0
	b := testBreaker(stubCache{get: func(context.Context, string) (Vehicle, bool, error) {
		return Vehicle{}, false, errors.New("unavailable")
	}, invalidate: func(context.Context, string, int64) error {
		invalidations++
		return nil
	}}, &clock)
	for range b.Threshold {
		b.Get(context.Background(), "auto-001")
	}
	if err := b.Invalidate(context.Background(), "auto-001", 3); err != nil || invalidations != 1 {
		t.Fatalf("invalidation must reach Redis while open: err=%v calls=%d", err, invalidations)
	}
	b.mu.Lock()
	open := b.open
	b.mu.Unlock()
	if open {
		t.Fatal("a successful invalidation proves Redis is back and must close the breaker")
	}
}
func TestBreakerIgnoresClientCancellation(t *testing.T) {
	clock := time.Unix(0, 0)
	b := testBreaker(stubCache{get: func(context.Context, string) (Vehicle, bool, error) {
		return Vehicle{}, false, context.Canceled
	}}, &clock)
	for range b.Threshold * 2 {
		b.Get(context.Background(), "auto-001")
	}
	if _, _, err := b.Get(context.Background(), "auto-001"); errors.Is(err, ErrCacheBypassed) {
		t.Fatal("client cancellation must not open the breaker")
	}
}
func TestServiceReadsSourceWithoutFillingWhenBreakerBypasses(t *testing.T) {
	sourceCalls, writes := 0, 0
	repo := stubRepository{get: func(context.Context, string) (Vehicle, error) {
		sourceCalls++
		return Vehicle{VehicleID: "auto-001", Version: 4}, nil
	}}
	cache := stubCache{get: func(context.Context, string) (Vehicle, bool, error) {
		return Vehicle{}, false, ErrCacheBypassed
	}, put: func(context.Context, Vehicle) error {
		writes++
		return nil
	}}
	v, err := NewService(repo, cache, nil).Get(context.Background(), "auto-001")
	if err != nil || v.Version != 4 || sourceCalls != 1 || writes != 0 {
		t.Fatalf("bypass: err=%v version=%d source=%d writes=%d", err, v.Version, sourceCalls, writes)
	}
}
