package catalog

import (
	"math"
	"strings"
	"testing"
)

func f(v float64) *float64 { return &v }

func mustEmbedded(t *testing.T) *Catalog {
	t.Helper()
	c, err := Embedded()
	if err != nil {
		t.Fatal(err)
	}
	return c
}

// paramByName returns the first (lowest-index) param of a model with this name.
func paramByName(t *testing.T, c *Catalog, model, name string) Param {
	t.Helper()
	m, ok := c.Lookup(model)
	if !ok {
		t.Fatalf("model %q missing", model)
	}
	p := findParam(m, name)
	if p == nil {
		t.Fatalf("%s: param %q missing", model, name)
	}
	return *p
}

const (
	ampModel = "Brit 2203"           // amp OUTPUT, GAIN, MASTER (G4)
	cabModel = "110 US PRN C10R (M)" // cab cloning 12000, read back in G1
	hpfModel = "Low-High Cut"        // HPF FREQ (G4)
	screenDP = 0.05                  // the unit's screen shows one decimal
)

// G4 screen readings, wire -> display, on the owner's unit (CorOS 4.1.0). These are the
// hard vectors: each was read off the unit's screen, not derived from the formula.
func TestToDisplayGoldenG4ScreenReadings(t *testing.T) {
	c := mustEmbedded(t)
	cases := []struct {
		name string
		p    Param
		wire float64
		want float64
		tol  float64
	}{
		{"amp OUTPUT 0.25 -> -10.0 dB", paramByName(t, c, ampModel, "OUTPUT"), 0.25, -10.0, screenDP},
		{"cab LEVEL 0.01 -> -21.8 dB", paramByName(t, c, cabModel, "LEVEL"), 0.01, -21.8, screenDP},
		{"cab LEVEL 1.0 -> +6.0 dB", paramByName(t, c, cabModel, "LEVEL"), 1.0, 6.0, screenDP},
		{"HPF FREQ 0.25 -> 217 Hz", paramByName(t, c, hpfModel, "HPF FREQ"), 0.25, 217, 0.5},
		{"GAIN 0.45 -> 4.5 (linear dial)", paramByName(t, c, ampModel, "GAIN"), 0.45, 4.5, 1e-9},
		{"MASTER 0.3 -> 3.0 (linear dial)", paramByName(t, c, ampModel, "MASTER"), 0.3, 3.0, 1e-9},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := tc.p.ToDisplay(tc.wire)
			if err != nil {
				t.Fatal(err)
			}
			if math.Abs(got-tc.want) > tc.tol {
				t.Errorf("ToDisplay(%v) = %v, want %v +- %v", tc.wire, got, tc.want, tc.tol)
			}
		})
	}
}

// The catalog parameters used above must carry the skews G4 exercised, so the vectors
// test the imported data and not only the formula.
func TestG4ParamsCarryTheMeasuredSkew(t *testing.T) {
	c := mustEmbedded(t)
	for _, tc := range []struct {
		model, name string
		skew        *float64
	}{
		{ampModel, "OUTPUT", f(3.8018)},
		{cabModel, "LEVEL", f(4.9594844)},
		{hpfModel, "HPF FREQ", f(0.3)},
		{ampModel, "GAIN", nil},
		{ampModel, "MASTER", nil},
	} {
		p := paramByName(t, c, tc.model, tc.name)
		switch {
		case tc.skew == nil && p.Skew != nil:
			t.Errorf("%s %s: skew = %v, want none (linear)", tc.model, tc.name, *p.Skew)
		case tc.skew != nil && (p.Skew == nil || *p.Skew != *tc.skew):
			t.Errorf("%s %s: skew = %v, want %v", tc.model, tc.name, p.Skew, *tc.skew)
		}
	}
}

// INFERRED, NOT SCREEN-READ. These stored wire values (0.5 for amp OUTPUT and cab LEVEL,
// 0.5595 for cab OUTPUT VOLUME) were read back from a preset in plan 7.2 and equal the
// formula's output for 0 dB, ASSUMING those params sat at their XML default of 0 dB. The
// screen was not read for them. A failure here means the formula or constants drifted from
// that inference; it does not by itself say anything about the unit.
func TestToWireZeroDBInferredFromStoredValues(t *testing.T) {
	c := mustEmbedded(t)
	for _, tc := range []struct {
		model, name string
		want        float64
	}{
		{ampModel, "OUTPUT", 0.5},
		{cabModel, "LEVEL", 0.5},
		{cabModel, "OUTPUT VOLUME", 0.5595},
	} {
		p := paramByName(t, c, tc.model, tc.name)
		got, err := p.ToWire(0)
		if err != nil {
			t.Fatalf("%s %s: %v", tc.model, tc.name, err)
		}
		if math.Abs(got-tc.want) > 5e-5 {
			t.Errorf("%s %s: ToWire(0 dB) = %.5f, want %.4f (inferred)", tc.model, tc.name, got, tc.want)
		}
	}
}

func TestToWireToDisplayRoundTrip(t *testing.T) {
	c := mustEmbedded(t)
	ps := []Param{
		paramByName(t, c, ampModel, "OUTPUT"),
		paramByName(t, c, cabModel, "LEVEL"),
		paramByName(t, c, hpfModel, "HPF FREQ"),
		paramByName(t, c, ampModel, "GAIN"),
	}
	for _, p := range ps {
		for _, w := range []float64{0, 0.01, 0.25, 0.5, 0.9, 1} {
			d, err := p.ToDisplay(w)
			if err != nil {
				t.Fatalf("%s ToDisplay(%v): %v", p.Name, w, err)
			}
			back, err := p.ToWire(d)
			if err != nil {
				t.Fatalf("%s ToWire(%v): %v", p.Name, d, err)
			}
			if math.Abs(back-w) > 1e-9 {
				t.Errorf("%s: wire %v -> display %v -> wire %v", p.Name, w, d, back)
			}
		}
	}
}

func TestToWireEndpoints(t *testing.T) {
	p := Param{Name: "X", Type: "float", Min: f(-40), Max: f(6), Skew: f(4.9594844)}
	if w, err := p.ToWire(-40); err != nil || w != 0 {
		t.Errorf("ToWire(min) = %v, %v; want 0", w, err)
	}
	if w, err := p.ToWire(6); err != nil || w != 1 {
		t.Errorf("ToWire(max) = %v, %v; want 1", w, err)
	}
}

func TestConversionsRefuse(t *testing.T) {
	ok := Param{Name: "X", Type: "float", Min: f(0), Max: f(10)}
	sym := func(min, max string) Param { return Param{Name: "S", Type: "float", MinSymbol: min, MaxSymbol: max} }
	bad := func(name string, p Param, wire, display float64, want string) {
		t.Helper()
		t.Run(name, func(t *testing.T) {
			if v, err := p.ToWire(display); err == nil || !strings.Contains(err.Error(), want) {
				t.Errorf("ToWire(%v) = %v, %v; want an error containing %q", display, v, err, want)
			}
			if v, err := p.ToDisplay(wire); err == nil || !strings.Contains(err.Error(), want) {
				t.Errorf("ToDisplay(%v) = %v, %v; want an error containing %q", wire, v, err, want)
			}
		})
	}
	// Unmeasured / unknown bounds refuse in both directions, whatever the value.
	bad("unmeasured min", sym("MIN_INPUT_TRIM", "MAX_INPUT_TRIM"), 0.5, 1, "unmeasured")
	bad("unmeasured max", Param{Name: "S", Type: "float", Min: f(0), MaxSymbol: "MAX_INPUT_TRIM"}, 0.5, 1, "unmeasured")
	bad("unknown symbol", sym("MIN_FOO", "MAX_FOO"), 0.5, 1, "unknown")
	bad("absent bound", Param{Name: "A", Type: "float", Min: f(0)}, 0.5, 1, "absent")
	bad("inverted range", Param{Name: "I", Type: "float", Min: f(5), Max: f(1)}, 0.5, 2, "empty or inverted")
	bad("empty range", Param{Name: "E", Type: "float", Min: f(5), Max: f(5)}, 0.5, 5, "empty or inverted")
	bad("switch type", Param{Name: "W", Type: "switch", Min: f(0), Max: f(1), Steps: 2}, 0.5, 1, "no measured wire mapping")
	bad("floatWithLed type", Param{Name: "L", Type: "floatWithLed", Min: f(0), Max: f(1)}, 0.5, 1, "no measured wire mapping")
	bad("combo type", Param{Name: "C", Type: "comboBox", Min: f(0), Max: f(3)}, 0.5, 1, "no measured wire mapping")
	bad("zero skew", Param{Name: "Z", Type: "float", Min: f(0), Max: f(10), Skew: f(0)}, 0.5, 1, "skew")
	bad("negative skew", Param{Name: "Z", Type: "float", Min: f(0), Max: f(10), Skew: f(-1)}, 0.5, 1, "skew")
	bad("NaN skew", Param{Name: "Z", Type: "float", Min: f(0), Max: f(10), Skew: f(math.NaN())}, 0.5, 1, "skew")
	bad("NaN bound", Param{Name: "N", Type: "float", Min: f(math.NaN()), Max: f(10)}, 0.5, 1, "not finite")

	// Value refusals on an otherwise good parameter. Never clamped.
	for _, d := range []float64{-0.0001, 10.0001, 100, math.NaN(), math.Inf(1), math.Inf(-1)} {
		if v, err := ok.ToWire(d); err == nil {
			t.Errorf("ToWire(%v) = %v, want a refusal", d, v)
		}
	}
	for _, w := range []float64{-0.0001, 1.0001, 2, math.NaN(), math.Inf(1), math.Inf(-1)} {
		if v, err := ok.ToDisplay(w); err == nil {
			t.Errorf("ToDisplay(%v) = %v, want a refusal", w, v)
		}
	}
}

func TestBoundsResolveSymbols(t *testing.T) {
	c := mustEmbedded(t)
	p := paramByName(t, c, cabModel, "LEVEL")
	if p.Min != nil || p.MinSymbol != "MIN_CABSIM_DB" {
		t.Fatalf("cab LEVEL should carry the symbol, got %+v", p)
	}
	lo, hi, err := p.Bounds()
	if err != nil || lo != -40 || hi != 6 {
		t.Errorf("cab LEVEL Bounds = %v..%v, %v; want -40..6", lo, hi, err)
	}
}

// Evidence honesty: only the cab bounds were read on the owner's 4.1.0 unit.
func TestFirmwareBoundEvidence(t *testing.T) {
	confirmed := map[string]bool{"MIN_CABSIM_DB": true, "MAX_CABSIM_DB": true}
	for name, b := range FirmwareBounds {
		want := EvidencePyQuadCortex
		if confirmed[name] {
			want = EvidenceConfirmed410
		}
		if b.Evidence != want {
			t.Errorf("%s evidence = %q, want %q", name, b.Evidence, want)
		}
	}
	for name := range confirmed {
		if _, ok := FirmwareBounds[name]; !ok {
			t.Errorf("%s missing", name)
		}
	}
}

// Every symbolic bound in the shipped catalog is either resolved or explicitly listed as
// unmeasured, never in both, so a CorOS update that adds a symbol fails here instead of
// silently refusing (or worse, guessing).
func TestEverySymbolicBoundResolvedOrUnmeasured(t *testing.T) {
	c := mustEmbedded(t)
	seen := map[string]bool{}
	for _, m := range c.Models {
		for _, p := range m.Params {
			for _, s := range []string{p.MinSymbol, p.MaxSymbol} {
				if s != "" {
					seen[s] = true
				}
			}
		}
	}
	if len(seen) == 0 {
		t.Fatal("no symbolic bounds found; the test would pass vacuously")
	}
	for s := range seen {
		_, resolved := FirmwareBounds[s]
		_, unmeasured := UnmeasuredBounds[s]
		if resolved == unmeasured {
			t.Errorf("symbol %s: resolved=%v unmeasured=%v; want exactly one", s, resolved, unmeasured)
		}
	}
	for s := range FirmwareBounds {
		if _, both := UnmeasuredBounds[s]; both {
			t.Errorf("%s is in both tables", s)
		}
	}
}

// A fader converts like a float (owner decision, 2026-11-06; deviceexport labels it a guess):
// linear on its range when it has no skew, with the skew when it has one, and refuses what a float refuses.
func TestFaderConvertsLikeAFloat(t *testing.T) {
	db := Param{Name: "65Hz", Type: "fader", Min: f(-12), Max: f(12), Default: f(0), Units: "dB"}
	for _, c := range []struct{ d, w float64 }{{-12, 0}, {-6, 0.25}, {0, 0.5}, {6, 0.75}, {12, 1}} {
		if w, err := db.ToWire(c.d); err != nil || math.Abs(w-c.w) > 1e-12 {
			t.Errorf("ToWire(%g) = %g, %v; want %g", c.d, w, err, c.w)
		}
		if d, err := db.ToDisplay(c.w); err != nil || math.Abs(d-c.d) > 1e-12 {
			t.Errorf("ToDisplay(%g) = %g, %v; want %g", c.w, d, err, c.d)
		}
	}
	for _, d := range []float64{-12.0001, 12.0001, math.NaN(), math.Inf(1)} {
		if v, err := db.ToWire(d); err == nil {
			t.Errorf("ToWire(%v) = %v, want a refusal (never clamped)", d, v)
		} else if !strings.Contains(err.Error(), "outside") && !strings.Contains(err.Error(), "finite") {
			t.Errorf("ToWire(%v): want an out-of-range or not-finite error, got %v", d, err)
		}
	}
	// A fader that did carry a skew would use it, as qc-mcp's to_norm does for any parameter.
	sk := Param{Name: "F", Type: "fader", Min: f(0), Max: f(10), Skew: f(2)}
	if w, err := sk.ToWire(5); err != nil || math.Abs(w-0.25) > 1e-12 {
		t.Errorf("skewed fader ToWire(5) = %g, %v; want 0.25", w, err)
	}
	if d, err := sk.ToDisplay(0.25); err != nil || math.Abs(d-5) > 1e-12 {
		t.Errorf("skewed fader ToDisplay(0.25) = %g, %v; want 5", d, err)
	}
}
