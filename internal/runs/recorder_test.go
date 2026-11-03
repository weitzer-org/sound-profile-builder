package runs

import (
	"context"
	"fmt"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/weitzer-org/sound-builder/internal/agents"
	"github.com/weitzer-org/sound-builder/internal/storage"
)

// memClient is a minimal map-backed storage.Client. Other packages' fakes live
// in their own _test.go files and cannot be imported, so this lives here and is
// shared by the tests in this package.
type memClient struct {
	mu   sync.Mutex
	data map[string][]byte
}

var _ storage.Client = (*memClient)(nil)

func newMemClient() *memClient { return &memClient{data: make(map[string][]byte)} }

func (m *memClient) ReadFile(_ context.Context, bucket, object string) ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	v, ok := m.data[bucket+"/"+object]
	if !ok {
		return nil, fmt.Errorf("not found: %s/%s", bucket, object)
	}
	return append([]byte(nil), v...), nil
}

func (m *memClient) WriteFile(_ context.Context, bucket, object string, data []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.data[bucket+"/"+object] = append([]byte(nil), data...)
	return nil
}

func (m *memClient) ListFiles(_ context.Context, bucket, prefix string) ([]string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []string
	full := bucket + "/" + prefix
	for k := range m.data {
		if len(k) >= len(full) && k[:len(full)] == full {
			out = append(out, k[len(bucket)+1:])
		}
	}
	return out, nil
}

func (m *memClient) DeleteFile(_ context.Context, bucket, object string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.data, bucket+"/"+object)
	return nil
}

func (m *memClient) Close() {}

const testBucket = "test-bucket"

func newTestStore(t *testing.T) (*Store, *memClient) {
	t.Helper()
	mc := newMemClient()
	return NewStore(mc, testBucket), mc
}

func TestRecorderConcurrentAgents(t *testing.T) {
	store, mc := newTestStore(t)
	rec := NewRecorder(store, RunRecord{ID: "gen-test-1", Kind: KindGenerate, Source: SourceUI})

	roles := []string{"Tone Historian", "Sonic Profiler", "Community Scraper"}
	usage := agents.AgentUsage{InputTokens: 10, OutputTokens: 20, ThinkingTokens: 5, LatencyMs: 30}

	var wg sync.WaitGroup
	start := make(chan struct{})
	for _, role := range roles {
		wg.Add(1)
		go func(role string) {
			defer wg.Done()
			<-start
			rec.OnAgentStart(role, "system "+role, "input "+role)
			rec.OnAttempt(role, "gemini-test", 5, nil)
			rec.OnAgentDone(role, "gemini-test", "output "+role, usage, nil)
		}(role)
	}
	close(start)
	wg.Wait()

	snap := rec.Snapshot()
	if got := len(snap.Agents); got != len(roles) {
		t.Fatalf("agent traces: got %d, want %d", got, len(roles))
	}
	for _, role := range roles {
		tr, ok := snap.Agents[role]
		if !ok {
			t.Fatalf("missing trace for %q", role)
		}
		if tr.Status != "done" {
			t.Errorf("%s: status %q, want done", role, tr.Status)
		}
		if tr.Attempts != 1 {
			t.Errorf("%s: attempts %d, want 1", role, tr.Attempts)
		}
		if tr.PromptKey != agents.AgentRoleKeys[role] {
			t.Errorf("%s: prompt key %q, want %q", role, tr.PromptKey, agents.AgentRoleKeys[role])
		}
		if tr.OutputChars != len("output "+role) {
			t.Errorf("%s: output chars %d", role, tr.OutputChars)
		}
	}
	if snap.Totals.Calls != len(roles) {
		t.Errorf("totals calls %d, want %d", snap.Totals.Calls, len(roles))
	}
	if want := int32(len(roles)) * usage.InputTokens; snap.Totals.InputTokens != want {
		t.Errorf("totals input tokens %d, want %d", snap.Totals.InputTokens, want)
	}

	// Each agent contributes one start and one done event; attempts without
	// an error add none.
	if got, want := len(snap.Events), 2*len(roles); got != want {
		t.Errorf("events: got %d, want %d", got, want)
	}

	// The last flush must have reached storage with the complete state.
	data, err := mc.ReadFile(context.Background(), testBucket, recordKey("gen-test-1"))
	if err != nil {
		t.Fatalf("persisted record missing: %v", err)
	}
	persisted, err := store.Get(context.Background(), "gen-test-1")
	if err != nil {
		t.Fatalf("Get: %v (raw %d bytes)", err, len(data))
	}
	if len(persisted.Agents) != len(roles) || len(persisted.Events) != len(snap.Events) {
		t.Errorf("persisted record incomplete: %d agents, %d events", len(persisted.Agents), len(persisted.Events))
	}
}

func TestRecorderEventSeqStrictlyIncreasing(t *testing.T) {
	type op struct {
		name string
		do   func(r *Recorder)
	}
	usage := agents.AgentUsage{LatencyMs: 7}
	tests := []struct {
		name  string
		ops   []op
		types []string // expected event types, in order
	}{
		{
			name: "happy path",
			ops: []op{
				{"phase", func(r *Recorder) { r.OnPhase("plan") }},
				{"info", func(r *Recorder) { r.OnInfo("hello") }},
				{"agent start", func(r *Recorder) { r.OnAgentStart("Tone Historian", "s", "u") }},
				{"agent done", func(r *Recorder) { r.OnAgentDone("Tone Historian", "m", "out", usage, nil) }},
				{"complete", func(r *Recorder) { r.Complete("preset-1") }},
			},
			types: []string{EventPhase, EventInfo, EventAgentStart, EventAgentDone, EventInfo},
		},
		{
			name: "repeat phase ignored, failures logged",
			ops: []op{
				{"phase", func(r *Recorder) { r.OnPhase("plan") }},
				{"same phase", func(r *Recorder) { r.OnPhase("plan") }},
				{"agent start", func(r *Recorder) { r.OnAgentStart("Sonic Profiler", "s", "u") }},
				{"attempt failed", func(r *Recorder) { r.OnAttempt("Sonic Profiler", "m1", 3, fmt.Errorf("boom")) }},
				{"agent error", func(r *Recorder) { r.OnAgentDone("Sonic Profiler", "", "", usage, fmt.Errorf("bad")) }},
				{"fail", func(r *Recorder) { r.Fail("run broke") }},
			},
			types: []string{EventPhase, EventAgentStart, EventAttemptFailed, EventAgentError, EventInfo},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store, _ := newTestStore(t)
			rec := NewRecorder(store, RunRecord{ID: "gen-seq-1"})
			for _, o := range tt.ops {
				o.do(rec)
			}
			events := rec.Snapshot().Events
			if len(events) != len(tt.types) {
				t.Fatalf("events: got %d, want %d (%+v)", len(events), len(tt.types), events)
			}
			for i, ev := range events {
				if ev.Seq != i+1 {
					t.Errorf("event %d: Seq %d, want %d", i, ev.Seq, i+1)
				}
				if ev.Type != tt.types[i] {
					t.Errorf("event %d: type %q, want %q", i, ev.Type, tt.types[i])
				}
			}
		})
	}
}

func TestRecorderConcurrentSeqStrictlyIncreasing(t *testing.T) {
	store, _ := newTestStore(t)
	rec := NewRecorder(store, RunRecord{ID: "gen-seq-2"})

	const workers = 3
	const perWorker = 20
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for i := 0; i < perWorker; i++ {
				rec.OnInfo(fmt.Sprintf("w%d-%d", w, i))
			}
		}(w)
	}
	wg.Wait()

	events := rec.Snapshot().Events
	if len(events) != workers*perWorker {
		t.Fatalf("events: got %d, want %d", len(events), workers*perWorker)
	}
	for i, ev := range events {
		if ev.Seq != i+1 {
			t.Fatalf("event %d: Seq %d, want %d", i, ev.Seq, i+1)
		}
	}
}

func TestStorePutGetRoundTrip(t *testing.T) {
	t0 := time.Date(2026, 11, 3, 14, 0, 0, 0, time.UTC)
	completed := t0.Add(2 * time.Minute)

	base := func() RunRecord {
		return RunRecord{
			ID:              "gen-roundtrip-1",
			Kind:            KindRefine,
			Source:          SourceMCP,
			Status:          StatusComplete,
			Phase:           "done",
			UserPrompt:      "warm bluesy lead",
			EffectivePrompt: "GLOBAL: warm bluesy lead",
			Constraints:     map[string]any{"max_blocks": "8", "cab_only": true},
			PromptVersions:  map[string]string{"1_tone_historian": "v3"},
			PresetID:        "preset-1",
			ParentPresetID:  "preset-0",
			Agents: map[string]*AgentTrace{
				"Tone Historian": {
					Role: "Tone Historian", Model: "gemini-test", Status: "done",
					PromptKey: "1_tone_historian", PromptVersion: "v3",
					InputTokens: 11, OutputTokens: 22, ThinkingTokens: 3, LatencyMs: 44,
					Attempts: 1, SystemPromptChars: 100, InputChars: 50, OutputChars: 60,
				},
			},
			Events: []RunEvent{
				{Seq: 1, TS: t0, Type: EventPhase, Msg: "done"},
				{Seq: 2, TS: t0, Type: EventAgentDone, Agent: "Tone Historian", Model: "gemini-test", LatencyMs: 44, Msg: "finished"},
			},
			Totals:      Usage{InputTokens: 11, OutputTokens: 22, ThinkingTokens: 3, Calls: 1},
			Mock:        true,
			CreatedAt:   t0,
			UpdatedAt:   t0,
			CompletedAt: &completed,
		}
	}

	tests := []struct {
		name   string
		mutate func(r *RunRecord)
	}{
		{"complete record", func(r *RunRecord) {}},
		{"error record with error text", func(r *RunRecord) {
			r.Status = StatusError
			r.Error = "gemini quota exceeded"
		}},
		{"running record, fresh update", func(r *RunRecord) {
			r.Status = StatusRunning
			r.CompletedAt = nil
		}},
		{"no agents, no events", func(r *RunRecord) {
			r.Agents = map[string]*AgentTrace{}
			r.Events = nil
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store, _ := newTestStore(t)
			// Pin the clock so a running record is judged fresh, not stale.
			store.now = func() time.Time { return t0.Add(time.Minute) }

			want := base()
			tt.mutate(&want)
			if err := store.Put(context.Background(), &want); err != nil {
				t.Fatalf("Put: %v", err)
			}
			got, err := store.Get(context.Background(), want.ID)
			if err != nil {
				t.Fatalf("Get: %v", err)
			}
			if !reflect.DeepEqual(*got, want) {
				t.Errorf("round trip mismatch\n got: %+v\nwant: %+v", *got, want)
			}
		})
	}
}

func TestStoreStaleRunningReadsAbandoned(t *testing.T) {
	now := time.Date(2026, 11, 3, 14, 0, 0, 0, time.UTC)

	tests := []struct {
		name         string
		status       string
		age          time.Duration // now - UpdatedAt
		abandonAfter time.Duration // zero leaves the default (10m)
		want         string
	}{
		{"running and older than default threshold", StatusRunning, 11 * time.Minute, 0, StatusAbandoned},
		{"running exactly at threshold stays running", StatusRunning, 10 * time.Minute, 0, StatusRunning},
		{"running and fresh", StatusRunning, 30 * time.Second, 0, StatusRunning},
		{"running past a custom shorter threshold", StatusRunning, 2 * time.Minute, time.Minute, StatusAbandoned},
		{"running within a custom longer threshold", StatusRunning, 11 * time.Minute, 15 * time.Minute, StatusRunning},
		{"complete and stale is left alone", StatusComplete, 2 * time.Hour, 0, StatusComplete},
		{"error and stale is left alone", StatusError, 2 * time.Hour, 0, StatusError},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store, mc := newTestStore(t)
			store.now = func() time.Time { return now }
			if tt.abandonAfter != 0 {
				store.AbandonAfter = tt.abandonAfter
			}
			rec := RunRecord{ID: "gen-stale-1", Kind: KindGenerate, Source: SourceUI,
				Status: tt.status, CreatedAt: now.Add(-tt.age), UpdatedAt: now.Add(-tt.age)}
			if err := store.Put(context.Background(), &rec); err != nil {
				t.Fatalf("Put: %v", err)
			}

			got, err := store.Get(context.Background(), rec.ID)
			if err != nil {
				t.Fatalf("Get: %v", err)
			}
			if got.Status != tt.want {
				t.Fatalf("status: got %q, want %q", got.Status, tt.want)
			}
			if tt.want != StatusAbandoned {
				return
			}
			if got.Error == "" || got.CompletedAt == nil {
				t.Errorf("abandoned record missing Error or CompletedAt: %+v", got)
			}
			if n := len(got.Events); n != 1 || got.Events[0].Type != EventInfo || got.Events[0].Seq != 1 {
				t.Errorf("expected one info event marking abandonment, got %+v", got.Events)
			}

			// The verdict is written back once: a fresh Store over the same
			// client reads it as abandoned without adding another event.
			reader := &Store{client: mc, bucket: testBucket, AbandonAfter: store.AbandonAfter, now: store.now}
			again, err := reader.Get(context.Background(), rec.ID)
			if err != nil {
				t.Fatalf("second Get: %v", err)
			}
			if again.Status != StatusAbandoned || len(again.Events) != 1 {
				t.Errorf("persisted verdict: status %q, %d events; want abandoned, 1 event", again.Status, len(again.Events))
			}
		})
	}
}
