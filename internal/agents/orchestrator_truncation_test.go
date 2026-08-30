package agents

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// waitForRecord blocks until got receives a value or a timeout elapses -- mirrors
// waitForRequest in usage_reporter_test.go, avoiding an arbitrary sleep for
// UsageReporter's async background worker to POST a queued record.
func waitForRecord(t *testing.T, got <-chan reportedUsageRecord) reportedUsageRecord {
	t.Helper()
	select {
	case r := <-got:
		return r
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for a usage record to be reported")
		return reportedUsageRecord{}
	}
}

// TestOrchestrator_MaxTokensTruncation_RetriesFallbackChain covers the fix for a real
// incident: a primary-model call that completes (err == nil from the SDK) but was cut off
// by the MaxOutputTokens cap used to be treated as a success, handing the caller truncated,
// unparseable JSON and discarding every other agent's already-completed work for the whole
// pipeline run. The fix treats a MAX_TOKENS finish reason as a retryable failure inside the
// fallback loop instead of a terminal one.
func TestOrchestrator_MaxTokensTruncation_RetriesFallbackChain(t *testing.T) {
	truncatedResponse := `{
		"candidates": [
			{
				"content": {"parts": [{"text": "{\"incomplete\": \"json truncated mid-str"}], "role": "model"},
				"finishReason": "MAX_TOKENS"
			}
		],
		"usageMetadata": {"promptTokenCount": 1943, "candidatesTokenCount": 6785, "thoughtsTokenCount": 1200}
	}`
	goodResponse := `{
		"candidates": [
			{
				"content": {"parts": [{"text": "Recovered Fallback Response"}], "role": "model"},
				"finishReason": "STOP"
			}
		],
		"usageMetadata": {"promptTokenCount": 1943, "candidatesTokenCount": 233, "thoughtsTokenCount": 1223}
	}`

	var requestedModels []string
	geminiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.Contains(r.URL.Path, "gemini-3.1-pro-preview"):
			requestedModels = append(requestedModels, "gemini-3.1-pro-preview")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(truncatedResponse))
		case strings.Contains(r.URL.Path, "gemini-3.5-flash"):
			requestedModels = append(requestedModels, "gemini-3.5-flash")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(goodResponse))
		default:
			t.Errorf("unexpected model requested in path: %s", r.URL.Path)
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	defer geminiServer.Close()

	// UsageReporter.send POSTs one record per call from a pool of worker goroutines
	// (see usage_reporter.go), so two attempts can arrive out of send order -- collect
	// both off a channel and match by content rather than assuming POST #1 == attempt #1.
	reportedRecords := make(chan reportedUsageRecord, 4)
	reportServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload usageIngestPayload
		if err := json.NewDecoder(r.Body).Decode(&payload); err == nil {
			for _, rec := range payload.Records {
				reportedRecords <- rec
			}
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer reportServer.Close()

	ctx := context.Background()
	orch, err := NewOrchestrator(ctx, "fake-gemini-key", nil,
		WithEndpoint(geminiServer.URL),
		WithHTTPClient(geminiServer.Client()))
	if err != nil {
		t.Fatalf("Failed to instantiate Orchestrator: %v", err)
	}
	defer orch.Close()
	orch.Reporter = NewUsageReporter(reportServer.URL, "test-shared-secret", "weitzer-org/sound-profile-builder")

	res, err := orch.RunAgentSplit(ctx, "Tone Historian", "System prompt", "User prompt")
	if err != nil {
		t.Fatalf("RunAgentSplit failed: %v", err)
	}
	if res != "Recovered Fallback Response" {
		t.Errorf("expected the fallback model's response to be returned, got: %q", res)
	}

	if len(requestedModels) != 2 || requestedModels[0] != "gemini-3.1-pro-preview" || requestedModels[1] != "gemini-3.5-flash" {
		t.Errorf("expected primary model to be tried and truncate, then the fallback to be tried and succeed; got sequence: %v", requestedModels)
	}

	rec1 := waitForRecord(t, reportedRecords)
	rec2 := waitForRecord(t, reportedRecords)

	byModel := map[string]reportedUsageRecord{rec1.Model: rec1, rec2.Model: rec2}
	truncatedRec, ok1 := byModel["gemini-3.1-pro-preview"]
	recoveredRec, ok2 := byModel["gemini-3.5-flash"]
	if !ok1 || !ok2 {
		t.Fatalf("expected one record per model in {gemini-3.1-pro-preview, gemini-3.5-flash}, got: %+v, %+v", rec1, rec2)
	}

	if truncatedRec.Success {
		t.Errorf("expected the truncated primary attempt to be recorded as a failure, got Success=true: %+v", truncatedRec)
	}
	if truncatedRec.ErrorKind != "max_tokens_truncated" {
		t.Errorf("ErrorKind = %q, want %q", truncatedRec.ErrorKind, "max_tokens_truncated")
	}
	if truncatedRec.OutputTokens != 6785 {
		t.Errorf("expected the truncated attempt's real burned token spend to still be recorded, OutputTokens = %d, want 6785", truncatedRec.OutputTokens)
	}

	if !recoveredRec.Success {
		t.Errorf("expected the fallback attempt to be recorded as a success: %+v", recoveredRec)
	}
}

// TestOrchestrator_MaxTokensTruncation_AllCandidatesTruncateReturnsError covers the case
// where every model in the fallback chain truncates: RunAgentSplit must surface a clear
// error rather than silently succeeding with unusable JSON.
func TestOrchestrator_MaxTokensTruncation_AllCandidatesTruncateReturnsError(t *testing.T) {
	truncatedResponse := `{
		"candidates": [
			{
				"content": {"parts": [{"text": "{\"incomplete"}], "role": "model"},
				"finishReason": "MAX_TOKENS"
			}
		],
		"usageMetadata": {"promptTokenCount": 100, "candidatesTokenCount": 4000}
	}`
	geminiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(truncatedResponse))
	}))
	defer geminiServer.Close()

	ctx := context.Background()
	orch, err := NewOrchestrator(ctx, "fake-gemini-key", nil,
		WithEndpoint(geminiServer.URL),
		WithHTTPClient(geminiServer.Client()))
	if err != nil {
		t.Fatalf("Failed to instantiate Orchestrator: %v", err)
	}
	defer orch.Close()

	_, err = orch.RunAgentSplit(ctx, "Tone Historian", "System prompt", "User prompt")
	if err == nil {
		t.Fatal("expected an error when every fallback candidate truncates, got nil")
	}
	if !strings.Contains(err.Error(), "MaxOutputTokens") {
		t.Errorf("expected the error to mention MaxOutputTokens truncation, got: %v", err)
	}
}
