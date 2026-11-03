// Package presetsvc owns preset generation, refinement and saving. It is the
// logic that used to live inline in the HTMX handlers (handleGeneratePreset,
// handleChatPreset), extracted so other front ends can share it. It returns
// plain Go structs, never HTML.
//
// Every LLM run goes through here, which is also where the run log
// (internal/runs) is created and where the global concurrency cap applies.
package presetsvc

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"

	"github.com/weitzer-org/sound-builder/internal/agents"
	"github.com/weitzer-org/sound-builder/internal/config"
	"github.com/weitzer-org/sound-builder/internal/runs"
	"github.com/weitzer-org/sound-builder/internal/storage"
)

// DefaultMaxConcurrentRuns is the spend guard applied to all LLM runs.
const DefaultMaxConcurrentRuns = 2

// finishedRunTTL is how long a finished run stays in the in-memory cache
// before it is evicted (it remains readable from storage).
const finishedRunTTL = time.Hour

// ErrBusy means the concurrency cap is reached; the caller should retry later.
var ErrBusy = errors.New("another run is in progress; try again shortly")

// ErrPresetBusy means a refinement of the same preset is still running. Refinements
// read the preset, change it and save it whole, so two at once would silently
// drop one of the changes.
var ErrPresetBusy = errors.New("this preset is already being refined; wait for that change to finish before sending another")

// abandonDeadline bounds how long AbandonActive waits for its flushes; a stopping
// server has little time and must not block on a stalled storage backend.
var abandonDeadline = 4 * time.Second

// Error kinds recorded on a failed run (RunResult.ErrKind), so front ends can
// word the same failure their own way without parsing messages.
const (
	ErrKindInit      = "init"      // orchestrator could not be constructed
	ErrKindExec      = "exec"      // the pipeline / refinement agent failed
	ErrKindSerialize = "serialize" // the architect's output could not be decoded or re-encoded
	ErrKindStorage   = "storage"   // saving the result failed
	ErrKindInternal  = "internal"  // recovered panic
)

// CredentialsError is returned when the LLM API key cannot be resolved.
// Public is a message safe to show to the end user.
type CredentialsError struct {
	Public string
	Err    error
}

func (e *CredentialsError) Error() string { return e.Public }
func (e *CredentialsError) Unwrap() error { return e.Err }

// LookupError is returned by StartRefine when the preset cannot be loaded.
type LookupError struct{ Err error }

func (e *LookupError) Error() string { return e.Err.Error() }
func (e *LookupError) Unwrap() error { return e.Err }

// GenerateOptions tunes one generate run. Nil pointers mean "use the config
// default" (for the allow_* switches: allowed, further limited by config).
type GenerateOptions struct {
	AllowFactoryCaptures, AllowUserCaptures, AllowPaidPlugins *bool
	FavorCaptures, FavorCloudCaptures                         *bool
	Mock                                                      bool
	Source                                                    string // "ui" | "mcp"
}

// Deps are the Service's collaborators. Secrets, OrchMaker and Config are
// functions, not values, so the owner can swap the underlying dependency after
// construction (the HTTP server's tests do) and the Service still sees it.
type Deps struct {
	Store     *storage.PresetStore
	Memories  *storage.MemoryStore
	Client    storage.Client
	Bucket    string
	Secrets   func() storage.SecretFetcher
	OrchMaker func(ctx context.Context, apiKey string) (agents.OrchestratorService, error)
	Config    func() *config.AppConfig
}

// RunResult is the in-memory outcome of a run that this process started.
type RunResult struct {
	Record  runs.RunRecord
	Preset  *storage.Preset // set once the run completes
	ErrKind string          // set when the run failed
}

// Service runs and tracks presets' LLM runs.
type Service struct {
	d    Deps
	runs *runs.Store
	sem  chan struct{}

	keyMu    sync.RWMutex
	keyCache string

	mu       sync.Mutex
	live     map[string]*liveRun
	refining map[string]struct{} // preset IDs with a refinement in flight
}

// New builds a Service. The concurrency cap comes from MCP_MAX_CONCURRENT_RUNS
// (default 2).
func New(d Deps) *Service {
	max := DefaultMaxConcurrentRuns
	if v := os.Getenv("MCP_MAX_CONCURRENT_RUNS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 1 {
			max = n
		} else {
			log.Printf("presetsvc: ignoring invalid MCP_MAX_CONCURRENT_RUNS %q", v)
		}
	}
	return &Service{
		d:        d,
		runs:     runs.NewStore(d.Client, d.Bucket),
		sem:      make(chan struct{}, max),
		live:     make(map[string]*liveRun),
		refining: make(map[string]struct{}),
	}
}

// Runs exposes the run store (for readers of persisted logs).
func (s *Service) Runs() *runs.Store { return s.runs }

// liveRun is one run this process started and still caches.
type liveRun struct {
	rec *runs.Recorder

	mu      sync.Mutex
	changed chan struct{}
	preset  *storage.Preset
	errKind string
	doneAt  time.Time
}

func newLiveRun(rec *runs.Recorder) *liveRun {
	l := &liveRun{rec: rec, changed: make(chan struct{})}
	rec.SetOnChange(l.notify)
	return l
}

// notify wakes every waiter and arms a fresh channel for the next change.
func (l *liveRun) notify() {
	l.mu.Lock()
	close(l.changed)
	l.changed = make(chan struct{})
	l.mu.Unlock()
}

func (l *liveRun) changeChan() <-chan struct{} {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.changed
}

func (l *liveRun) markDone() {
	l.mu.Lock()
	l.doneAt = time.Now()
	l.mu.Unlock()
}

func (s *Service) register(lr *liveRun, id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	for k, v := range s.live {
		v.mu.Lock()
		stale := !v.doneAt.IsZero() && now.Sub(v.doneAt) > finishedRunTTL
		v.mu.Unlock()
		if stale {
			delete(s.live, k)
		}
	}
	s.live[id] = lr
}

func (s *Service) liveRun(id string) *liveRun {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.live[id]
}

// tryAcquire takes a slot of the concurrency cap without blocking.
func (s *Service) tryAcquire() bool {
	select {
	case s.sem <- struct{}{}:
		return true
	default:
		return false
	}
}

func (s *Service) release() { <-s.sem }

func (s *Service) config() *config.AppConfig {
	if s.d.Config == nil {
		return nil
	}
	return s.d.Config()
}

func newRunID(prefix string) string { return prefix + uuid.New().String() }

func isMock(requested bool) bool { return requested || os.Getenv("MOCK_MODE") == "true" }

// startRun registers a run record and returns the context its goroutine must
// use: detached from the caller, mock flag, run ID and observer attached.
func (s *Service) startRun(parent context.Context, rec runs.RunRecord, mock bool) (*liveRun, context.Context) {
	if mock {
		rec.Mock = true
	}
	recorder := runs.NewRecorder(s.runs, rec)
	lr := newLiveRun(recorder)
	s.register(lr, rec.ID)
	if err := recorder.Flush(); err != nil {
		log.Printf("presetsvc: initial run record write failed for %s: %v", rec.ID, err)
	}
	ctx := context.WithoutCancel(parent)
	if mock {
		ctx = context.WithValue(ctx, agents.MockModeKey, true)
	}
	ctx = agents.WithRunID(ctx, rec.ID)
	ctx = agents.WithRunObserver(ctx, recorder)
	return lr, ctx
}

func (s *Service) succeed(lr *liveRun, p *storage.Preset) {
	lr.mu.Lock()
	lr.preset = p
	lr.mu.Unlock()
	lr.rec.Complete(p.ID)
	lr.markDone()
}

func (s *Service) fail(lr *liveRun, kind, msg string) {
	lr.mu.Lock()
	lr.errKind = kind
	lr.mu.Unlock()
	lr.rec.Fail(msg)
	lr.markDone()
}

// Run returns a run's record: from memory if this process started it,
// otherwise from storage (where a stale "running" record reads as abandoned).
func (s *Service) Run(ctx context.Context, runID string) (*runs.RunRecord, error) {
	if lr := s.liveRun(runID); lr != nil {
		snap := lr.rec.Snapshot()
		return &snap, nil
	}
	return s.runs.Get(ctx, runID)
}

// Result returns the in-memory outcome (record plus resulting preset) of a run
// this process started and still caches.
func (s *Service) Result(runID string) (*RunResult, bool) {
	lr := s.liveRun(runID)
	if lr == nil {
		return nil, false
	}
	lr.mu.Lock()
	res := &RunResult{Preset: lr.preset, ErrKind: lr.errKind}
	lr.mu.Unlock()
	res.Record = lr.rec.Snapshot()
	return res, true
}

// WaitRun blocks until the run finishes, max elapses, or ctx ends, and returns
// the record as of then. A run that is already finished returns immediately.
func (s *Service) WaitRun(ctx context.Context, runID string, max time.Duration) (*runs.RunRecord, error) {
	return s.wait(ctx, runID, max, true)
}

// WaitRunChange is WaitRun that also returns on the first state change (a new
// phase, an agent finishing) after the call began.
func (s *Service) WaitRunChange(ctx context.Context, runID string, max time.Duration) (*runs.RunRecord, error) {
	return s.wait(ctx, runID, max, false)
}

func (s *Service) wait(ctx context.Context, runID string, max time.Duration, untilDone bool) (*runs.RunRecord, error) {
	lr := s.liveRun(runID)
	if lr == nil {
		return s.runs.Get(ctx, runID)
	}
	timer := time.NewTimer(max)
	defer timer.Stop()
	for {
		// Take the channel before the snapshot so a change landing in between
		// still wakes us.
		ch := lr.changeChan()
		snap := lr.rec.Snapshot()
		if snap.Status != runs.StatusRunning || max <= 0 {
			return &snap, nil
		}
		select {
		case <-ch:
			if !untilDone {
				fresh := lr.rec.Snapshot()
				return &fresh, nil
			}
		case <-timer.C:
			fresh := lr.rec.Snapshot()
			return &fresh, nil
		case <-ctx.Done():
			fresh := lr.rec.Snapshot()
			return &fresh, ctx.Err()
		}
	}
}

// AbandonActive marks every still-running run abandoned with msg and flushes
// each record. Called on shutdown so a stopped server reports a clear verdict
// immediately instead of leaving runs to look alive until the staleness
// threshold. The runs are flushed in parallel and the whole call is bounded by
// abandonDeadline, so a stalled storage backend can't hold up the shutdown. It
// returns how many runs were marked.
func (s *Service) AbandonActive(msg string) int {
	s.mu.Lock()
	all := make([]*liveRun, 0, len(s.live))
	for _, lr := range s.live {
		all = append(all, lr)
	}
	s.mu.Unlock()

	var n atomic.Int32
	var wg sync.WaitGroup
	for _, lr := range all {
		wg.Add(1)
		go func(lr *liveRun) {
			defer wg.Done()
			if lr.rec.Abandon(msg) {
				lr.markDone()
				n.Add(1)
			}
		}(lr)
	}
	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(abandonDeadline):
		log.Printf("presetsvc: AbandonActive still flushing after %s; shutting down anyway", abandonDeadline)
	}
	return int(n.Load())
}

// Save names a preset (e.g. promotes "Draft Preset" to a saved name).
func (s *Service) Save(ctx context.Context, presetID, name string) (*storage.Preset, error) {
	name = strings.TrimSpace(name)
	if presetID == "" || name == "" {
		return nil, errors.New("preset id and name are required")
	}
	p, err := s.d.Store.Get(ctx, presetID)
	if err != nil {
		return nil, &LookupError{Err: err}
	}
	p.Name = name
	if err := s.d.Store.Save(ctx, p); err != nil {
		return nil, fmt.Errorf("saving preset: %w", err)
	}
	return p, nil
}

// apiKey resolves the LLM key, caching it after the first success. Failures
// are *CredentialsError carrying a user-safe message.
func (s *Service) apiKey(ctx context.Context) (string, error) {
	s.keyMu.RLock()
	key := s.keyCache
	s.keyMu.RUnlock()
	if key != "" {
		return key, nil
	}

	s.keyMu.Lock()
	defer s.keyMu.Unlock()
	if s.keyCache != "" {
		return s.keyCache, nil
	}

	projectID := "710019748844"
	if cfg := s.config(); cfg != nil && cfg.ProjectID != "" {
		projectID = cfg.ProjectID
	}

	var err error
	if os.Getenv("USE_OPENLLM") == "true" {
		key = os.Getenv("OPENLLM_API_KEY")
		if key == "" {
			key, err = s.d.Secrets().GetPassword(ctx, projectID, "open-llm-api-auth-secret")
			if err != nil {
				log.Printf("Failed to fetch Open-LLM API key from Secret Manager: %v", err)
				return "", &CredentialsError{Public: "Missing Secure Open-LLM Credentials", Err: err}
			}
		}
	} else {
		secretName := os.Getenv("GEMINI_API_KEY_NAME")
		if secretName == "" {
			secretName = "gsr-gemini-api-key"
		}
		key, err = s.d.Secrets().GetPassword(ctx, projectID, secretName)
		if err != nil {
			log.Printf("Failed to fetch Gemini API key: %v", err)
			return "", &CredentialsError{Public: "Missing Secure AI Credentials", Err: err}
		}
	}
	key = strings.TrimSpace(key)
	if key == "" {
		log.Printf("Security alert: Loaded or fetched dynamic key is blank/empty!")
		return "", &CredentialsError{Public: "Invalid Secure AI Credentials Configuration", Err: errors.New("blank API key")}
	}
	s.keyCache = key
	return key, nil
}
