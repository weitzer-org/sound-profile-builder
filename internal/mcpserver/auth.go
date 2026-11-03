package mcpserver

import (
	"crypto/sha256"
	"crypto/subtle"
	"log"
	"net/http"
	"os"
	"strings"
)

// MinTokenLen is the shortest MCP_TOKEN accepted. `openssl rand -hex 32` gives 64.
const MinTokenLen = 32

// Fixed route labels: every log line in this package names the route with one
// of these, never r.URL (the /mcp/{token} form carries the credential in the path).
const (
	routeHeader = "/mcp"
	routePath   = "/mcp/{token}"
	routeOther  = "/mcp/*"
)

// Config is the MCP server's auth configuration.
type Config struct {
	// Token is the shared secret (MCP_TOKEN). Unset or shorter than MinTokenLen
	// disables MCP: its routes stay mounted but reject every request.
	Token string
	// TokenNext (MCP_TOKEN_NEXT) is accepted alongside Token during a rotation.
	TokenNext string
}

// ConfigFromEnv reads MCP_TOKEN and MCP_TOKEN_NEXT.
func ConfigFromEnv() Config {
	return Config{
		Token:     strings.TrimSpace(os.Getenv("MCP_TOKEN")),
		TokenNext: strings.TrimSpace(os.Getenv("MCP_TOKEN_NEXT")),
	}
}

// Enabled reports whether a usable token is configured.
func (c Config) Enabled() bool { return len(c.Token) >= MinTokenLen }

// verifier compares SHA-256 digests of the presented and configured tokens.
// Hashing first means the comparison time does not depend on the token length,
// and ConstantTimeCompare means it does not depend on where the bytes differ.
type verifier struct {
	cur, next       [sha256.Size]byte
	curSet, nextSet int // 1 when that token is configured and long enough
}

func newVerifier(c Config) *verifier {
	v := &verifier{}
	// A short or missing current token disables the whole endpoint, even if
	// TokenNext is set: rotation only makes sense alongside a valid MCP_TOKEN.
	if c.Enabled() {
		v.cur, v.curSet = sha256.Sum256([]byte(c.Token)), 1
		if len(c.TokenNext) >= MinTokenLen {
			v.next, v.nextSet = sha256.Sum256([]byte(c.TokenNext)), 1
		}
	}
	return v
}

// ok reports whether presented matches a configured token. It always performs
// both comparisons (no early exit), and never matches when nothing is configured.
func (v *verifier) ok(presented string) bool {
	h := sha256.Sum256([]byte(presented))
	a := subtle.ConstantTimeCompare(h[:], v.cur[:]) & v.curSet
	b := subtle.ConstantTimeCompare(h[:], v.next[:]) & v.nextSet
	return (a | b) == 1
}

// bearer extracts the token from an "Authorization: Bearer <token>" header.
func bearer(r *http.Request) string {
	h := r.Header.Get("Authorization")
	const prefix = "bearer "
	if len(h) > len(prefix) && strings.EqualFold(h[:len(prefix)], prefix) {
		return strings.TrimSpace(h[len(prefix):])
	}
	return ""
}

// deny writes the bare 401. It deliberately carries no resource_metadata
// parameter: that is what points an MCP client at an OAuth flow, and there is
// none here (DESIGN §4). The body is empty so nothing is echoed back.
func deny(w http.ResponseWriter) {
	w.Header().Set("WWW-Authenticate", `Bearer realm="qc2-mcp"`)
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusUnauthorized)
}

// RequireToken guards next with the static MCP token, accepted either as
// "Authorization: Bearer <token>" or (on the /mcp/{token} route) as the path
// segment. route is a fixed label used in logs. With no usable token configured
// every request is rejected (fail closed).
func RequireToken(cfg Config, route string) func(http.Handler) http.Handler {
	v := newVerifier(cfg)
	disabled := !cfg.Enabled()
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if disabled {
				log.Printf("mcp: rejected request route=%s reason=disabled", route)
				deny(w)
				return
			}
			// Evaluate both sources unconditionally so timing does not reveal
			// which one was presented.
			okHeader := v.ok(bearer(r))
			okPath := v.ok(r.PathValue("token"))
			if !okHeader && !okPath {
				log.Printf("mcp: rejected request route=%s reason=bad_or_missing_token", route)
				deny(w)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
