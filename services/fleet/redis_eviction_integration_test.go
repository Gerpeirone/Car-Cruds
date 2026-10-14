package fleet

import (
	"context"
	"net"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Gerpeirone/Car-Cruds/internal/platform"
)

// This pressure test requires an explicitly supplied, disposable Redis. It
// never changes server configuration or removes keys owned by another run.
func TestRedisEvictionPreservesVersionFenceForDelayedFill(t *testing.T) {
	addr := os.Getenv("TEST_REDIS_EVICTION_ADDR")
	if addr == "" {
		t.Skip("TEST_REDIS_EVICTION_ADDR is required for isolated real Redis eviction integration")
	}
	host, _, err := net.SplitHostPort(addr)
	ip := net.ParseIP(host)
	if err != nil || (host != "localhost" && (ip == nil || !ip.IsLoopback())) || addr == os.Getenv("TEST_REDIS_ADDR") {
		t.Fatal("eviction test requires a dedicated loopback Redis address, different from TEST_REDIS_ADDR")
	}
	cache := NewRedisCache(addr, os.Getenv("TEST_REDIS_EVICTION_PASSWORD"), "fleet_eviction_test:"+platform.UUID())
	defer cache.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	policy, err := cache.Client.ConfigGet(ctx, "maxmemory-policy").Result()
	if err != nil || policy["maxmemory-policy"] != "volatile-lru" {
		t.Fatal("isolated Redis must already use volatile-lru; test does not change configuration")
	}
	configuration, err := cache.Client.ConfigGet(ctx, "maxmemory").Result()
	maxMemory, parseErr := strconv.ParseInt(configuration["maxmemory"], 10, 64)
	if err != nil || parseErr != nil || maxMemory <= 0 || maxMemory > 8<<20 {
		t.Fatal("isolated Redis must already have a bounded maxmemory of at most 8 MiB")
	}
	ownedKeys := append([]string(nil), cache.keys("auto-001")...)
	defer func() {
		cleanCtx, cleanCancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cleanCancel()
		if err := cache.Client.Del(cleanCtx, ownedKeys...).Err(); err != nil {
			t.Error("could not clean this eviction test's own keys")
		}
	}()
	old := Vehicle{VehicleID: "auto-001", Version: 1, DailyRateMinor: 5000000}
	if err := cache.Put(ctx, old); err != nil {
		t.Fatal("could not prepare old cached version")
	}
	// The reader has captured V1. V2 commits in the source and invalidates;
	// cached data is gone, so only the persistent watermark can fence that reader.
	if err := cache.Invalidate(ctx, old.VehicleID, 2); err != nil {
		t.Fatal("could not establish version 2 invalidation")
	}
	if count, err := cache.Client.Exists(ctx, ownedKeys[0]).Result(); err != nil || count != 0 {
		t.Fatal("test requires absent cached data before pressure")
	}
	baseline := evictionCount(t, ctx, cache)
	evicted := baseline
	payload := strings.Repeat("x", 32<<10)
	for i := 0; i < 2048 && evicted-baseline < 64; i++ {
		key := cache.prefix + ":pressure:" + strconv.Itoa(i)
		ownedKeys = append(ownedKeys, key)
		if err := cache.Client.Set(ctx, key, payload, time.Minute).Err(); err != nil {
			t.Fatal("bounded pressure write failed before confirming eviction")
		}
		if i%8 == 7 {
			evicted = evictionCount(t, ctx, cache)
		}
	}
	evicted = evictionCount(t, ctx, cache)
	if evicted-baseline < 64 {
		t.Fatalf("test did not reach real memory pressure: observed %d evictions", evicted-baseline)
	}
	marker, err := cache.Client.Get(ctx, ownedKeys[1]).Result()
	if err != nil || marker != "2" {
		t.Fatal("memory pressure evicted or changed the invalidation watermark")
	}
	if ttl, err := cache.Client.PTTL(ctx, ownedKeys[1]).Result(); err != nil || ttl != -1 {
		t.Fatal("invalidation watermark must remain without expiration")
	}
	if err := cache.Put(ctx, old); err != nil {
		t.Fatal("delayed old fill failed instead of being conditionally rejected")
	}
	// Check storage directly: a cache read could repair a wrongly stored stale
	// payload and would conceal the very race this test is intended to detect.
	if count, err := cache.Client.Exists(ctx, ownedKeys[0]).Result(); err != nil || count != 0 {
		t.Fatal("delayed V1 fill repopulated cached data after successful V2 invalidation")
	}
	fresh := old
	fresh.Version, fresh.DailyRateMinor = 2, 7500000
	if err := cache.Put(ctx, fresh); err != nil {
		t.Fatal("could not cache current version after eviction pressure")
	}
	value, hit, err := cache.Get(ctx, old.VehicleID)
	if err != nil || !hit || value.Version != 2 || value.DailyRateMinor != fresh.DailyRateMinor {
		t.Fatal("eviction policy did not preserve usable current-version caching")
	}
	t.Logf("observed %d real evictions with maxmemory=%d bytes; non-expiring V2 fence preserved and delayed V1 rejected", evicted-baseline, maxMemory)
}

func evictionCount(t *testing.T, ctx context.Context, cache *RedisCache) int64 {
	t.Helper()
	stats, err := cache.Client.Info(ctx, "stats").Result()
	if err != nil {
		t.Fatal("could not read isolated Redis eviction statistics")
	}
	for _, line := range strings.Split(stats, "\n") {
		key, value, present := strings.Cut(strings.TrimSpace(line), ":")
		if present && key == "evicted_keys" {
			count, err := strconv.ParseInt(value, 10, 64)
			if err != nil {
				t.Fatal("invalid isolated Redis eviction statistic")
			}
			return count
		}
	}
	t.Fatal("isolated Redis did not report evicted_keys")
	return 0
}
