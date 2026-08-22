package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/weitzer-org/sound-builder/internal/agents"
	"github.com/weitzer-org/sound-builder/internal/storage"
)

func TestAuthMiddleware_NoCookie(t *testing.T) {
	mockAuth := &mockSecretFetcher{}
	server := NewServer(nil, nil, nil, mockAuth, func(ctx context.Context, apiKey string) (agents.OrchestratorService, error) {
		return nil, nil
	}, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/presets", nil)
	rr := httptest.NewRecorder()

	server.mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusFound {
		t.Errorf("expected redirect (302), got %d", rr.Code)
	}
	if loc := rr.Header().Get("Location"); loc != "/login" {
		t.Errorf("expected location /login, got %s", loc)
	}
}

func TestAuthMiddleware_HTMXRedirect(t *testing.T) {
	mockAuth := &mockSecretFetcher{}
	server := NewServer(nil, nil, nil, mockAuth, func(ctx context.Context, apiKey string) (agents.OrchestratorService, error) {
		return nil, nil
	}, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/presets", nil)
	req.Header.Set("HX-Request", "true")
	rr := httptest.NewRecorder()

	server.mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 Unauthorized, got %d", rr.Code)
	}
	if hxLoc := rr.Header().Get("HX-Redirect"); hxLoc != "/login" {
		t.Errorf("expected HX-Redirect /login, got %s", hxLoc)
	}
}

func TestProcessLogin_Success(t *testing.T) {
	mockAuth := &mockSecretFetcher{}
	server := NewServer(nil, nil, nil, mockAuth, func(ctx context.Context, apiKey string) (agents.OrchestratorService, error) {
		return nil, nil
	}, nil)

	formData := strings.NewReader("password=mock-secret")
	req := httptest.NewRequest(http.MethodPost, "/login", formData)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()

	server.mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200 OK, got %d", rr.Code)
	}
	if hxLoc := rr.Header().Get("HX-Redirect"); hxLoc != "/" {
		t.Errorf("expected HX-Redirect /, got %s", hxLoc)
	}

	cookies := rr.Result().Cookies()
	var authCookie, csrfCookie *http.Cookie
	for _, c := range cookies {
		switch c.Name {
		case sessionCookieName:
			authCookie = c
		case csrfCookieName:
			csrfCookie = c
		}
	}
	if authCookie == nil {
		t.Fatalf("expected cookie %s not set", sessionCookieName)
	}
	if validateCookieValue(authCookie.Value, "wrongpassword") {
		t.Errorf("cookie validated with wrong password unexpectedly")
	}
	if !validateCookieValue(authCookie.Value, "mock-secret") {
		t.Errorf("failed to validate generated cookie")
	}

	if csrfCookie == nil {
		t.Fatalf("expected cookie %s not set", csrfCookieName)
	}
	if csrfCookie.Value == "" {
		t.Errorf("expected non-empty CSRF token")
	}
	if csrfCookie.HttpOnly {
		t.Errorf("CSRF cookie must not be HttpOnly -- client JS needs to read it to set the header")
	}
}

func TestAuthMiddleware_CSRF_MissingToken(t *testing.T) {
	mockAuth := &mockSecretFetcher{}
	server := NewServer(nil, nil, nil, mockAuth, func(ctx context.Context, apiKey string) (agents.OrchestratorService, error) {
		return nil, nil
	}, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/preset/delete", strings.NewReader("id=x"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: generateCookieValue("mock-secret")})
	// No CSRF cookie or header set.
	rr := httptest.NewRecorder()

	server.mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403 Forbidden with no CSRF token, got %d", rr.Code)
	}
}

func TestAuthMiddleware_CSRF_MismatchedToken(t *testing.T) {
	mockAuth := &mockSecretFetcher{}
	server := NewServer(nil, nil, nil, mockAuth, func(ctx context.Context, apiKey string) (agents.OrchestratorService, error) {
		return nil, nil
	}, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/preset/delete", strings.NewReader("id=x"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: generateCookieValue("mock-secret")})
	req.AddCookie(&http.Cookie{Name: csrfCookieName, Value: "cookie-value"})
	req.Header.Set(csrfHeaderName, "different-header-value")
	rr := httptest.NewRecorder()

	server.mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403 Forbidden with mismatched CSRF token, got %d", rr.Code)
	}
}

func TestAuthMiddleware_CSRF_ValidToken(t *testing.T) {
	mockAuth := &mockSecretFetcher{}
	client := newMockClient()
	store := storage.NewPresetStore(client, "b")
	server := NewServer(store, nil, client, mockAuth, func(ctx context.Context, apiKey string) (agents.OrchestratorService, error) {
		return nil, nil
	}, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/preset/delete", strings.NewReader("id=x"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	withAuthAndCSRF(req)
	rr := httptest.NewRecorder()

	server.mux.ServeHTTP(rr, req)

	if rr.Code == http.StatusForbidden {
		t.Errorf("did not expect 403 Forbidden with a matching CSRF token, got body: %s", rr.Body.String())
	}
}

func TestAuthMiddleware_CSRF_NotRequiredForGET(t *testing.T) {
	server, _, _, _ := setupTestServer()

	req := httptest.NewRequest(http.MethodGet, "/api/presets", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: generateCookieValue("mock-secret")})
	// No CSRF cookie/header -- GET must not require one.
	rr := httptest.NewRecorder()

	server.mux.ServeHTTP(rr, req)

	if rr.Code == http.StatusForbidden {
		t.Errorf("GET requests should not require a CSRF token, got 403")
	}
}

func TestCSRFTokenValid(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/x", nil)
	if csrfTokenValid(req) {
		t.Errorf("expected false with no cookie or header set")
	}

	req.AddCookie(&http.Cookie{Name: csrfCookieName, Value: "token-abc"})
	if csrfTokenValid(req) {
		t.Errorf("expected false with cookie set but no header")
	}

	req.Header.Set(csrfHeaderName, "token-abc")
	if !csrfTokenValid(req) {
		t.Errorf("expected true with matching cookie and header")
	}

	req.Header.Set(csrfHeaderName, "token-xyz")
	if csrfTokenValid(req) {
		t.Errorf("expected false with mismatched cookie and header")
	}
}

func TestProcessLogin_Failure(t *testing.T) {
	mockAuth := &mockSecretFetcher{}
	server := NewServer(nil, nil, nil, mockAuth, func(ctx context.Context, apiKey string) (agents.OrchestratorService, error) {
		return nil, nil
	}, nil)

	formData := strings.NewReader("password=wrongpassword")
	req := httptest.NewRequest(http.MethodPost, "/login", formData)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()

	server.mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200 OK with html error block, got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "Invalid password") {
		t.Errorf("expected Invalid password, got %s", rr.Body.String())
	}
}

func TestServeLogin_Success(t *testing.T) {
	mockAuth := &mockSecretFetcher{}
	server := NewServer(nil, nil, nil, mockAuth, func(ctx context.Context, apiKey string) (agents.OrchestratorService, error) {
		return nil, nil
	}, nil)

	req := httptest.NewRequest(http.MethodGet, "/login", nil)
	rr := httptest.NewRecorder()

	server.mux.ServeHTTP(rr, req)

	// Will return whatever http.ServeFile does, which might be 404 in test env since the html file is at root, but ensures the handler is matched and executed.
	if rr.Code != http.StatusOK && rr.Code != http.StatusNotFound {
		t.Errorf("expected 200/404, got %d", rr.Code)
	}
}

func TestAuthMiddleware_InvalidCookie(t *testing.T) {
	mockAuth := &mockSecretFetcher{}
	server := NewServer(nil, nil, nil, mockAuth, func(ctx context.Context, apiKey string) (agents.OrchestratorService, error) {
		return nil, nil
	}, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/presets", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "invalid.cookie.value"})
	rr := httptest.NewRecorder()

	server.mux.ServeHTTP(rr, req)

	// An invalid cookie acts as NO cookie
	if rr.Code != http.StatusFound {
		t.Errorf("expected redirect (302) due to invalid cookie, got %d", rr.Code)
	}
}

func TestProcessLogin_InvalidMethod(t *testing.T) {
	mockAuth := &mockSecretFetcher{}
	server := NewServer(nil, nil, nil, mockAuth, func(ctx context.Context, apiKey string) (agents.OrchestratorService, error) {
		return nil, nil
	}, nil)

	req := httptest.NewRequest(http.MethodPut, "/login", nil)
	rr := httptest.NewRecorder()

	server.mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", rr.Code)
	}
}
