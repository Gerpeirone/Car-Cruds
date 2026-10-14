package fleet

import (
	"context"
	"github.com/Gerpeirone/Car-Cruds/internal/platform"
	"os"
	"testing"
	"time"
)

func TestRedisRejectsDelayedFillAndOldInvalidation(t *testing.T) {
	addr := os.Getenv("TEST_REDIS_ADDR")
	if addr == "" {
		t.Skip("TEST_REDIS_ADDR is required for real Redis integration")
	}
	cache := NewRedisCache(addr, os.Getenv("TEST_REDIS_PASSWORD"), "fleet_test:"+platform.UUID())
	defer cache.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := cache.Client.Ping(ctx).Err(); err != nil {
		t.Fatal(err)
	}
	defer cache.Client.Del(context.Background(), cache.keys("auto-001")...)
	old := Vehicle{VehicleID: "auto-001", Version: 1, DailyRateMinor: 5000000}
	fresh := old
	fresh.Version = 2
	fresh.DailyRateMinor = 7500000
	if err := cache.Put(ctx, old); err != nil {
		t.Fatal(err)
	}
	// A reader captured version 1 before the source write; invalidation fences its delayed fill.
	if err := cache.Invalidate(ctx, "auto-001", 2); err != nil {
		t.Fatal(err)
	}
	if err := cache.Put(ctx, old); err != nil {
		t.Fatal(err)
	}
	if _, hit, err := cache.Get(ctx, "auto-001"); err != nil || hit {
		t.Fatalf("old fill survived fence: hit=%v err=%v", hit, err)
	}
	if err := cache.Put(ctx, fresh); err != nil {
		t.Fatal(err)
	}
	if err := cache.Invalidate(ctx, "auto-001", 1); err != nil {
		t.Fatal(err)
	}
	value, hit, err := cache.Get(ctx, "auto-001")
	if err != nil || !hit || value.Version != 2 || value.DailyRateMinor != 7500000 {
		t.Fatalf("older invalidation harmed newer cache: %+v hit=%v err=%v", value, hit, err)
	}
	ttl, err := cache.Client.PTTL(ctx, cache.keys("auto-001")[0]).Result()
	if err != nil || ttl <= 0 || ttl > 60*time.Second {
		t.Fatalf("TTL=%v err=%v", ttl, err)
	}
}
