package api

import (
	"fmt"
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

func TestSweepRateLimiters_EvictsStaleNotFresh(t *testing.T) {
	s, _, _, _ := setupTestServer()
	now := time.Now()

	s.rateLimiters = map[string]*rateLimiterEntry{
		"stale":  {limiter: rate.NewLimiter(rate.Every(time.Hour), 1), lastSeen: now.Add(-rateLimiterTTL - time.Minute)},
		"fresh":  {limiter: rate.NewLimiter(rate.Every(time.Hour), 1), lastSeen: now.Add(-time.Minute)},
		"border": {limiter: rate.NewLimiter(rate.Every(time.Hour), 1), lastSeen: now.Add(-rateLimiterTTL + time.Minute)},
	}

	s.sweepRateLimiters(now)

	if _, ok := s.rateLimiters["stale"]; ok {
		t.Errorf("expected stale entry (last seen beyond TTL) to be evicted")
	}
	if _, ok := s.rateLimiters["fresh"]; !ok {
		t.Errorf("expected fresh entry to survive the sweep")
	}
	if _, ok := s.rateLimiters["border"]; !ok {
		t.Errorf("expected entry just inside the TTL to survive the sweep")
	}
}

func TestRateLimitMiddleware_UpdatesLastSeen(t *testing.T) {
	s, _, _, _ := setupTestServer()
	handler := s.rateLimitMiddleware(rate.Every(time.Hour), 5)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodPost, "/x", nil)
	req.RemoteAddr = "203.0.113.40:1234"
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	s.rateLimitersMu.Lock()
	entry, ok := s.rateLimiters["203.0.113.40"]
	s.rateLimitersMu.Unlock()
	if !ok {
		t.Fatalf("expected an entry to be recorded for the client IP")
	}
	if time.Since(entry.lastSeen) > time.Minute {
		t.Errorf("expected lastSeen to be updated to roughly now, got %v", entry.lastSeen)
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

func TestClientIP_PrefersFlyClientIPHeader(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.RemoteAddr = "192.0.2.5:54321" // what RemoteAddr looks like behind Fly's proxy -- not the real caller
	req.Header.Set("Fly-Client-IP", "203.0.113.99")

	if got := clientIP(req); got != "203.0.113.99" {
		t.Errorf("expected Fly-Client-IP to take precedence over RemoteAddr, got %q", got)
	}
}

func TestRateLimitMiddleware_CapsMapSizeUnderChurn(t *testing.T) {
	s, _, _, _ := setupTestServer()
	handler := s.rateLimitMiddleware(rate.Every(time.Hour), 1)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Simulate many distinct clients within one sweep window (the sweeper
	// never fires in a unit test), enough to exceed maxRateLimiterEntries,
	// and confirm the map never grows past that bound.
	for i := 0; i < maxRateLimiterEntries+50; i++ {
		req := httptest.NewRequest(http.MethodPost, "/x", nil)
		req.RemoteAddr = fmt.Sprintf("10.0.%d.%d:1234", (i>>8)&0xFF, i&0xFF) // unique per i
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
	}

	s.rateLimitersMu.Lock()
	size := len(s.rateLimiters)
	s.rateLimitersMu.Unlock()

	if size > maxRateLimiterEntries {
		t.Errorf("expected rateLimiters to stay capped at %d entries, got %d", maxRateLimiterEntries, size)
	}
}
