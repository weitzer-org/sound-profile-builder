package api

import (
	"net"
	"net/http"
	"time"

	"golang.org/x/time/rate"
)

// rateLimiterEntry pairs a client's token bucket with when it was last
// used, so the sweeper (see sweepRateLimiters) can evict buckets for
// clients that stopped showing up instead of growing s.rateLimiters
// forever.
type rateLimiterEntry struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

const rateLimiterTTL = 10 * time.Minute
const rateLimiterSweepInterval = 5 * time.Minute

// rateLimitMiddleware throttles requests per client IP using a token
// bucket, so a looping client (or a leaked session) can't hammer a metered
// Gemini/OpenLLM-backed endpoint for free. Deliberately generous (burst
// well above normal interactive use) -- the goal is to stop a runaway loop,
// not to police normal retries.
func (s *Server) rateLimitMiddleware(limit rate.Limit, burst int) func(http.Handler) http.Handler {
	s.startRateLimiterSweeper()

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := clientIP(r)
			now := time.Now()

			s.rateLimitersMu.Lock()
			entry, ok := s.rateLimiters[key]
			if !ok {
				entry = &rateLimiterEntry{limiter: rate.NewLimiter(limit, burst)}
				s.rateLimiters[key] = entry
			}
			entry.lastSeen = now
			limiter := entry.limiter
			s.rateLimitersMu.Unlock()

			if !limiter.Allow() {
				http.Error(w, "Too many requests, please slow down.", http.StatusTooManyRequests)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// startRateLimiterSweeper starts (once per Server) a background goroutine
// that periodically evicts rate limiter entries for clients that haven't
// been seen in a while, so s.rateLimiters doesn't grow without bound over
// the life of a long-running process.
func (s *Server) startRateLimiterSweeper() {
	s.rateLimiterSweepOnce.Do(func() {
		go func() {
			ticker := time.NewTicker(rateLimiterSweepInterval)
			defer ticker.Stop()
			for now := range ticker.C {
				s.sweepRateLimiters(now)
			}
		}()
	})
}

// sweepRateLimiters removes entries last used before now-rateLimiterTTL.
// Split out from startRateLimiterSweeper's ticker loop so tests can drive
// eviction deterministically instead of waiting on a real ticker.
func (s *Server) sweepRateLimiters(now time.Time) {
	cutoff := now.Add(-rateLimiterTTL)
	s.rateLimitersMu.Lock()
	defer s.rateLimitersMu.Unlock()
	for key, entry := range s.rateLimiters {
		if entry.lastSeen.Before(cutoff) {
			delete(s.rateLimiters, key)
		}
	}
}

// clientIP extracts the host portion of r.RemoteAddr, falling back to the
// raw value if it isn't in host:port form (e.g. in some test harnesses).
func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
