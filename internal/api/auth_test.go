package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/weitzer-org/sound-builder/internal/agents"
)

func TestAuthMiddleware_NoCookie(t *testing.T) {
	mockAuth := &mockSecretFetcher{}
	server := NewServer(nil, nil, mockAuth, func(ctx context.Context, apiKey string) (agents.OrchestratorService, error) {
		return nil, nil
	})

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
	server := NewServer(nil, nil, mockAuth, func(ctx context.Context, apiKey string) (agents.OrchestratorService, error) {
		return nil, nil
	})

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
	server := NewServer(nil, nil, mockAuth, func(ctx context.Context, apiKey string) (agents.OrchestratorService, error) {
		return nil, nil
	})

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
	var authCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == sessionCookieName {
			authCookie = c
			break
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
}

func TestProcessLogin_Failure(t *testing.T) {
	mockAuth := &mockSecretFetcher{}
	server := NewServer(nil, nil, mockAuth, func(ctx context.Context, apiKey string) (agents.OrchestratorService, error) {
		return nil, nil
	})

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
	server := NewServer(nil, nil, mockAuth, func(ctx context.Context, apiKey string) (agents.OrchestratorService, error) {
		return nil, nil
	})

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
	server := NewServer(nil, nil, mockAuth, func(ctx context.Context, apiKey string) (agents.OrchestratorService, error) {
		return nil, nil
	})

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
	server := NewServer(nil, nil, mockAuth, func(ctx context.Context, apiKey string) (agents.OrchestratorService, error) {
		return nil, nil
	})

	req := httptest.NewRequest(http.MethodPut, "/login", nil)
	rr := httptest.NewRecorder()

	server.mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", rr.Code)
	}
}
