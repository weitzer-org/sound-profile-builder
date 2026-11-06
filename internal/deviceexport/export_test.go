package deviceexport

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/weitzer-org/sound-builder/internal/catalog"
	"github.com/weitzer-org/sound-builder/internal/devicejobs"
	"github.com/weitzer-org/sound-builder/internal/storage"
)

var update = flag.Bool("update", false, "rewrite testdata/golden/*.json from current output")

// testCaptures are the capture names the eval presets use (user_captures.json entries), so
// the goldens don't change when the capture library does.
var testCaptures = map[string]string{"1964 Fender Super Reverb": "user", "66 AC30 JMP OD": "user",
	"CA John's": "factory", "FNDR TWDLX IN Edge BAL CAB": "ir"}

func testExporter(t *testing.T) *Exporter {
	t.Helper()
	e, err := NewEmbedded(Options{
		CaptureSource: func(n string) string { return testCaptures[n] },
		OwnedPlugins:  OwnedPluginSet(true, []string{"Cory Wong"}),
	})
	if err != nil {
		t.Fatal(err)
	}
	return e
}

func loadPreset(t *testing.T, name string) storage.StructuredPreset {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "presets", name+".json"))
	if err != nil {
		t.Fatal(err)
	}
	var sp storage.StructuredPreset
	if err := json.Unmarshal(data, &sp); err != nil {
		t.Fatal(err)
	}
	return sp
}

func export(t *testing.T, e *Exporter, sp storage.StructuredPreset, guitar string) (*devicejobs.Export, Payload) {
	t.Helper()
	ex, err := e.Export(context.Background(), sp, guitar)
	if err != nil {
		t.Fatalf("Export(%s): %v", guitar, err)
	}
	if err := ex.Validate(); err != nil {
		t.Fatalf("export fails devicejobs validation: %v", err)
	}
	var p Payload
	dec := json.NewDecoder(bytes.NewReader(ex.Payload))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&p); err != nil {
		t.Fatal(err)
	}
	return ex, p
}

// goldenCases are three v6 eval presets (eval_results/v5_vs_v6_r2/v6_a/PROD/rep1, the
// Architect's structured_payload) and a synthetic preset shaped like gate G3's test layout,
// the one that can be pushed under v1's rules.
var goldenCases = []struct{ preset, guitar string }{
	{"01_SRV_Clean", "Fender Telecaster Single Coil"},
	{"03_British_Invasion", "Gibson ES-339 Humbuckers"},
	{"05_Clapton", "Gibson ES-339 Humbuckers"},
	{"pushable_g3", "Gibson ES-339 Humbuckers"},
}

// golden is what a golden file holds: the payload plus the readable plan.
type golden struct {
	Format     string                  `json:"format"`
	PlanSHA256 string                  `json:"plan_sha256"`
	Summary    []devicejobs.PlanBlock  `json:"summary"`
	Unresolved []devicejobs.Unresolved `json:"unresolved"`
	Payload    json.RawMessage         `json:"payload"`
}

func TestGoldenExports(t *testing.T) {
	e := testExporter(t)
	for _, c := range goldenCases {
		t.Run(c.preset, func(t *testing.T) {
			ex, _ := export(t, e, loadPreset(t, c.preset), c.guitar)
			sum, err := ex.PlanSHA256()
			if err != nil {
				t.Fatal(err)
			}
			got, err := json.MarshalIndent(golden{ex.Format, sum, ex.Summary, ex.Unresolved, ex.Payload}, "", " ")
			if err != nil {
				t.Fatal(err)
			}
			got = append(got, '\n')
			path := filepath.Join("testdata", "golden", c.preset+".json")
			if *update {
				if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, got, 0o644); err != nil {
					t.Fatal(err)
				}
				return
			}
			want, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("%v (run with -update to create)", err)
			}
			if !bytes.Equal(got, want) {
				t.Errorf("export differs from %s; rerun with -update and review the diff", path)
			}
		})
	}
}

func TestPushablePresetHasNothingUnresolved(t *testing.T) {
	e := testExporter(t)
	sp := loadPreset(t, "pushable_g3")
	for g := range sp.Guitars {
		ex, p := export(t, e, sp, g)
		if len(ex.Unresolved) != 0 {
			t.Fatalf("%s: unresolved %+v", g, ex.Unresolved)
		}
		if n := len(p.Rows[0].Blocks); n != 5 {
			t.Fatalf("%s: %d blocks placed, want 5", g, n)
		}
	}
}

// TestEveryIDAndIndexExists checks every export the goldens produce against the catalog:
// the id is a real model with that name, every index passes ValidParamIndex (so the
// trailing slot is never written) and is the catalog param the label names.
func TestEveryIDAndIndexExists(t *testing.T) {
	e := testExporter(t)
	cat, _ := catalog.Embedded()
	byID := map[int]*catalog.Model{}
	for i := range cat.Models {
		byID[cat.Models[i].ID] = &cat.Models[i]
	}
	for _, c := range goldenCases {
		sp := loadPreset(t, c.preset)
		for g := range sp.Guitars {
			_, p := export(t, e, sp, g)
			for _, r := range p.Rows {
				if r.Row != GridRow || r.InPort != InPort || r.OutPort != OutPort {
					t.Errorf("row %+v: want row 0, in 1, out 19", r)
				}
				for i, b := range r.Blocks {
					m, ok := byID[b.ModelID]
					if b.Capture != nil {
						m, ok = byID[captureBlockID]
						ok = ok && b.ModelID == 0 && b.Model == b.Capture.Name
					} else {
						ok = ok && m.Name == b.Model
					}
					if !ok || m.Category != b.Category {
						t.Fatalf("%s/%s: block %d model %d %q is not that catalog model", c.preset, g, i, b.ModelID, b.Model)
					}
					if b.Col < 0 || b.Col >= MaxBlocks || (i > 0 && b.Col <= r.Blocks[i-1].Col) {
						t.Errorf("%s: col %d out of order or range", b.Model, b.Col)
					}
					if m.Clones != 0 {
						if _, ok := catalog.VerifiedCloneBases[m.Clones]; !ok || b.IndexEvidence != fmt.Sprintf("clone-%d-g1", m.Clones) {
							t.Errorf("%s: clone of %d exported with evidence %q", b.Model, m.Clones, b.IndexEvidence)
						}
					}
					labels := map[int]string{}
					for _, lp := range catalog.LabelParams(m) {
						labels[lp.Param.Index] = lp.Label
					}
					for _, pr := range b.Params {
						if !m.ValidParamIndex(pr.Index) || pr.Index >= m.ParamCount {
							t.Errorf("%s: index %d invalid (param_count %d)", b.Model, pr.Index, m.ParamCount)
						}
						if labels[pr.Index] != pr.Label {
							t.Errorf("%s: index %d is %q in the catalog, exported as %q", b.Model, pr.Index, labels[pr.Index], pr.Label)
						}
						for s := 0; s < Scenes; s++ {
							if !(pr.Wire[s] >= 0 && pr.Wire[s] <= 1) {
								t.Errorf("%s %s: wire %v outside 0..1", b.Model, pr.Label, pr.Wire[s])
							}
						}
					}
				}
			}
		}
	}
}

// TestRoundTrip converts display -> wire -> display for every float parameter the export
// would write (its taper is confirmed), at min, max, default and midpoint.
func TestRoundTrip(t *testing.T) {
	cat, _ := catalog.Embedded()
	checked := 0
	for _, m := range cat.Models {
		for _, p := range m.Params {
			if p.Type != "float" || taperRefusal(p) != "" {
				continue
			}
			lo, hi, err := p.Bounds()
			if err != nil {
				continue
			}
			pts := []float64{lo, hi, (lo + hi) / 2, lo + (hi-lo)/4}
			if p.Default != nil && *p.Default >= lo && *p.Default <= hi {
				pts = append(pts, *p.Default) // a few catalog defaults lie outside their range
			}
			for _, x := range pts {
				w, err := p.ToWire(x)
				if err != nil {
					t.Fatalf("%s %s ToWire(%v): %v", m.Name, p.Name, x, err)
				}
				back, err := p.ToDisplay(w)
				if err != nil {
					t.Fatalf("%s %s ToDisplay(%v): %v", m.Name, p.Name, w, err)
				}
				if math.Abs(back-x) > 1e-9*math.Max(1, hi-lo) {
					t.Errorf("%s %s: %v -> %v -> %v", m.Name, p.Name, x, w, back)
				}
				checked++
			}
		}
	}
	if checked < 1000 {
		t.Fatalf("only %d round trips checked", checked)
	}
}

// TestG4Points pins the export's wire values to the four screen readings of gate G4.
func TestG4Points(t *testing.T) {
	e := testExporter(t)
	cases := []struct {
		model, param, value string
		wire                float64
	}{
		{"Brit 2203", "OUTPUT", "-10 dB", 0.25},
		{"110 US PRN C10R", "MIC 1 LEVEL", "6 dB", 1.0},
		{"Low-High Cut", "HPF FREQ", "217 Hz", 0.25},
		{"Brit 2203", "GAIN", "4.5", 0.45},
	}
	for _, c := range cases {
		sp := preset(block(c.model, prm(c.param, c.value, "")))
		_, p := export(t, e, sp, "g")
		if len(p.Unresolved) != 0 || len(p.Rows[0].Blocks) != 1 || len(p.Rows[0].Blocks[0].Params) != 1 {
			t.Fatalf("%s %s: %+v", c.model, c.param, p)
		}
		if w := p.Rows[0].Blocks[0].Params[0].Wire[0]; math.Abs(w-c.wire) > 0.002 {
			t.Errorf("%s %s %s: wire %v, G4 measured %v", c.model, c.param, c.value, w, c.wire)
		}
	}
}

func TestScenesAndBypass(t *testing.T) {
	e := testExporter(t)
	sp := preset(
		block("Myth Drive", prm("GAIN", "2", "8"), prm("LEVEL", "6", ""), prm("Bypass", "Off", "On")),
		block("Spring", prm("MIX", "30%", ""), prm("Bypass", "On", "")),
	)
	_, p := export(t, e, sp, "g")
	if len(p.Unresolved) != 0 {
		t.Fatalf("unresolved: %+v", p.Unresolved)
	}
	drive, spring := p.Rows[0].Blocks[0], p.Rows[0].Blocks[1]
	gain, level := drive.Params[0], drive.Params[1]
	if !gain.SceneMode || gain.Display != [Scenes]float64{2, 8, 2, 2, 2, 2, 2, 2} {
		t.Errorf("GAIN: %+v", gain)
	}
	if gain.Wire != [Scenes]float64{0.2, 0.8, 0.2, 0.2, 0.2, 0.2, 0.2, 0.2} {
		t.Errorf("GAIN wire: %v", gain.Wire)
	}
	if level.SceneMode || level.Display != [Scenes]float64{6, 6, 6, 6, 6, 6, 6, 6} {
		t.Errorf("LEVEL: %+v", level)
	}
	if drive.Bypass != [Scenes]bool{false, true, false, false, false, false, false, false} {
		t.Errorf("drive bypass: %v", drive.Bypass)
	}
	if spring.Bypass != [Scenes]bool{true, true, true, true, true, true, true, true} {
		t.Errorf("spring bypass: %v", spring.Bypass)
	}
	if spring.Model != "Spring (M)" {
		t.Errorf("family Spring resolved to %q, want the mono model", spring.Model)
	}
	// No Bypass parameter: active in all eight scenes, still written.
	_, p = export(t, e, preset(block("Myth Drive", prm("GAIN", "3", ""))), "g")
	if p.Rows[0].Blocks[0].Bypass != [Scenes]bool{} {
		t.Errorf("default bypass: %v", p.Rows[0].Blocks[0].Bypass)
	}
}

func TestValueParsing(t *testing.T) {
	e := testExporter(t)
	cases := []struct {
		model, param, value, unit string
		display                   float64
		atDefault                 bool
	}{
		{"Myth Drive", "Gain", "6.5*", "", 6.5, false},
		{"Myth Drive", "Gain", `7\*`, "", 7, false},
		{"Myth Drive", "Gain", ".5", "", 0.5, false},
		{"Myth Drive", "Gain", "5", "", 5, true}, // the default
		{"Spring", "Mix", "40 %", "", 40, false},
		{"Spring", "Mix", "40", "%", 40, false},
		{"Brit 2203", "Output", "-6dB", "", -6, false},
		{"Low-High Cut", "HPF Freq", "0.1 kHz", "", 100, false},
		{"Low-High Cut", "Low Cut", "150 Hz", "", 150, false},   // alias
		{"110 US PRN C10R", "HPF", "Off", "", 20, true},         // min label OFF, the default
		{"110 US PRN C10R", "Mic 1 Pan", "Center", "", 5, true}, // pan centre, the default
		{"Spring", "Trails", "on", "", 0, true},                 // switch at its default
	}
	for _, c := range cases {
		bp := prm(c.param, c.value, "")
		bp.Unit = c.unit
		_, p := export(t, e, preset(block(c.model, bp)), "g")
		if len(p.Unresolved) != 0 {
			t.Errorf("%s %s %q: unresolved %+v", c.model, c.param, c.value, p.Unresolved)
			continue
		}
		b := p.Rows[0].Blocks[0]
		if c.atDefault {
			if len(b.Params) != 0 || len(b.AtDefault) != 1 {
				t.Errorf("%s %s %q: want at default, got %+v", c.model, c.param, c.value, b)
			}
			continue
		}
		if len(b.Params) != 1 || b.Params[0].Display[0] != c.display {
			t.Errorf("%s %s %q: got %+v, want display %v", c.model, c.param, c.value, b.Params, c.display)
		}
	}
}

// TestRefusals covers every reason the export refuses. Each case must produce exactly one
// unresolved item containing want.
func TestRefusals(t *testing.T) {
	e := testExporter(t)
	cases := []struct {
		name  string
		sp    storage.StructuredPreset
		guitr string
		want  string
	}{
		{"unknown block", preset(block("Plate Reverb")), "", "not a block on the device"},
		{"cab IR from the library", preset(block("FNDR TWDLX IN Edge BAL CAB")), "", "cab IR"},
		{"capture annotation without a library entry", preset(block("Brit 2203 (capture)")), "", "isn't in coros_map.json or user_captures.json"},
		{"catalog capture slot", preset(block("Eltron 30")), "", "needs a specific capture"},
		{"IR loader", preset(block("Dual (M)")), "", "IR loaders are not pushed"},
		{"capture param unknown", preset(block("CA John's", prm("Presence", "+1 dB", ""))), "", "not a control of"},
		{"unowned plugin", preset(block("John Mayer Spring Reverb")), "", "needs the John Mayer plugin"},
		{"FX loop", preset(block("FX Loop 1")), "", "FX Loop blocks are not pushed"},
		{"splitter", preset(block("Splitter AB")), "", "Splitter blocks are not pushed"},
		{"side-chain", preset(block("Adaptive Gate (S/C)")), "", "side-chain"},
		{"shared name, different controls", preset(block("Graphic-9")), "", "ambiguous: 2 device models share this name with different controls"},
		{"unverified clone base", preset(block("Cory Wong The Wash")), "", "not verified on the device"},
		{"stereo cab clone", preset(block("110 US PRN C10R (ST)")), "", "not verified on the device"},
		{"global EQ", preset(block("Global EQ")), "", "never writes global settings"},
		{"input gate", preset(block("Input Gate", prm("Threshold", "-60 dB", ""))), "", "input and output block"},
		{"unknown param", preset(block("Myth Drive", prm("Bass", "5", ""))), "", "not a control of Myth Drive"},
		{"internal param", preset(block("110 US PRN C10R", prm("phi", "1", ""))), "", "internal device parameter"},
		{"written twice", preset(block("Low-High Cut", prm("HPF FREQ", "100", ""), prm("Low Cut", "120", ""))), "", "written twice"},
		{"cab blend", preset(block("110 US PRN C10R", prm("Blend", "50%", ""))), "", "no Blend control"},
		{"no value", preset(block("Myth Drive", prm("Gain", "", ""))), "", "no value"},
		{"unparseable", preset(block("Myth Drive", prm("Gain", "noon", ""))), "", "unparseable value"},
		{"range text", preset(block("Myth Drive", prm("Gain", "4-5", ""))), "", "unparseable value"},
		{"NaN", preset(block("Myth Drive", prm("Gain", "NaN", ""))), "", "unparseable value"},
		{"unit on a dial", preset(block("Myth Drive", prm("Gain", "60%", ""))), "", "the control is unitless"},
		{"wrong unit", preset(block("Spring", prm("Mix", "30 ms", ""))), "", "the control is in %"},
		{"out of range", preset(block("Myth Drive", prm("Gain", "11", ""))), "", "cannot convert"},
		{"out of range in B", preset(block("Myth Drive", prm("Gain", "5", "12"))), "", "cannot convert"},
		{"pan", preset(block("110 US PRN C10R", prm("Mic 1 Pan", "L20", ""))), "", "pan display scale"},
		{"unitless 0..1 with a unit", preset(block("Crying Clyde Wah", prm("Wah", "40%", ""))), "", "only a plain number on its 0..1 scale"},
		{"unitless 0..1 on another scale", preset(block("Crying Clyde Wah", prm("Wah", "40", ""))), "", "cannot convert"},
		{"combo box off default", preset(block("Tremolo", prm("Waveform", "Square", ""))), "", "comboBox positions have no known wire mapping"},
		{"fader", preset(block("Plugin Graphic-9", prm("65Hz", "3 dB", ""))), "", "type \"fader\" has no measured wire mapping"},
		{"switch bad name", preset(block("Spring", prm("Trails", "Maybe", ""))), "", "isn't a position"},
		{"unnamed switch named", preset(block("Jewel", prm("High Cut", "On", ""))), "", "isn't a position"},
		{"pyquadcortex bound, out of range", preset(block("Parametric-8", prm("1 Gain", "15 dB", ""))), "", "cannot convert"},
		{"bad bypass", preset(block("Myth Drive", prm("Bypass", "Engaged", ""))), "", "Bypass must be On"},
		{"legacy bypass", legacy(block("Myth Drive", prm("Bypass", "On", ""))), "", "predates the rule"},
		{"nine blocks", preset(nine()...), "", "the chain has 9 grid blocks"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			g := c.guitr
			if g == "" {
				g = "g"
			}
			ex, _ := export(t, e, c.sp, g)
			if len(ex.Unresolved) != 1 {
				t.Fatalf("want 1 unresolved item, got %+v", ex.Unresolved)
			}
			if !strings.Contains(ex.Unresolved[0].Reason, c.want) {
				t.Errorf("reason %q does not contain %q", ex.Unresolved[0].Reason, c.want)
			}
		})
	}
}

// TestLogSkewLabelled finds the real LOG_SKEW parameters and checks they convert with the
// pyquadcortex label, though their stored skew (0.3) equals a measured numeric skew.
func TestLogSkewLabelled(t *testing.T) {
	cat, _ := catalog.Embedded()
	found := 0
	for _, m := range cat.Models {
		for _, p := range m.Params {
			if p.SkewSymbol != "LOG_SKEW" {
				continue
			}
			found++
			ev, why := taperEvidence(p)
			if why != "" || len(ev) == 0 || ev[0] != "taper:pyquadcortex" {
				t.Errorf("%s %s: evidence %v, refusal %q", m.Name, p.Name, ev, why)
			}
		}
	}
	if found == 0 {
		t.Fatal("no LOG_SKEW parameter in the catalog; the importer lost skew_symbol")
	}
}

// TestEvidenceLabels pins what each kind of mapping is labelled with (plan 4.1).
func TestEvidenceLabels(t *testing.T) {
	e := testExporter(t)
	cases := []struct {
		model, param, value string
		want                []string
	}{
		{"Myth Drive", "Gain", "3", []string{"taper:g4"}},
		{"Brit 2203", "Output", "-6 dB", []string{"taper:g4"}},
		{"110 US PRN C10R", "Mic 1 Level", "-3 dB", []string{"taper:g4", "bounds:g4"}},
		{"Slapback Delay", "Delay Time", "120 ms", []string{"taper:device-xml"}},
		{"110 US PRN C10R", "Output Volume", "-3 dB", []string{"taper:device-xml"}},
		{"Parametric-8", "1 Gain", "3 dB", []string{"taper:g4", "bounds:pyquadcortex-4.0.1"}},
		{"Spring", "Trails", "Off", []string{"encoding:qc-mcp-linear"}},
	}
	for _, c := range cases {
		ex, p := export(t, e, preset(block(c.model, prm(c.param, c.value, ""))), "g")
		if len(ex.Unresolved) != 0 || len(p.Rows[0].Blocks[0].Params) != 1 {
			t.Errorf("%s %s: %+v", c.model, c.param, ex.Unresolved)
			continue
		}
		pr := p.Rows[0].Blocks[0].Params[0]
		if strings.Join(pr.Evidence, ",") != strings.Join(c.want, ",") {
			t.Errorf("%s %s: evidence %v, want %v", c.model, c.param, pr.Evidence, c.want)
		}
		confirmed := true
		for _, w := range c.want {
			confirmed = confirmed && strings.HasSuffix(w, ":g4")
		}
		if ex.Summary[0].Params[0].Unconfirmed == confirmed {
			t.Errorf("%s %s: plan Unconfirmed = %v", c.model, c.param, ex.Summary[0].Params[0].Unconfirmed)
		}
	}
}

// TestSwitchEncoding checks qc-mcp's linear position encoding, wire = (value-min)/(max-min).
func TestSwitchEncoding(t *testing.T) {
	e := testExporter(t)
	cases := []struct {
		model, param, a, b string
		wire               [2]float64
		names              [2]string
	}{
		{"Spring", "Trails", "Off", "On", [2]float64{0, 1}, [2]string{"Off", "On"}},
		{"Low-High Cut", "HPF Slope", "-6", "-48", [2]float64{1.0 / 8, 1}, [2]string{"-6", "-48"}},
		{"Jewel", "High Cut", "1", "", [2]float64{1, 1}, [2]string{}},
	}
	for _, c := range cases {
		ex, p := export(t, e, preset(block(c.model, prm(c.param, c.a, c.b))), "g")
		if len(ex.Unresolved) != 0 {
			t.Fatalf("%s %s: %+v", c.model, c.param, ex.Unresolved)
		}
		pr := p.Rows[0].Blocks[0].Params[0]
		if pr.Wire[0] != c.wire[0] || pr.Wire[1] != c.wire[1] || pr.Wire[2] != c.wire[0] {
			t.Errorf("%s %s: wire %v, want A %v B %v", c.model, c.param, pr.Wire, c.wire[0], c.wire[1])
		}
		if c.names[0] != "" && (ex.Summary[0].Params[0].A != c.names[0] || ex.Summary[0].Params[0].B != c.names[1]) {
			t.Errorf("%s %s: plan shows %+v", c.model, c.param, ex.Summary[0].Params[0])
		}
	}
}

// TestDefaultMics: a cab's Mic 1/Mic 2 equal to the slot's default IR name is at-default.
func TestDefaultMics(t *testing.T) {
	e := testExporter(t)
	sp := preset(block("110 US PRN C10R",
		prm("Mic 1", "Dyn 57", ""), prm("Mic 2", "ribbon 160 - off axis", "Ribbon 160"), prm("Mic 1 Level", "-3 dB", "")))
	ex, p := export(t, e, sp, "g")
	if len(ex.Unresolved) != 0 {
		t.Fatalf("unresolved: %+v", ex.Unresolved)
	}
	b := p.Rows[0].Blocks[0]
	if strings.Join(b.AtDefault, ",") != "MIC 1,MIC 2" || len(b.Params) != 1 {
		t.Fatalf("block: %+v", b)
	}
}

// TestCaptureBlock: a library capture exports as a Neural Capture block with no model id,
// the capture's name and source, and its five controls on the capture block's layout.
func TestCaptureBlock(t *testing.T) {
	e := testExporter(t)
	sp := preset(block("CA John's", prm("Gain", "0 dB", "+3 dB"), prm("Treble", "-1.5 dB", ""), prm("Bass", "0", ""), prm("Bypass", "Off", "")),
		block("66 AC30 JMP OD (capture)", prm("Volume", "+1 dB", "")))
	ex, p := export(t, e, sp, "g")
	if len(ex.Unresolved) != 0 {
		t.Fatalf("unresolved: %+v", ex.Unresolved)
	}
	john, ac30 := p.Rows[0].Blocks[0], p.Rows[0].Blocks[1]
	if john.ModelID != 0 || john.Capture == nil || *john.Capture != (Capture{Name: "CA John's", Source: "factory"}) ||
		john.Category != "Neural Capture" || john.IndexEvidence != "capture-block" {
		t.Fatalf("capture block: %+v", john)
	}
	if len(john.Params) != 2 || john.Params[0].Label != "GAIN" || john.Params[0].Index != 0 ||
		john.Params[1].Label != "TREBLE" || john.Params[1].Index != 3 || !john.Params[0].SceneMode {
		t.Fatalf("capture params: %+v", john.Params)
	}
	if w := john.Params[0].Wire; w[0] != 0.5 || w[1] != 27.0/48 {
		t.Errorf("GAIN wire %v, want 0.5 and %v (linear -24..24 dB)", w, 27.0/48)
	}
	if strings.Join(john.AtDefault, ",") != "BASS" {
		t.Errorf("at default: %v", john.AtDefault)
	}
	if ac30.Capture == nil || ac30.Capture.Name != "66 AC30 JMP OD" || ac30.Capture.Source != "user" {
		t.Fatalf("annotated capture: %+v", ac30.Capture)
	}
	if ex.Summary[0].Capture != "CA John's" || ex.Summary[0].ModelID != 0 {
		t.Errorf("plan block: %+v", ex.Summary[0])
	}
}

// TestCaptureBlockLayouts: the V1 (14000) and V2 (14001) capture blocks share the controls
// a capture block exports, so the agent can pick either by the capture's version.
func TestCaptureBlockLayouts(t *testing.T) {
	cat, _ := catalog.Embedded()
	var v1, v2 *catalog.Model
	for i := range cat.Models {
		switch cat.Models[i].ID {
		case 14000:
			v1 = &cat.Models[i]
		case 14001:
			v2 = &cat.Models[i]
		}
	}
	if v1 == nil || v2 == nil {
		t.Fatal("capture blocks missing from the catalog")
	}
	a, _ := json.Marshal(v1.Params)
	b, _ := json.Marshal(v2.Params)
	if !bytes.Equal(a, b) || v1.ParamCount != v2.ParamCount {
		t.Fatalf("V1 and V2 capture blocks differ:\n%s\n%s", a, b)
	}
}

func TestUnmeasuredBoundRefused(t *testing.T) {
	lo, hi := "MIN_INPUT_TRIM", "MAX_INPUT_TRIM"
	p := catalog.Param{Name: "TRIM", Type: "float", MinSymbol: lo, MaxSymbol: hi, Units: "dB"}
	if why := taperRefusal(p); !strings.Contains(why, "unmeasured") {
		t.Errorf("taperRefusal = %q", why)
	}
}

func TestInternalModelsRefused(t *testing.T) {
	b := &builder{e: testExporter(t)}
	for _, m := range []catalog.Model{
		{ID: 20000, Name: "NC_Recorder", Category: "Neural Capture Internal"},
		{ID: 20001, Name: "NC_Trainer", Category: "Guitar Amplifier"},
		{ID: 99999, Name: "Lane Output", Category: "Lane Internal"},
		{ID: 99998, Name: "Old", Category: "Utility_Deprecated"},
	} {
		if why := b.modelRefusal(&m); !strings.Contains(why, "internal device model") {
			t.Errorf("%s: %q", m.Name, why)
		}
	}
	cat, _ := catalog.Embedded()
	for _, m := range cat.Models {
		if internalModelIDs[m.ID] || strings.Contains(m.Category, "Internal") {
			t.Errorf("catalog holds internal model %d %s; the importer should drop it", m.ID, m.Name)
		}
	}
}

func TestOwnedPluginAllowed(t *testing.T) {
	e := testExporter(t) // owns Cory Wong
	cat, _ := catalog.Embedded()
	var name string
	for _, m := range cat.Models {
		if p, _ := m.RequiredPlugin(); p == "Cory Wong" && m.Category == "Compressor" && len(cat.Resolve(m.Name)) == 1 {
			name = m.Name
			break
		}
	}
	if name == "" {
		t.Skip("no Cory Wong compressor in the catalog")
	}
	_, p := export(t, e, preset(block(name)), "g")
	if len(p.Unresolved) != 0 || p.Rows[0].Blocks[0].Plugin != "Cory Wong" {
		t.Fatalf("%s: %+v", name, p)
	}
	none, _ := NewEmbedded(Options{OwnedPlugins: OwnedPluginSet(false, []string{"Cory Wong"})})
	ex, _ := export(t, none, preset(block(name)), "g")
	if len(ex.Unresolved) != 1 || !strings.Contains(ex.Unresolved[0].Reason, "Cory Wong") {
		t.Fatalf("allow_paid_plugins=false: %+v", ex.Unresolved)
	}
}

func TestUnknownGuitarIsAnError(t *testing.T) {
	if _, err := testExporter(t).Export(context.Background(), preset(block("Myth Drive")), "nope"); err == nil {
		t.Fatal("want an error")
	}
}

func TestDeterministic(t *testing.T) {
	e := testExporter(t)
	sp := loadPreset(t, "05_Clapton")
	first, _ := export(t, e, sp, "Gibson ES-339 Humbuckers")
	for i := 0; i < 20; i++ {
		again, _ := export(t, e, sp, "Gibson ES-339 Humbuckers")
		if !bytes.Equal(first.Payload, again.Payload) {
			t.Fatal("payload bytes differ between runs")
		}
	}
}

// TestWorstCaseSize builds the largest export v1 can make (eight blocks with the most
// parameters, every one scene-varying) and checks it fits devicejobs' limit.
func TestWorstCaseSize(t *testing.T) {
	cat, _ := catalog.Embedded()
	b := &builder{e: testExporter(t)}
	type cand struct {
		name string
		n    int
	}
	var cs []cand
	for i := range cat.Models {
		m := &cat.Models[i]
		if b.modelRefusal(m) != "" || len(cat.Resolve(m.Name)) != 1 {
			continue
		}
		cs = append(cs, cand{m.Name, len(m.Params)})
	}
	sort.Slice(cs, func(i, j int) bool { return cs[i].n > cs[j].n })
	p := Payload{Format: Format, Rows: []Row{{Blocks: []Block{}}}, Unresolved: []devicejobs.Unresolved{}}
	for i := 0; i < MaxBlocks; i++ {
		m, _ := cat.Lookup(cs[i].name)
		bl := Block{Col: i, ModelID: m.ID, Model: m.Name, Category: m.Category, IndexEvidence: indexEvidence(m), AtDefault: []string{}}
		for _, lp := range catalog.LabelParams(m) {
			pr := Param{Index: lp.Param.Index, Label: lp.Label, Units: lp.Param.Units, SceneMode: true}
			for s := range pr.Wire {
				pr.Display[s], pr.Wire[s] = -12345.6789012345, 0.123456789012345
			}
			bl.Params = append(bl.Params, pr)
		}
		p.Rows[0].Blocks = append(p.Rows[0].Blocks, bl)
	}
	raw, err := marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(raw) > devicejobs.MaxExportBytes/2 {
		t.Fatalf("worst case is %d bytes, more than half of the %d limit", len(raw), devicejobs.MaxExportBytes)
	}
	t.Logf("worst case %d bytes", len(raw))
}

func TestCatalogHash(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "catalog", "qc_catalog.json"))
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(data)
	want := hex.EncodeToString(sum[:])
	if got := catalog.EmbeddedSHA256(); got != want {
		t.Fatalf("EmbeddedSHA256 = %s, file hash %s", got, want)
	}
	_, p := export(t, testExporter(t), preset(block("Myth Drive")), "g")
	if p.Catalog.SHA256 != want || p.Catalog.CorOS != "4.1" || !strings.Contains(p.Catalog.Origin, "f634ecd") {
		t.Fatalf("catalog header %+v", p.Catalog)
	}
}

// ---- helpers ----

func prm(name, a, b string) storage.BlockParameter {
	return storage.BlockParameter{Name: name, Value: a, ValueB: b, Basis: "estimate"}
}

func block(model string, ps ...storage.BlockParameter) storage.EffectBlock {
	if ps == nil {
		ps = []storage.BlockParameter{}
	}
	return storage.EffectBlock{Model: model, Parameters: ps}
}

func preset(bs ...storage.EffectBlock) storage.StructuredPreset {
	// A basis on one parameter marks the preset as v4+ (see usesBasis), even when the
	// blocks under test carry no parameters.
	marker := storage.EffectBlock{Model: "Myth Drive", Parameters: []storage.BlockParameter{prm("Gain", "5", "")}}
	return storage.StructuredPreset{Guitars: map[string][]storage.EffectBlock{"g": bs, "basis-marker": {marker}}}
}

func legacy(bs ...storage.EffectBlock) storage.StructuredPreset {
	for i := range bs {
		for j := range bs[i].Parameters {
			bs[i].Parameters[j].Basis = ""
		}
	}
	return storage.StructuredPreset{Guitars: map[string][]storage.EffectBlock{"g": bs}}
}

func nine() []storage.EffectBlock {
	out := make([]storage.EffectBlock, 9)
	for i := range out {
		out[i] = block("Myth Drive")
	}
	return out
}

// taperRefusal is why taperEvidence refuses a parameter, or "".
func taperRefusal(p catalog.Param) string {
	_, why := taperEvidence(p)
	return why
}

// TestNonDefaultMicIsAWarning: a mic other than the cab's default is left out with a warning
// (owner decision, plan 4.1). The push isn't blocked and the mic isn't written.
func TestNonDefaultMicIsAWarning(t *testing.T) {
	e := testExporter(t)
	sp := preset(block("110 US PRN C10R",
		prm("Mic 1", "Dyn 57", "Condenser 414"), prm("Mic 2", "Ribbon 121", ""), prm("Mic 1 Level", "-3 dB", "")))
	ex, p := export(t, e, sp, "g")
	if len(ex.Unresolved) != 0 {
		t.Fatalf("unresolved: %+v", ex.Unresolved)
	}
	if len(ex.Warnings) != 2 || len(p.Warnings) != 2 {
		t.Fatalf("warnings: %+v", ex.Warnings)
	}
	for i, want := range []string{`"Condenser 414" can't be written yet`, `"Ribbon 121" can't be written yet`} {
		if !strings.Contains(ex.Warnings[i].Reason, want) {
			t.Errorf("warning %d %q lacks %q", i, ex.Warnings[i].Reason, want)
		}
	}
	if !strings.Contains(ex.Warnings[1].Reason, "keeps its default Ribbon 160") {
		t.Errorf("warning doesn't name the default: %q", ex.Warnings[1].Reason)
	}
	b := p.Rows[0].Blocks[0]
	if len(b.AtDefault) != 0 || len(b.Params) != 1 || b.Params[0].Label != "MIC 1 LEVEL" {
		t.Fatalf("block: %+v", b)
	}
}

// TestSharedNameSameControls: models that share a name and have identical controls resolve
// to the lowest id (owner decision); G1 read a hand-placed Green 808 back as id 5.
func TestSharedNameSameControls(t *testing.T) {
	ex, p := export(t, testExporter(t), preset(block("Green 808", prm("Overdrive", "6", ""))), "g")
	if len(ex.Unresolved) != 0 || p.Rows[0].Blocks[0].ModelID != 5 || p.Rows[0].Blocks[0].Model != "Green 808" {
		t.Fatalf("unresolved %+v, block %+v", ex.Unresolved, p.Rows[0].Blocks)
	}
}

// TestUnitless01: a plain number on a unitless 0..1 control is taken on that scale and
// labelled, so the dry run marks it unconfirmed.
func TestUnitless01(t *testing.T) {
	ex, p := export(t, testExporter(t), preset(block("Crying Clyde Wah", prm("Wah", "0.4", "0.75"))), "g")
	if len(ex.Unresolved) != 0 {
		t.Fatalf("unresolved %+v", ex.Unresolved)
	}
	pr := p.Rows[0].Blocks[0].Params[0]
	if pr.Wire[0] != 0.4 || pr.Wire[1] != 0.75 || strings.Join(pr.Evidence, ",") != "taper:g4,scale:catalog-range" {
		t.Fatalf("param %+v", pr)
	}
	if !ex.Summary[0].Params[0].Unconfirmed {
		t.Errorf("not marked unconfirmed in the plan")
	}
}
