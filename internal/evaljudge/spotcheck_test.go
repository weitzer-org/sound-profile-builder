package evaljudge

import (
	"fmt"
	"strings"
	"testing"
)

var testIndex = NewIndex()

func TestIndex_ExactFuzzyAndCommonWords(t *testing.T) {
	if testIndex.Size() < 500 {
		t.Fatalf("index suspiciously small: %d", testIndex.Size())
	}
	find := func(text string) map[string]Mention {
		out := map[string]Mention{}
		for _, m := range testIndex.Find(text) {
			out[m.Name] = m
		}
		return out
	}
	// user capture, exact, case/punctuation-insensitive
	if m, ok := find("Preset B relies on a fabricated Neural Capture ('ods joe lead') and bypasses the cab")["ODS Joe Lead"]; !ok || m.Kind != "exact" || m.Records[0].Source != "user_captures.json" {
		t.Errorf("ODS Joe Lead: %+v", m)
	}
	// coros_map: a QC block name backed by several real-world entries, all shown (capped)
	m, ok := find("avoids hallucinating Axe-Fx exclusive models (like Preset A's 'Circular Delay')")["Circular Delay"]
	if !ok || len(m.Records) == 0 || m.Records[0].Source != "coros_map.json" || m.Records[0].Data["coros_equivalent"] != "Circular Delay" {
		t.Errorf("Circular Delay: %+v", m)
	}
	if len(m.Records) != 6 || m.MoreRecords != 0 {
		t.Errorf("Circular Delay has exactly 6 coros_map entries today: %+v", m)
	}
	var many []Record
	for i := 0; i < 9; i++ {
		many = append(many, Record{Key: fmt.Sprint(i)})
	}
	if om := mention(indexEntry{name: "X", records: many}, "x", "exact", 0); len(om.Records) != maxRecordsPerMention || om.MoreRecords != 3 {
		t.Errorf("overflow must be capped and counted: %+v", om)
	}
	// fuzzy: one-letter misspelling of a real block
	if m, ok := find("It uses the Dumbell ODS channel")["Dumbbell ODS"]; !ok || m.Kind != "fuzzy" || m.Distance != 1 {
		t.Errorf("fuzzy Dumbbell ODS: %+v", m)
	}
	// ordinary prose must not match
	if got := testIndex.Find("The preset has a warm tone and a sensible block order with good gain staging."); len(got) != 0 {
		t.Errorf("false positives on plain prose: %v", got)
	}
}

func TestClaimWords(t *testing.T) {
	got := ClaimWords("This is FABRICATED and avoids hallucinating Axe-Fx exclusive models")
	if strings.Join(got, ",") != "axe-fx,fabricated,hallucinating" {
		t.Errorf("%v", got)
	}
}

func agreed(n int, texts func(i int) (g, s, r string)) []Item {
	var items []Item
	for i := 0; i < n; i++ {
		g, s, r := texts(i)
		vv := Verdict{Name: fmt.Sprintf("q%02d", i), Winner: "S2", Ground: g, Struct: s, Rationale: r}
		items = append(items, Item{ID: fmt.Sprintf("S2|d1|d2|q%02d", i), Candidate: "S2", Pair: PairID{"d1", "d2", vv.Name}, Status: Agreed, Winner: "S2", A: vv, B: vv})
	}
	return items
}

func TestSample_NamedAlwaysIncludedPlusRandomTwentyPercentOfTheRest(t *testing.T) {
	items := agreed(50, func(i int) (string, string, string) {
		switch {
		case i < 10:
			return "uses the Circular Delay well", "ok", "better"
		case i < 12:
			return "claims it is fabricated", "ok", "better"
		}
		return "generic", "fine", "better"
	})
	plan := Sample(testIndex, items, 0.20, 1)
	if plan.Agreed != 50 || plan.Named != 12 || plan.RandomTarget != 10 {
		t.Fatalf("plan = %+v", plan)
	}
	why := map[string]int{}
	for _, e := range plan.Entries {
		why[e.Why]++
	}
	if why["names_block"] != 10 || why["claim_wording"] != 2 || why["random"] != 10 {
		t.Errorf("why = %v", why)
	}
	if float64(len(plan.Entries)) < 0.20*50 {
		t.Errorf("sample %d < 20%% of 50", len(plan.Entries))
	}
	// deterministic for a seed, different for another
	again := Sample(testIndex, items, 0.20, 1)
	other := Sample(testIndex, items, 0.20, 2)
	ids := func(p SamplePlan) string {
		var s []string
		for _, e := range p.Entries {
			s = append(s, e.ID)
		}
		return strings.Join(s, ",")
	}
	if ids(plan) != ids(again) || ids(plan) == ids(other) {
		t.Error("sampling must be reproducible per seed and vary across seeds")
	}
}

func TestSample_IgnoresNonAgreedAndSmallPopulations(t *testing.T) {
	items := agreed(3, func(int) (string, string, string) { return "g", "s", "r" })
	items = append(items, Item{ID: "x", Status: Disagreed}, Item{ID: "y", Status: Unpaired})
	plan := Sample(testIndex, items, 0.2, 1)
	if plan.Agreed != 3 || len(plan.Entries) != 1 { // ceil(0.6)=1
		t.Errorf("plan = %+v", plan)
	}
	if p := Sample(testIndex, nil, 0.2, 1); len(p.Entries) != 0 || p.RandomTarget != 0 {
		t.Errorf("empty: %+v", p)
	}
}

func TestApplyReview(t *testing.T) {
	items := agreed(4, func(i int) (string, string, string) { return "uses the Circular Delay", "s", "r" })
	plan := Sample(testIndex, items, 0.2, 1)
	yes, no := true, false
	plan.Entries[0].Discard = &yes
	plan.Entries[1].Discard = &no
	rv, err := ApplyReview(items, plan)
	if err != nil || rv.Discarded != 1 || rv.Reviewed != 2 || rv.Pending != 2 {
		t.Fatalf("%+v %v", rv, err)
	}
	res := Result{Candidate: "S2", Baseline: "S0", Items: items}
	if c := res.Counts(); c.Discarded != 1 || c.Agreed != 3 || c.CandWins != 3 {
		t.Errorf("discarded verdicts must leave the win tally: %+v", c)
	}
	plan.Entries[0].ID = "someone-else"
	if _, err := ApplyReview(items, plan); err == nil {
		t.Error("a foreign review entry must be an error")
	}
}
