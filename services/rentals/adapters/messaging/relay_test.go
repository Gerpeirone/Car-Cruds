package messaging

import (
	"context"
	"errors"
	"net"
	"os"
	"testing"
	"time"
)

func TestPublicationDeadlineUnblocksAStalledTransport(t *testing.T) {
	writer, peer := net.Pipe()
	defer writer.Close()
	defer peer.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 80*time.Millisecond)
	defer cancel()
	start := time.Now()
	err := withWriteDeadline(ctx, writer, func() error { _, err := writer.Write([]byte("frame whose peer does not read")); return err })
	if err == nil || (!errors.Is(err, os.ErrDeadlineExceeded) && !errors.Is(ctx.Err(), context.DeadlineExceeded)) {
		t.Fatalf("blocked write did not time out: %v", err)
	}
	if time.Since(start) > 700*time.Millisecond {
		t.Fatal("transport deadline did not bound publication")
	}
	// No publisher goroutine was spawned: the blocked Write returned directly.
}

func TestCancelledPublicationClosesAStalledTransport(t *testing.T) {
	writer, peer := net.Pipe()
	defer writer.Close()
	defer peer.Close()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	trigger := time.AfterFunc(40*time.Millisecond, cancel)
	defer trigger.Stop()
	start := time.Now()
	err := withWriteDeadline(ctx, writer, func() error { _, err := writer.Write([]byte("stalled frame")); return err })
	if err == nil || ctx.Err() != context.Canceled {
		t.Fatal("cancellation did not interrupt the transport", err, ctx.Err())
	}
	if time.Since(start) > 700*time.Millisecond {
		t.Fatal("cancelled publication waited for its original timeout")
	}
}

func TestSuccessfulPublicationClearsTheWriteDeadline(t *testing.T) {
	writer, peer := net.Pipe()
	defer writer.Close()
	defer peer.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	if err := withWriteDeadline(ctx, writer, func() error { return nil }); err != nil {
		t.Fatal(err)
	}
	<-ctx.Done()
	cancel()
	done := make(chan error, 1)
	go func() { buffer := make([]byte, 1); _, err := peer.Read(buffer); done <- err }()
	if _, err := writer.Write([]byte("x")); err != nil {
		t.Fatal("successful publication left a stale transport deadline", err)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}
