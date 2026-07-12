package agents

import (
	"net/http"

	"google.golang.org/genai"
)

// ClientOption mutates the genai.ClientConfig used to construct the Gemini client. It
// replaces the old SDK's option.ClientOption for the handful of things tests need to
// override (a mock HTTP endpoint/transport, or a deliberately invalid combination of
// fields to force a construction error).
type ClientOption func(*genai.ClientConfig)

// WithEndpoint points the client at a different base URL (e.g. an httptest.Server) instead
// of the real Gemini API.
func WithEndpoint(url string) ClientOption {
	return func(cc *genai.ClientConfig) {
		cc.HTTPOptions.BaseURL = url
	}
}

// WithHTTPClient overrides the HTTP client used for all requests (e.g. an
// httptest.Server's client, or one with a short timeout for failure-path tests).
func WithHTTPClient(c *http.Client) ClientOption {
	return func(cc *genai.ClientConfig) {
		cc.HTTPClient = c
	}
}

// WithInvalidConfig forces genai.NewClient to return a construction-time error, for tests
// that need to verify NewOrchestrator surfaces that error. Project and APIKey are mutually
// exclusive per the SDK's own validation, which is a real error path rather than a
// contrived one.
func WithInvalidConfig() ClientOption {
	return func(cc *genai.ClientConfig) {
		cc.Project = "conflicting-project"
	}
}
