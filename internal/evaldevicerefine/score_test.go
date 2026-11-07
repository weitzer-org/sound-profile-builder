package evaldevicerefine

import (
	"context"
	"encoding/json"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/weitzer-org/sound-builder/internal/agents"
	"github.com/weitzer-org/sound-builder/internal/deviceexport"
	"github.com/weitzer-org/sound-builder/internal/storage"
)

const corpusDir = "../../testdata/devicejobs/readback_roundtrip"

func setup(t *testing.T) (*Scorer, []*Draft) {
	t.Helper()
	e, err := NewExporter(nil)
	if err != nil {
		t.Fatal(err)
	}
	ds, err := LoadCorpus(context.Background(), []string{corpusDir}, e)
	if err != nil {
		t.Fatal(err)
	}
	return NewScorer(e), ds
}

func output(t *testing.T, chain []storage.EffectBlock, updated bool) string {
	t.Helper()
	env := map[string]any{"conversational_response": "done", "dsp_matrix_updated": updated,
		"structured_payload": map[string]any{"guitars": map[string]any{deviceexport.ReadbackGuitar: chain}}}
	b, err := json.Marshal(env)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func clone(c []storage.EffectBlock) []storage.EffectBlock {
	out := make([]storage.EffectBlock, len(c))
	for i, b := range c {
		b.Parameters = append([]storage.BlockParameter(nil), b.Parameters...)
		out[i] = b
	}
	return out
}

var lead = regexp.MustCompile(`^(-?\d+(?:\.\d+)?)(.*)$`)

// nudge moves the first plain numeric control of the first block that has one.
func nudge(chain []storage.EffectBlock) bool {
	for i := range chain {
		for j, p := range chain[i].Parameters {
			m := lead.FindStringSubmatch(p.Value)
			if m == nil || p.Name == "Bypass" {
				continue
			}
			x, _ := strconv.ParseFloat(m[1], 64)
			if x >= 0.5 {
				x -= 0.5
			} else {
				x += 0.5
			}
			chain[i].Parameters[j].Value = strconv.FormatFloat(x, 'f', -1, 64) + m[2]
			chain[i].Parameters[j].ValueB = ""
			return true
		}
	}
	return false
}

func TestCorpusLoadsFixtures(t *testing.T) {
	_, ds := setup(t)
	if len(ds) != 6 {
		t.Fatalf("loaded %d drafts, want the 6 round-trip fixtures", len(ds))
	}
	for _, d := range ds {
		p := d.Preset()
		if p.Origin == nil || p.Origin.Kind != storage.OriginDeviceReadback || p.Origin.JobID != d.JobID {
			t.Errorf("%s: origin %+v", d.Name, p.Origin)
		}
	}
}

func TestScoreUnchangedDraftHasNoOpsAndKeepsEverything(t *testing.T) {
	s, ds := setup(t)
	for _, d := range ds {
		m := s.Score(d, Requests[0], output(t, clone(d.Chain), true))
		if !m.Valid() || m.Ops != 0 || m.NotApplied != 0 || m.Kept != len(d.Chain) || m.OutBlocks != len(d.Chain) || m.Structural != 0 || m.Locked != 0 {
			t.Errorf("%s: %+v", d.Name, m)
		}
	}
}

func TestScoreValueChangeIsAnOp(t *testing.T) {
	s, ds := setup(t)
	got := 0
	for _, d := range ds {
		c := clone(d.Chain)
		if !nudge(c) {
			continue
		}
		m := s.Score(d, Requests[0], output(t, c, true))
		if !m.Valid() {
			t.Fatalf("%s: %+v", d.Name, m)
		}
		if m.Ops > 0 && m.NotApplied == 0 {
			got++
		}
		if m.Kept != len(d.Chain) {
			t.Errorf("%s: a value change lost blocks: %+v", d.Name, m)
		}
	}
	if got == 0 {
		t.Fatal("no fixture turned a nudged value into an op")
	}
}

func TestScoreStructuralChangesAreCountedAndUnasked(t *testing.T) {
	s, ds := setup(t)
	d := ds[0]

	swap := clone(d.Chain)
	swap[0].Model = "Rage Booster"
	m := s.Score(d, Requests[3], output(t, swap, true))
	if m.Structural == 0 || m.Unasked != m.Structural || m.Kept != len(d.Chain)-1 {
		t.Errorf("swap: %+v", m)
	}

	// The same change for a request that asks for it is structural but not unasked.
	m = s.Score(d, Request{ID: "x", Message: "swap it", AsksStructural: true}, output(t, swap, true))
	if m.Structural == 0 || m.Unasked != 0 {
		t.Errorf("asked swap: %+v", m)
	}

	added := append(clone(d.Chain), storage.EffectBlock{ID: "b99", Model: "Circular Delay", Type: "Delay"})
	m = s.Score(d, Requests[0], output(t, added, true))
	if m.Structural == 0 || m.Kept != len(d.Chain) || m.OutBlocks != len(d.Chain)+1 {
		t.Errorf("add: %+v", m)
	}

	dropped := clone(d.Chain)[:len(d.Chain)-1]
	m = s.Score(d, Requests[0], output(t, dropped, true))
	if m.Removed != 1 && m.Ops == 0 { // the last block may already be bypassed everywhere
		t.Errorf("drop: %+v", m)
	}
	if m.Kept != len(d.Chain)-1 {
		t.Errorf("drop: kept %d, want %d", m.Kept, len(d.Chain)-1)
	}
}

func TestScoreLockedControlIsAViolation(t *testing.T) {
	s, ds := setup(t)
	d := ds[0]
	c := clone(d.Chain)
	c[0].Parameters = append(c[0].Parameters, storage.BlockParameter{Name: "Made Up Control", Type: "slider", Value: "5", Basis: "estimate"})
	m := s.Score(d, Requests[0], output(t, c, true))
	if m.Locked != 1 || m.CodeCounts["not_a_control"] != 1 {
		t.Errorf("%+v", m)
	}
}

func TestScoreParseAndNoChange(t *testing.T) {
	s, ds := setup(t)
	d := ds[0]
	if m := s.Score(d, Requests[0], "not json at all"); m.Parsed || m.Valid() || m.ParseError == "" {
		t.Errorf("garbage: %+v", m)
	}
	m := s.Score(d, Requests[0], "```json\n"+output(t, nil, false)+"\n```")
	if !m.Parsed || !m.NoChange || !m.Valid() || m.Kept != len(d.Chain) || m.Ops != 0 {
		t.Errorf("no change: %+v", m)
	}
	if m := s.Score(d, Requests[0], `{"dsp_matrix_updated": true, "structured_payload": {"guitars": {"A": [], "B": []}}}`); m.PlanOK || m.PlanError == "" {
		t.Errorf("two unrelated guitars should not plan: %+v", m)
	}
}

func TestPreserved(t *testing.T) {
	mk := func(s string) []storage.EffectBlock {
		var out []storage.EffectBlock
		for _, f := range strings.Fields(s) {
			id, model, _ := strings.Cut(f, ":")
			out = append(out, storage.EffectBlock{ID: id, Model: model})
		}
		return out
	}
	for _, tc := range []struct {
		draft, out string
		want       int
	}{
		{"b1:A b2:B b3:C", "b1:A b2:B b3:C", 3},
		{"b1:A b2:B b3:C", "b1:A b9:X b2:B b3:C", 3},
		{"b1:A b2:B b3:C", "b3:C b2:B b1:A", 1},
		{"b1:A b2:B b3:C", "b1:A b2:Z b3:C", 2},
		{"b1:A b2:B b3:C", "x1:A x2:B x3:C", 0},
		{"b1:A b2:B b3:C", "", 0},
	} {
		if got := preserved(mk(tc.draft), mk(tc.out)); got != tc.want {
			t.Errorf("preserved(%q, %q) = %d, want %d", tc.draft, tc.out, got, tc.want)
		}
	}
}

func TestRenderChainFormat(t *testing.T) {
	chain := []storage.EffectBlock{{ID: "b1", Model: "Green 808", Parameters: []storage.BlockParameter{
		{Name: "Gain", Value: "5.0", ValueB: "7.0"}, {Name: "Tone", Value: "6.0", ValueB: "6.0"}, {Name: "Level", Value: "4"}}}}
	want := "- b1 | Green 808 | Gain=5.0 (scene B 7.0) | Tone=6.0 | Level=4\n"
	if got := RenderChain(chain); got != want {
		t.Errorf("RenderChain = %q, want %q", got, want)
	}
}

// lookupCapture mirrors api.lookupCapture's rule: an exact name ignoring case and space, a user entry
// wins, empty never matches.
func TestLookupCapture(t *testing.T) {
	if _, _, ok := lookupCapture("   "); ok {
		t.Error("a blank name must not match")
	}
	if _, _, ok := lookupCapture("No Such Capture Anywhere"); ok {
		t.Error("an unknown name must not match")
	}
	checked, users := 0, 0
	for n, isCap := range agents.GetValidNativeBlocks() {
		if !isCap {
			continue
		}
		name, source, ok := lookupCapture("  " + strings.ToUpper(n) + " ")
		if !ok || !strings.EqualFold(name, n) {
			t.Fatalf("lookupCapture(%q) = %q, %q, %v", n, name, source, ok)
		}
		if agents.IsUserCaptureName(n) {
			users++
			if source != "user" {
				t.Errorf("%q is a user capture but came back as %q", n, source)
			}
		}
		if checked++; checked >= 200 {
			break
		}
	}
	if checked == 0 {
		t.Fatal("no captures to check")
	}
	_ = users
}
