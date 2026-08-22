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

// maxRateLimiterEntries hard-bounds s.rateLimiters regardless of how fast
// distinct client IPs show up between sweeps -- the periodic sweep alone
// only bounds steady-state size, not a burst within one sweep interval.
const maxRateLimiterEntries = 10000

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
				if len(s.rateLimiters) >= maxRateLimiterEntries {
					s.evictOldestRateLimiterLocked()
				}
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

// evictOldestRateLimiterLocked removes the least-recently-seen entry to
// make room for a new one once maxRateLimiterEntries is reached. Caller
// must hold rateLimitersMu.
// evictionSampleSize bounds evictOldestRateLimiterLocked to an O(1) scan
// instead of walking the full map -- Go's randomized map iteration order
// gives a free random sample, so picking the oldest of a small sample
// approximates true LRU without the lock-contention cost of a full pass.
const evictionSampleSize = 10

func (s *Server) evictOldestRateLimiterLocked() {
	var oldestKey string
	var oldestSeen time.Time
	first := true
	sampled := 0
	for key, entry := range s.rateLimiters {
		if first || entry.lastSeen.Before(oldestSeen) {
			oldestKey = key
			oldestSeen = entry.lastSeen
			first = false
		}
		sampled++
		if sampled >= evictionSampleSize {
			break
		}
	}
	if !first {
		delete(s.rateLimiters, oldestKey)
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

// clientIP identifies the real client for rate-limiting purposes.
//
// In prod (fly.toml's [http_service]) every request reaches this process
// through Fly's edge proxy, so r.RemoteAddr is the proxy's connection, not
// the caller's -- every user would otherwise share one bucket. Fly's edge
// sets Fly-Client-IP itself (https://fly.io/docs/networking/request-headers/)
// on everything it forwards, overwriting any client-supplied value, so it's
// safe to trust here: this app has no other ingress path in prod. Local dev
// (go run / MOCK_MODE, no Fly proxy in front) never sees that header, so it
// falls back to r.RemoteAddr's host portion, raw if that isn't host:port
// form (e.g. in some test harnesses).
func clientIP(r *http.Request) string {
	if flyIP := r.Header.Get("Fly-Client-IP"); flyIP != "" {
		return flyIP
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
