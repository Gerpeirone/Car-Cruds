package fleet

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func TestSolrCASDoesNotRegressAfterConcurrentOrDelayedEvents(t *testing.T) {
	var mu sync.Mutex
	var stored *solrDocument
	nextVersion := int64(100)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/get":
			_ = json.NewEncoder(w).Encode(map[string]any{"doc": stored})
		case "/update":
			var docs []solrDocument
			if err := json.NewDecoder(r.Body).Decode(&docs); err != nil || len(docs) != 1 {
				http.Error(w, "bad update", 400)
				return
			}
			incoming := docs[0]
			if (stored == nil && incoming.SolrVersion != -1) || (stored != nil && incoming.SolrVersion != stored.SolrVersion) {
				w.WriteHeader(409)
				return
			}
			nextVersion++
			incoming.SolrVersion = nextVersion
			stored = &incoming
			_, _ = w.Write([]byte(`{"responseHeader":{"status":0}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	client := NewSolr(server.URL, server.Client())
	old := Vehicle{VehicleID: "auto-001", Version: 1, DailyRateMinor: 5000000, Currency: "ARS", Active: true, UpdatedAt: time.Now().UTC()}
	fresh := old
	fresh.Version = 2
	fresh.DailyRateMinor = 7500000
	var wg sync.WaitGroup
	for _, v := range []Vehicle{old, fresh} {
		wg.Add(1)
		go func(v Vehicle) {
			defer wg.Done()
			if err := client.Upsert(context.Background(), v); err != nil {
				t.Errorf("CAS update: %v", err)
			}
		}(v)
	}
	wg.Wait()
	if err := client.Upsert(context.Background(), old); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	defer mu.Unlock()
	if stored == nil || stored.VehicleVersion != 2 || stored.DailyRateMinor != 7500000 {
		t.Fatalf("index regressed: %+v", stored)
	}
}
func TestSolrSearchSeparatesLiteralFiltersAndPaginates(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		values := r.URL.Query()
		if values.Get("start") != "20" || values.Get("rows") != "20" || values.Get("sort") != "dailyRateMinor_l desc,id asc" {
			t.Errorf("wrong pagination/order: %v", values)
		}
		filters := values["fq"]
		if len(filters) != 2 || filters[0] != "active_b:true" || filters[1] != "{!term f=brand_s}Demo OR *:*" {
			t.Errorf("unsafe filter: %v", filters)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"response":{"numFound":0,"docs":[]}}`))
	}))
	defer server.Close()
	page, err := NewSolr(server.URL, server.Client()).Search(context.Background(), Query{Page: 2, PageSize: 20, Sort: "price_desc", Brand: "Demo OR *:*"})
	if err != nil || page.Page != 2 || page.Items == nil {
		t.Fatalf("page=%+v err=%v", page, err)
	}
}
func TestSolrFailureDoesNotReturnSuccessfulEmptySearch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Error(w, "dependency down", 503) }))
	defer server.Close()
	if _, err := NewSolr(server.URL, server.Client()).Search(context.Background(), Query{Page: 1, PageSize: 20}); err == nil {
		t.Fatal("failed dependency became empty success")
	}
}

func TestMalformedSolrSuccessIsStillUnavailable(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"error":"missing response"}`))
	}))
	defer server.Close()
	engine := NewSolr(server.URL, server.Client())
	if _, err := engine.Search(context.Background(), Query{Page: 1, PageSize: 20}); err == nil {
		t.Fatal("malformed response became empty success")
	}
	if err := engine.Upsert(context.Background(), Vehicle{VehicleID: "auto-001", Version: 1}); err == nil {
		t.Fatal("malformed real-time get became absence")
	}
}
