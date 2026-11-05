package evalscore

import (
	"strings"
	"testing"
)

func pr(q string, s int, ok bool, allHard bool, defects int, in, cached int32, cost float64) PipelineRun {
	r := PipelineRun{Query: q, Sample: s, OK: ok, In: in, Cached: cached, CostExP1USD: cost, MechDefects: defects, LatencySec: 100}
	if ok {
		hp := 0
		if allHard {
			hp = 3
		}
		r.PresetScore = &Result{HardTotal: 3, HardPass: hp, SoftTotal: 1, SoftPass: 1, AllHardPass: allHard}
		r.LibScore = &Result{HardTotal: 3, HardPass: hp, SoftTotal: 1, SoftPass: 1, AllHardPass: allHard}
	}
	return r
}

func find(rows []MetricRow, name string) MetricRow {
	for _, r := range rows {
		if r.Name == name {
			return r
		}
	}
	return MetricRow{}
}

func TestComparePipelines_PairingSignsAndFailures(t *testing.T) {
	var nw, od []PipelineRun
	for _, q := range []string{"a", "b", "c", "d", "e", "f", "g", "h"} {
		for s := 0; s < 3; s++ {
			nw = append(nw, pr(q, s, true, true, 1, 1000, 600, 0.20))
			od = append(od, pr(q, s, true, true, 3, 1000, 0, 0.30))
		}
	}
	// One failed new run and one failed old run: dropped from quality pairs, counted in 'succeeded'.
	nw = append(nw, pr("z", 0, false, false, 0, 0, 0, 0))
	od = append(od, pr("z", 0, true, true, 0, 1000, 0, 0.3))
	rows := ComparePipelines(nw, od, 0.05)

	if r := find(rows, "preset: all hard rules + single amp"); r.Verdict != NotWorse || r.N != 24 {
		t.Errorf("identical quality should be NOT_WORSE over 24 pairs: %+v", r)
	}
	// Defects: new has 1, old has 3 -> lower is better -> diff must be positive (new better).
	if r := find(rows, "mechanical defects (Architect JSON)"); r.Diff <= 0 || r.SignPos != 24 {
		t.Errorf("fewer defects should read as new-better (+): %+v", r)
	}
	// Cost lower in new -> positive diff.
	if r := find(rows, "cost excl. Phase 1 (USD)"); r.Diff <= 0 {
		t.Errorf("cheaper new should read as +: %+v", r)
	}
	// A failure is visible as a loss on 'pipeline succeeded'.
	if r := find(rows, "pipeline succeeded"); r.N != 25 || r.SignNeg != 1 {
		t.Errorf("failure accounting: %+v", r)
	}
	if !strings.Contains(PipelineMarkdown(rows, 0.05, len(nw), len(od)), "positive = new is better") {
		t.Error("report must state the sign convention")
	}
}

func TestComparePipelines_RealRegressionIsWorse(t *testing.T) {
	var nw, od []PipelineRun
	for _, q := range []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j"} {
		for s := 0; s < 3; s++ {
			nw = append(nw, pr(q, s, true, false, 0, 1, 0, 0))
			od = append(od, pr(q, s, true, true, 0, 1, 0, 0))
		}
	}
	if r := find(ComparePipelines(nw, od, 0.05), "preset: all hard rules + single amp"); r.Verdict != Worse {
		t.Errorf("want WORSE: %+v", r)
	}
}
