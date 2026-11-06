package deviceexport

import (
	"fmt"
	"math"
	"strings"
	"testing"

	"github.com/weitzer-org/sound-builder/internal/catalog"
	"github.com/weitzer-org/sound-builder/internal/storage"
)

// Faders (owner decision, 2026-11-06): converted like a float on the catalog range, labelled a
// guess. These tests pin the premise (which controls are faders), the conversion, the
// refusals, the labelling and the read-back.

// TestFaderCatalogPremise pins what the decision was made on: 48 faders on 7 models, none with
// a skew, ranges -12..12 dB (Graphic-9 family, Parallax) or 0..10 (CA John's 2C). A catalog
// re-import that changes any of it must be looked at before the linear rule is trusted.
func TestFaderCatalogPremise(t *testing.T) {
	cat, err := catalog.Embedded()
	if err != nil {
		t.Fatal(err)
	}
	perModel := map[int]int{}
	total := 0
	for _, m := range cat.Models {
		for _, p := range m.Params {
			if p.Type != "fader" {
				continue
			}
			total++
			perModel[m.ID]++
			if p.Skew != nil || p.SkewSymbol != "" || p.MinSymbol != "" || p.MaxSymbol != "" {
				t.Errorf("%d %s %s: fader has a skew or symbolic bound: %+v", m.ID, m.Name, p.Name, p)
			}
			wantMin, wantMax, wantUnit := -12.0, 12.0, "dB"
			if m.ID >= 1139 && m.ID <= 1141 {
				wantMin, wantMax, wantUnit = 0, 10, ""
			}
			if p.Min == nil || p.Max == nil || *p.Min != wantMin || *p.Max != wantMax || p.Units != wantUnit {
				t.Errorf("%d %s %s: range %v..%v %q, want %g..%g %q", m.ID, m.Name, p.Name, p.Min, p.Max, p.Units, wantMin, wantMax, wantUnit)
			}
		}
	}
	want := map[int]int{4005: 9, 4006: 9, 4007: 9, 3008: 6, 1139: 5, 1140: 5, 1141: 5}
	if total != 48 || len(perModel) != len(want) {
		t.Errorf("%d faders on %d models %v, want 48 on 7", total, len(perModel), perModel)
	}
	for id, n := range want {
		if perModel[id] != n {
			t.Errorf("model %d: %d faders, want %d", id, perModel[id], n)
		}
	}
}

func faderParam(t *testing.T, e *Exporter, block *Block, label string) (Param, bool) {
	t.Helper()
	for _, p := range block.Params {
		if p.Label == label {
			return p, true
		}
	}
	return Param{}, false
}

// TestFaderConversion: min, max, default (written when the other scene differs), a mid value
// and a negative mid value, for a dB fader and a 0..10 fader.
func TestFaderConversion(t *testing.T) {
	e := testExporter(t)
	cases := []struct {
		model, param, a, b string
		wire               [2]float64
		disp               [2]float64
	}{
		// Plugin Graphic-9 65Hz: -12..12 dB, default 0, linear.
		{"Plugin Graphic-9", "65Hz", "-12 dB", "12 dB", [2]float64{0, 1}, [2]float64{-12, 12}},
		{"Plugin Graphic-9", "125Hz", "0 dB", "6 dB", [2]float64{0.5, 0.75}, [2]float64{0, 6}},
		{"Plugin Graphic-9", "1kHz", "-6", "+3", [2]float64{0.25, 0.625}, [2]float64{-6, 3}}, // bare number: the control's own unit
		{"Plugin Graphic-9", "16kHz", "0.000 dB", "-0.5dB", [2]float64{0.5, 11.5 / 24}, [2]float64{0, -0.5}},
		// CA John's 2C Ch1 80 Hz: 0..10 unitless, default 5, linear.
		{"CA John's 2C Ch1", "80 Hz", "0", "10", [2]float64{0, 1}, [2]float64{0, 10}},
		{"CA John's 2C Ch1", "240 Hz", "5", "7.5", [2]float64{0.5, 0.75}, [2]float64{5, 7.5}},
		{"CA John's 2C Ch1", "750 Hz", "2.5", "3", [2]float64{0.25, 0.3}, [2]float64{2.5, 3}},
	}
	for _, c := range cases {
		t.Run(c.model+" "+c.param+" "+c.a, func(t *testing.T) {
			// a block that uses LPF resolves Graphic-9's name to the fader model; the other
			// two names are single models.
			bp := []storage.BlockParameter{prm(c.param, c.a, c.b)}
			ex, p := export(t, e, preset(block(c.model, bp...)), "g")
			if len(ex.Unresolved) != 0 {
				t.Fatalf("unresolved %+v", ex.Unresolved)
			}
			pr, ok := faderParam(t, e, &p.Rows[0].Blocks[0], strings.ToUpper(c.param))
			if !ok {
				pr, ok = faderParam(t, e, &p.Rows[0].Blocks[0], c.param)
			}
			if !ok {
				t.Fatalf("param not written: %+v", p.Rows[0].Blocks[0].Params)
			}
			for s := 0; s < Scenes; s++ {
				i := 0 // default layout: Scene B is index 1, every other scene repeats A
				if s == 1 {
					i = 1
				}
				if math.Abs(pr.Wire[s]-c.wire[i]) > 1e-12 || math.Abs(pr.Display[s]-c.disp[i]) > 1e-12 {
					t.Errorf("scene %d: display %g wire %g, want %g / %g", s, pr.Display[s], pr.Wire[s], c.disp[i], c.wire[i])
				}
			}
			hasGuess := false
			for _, ev := range pr.Evidence {
				hasGuess = hasGuess || ev == FaderEvidence
				if strings.HasPrefix(ev, "taper:") {
					t.Errorf("a fader with no skew must not claim a taper label: %v", pr.Evidence)
				}
			}
			if !hasGuess || pr.Confirmed() {
				t.Errorf("evidence %v confirmed=%v: want %s and unconfirmed", pr.Evidence, pr.Confirmed(), FaderEvidence)
			}
			for _, s := range ex.Summary[0].Params {
				if !s.Unconfirmed {
					t.Errorf("plan row %+v not marked unconfirmed (dagger)", s)
				}
			}
		})
	}
}

// TestFaderEvidenceSkew: a fader with no skew of its own gets only the guess label; one that
// declares a skew keeps its taper label next to it (no catalog fader has one today, so this
// is built by hand), and an unmeasured bound is still refused.
func TestFaderEvidenceSkew(t *testing.T) {
	f := func(x float64) *float64 { return &x }
	base := catalog.Param{Name: "F", Type: "fader", Min: f(0), Max: f(10)}
	if ev, why := paramEvidence(base); why != "" || strings.Join(ev, ",") != FaderEvidence {
		t.Errorf("no skew: %v, %q; want just %s", ev, why, FaderEvidence)
	}
	skewed := base
	skewed.Skew = f(2)
	ev, why := paramEvidence(skewed)
	if why != "" || strings.Join(ev, ",") != FaderEvidence+",taper:device-xml" {
		t.Errorf("skew 2: %v, %q; want %s,taper:device-xml", ev, why, FaderEvidence)
	}
	unmeasured := base
	unmeasured.MinSymbol = "MIN_INPUT_TRIM"
	if _, why := paramEvidence(unmeasured); !strings.Contains(why, "MIN_INPUT_TRIM") {
		t.Errorf("an unmeasured bound must still be refused, got %q", why)
	}
	// a float keeps its taper label as before
	fl := base
	fl.Type = "float"
	if ev, _ := paramEvidence(fl); strings.Join(ev, ",") != "taper:g4" {
		t.Errorf("float: %v", ev)
	}
}

// TestFaderAtDefaultNotWritten: a fader at its catalog default in both scenes is not written,
// like any control, and carries no evidence.
func TestFaderAtDefaultNotWritten(t *testing.T) {
	e := testExporter(t)
	for _, c := range []struct{ model, param, v string }{
		{"Plugin Graphic-9", "65Hz", "0 dB"}, {"CA John's 2C Ch1", "80 Hz", "5"},
	} {
		ex, p := export(t, e, preset(block(c.model, prm(c.param, c.v, ""))), "g")
		if len(ex.Unresolved) != 0 || len(p.Rows[0].Blocks[0].Params) != 0 || len(p.Rows[0].Blocks[0].AtDefault) != 1 {
			t.Errorf("%s %s: unresolved %+v block %+v", c.model, c.param, ex.Unresolved, p.Rows[0].Blocks[0])
		}
	}
}

// TestFaderRefusals: out of range, a unit that contradicts the catalog, and non-numeric text.
// Each is exactly one unresolved item; nothing is clamped.
func TestFaderRefusals(t *testing.T) {
	e := testExporter(t)
	cases := []struct{ name, model, param, a, b, want string }{
		{"above max", "Plugin Graphic-9", "65Hz", "12.5 dB", "", "cannot convert"},
		{"below min", "Plugin Graphic-9", "65Hz", "-13", "", "cannot convert"},
		{"B out of range", "Plugin Graphic-9", "65Hz", "3 dB", "30 dB", "cannot convert"},
		{"0..10 above max", "CA John's 2C Ch1", "80 Hz", "11", "", "cannot convert"},
		{"0..10 below min", "CA John's 2C Ch1", "80 Hz", "-1", "", "cannot convert"},
		{"wrong dimension", "Plugin Graphic-9", "65Hz", "100 Hz", "", "the control is in dB"},
		{"percent on a dB fader", "Plugin Graphic-9", "65Hz", "50%", "", "the control is in dB"},
		{"dB on a 0..10 fader", "CA John's 2C Ch1", "80 Hz", "3 dB", "", "the control is unitless"},
		{"percent on a 0..10 fader", "CA John's 2C Ch1", "80 Hz", "60%", "", "the control is unitless"},
		{"text", "Plugin Graphic-9", "65Hz", "noon", "", "unparseable value"},
		{"text 0..10", "CA John's 2C Ch1", "80 Hz", "max", "", "unparseable value"},
		{"NaN", "Plugin Graphic-9", "65Hz", "NaN", "", "unparseable value"},
		{"Inf", "Plugin Graphic-9", "65Hz", "Inf", "", "unparseable value"},
		{"range", "Plugin Graphic-9", "65Hz", "2-4 dB", "", "unparseable value"},
		{"Off", "Plugin Graphic-9", "65Hz", "Off", "", "isn't a value of this control"},
		{"empty", "Plugin Graphic-9", "65Hz", "", "", "no value"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ex, _ := export(t, e, preset(block(c.model, prm(c.param, c.a, c.b))), "g")
			if len(ex.Unresolved) != 1 || !strings.Contains(ex.Unresolved[0].Reason, c.want) {
				t.Fatalf("want one unresolved item containing %q, got %+v", c.want, ex.Unresolved)
			}
		})
	}
}

// TestFaderDecisionDoesNotCoverOtherTypes: floatWithLed (Reverse Delay's TRIG THRESHOLD) and a
// combo box off its default are still refused.
func TestFaderDecisionDoesNotCoverOtherTypes(t *testing.T) {
	e := testExporter(t)
	ex, _ := export(t, e, preset(block("Reverse Delay (M)", prm("Trig Threshold", "-10 dB", ""))), "g")
	if len(ex.Unresolved) != 1 || !strings.Contains(ex.Unresolved[0].Reason, `"floatWithLed" has no measured wire mapping`) {
		t.Errorf("floatWithLed: %+v", ex.Unresolved)
	}
	ex, _ = export(t, e, preset(block("Tremolo", prm("Waveform", "Square", ""))), "g")
	if len(ex.Unresolved) != 1 || !strings.Contains(ex.Unresolved[0].Reason, "comboBox positions have no known wire mapping") {
		t.Errorf("comboBox: %+v", ex.Unresolved)
	}
}

// TestEveryFaderConvertsAtItsEnds drives every one of the catalog's faders through the exporter's own
// conversion: min -> 0, max -> 1, the midpoint -> 0.5, and one step outside refused.
func TestEveryFaderConvertsAtItsEnds(t *testing.T) {
	cat, _ := catalog.Embedded()
	n := 0
	for _, m := range cat.Models {
		for _, p := range m.Params {
			if p.Type != "fader" {
				continue
			}
			n++
			for _, c := range []struct{ d, w float64 }{{*p.Min, 0}, {*p.Max, 1}, {(*p.Min + *p.Max) / 2, 0.5}} {
				if w, err := p.ToWire(c.d); err != nil || math.Abs(w-c.w) > 1e-12 {
					t.Errorf("%d %s %s: ToWire(%g) = %g, %v; want %g", m.ID, m.Name, p.Name, c.d, w, err, c.w)
				}
				if d, err := p.ToDisplay(c.w); err != nil || math.Abs(d-c.d) > 1e-12 {
					t.Errorf("%d %s %s: ToDisplay(%g) = %g, %v; want %g", m.ID, m.Name, p.Name, c.w, d, err, c.d)
				}
			}
			if _, err := p.ToWire(*p.Max + 0.01); err == nil {
				t.Errorf("%d %s %s: out-of-range accepted", m.ID, m.Name, p.Name)
			} else if !strings.Contains(err.Error(), "outside") {
				t.Errorf("%d %s %s: want an out-of-range error, got %v", m.ID, m.Name, p.Name, err)
			}
		}
	}
	if n != 48 {
		t.Errorf("%d faders", n)
	}
}

// TestFaderRoundTrip: export a preset with faders, let the simulated agent/unit hold it, read it
// back. Values agree, and the draft's value is labelled the same way.
func TestFaderRoundTrip(t *testing.T) {
	e := importExporter(t)
	sp := preset(
		block("Plugin Graphic-9", prm("65Hz", "+6 dB", "-4.5 dB"), prm("1kHz", "-12 dB", ""), prm("LPF", "5000 Hz", "")),
		block("CA John's 2C Ch1", prm("80 Hz", "8", "2"), prm("2200 Hz", "0", "")),
	)
	ex, p := export(t, e, sp, "g")
	if len(ex.Unresolved) != 0 {
		t.Fatalf("unresolved %+v", ex.Unresolved)
	}
	for _, f32 := range []bool{false, true} {
		t.Run(fmt.Sprintf("f32=%v", f32), func(t *testing.T) {
			_, rb := simDevice(t, e, p, f32)
			res := importOf(t, e, rb)
			chain := res.Preset.Guitars[ReadbackGuitar]
			if len(chain) != 2 {
				t.Fatalf("%d blocks, unconverted %+v", len(chain), res.Unconverted)
			}
			want := map[string]map[string][2]float64{
				"Plugin Graphic-9": {"65HZ": {6, -4.5}, "1KHZ": {-12, -12}},
				"CA John's 2C Ch1": {"80HZ": {8, 2}, "2200HZ": {0, 0}},
			}
			for _, b := range chain {
				for _, bp := range b.Parameters {
					w, ok := want[b.Model][catalog.ParamNameKey(bp.Name)]
					if !ok {
						continue
					}
					a := parseDisplay(t, bp.Value)
					bb := a
					if bp.ValueB != "" {
						bb = parseDisplay(t, bp.ValueB)
					}
					if math.Abs(a-w[0]) > 1e-3 || math.Abs(bb-w[1]) > 1e-3 {
						t.Errorf("f32=%v %s %s = %q/%q, want %g/%g", f32, b.Model, bp.Name, bp.Value, bp.ValueB, w[0], w[1])
					}
					if !strings.Contains(bp.Evidence, FaderEvidence) {
						t.Errorf("%s %s: evidence %q lacks %s", b.Model, bp.Name, bp.Evidence, FaderEvidence)
					}
					delete(want[b.Model], catalog.ParamNameKey(bp.Name))
				}
			}
			for m, left := range want {
				for k := range left {
					t.Errorf("f32=%v %s %s not imported", f32, m, k)
				}
			}

		})
	}
}

// TestFaderReadbackRefusals: a wire value outside 0..1 or NaN on a fader is listed as unconverted,
// as for a float, never clamped.
func TestFaderReadbackRefusals(t *testing.T) {
	e := importExporter(t)
	for _, c := range []struct {
		name string
		w    float64
		want string
	}{{"above 1", 1.2, "outside 0..1"}, {"below 0", -0.1, "outside 0..1"}} {
		t.Run(c.name, func(t *testing.T) {
			rb := rbJSON(t, 4006, map[int][]any{1: {c.w}}, nil)
			res := importOf(t, e, rb)
			if !findUnconv(res, "65Hz", c.want) {
				t.Errorf("unconverted %+v", res.Unconverted)
			}
		})
	}
}
