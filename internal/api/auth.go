package api

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"log"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/weitzer-org/sound-builder/internal/config"
)

const sessionCookieName = "qc_auth_session"
const csrfCookieName = "qc_csrf_token"
const csrfHeaderName = "X-CSRF-Token"
const secretName = "spb-login-pw"
const sessionDuration = 24 * time.Hour

// simple hashing to secure the cookie based on the SecretManager password
func generateCookieValue(password string) string {
	expireTime := time.Now().Add(sessionDuration).Unix()
	mac := hmac.New(sha256.New, []byte(password))
	mac.Write([]byte(fmt.Sprintf("%d", expireTime)))
	signature := base64.URLEncoding.EncodeToString(mac.Sum(nil))

	return fmt.Sprintf("%d.%s", expireTime, signature)
}

func validateCookieValue(cookieValue, password string) bool {
	parts := strings.Split(cookieValue, ".")
	if len(parts) != 2 {
		return false
	}

	expireTime, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return false
	}

	if time.Now().Unix() > expireTime {
		return false
	}

	mac := hmac.New(sha256.New, []byte(password))
	mac.Write([]byte(parts[0]))
	expectedSignature := base64.URLEncoding.EncodeToString(mac.Sum(nil))

	return hmac.Equal([]byte(parts[1]), []byte(expectedSignature))
}

// generateCSRFToken produces a random token for the double-submit CSRF
// cookie. It carries no secret of its own (a CSRF attacker can trigger a
// cross-site request but, unlike the session cookie's contents, can't read
// the browser's cookie jar to put this value in a header) -- the security
// property comes entirely from requiring the header and cookie to match.
func generateCSRFToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// csrfTokenValid checks the double-submit CSRF token: the X-CSRF-Token
// header (set on every htmx request client-side, see index.html) must match
// the qc_csrf_token cookie. Only called for state-changing methods.
func csrfTokenValid(r *http.Request) bool {
	cookie, err := r.Cookie(csrfCookieName)
	if err != nil || cookie.Value == "" {
		return false
	}
	headerToken := r.Header.Get(csrfHeaderName)
	if headerToken == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(cookie.Value), []byte(headerToken)) == 1
}

// isStateChangingMethod reports whether r's method can mutate state and
// therefore needs a CSRF check -- GET/HEAD/OPTIONS are assumed side-effect
// free per HTTP semantics.
func isStateChangingMethod(method string) bool {
	return method != http.MethodGet && method != http.MethodHead && method != http.MethodOptions
}

// isRequestSecure reports whether r arrived over HTTPS, directly or via a
// TLS-terminating proxy. Local dev (MOCK_MODE over plain http://localhost)
// has neither r.TLS nor this header, so cookies stay usable there; Fly's
// proxy (fly.toml sets force_https) sets X-Forwarded-Proto on everything it
// forwards, so prod cookies still get Secure.
func isRequestSecure(r *http.Request) bool {
	return r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https"
}

// TODO: Integrate with Google Authentication (OAuth2/OIDC) at a later time.

// AuthMiddleware intercepts requests to check if they have a valid session cookie.
func (s *Server) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(sessionCookieName)
		if err != nil {
			if r.Header.Get("HX-Request") == "true" {
				w.Header().Set("HX-Redirect", "/login")
				w.WriteHeader(http.StatusUnauthorized)
			} else {
				http.Redirect(w, r, "/login", http.StatusFound)
			}
			return
		}

		ctx := r.Context()
		cfg, err := config.LoadConfig("config.json")
		projectID := ""
		if err == nil && cfg.ProjectID != "" {
			projectID = cfg.ProjectID
		}

		// Fetch the expected password from Secret Manager
		expectedPassword, err := s.smFetcher.GetPassword(ctx, projectID, secretName)
		if err != nil {
			log.Printf("Failed to fetch UI password from secret manager: %v", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		expectedPassword = strings.TrimSpace(expectedPassword)

		if !validateCookieValue(cookie.Value, expectedPassword) {
			if r.Header.Get("HX-Request") == "true" {
				w.Header().Set("HX-Redirect", "/login")
				w.WriteHeader(http.StatusUnauthorized)
			} else {
				http.Redirect(w, r, "/login", http.StatusFound)
			}
			return
		}

		if isStateChangingMethod(r.Method) && !csrfTokenValid(r) {
			http.Error(w, "Invalid or missing CSRF token", http.StatusForbidden)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// handleServeLogin serves the login HTML page.
func (s *Server) handleServeLogin() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, filepath.Join("web", "templates", "login.html"))
	}
}

// handleProcessLogin authenticates the user and sets the session cookie.
func (s *Server) handleProcessLogin() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			return
		}

		err := r.ParseForm()
		if err != nil {
			http.Error(w, "Bad Request", http.StatusBadRequest)
			return
		}

		submittedPassword := r.FormValue("password")

		ctx := r.Context()
		cfg, err := config.LoadConfig("config.json")
		projectID := ""
		if err == nil && cfg.ProjectID != "" {
			projectID = cfg.ProjectID
		}

		expectedPassword, err := s.smFetcher.GetPassword(ctx, projectID, secretName)
		if err != nil {
			log.Printf("Failed to fetch UI password from secret manager: %v", err)
			w.Header().Set("Content-Type", "text/html")
			w.Write([]byte(`<div style="color: #ef4444; font-size: 0.95rem; margin-top: 1rem;">Internal Error Occurred. Try again later.</div>`))
			return
		}
		expectedPassword = strings.TrimSpace(expectedPassword)

		if submittedPassword == expectedPassword {
			secure := isRequestSecure(r)

			// Issue secure cookie
			cookieValue := generateCookieValue(expectedPassword)
			http.SetCookie(w, &http.Cookie{
				Name:     sessionCookieName,
				Value:    cookieValue,
				Path:     "/",
				HttpOnly: true,
				Secure:   secure,
				MaxAge:   int(sessionDuration.Seconds()),
				SameSite: http.SameSiteLaxMode,
			})

			// Issue the CSRF double-submit cookie alongside the session
			// cookie. Deliberately NOT HttpOnly -- index.html's
			// htmx:configRequest listener reads it to set the X-CSRF-Token
			// header on every state-changing request.
			csrfToken, err := generateCSRFToken()
			if err != nil {
				log.Printf("Failed to generate CSRF token: %v", err)
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
				return
			}
			http.SetCookie(w, &http.Cookie{
				Name:     csrfCookieName,
				Value:    csrfToken,
				Path:     "/",
				HttpOnly: false,
				Secure:   secure,
				MaxAge:   int(sessionDuration.Seconds()),
				SameSite: http.SameSiteLaxMode,
			})

			// HTMX redirect to index
			w.Header().Set("HX-Redirect", "/")
			w.WriteHeader(http.StatusOK)
		} else {
			// Return error message for inline HTMX display
			w.Header().Set("Content-Type", "text/html")
			w.Write([]byte(`<div style="color: #ef4444; font-size: 0.95rem; margin-top: 1rem;">Invalid password.</div>`))
		}
	}
}
