package fleet

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

// Use the actual Redis SDK and local transport failures: a nil result paired
// with an error must never be classified as a successful cache miss.
func TestDriverCacheReadFailuresAreErrors(t *testing.T) {
	t.Run("closed Redis client", func(t *testing.T) {
		cache := NewRedisCache("127.0.0.1:1", "", "fleet_driver_test")
		if err := cache.Close(); err != nil {
			t.Fatal("could not close Redis client")
		}
		ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
		defer cancel()
		if _, hit, err := cache.Get(ctx, "auto-001"); err == nil || hit {
			t.Fatalf("closed client classified as a miss: hit=%v errorPresent=%v", hit, err != nil)
		}
	})
	t.Run("unreachable loopback endpoint", func(t *testing.T) {
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal("could not allocate a controlled local port")
		}
		addr := listener.Addr().String()
		if err = listener.Close(); err != nil {
			t.Fatal("could not close the controlled local port")
		}
		cache := NewRedisCache(addr, "", "fleet_driver_test")
		defer cache.Close()
		ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
		defer cancel()
		started := time.Now()
		if _, hit, err := cache.Get(ctx, "auto-001"); err == nil || hit {
			t.Fatalf("unreachable Redis classified as a miss: hit=%v errorPresent=%v", hit, err != nil)
		}
		if elapsed := time.Since(started); elapsed > 300*time.Millisecond {
			t.Fatalf("failed dial exceeded the shared 150ms context with tolerance: %s", elapsed)
		}
	})
}

type readDeadlineProbe struct {
	net.Conn
	deadlines chan time.Time
}

func (c *readDeadlineProbe) SetReadDeadline(deadline time.Time) error {
	c.deadlines <- deadline
	return c.Conn.SetReadDeadline(deadline)
}

func TestDriverSilentRedisHonorsActualContextDeadline(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal("could not allocate controlled local TCP endpoint")
	}
	release := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		// Accept transport but send no RESP reply, forcing a real socket read.
		<-release
	}()
	defer func() { _ = listener.Close(); close(release); <-done }()
	cache := NewRedisCache(listener.Addr().String(), "", "fleet_driver_deadline_test")
	options := *cache.Client.Options()
	_ = cache.Close()
	// NewClient attaches mutable push handlers to Options. Give the observed
	// client its own registry rather than copying handlers from the closed one.
	options.PushNotificationProcessor = nil
	deadlines := make(chan time.Time, 8)
	options.Dialer = func(ctx context.Context, network, addr string) (net.Conn, error) {
		conn, err := (&net.Dialer{Timeout: options.DialTimeout}).DialContext(ctx, network, addr)
		if err != nil {
			return nil, err
		}
		return &readDeadlineProbe{Conn: conn, deadlines: deadlines}, nil
	}
	// Preserve the constructor options; only observe its real TCP deadline.
	cache.Client = redis.NewClient(&options)
	defer cache.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 75*time.Millisecond)
	defer cancel()
	expectedDeadline, _ := ctx.Deadline()
	started := time.Now()
	if _, hit, err := cache.Get(ctx, "auto-001"); err == nil || hit {
		t.Fatal("silent TCP peer must produce a cache read error")
	}
	elapsed := time.Since(started)
	if elapsed > 300*time.Millisecond {
		t.Fatalf("silent peer exceeded the bounded context with tolerance: %s", elapsed)
	}
	observed := 0
	for {
		select {
		case deadline := <-deadlines:
			if deadline.IsZero() || deadline.After(expectedDeadline) {
				t.Fatal("Redis SDK installed a socket deadline later than the caller budget")
			}
			observed++
		default:
			if observed == 0 {
				t.Fatal("test did not observe a real socket read deadline")
			}
			t.Logf("silent TCP returned in %s; %d socket read deadline(s) respected the 75ms caller context", elapsed, observed)
			return
		}
	}
}
