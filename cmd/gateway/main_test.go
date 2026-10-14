package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

// Cuatro dependencias sanas pueden consumir juntas más que el presupuesto
// de la sonda. Deben verificarse en paralelo dentro de un mismo deadline.
func TestReadinessUsesSharedBudgetWithoutSequentialFalseFailures(t *testing.T) {
	var calls atomic.Int32
	for _, key := range []string{"CUSTOMERS_URL", "FLEET_URL", "RENTALS_URL", "EMPLOYEES_URL"} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/internal/ready" || r.Header.Get("Authorization") != "Bearer probe-token" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			select {
			case <-time.After(350 * time.Millisecond):
				calls.Add(1)
				w.WriteHeader(http.StatusOK)
			case <-r.Context().Done():
			}
		}))
		t.Cleanup(server.Close)
		t.Setenv(key, server.URL)
	}
	t.Setenv("AUTH_SERVICE_TOKEN", "probe-token")
	_, ready, err := handler()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := ready(ctx); err != nil {
		t.Fatalf("dependencias sanas rechazadas: %v", err)
	}
	if calls.Load() != 4 {
		t.Fatalf("sondas completadas: %d", calls.Load())
	}
}

func TestReadinessCancelsUnresponsiveDependencies(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { <-r.Context().Done() }))
	defer server.Close()
	for _, key := range []string{"CUSTOMERS_URL", "FLEET_URL", "RENTALS_URL", "EMPLOYEES_URL"} {
		t.Setenv(key, server.URL)
	}
	_, ready, err := handler()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	started := time.Now()
	if err := ready(ctx); err == nil {
		t.Fatal("dependencias sin respuesta declaradas sanas")
	}
	if time.Since(started) > 500*time.Millisecond {
		t.Fatal("se excedió el presupuesto de cancelación")
	}
}

func TestProxyPreservesCallerAndNormalizesUpstreamFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer caller-token" || r.Header.Get("X-Request-ID") != "gateway-test" || r.URL.Path != "/v1/vehicles/auto-1" {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer server.Close()
	h, err := proxy(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest(http.MethodGet, "/v1/vehicles/auto-1", nil)
	r.Header.Set("Authorization", "Bearer caller-token")
	r.Header.Set("X-Request-ID", "gateway-test")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusServiceUnavailable || w.Header().Get("Content-Type") != "application/json" {
		t.Fatalf("respuesta no normalizada: %d %s", w.Code, w.Body.String())
	}
	if w.Header().Get("X-Request-ID") != "gateway-test" {
		t.Fatal("error de upstream sin correlación")
	}
}

func TestProxyReturnsOnlyBackendCorrelationHeaders(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Service-Instance", "fleet-a")
		w.Header().Set("X-Request-ID", r.Header.Get("X-Request-ID"))
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()
	h, err := proxy(backend.URL)
	if err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest(http.MethodGet, "/v1/vehicles", nil)
	r.Header.Set("X-Request-ID", "correlated-request")
	w := httptest.NewRecorder()
	w.Header().Set("X-Service-Instance", "gateway")
	w.Header().Set("X-Request-ID", "correlated-request")
	h.ServeHTTP(w, r)
	if values := w.Header().Values("X-Service-Instance"); len(values) != 1 || values[0] != "fleet-a" {
		t.Fatalf("instancia ambigua: %v", values)
	}
	if values := w.Header().Values("X-Request-ID"); len(values) != 1 || values[0] != "correlated-request" {
		t.Fatalf("correlación ambigua: %v", values)
	}
}
