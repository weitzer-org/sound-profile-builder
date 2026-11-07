package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/weitzer-org/sound-builder/internal/agents"
	"github.com/weitzer-org/sound-builder/internal/config"
	"github.com/weitzer-org/sound-builder/internal/deviceexport"
	"github.com/weitzer-org/sound-builder/internal/evalcheck"
	er "github.com/weitzer-org/sound-builder/internal/evaldevicerefine"
)

func TestArmsDifferOnlyInTheSectionKey(t *testing.T) {
	cfg := &config.AppConfig{AgentPrompts: map[string]string{"12_architect": "v6", agents.DeviceEditPromptKey: "v1"}}
	a := versionsFor(cfg, er.ArmA, "v1")
	if _, has := a[agents.DeviceEditPromptKey]; has || a["12_architect"] != "v6" {
		t.Errorf("arm A must be today's refine even if config turns the section on: %v", a)
	}
	b := versionsFor(&config.AppConfig{AgentPrompts: map[string]string{"12_architect": "v6"}}, er.ArmB, "v1")
	if b[agents.DeviceEditPromptKey] != "v1" || b["12_architect"] != "v6" || len(b) != 2 {
		t.Errorf("arm B = %v", b)
	}
	if _, has := cfg.AgentPrompts[agents.DeviceEditPromptKey]; !has {
		t.Error("versionsFor must not modify the config map")
	}
}

// -mode score on saved outputs makes no Gemini call: this test has no key and no network.
func TestScoreModeOverSavedOutputs(t *testing.T) {
	e, err := er.NewExporter(nil)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	all, err := er.LoadCorpus(ctx, []string{"../../testdata/devicejobs/readback_roundtrip"}, e)
	if err != nil {
		t.Fatal(err)
	}
	d := all[0]
	out := t.TempDir()
	env := map[string]any{"conversational_response": "ok", "dsp_matrix_updated": true,
		"structured_payload": map[string]any{"guitars": map[string]any{deviceexport.ReadbackGuitar: d.Chain}}}
	body, _ := json.Marshal(env)
	var lines []string
	for _, arm := range []string{"A", "B"} {
		r := er.Run{Triple: er.Triple{Draft: d.Name, Request: "tone", Rep: 1}, Arm: arm}
		path := outputPath(out, r)
		os.MkdirAll(filepath.Dir(path), 0o755)
		os.WriteFile(path, body, 0o644)
		rec, _ := json.Marshal(evalcheck.RunRecord{Scenario: arm, Query: r.Triple.Key(), Rep: 1, Success: true, IntegrityOK: true, OutputFile: path})
		lines = append(lines, string(rec))
	}
	// A failed attempt in arm B on another triple, and a record for a draft not in the corpus.
	fail, _ := json.Marshal(evalcheck.RunRecord{Scenario: "B", Query: d.Name + "/scene", Rep: 1, Success: false, Error: "boom"})
	lines = append(lines, string(fail))
	os.WriteFile(filepath.Join(out, "runs.jsonl"), []byte(strings.Join(lines, "\n")+"\n"), 0o644)

	o := opts{out: out}
	runs := scoreRuns(o, er.NewScorer(e), all)
	if len(runs) != 3 {
		t.Fatalf("scored %d runs, want 3", len(runs))
	}
	pairs, dropped := er.Pairs(runs)
	if len(pairs) != 1 || len(dropped) != 1 {
		t.Fatalf("pairs %d dropped %d", len(pairs), len(dropped))
	}
	var sb strings.Builder
	er.Report(&sb, runs)
	costReport(&sb, o, runs)
	if !strings.Contains(sb.String(), "1 (draft, request, rep) triples usable in both arms; 1 triples left out") {
		t.Errorf("report:\n%s", sb.String())
	}

	// todo(): nothing is rerun, and a failed run is rerun only with -retry-failed.
	m := er.Matrix([]string{d.Name}, []string{"tone", "scene"}, er.Arms, 1)
	if got := len(todo(o, m)); got != 1 { // scene A never ran; scene B failed and is not retried
		t.Errorf("todo = %d, want 1", got)
	}
	o.retryFailed = true
	if got := len(todo(o, m)); got != 2 {
		t.Errorf("with -retry-failed todo = %d, want scene A and scene B", got)
	}
}
