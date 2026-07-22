package server

import (
	"context"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/abhiramnajith/vellum/server/internal/storage"
)

func newServeTestServer(t *testing.T) (*Server, net.Listener) {
	t.Helper()
	srv, err := New(storage.New(t.TempDir()))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	return srv, ln
}

// A server with an idle timeout and no traffic shuts itself down.
func TestServeIdleShutsDown(t *testing.T) {
	srv, ln := newServeTestServer(t)
	done := make(chan error, 1)
	go func() { done <- srv.Serve(context.Background(), ln, 100*time.Millisecond) }()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Serve returned error on idle shutdown: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("server did not idle-shut-down within 3s")
	}
}

// Traffic keeps the server alive past the idle window; it exits cleanly when
// the context is canceled.
func TestServeStaysUpWhileActive(t *testing.T) {
	srv, ln := newServeTestServer(t)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- srv.Serve(ctx, ln, 200*time.Millisecond) }()

	base := "http://" + ln.Addr().String()
	// Hit it every 50ms for 500ms — well past the 200ms idle window.
	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		resp, err := http.Get(base + "/artifacts")
		if err != nil {
			t.Fatalf("server went down while active: %v", err)
		}
		resp.Body.Close()
		time.Sleep(50 * time.Millisecond)
	}

	// Still up — now cancel and expect a clean return.
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Serve returned error on context cancel: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("server did not stop within 3s of context cancel")
	}
}

// idle=0 means "never idle-exit"; the server stops only when the context is
// canceled (the signal path in main).
func TestServeNeverIdleWhenZero(t *testing.T) {
	srv, ln := newServeTestServer(t)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- srv.Serve(ctx, ln, 0) }()

	// With no idle timeout and no traffic, it must still be running after a
	// window that would have tripped any idle logic.
	select {
	case err := <-done:
		t.Fatalf("server exited unexpectedly with idle=0: %v", err)
	case <-time.After(400 * time.Millisecond):
		// good, still up
	}
	cancel()
	if err := <-done; err != nil {
		t.Fatalf("Serve returned error on context cancel: %v", err)
	}
}
