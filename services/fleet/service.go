package fleet

import (
	"context"
	"github.com/Gerpeirone/Car-Cruds/internal/platform"
	"github.com/prometheus/client_golang/prometheus"
	"log/slog"
	"time"
)

var cacheRequests = prometheus.NewCounterVec(prometheus.CounterOpts{Name: "fleet_cache_requests_total", Help: "Cache hits, misses and errors."}, []string{"result"})
var sourceReads = prometheus.NewCounterVec(prometheus.CounterOpts{Name: "fleet_source_reads_total", Help: "Authoritative repository reads."}, []string{"operation"})
var cacheInvalidationFailures = prometheus.NewCounter(prometheus.CounterOpts{Name: "fleet_cache_invalidation_failures_total", Help: "Invalidation errors after durable writes."})

func RegisterMetrics() {
	platform.RegisterMetrics(cacheRequests, sourceReads, cacheInvalidationFailures, publishedEvents, indexedEvents, indexWriteDelay)
}

type Service struct {
	Repo   Repository
	Cache  Cache
	Search SearchEngine
	slots  chan struct{}
}

func NewService(repo Repository, cache Cache, search SearchEngine) *Service {
	return &Service{Repo: repo, Cache: cache, Search: search, slots: make(chan struct{}, 32)}
}
func (s *Service) acquire() bool {
	select {
	case s.slots <- struct{}{}:
		return true
	default:
		return false
	}
}
func (s *Service) sourceGet(ctx context.Context, id, operation string) (Vehicle, error) {
	if !s.acquire() {
		return Vehicle{}, ErrBusy
	}
	defer func() { <-s.slots }()
	sourceReads.WithLabelValues(operation).Inc()
	return s.Repo.Get(ctx, id)
}
func (s *Service) Get(ctx context.Context, id string) (Vehicle, error) {
	if err := ValidateID(id); err != nil {
		return Vehicle{}, err
	}
	populateCache := false
	if s.Cache != nil {
		cacheCtx, cancel := context.WithTimeout(ctx, 150*time.Millisecond)
		v, hit, err := s.Cache.Get(cacheCtx, id)
		cancel()
		switch {
		case err != nil:
			cacheRequests.WithLabelValues("error").Inc()
		case hit:
			cacheRequests.WithLabelValues("hit").Inc()
			return v, nil
		default:
			cacheRequests.WithLabelValues("miss").Inc()
			populateCache = true
		}
	}
	v, err := s.sourceGet(ctx, id, "detail")
	if err != nil {
		return Vehicle{}, err
	}
	if populateCache {
		cacheCtx, cancel := context.WithTimeout(ctx, 150*time.Millisecond)
		err = s.Cache.Put(cacheCtx, v)
		cancel()
		if err != nil {
			cacheRequests.WithLabelValues("write_error").Inc()
		}
	}
	return v, nil
}
func (s *Service) Quote(ctx context.Context, id string) (Quote, error) {
	if err := ValidateID(id); err != nil {
		return Quote{}, err
	}
	v, err := s.sourceGet(ctx, id, "quote")
	if err != nil {
		return Quote{}, err
	}
	return Quote{VehicleID: v.VehicleID, Active: v.Active, DailyRateMinor: v.DailyRateMinor, Currency: v.Currency, Version: v.Version}, nil
}
func (s *Service) List(ctx context.Context, q Query) (Page, error) {
	if err := q.Validate(); err != nil {
		return Page{}, err
	}
	if !s.acquire() {
		return Page{}, ErrBusy
	}
	defer func() { <-s.slots }()
	sourceReads.WithLabelValues("list").Inc()
	return s.Repo.List(ctx, q)
}
func (s *Service) Find(ctx context.Context, q Query) (Page, error) {
	if err := q.Validate(); err != nil {
		return Page{}, err
	}
	if s.Search == nil {
		return Page{}, ErrSearchUnavailable
	}
	return s.Search.Search(ctx, q)
}
func (s *Service) Create(ctx context.Context, in Input) (Vehicle, error) {
	v, err := in.vehicle(in.VehicleID, 1, true)
	if err != nil {
		return Vehicle{}, err
	}
	if in.Version != 0 {
		return Vehicle{}, ErrInvalid
	}
	event := newEvent(v)
	saved, err := s.Repo.Create(ctx, v, event)
	if err == nil {
		s.invalidate(ctx, saved)
	}
	return saved, err
}
func (s *Service) Update(ctx context.Context, id string, in Input) (Vehicle, error) {
	if err := ValidateID(id); err != nil {
		return Vehicle{}, err
	}
	if in.Version < 1 {
		return Vehicle{}, ErrInvalid
	}
	current, err := s.Repo.Get(ctx, id)
	if err != nil {
		return Vehicle{}, err
	}
	if current.Version != in.Version {
		return Vehicle{}, ErrConflict
	}
	v, err := in.vehicle(id, in.Version+1, current.Active)
	if err != nil {
		return Vehicle{}, err
	}
	saved, err := s.Repo.Update(ctx, v, in.Version, newEvent(v))
	if err == nil {
		s.invalidate(ctx, saved)
	}
	return saved, err
}
func newEvent(v Vehicle) Event {
	return Event{EventID: platform.UUID(), Type: "VehicleChanged.v1", VehicleID: v.VehicleID, Version: v.Version, OccurredAt: v.UpdatedAt}
}
func (s *Service) invalidate(ctx context.Context, v Vehicle) {
	if s.Cache == nil {
		return
	}
	cacheCtx, cancel := context.WithTimeout(ctx, 150*time.Millisecond)
	defer cancel()
	if err := s.Cache.Invalidate(cacheCtx, v.VehicleID, v.Version); err != nil {
		cacheInvalidationFailures.Inc()
		slog.Warn("fleet cache invalidation failed", "vehicleId", v.VehicleID, "version", v.Version)
	}
}
func (s *Service) Rebuild(ctx context.Context) (int, error) {
	store, ok := s.Repo.(OutboxStore)
	if !ok {
		return 0, ErrSearchUnavailable
	}
	count := 0
	err := store.Scan(ctx, func(v Vehicle) error {
		if err := store.EnqueueRebuild(ctx, v.VehicleID, newEvent(v)); err != nil {
			return err
		}
		count++
		return nil
	})
	return count, err
}
