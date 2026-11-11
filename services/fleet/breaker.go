package fleet

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// ErrCacheBypassed means the breaker skipped Redis; callers read the source.
var ErrCacheBypassed = errors.New("cache bypassed by circuit breaker")

var cacheBreakerState = prometheus.NewGauge(prometheus.GaugeOpts{Name: "fleet_cache_breaker_state", Help: "Cache circuit breaker: 0 closed, 1 open, 2 half-open."})
var cacheBreakerOpenings = prometheus.NewCounter(prometheus.CounterOpts{Name: "fleet_cache_breaker_openings_total", Help: "Transitions of the cache circuit breaker to open."})

// BreakerCache stops paying the Redis deadline on every read while Redis is
// failing. Only Get and Put are short-circuited: Invalidate always reaches
// Redis, because skipping it could serve an older vehicle after recovery.
type BreakerCache struct {
	Cache     Cache
	Threshold int
	Cooldown  time.Duration
	now       func() time.Time
	mu        sync.Mutex
	failures  int
	open      bool
	probing   bool
	openUntil time.Time
}

func NewBreakerCache(cache Cache) *BreakerCache {
	return &BreakerCache{Cache: cache, Threshold: 5, Cooldown: 2 * time.Second, now: time.Now}
}
func (b *BreakerCache) allow() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	if !b.open {
		return true
	}
	if b.probing || b.now().Before(b.openUntil) {
		return false
	}
	// Half-open: a single request probes Redis; the rest keep bypassing it.
	b.probing = true
	cacheBreakerState.Set(2)
	return true
}
func (b *BreakerCache) record(err error) {
	if errors.Is(err, context.Canceled) {
		// The client left; that says nothing about Redis.
		b.mu.Lock()
		b.probing = false
		b.mu.Unlock()
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.probing = false
	if err == nil {
		if b.open {
			slog.Info("fleet cache breaker closed")
		}
		b.failures, b.open = 0, false
		cacheBreakerState.Set(0)
		return
	}
	b.failures++
	if b.failures < b.Threshold {
		return
	}
	if !b.open {
		cacheBreakerOpenings.Inc()
		slog.Warn("fleet cache breaker open", "failures", b.failures, "cooldown", b.Cooldown.String())
	}
	b.open = true
	b.openUntil = b.now().Add(b.Cooldown)
	cacheBreakerState.Set(1)
}
func (b *BreakerCache) Get(ctx context.Context, id string) (Vehicle, bool, error) {
	if !b.allow() {
		return Vehicle{}, false, ErrCacheBypassed
	}
	v, hit, err := b.Cache.Get(ctx, id)
	b.record(err)
	return v, hit, err
}
func (b *BreakerCache) Put(ctx context.Context, v Vehicle) error {
	if !b.allow() {
		return ErrCacheBypassed
	}
	err := b.Cache.Put(ctx, v)
	b.record(err)
	return err
}
func (b *BreakerCache) Invalidate(ctx context.Context, id string, version int64) error {
	err := b.Cache.Invalidate(ctx, id, version)
	b.record(err)
	return err
}
