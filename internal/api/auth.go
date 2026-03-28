package api

import (
	"crypto/hmac"
	"crypto/sha256"
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
const secretName = "spb-login-pw"
const sessionDuration = 30 * time.Minute

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
		projectID := "710019748844"
		cfg, err := config.LoadConfig("config.json")
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
		projectID := "710019748844"
		cfg, err := config.LoadConfig("config.json")
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
			// Issue secure cookie
			cookieValue := generateCookieValue(expectedPassword)
			http.SetCookie(w, &http.Cookie{
				Name:     sessionCookieName,
				Value:    cookieValue,
				Path:     "/",
				HttpOnly: true,
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
