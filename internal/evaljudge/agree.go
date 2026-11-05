// Package evaljudge implements plan §6.2.2: two-judge agreement over pairwise verdicts, and the
// human spot-check that guards against judge factual errors.
//
// The LLM judge is not ground truth. In July it called a real capture ("ODS Joe Lead", present in
// user_captures.json) fabricated, and called a native block ("Circular Delay", mapped in
// coros_map.json) Axe-Fx-exclusive. Two judges agreeing reduces idiosyncratic noise but two
// models can share a hallucination, so agreement is necessary, not sufficient: the sampler puts
// the ground-truth record next to every verdict that names a block, for a human to rule on.
package evaljudge

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/weitzer-org/sound-builder/internal/evalcheck"
)

// Equal is the judge's "no preference" outcome.
const Equal = "Equal"

// Verdict is one row of a judge_compare result file (Go field names, no json tags, as written).
type Verdict struct {
	Name        string
	Winner      string // already mapped back to a scenario label by judge_compare
	Conf        float64
	Ground      string
	Struct      string
	Rationale   string
	Judge       string // JUDGE_MODEL requested (absent in pre-Phase-0.7 files)
	JudgeServed string // API-reported ModelVersion
	SwappedAB   bool
	// Audit fields (absent in July files).
	Preference string
	PresentedA string
	PresentedB string
	DirA       string
	DirB       string
}

// ReadFile loads one judge result file.
func ReadFile(path string) ([]Verdict, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var vs []Verdict
	if err := json.Unmarshal(b, &vs); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return vs, nil
}

// Resolve returns the verdict's winner as a scenario label. When the audit fields are present it
// RE-DERIVES the winner from the judge's raw A/B answer and the presented labels (undoing the
// position swap) and fails if that disagrees with the stored Winner. Without them it trusts Winner.
func (v Verdict) Resolve() (string, error) {
	if v.Preference == "" || v.PresentedA == "" || v.PresentedB == "" {
		return v.Winner, nil
	}
	var derived string
	switch v.Preference {
	case "A":
		derived = v.PresentedA
	case "B":
		derived = v.PresentedB
	case Equal:
		derived = Equal
	default:
		return "", fmt.Errorf("preference %q is not A/B/Equal", v.Preference)
	}
	if derived != v.Winner {
		return "", fmt.Errorf("stored winner %q contradicts preference %q with A=%q B=%q (derived %q): A/B mapping is inconsistent", v.Winner, v.Preference, v.PresentedA, v.PresentedB, derived)
	}
	return derived, nil
}

// PairID identifies one comparison. Files from judge_compare carry DirA/DirB; legacy files only
// the query name (one comparison per file).
type PairID struct{ DirA, DirB, Name string }

func (v Verdict) pairID() PairID {
	return PairID{filepath.ToSlash(filepath.Clean(orDot(v.DirA))), filepath.ToSlash(filepath.Clean(orDot(v.DirB))), v.Name}
}

func orDot(s string) string {
	if s == "" {
		return "."
	}
	return s
}

// Status of one pair after comparing both judges.
type Status string

const (
	Agreed     Status = "agreed"
	Disagreed  Status = "disagreed"
	Unusable   Status = "unusable"  // a judge's verdict is malformed / outside the label set / mapping inconsistent
	Unpaired   Status = "unpaired"  // only one judge answered this pair
	Unscoreble Status = "excluded"  // a run in the pair failed integrity / did not succeed
	Discarded  Status = "discarded" // agreed, but a human found a judge factual error
)

// Item is one pair's outcome.
type Item struct {
	ID        string  `json:"id"`
	Candidate string  `json:"candidate"`
	Pair      PairID  `json:"pair"`
	Status    Status  `json:"status"`
	Winner    string  `json:"winner,omitempty"` // when Agreed/Discarded
	WinnerA   string  `json:"winner_judge_a,omitempty"`
	WinnerB   string  `json:"winner_judge_b,omitempty"`
	Reason    string  `json:"reason,omitempty"`
	A         Verdict `json:"judge_a"`
	B         Verdict `json:"judge_b"`
}

// Options configure Compare.
type Options struct {
	Baseline, Candidate string
	// Exclude, if set, is asked about each pair's two directories: a non-empty reason drops the
	// pair (e.g. a run that failed served-model integrity must never be judged).
	Exclude func(dirA, dirB, name string) string
}

// FileMeta describes one judge file after validation.
type FileMeta struct {
	Path     string   `json:"path"`
	Judge    string   `json:"judge,omitempty"`
	Served   string   `json:"served,omitempty"`
	Rows     int      `json:"rows"`
	Warnings []string `json:"warnings,omitempty"`
	// FirstPresentedRate is the share of decisive verdicts (not Equal) whose winner was the preset
	// shown as "Preset A" -- 0.5 is no position bias. -1 when the file has no audit fields.
	FirstPresentedRate float64 `json:"first_presented_rate"`
}

// Result is the comparison of two judges on one baseline/candidate pairing.
type Result struct {
	Candidate, Baseline string
	Items               []Item
	JudgeA, JudgeB      FileMeta
}

// Counts summarises Items.
type Counts struct {
	Pairs       int `json:"pairs"`
	Agreed      int `json:"agreed"`
	Disagreed   int `json:"disagreed"`
	Unusable    int `json:"unusable"`
	Unpaired    int `json:"unpaired"`
	Excluded    int `json:"excluded_unscoreable"`
	Discarded   int `json:"discarded_by_spotcheck"`
	CandWins    int `json:"candidate_wins"`
	BaseWins    int `json:"baseline_wins"`
	AgreedEqual int `json:"agreed_equal"`
}

// Counts tallies Items. Wins/losses count only Agreed items; Discarded ones are excluded from
// them (that is what discarding means) and counted separately.
func (r Result) Counts() Counts {
	var c Counts
	for _, it := range r.Items {
		c.Pairs++
		switch it.Status {
		case Agreed:
			c.Agreed++
			switch it.Winner {
			case r.Candidate:
				c.CandWins++
			case r.Baseline:
				c.BaseWins++
			default:
				c.AgreedEqual++
			}
		case Disagreed:
			c.Disagreed++
		case Unusable:
			c.Unusable++
		case Unpaired:
			c.Unpaired++
		case Unscoreble:
			c.Excluded++
		case Discarded:
			c.Discarded++
		}
	}
	return c
}

func analyseFile(path string, vs []Verdict) FileMeta {
	m := FileMeta{Path: path, Rows: len(vs), FirstPresentedRate: -1}
	judges, served := map[string]bool{}, map[string]bool{}
	decisive, first := 0, 0
	for _, v := range vs {
		if v.Judge != "" {
			judges[v.Judge] = true
		}
		if v.JudgeServed != "" {
			served[v.JudgeServed] = true
		}
		if v.Preference == "A" || v.Preference == "B" {
			decisive++
			if v.Preference == "A" {
				first++
			}
		}
	}
	if decisive > 0 {
		m.FirstPresentedRate = float64(first) / float64(decisive)
	}
	switch len(judges) {
	case 0:
		m.Warnings = append(m.Warnings, "no judge model recorded (file predates JUDGE_MODEL): judge identity and independence are UNVERIFIED")
	case 1:
		for j := range judges {
			m.Judge = j
		}
	default:
		m.Warnings = append(m.Warnings, "file mixes several judge models")
	}
	if len(served) == 1 {
		for s := range served {
			m.Served = s
		}
	} else if len(served) > 1 {
		m.Warnings = append(m.Warnings, "file mixes several served judge model ids")
	}
	return m
}

// Compare pairs the two judges' verdicts and classifies each pair. It refuses (error) when the
// two files cannot be two independent judges of the same comparison.
func Compare(a, b []Verdict, pathA, pathB string, opt Options) (*Result, error) {
	if opt.Baseline == "" || opt.Candidate == "" || opt.Baseline == opt.Candidate {
		return nil, fmt.Errorf("baseline and candidate labels are required and must differ (got %q, %q)", opt.Baseline, opt.Candidate)
	}
	res := &Result{Candidate: opt.Candidate, Baseline: opt.Baseline, JudgeA: analyseFile(pathA, a), JudgeB: analyseFile(pathB, b)}
	if res.JudgeA.Judge != "" && res.JudgeA.Judge == res.JudgeB.Judge {
		return nil, fmt.Errorf("both files were judged by %q: that is one judge twice, not two judges", res.JudgeA.Judge)
	}
	if res.JudgeA.Served != "" && res.JudgeA.Served == res.JudgeB.Served {
		return nil, fmt.Errorf("both judges were SERVED by %q (requested %q and %q): not independent", res.JudgeA.Served, res.JudgeA.Judge, res.JudgeB.Judge)
	}
	valid := map[string]bool{opt.Baseline: true, opt.Candidate: true, Equal: true}
	index := func(vs []Verdict) (map[PairID]Verdict, error) {
		m := map[PairID]Verdict{}
		for _, v := range vs {
			id := v.pairID()
			if _, dup := m[id]; dup {
				return nil, fmt.Errorf("duplicate verdict for %s/%s/%s in one judge file", id.DirA, id.DirB, id.Name)
			}
			m[id] = v
		}
		return m, nil
	}
	ia, err := index(a)
	if err != nil {
		return nil, err
	}
	ib, err := index(b)
	if err != nil {
		return nil, err
	}
	var ids []PairID
	seen := map[PairID]bool{}
	for id := range ia {
		ids = append(ids, id)
		seen[id] = true
	}
	for id := range ib {
		if !seen[id] {
			ids = append(ids, id)
		}
	}
	sort.Slice(ids, func(i, j int) bool {
		x, y := ids[i], ids[j]
		return fmt.Sprint(x.DirA, x.DirB, x.Name) < fmt.Sprint(y.DirA, y.DirB, y.Name)
	})
	for _, id := range ids {
		va, oka := ia[id]
		vb, okb := ib[id]
		it := Item{ID: opt.Candidate + "|" + id.DirA + "|" + id.DirB + "|" + id.Name, Candidate: opt.Candidate, Pair: id, A: va, B: vb}
		switch {
		case !oka || !okb:
			it.Status, it.Reason = Unpaired, "only one judge answered this pair"
		default:
			wa, ea := va.Resolve()
			wb, eb := vb.Resolve()
			switch {
			case ea != nil:
				it.Status, it.Reason = Unusable, "judge A: "+ea.Error()
			case eb != nil:
				it.Status, it.Reason = Unusable, "judge B: "+eb.Error()
			case !valid[wa] || !valid[wb]:
				it.Status, it.Reason = Unusable, fmt.Sprintf("winner label outside {%s,%s,%s}: A=%q B=%q", opt.Baseline, opt.Candidate, Equal, wa, wb)
			default:
				it.WinnerA, it.WinnerB = wa, wb
				if wa == wb {
					it.Status, it.Winner = Agreed, wa
				} else {
					it.Status = Disagreed
				}
			}
		}
		if opt.Exclude != nil && (it.Status == Agreed || it.Status == Disagreed) {
			if why := opt.Exclude(id.DirA, id.DirB, id.Name); why != "" {
				it.Status, it.Reason, it.Winner = Unscoreble, why, ""
			}
		}
		res.Items = append(res.Items, it)
	}
	return res, nil
}

// ScenarioRep extracts ("S0","rep1")-style identity from a judge DIR like ".../S0/rep1".
func ScenarioRep(dir string) (scenario string, rep string, ok bool) {
	parts := strings.Split(filepath.ToSlash(filepath.Clean(dir)), "/")
	if len(parts) < 2 || !strings.HasPrefix(parts[len(parts)-1], "rep") {
		return "", "", false
	}
	return parts[len(parts)-2], parts[len(parts)-1], true
}

// ExcludeUnscoreable builds Options.Exclude from a batch: a pair is dropped when either side's
// run (identified by the judge DIR_A/DIR_B ending in <scenario>/rep<N>, plus the query name) is
// missing from the batch, failed, or was not served by its requested model. Directories that do
// not follow that layout cannot be mapped to a run and are not excluded.
func ExcludeUnscoreable(b *evalcheck.Batch) func(dirA, dirB, name string) string {
	return func(dirA, dirB, name string) string {
		for _, d := range []string{dirA, dirB} {
			scen, rep, ok := ScenarioRep(d)
			if !ok {
				return ""
			}
			var n int
			if _, err := fmt.Sscanf(rep, "rep%d", &n); err != nil {
				return ""
			}
			k := evalcheck.RunKey{Scenario: scen, Query: name, Rep: n}
			found := false
			for _, r := range b.Records {
				if r.Key() != k {
					continue
				}
				found = true
				if ok, why := b.Scoreable(r); !ok {
					return fmt.Sprintf("%s is not scoreable: %s", k, why)
				}
			}
			if !found {
				return fmt.Sprintf("%s is not in the batch", k)
			}
		}
		return ""
	}
}
