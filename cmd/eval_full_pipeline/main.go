package main

// eval_full_pipeline runs the real 13-agent pipeline (RunPipeline) under the routing scenarios
// of docs/eval/gemini-model-upgrade-test-plan.md §5 (S0-S5, A1-A3), so every agent gets the
// genuine upstream context it gets in production -- unlike cmd/eval_subagent, which can only
// isolate agents whose input doesn't depend on another agent.
//
// Safeguards (plan §2 principles):
//   - -dry-run defaults to true and prints the exact run count; pass -dry-run=false to spend money.
//   - Live runs REQUIRE EVAL_NO_FALLBACK=true (unless -allow-fallback), so a failing candidate
//     fails visibly instead of being scored on gemini-2.5-pro.
//   - Every attempt is recorded with the API-reported served model and finish reason; a run in
//     which any attempt failed, fell back, or was served by a different model than the
//     scenario asked for is marked integrity_ok=false and must not be scored.
//   - One JSON line per run goes to <out>/runs.jsonl as soon as the run finishes.

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/weitzer-org/sound-builder/internal/agents"
	"github.com/weitzer-org/sound-builder/internal/config"
	"github.com/weitzer-org/sound-builder/internal/evalcheck"
	"github.com/weitzer-org/sound-builder/internal/evalfixtures"
)

func main() {
	scenariosFlag := flag.String("scenarios", "all", "comma-separated scenario IDs: S0,S1,S2,S3,S4,S5,A1,A2,A3 (or all)")
	queriesFlag := flag.String("queries", "all", "comma-separated golden-set query names (or all 13)")
	reps := flag.Int("reps", 2, "repetitions per (scenario, query)")
	dryRun := flag.Bool("dry-run", true, "print the run plan and exit without calling Gemini; pass -dry-run=false to spend real money")
	includeSkipped := flag.Bool("include-skipped", false, "also run scenarios marked skipped (e.g. S3 once pro-latest resolves to the baseline Pro model)")
	allowFallback := flag.Bool("allow-fallback", false, "permit a live run without EVAL_NO_FALLBACK=true (results will NOT be scoreable)")
	argonNoSearch := flag.Bool("argon-no-search", false, "keep agents 1,2,3 off Argon in A1/A2/A3 (use if the Phase 0.5 probe shows Argon cannot use GoogleSearch)")
	smoke := flag.Bool("smoke", false, "plan §5 cost-gate smoke pass: "+SmokeQuery+" x 1 rep x every selected scenario (8 executable runs for the full set). Cannot be combined with -queries/-reps. Then run: go run ./cmd/eval_cost -root <out>")
	outDir := flag.String("out", "", "output directory (default: eval_results/full_pipeline_gemini_upgrade/<UTC timestamp>, or .../smoke-<UTC timestamp> with -smoke; a fresh one per invocation); an existing one that already has runs.jsonl is refused so earlier artifacts are never overwritten")
	flag.Parse()

	if *smoke {
		explicit := map[string]bool{}
		flag.Visit(func(f *flag.Flag) { explicit[f.Name] = true })
		var err error
		if *queriesFlag, *reps, err = ApplySmoke(explicit, *queriesFlag, *reps); err != nil {
			log.Fatal(err)
		}
	}
	if *reps < 1 {
		log.Fatal("-reps must be >= 1")
	}
	allScenarios := BuildScenarios(*argonNoSearch)
	selected, err := SelectScenarios(allScenarios, *scenariosFlag)
	if err != nil {
		log.Fatal(err)
	}
	goldenQueries := evalfixtures.GoldenQueries()
	queryNames, err := SelectQueries(evalfixtures.GoldenQueryOrder(), goldenQueries, *queriesFlag)
	if err != nil {
		log.Fatal(err)
	}
	plan := MakePlan(selected, len(queryNames), *reps, *includeSkipped)

	if *smoke {
		fmt.Printf("SMOKE PASS (plan §5 cost gate): %s x 1 rep x every selected scenario\n", SmokeQuery)
	}
	fmt.Printf("Plan: %d scenarios x %d queries x %d reps\n", plan.Scenarios, plan.Queries, plan.Reps)
	for _, s := range selected {
		note := ""
		if s.Skip != "" && !*includeSkipped {
			note = "   SKIPPED: " + s.Skip
		}
		fmt.Printf("  %-3s %-36s %3d runs%s\n", s.ID, s.Label, plan.Queries*plan.Reps, note)
	}
	fmt.Printf("TOTAL: %d pipeline runs", plan.Total)
	if plan.Executable != plan.Total {
		fmt.Printf(" planned, %d executable (skipped scenarios excluded; -include-skipped to force)", plan.Executable)
	}
	fmt.Printf("\n~%d agent calls before retries (13 per run)\n", plan.AgentCalls)
	if *dryRun {
		fmt.Println("DRY RUN -- no Gemini calls made. Pass -dry-run=false to execute.")
		return
	}

	if !agents.EvalNoFallbackEnabled() && !*allowFallback {
		log.Fatalf("%s=true is required for live runs (otherwise a failed candidate is silently scored on a fallback model). Set it, or pass -allow-fallback to override.", agents.EvalNoFallbackEnv)
	}
	key := os.Getenv("GEMINI_API_KEY")
	if key == "" {
		log.Fatal("GEMINI_API_KEY must be set to run live evals")
	}
	cfg, err := config.LoadConfig("config.json")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}
	ctx := context.Background()
	orch, err := agents.NewOrchestrator(ctx, key, nil)
	if err != nil {
		log.Fatalf("Failed to initialize Orchestrator: %v", err)
	}
	defer orch.Close()

	if *outDir == "" {
		stamp := time.Now().UTC().Format("20060102T150405Z")
		if *smoke {
			stamp = "smoke-" + stamp
		}
		*outDir = filepath.Join("eval_results", "full_pipeline_gemini_upgrade", stamp)
	}
	if _, err := os.Stat(filepath.Join(*outDir, "runs.jsonl")); err == nil {
		log.Fatalf("%s already has runs.jsonl; pick a fresh -out so earlier outputs are not overwritten (a failed rerun would otherwise leave stale HTML that eval_checks still scores)", *outDir)
	}
	if err := os.MkdirAll(*outDir, 0o755); err != nil {
		log.Fatal(err)
	}
	runsFile, err := os.OpenFile(filepath.Join(*outDir, "runs.jsonl"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		log.Fatal(err)
	}
	defer runsFile.Close()

	// The manifest is the batch's identity (plan §2 principle 5): the Phase 2 report/cost tools
	// refuse to compare runs that do not share it.
	manifest := BuildManifest(selected, queryNames, *reps, *smoke, *includeSkipped, time.Now().UTC())
	if err := evalcheck.WriteManifest(*outDir, manifest); err != nil {
		log.Fatalf("manifest: %v", err)
	}

	constraints := evalfixtures.DefaultConstraints()
	var records []evalcheck.RunRecord

	// Sequential on purpose (API rate limits; same as the July tool).
	for _, sc := range selected {
		if sc.Skip != "" && !*includeSkipped {
			log.Printf("skipping %s: %s", sc.ID, sc.Skip)
			continue
		}
		orch.AgentModels = sc.AgentModels
		for rep := 1; rep <= *reps; rep++ {
			dir := filepath.Join(*outDir, sc.ID, fmt.Sprintf("rep%d", rep))
			if err := os.MkdirAll(dir, 0o755); err != nil {
				log.Fatal(err)
			}
			for _, name := range queryNames {
				log.Printf("%s rep%d %s ...", sc.ID, rep, name)
				orch.Usage = &agents.TokenUsage{ModelsUsed: map[string]int{}}
				start := time.Now()
				out, _, runErr := orch.RunPipeline(ctx, goldenQueries[name], constraints, cfg.AgentPrompts, nil)
				rec := evalcheck.RunRecord{BatchID: manifest.BatchID, Scenario: sc.ID, Query: name, Rep: rep, LatencySec: time.Since(start).Seconds(), Success: runErr == nil}
				if runErr != nil {
					rec.Error = runErr.Error()
				} else {
					rec.OutputFile = filepath.Join(dir, name+".html") // JSON envelope; extension kept for cmd/judge_compare
					if err := os.WriteFile(rec.OutputFile, []byte(out), 0o644); err != nil {
						log.Printf("write output: %v", err)
					}
				}
				calls := orch.Usage.CallsSnapshot()
				rec.IntegrityOK, rec.IntegrityIssues, rec.MaxTokensByAgent = evalcheck.AssessCalls(calls, sc.AgentModels, sc.Pins)
				rec.CallsFile = filepath.Join(dir, name+".calls.json")
				if b, err := json.MarshalIndent(calls, "", "  "); err != nil {
					log.Printf("calls marshal: %v", err)
				} else if err := os.WriteFile(rec.CallsFile, b, 0o644); err != nil {
					log.Printf("write calls file %s: %v", rec.CallsFile, err)
				}
				if line, err := json.Marshal(rec); err == nil {
					fmt.Fprintln(runsFile, string(line))
				}
				records = append(records, rec)
				log.Printf("   success=%v integrity_ok=%v %.0fs %s", rec.Success, rec.IntegrityOK, rec.LatencySec, strings.Join(rec.IntegrityIssues, "; "))
			}
		}
	}
	writeSummary(filepath.Join(*outDir, "summary.md"), records)
	log.Printf("Done. Per-run records: %s ; deterministic scoring: go run ./cmd/eval_checks -root %s -runs %s", filepath.Join(*outDir, "runs.jsonl"), *outDir, filepath.Join(*outDir, "runs.jsonl"))
}

func writeSummary(path string, recs []evalcheck.RunRecord) {
	type agg struct {
		n, ok, integ int
		lat          float64
		trunc        map[string]int
	}
	by := map[string]*agg{}
	for _, r := range recs {
		a := by[r.Scenario]
		if a == nil {
			a = &agg{trunc: map[string]int{}}
			by[r.Scenario] = a
		}
		a.n++
		a.lat += r.LatencySec
		if r.Success {
			a.ok++
		}
		if r.IntegrityOK {
			a.integ++
		}
		for k, v := range r.MaxTokensByAgent {
			a.trunc[k] += v
		}
	}
	var ids []string
	for id := range by {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	var sb strings.Builder
	sb.WriteString("# Full-pipeline eval summary\n\nScore only runs with integrity_ok=true. Quality: `go run ./cmd/eval_checks`.\n\n")
	sb.WriteString("| Scenario | Runs | Success | Served-model integrity OK | Avg latency (s) | MAX_TOKENS by agent |\n| :-- | --: | --: | --: | --: | :-- |\n")
	for _, id := range ids {
		a := by[id]
		sb.WriteString(fmt.Sprintf("| %s | %d | %d | %d | %.0f | %v |\n", id, a.n, a.ok, a.integ, a.lat/float64(a.n), a.trunc))
	}
	if err := os.WriteFile(path, []byte(sb.String()), 0o644); err != nil {
		log.Printf("summary: %v", err)
	}
}
