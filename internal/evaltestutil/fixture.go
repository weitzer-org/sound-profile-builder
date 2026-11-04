// Package evaltestutil builds synthetic eval_full_pipeline result directories for tests. It
// exists so the Phase 2 tools are validated on fixtures whose every number is known, with zero
// Gemini calls. Not used by production code.
package evaltestutil

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/weitzer-org/sound-builder/internal/agents"
	"github.com/weitzer-org/sound-builder/internal/evalcheck"
)

// Roles is the 13 pipeline agents in order, with the prompt key each routes by.
var Roles = []struct{ Role, Key string }{
	{"Tone Historian", "1_tone_historian"}, {"Sonic Profiler", "2_sonic_profiler"},
	{"Community Scraper", "3_community_scraper"}, {"CorOS Librarian", "4_coros_librarian"},
	{"Cloud Navigator", "5_cloud_navigator"}, {"Acoustician", "6_acoustician"},
	{"Transducer Tech", "7_transducer_tech"}, {"FOH Optimizer", "8_foh_optimizer"},
	{"Mix Engineer", "9_mix_engineer"}, {"Control Mapper", "10_control_mapper"},
	{"DSP Dispatcher", "11_dsp_dispatcher"}, {"Architect & Evaluator", "12_architect"},
	{"Preset Critic", "13_critic"},
}

// Routing builds a full 13-key agent->model map: pro for agents 1 and 12, flash for 2-11, critic for 13.
func Routing(pro, flash, critic string) map[string]string {
	m := map[string]string{}
	for _, r := range Roles {
		switch r.Key {
		case "1_tone_historian", "12_architect":
			m[r.Key] = pro
		case "13_critic":
			m[r.Key] = critic
		default:
			m[r.Key] = flash
		}
	}
	return m
}

// Calls builds one clean call record per agent: attempted on the routed model, served as the pin
// (or the same id), STOP, with the given token volumes and latency.
func Calls(route, pins map[string]string, in, out, think int32, latencyMs int64) []agents.CallRecord {
	var cs []agents.CallRecord
	for _, r := range Roles {
		want := route[r.Key]
		served := want
		if p, ok := pins[want]; ok {
			served = p
		}
		cs = append(cs, agents.CallRecord{Agent: r.Role, AttemptModel: want, ServedModel: served, FinishReason: "STOP", Success: true,
			InputTokens: in, OutputTokens: out, ThinkingTokens: think, LatencyMs: latencyMs})
	}
	return cs
}

// Block is one effect block of a synthetic Architect output.
type Block struct {
	Type, Model, Rationale string
	Params                 [][2]string
}

// Envelope renders a pipeline output envelope with a single guitar "G".
func Envelope(blocks ...Block) string {
	var bs []map[string]any
	for i, b := range blocks {
		var ps []map[string]string
		for _, p := range b.Params {
			ps = append(ps, map[string]string{"name": p[0], "value": p[1], "type": "slider"})
		}
		bs = append(bs, map[string]any{"id": fmt.Sprintf("b%d", i+1), "type": b.Type, "model": b.Model, "rationale": b.Rationale, "parameters": ps})
	}
	raw, _ := json.Marshal(map[string]any{"builder_statement": "x", "structured_payload": map[string]any{"guitars": map[string]any{"G": bs}}})
	return string(raw)
}

// CleanOutput is a defect-free output (Iba Green is a capture in coros_map.json and is dB-formatted).
func CleanOutput() string {
	return Envelope(
		Block{Type: "Amplifier", Model: "Iba Green", Rationale: "Pre-trained Neural Capture of a Marshall-style amp.", Params: [][2]string{{"Gain", "+1.0 dB"}, {"Bass", "0.0 dB"}, {"Mid", "+0.5 dB"}, {"Treble", "0.0 dB"}, {"Volume", "-1.0 dB"}}},
		Block{Type: "Reverb", Model: "Spring Reverb", Rationale: "Spring.", Params: [][2]string{{"Mix", "20%"}}},
	)
}

// R1R2Output claims capture on, and dB-formats, a modeled amp (UK C30 TopBoost): R1=1, R2>0.
func R1R2Output() string {
	return Envelope(Block{Type: "Amplifier", Model: "UK C30 TopBoost", Rationale: "Factory Neural Capture of the Vox AC30.", Params: [][2]string{{"Gain", "+1.0 dB"}, {"Bass", "0.0 dB"}}})
}

// Run describes one synthetic run.
type Run struct {
	Scenario string
	Query    string
	Rep      int
	Success  bool
	// IntegrityOK is what runs.jsonl SAYS. Calls decides what the calls file says.
	IntegrityOK bool
	LatencySec  float64
	Calls       []agents.CallRecord
	Output      string // empty: no output file written
	MaxTokens   map[string]int
}

// WriteBatch writes manifest.json, runs.jsonl, per-run calls and output files under dir.
func WriteBatch(t testing.TB, dir string, m evalcheck.Manifest, runs []Run) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := evalcheck.WriteManifest(dir, m); err != nil {
		t.Fatal(err)
	}
	f, err := os.Create(filepath.Join(dir, "runs.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	for _, r := range runs {
		k := evalcheck.RunKey{Scenario: r.Scenario, Query: r.Query, Rep: r.Rep}
		rd := filepath.Dir(evalcheck.OutputPath(dir, k))
		if err := os.MkdirAll(rd, 0o755); err != nil {
			t.Fatal(err)
		}
		rec := evalcheck.RunRecord{BatchID: m.BatchID, Scenario: r.Scenario, Query: r.Query, Rep: r.Rep, Success: r.Success,
			LatencySec: r.LatencySec, IntegrityOK: r.IntegrityOK, MaxTokensByAgent: r.MaxTokens}
		if !r.Success {
			rec.Error = "Phase 3 failures: synthetic"
		}
		if r.Output != "" {
			rec.OutputFile = evalcheck.OutputPath(dir, k)
			if err := os.WriteFile(rec.OutputFile, []byte(r.Output), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		if r.Calls != nil {
			b, _ := json.Marshal(r.Calls)
			if err := os.WriteFile(evalcheck.CallsPath(dir, k), b, 0o644); err != nil {
				t.Fatal(err)
			}
		}
		line, _ := json.Marshal(rec)
		fmt.Fprintln(f, string(line))
	}
}

// Manifest builds a manifest for the given scenarios (id -> routing) and queries.
func Manifest(batch string, smoke bool, reps int, queries []string, scenarios []evalcheck.ManifestScenario) evalcheck.Manifest {
	return evalcheck.Manifest{BatchID: batch, CreatedAt: "2026-11-05T00:00:00Z", Smoke: smoke, Reps: reps, Queries: queries, Scenarios: scenarios}
}
