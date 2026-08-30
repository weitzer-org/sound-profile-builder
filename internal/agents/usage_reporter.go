package agents

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"
)

// reportedUsageRecord is the wire shape POSTed to GSR's ingest endpoint
// (POST /api/usage/ingest, adk/backend/src/app.ts). GSR's ingest contract is
// the source of truth for field names here, not this package's own
// UsageRecord — field names already match one-for-one except that this
// project has no cached-token concept at all, so cachedTokens is simply
// never populated (GSR already treats it as optional).
type reportedUsageRecord struct {
	Timestamp      string  `json:"timestamp"`
	Provider       string  `json:"provider"`
	CallType       string  `json:"callType"`
	RefID          string  `json:"refId,omitempty"`
	Model          string  `json:"model"`
	InputTokens    int32   `json:"inputTokens"`
	OutputTokens   int32   `json:"outputTokens"`
	ThinkingTokens int32   `json:"thinkingTokens,omitempty"`
	LatencyMS      int64   `json:"latencyMs"`
	CostUSD        float64 `json:"costUsd"`
	Success        bool    `json:"success"`
	ErrorKind      string  `json:"errorKind,omitempty"`
}

// usageIngestPayload mirrors the body shape adk/backend/src/usageReporter.ts's
// reportUsage() sends — this project isn't the only client of this
// endpoint, so match its exact shape rather than inventing a new one.
type usageIngestPayload struct {
	Repository string                `json:"repository"`
	Records    []reportedUsageRecord `json:"records"`
}

// reporterQueueSize bounds how many not-yet-sent reports can wait at once.
// A full queue means GSR's ingest endpoint (or the network to it) is
// currently unhealthy; Report drops the newest report and logs rather than
// blocking its caller — this is a non-blocking analytics side-channel, not
// a delivery-guaranteed queue.
const reporterQueueSize = 256

// reporterWorkers is the fixed number of goroutines draining the queue,
// started once at construction — bounded so a real traffic burst can't
// spawn unbounded concurrent outbound connections to GSR the way one
// goroutine per recordUsage call would.
const reporterWorkers = 4

// reportTimeout bounds a single POST to GSR.
const reportTimeout = 10 * time.Second

// UsageReporter pushes this project's own Gemini usage to GSR's hosted
// usage dashboard, so it shows up there under GSR's "product" workload
// alongside GSR's own review/eval usage. Reporting is best-effort,
// non-blocking, and reports still queued are lost outright on a
// deploy/restart — an accepted tradeoff for a side-channel that must never
// slow down or fail a real agent-pipeline request. Construct with
// NewUsageReporter; the zero value is not usable (its queue is nil).
type UsageReporter struct {
	baseURL    string
	key        string
	repository string
	httpClient *http.Client
	queue      chan UsageRecord
}

// NewUsageReporter builds a UsageReporter and starts its fixed pool of
// worker goroutines. Call once per process (mirroring how Orchestrator's own
// gcs client is built once in cmd/server/main.go), not once per event.
func NewUsageReporter(baseURL, key, repository string) *UsageReporter {
	r := &UsageReporter{
		baseURL:    baseURL,
		key:        key,
		repository: repository,
		httpClient: &http.Client{Timeout: reportTimeout},
		queue:      make(chan UsageRecord, reporterQueueSize),
	}
	for i := 0; i < reporterWorkers; i++ {
		go r.worker()
	}
	return r
}

func (r *UsageReporter) worker() {
	for rec := range r.queue {
		// context.Background(), not the request context that was live when
		// Report() enqueued rec: recordUsage/recordAttemptUsage are called
		// from inside RunAgentSplit, itself invoked synchronously from an
		// HTTP handler — that inbound context is canceled the instant the
		// handler returns, almost always before a worker goroutine gets
		// around to sending this. A worker has no natural parent request, so
		// it gets its own bounded timeout instead of inheriting a context
		// that would abort nearly every report.
		ctx, cancel := context.WithTimeout(context.Background(), reportTimeout)
		r.send(ctx, rec)
		cancel()
	}
}

// Report enqueues rec for background reporting to GSR and returns
// immediately — it never blocks on the network, mirroring recordUsage's own
// best-effort, never-blocking contract for its caller. A non-"gemini"
// provider is dropped before ever reaching the queue: this project's own
// UsageRecord also carries "openllm" (see RunAgentSplit's Open-LLM branch),
// and GSR's ingest endpoint requires provider=="gemini", silently rejecting
// anything else.
func (r *UsageReporter) Report(rec UsageRecord) {
	if r == nil || rec.Provider != "gemini" {
		return
	}
	select {
	case r.queue <- rec:
	default:
		log.Printf("usage: report queue full, dropping a GSR usage report (callType=%s)", rec.CallType)
	}
}

func (r *UsageReporter) send(ctx context.Context, rec UsageRecord) {
	payload := usageIngestPayload{
		Repository: r.repository,
		Records: []reportedUsageRecord{{
			Timestamp:      rec.Timestamp,
			Provider:       rec.Provider,
			CallType:       rec.CallType,
			RefID:          rec.RefID,
			Model:          rec.Model,
			InputTokens:    rec.InputTokens,
			OutputTokens:   rec.OutputTokens,
			ThinkingTokens: rec.ThinkingTokens,
			LatencyMS:      rec.LatencyMS,
			CostUSD:        rec.CostUSD,
			Success:        rec.Success,
			ErrorKind:      rec.ErrorKind,
		}},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		log.Printf("usage: marshaling GSR report payload: %v", err)
		return
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, r.baseURL, bytes.NewReader(body))
	if err != nil {
		log.Printf("usage: building GSR report request: %v", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	// Shared-secret header, not a URL query param or Authorization header —
	// matches GSR's actual ingest contract (adk/backend/src/app.ts checks
	// X-Usage-Ingest-Key, not Authorization).
	req.Header.Set("X-Usage-Ingest-Key", r.key)

	resp, err := r.httpClient.Do(req)
	if err != nil {
		log.Printf("usage: reporting to GSR: %v", err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		log.Printf("usage: GSR ingest returned %d", resp.StatusCode)
	}
}
