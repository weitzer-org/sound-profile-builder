package runs

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/weitzer-org/sound-builder/internal/agents"
)

// persistTimeout bounds a single record write. Writes are detached from the
// request context so a client disconnect can't truncate the run log.
const persistTimeout = 10 * time.Second

// Recorder implements agents.RunObserver for one run and keeps its RunRecord.
// Phase 1/3/4 agents call it concurrently, so every mutation takes mu; storage
// writes happen outside the lock on a copy.
type Recorder struct {
	store *Store

	mu       sync.Mutex
	rec      RunRecord
	version  uint64 // bumped on every mutation
	onChange func()

	persistMu sync.Mutex
	persisted uint64 // highest version written; guarded by persistMu

	logWG sync.WaitGroup // in-flight prompt-log writes
}

var _ agents.RunObserver = (*Recorder)(nil)

// NewRecorder wraps rec (copied). CreatedAt/UpdatedAt default to now.
func NewRecorder(store *Store, rec RunRecord) *Recorder {
	now := time.Now().UTC()
	if rec.CreatedAt.IsZero() {
		rec.CreatedAt = now
	}
	if rec.UpdatedAt.IsZero() {
		rec.UpdatedAt = now
	}
	if rec.Status == "" {
		rec.Status = StatusRunning
	}
	if rec.Agents == nil {
		rec.Agents = map[string]*AgentTrace{}
	}
	return &Recorder{store: store, rec: rec.Clone()}
}

// SetOnChange registers fn, called (outside the lock) after every mutation.
// presetsvc uses it to wake long-pollers.
func (r *Recorder) SetOnChange(fn func()) {
	r.mu.Lock()
	r.onChange = fn
	r.mu.Unlock()
}

// Snapshot returns a deep copy of the current record.
func (r *Recorder) Snapshot() RunRecord {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.rec.Clone()
}

// apply runs fn under the lock, stamps UpdatedAt and bumps the version. It
// returns the change callback for the caller to invoke once it is ready to wake
// waiters (finish() persists first, so a woken waiter never reads stale state).
func (r *Recorder) apply(fn func(rec *RunRecord)) func() {
	r.mu.Lock()
	fn(&r.rec)
	r.rec.UpdatedAt = time.Now().UTC()
	r.version++
	cb := r.onChange
	r.mu.Unlock()
	return cb
}

// mutate applies fn and notifies waiters immediately.
func (r *Recorder) mutate(fn func(rec *RunRecord)) {
	if cb := r.apply(fn); cb != nil {
		cb()
	}
}

func (r *Recorder) addEvent(rec *RunRecord, ev RunEvent) {
	ev.Seq = len(rec.Events) + 1
	ev.TS = time.Now().UTC()
	rec.Events = append(rec.Events, ev)
}

// Flush persists the current state. Concurrent flushes coalesce: a snapshot
// older than one already written is dropped, so a slow write can never
// overwrite newer state.
func (r *Recorder) Flush() error {
	if r.store == nil {
		return nil
	}
	r.mu.Lock()
	snap := r.rec.Clone()
	v := r.version
	r.mu.Unlock()

	r.persistMu.Lock()
	defer r.persistMu.Unlock()
	if v <= r.persisted && r.persisted != 0 {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), persistTimeout)
	defer cancel()
	if err := r.store.Put(ctx, &snap); err != nil {
		log.Printf("runs: persist %s: %v", snap.ID, err)
		return err
	}
	r.persisted = v
	return nil
}

// OnPhase records a progress phase. A repeat of the current phase is ignored
// entirely (no version bump, no wake-up), since both the observer and the legacy
// onProgress callback report each phase and WaitRunChange must only fire on a
// real change.
func (r *Recorder) OnPhase(phase string) {
	r.mu.Lock()
	if r.rec.Phase == phase {
		r.mu.Unlock()
		return
	}
	r.rec.Phase = phase
	r.addEvent(&r.rec, RunEvent{Type: EventPhase, Msg: phase})
	r.rec.UpdatedAt = time.Now().UTC()
	r.version++
	cb := r.onChange
	r.mu.Unlock()
	if cb != nil {
		cb()
	}
	_ = r.Flush()
}

// OnInfo records a free-form note.
func (r *Recorder) OnInfo(msg string) {
	r.mutate(func(rec *RunRecord) { r.addEvent(rec, RunEvent{Type: EventInfo, Msg: msg}) })
}

func (r *Recorder) trace(rec *RunRecord, role string) *AgentTrace {
	t, ok := rec.Agents[role]
	if !ok {
		key := agents.AgentRoleKeys[role]
		t = &AgentTrace{Role: role, PromptKey: key, PromptVersion: rec.PromptVersions[key]}
		rec.Agents[role] = t
	}
	return t
}

// OnAgentStart opens the agent's trace and stores the exact prompts sent.
func (r *Recorder) OnAgentStart(role, systemPrompt, userPrompt string) {
	var key, id string
	logIt := false
	r.mutate(func(rec *RunRecord) {
		t := r.trace(rec, role)
		t.Status = "running"
		t.SystemPromptChars = utf8.RuneCountInString(systemPrompt)
		t.InputChars = utf8.RuneCountInString(userPrompt)
		key = t.PromptKey
		id = rec.ID
		r.addEvent(rec, RunEvent{Type: EventAgentStart, Agent: role, Msg: role + " started"})
		// Register the log write under the lock, and only while the run is still
		// running: finish() flips the status under the same lock before it Waits,
		// so no Add can race that Wait (e.g. shutdown abandoning a live run).
		if r.store != nil && key != "" && rec.Status == StatusRunning {
			r.logWG.Add(1)
			logIt = true
		}
	})
	if !logIt {
		return
	}
	// Prompt logs are written once per agent, off the agent's critical path (the
	// Librarian's input alone is ~100 KB). finish() waits for them, so they are
	// durable before the run reads as terminal. Failures are logged, never fatal.
	go func() {
		defer r.logWG.Done()
		ctx, cancel := context.WithTimeout(context.Background(), persistTimeout)
		defer cancel()
		if err := r.store.WriteLog(ctx, id, key+".system.md", []byte(systemPrompt)); err != nil {
			log.Printf("runs: write system prompt log %s/%s: %v", id, key, err)
		}
		if err := r.store.WriteLog(ctx, id, key+".input.txt", []byte(userPrompt)); err != nil {
			log.Printf("runs: write input log %s/%s: %v", id, key, err)
		}
	}()
}

// OnAttempt counts a fallback-chain attempt and logs failed ones.
func (r *Recorder) OnAttempt(role, model string, latencyMs int64, err error) {
	r.mutate(func(rec *RunRecord) {
		t := r.trace(rec, role)
		t.Attempts++
		if err != nil {
			r.addEvent(rec, RunEvent{Type: EventAttemptFailed, Agent: role, Model: model, LatencyMs: latencyMs,
				Msg: fmt.Sprintf("%s attempt on %s failed: %v", role, model, err)})
		}
	})
	// Heartbeat: every attempt is capped at 3 minutes, so persisting after each one
	// keeps UpdatedAt well inside the abandonment threshold however long a
	// fallback chain runs.
	_ = r.Flush()
}

// OnAgentDone closes the agent's trace, folds its usage into the totals and
// persists the record.
func (r *Recorder) OnAgentDone(role, model, output string, u agents.AgentUsage, err error) {
	var key, id string
	logOut := false
	r.mutate(func(rec *RunRecord) {
		t := r.trace(rec, role)
		// The pipeline's own logToGCS writes <key>.json for real generate runs.
		// Mock runs call no agents and refine runs never reach logToGCS, so the
		// recorder writes the output log for those, giving every run the same
		// logs/<run_id>/<key>.json layout (DESIGN §3.2).
		if r.store != nil && err == nil && t.PromptKey != "" && rec.Status == StatusRunning &&
			(rec.Mock || rec.Kind == KindRefine) {
			key, id = t.PromptKey, rec.ID
			r.logWG.Add(1)
			logOut = true
		}
		t.Model = model
		t.InputTokens = u.InputTokens
		t.OutputTokens = u.OutputTokens
		t.ThinkingTokens = u.ThinkingTokens
		t.LatencyMs = u.LatencyMs
		t.OutputChars = utf8.RuneCountInString(output)
		rec.Totals.InputTokens += u.InputTokens
		rec.Totals.OutputTokens += u.OutputTokens
		rec.Totals.ThinkingTokens += u.ThinkingTokens
		rec.Totals.Calls++
		if err != nil {
			t.Status = "error"
			t.Error = err.Error()
			r.addEvent(rec, RunEvent{Type: EventAgentError, Agent: role, Model: model, LatencyMs: u.LatencyMs, Msg: role + " failed: " + err.Error()})
			return
		}
		t.Status = "done"
		r.addEvent(rec, RunEvent{Type: EventAgentDone, Agent: role, Model: model, LatencyMs: u.LatencyMs, Msg: role + " finished"})
	})
	if logOut {
		go func() {
			defer r.logWG.Done()
			ctx, cancel := context.WithTimeout(context.Background(), persistTimeout)
			defer cancel()
			if werr := r.store.WriteLog(ctx, id, key+".json", []byte(output)); werr != nil {
				log.Printf("runs: write output log %s/%s: %v", id, key, werr)
			}
		}()
	}
	_ = r.Flush()
}

// SetEffectivePrompt records the prompt after config-flag prefixes (if known).
func (r *Recorder) SetEffectivePrompt(p string) {
	r.mutate(func(rec *RunRecord) { rec.EffectivePrompt = p })
}

// finish moves a running record to a terminal status. It reports whether the
// transition happened (a run already terminal is left alone).
func (r *Recorder) finish(status, errMsg, presetID string, waitLogs, retry bool) bool {
	did := false
	cb := r.apply(func(rec *RunRecord) {
		if rec.Status != StatusRunning {
			return
		}
		did = true
		now := time.Now().UTC()
		rec.Status = status
		rec.Error = errMsg
		if presetID != "" {
			rec.PresetID = presetID
		}
		rec.CompletedAt = &now
		msg := "run " + status
		if errMsg != "" {
			msg += ": " + errMsg
		}
		r.addEvent(rec, RunEvent{Type: EventInfo, Msg: msg})
	})
	if did {
		if waitLogs {
			r.logWG.Wait() // status is terminal now, so no new log writes register; let in-flight ones land
		}
		// Persist the terminal state BEFORE waking waiters, so anyone woken by
		// the change (WaitRun) and then reading storage sees the same status.
		r.flushTerminal(retry)
	}
	if cb != nil {
		cb()
	}
	return did
}

// Complete marks the run successful and links the resulting preset.
func (r *Recorder) Complete(presetID string) bool {
	return r.finish(StatusComplete, "", presetID, true, true)
}

// Fail marks the run failed.
func (r *Recorder) Fail(errMsg string) bool { return r.finish(StatusError, errMsg, "", true, true) }

// Abandon marks a still-running run abandoned (e.g. on shutdown). It is built
// for a tight deadline: it does not wait for in-flight prompt-log writes and
// makes a single flush attempt.
func (r *Recorder) Abandon(errMsg string) bool {
	return r.finish(StatusAbandoned, errMsg, "", false, false)
}

// flushTerminal persists a terminal record. A transient storage error would
// otherwise leave runs/<id>.json reading "running" (and later "abandoned") for
// a run that finished, so completion and failure retry a few times.
func (r *Recorder) flushTerminal(retry bool) {
	attempts := 1
	if retry {
		attempts = 3
	}
	var err error
	for i := 0; i < attempts; i++ {
		if err = r.Flush(); err == nil {
			return
		}
		if i < attempts-1 {
			time.Sleep(time.Duration(i+1) * 100 * time.Millisecond)
		}
	}
	log.Printf("runs: terminal state of the run was not persisted after %d attempt(s): %v", attempts, err)
}
