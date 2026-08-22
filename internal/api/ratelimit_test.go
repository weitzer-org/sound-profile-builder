package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"golang.org/x/time/rate"
)

func TestRateLimitMiddleware_AllowsWithinBurst(t *testing.T) {
	s, _, _, _ := setupTestServer()
	handler := s.rateLimitMiddleware(rate.Every(time.Hour), 3)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	for i := 0; i < 3; i++ {
		req := httptest.NewRequest(http.MethodPost, "/x", nil)
		req.RemoteAddr = "203.0.113.10:1234"
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Errorf("request %d: expected 200 within burst, got %d", i, rr.Code)
		}
	}
}

func TestRateLimitMiddleware_BlocksOverBurst(t *testing.T) {
	s, _, _, _ := setupTestServer()
	handler := s.rateLimitMiddleware(rate.Every(time.Hour), 2)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodPost, "/x", nil)
		req.RemoteAddr = "203.0.113.20:1234"
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("request %d: expected 200 within burst, got %d", i, rr.Code)
		}
	}

	req := httptest.NewRequest(http.MethodPost, "/x", nil)
	req.RemoteAddr = "203.0.113.20:1234"
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusTooManyRequests {
		t.Errorf("expected 429 once burst is exhausted, got %d", rr.Code)
	}
}

func TestRateLimitMiddleware_PerClientIP(t *testing.T) {
	s, _, _, _ := setupTestServer()
	handler := s.rateLimitMiddleware(rate.Every(time.Hour), 1)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	reqA := httptest.NewRequest(http.MethodPost, "/x", nil)
	reqA.RemoteAddr = "203.0.113.30:1234"
	rrA := httptest.NewRecorder()
	handler.ServeHTTP(rrA, reqA)
	if rrA.Code != http.StatusOK {
		t.Fatalf("expected 200 for first client, got %d", rrA.Code)
	}

	// Different client IP -- its own bucket, should not be affected by
	// client A having exhausted its burst of 1.
	reqB := httptest.NewRequest(http.MethodPost, "/x", nil)
	reqB.RemoteAddr = "203.0.113.31:1234"
	rrB := httptest.NewRecorder()
	handler.ServeHTTP(rrB, reqB)
	if rrB.Code != http.StatusOK {
		t.Errorf("expected 200 for a different client IP, got %d", rrB.Code)
	}

	// Client A's second request should now be blocked.
	reqA2 := httptest.NewRequest(http.MethodPost, "/x", nil)
	reqA2.RemoteAddr = "203.0.113.30:1234"
	rrA2 := httptest.NewRecorder()
	handler.ServeHTTP(rrA2, reqA2)
	if rrA2.Code != http.StatusTooManyRequests {
		t.Errorf("expected 429 for client A's second request, got %d", rrA2.Code)
	}
}

func TestClientIP(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.RemoteAddr = "192.0.2.5:54321"
	if got := clientIP(req); got != "192.0.2.5" {
		t.Errorf("expected host portion '192.0.2.5', got %q", got)
	}

	req.RemoteAddr = "not-a-host-port"
	if got := clientIP(req); got != "not-a-host-port" {
		t.Errorf("expected raw fallback %q, got %q", "not-a-host-port", got)
	}
}
