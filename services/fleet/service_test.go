package fleet

import (
	"context"
	"errors"
	"github.com/Gerpeirone/Car-Cruds/internal/platform"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

type stubRepository struct {
	get    func(context.Context, string) (Vehicle, error)
	list   func(context.Context, Query) (Page, error)
	create func(context.Context, Vehicle, Event) (Vehicle, error)
	update func(context.Context, Vehicle, int64, Event) (Vehicle, error)
}

func (r stubRepository) Get(ctx context.Context, id string) (Vehicle, error) { return r.get(ctx, id) }
func (r stubRepository) List(ctx context.Context, q Query) (Page, error) {
	if r.list != nil {
		return r.list(ctx, q)
	}
	return Page{}, nil
}
func (r stubRepository) Create(ctx context.Context, v Vehicle, e Event) (Vehicle, error) {
	if r.create != nil {
		return r.create(ctx, v, e)
	}
	return v, nil
}
func (r stubRepository) Update(ctx context.Context, v Vehicle, expected int64, e Event) (Vehicle, error) {
	if r.update != nil {
		return r.update(ctx, v, expected, e)
	}
	return v, nil
}

type stubCache struct {
	get        func(context.Context, string) (Vehicle, bool, error)
	put        func(context.Context, Vehicle) error
	invalidate func(context.Context, string, int64) error
}

func (c stubCache) Get(ctx context.Context, id string) (Vehicle, bool, error) {
	if c.get != nil {
		return c.get(ctx, id)
	}
	return Vehicle{}, false, nil
}
func (c stubCache) Put(ctx context.Context, v Vehicle) error {
	if c.put != nil {
		return c.put(ctx, v)
	}
	return nil
}
func (c stubCache) Invalidate(ctx context.Context, id string, v int64) error {
	if c.invalidate != nil {
		return c.invalidate(ctx, id, v)
	}
	return nil
}

func validInput(id string) Input {
	return Input{VehicleID: id, Brand: "Demo", Model: "Modelo", Category: "sedan", Transmission: "manual", Seats: 5, DailyRateMinor: 5000000, Currency: "ARS"}
}
func TestQuoteAlwaysUsesAuthoritativeRepository(t *testing.T) {
	cachedReads := 0
	cache := stubCache{get: func(context.Context, string) (Vehicle, bool, error) {
		cachedReads++
		return Vehicle{VehicleID: "auto-001", DailyRateMinor: 1, Version: 1}, true, nil
	}}
	repo := stubRepository{get: func(context.Context, string) (Vehicle, error) {
		return Vehicle{VehicleID: "auto-001", Active: true, DailyRateMinor: 6500000, Currency: "ARS", Version: 2}, nil
	}}
	s := NewService(repo, cache, nil)
	quote, err := s.Quote(context.Background(), "auto-001")
	if err != nil || quote.DailyRateMinor != 6500000 || quote.Version != 2 || cachedReads != 0 {
		t.Fatalf("authoritative quote: %+v, err=%v, cache reads=%d", quote, err, cachedReads)
	}
}
func TestCacheFailureDoesNotFailAvailableSource(t *testing.T) {
	sourceCalls, cacheReads, cacheWrites := 0, 0, 0
	repo := stubRepository{get: func(context.Context, string) (Vehicle, error) {
		sourceCalls++
		return Vehicle{VehicleID: "auto-001", Version: 2}, nil
	}}
	cache := stubCache{get: func(context.Context, string) (Vehicle, bool, error) {
		cacheReads++
		return Vehicle{}, false, errors.New("unavailable")
	}, put: func(context.Context, Vehicle) error {
		cacheWrites++
		return errors.New("unavailable")
	}}
	v, err := NewService(repo, cache, nil).Get(context.Background(), "auto-001")
	if err != nil || v.Version != 2 || sourceCalls != 1 || cacheReads != 1 || cacheWrites != 0 {
		t.Fatalf("source fallback: err=%v source=%d cacheReads=%d cacheWrites=%d", err, sourceCalls, cacheReads, cacheWrites)
	}
}
func TestNormalCacheMissStillFillsFromSource(t *testing.T) {
	sourceCalls, cacheWrites := 0, 0
	expected := Vehicle{VehicleID: "auto-001", Version: 2}
	repo := stubRepository{get: func(context.Context, string) (Vehicle, error) {
		sourceCalls++
		return expected, nil
	}}
	cache := stubCache{put: func(_ context.Context, value Vehicle) error {
		cacheWrites++
		if value != expected {
			t.Fatal("cache fill did not use the authoritative source result")
		}
		return nil
	}}
	v, err := NewService(repo, cache, nil).Get(context.Background(), "auto-001")
	if err != nil || v != expected || sourceCalls != 1 || cacheWrites != 1 {
		t.Fatalf("normal miss: err=%v source=%d cacheWrites=%d", err, sourceCalls, cacheWrites)
	}
}
func TestSourceFallbackHasBoundedConcurrency(t *testing.T) {
	entered := make(chan struct{}, 32)
	release := make(chan struct{})
	repo := stubRepository{get: func(context.Context, string) (Vehicle, error) {
		entered <- struct{}{}
		<-release
		return Vehicle{VehicleID: "auto-001"}, nil
	}}
	s := NewService(repo, nil, nil)
	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); _, _ = s.Get(context.Background(), "auto-001") }()
	}
	defer func() { close(release); wg.Wait() }()
	for i := 0; i < 32; i++ {
		select {
		case <-entered:
		case <-time.After(2 * time.Second):
			t.Fatal("source calls did not start")
		}
	}
	if _, err := s.Get(context.Background(), "auto-001"); !errors.Is(err, ErrBusy) {
		t.Fatalf("expected bounded rejection, got %v", err)
	}
}
func TestVehicleRulesRejectBeforePersistence(t *testing.T) {
	cases := []struct {
		name   string
		change func(*Input)
	}{
		{"negative price", func(i *Input) { i.DailyRateMinor = -1 }},
		{"wrong currency", func(i *Input) { i.Currency = "USD" }},
		{"no seats", func(i *Input) { i.Seats = 0 }},
		{"invalid transmission", func(i *Input) { i.Transmission = "magic" }},
		{"unsafe id", func(i *Input) { i.VehicleID = "../../vehicle" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			creates := 0
			repo := stubRepository{create: func(context.Context, Vehicle, Event) (Vehicle, error) { creates++; return Vehicle{}, nil }}
			input := validInput("auto-001")
			tc.change(&input)
			if _, err := NewService(repo, nil, nil).Create(context.Background(), input); !errors.Is(err, ErrInvalid) || creates != 0 {
				t.Fatalf("err=%v writes=%d", err, creates)
			}
		})
	}
}
func TestFailedWriteDoesNotInvalidateCache(t *testing.T) {
	invalidations := 0
	cache := stubCache{invalidate: func(context.Context, string, int64) error { invalidations++; return nil }}
	repo := stubRepository{get: func(context.Context, string) (Vehicle, error) { return Vehicle{Version: 1}, nil }, update: func(context.Context, Vehicle, int64, Event) (Vehicle, error) { return Vehicle{}, ErrOutboxFull }}
	input := validInput("auto-001")
	input.Version = 1
	if _, err := NewService(repo, cache, nil).Update(context.Background(), "auto-001", input); !errors.Is(err, ErrOutboxFull) || invalidations != 0 {
		t.Fatalf("err=%v invalidations=%d", err, invalidations)
	}
}
func TestHTTPQuoteRestrictedAndStrictJSON(t *testing.T) {
	repo := stubRepository{get: func(context.Context, string) (Vehicle, error) {
		return Vehicle{VehicleID: "auto-001", Currency: "ARS", DailyRateMinor: 5000000, Version: 1}, nil
	}}
	h := Handler(NewService(repo, nil, nil))
	request := httptest.NewRequest("GET", "/v1/vehicles/auto-001/quote", nil)
	request = request.WithContext(platform.WithPrincipal(request.Context(), platform.Principal{App: "client", Role: "client"}))
	response := httptest.NewRecorder()
	h.ServeHTTP(response, request)
	if response.Code != http.StatusForbidden {
		t.Fatalf("client quote status %d", response.Code)
	}
	request = httptest.NewRequest("POST", "/v1/vehicles", strings.NewReader(`{"vehicleId":"auto-001","vehicleId":"auto-002"}`))
	request = request.WithContext(platform.WithPrincipal(request.Context(), platform.Principal{App: "admin", Role: "admin"}))
	response = httptest.NewRecorder()
	h.ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("duplicate JSON status %d", response.Code)
	}
}

func TestHTTPKeepsStableRoutePatternThroughOuterMux(t *testing.T) {
	repo := stubRepository{get: func(_ context.Context, id string) (Vehicle, error) {
		return Vehicle{VehicleID: id, Currency: "ARS", DailyRateMinor: 5000000, Version: 1}, nil
	}}
	outer := http.NewServeMux()
	outer.Handle("/", Handler(NewService(repo, nil, nil)))
	for _, example := range []struct{ path, pattern string }{
		{"/v1/vehicles/auto-001", "GET /v1/vehicles/{id}"},
		{"/v1/vehicles/auto-002", "GET /v1/vehicles/{id}"},
		{"/v1/vehicles/auto-001/quote", "GET /v1/vehicles/{id}/quote"},
	} {
		request := httptest.NewRequest("GET", example.path, nil)
		request = request.WithContext(platform.WithPrincipal(request.Context(), platform.Principal{App: "service", Role: "service"}))
		response := httptest.NewRecorder()
		outer.ServeHTTP(response, request)
		if response.Code != 200 || request.Pattern != example.pattern {
			t.Fatalf("route metadata: status=%d pattern=%q expected=%q", response.Code, request.Pattern, example.pattern)
		}
	}
}
