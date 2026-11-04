package runs

import (
	"context"
	"errors"
	"testing"

	"github.com/weitzer-org/sound-builder/internal/agents"
)

// Sizes are characters, not bytes: paging tools slice by character, and the
// real prompts contain non-ASCII text (coros_map.json, several prompt files).
func TestRecorderCountsCharactersNotBytes(t *testing.T) {
	store, _ := newTestStore(t)
	rec := NewRecorder(store, RunRecord{ID: "gen-chars", Kind: KindGenerate, Status: StatusRunning})
	rec.OnAgentStart("Tone Historian", "système", "Fender – Twin")
	rec.OnAgentDone("Tone Historian", "m", "°C ok", agents.AgentUsage{}, nil)
	tr := rec.Snapshot().Agents["Tone Historian"]
	if tr.SystemPromptChars != 7 || tr.InputChars != 13 || tr.OutputChars != 5 {
		t.Errorf("chars system/input/output = %d/%d/%d, want 7/13/5", tr.SystemPromptChars, tr.InputChars, tr.OutputChars)
	}
	rec.Complete("p1")
}

// The recorder writes <key>.json only for runs logToGCS never covers (mock and
// refine); a real generate run's output log stays logToGCS's alone.
func TestRecorderOutputLogOnlyForMockAndRefine(t *testing.T) {
	cases := []struct {
		name  string
		rec   RunRecord
		role  string
		key   string
		write bool
	}{
		{"mock generate", RunRecord{ID: "gen-mock", Kind: KindGenerate, Mock: true}, "Tone Historian", "1_tone_historian", true},
		{"refine", RunRecord{ID: "chat-real", Kind: KindRefine}, "Refinement Architect", "12_architect", true},
		{"real generate", RunRecord{ID: "gen-real", Kind: KindGenerate}, "Tone Historian", "1_tone_historian", false},
	}
	for _, c := range cases {
		store, _ := newTestStore(t)
		c.rec.Status = StatusRunning
		r := NewRecorder(store, c.rec)
		r.OnAgentStart(c.role, "sys", "in")
		r.OnAgentDone(c.role, "m", "the output", agents.AgentUsage{}, nil)
		r.Complete("p1") // waits for pending log writes
		got, err := store.ReadLog(context.Background(), c.rec.ID, c.key+".json")
		if c.write && (err != nil || string(got) != "the output") {
			t.Errorf("%s: output log = %q, %v; want it written", c.name, got, err)
		}
		if !c.write && !errors.Is(err, ErrNotFound) {
			t.Errorf("%s: recorder wrote an output log (%q) that logToGCS owns", c.name, got)
		}
		if sys, err := store.ReadLog(context.Background(), c.rec.ID, c.key+".system.md"); err != nil || string(sys) != "sys" {
			t.Errorf("%s: system log = %q, %v", c.name, sys, err)
		}
	}
}

func TestReadLogRejectsUnsafeNames(t *testing.T) {
	store, _ := newTestStore(t)
	for _, tc := range []struct{ id, name string }{
		{"gen-1", "../runs/gen-1.json"},
		{"gen-1", "x/y.json"},
		{"gen-1", "1_tone_historian.txt"},
		{"gen-1", ".json"},
		{"../x", "1_tone_historian.json"},
		{"gen-1/..", "1_tone_historian.json"},
	} {
		if _, err := store.ReadLog(context.Background(), tc.id, tc.name); !errors.Is(err, ErrInvalidID) {
			t.Errorf("ReadLog(%q, %q) err = %v, want ErrInvalidID", tc.id, tc.name, err)
		}
	}
}
