// eval_report is the Phase 2 report generator (plan §6.1 gates, §6.2-6.3, §7 decision rules). It
// reads ONE results directory written by `eval_full_pipeline` and prints, per scenario against S0
// IN THE SAME BATCH, the measurements, the PASS/FAIL of every hard gate with the numbers behind
// it, and the mechanical application of the §7 rules. It makes NO Gemini calls.
//
//	go run ./cmd/eval_report -root eval_results/full_pipeline_gemini_upgrade/<ts>
//
// Optional inputs it picks up from the same directory: recritic.jsonl (cmd/eval_recritic) and
// judge_summary.json (cmd/eval_judge_agree). Anything missing is reported UNMEASURED and keeps
// the gates and rules that need it from passing.
//
// It refuses to compare across batches (mixed batch ids, or -baseline-root from another run).
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"os"
	"sort"
	"strings"

	"github.com/weitzer-org/sound-builder/internal/agents"
	"github.com/weitzer-org/sound-builder/internal/evalcheck"
	"github.com/weitzer-org/sound-builder/internal/evalcost"
	"github.com/weitzer-org/sound-builder/internal/evalreport"
)

func main() {
	root := flag.String("root", "", "results directory (manifest.json, runs.jsonl, <scenario>/rep<N>/...)")
	legacy := flag.Bool("legacy", false, "root is a pre-manifest directory (the July eval_results/full_pipeline layout); most gates are then UNMEASURED")
	baseID := flag.String("baseline-id", "S0", "baseline scenario id (legacy July layout: baseline)")
	other := flag.String("baseline-root", "", "a SECOND results directory to take the baseline from: refused unless it is the same batch (it never is across invocations)")
	deprecated := flag.String("baseline-flash-deprecated", "unknown", "yes|no|unknown: whether S0's Flash model is deprecated, for the §7 Flash rule (plan §10: unverified, so the default is unknown)")
	prov := flag.String("provisional-models", "gemini-4-argon", "comma-separated model ids §7 forbids adopting while provisional/unpriced")
	basisFlag := flag.String("price-basis", "requested", "requested|served model id prices each call")
	judge := flag.String("judge-summary", "", "judge_summary.json (default: <root>/judge_summary.json if present)")
	jsonOut := flag.String("json", "", "also write the full report as JSON")
	sample := flag.Int("critic-sample", 0, "print N sampled pipeline-Critic flags per Critic-tier scenario (and S0) for the §7 manual accuracy check")
	seed := flag.Int64("seed", 1, "seed for -critic-sample")
	flag.Parse()
	if *root == "" {
		log.Fatal("-root is required")
	}
	dep, err := evalreport.ParseTri(*deprecated)
	if err != nil {
		log.Fatal(err)
	}
	basis, err := evalcost.ParseBasis(*basisFlag)
	if err != nil {
		log.Fatal(err)
	}
	pm := map[string]bool{}
	for _, m := range strings.Split(*prov, ",") {
		if m = strings.TrimSpace(m); m != "" {
			pm[m] = true
		}
	}
	rep, err := evalreport.Build(evalreport.Options{Root: *root, Legacy: *legacy, BaselineID: *baseID, OtherRoot: *other,
		BaselineFlashDeprecated: dep, Provisional: pm, Basis: basis, JudgeSummaryPath: *judge})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	rep.Render(os.Stdout)
	if *jsonOut != "" {
		b, err := json.MarshalIndent(rep, "", "  ")
		if err != nil {
			log.Fatal(err)
		}
		if err := os.WriteFile(*jsonOut, b, 0o644); err != nil {
			log.Fatal(err)
		}
	}
	if *sample > 0 {
		criticSample(rep, *sample, *seed)
	}
}

func criticSample(rep *evalreport.Report, n int, seed int64) {
	valid := agents.BuildEffectiveValidBlocks(true, true)
	b, err := evalcheck.LoadBatch(rep.Root)
	if rep.Legacy {
		b, err = evalcheck.LoadLegacy(rep.Root)
	}
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("\nCRITIC ACCURACY SAMPLE (§7): label each flag TRUE or FALSE contradiction against the ground truth shown; the Critic is not ground truth\n")
	for _, s := range rep.Scenarios {
		if s.Skipped != "" {
			continue
		}
		isCritic := s.ID == rep.BaselineID
		for _, t := range s.Tiers {
			if t == "critic" {
				isCritic = true
			}
		}
		if !isCritic {
			continue
		}
		type item struct {
			key  string
			note evalcheck.CriticNote
		}
		var all []item
		for _, r := range b.Records {
			if r.Scenario != s.ID {
				continue
			}
			if ok, _ := b.Scoreable(r); !ok {
				continue
			}
			raw, err := os.ReadFile(b.OutputFile(r.Key()))
			if err != nil {
				continue
			}
			for _, cn := range evalcheck.CriticNotes(string(raw), valid) {
				all = append(all, item{r.Key().String(), cn})
			}
		}
		sort.Slice(all, func(i, j int) bool {
			a, b := all[i], all[j]
			return a.key+"\x00"+a.note.Guitar+"\x00"+a.note.BlockID+"\x00"+a.note.Note < b.key+"\x00"+b.note.Guitar+"\x00"+b.note.BlockID+"\x00"+b.note.Note
		})
		rand.New(rand.NewSource(seed)).Shuffle(len(all), func(i, j int) { all[i], all[j] = all[j], all[i] })
		fmt.Printf("\n  %s: %d flags in total, sampling %d\n", s.ID, len(all), min(n, len(all)))
		for i := 0; i < len(all) && i < n; i++ {
			c := all[i].note
			truth := "block not in the maps"
			if c.KnownBlock {
				truth = fmt.Sprintf("coros_map/user_captures: is_capture=%v", c.MapSaysCapture)
			}
			fmt.Printf("    %2d. %s %s/%s %q [%s; %s]\n        %s\n        verdict: ____\n", i+1, all[i].key, c.Guitar, c.BlockID, c.Model, c.Class, truth, c.Note)
		}
	}
}
