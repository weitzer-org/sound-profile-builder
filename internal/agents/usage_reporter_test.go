package agents

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// waitForRequest blocks until got receives a value or the timeout elapses,
// failing the test in the latter case — avoids an arbitrary time.Sleep to
// let UsageReporter's background worker catch up.
func waitForRequest(t *testing.T, got <-chan *http.Request) *http.Request {
	t.Helper()
	select {
	case r := <-got:
		return r
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for UsageReporter to POST to the test server")
		return nil
	}
}

func testUsageRecord() UsageRecord {
	return UsageRecord{
		Timestamp:      "2026-07-29T14:32:05Z",
		Provider:       "gemini",
		CallType:       "Tone Historian",
		Model:          "gemini-3.1-pro-preview",
		InputTokens:    1345,
		OutputTokens:   210,
		ThinkingTokens: 512,
		LatencyMS:      2431,
		CostUSD:        0.0034,
		Success:        true,
	}
}

func TestUsageReporter_PostsTranslatedPayloadWithSharedSecretHeader(t *testing.T) {
	requests := make(chan *http.Request, 1)
	bodies := make(chan usageIngestPayload, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload usageIngestPayload
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Errorf("decoding request body: %v", err)
			return
		}
		requests <- r
		bodies <- payload
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	r := NewUsageReporter(srv.URL, "test-shared-secret", "weitzer-org/sound-profile-builder")
	r.Report(testUsageRecord())

	req := waitForRequest(t, requests)
	if got := req.Header.Get("X-Usage-Ingest-Key"); got != "test-shared-secret" {
		t.Errorf("X-Usage-Ingest-Key = %q, want %q", got, "test-shared-secret")
	}
	if got := req.Header.Get("Authorization"); got != "" {
		t.Errorf("expected no Authorization header (GSR's ingest endpoint checks X-Usage-Ingest-Key, not Authorization), got %q", got)
	}

	payload := <-bodies
	if payload.Repository != "weitzer-org/sound-profile-builder" {
		t.Errorf("Repository = %q, want %q", payload.Repository, "weitzer-org/sound-profile-builder")
	}
	if len(payload.Records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(payload.Records))
	}
	rec := payload.Records[0]
	if rec.CallType != "Tone Historian" {
		t.Errorf("CallType not translated: %+v", rec)
	}
	if rec.Provider != "gemini" || rec.Model != "gemini-3.1-pro-preview" {
		t.Errorf("Provider/Model not translated: %+v", rec)
	}
	if rec.ThinkingTokens != 512 {
		t.Errorf("ThinkingTokens = %d, want 512", rec.ThinkingTokens)
	}
	if rec.LatencyMS != 2431 || rec.CostUSD != 0.0034 {
		t.Errorf("latency/cost not translated: %+v", rec)
	}
}

func TestUsageReporter_FiltersOutNonGeminiProviderBeforePosting(t *testing.T) {
	called := make(chan struct{}, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called <- struct{}{}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	r := NewUsageReporter(srv.URL, "test-shared-secret", "weitzer-org/sound-profile-builder")

	// This project's own RunAgentSplit reports "openllm" for its Open-LLM
	// branch — that provider must never reach GSR's ingest endpoint, which
	// only accepts provider=="gemini".
	openllmRec := testUsageRecord()
	openllmRec.Provider = "openllm"
	r.Report(openllmRec)

	// A gemini record proves the worker pool is actually alive and draining
	// — if it arrives and the openllm one never did, the filter (not a dead
	// worker) is what suppressed it.
	r.Report(testUsageRecord())

	select {
	case <-called:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for the gemini record's request")
	}
	select {
	case <-called:
		t.Fatal("an openllm-provider record reached the ingest endpoint")
	case <-time.After(200 * time.Millisecond):
	}
}

func TestUsageReporter_ReportDoesNotBlockOnAFailingOrSlowServer(t *testing.T) {
	block := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-block // never unblocks during this test
	}))
	defer func() {
		close(block)
		srv.Close()
	}()

	r := NewUsageReporter(srv.URL, "test-shared-secret", "weitzer-org/sound-profile-builder")

	done := make(chan struct{})
	go func() {
		r.Report(testUsageRecord())
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(1 * time.Second):
		t.Fatal("Report blocked on a slow server instead of returning immediately")
	}
}

func TestUsageReporter_NilReporterIsSafeToCall(t *testing.T) {
	var r *UsageReporter
	r.Report(testUsageRecord()) // must not panic
}
