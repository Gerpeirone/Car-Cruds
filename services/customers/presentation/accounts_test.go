package presentation_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Gerpeirone/Car-Cruds/services/customers/business"
	"github.com/Gerpeirone/Car-Cruds/services/customers/presentation"
)

// blockingStore holds every account lookup until released, so both hash
// slots stay busy for as long as the test needs.
type blockingStore struct {
	entered chan struct{}
	release chan struct{}
}

func (s blockingStore) InsertAccount(context.Context, business.Account, string) error { return nil }
func (s blockingStore) AccountByEmail(ctx context.Context, _ string) (business.Account, string, error) {
	s.entered <- struct{}{}
	<-s.release
	return business.Account{}, "", business.ErrAccountAuth
}
func (s blockingStore) InsertSession(context.Context, string, []byte, time.Time) error { return nil }
func (s blockingStore) AccountBySession(context.Context, []byte) (business.Account, error) {
	return business.Account{}, business.ErrAccountAuth
}
func (s blockingStore) DeleteSession(context.Context, []byte) error { return nil }

func login(h http.Handler, email string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(http.MethodPost, "/v1/auth/login", strings.NewReader(`{"email":"`+email+`","password":"incorrecta"}`))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

// Saturated hashing is a momentary capacity limit (503 AUTH_BUSY), not the
// per-address attempt limit (429 AUTH_RATE_LIMITED), and it does not consume
// the caller's attempts.
func TestBusyHashingIsNotReportedAsTooManyAttempts(t *testing.T) {
	store := blockingStore{entered: make(chan struct{}), release: make(chan struct{})}
	h := presentation.NewAccountHandler(business.Accounts{Store: store, TTL: time.Hour})
	var wg sync.WaitGroup
	for _, email := range []string{"uno@example.test", "dos@example.test"} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			login(h, email)
		}()
		<-store.entered
	}
	busy := login(h, "tres@example.test")
	if busy.Code != 503 || !strings.Contains(busy.Body.String(), `"AUTH_BUSY"`) || busy.Header().Get("Retry-After") != "1" {
		t.Fatalf("busy=%d %s %q", busy.Code, busy.Body, busy.Header().Get("Retry-After"))
	}
	close(store.release)
	wg.Wait()
	go func() {
		for range store.entered {
		}
	}()
	for i := range 8 {
		if w := login(h, "tres@example.test"); w.Code != 401 {
			t.Fatalf("attempt %d=%d %s", i+1, w.Code, w.Body)
		}
	}
	if w := login(h, "TRES@example.test"); w.Code != 429 || !strings.Contains(w.Body.String(), `"AUTH_RATE_LIMITED"`) {
		t.Fatalf("limit=%d %s", w.Code, w.Body)
	}
}
