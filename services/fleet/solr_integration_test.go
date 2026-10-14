package fleet

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/Gerpeirone/Car-Cruds/internal/platform"
	"io"
	"net/http"
	"os"
	"testing"
	"time"
)

func TestSolrRealVersionFiltersAndVisibility(t *testing.T) {
	base := os.Getenv("TEST_SOLR_URL")
	if base == "" {
		t.Skip("TEST_SOLR_URL is required for real Solr integration")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()
	engine := NewSolr(base, &http.Client{Timeout: 2 * time.Second})
	if err := engine.Ready(ctx); err != nil {
		t.Fatal(err)
	}
	unique := "fleet-test-" + platform.UUID()
	a := Vehicle{VehicleID: unique + "-a", Brand: unique, Model: "Sedán Demo", Category: "sedan", Transmission: "manual", Seats: 5, DailyRateMinor: 5000000, Currency: "ARS", Active: true, Version: 1, UpdatedAt: time.Now().UTC()}
	b := a
	b.VehicleID = unique + "-b"
	b.DailyRateMinor = 6500000
	t.Cleanup(func() {
		for _, id := range []string{a.VehicleID, b.VehicleID} {
			raw, _ := json.Marshal(map[string]any{"delete": map[string]string{"id": id}})
			cleanup, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			request, err := http.NewRequestWithContext(cleanup, http.MethodPost, base+"/update?commit=true", bytes.NewReader(raw))
			if err == nil {
				request.Header.Set("Content-Type", "application/json")
				response, err := engine.client.Do(request)
				if err == nil {
					_, _ = io.Copy(io.Discard, response.Body)
					response.Body.Close()
				}
			}
			cancel()
		}
	})
	if err := engine.Upsert(ctx, a); err != nil {
		t.Fatal(err)
	}
	if err := engine.Upsert(ctx, b); err != nil {
		t.Fatal(err)
	}
	fresh := a
	fresh.Version = 2
	fresh.DailyRateMinor = 7500000
	if err := engine.Upsert(ctx, fresh); err != nil {
		t.Fatal(err)
	}
	if err := engine.Upsert(ctx, a); err != nil {
		t.Fatal(err)
	}
	current, exists, err := engine.get(ctx, a.VehicleID)
	if err != nil || !exists || current.VehicleVersion != 2 || current.DailyRateMinor != 7500000 {
		t.Fatalf("real index regressed %+v exists=%v err=%v", current, exists, err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		page, err := engine.Search(ctx, Query{Page: 1, PageSize: 1, Brand: unique, Category: "sedan", Seats: 5, Sort: "price_asc"})
		if err != nil {
			t.Fatal(err)
		}
		if page.Total == 2 && len(page.Items) == 1 && page.Items[0].VehicleID == b.VehicleID {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("index not visible within five seconds: %+v", page)
		}
		time.Sleep(100 * time.Millisecond)
	}
	page, err := engine.Search(ctx, Query{Page: 2, PageSize: 1, Brand: unique, Sort: "price_asc"})
	if err != nil || len(page.Items) != 1 || page.Items[0].VehicleID != a.VehicleID {
		t.Fatalf("pagination mismatch %+v err=%v", page, err)
	}
}
