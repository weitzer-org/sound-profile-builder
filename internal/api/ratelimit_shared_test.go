package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// RateLimit() is called once per front end (the UI routes, the MCP mount). Every
// wrapper it returns must draw from the same per-IP buckets, or a client could
// get the burst twice by alternating between the UI and MCP.
func TestRateLimit_WrappersShareBuckets(t *testing.T) {
	s, _, _, _ := setupTestServer()
	ok := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })
	ui, mcp := s.RateLimit()(ok), s.RateLimit()(ok)

	hit := func(h http.Handler, ip string) int {
		req := httptest.NewRequest(http.MethodPost, "/x", nil)
		req.RemoteAddr = ip + ":1234"
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		return rr.Code
	}

	// Spend the whole burst through the first wrapper. The burst size is not assumed:
	// keep going until the limiter itself says 429 (the refill rate is far too slow to
	// matter within this loop).
	exhausted := false
	for i := 0; i < 200; i++ {
		c := hit(ui, "203.0.113.50")
		if c == http.StatusTooManyRequests {
			exhausted = true
			break
		}
		if c != http.StatusOK {
			t.Fatalf("request %d: unexpected status %d", i, c)
		}
	}
	if !exhausted {
		t.Fatal("never hit the limit through the first wrapper within 200 requests")
	}
	if c := hit(mcp, "203.0.113.50"); c != http.StatusTooManyRequests {
		t.Errorf("second wrapper let the same IP through after its burst was spent: got %d, want 429", c)
	}
	if c := hit(mcp, "203.0.113.51"); c != http.StatusOK {
		t.Errorf("a different IP must have its own bucket: got %d", c)
	}
}
