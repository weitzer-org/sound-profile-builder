// eval_cost is the plan §5 cost gate: it reads a SMOKE batch written by
// `eval_full_pipeline -smoke`, prices every run from its recorded tokens x the repo's
// geminiPriceTable, and extrapolates the measured per-run cost to the full matrix. It makes NO
// Gemini calls.
//
//	go run ./cmd/eval_cost -root eval_results/full_pipeline_gemini_upgrade/smoke-<ts>
//
// Models without an official price are reported UNKNOWN (never $0); the total is then a LOWER
// BOUND and the output lists exactly what it leaves out.
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/weitzer-org/sound-builder/internal/evalcheck"
	"github.com/weitzer-org/sound-builder/internal/evalcost"
)

type multiFlag []string

func (m *multiFlag) String() string     { return strings.Join(*m, ",") }
func (m *multiFlag) Set(v string) error { *m = append(*m, v); return nil }

func main() {
	root := flag.String("root", "", "smoke batch directory (contains manifest.json, runs.jsonl, <scenario>/rep1/*.calls.json)")
	queries := flag.Int("full-queries", 13, "queries in the full matrix (plan §5: 13)")
	reps := flag.Int("full-reps", 2, "reps in the full matrix (plan §5: 2)")
	basisFlag := flag.String("price-basis", "requested", "price each call by the model id we REQUESTED (matches usage/ records) or the SERVED id: requested|served")
	var assume multiFlag
	flag.Var(&assume, "assume", "HYPOTHETICAL price for an unpriced model, model=inPerM/outPerM (repeatable); shown separately, labelled unverified")
	flag.Parse()
	if *root == "" {
		log.Fatal("-root is required")
	}
	basis, err := evalcost.ParseBasis(*basisFlag)
	if err != nil {
		log.Fatal(err)
	}
	hyp := map[string]evalcost.Hypothetical{}
	for _, a := range assume {
		m, h, err := evalcost.ParseAssume(a)
		if err != nil {
			log.Fatal(err)
		}
		hyp[m] = h
	}
	b, err := evalcheck.LoadBatch(*root)
	if err != nil {
		log.Fatal(err)
	}
	ex, err := evalcost.Extrapolate(b, *queries**reps, basis)
	if err != nil {
		log.Fatal(err)
	}
	evalcost.Render(os.Stdout, ex, b.Manifest.BatchID, b.Manifest.Smoke, hyp)
	fmt.Println()
}
