package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestSearchFreshnessWaitsForExpectedVersion(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Request-ID", r.Header.Get("X-Request-ID"))
		w.Header().Set("X-Service-Instance", "fleet-test")
		w.Header().Set("Content-Type", "application/json")
		version := 1
		if calls.Add(1) >= 3 {
			version = 2
		}
		fmt.Fprintf(w, `{"items":[{"vehicleId":"test-vehicle","brand":"Test","version":%d,"dailyRateMinor":200,"active":true}],"total":1}`, version)
	}))
	defer server.Close()
	h := &harness{settings: settings{gateway: server.URL, client: "test-token"}, clientHTTP: server.Client(), vehicleID: "test-vehicle", vehicle: vehicle{ID: "test-vehicle", Brand: "Test", Version: 2, DailyRateMinor: 200, Active: true}}
	if err := h.searchVisible(context.Background(), "actualización", true, time.Now()); err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 3 {
		t.Fatalf("se aceptó una versión vieja o se consultó de más: %d", calls.Load())
	}
}

func TestSearchFreshnessIncludesWriteTimeInDeadline(t *testing.T) {
	h := &harness{settings: settings{gateway: "http://127.0.0.1:1", client: "test-token"}, clientHTTP: &http.Client{Timeout: time.Second}, vehicleID: "test-vehicle", vehicle: vehicle{ID: "test-vehicle", Brand: "Test", Version: 1, DailyRateMinor: 200, Active: true}}
	err := h.searchVisible(context.Background(), "alta", true, time.Now().Add(-6*time.Second))
	if err == nil || !strings.Contains(err.Error(), "fallida") {
		t.Fatalf("se ignoró el presupuesto vencido antes de consultar: %v", err)
	}
}
