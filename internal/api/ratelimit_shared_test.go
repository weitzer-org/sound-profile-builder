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

	for i := 0; i < 10; i++ { // the full burst, through the first wrapper
		if c := hit(ui, "203.0.113.50"); c != http.StatusOK {
			t.Fatalf("request %d within burst: got %d", i, c)
		}
	}
	if c := hit(mcp, "203.0.113.50"); c != http.StatusTooManyRequests {
		t.Errorf("second wrapper let the same IP through after its burst was spent: got %d, want 429", c)
	}
	if c := hit(mcp, "203.0.113.51"); c != http.StatusOK {
		t.Errorf("a different IP must have its own bucket: got %d", c)
	}
}
