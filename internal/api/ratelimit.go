package api

import (
	"net"
	"net/http"

	"golang.org/x/time/rate"
)

// rateLimitMiddleware throttles requests per client IP using a token
// bucket, so a looping client (or a leaked session) can't hammer a metered
// Gemini/OpenLLM-backed endpoint for free. Deliberately generous (burst
// well above normal interactive use) -- the goal is to stop a runaway loop,
// not to police normal retries.
func (s *Server) rateLimitMiddleware(limit rate.Limit, burst int) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := clientIP(r)

			s.rateLimitersMu.Lock()
			limiter, ok := s.rateLimiters[key]
			if !ok {
				limiter = rate.NewLimiter(limit, burst)
				s.rateLimiters[key] = limiter
			}
			s.rateLimitersMu.Unlock()

			if !limiter.Allow() {
				http.Error(w, "Too many requests, please slow down.", http.StatusTooManyRequests)
				return
			}

			next.ServeHTTP(w, r)
		})
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
