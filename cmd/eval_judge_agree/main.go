// eval_judge_agree is plan §6.2.2: count only the pairwise verdicts on which TWO judge models
// agree (after mapping each judge's A/B back to scenario labels), report disagreements
// separately, and produce the human spot-check of the agreed ones. It makes NO Gemini calls: it
// reads result files written by cmd/judge_compare.
//
//	# 1. judge each pair twice (two JUDGE_MODELs; DIR_A/DIR_B point at <batch>/<scenario>/rep<N>):
//	JUDGE_MODEL=gemini-3.1-pro-preview DIR_A=B/S0/rep1 DIR_B=B/S2/rep1 LABEL_A=S0 LABEL_B=S2 OUT_FILE=j/S2_rep1_pro.json  go run ./cmd/judge_compare
//	JUDGE_MODEL=gemini-4-argon         DIR_A=B/S0/rep1 DIR_B=B/S2/rep1 LABEL_A=S0 LABEL_B=S2 OUT_FILE=j/S2_rep1_argon.json go run ./cmd/judge_compare
//	# 2. agree + sample:
//	go run ./cmd/eval_judge_agree -batch B -out j/ -pair S2=j/S2_rep1_pro.json,j/S2_rep1_argon.json ...
//	# 3. a human fills "discard" in j/spotcheck.json, then re-run with -review j/spotcheck.json
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/weitzer-org/sound-builder/internal/evalcheck"
	"github.com/weitzer-org/sound-builder/internal/evaljudge"
)

type multiFlag []string

func (m *multiFlag) String() string     { return strings.Join(*m, ";") }
func (m *multiFlag) Set(v string) error { *m = append(*m, v); return nil }

func main() {
	var pairs multiFlag
	flag.Var(&pairs, "pair", "CANDIDATE=judgeA.json,judgeB.json (repeatable; repeat a candidate for several reps)")
	baseline := flag.String("baseline-label", "S0", "the baseline's label in the judge files (LABEL_A/LABEL_B used with judge_compare)")
	batchDir := flag.String("batch", "", "optional results directory: drop pairs whose run failed or was not integrity-clean (needs DIR_A/DIR_B ending in <scenario>/rep<N>)")
	out := flag.String("out", "", "output directory for agreement.json, spotcheck.json, spotcheck.md, judge_summary.json")
	rate := flag.Float64("rate", 0.20, "random spot-check share of agreed verdicts (plan: >= 20%)")
	seed := flag.Int64("seed", 1, "sampling seed (the same seed gives the same sample)")
	review := flag.String("review", "", "a spotcheck.json a human has filled in; its discards are applied")
	flag.Parse()
	if len(pairs) == 0 || *out == "" {
		log.Fatal("need at least one -pair and -out")
	}
	if *rate < 0.2 {
		log.Fatal("-rate below 0.20 would violate plan §6.2.2")
	}
	var batch *evalcheck.Batch
	if *batchDir != "" {
		var err error
		if batch, err = evalcheck.LoadBatch(*batchDir); err != nil {
			log.Fatal(err)
		}
		if len(batch.Problems) > 0 {
			log.Fatalf("batch has structural problems: %s", strings.Join(batch.Problems, "; "))
		}
	}
	var exclude func(dirA, dirB, name string) string
	if batch != nil {
		exclude = evaljudge.ExcludeUnscoreable(batch)
	}

	byCand := map[string]*evaljudge.Result{}
	var order []string
	for _, p := range pairs {
		cand, files, ok := strings.Cut(p, "=")
		fa, fb, ok2 := strings.Cut(files, ",")
		if !ok || !ok2 || cand == "" {
			log.Fatalf("-pair %q: want CANDIDATE=judgeA.json,judgeB.json", p)
		}
		a, err := evaljudge.ReadFile(fa)
		if err != nil {
			log.Fatal(err)
		}
		b, err := evaljudge.ReadFile(fb)
		if err != nil {
			log.Fatal(err)
		}
		res, err := evaljudge.Compare(a, b, fa, fb, evaljudge.Options{Baseline: *baseline, Candidate: cand, Exclude: exclude})
		if err != nil {
			log.Fatalf("%s: %v", p, err)
		}
		if prev, ok := byCand[cand]; ok {
			prev.Items = append(prev.Items, res.Items...)
		} else {
			byCand[cand] = res
			order = append(order, cand)
		}
	}
	var results []*evaljudge.Result
	var all []evaljudge.Item
	for _, c := range order {
		results = append(results, byCand[c])
	}
	// Sample over a pointer-stable copy, then apply the review back to the originals.
	for _, r := range results {
		all = append(all, r.Items...)
	}

	ix := evaljudge.NewIndex()
	plan := evaljudge.Sample(ix, all, *rate, *seed)
	if *review != "" {
		raw, err := os.ReadFile(*review)
		if err != nil {
			log.Fatal(err)
		}
		var reviewed evaljudge.SamplePlan
		if err := json.Unmarshal(raw, &reviewed); err != nil {
			log.Fatalf("%s: %v", *review, err)
		}
		if reviewed.Seed != plan.Seed || len(reviewed.Entries) != len(plan.Entries) {
			log.Fatalf("%s was made from a different sample (seed %d, %d entries; this run: seed %d, %d entries): re-run with the same -seed and the same inputs", *review, reviewed.Seed, len(reviewed.Entries), plan.Seed, len(plan.Entries))
		}
		plan = reviewed
	}
	var rv evaljudge.Review
	{
		var err error
		flat := make([]evaljudge.Item, 0, len(all))
		for _, r := range results {
			flat = append(flat, r.Items...)
		}
		if rv, err = evaljudge.ApplyReview(flat, plan); err != nil {
			log.Fatal(err)
		}
		i := 0
		for _, r := range results { // copy statuses back (ApplyReview mutated the flat copy)
			for j := range r.Items {
				r.Items[j] = flat[i]
				i++
			}
		}
	}
	var contestants func(string) map[string]bool
	if batch != nil && batch.Manifest != nil {
		contestants = func(cand string) map[string]bool {
			set := map[string]bool{}
			for _, id := range []string{*baseline, cand} {
				if sc, ok := batch.Manifest.Scenario(id); ok {
					for _, m := range sc.AgentModels {
						set[m] = true
						if p, ok := sc.Pins[m]; ok {
							set[p] = true
						}
					}
				}
			}
			return set
		}
	}
	summary := evaljudge.Summarize(*baseline, results, plan, rv, contestants)
	if batch != nil && batch.Manifest != nil {
		summary.BatchID = batch.Manifest.BatchID
	}

	if err := os.MkdirAll(*out, 0o755); err != nil {
		log.Fatal(err)
	}
	write := func(name string, v any) {
		b, err := json.MarshalIndent(v, "", "  ")
		if err != nil {
			log.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(*out, name), b, 0o644); err != nil {
			log.Fatal(err)
		}
	}
	write("agreement.json", map[string]any{"baseline": *baseline, "results": results})
	write(evaljudge.SummaryFile, summary)
	if *review == "" {
		write("spotcheck.json", plan)
		if err := os.WriteFile(filepath.Join(*out, "spotcheck.md"), []byte(evaljudge.Worksheet(plan)), 0o644); err != nil {
			log.Fatal(err)
		}
	}

	fmt.Printf("=== two-judge agreement (baseline %s) ===\n", *baseline)
	for _, c := range summary.Candidates {
		k := c.Counts
		fmt.Printf("%-4s pairs=%d agreed=%d (cand wins %d, base wins %d, tie %d) DISAGREED=%d unpaired=%d unusable=%d excluded(unscoreable run)=%d discarded(spot-check)=%d\n",
			c.Candidate, k.Pairs, k.Agreed, k.CandWins, k.BaseWins, k.AgreedEqual, k.Disagreed, k.Unpaired, k.Unusable, k.Excluded, k.Discarded)
		if len(c.ContestantJudges) > 0 {
			fmt.Printf("     NOTE: judge(s) %s are also routed into %s or %s (self-preference risk; plan §6.2.2 only requires the SECOND judge to be a non-contestant)\n", strings.Join(c.ContestantJudges, ", "), *baseline, c.Candidate)
		}
		for _, m := range []evaljudge.FileMeta{c.JudgeA, c.JudgeB} {
			fmt.Printf("     judge %-26s served %-26s rows=%d first-presented win rate=%s\n", orNone(m.Judge), orNone(m.Served), m.Rows, rate2(m.FirstPresentedRate))
			for _, w := range m.Warnings {
				fmt.Printf("       WARNING: %s\n", w)
			}
		}
	}
	status := "PENDING"
	if *review != "" {
		status = fmt.Sprintf("%d reviewed, %d still pending, %d discarded", rv.Reviewed, rv.Pending, rv.Discarded)
	}
	fmt.Printf("\nSpot-check: %d of %d agreed verdicts sampled (%d name a block/capture or claim a fact). Review status: %s.\n", len(plan.Entries), plan.Agreed, plan.Named, status)
	if !summary.ReviewComplete {
		fmt.Println("The tallies above are PROVISIONAL until every sampled verdict has a human decision (set \"discard\" in spotcheck.json, re-run with -review).")
	}
	fmt.Printf("Wrote %s/{agreement.json,%s,spotcheck.json,spotcheck.md}\n", *out, evaljudge.SummaryFile)
}

func orNone(s string) string {
	if s == "" {
		return "(not recorded)"
	}
	return s
}

func rate2(r float64) string {
	if r < 0 {
		return "n/a"
	}
	return fmt.Sprintf("%.2f", r)
}
