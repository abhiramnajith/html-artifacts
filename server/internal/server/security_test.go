package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRejectsNonLoopbackHost(t *testing.T) {
	h, _ := newTestServer(t)
	req := httptest.NewRequest("GET", "/artifacts", nil)
	req.Host = "evil.example.com" // DNS-rebinding: browser connected to 127.0.0.1, Host is attacker domain
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("non-loopback Host: want 403, got %d", rec.Code)
	}
}

func TestAllowsLoopbackHosts(t *testing.T) {
	h, _ := newTestServer(t)
	for _, host := range []string{"127.0.0.1:47600", "localhost:47600", "127.0.0.1", "[::1]:47600"} {
		req := httptest.NewRequest("GET", "/artifacts", nil)
		req.Host = host
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code == http.StatusForbidden {
			t.Fatalf("loopback Host %q was rejected", host)
		}
	}
}

func TestRejectsCrossOriginPost(t *testing.T) {
	h, id := newTestServer(t)
	req := httptest.NewRequest("POST", "/annotations/"+id, strings.NewReader(`{"annotations":[]}`))
	req.Host = "127.0.0.1:47600"
	req.Header.Set("Origin", "https://evil.example.com")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("cross-origin POST: want 403, got %d", rec.Code)
	}
}

func TestAllowsSameOriginPost(t *testing.T) {
	h, id := newTestServer(t)
	req := httptest.NewRequest("POST", "/annotations/"+id, strings.NewReader(`{"annotations":[]}`))
	req.Host = "127.0.0.1:47600"
	req.Header.Set("Origin", "http://127.0.0.1:47600")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code == http.StatusForbidden {
		t.Fatalf("same-origin POST was rejected (%d)", rec.Code)
	}
}
