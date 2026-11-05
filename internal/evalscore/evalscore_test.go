package evalscore

import (
	"testing"

	"github.com/weitzer-org/sound-builder/internal/agents"
	"github.com/weitzer-org/sound-builder/internal/evalfixtures"
)

func mustScorer(t *testing.T) *Scorer {
	t.Helper()
	s, err := New()
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// The checklist must cover exactly the golden queries, or an eval silently skips one.
func TestChecklistCoversGoldenQueriesExactly(t *testing.T) {
	s := mustScorer(t)
	want := evalfixtures.GoldenQueries()
	got := map[string]bool{}
	for _, q := range s.Queries() {
		got[q] = true
		if _, ok := want[q]; !ok {
			t.Errorf("checklist has query %q that is not a golden query", q)
		}
	}
	for q := range want {
		if !got[q] {
			t.Errorf("golden query %q has no checklist", q)
		}
	}
}

// Every 'require' rule must be satisfiable by something in the real catalog / user captures,
// otherwise it is a rule that can never pass.
func TestEveryRequireRuleIsSatisfiable(t *testing.T) {
	s := mustScorer(t)
	var all []string
	for n := range agents.CorosCatalog() {
		all = append(all, n)
	}
	for n := range s.userBT {
		all = append(all, n)
	}
	for _, q := range s.Queries() {
		for _, r := range s.cl.Queries[q].Rules {
			if r.Kind != "require" {
				continue
			}
			ok := false
			for _, n := range all {
				if inScope(s.IsAmp(n), r.Scope) && s.matches(n, r.Pattern) {
					ok = true
					break
				}
			}
			if !ok {
				t.Errorf("rule %s can never pass: nothing in the catalog matches %q (scope %s)", r.ID, r.Pattern, r.Scope)
			}
		}
	}
}

func TestRangeRverbIsNeverAnAmpAndIsCounted(t *testing.T) {
	s := mustScorer(t)
	if s.IsAmp("Range Rverb") {
		t.Fatal("Range Rverb must not be classified as an amp (checklist ambiguous_names)")
	}
	r, err := s.Score("09_BB_King", []string{"US TWN Vibrato", "Range Rverb", "Green 808"}, []string{"US TWN Vibrato"})
	if err != nil {
		t.Fatal(err)
	}
	if r.RangeRverb != 1 {
		t.Errorf("G2 = %d, want 1", r.RangeRverb)
	}
	if !r.OneAmp || !r.AllHardPass {
		t.Errorf("a Twin + reverb-named-Range-Rverb should still pass: %+v", r)
	}
}

func TestScore_PassAndFail(t *testing.T) {
	s := mustScorer(t)
	good, _ := s.Score("10_Slash", []string{"Brit 2203", "Circular Delay", "Crying Wah", "Chief SD1"}, []string{"Brit 2203"})
	if !good.AllHardPass || good.SoftPass != good.SoftTotal {
		t.Errorf("expected full pass, got %+v", good)
	}
	bad, _ := s.Score("10_Slash", []string{"US TWN Vibrato"}, []string{"US TWN Vibrato"})
	if bad.AllHardPass {
		t.Error("a Twin must not satisfy the JCM800 requirement")
	}
	// 2 amps violates single_amp_mode (G3) even though the hard rules are met.
	two, _ := s.Score("10_Slash", []string{"Brit 2203", "Brit Plexi 100 Patch", "Circular Delay"}, []string{"Brit 2203", "Brit Plexi 100 Patch"})
	if two.OneAmp || two.AllHardPass {
		t.Errorf("two amps must fail G3: %+v", two)
	}
}

func TestScore_ForbidAndScope(t *testing.T) {
	s := mustScorer(t)
	// A high-gain modern amp on a clean brief fails the forbid rule.
	r, _ := s.Score("09_BB_King", []string{"US TWN Vibrato", "Bogna Uber Lead"}, nil)
	failed := false
	for _, rr := range r.Rules {
		if rr.ID == "09-no-hg" && !rr.Pass {
			failed = true
		}
	}
	if !failed {
		t.Error("09-no-hg should fail when a high-gain modern amp is chosen")
	}
	// A pedal rule must not be satisfied by an amp whose name merely contains the word.
	r2, _ := s.Score("11_Mayer_Lead", []string{"Dumbbell ODS", "65 Deluxe N Overdrive"}, nil)
	for _, rr := range r2.Rules {
		if rr.ID == "11-drive" && rr.Pass {
			t.Error("11-drive (non_amp scope) was satisfied by an amp/rig named '...Overdrive'")
		}
	}
}

func TestBigMuffPatternDoesNotMatchParametric(t *testing.T) {
	s := mustScorer(t)
	r, _ := s.Score("06_Gilmour", []string{"Watt D103 Normal", "Parametric-8", "Studio Plate 70 (ST)"}, nil)
	for _, rr := range r.Rules {
		if rr.ID == "06-big-muff" && rr.Pass {
			t.Error("'Parametric-8' must not satisfy the Big Muff rule")
		}
	}
}

func TestLibrarianNamesAndPresetModels(t *testing.T) {
	all, mand, err := LibrarianNames(`{"mandatory_blocks":["Brit 2203"],"suggested_blocks":["Green 808"],"dropped_gear":[]}`)
	if err != nil || len(all) != 2 || len(mand) != 1 {
		t.Fatalf("v2 parse: %v %v %v", all, mand, err)
	}
	all, mand, err = LibrarianNames(`{"verified_blocks":["Brit 2203","Circular Delay"],"dropped_gear":[]}`)
	if err != nil || len(all) != 2 || len(mand) != 2 {
		t.Fatalf("v1 parse: %v %v %v", all, mand, err)
	}
	if _, _, err := LibrarianNames("not json"); err == nil {
		t.Error("want a parse error for non-JSON")
	}
	m := PresetModels(`{"model":"Brit 2203"},{"model":"Green 808"},{"model":"Brit 2203"}`)
	if len(m) != 2 {
		t.Errorf("PresetModels dedupe: %v", m)
	}
}

// Regression: false negatives found in the 2026-11-05 Librarian-only A/B.
func TestScorerFalseNegativeRegressions(t *testing.T) {
	s := mustScorer(t)
	if !s.IsAmp("Watt Custom Clean") {
		t.Error("Hiwatt Custom captures (typed fx in coros_map) must count as amps")
	}
	g, _ := s.Score("06_Gilmour", []string{"Watt Custom Clean", "412 Watt S4123", "Fuzz Pi", "Studio Plate 70 (ST)"}, []string{"Watt Custom Clean"})
	if !g.AllHardPass || !g.OneAmp {
		t.Errorf("a Hiwatt Custom capture should satisfy 06-amp-hiwatt and G3: %+v", g)
	}
	m, _ := s.Score("11_Mayer_Lead", []string{"Two Rock SSS EQ1 (No Cab)", "Keeley Mod BD-2 3", "OG TS808"}, nil)
	if !m.AllHardPass {
		t.Errorf("BD-2 / TS808 drives must satisfy 11-drive: %+v", m)
	}
}

// The amp menu offers device-catalog amps that coros_map.json doesn't list; the scorer must
// still count them as amps, and native device names must not count as "not in catalog".
func TestDeviceCatalogNamesAreClassified(t *testing.T) {
	s := mustScorer(t)
	var amp string
	for _, n := range s.device.Names() {
		if _, inMap := agents.CorosCatalog()[n]; inMap {
			continue
		}
		for _, m := range s.device.Resolve(n) {
			if m.Category == "Guitar Amplifier" && m.PluginID == "" {
				amp = n
			}
		}
		if amp != "" {
			break
		}
	}
	if amp == "" {
		t.Skip("no catalog-only guitar amp in the embedded device catalog")
	}
	if !s.IsAmp(amp) {
		t.Errorf("catalog-only amp %q not classified as an amp", amp)
	}
	r, _ := s.Score("07_Edge", []string{"UK C30 TopBoost", "Spring (ST)", "Analog Delay (M)"}, nil)
	if r.NotInCatalog != 0 {
		t.Errorf("native device names must not count as unknown: G1=%d", r.NotInCatalog)
	}
}
