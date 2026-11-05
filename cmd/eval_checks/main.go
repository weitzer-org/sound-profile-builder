// eval_checks runs the deterministic checker (internal/evalcheck) over a directory of pipeline
// outputs and prints per-scenario totals. It makes NO Gemini calls.
//
//	go run ./cmd/eval_checks -root eval_results/full_pipeline \
//	    -report eval_results/full_pipeline/full_pipeline_gemini36_benchmark_report.md
//
// Layout understood: <root>/<scenario>/[rep<N>/]<query>.html  (the files are JSON envelopes,
// despite the extension -- see RunPipeline / injectRenderedHTML).
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

type fileReport struct {
	Scenario string           `json:"scenario"`
	Query    string           `json:"query"`
	Path     string           `json:"path"`
	Report   evalcheck.Report `json:"report"`
}

func main() {
	root := flag.String("root", "eval_results/full_pipeline", "directory of <scenario>/<query>.html outputs")
	report := flag.String("report", "", "optional July-format markdown benchmark report, to count failed/truncated runs")
	runs := flag.String("runs", "", "optional runs.jsonl written by cmd/eval_full_pipeline")
	jsonOut := flag.String("json", "", "optional path for the full per-file JSON")
	verbose := flag.Bool("v", false, "print per-file examples for every file with a defect")
	flag.Parse()

	valid := agents.BuildEffectiveValidBlocks(true, true)
	// Outputs from runs that failed the served-model integrity check must never be scored: they may
	// come from a fallback model. Match by path against runs.jsonl's output_file.
	excluded := map[string]bool{}
	var recs []evalcheck.RunRecord
	if *runs != "" {
		var err error
		if recs, err = evalcheck.ReadRunRecords(*runs); err != nil {
			log.Fatalf("runs: %v", err)
		}
		for _, r := range recs {
			if r.OutputFile != "" && !r.IntegrityOK {
				excluded[filepath.Clean(r.OutputFile)] = true
			}
		}
	}
	skipped := 0
	var files []fileReport
	err := filepath.WalkDir(*root, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(p, ".html") {
			return err
		}
		if excluded[filepath.Clean(p)] {
			skipped++
			return nil
		}
		rel, _ := filepath.Rel(*root, p)
		parts := strings.Split(filepath.ToSlash(rel), "/")
		if len(parts) < 2 {
			return nil
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		scen := parts[0] // always the scenario ID; rep<N> dirs aggregate into it (matches runs.jsonl)
		files = append(files, fileReport{scen, strings.TrimSuffix(parts[len(parts)-1], ".html"), p, evalcheck.Check(string(b), valid)})
		return nil
	})
	if err != nil {
		log.Fatal(err)
	}
	sort.Slice(files, func(i, j int) bool {
		if files[i].Scenario != files[j].Scenario {
			return files[i].Scenario < files[j].Scenario
		}
		return files[i].Query < files[j].Query
	})

	type agg struct {
		n, parseErr, blocks, nonNum, unitB, capDB, modStyle, capClaim, critic, criticMapWrong, gear, snapped, unver, dialOff int
		critTypes                                                                                                            map[string]int
	}
	byScen := map[string]*agg{}
	var order []string
	for _, f := range files {
		a := byScen[f.Scenario]
		if a == nil {
			a = &agg{critTypes: map[string]int{}}
			byScen[f.Scenario] = a
			order = append(order, f.Scenario)
		}
		r := f.Report
		a.n++
		if r.ParseError != "" {
			a.parseErr++
			continue
		}
		a.blocks += r.Blocks
		a.nonNum += r.NonNumericValues
		a.unitB += r.UnitBearingValues
		a.capDB += r.CaptureDBOnModeled
		a.dialOff += r.DialOffsetOnModeled
		a.modStyle += r.ModeledStyleOnCapture
		a.capClaim += r.CaptureClaimOnModeled
		a.critic += r.CriticFlags
		a.criticMapWrong += r.CriticCaptureContradictsMap
		a.gear += r.GearBlocks
		a.snapped += r.SnappedNames
		a.unver += r.UnverifiedNames
		for k, v := range r.CriticByType {
			a.critTypes[k] += v
		}
	}

	fmt.Printf("=== deterministic checks over %d outputs in %s ===\n", len(files), *root)
	if skipped > 0 {
		fmt.Printf("EXCLUDED %d outputs whose run failed served-model integrity (integrity_ok=false in runs.jsonl)\n", skipped)
	}
	fmt.Println()
	fmt.Println("DEFECT counters (ground-truth gated) and REFERENCE counters (ungated, not a pass/fail signal)")
	fmt.Printf("%-24s %4s %5s | %8s %8s %9s %7s %8s | %9s %9s | %7s %6s %11s\n",
		"scenario", "runs", "block", "capDB/mod", "modStyle", "capClaim", "snapped", "unverif", "critic", "criticVsMap", "REF:nonNum", "REF:unit", "REF:dialOff")
	for _, s := range order {
		a := byScen[s]
		fmt.Printf("%-24s %4d %5d | %8d %8d %9d %7d %8d | %9d %9d | %7d %6d %11d\n", s, a.n, a.blocks, a.capDB, a.modStyle, a.capClaim, a.snapped, a.unver, a.critic, a.criticMapWrong, a.nonNum, a.unitB, a.dialOff)
	}
	fmt.Println("\nCritic flags by type:")
	for _, s := range order {
		var ks []string
		for k := range byScen[s].critTypes {
			ks = append(ks, k)
		}
		sort.Strings(ks)
		var parts []string
		for _, k := range ks {
			parts = append(parts, fmt.Sprintf("%s=%d", k, byScen[s].critTypes[k]))
		}
		fmt.Printf("  %-24s %s\n", s, strings.Join(parts, " "))
	}

	fmt.Println("\nFiles with a ground-truth defect or parse error:")
	for _, f := range files {
		r := f.Report
		if r.ParseError == "" && r.Defects() == 0 && r.CriticCaptureContradictsMap == 0 {
			continue
		}
		fmt.Printf("  %-22s %-20s parseErr=%q capDB=%d modStyle=%d capClaim=%d snapped=%d unverified=%d criticVsMap=%d\n",
			f.Scenario, f.Query, r.ParseError, r.CaptureDBOnModeled, r.ModeledStyleOnCapture, r.CaptureClaimOnModeled, r.SnappedNames, r.UnverifiedNames, r.CriticCaptureContradictsMap)
		if *verbose {
			for _, e := range r.Examples {
				fmt.Println("      -", e)
			}
			for _, s := range r.UnverifiedList {
				fmt.Println("      - unverified name:", s)
			}
			for _, s := range r.Snaps {
				fmt.Println("      - snap:", s)
			}
		}
	}

	if *report != "" {
		fails, err := evalcheck.ParseBenchmarkReport(*report)
		if err != nil {
			log.Fatalf("report: %v", err)
		}
		fmt.Printf("\nFailed runs recorded in %s: %d\n", *report, len(fails))
		trunc := map[string]int{}
		for _, f := range fails {
			fmt.Printf("  %-26s %-20s agent=%-14s truncated(MAX_TOKENS)=%v\n", f.Scenario, f.Query, f.Agent, f.Truncated)
			if f.Truncated {
				trunc[f.Agent]++
			}
		}
		fmt.Printf("  truncations by agent: %v\n", trunc)
	}
	if *runs != "" {
		fmt.Printf("\nrun records (%s): %d\n", *runs, len(recs))
		type rs struct {
			n, ok, integ int
			trunc        map[string]int
		}
		by := map[string]*rs{}
		for _, r := range recs {
			x := by[r.Scenario]
			if x == nil {
				x = &rs{trunc: map[string]int{}}
				by[r.Scenario] = x
			}
			x.n++
			if r.Success {
				x.ok++
			}
			if r.IntegrityOK {
				x.integ++
			}
			for a, c := range r.MaxTokensByAgent {
				x.trunc[a] += c
			}
		}
		var ks []string
		for k := range by {
			ks = append(ks, k)
		}
		sort.Strings(ks)
		for _, k := range ks {
			x := by[k]
			fmt.Printf("  %-12s runs=%d success=%d served-model-integrity-ok=%d MAX_TOKENS by agent=%v\n", k, x.n, x.ok, x.integ, x.trunc)
		}
	}
	if *jsonOut != "" {
		b, _ := json.MarshalIndent(files, "", "  ")
		if err := os.WriteFile(*jsonOut, b, 0o644); err != nil {
			log.Fatal(err)
		}
	}
}
