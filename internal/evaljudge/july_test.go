package evaljudge

import (
	"strings"
	"testing"
)

const julyDir = "../../eval_results/full_pipeline/"

// secondJudge fabricates the "other" judge for a July file: same pairs, a different model name,
// with the winners of the named queries flipped so there are real disagreements. The July files
// themselves are real judge output (both from gemini-3.1-pro-preview, before JUDGE_MODEL existed).
func secondJudge(vs []Verdict, flip map[string]bool, cand, base string) []Verdict {
	out := make([]Verdict, len(vs))
	copy(out, vs)
	for i := range out {
		out[i].Judge, out[i].JudgeServed = "second-judge", "second-judge"
		if flip[out[i].Name] {
			if out[i].Winner == base {
				out[i].Winner = cand
			} else {
				out[i].Winner = base
			}
		}
	}
	return out
}

func TestJulyFlashFile_SamplerSurfacesTheKnownJudgeHallucination(t *testing.T) {
	a, err := ReadFile(julyDir + "judge_flash_vs_baseline_v2.json")
	if err != nil || len(a) != 10 {
		t.Fatalf("%v rows=%d", err, len(a))
	}
	b := secondJudge(a, map[string]bool{"07_Edge": true, "11_Mayer_Lead": true}, "flash-3.6", "baseline")
	res, err := Compare(a, b, "a", "b", Options{Baseline: "baseline", Candidate: "flash-3.6"})
	if err != nil {
		t.Fatal(err)
	}
	c := res.Counts()
	if c.Pairs != 10 || c.Agreed != 8 || c.Disagreed != 2 {
		t.Fatalf("counts = %+v", c)
	}
	// legacy file: no judge recorded -> warning, not silence
	if len(res.JudgeA.Warnings) == 0 {
		t.Error("a July file has no judge identity: that must be warned about")
	}
	plan := Sample(NewIndex(), res.Items, 0.20, 7)
	var bona *Entry
	for i := range plan.Entries {
		if plan.Entries[i].Query == "12_Bonamassa" {
			bona = &plan.Entries[i]
		}
	}
	if bona == nil {
		t.Fatal("12_Bonamassa (judge said ODS Joe Lead is fabricated) must be in the spot-check")
	}
	var ods *Mention
	for i := range bona.Mentions {
		if bona.Mentions[i].Name == "ODS Joe Lead" {
			ods = &bona.Mentions[i]
		}
	}
	if ods == nil || ods.Records[0].Source != "user_captures.json" || !strings.Contains(ods.Records[0].Data["description"].(string), "Dumble") {
		t.Fatalf("the ground-truth record that refutes 'fabricated' must be shown next to the verdict: %+v", ods)
	}
	if !contains(bona.Claims, "fabricated") {
		t.Errorf("claim words = %v", bona.Claims)
	}
	if float64(len(plan.Entries)) < 0.20*float64(plan.Agreed) {
		t.Errorf("sample %d of %d agreed is below 20%%", len(plan.Entries), plan.Agreed)
	}
	t.Logf("flash file: %d agreed, %d sampled (%d named), %d random target", plan.Agreed, len(plan.Entries), plan.Named, plan.RandomTarget)
}

func TestJulyProFile_SamplerSurfacesCircularDelayAxeFxClaim(t *testing.T) {
	a, err := ReadFile(julyDir + "judge_pro_vs_baseline_v2.json")
	if err != nil || len(a) != 11 {
		t.Fatalf("%v rows=%d", err, len(a))
	}
	res, err := Compare(a, secondJudge(a, nil, "pro-3.6", "baseline"), "a", "b", Options{Baseline: "baseline", Candidate: "pro-3.6"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Counts().Agreed != 11 {
		t.Fatalf("identical winners must all agree: %+v", res.Counts())
	}
	plan := Sample(NewIndex(), res.Items, 0.20, 7)
	for _, e := range plan.Entries {
		if e.Query != "12_Bonamassa" {
			continue
		}
		var circ bool
		for _, m := range e.Mentions {
			if m.Name == "Circular Delay" && m.Records[0].Data["coros_equivalent"] == "Circular Delay" {
				circ = true
			}
		}
		if !circ || !contains(e.Claims, "axe-fx") {
			t.Fatalf("12_Bonamassa entry: circular=%v claims=%v", circ, e.Claims)
		}
		return
	}
	t.Fatal("12_Bonamassa (judge said Circular Delay is Axe-Fx exclusive) must be sampled")
}

func contains(ss []string, s string) bool {
	for _, x := range ss {
		if x == s {
			return true
		}
	}
	return false
}
