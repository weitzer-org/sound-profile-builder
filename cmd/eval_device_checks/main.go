// eval_device_checks re-scores saved pipeline outputs with the production post-generation
// checks (agents.RunMechanicalQualityChecks, which ends with the device-catalog parameter
// check), so two arms of an eval are scored by the same checker regardless of which build
// produced them. It makes NO Gemini calls.
//
//	go run ./cmd/eval_device_checks -arm v5=eval_results/v5_vs_v6/v5_a,eval_results/v5_vs_v6/v5_b \
//	    -arm v6=eval_results/v5_vs_v6/v6_a,eval_results/v5_vs_v6/v6_b [-json out.json]
//
// Each arm directory is an eval_full_pipeline output (runs.jsonl plus <scenario>/rep<N>/<query>.html,
// JSON envelopes despite the extension). Runs that failed or failed the served-model integrity
// check are left out. Averages are reported per arm over its own parsed runs, and again over the
// (query, rep) pairs that parsed in every arm, which is the comparison to read.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/weitzer-org/sound-builder/internal/agents"
	"github.com/weitzer-org/sound-builder/internal/evalcheck"
)

type armFlag []string

func (a *armFlag) String() string     { return strings.Join(*a, " ") }
func (a *armFlag) Set(v string) error { *a = append(*a, v); return nil }

// result is one scored output.
type result struct {
	Arm    string                         `json:"arm"`
	Query  string                         `json:"query"`
	Rep    int                            `json:"rep"`
	Path   string                         `json:"path"`
	Blocks int                            `json:"blocks"`
	Report agents.MechanicalQualityReport `json:"report"`
}

func (r result) key() string { return fmt.Sprintf("%s#%d", r.Query, r.Rep) }

// columns are the reported metrics, in table order.
var columns = []struct {
	name string
	get  func(r result) float64
}{
	{"Blocks", func(r result) float64 { return float64(r.Blocks) }},
	{"Defects (all checks)", func(r result) float64 { return float64(r.Report.TotalDefects()) }},
	{"Device defects", func(r result) float64 { return float64(r.Report.Catalog.Defects()) }},
	{"Device defects / checked block", func(r result) float64 {
		if r.Report.Catalog.CheckedBlocks == 0 {
			return 0
		}
		return float64(r.Report.Catalog.Defects()) / float64(r.Report.Catalog.CheckedBlocks)
	}},
	{"  unknown param names", func(r result) float64 { return float64(r.Report.Catalog.UnknownNames) }},
	{"  out of range", func(r result) float64 { return float64(r.Report.Catalog.OutOfRange) }},
	{"  bad switch values", func(r result) float64 { return float64(r.Report.Catalog.BadSwitchValues) }},
	{"  unit mismatches", func(r result) float64 { return float64(r.Report.Catalog.UnitMismatches) }},
	{"  confirmed_range overclaims", func(r result) float64 { return float64(r.Report.Catalog.BasisOverclaims) }},
	{"  invented block names", func(r result) float64 { return float64(r.Report.Catalog.UnrecognizedBlocks) }},
	{"  cab Blend (old convention)", func(r result) float64 { return float64(r.Report.Catalog.CabConvention) }},
	{"  unconsidered controls (kept_defaults output only)", func(r result) float64 { return float64(r.Report.Catalog.UnconsideredParams) }},
	{"Checked blocks", func(r result) float64 { return float64(r.Report.Catalog.CheckedBlocks) }},
	{"Unverified blocks", func(r result) float64 { return float64(r.Report.UnverifiedBlocks) }},
	{"Capture formatting mismatches", func(r result) float64 { return float64(r.Report.CaptureFormattingMismatches) }},
	{"Incomplete cabinets", func(r result) float64 { return float64(r.Report.IncompleteCabinetBlocks) }},
	{"Leftover value ranges", func(r result) float64 { return float64(r.Report.LeftoverValueRanges) }},
	{"Critic issues (baked into the output)", func(r result) float64 { return float64(r.Report.CriticIssues) }},
}

func main() {
	var arms armFlag
	flag.Var(&arms, "arm", "name=dir[,dir...] (repeatable)")
	jsonOut := flag.String("json", "", "optional path for every per-file result")
	flag.Parse()
	if len(arms) < 1 {
		log.Fatal("give at least one -arm name=dir[,dir...]")
	}
	valid := agents.BuildEffectiveValidBlocks(true, true)

	var names []string
	byArm := map[string][]result{}
	for _, a := range arms {
		name, dirs, ok := strings.Cut(a, "=")
		if !ok || name == "" || dirs == "" {
			log.Fatalf("bad -arm %q: want name=dir[,dir...]", a)
		}
		names = append(names, name)
		for _, dir := range strings.Split(dirs, ",") {
			recs, err := evalcheck.ReadRunRecords(filepath.Join(dir, "runs.jsonl"))
			if err != nil {
				log.Fatalf("%s: %v", dir, err)
			}
			for _, rec := range recs {
				if !rec.Success || !rec.IntegrityOK || rec.OutputFile == "" {
					continue
				}
				raw, err := os.ReadFile(rec.OutputFile)
				if err != nil {
					log.Fatalf("%s: %v", rec.OutputFile, err)
				}
				rep := agents.RunMechanicalQualityChecks(string(raw), valid)
				res := result{Arm: name, Query: rec.Query, Rep: rec.Rep, Path: rec.OutputFile, Report: rep}
				res.Blocks = countBlocks(string(raw))
				byArm[name] = append(byArm[name], res)
			}
		}
	}

	// Pairs: (query, rep) parsed in every arm.
	parsed := func(r result) bool { return r.Report.ParseError == "" }
	inAll := map[string]int{}
	for _, name := range names {
		for _, r := range byArm[name] {
			if parsed(r) {
				inAll[r.key()]++
			}
		}
	}
	paired := func(r result) bool { return parsed(r) && inAll[r.key()] == len(names) }
	nPairs := 0
	for _, n := range inAll {
		if n == len(names) {
			nPairs++
		}
	}

	fmt.Print("# Device and mechanical checks, re-scored with one checker\n\n")
	fmt.Print("| Arm | Scored runs (success, integrity ok) | Parsed |\n|---|--:|--:|\n")
	for _, name := range names {
		n := 0
		for _, r := range byArm[name] {
			if parsed(r) {
				n++
			}
		}
		fmt.Printf("| %s | %d | %d |\n", name, len(byArm[name]), n)
	}
	fmt.Printf("\n## Means over the %d (query, rep) pairs parsed in every arm\n\n", nPairs)
	printTable(names, byArm, paired)
	fmt.Println("\n## Per-query paired difference in device defects (last arm minus first, mean over reps)")
	printPerQuery(names, byArm, paired)
	fmt.Print("\n## Means over each arm's own parsed runs\n\n")
	printTable(names, byArm, parsed)

	if *jsonOut != "" {
		var all []result
		for _, name := range names {
			all = append(all, byArm[name]...)
		}
		b, _ := json.MarshalIndent(all, "", "  ")
		if err := os.WriteFile(*jsonOut, b, 0o644); err != nil {
			log.Fatal(err)
		}
	}
}

func printTable(names []string, byArm map[string][]result, keep func(result) bool) {
	fmt.Print("| Metric (mean per preset) |")
	for _, name := range names {
		fmt.Printf(" %s |", name)
	}
	fmt.Print("\n|---|")
	for range names {
		fmt.Print("--:|")
	}
	fmt.Println()
	for _, c := range columns {
		fmt.Printf("| %s |", c.name)
		for _, name := range names {
			sum, n := 0.0, 0
			for _, r := range byArm[name] {
				if keep(r) {
					sum += c.get(r)
					n++
				}
			}
			if n == 0 {
				fmt.Print(" - |")
			} else {
				fmt.Printf(" %.2f |", sum/float64(n))
			}
		}
		fmt.Println()
	}
}

func printPerQuery(names []string, byArm map[string][]result, keep func(result) bool) {
	if len(names) < 2 {
		return
	}
	first, last := names[0], names[len(names)-1]
	mean := func(arm, q string) (float64, bool) {
		sum, n := 0.0, 0
		for _, r := range byArm[arm] {
			if r.Query == q && keep(r) {
				sum += float64(r.Report.Catalog.Defects())
				n++
			}
		}
		return sum / float64(max(n, 1)), n > 0
	}
	qs := map[string]bool{}
	for _, r := range byArm[first] {
		qs[r.Query] = true
	}
	var sorted []string
	for q := range qs {
		sorted = append(sorted, q)
	}
	sort.Strings(sorted)
	fmt.Printf("\n| Query | %s | %s | diff |\n|---|--:|--:|--:|\n", first, last)
	for _, q := range sorted {
		a, okA := mean(first, q)
		b, okB := mean(last, q)
		if okA && okB {
			fmt.Printf("| %s | %.1f | %.1f | %+.1f |\n", q, a, b, b-a)
		}
	}
}

// countBlocks counts the blocks across guitars in an output envelope (0 if it doesn't parse).
func countBlocks(raw string) int {
	var env struct {
		StructuredPayload *struct {
			Guitars map[string][]json.RawMessage `json:"guitars"`
		} `json:"structured_payload"`
	}
	if json.Unmarshal([]byte(agents.StripJSONFences(raw)), &env) != nil || env.StructuredPayload == nil {
		return 0
	}
	n := 0
	for _, bl := range env.StructuredPayload.Guitars {
		n += len(bl)
	}
	return n
}
