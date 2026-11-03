package mcpserver

import (
	"bytes"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

// authTokenNext is a second placeholder token (64 chars), for MCP_TOKEN_NEXT.
const authTokenNext = "fedcba9876543210fedcba9876543210fedcba9876543210fedcba9876543210"

// authWrongToken is distinctive so any leak into logs or responses is easy to spot.
var authWrongToken = "WRONGTOKEN-" + strings.Repeat("Z9", 32)

// authInitBody is a real MCP initialize request.
const authInitBody = `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"t","version":"1"}}}`

// authDiscoveryPaths are every discovery route Mount registers (server.go).
var authDiscoveryPaths = []string{
	"/.well-known/oauth-protected-resource",
	"/.well-known/oauth-protected-resource/mcp",
	"/.well-known/oauth-authorization-server",
	"/.well-known/oauth-authorization-server/mcp",
	"/.well-known/openid-configuration",
	"/.well-known/openid-configuration/mcp",
	"/mcp/.well-known/oauth-protected-resource",
	"/mcp/.well-known/oauth-authorization-server",
	"/mcp/.well-known/openid-configuration",
}

// authServer mounts MCP on a fresh mux, exactly as the app does, and serves it.
func authServer(t *testing.T, cfg Config, limit func(http.Handler) http.Handler) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	Mount(mux, newTestService(t).svc, cfg, limit)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

type authResult struct {
	status int
	header http.Header
	body   []byte
}

// authSend makes one request and reads the whole response. A non-empty auth is
// sent as the Authorization header; a non-empty body gets MCP's JSON headers.
func authSend(t *testing.T, srv *httptest.Server, method, path, auth, body string) authResult {
	t.Helper()
	var rd io.Reader
	if body != "" {
		rd = strings.NewReader(body)
	}
	req, err := http.NewRequest(method, srv.URL+path, rd)
	if err != nil {
		t.Fatal(err)
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json, text/event-stream")
	}
	if auth != "" {
		req.Header.Set("Authorization", auth)
	}
	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	return authResult{status: resp.StatusCode, header: resp.Header, body: b}
}

// TestCredentialMatrix covers missing / wrong / short / correct / scheme-case
// credentials on the header route and on the path route.
func TestCredentialMatrix(t *testing.T) {
	srv := authServer(t, Config{Token: testToken}, nil)
	short := testToken[:MinTokenLen-1]
	tests := []struct {
		name, path, auth string
		want401          bool
	}{
		// Header route: /mcp with Authorization.
		{"header/missing", "/mcp", "", true},
		{"header/wrong", "/mcp", "Bearer " + authWrongToken, true},
		{"header/short", "/mcp", "Bearer " + short, true},
		{"header/correct", "/mcp", "Bearer " + testToken, false},
		{"header/correct-lowercase-scheme", "/mcp", "bearer " + testToken, false},
		{"header/correct-mixedcase-scheme", "/mcp", "BeArEr " + testToken, false},
		// Path route: the token is the path segment. "missing" is the bare /mcp/
		// with no segment, which falls through to the deeper-path catch-all.
		{"path/missing", "/mcp/", "", true},
		{"path/wrong", "/mcp/" + authWrongToken, "", true},
		{"path/short", "/mcp/" + short, "", true},
		{"path/correct", "/mcp/" + testToken, "", false},
		// RequireToken accepts either source, so a correct bearer header is
		// honoured on the path route too (wrong path segment, correct header).
		{"path/correct-header-lowercase-scheme", "/mcp/" + authWrongToken, "bearer " + testToken, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := authSend(t, srv, http.MethodPost, tc.path, tc.auth, authInitBody)
			if tc.want401 {
				if r.status != http.StatusUnauthorized {
					t.Fatalf("status = %d, want 401", r.status)
				}
				return
			}
			if r.status != http.StatusOK {
				t.Fatalf("status = %d, want 200; body=%q", r.status, r.body)
			}
		})
	}
}

// TestTokenNext checks MCP_TOKEN_NEXT acceptance and the minimum-length rule.
func TestTokenNext(t *testing.T) {
	both := authServer(t, Config{Token: testToken, TokenNext: authTokenNext}, nil)
	shortNext := authServer(t, Config{Token: testToken, TokenNext: authTokenNext[:MinTokenLen-1]}, nil)
	minNext := authServer(t, Config{Token: testToken, TokenNext: authTokenNext[:MinTokenLen]}, nil)
	tests := []struct {
		name    string
		srv     *httptest.Server
		path    string
		auth    string
		want401 bool
	}{
		{"next/header", both, "/mcp", "Bearer " + authTokenNext, false},
		{"next/path", both, "/mcp/" + authTokenNext, "", false},
		{"current/header", both, "/mcp", "Bearer " + testToken, false},
		{"current/path", both, "/mcp/" + testToken, "", false},
		{"unrelated/header", both, "/mcp", "Bearer " + authWrongToken, true},
		{"unrelated/path", both, "/mcp/" + authWrongToken, "", true},
		// A TokenNext shorter than MinTokenLen is ignored, not accepted.
		{"short-next-ignored/header", shortNext, "/mcp", "Bearer " + authTokenNext[:MinTokenLen-1], true},
		{"short-next-ignored/path", shortNext, "/mcp/" + authTokenNext[:MinTokenLen-1], "", true},
		{"short-next-ignored/current-still-works", shortNext, "/mcp", "Bearer " + testToken, false},
		{"short-next-ignored/full-next-not-configured", shortNext, "/mcp", "Bearer " + authTokenNext, true},
		// Exactly MinTokenLen is the smallest accepted TokenNext.
		{"min-next/path", minNext, "/mcp/" + authTokenNext[:MinTokenLen], "", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := authSend(t, tc.srv, http.MethodPost, tc.path, tc.auth, authInitBody)
			if tc.want401 {
				if r.status != http.StatusUnauthorized {
					t.Fatalf("status = %d, want 401", r.status)
				}
				return
			}
			if r.status != http.StatusOK {
				t.Fatalf("status = %d, want 200; body=%q", r.status, r.body)
			}
		})
	}
}

// TestFailClosed: with no usable token, every request is 401 and Mount does not panic.
func TestFailClosed(t *testing.T) {
	configs := []struct {
		name string
		cfg  Config
	}{
		{"zero", Config{}},
		{"short", Config{Token: "short"}},
		{"short-31-with-next", Config{Token: testToken[:MinTokenLen-1], TokenNext: authTokenNext}},
	}
	for _, c := range configs {
		t.Run(c.name, func(t *testing.T) {
			srv := authServer(t, c.cfg, nil)
			reqs := []struct{ name, path, auth string }{
				{"header/correct-looking", "/mcp", "Bearer " + testToken},
				{"header/empty", "/mcp", ""},
				{"header/wrong", "/mcp", "Bearer " + authWrongToken},
				{"header/next-token", "/mcp", "Bearer " + authTokenNext},
				{"path/correct-looking", "/mcp/" + testToken, ""},
				{"path/wrong", "/mcp/" + authWrongToken, ""},
				{"path/next-token", "/mcp/" + authTokenNext, ""},
				{"path/short", "/mcp/short", ""},
				{"deep/correct-looking", "/mcp/" + testToken + "/x", "Bearer " + testToken},
				{"catchall/correct-looking", "/mcp/", "Bearer " + testToken},
				// The configured value itself, whatever it is.
				{"header/configured", "/mcp", "Bearer " + c.cfg.Token},
				{"path/configured", "/mcp/" + c.cfg.Token, ""},
			}
			for _, rq := range reqs {
				r := authSend(t, srv, http.MethodPost, rq.path, rq.auth, authInitBody)
				if r.status != http.StatusUnauthorized {
					t.Errorf("%s: status = %d, want 401", rq.name, r.status)
				}
			}
		})
	}
}

// TestDenyResponseIsBare checks the 401 shape: exact WWW-Authenticate, empty
// body, and no echo of the presented credential or path.
func TestDenyResponseIsBare(t *testing.T) {
	srv := authServer(t, Config{Token: testToken}, nil)
	disabled := authServer(t, Config{}, nil)
	short := testToken[:MinTokenLen-1]
	tests := []struct {
		name    string
		srv     *httptest.Server
		path    string
		auth    string
		secrets []string
	}{
		{"header/wrong", srv, "/mcp", "Bearer " + authWrongToken, []string{authWrongToken}},
		{"header/short", srv, "/mcp", "Bearer " + short, []string{short}},
		{"path/wrong", srv, "/mcp/" + authWrongToken, "", []string{authWrongToken}},
		{"path/missing", srv, "/mcp/", "", nil},
		{"deep/correct-bearer", srv, "/mcp/a/b", "Bearer " + testToken, []string{testToken, "/mcp/a/b"}},
		{"disabled/correct-bearer", disabled, "/mcp", "Bearer " + testToken, []string{testToken}},
		{"disabled/path", disabled, "/mcp/" + testToken, "", []string{testToken}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := authSend(t, tc.srv, http.MethodPost, tc.path, tc.auth, authInitBody)
			if r.status != http.StatusUnauthorized {
				t.Fatalf("status = %d, want 401", r.status)
			}
			wwwAuth := r.header.Values("WWW-Authenticate")
			if len(wwwAuth) != 1 || wwwAuth[0] != `Bearer realm="qc2-mcp"` {
				t.Errorf("WWW-Authenticate = %q, want exactly [%q]", wwwAuth, `Bearer realm="qc2-mcp"`)
			}
			if len(r.body) != 0 {
				t.Errorf("body = %q, want empty", r.body)
			}
			var hdrs []string
			for k, vs := range r.header {
				hdrs = append(hdrs, k+": "+strings.Join(vs, ","))
			}
			all := strings.Join(hdrs, "\n") + "\n" + string(r.body)
			if strings.Contains(all, "resource_metadata") {
				t.Errorf("response mentions resource_metadata:\n%s", all)
			}
			for _, s := range tc.secrets {
				if s != "" && strings.Contains(all, s) {
					t.Errorf("response echoes %q:\n%s", s, all)
				}
			}
		})
	}
}

// TestDiscoveryPathsAre404: no OAuth discovery document is served, with or
// without MCP enabled, even with a correct bearer header.
func TestDiscoveryPathsAre404(t *testing.T) {
	configs := []struct {
		name string
		cfg  Config
	}{
		{"enabled", Config{Token: testToken}},
		{"disabled", Config{}},
	}
	for _, c := range configs {
		t.Run(c.name, func(t *testing.T) {
			srv := authServer(t, c.cfg, nil)
			for _, p := range authDiscoveryPaths {
				for _, auth := range []string{"", "Bearer " + testToken} {
					r := authSend(t, srv, http.MethodGet, p, auth, "")
					if r.status != http.StatusNotFound {
						t.Errorf("GET %s (auth=%t): status = %d, want 404", p, auth != "", r.status)
					}
				}
			}
		})
	}
}

// TestDeepPathsAre401: anything under /mcp/ that is not a single token segment
// is answered like an unauthenticated request, even with a correct bearer.
func TestDeepPathsAre401(t *testing.T) {
	srv := authServer(t, Config{Token: testToken}, nil)
	tests := []struct {
		name, path, auth string
	}{
		{"slash/correct-bearer", "/mcp/", "Bearer " + testToken},
		{"slash/no-auth", "/mcp/", ""},
		{"deep/correct-bearer", "/mcp/a/b", "Bearer " + testToken},
		{"deep/no-auth", "/mcp/a/b", ""},
		{"deep-with-token/correct-bearer", "/mcp/" + testToken + "/extra", "Bearer " + testToken},
		{"deep-with-token/no-auth", "/mcp/" + testToken + "/extra", ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := authSend(t, srv, http.MethodPost, tc.path, tc.auth, authInitBody)
			if r.status != http.StatusUnauthorized {
				t.Fatalf("status = %d, want 401", r.status)
			}
		})
	}
}

// authSyncBuffer is a goroutine-safe log sink: handlers write from server goroutines.
type authSyncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *authSyncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *authSyncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// TestLogsNeverContainTokens sends many requests carrying the token in every
// position and asserts the captured log never holds the token (or any 12-char
// window of it), while still holding the fixed route labels for rejections.
// It mutates the global log output, so it must not run in parallel.
func TestLogsNeverContainTokens(t *testing.T) {
	var buf authSyncBuffer
	prev := log.Writer()
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(prev) })

	srv := authServer(t, Config{Token: testToken}, nil)
	short := testToken[:MinTokenLen-1]
	reqs := []struct{ path, auth string }{
		{"/mcp", "Bearer " + testToken},           // correct, header
		{"/mcp/" + testToken, ""},                 // correct, path
		{"/mcp/" + authWrongToken, ""},            // wrong, path
		{"/mcp/" + testToken + "/extra", ""},      // deeper path containing token
		{"/mcp/" + authWrongToken + "/extra", ""}, // deeper path, wrong token
		{"/mcp", "Bearer " + authWrongToken},      // wrong, header
		{"/mcp", "Bearer " + short},               // short, header
		{"/mcp/" + short, ""},                     // short, path
		{"/mcp/", "Bearer " + testToken},          // catch-all with correct header
		{"/mcp/a/b", "Bearer " + authWrongToken},  // deep, wrong header
	}
	for i := 0; i < 20; i++ {
		for _, rq := range reqs {
			authSend(t, srv, http.MethodPost, rq.path, rq.auth, authInitBody)
		}
	}

	logs := buf.String()
	if logs == "" {
		t.Fatal("captured no log output; the rejection logging this test relies on is missing")
	}
	for _, label := range []string{
		"route=/mcp reason=",
		"route=/mcp/{token} reason=",
		"route=/mcp/* reason=",
	} {
		if !strings.Contains(logs, label) {
			t.Errorf("log missing fixed label %q; logs:\n%s", label, logs)
		}
	}
	for _, secret := range []string{testToken, authWrongToken, short} {
		if strings.Contains(logs, secret) {
			t.Errorf("log contains a full secret; logs:\n%s", logs)
		}
	}
	for _, secret := range []string{testToken, authWrongToken} {
		for i := 0; i+12 <= len(secret); i++ {
			if w := secret[i : i+12]; strings.Contains(logs, w) {
				t.Errorf("log contains 12-char window %q of a secret; logs:\n%s", w, logs)
				break
			}
		}
	}
}

// TestRateLimitWrapsEveryRoute: the limit middleware runs for the header, path,
// and catch-all routes, and before auth (so rejected requests are counted too).
func TestRateLimitWrapsEveryRoute(t *testing.T) {
	configs := []struct {
		name string
		cfg  Config
	}{
		{"enabled", Config{Token: testToken}},
		{"disabled", Config{}},
	}
	for _, c := range configs {
		t.Run(c.name, func(t *testing.T) {
			var calls atomic.Int64
			count := func(next http.Handler) http.Handler {
				return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					calls.Add(1)
					next.ServeHTTP(w, r)
				})
			}
			srv := authServer(t, c.cfg, count)
			steps := []struct{ path, auth string }{
				{"/mcp", "Bearer " + authWrongToken},
				{"/mcp/" + testToken, ""},
				{"/mcp/a/b", "Bearer " + testToken},
				{"/mcp", "Bearer " + testToken},
			}
			for i, s := range steps {
				authSend(t, srv, http.MethodPost, s.path, s.auth, authInitBody)
				if got := calls.Load(); got != int64(i+1) {
					t.Fatalf("after %s: limit calls = %d, want %d", s.path, got, i+1)
				}
			}
		})
	}
}

// TestVerifier unit-tests the constant-time comparison directly.
func TestVerifier(t *testing.T) {
	tests := []struct {
		name      string
		cfg       Config
		presented string
		want      bool
	}{
		{"configured/empty", Config{Token: testToken}, "", false},
		{"configured/correct", Config{Token: testToken}, testToken, true},
		{"configured/shorter-prefix", Config{Token: testToken}, testToken[:len(testToken)-1], false},
		{"configured/longer", Config{Token: testToken}, testToken + "x", false},
		{"configured/wrong-same-length", Config{Token: testToken}, authWrongToken[:len(testToken)], false},
		{"configured/unconfigured-next", Config{Token: testToken}, authTokenNext, false},
		{"with-next/next", Config{Token: testToken, TokenNext: authTokenNext}, authTokenNext, true},
		{"with-next/current", Config{Token: testToken, TokenNext: authTokenNext}, testToken, true},
		{"with-next/empty", Config{Token: testToken, TokenNext: authTokenNext}, "", false},
		{"short-next/ignored", Config{Token: testToken, TokenNext: authTokenNext[:MinTokenLen-1]}, authTokenNext[:MinTokenLen-1], false},
		{"unconfigured/empty", Config{}, "", false},
		{"unconfigured/testToken", Config{}, testToken, false},
		{"short-current/empty", Config{Token: "short"}, "", false},
		{"short-current/short", Config{Token: "short"}, "short", false},
		{"short-current-with-next/next", Config{Token: "short", TokenNext: authTokenNext}, authTokenNext, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := newVerifier(tc.cfg).ok(tc.presented); got != tc.want {
				t.Errorf("ok(%q) = %t, want %t", tc.presented, got, tc.want)
			}
		})
	}
	if (&verifier{}).ok("") {
		t.Error("zero-value verifier accepted the empty string")
	}
}
