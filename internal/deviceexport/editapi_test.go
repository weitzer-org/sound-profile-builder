package deviceexport

import (
	"math"
	"strconv"
	"testing"

	"github.com/weitzer-org/sound-builder/internal/catalog"
)

// The no-op tolerance (ToleranceH) must hold for every continuous control in the catalog:
//   - what the import writes (5 significant figures of a float32 wire's display value), read
//     back as the target, is the same value as the unit's;
//   - a value the Architect writes exactly, stored by the unit as a float32, is the same value;
//   - a change of a few H is a change.
func TestToleranceHoldsForEveryContinuousControl(t *testing.T) {
	e := importExporter(t)
	checked, fails := 0, 0
	for i := range e.cat.Models {
		m := &e.cat.Models[i]
		for _, lp := range catalog.LabelParams(m) {
			cp := lp.Param
			if !catalog.ContinuousParamTypes[cp.Type] {
				continue
			}
			lo, hi, err := cp.Bounds()
			if err != nil || !(hi > lo) {
				continue
			}
			if _, why := paramEvidence(cp); why != "" {
				continue
			}
			for k := 0; k <= 40; k++ {
				x := lo + (hi-lo)*float64(k)/40
				w, err := cp.ToWire(x)
				if err != nil {
					continue
				}
				w32 := float64(float32(w))
				d, err := cp.ToDisplay(w32)
				if err != nil {
					continue
				}
				checked++
				// (1) the import's text of the unit's value, read back by the target parser.
				txt := displayText(d, hi-lo, "")
				tv, perr := strconv.ParseFloat(txt, 64)
				if perr != nil {
					t.Fatal(perr)
				}
				if math.Abs(tv-d) > ToleranceH(cp, tv) {
					fails++
					t.Errorf("%s %s: import text %q vs unit %v exceeds H=%v", m.Name, lp.Label, txt, d, ToleranceH(cp, tv))
				}
				// (2) the exact value and its float32 storage.
				if math.Abs(x-d) > ToleranceH(cp, x) {
					fails++
					t.Errorf("%s %s: %v stored as %v exceeds H=%v", m.Name, lp.Label, x, d, ToleranceH(cp, x))
				}
				// (3) a few H away is a change.
				if h := ToleranceH(cp, x); x+3*h <= hi && math.Abs((x+3*h)-d) <= h {
					fails++
					t.Errorf("%s %s: %v + 3H is swallowed", m.Name, lp.Label, x)
				}
				if fails > 20 {
					t.FailNow()
				}
			}
		}
	}
	if checked < 50000 {
		t.Fatalf("only %d samples checked", checked)
	}
	t.Logf("%d samples over the catalog's continuous controls", checked)
}

func TestToleranceExamples(t *testing.T) {
	dial := catalog.Param{Type: "float", Min: ptr(0), Max: ptr(10)}
	if h := ToleranceH(dial, 4.5); h != 5e-4 {
		t.Errorf("H(4.5 on 0..10) = %v, want 5e-4", h)
	}
	if h := ToleranceH(dial, 4.6); math.Abs(4.5-4.6) <= h {
		t.Error("4.5 -> 4.6 would be swallowed")
	}
}

func ptr(x float64) *float64 { return &x }
