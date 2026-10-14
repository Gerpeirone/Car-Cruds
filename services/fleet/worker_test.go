package fleet

import (
	"context"
	"errors"
	"io"
	"net"
	"testing"
	"time"
)

func TestPublisherDeadlineInterruptsBlockedSocketWrite(t *testing.T) {
	socket, peer := net.Pipe()
	defer socket.Close()
	defer peer.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	started := time.Now()
	err := withPublisherDeadline(ctx, socket, func(context.Context) error {
		// The peer never reads, so this models a publish stalled in real I/O.
		_, err := socket.Write([]byte("pending event"))
		return err
	})
	if err == nil || time.Since(started) > time.Second {
		t.Fatalf("blocked publish must fail within its budget: err=%v elapsed=%s", err, time.Since(started))
	}
	if _, err = socket.Write([]byte("must reconnect")); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatalf("failed session transport remains open: %v", err)
	}
}

func TestPublisherCancellationInterruptsBlockedSocketWrite(t *testing.T) {
	socket, peer := net.Pipe()
	defer socket.Close()
	defer peer.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	startedWrite := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		done <- withPublisherDeadline(ctx, socket, func(context.Context) error {
			close(startedWrite)
			_, err := socket.Write([]byte("pending event"))
			return err
		})
	}()
	<-startedWrite
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("expected cancellation, got %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("cancellation left the publisher blocked in Write")
	}
}

func TestPublisherSuccessRestoresReusableTransport(t *testing.T) {
	socket, peer := net.Pipe()
	defer socket.Close()
	defer peer.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	reader := make(chan error, 1)
	go func() {
		_, err := io.ReadFull(peer, make([]byte, 2))
		reader <- err
	}()
	if err := withPublisherDeadline(ctx, socket, func(context.Context) error {
		_, err := socket.Write([]byte("a"))
		return err
	}); err != nil {
		t.Fatal(err)
	}
	// A completed operation must clear its deadline and unregister cancellation;
	// otherwise the next event would inherit an expired budget or closed socket.
	<-ctx.Done()
	if _, err := socket.Write([]byte("b")); err != nil {
		t.Fatalf("successful publish did not leave a reusable transport: %v", err)
	}
	if err := <-reader; err != nil {
		t.Fatal(err)
	}
}
