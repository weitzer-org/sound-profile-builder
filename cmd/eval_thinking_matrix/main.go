package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/weitzer-org/sound-builder/internal/agents"
	"github.com/weitzer-org/sound-builder/internal/evalfixtures"
)

// eval_thinking_matrix is Phase 1 of the gemini-3.5-flash/gemini-3.6-flash/gemini-3.1-pro-preview
// thinking-budget comparison. It runs the real full 13-agent pipeline (RunPipeline, like
// cmd/eval_full_pipeline) across a {model, thinking budget} grid and scores each output with
// RunMechanicalQualityChecks -- a deterministic, judge-free signal built from the same Flag*
// checks the API layer already runs in production -- instead of cmd/judge_compare's blind
// pairwise LLM judge, which TODO.md's Pipeline Quality Work section documents as demonstrably
// unstable (issue #68: re-judging identical files produces different verdicts, at least one
// self-contradictory, and two confirmed judge hallucinations against real, verified data).
//
// The budget grid excludes cells cmd/probe_thinking already confirmed don't work: budget=0
// (disable thinking) is only honored by gemini-3.5-flash -- both gemini-3.6-flash and
// gemini-3.1-pro-preview reject it outright (400 INVALID_ARGUMENT, "This model only works in
// thinking mode"), and gemini-3.6-flash additionally has no working fallback at budget=0
// (its only fallback, gemini-2.5-pro, rejects it too -- see getFallbackChain's default case).
// Testing those cells would either error every rep or (worse) silently substitute a different
// model via the fallback chain, which cmd/probe_thinking caught happening for
// gemini-3.1-pro-preview/budget=0 before this exclusion was added.
//
// Defaults are deliberately cost-conservative: -dry-run defaults true (this repo has no
// per-token pricing table the way job_tracker's internal/scoring/pricing.go does, so the only
// cost control available here is call count -- printing the plan by default, requiring an
// explicit -dry-run=false to spend real API budget, is the safer default for a tool that runs
// the full 12-13-agent pipeline per cell rather than one cheap call), and -queries defaults to
// the 3 golden-set prompts TODO.md already ties to the known gemini-3.6-flash regressions
// (07_Edge: description-vs-implementation contradiction; 08_EVH and 09_BB_King: the two
// truncation/verbosity outliers from the 2026-07-22 eval round) rather than the full 13.

type thinkingConfig struct {
	ModelLabel  string
	Model       string
	BudgetLabel string
	Budget      *int32
}

func int32Ptr(v int32) *int32 { return &v }

// configGrid is hardcoded (not flag-driven) because the valid budget set differs per model --
// see cmd/probe_thinking's findings in the package doc comment above -- and a flag-driven
// cross product would let a caller silently re-request the confirmed-invalid combinations.
func configGrid() []thinkingConfig {
	return []thinkingConfig{
		{"gemini-3.5-flash", "gemini-3.5-flash", "default", nil},
		{"gemini-3.5-flash", "gemini-3.5-flash", "0 (disabled)", int32Ptr(0)},
		{"gemini-3.5-flash", "gemini-3.5-flash", "512", int32Ptr(512)},
		{"gemini-3.5-flash", "gemini-3.5-flash", "2048", int32Ptr(2048)},

		{"gemini-3.6-flash", "gemini-3.6-flash", "default", nil},
		// budget=0 deliberately excluded: confirmed rejected with no working fallback (see doc comment)
		{"gemini-3.6-flash", "gemini-3.6-flash", "512", int32Ptr(512)},
		{"gemini-3.6-flash", "gemini-3.6-flash", "2048", int32Ptr(2048)},

		{"gemini-3.1-pro-preview", "gemini-3.1-pro-preview", "default", nil},
		// budget=0 deliberately excluded: confirmed rejected by the primary model (see doc comment)
		{"gemini-3.1-pro-preview", "gemini-3.1-pro-preview", "512", int32Ptr(512)},
		{"gemini-3.1-pro-preview", "gemini-3.1-pro-preview", "2048", int32Ptr(2048)},
	}
}

type runOutcome struct {
	LatencySec     float64
	InputTokens    int32
	OutputTokens   int32
	ThinkingTokens int32
	Report         agents.MechanicalQualityReport
	ServedByOther  string
	Error          string
}

func main() {
	dryRun := flag.Bool("dry-run", true, "print the run plan (config count, query count, total live pipeline runs) and exit without calling Gemini; pass -dry-run=false to actually run")
	reps := flag.Int("reps", 2, "repetitions per (model, thinking budget, query) cell, for a basic stability signal (min/max/avg across reps)")
	queriesFlag := flag.String("queries", "07_Edge,08_EVH,09_BB_King", "comma-separated golden-set query names to run (see evalfixtures.GoldenQueryOrder for the full list)")
	allQueries := flag.Bool("all-queries", false, "run the full 13-query golden set instead of -queries (expensive: 10 configs x 13 queries x reps full-pipeline runs)")
	modelsFlag := flag.String("models", "", "comma-separated model labels to restrict the config grid to (e.g. 'gemini-3.5-flash'); empty = all 3 models. Useful for a cheap smoke run or resuming a partial sweep.")
	budgetsFlag := flag.String("budgets", "", "comma-separated budget labels to restrict the config grid to (e.g. 'default,512'); empty = all valid budgets per model")
	outDir := flag.String("out", "eval_results/thinking_matrix", "output directory for the report")
	flag.Parse()

	if *reps < 1 {
		log.Fatalf("-reps must be >= 1, got %d", *reps)
	}

	goldenQueries := evalfixtures.GoldenQueries()

	var queryNames []string
	if *allQueries {
		queryNames = evalfixtures.GoldenQueryOrder()
	} else {
		queryNames = strings.Split(*queriesFlag, ",")
		for i := range queryNames {
			queryNames[i] = strings.TrimSpace(queryNames[i])
		}
	}
	for _, name := range queryNames {
		if _, ok := goldenQueries[name]; !ok {
			log.Fatalf("unknown query name %q -- see evalfixtures.GoldenQueryOrder for valid names", name)
		}
	}

	configs := configGrid()
	if *modelsFlag != "" {
		wanted := toSet(*modelsFlag)
		configs = filterConfigs(configs, func(c thinkingConfig) bool { return wanted[c.ModelLabel] })
	}
	if *budgetsFlag != "" {
		wanted := toSet(*budgetsFlag)
		configs = filterConfigs(configs, func(c thinkingConfig) bool { return wanted[c.BudgetLabel] })
	}
	if len(configs) == 0 {
		log.Fatalf("-models/-budgets filtered out every config -- nothing to run")
	}
	totalRuns := len(configs) * len(queryNames) * *reps

	log.Printf("Plan: %d configs x %d queries x %d reps = %d live full-pipeline runs (each is ~12-13 agent calls)", len(configs), len(queryNames), *reps, totalRuns)
	for _, c := range configs {
		log.Printf("  - %s @ thinking_budget=%s", c.ModelLabel, c.BudgetLabel)
	}
	log.Printf("Queries: %s", strings.Join(queryNames, ", "))

	if *dryRun {
		log.Println("DRY RUN -- no Gemini calls made. Pass -dry-run=false to execute this plan.")
		return
	}

	geminiKey := os.Getenv("GEMINI_API_KEY")
	if geminiKey == "" {
		log.Fatalf("GEMINI_API_KEY must be set to run live evals")
	}
	if err := os.MkdirAll(*outDir, 0755); err != nil {
		log.Fatalf("Failed to create output dir: %v", err)
	}

	ctx := context.Background()
	// allow_factory_captures=true (evalfixtures.DefaultConstraints) and allow_user_captures
	// defaulting true (RunPipeline's own default when the constraint key is absent) --
	// mirrors the same effective values RunPipeline itself computes for this constraints
	// payload, so the ground truth used here matches what the pipeline run actually saw.
	validBlocks := agents.BuildEffectiveValidBlocks(true, true)
	constraints := evalfixtures.DefaultConstraints()

	// The full grid can be 100s of live full-pipeline runs taking hours -- the report is
	// written incrementally, one row per (config, query) cell flushed to disk as soon as
	// that cell's reps finish, so an interrupted run still leaves every completed cell's
	// results on disk instead of losing the whole run to a single missing final write.
	reportPath := filepath.Join(*outDir, fmt.Sprintf("report-%s.md", time.Now().Format("2006-01-02-150405")))
	reportFile, err := os.Create(reportPath)
	if err != nil {
		log.Fatalf("Failed to create report file: %v", err)
	}
	defer reportFile.Close()
	fmt.Fprintf(reportFile, "# Thinking-Budget Matrix Eval Report\n\n")
	fmt.Fprintf(reportFile, "Queries: %s | Reps per cell: %d\n\n", strings.Join(queryNames, ", "), *reps)
	fmt.Fprintf(reportFile, "Scored with RunMechanicalQualityChecks (deterministic, judge-free -- see package doc comment). Lower TotalDefects is better; 0 is clean.\n\n")
	fmt.Fprintf(reportFile, "Written incrementally -- if this run is interrupted, every row up to the last one present already reflects a completed cell.\n\n")
	fmt.Fprintf(reportFile, "| Model | Budget | Query | Reps OK | Avg Latency(s) | Avg In | Avg Out | Avg Thinking | Avg Defects | Unverified | CaptureFmt | Cabinet | Ranges | Critic | Fallback/Parse Issues |\n")
	fmt.Fprintf(reportFile, "|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|\n")
	reportFile.Sync()

	for _, cfg := range configs {
		for _, qName := range queryNames {
			query := goldenQueries[qName]
			var outcomes []runOutcome

			for rep := 0; rep < *reps; rep++ {
				log.Printf("Running %s @ budget=%s | %s | rep %d/%d ...", cfg.ModelLabel, cfg.BudgetLabel, qName, rep+1, *reps)

				orch, err := agents.NewOrchestrator(ctx, geminiKey, nil)
				if err != nil {
					// Recorded as a failed cell, not log.Fatalf: this can be a transient
					// network/client-construction hiccup, and crashing the whole (potentially
					// multi-hour) matrix over one cell defeats the incremental-write design's
					// whole purpose of surviving a single bad cell (GSR finding on PR #84).
					log.Printf("  ❌ failed to init orchestrator: %v", err)
					outcomes = append(outcomes, runOutcome{Error: err.Error()})
					continue
				}
				orch.AgentModels = map[string]string{
					"1_tone_historian": cfg.Model, "2_sonic_profiler": cfg.Model, "3_community_scraper": cfg.Model,
					"4_coros_librarian": cfg.Model, "5_cloud_navigator": cfg.Model, "6_acoustician": cfg.Model,
					"7_transducer_tech": cfg.Model, "8_foh_optimizer": cfg.Model, "9_mix_engineer": cfg.Model,
					"10_control_mapper": cfg.Model, "11_dsp_dispatcher": cfg.Model, "12_architect": cfg.Model,
					"13_critic": cfg.Model,
				}
				orch.ThinkingBudget = cfg.Budget

				start := time.Now()
				_, usage, runErr := orch.RunPipeline(ctx, query, constraints, nil, nil)
				latency := time.Since(start).Seconds()

				out := runOutcome{LatencySec: latency}
				if runErr != nil {
					out.Error = runErr.Error()
					log.Printf("  ❌ %v", runErr)
				} else {
					out.InputTokens = usage.InputTokens
					out.OutputTokens = usage.OutputTokens
					out.ThinkingTokens = usage.ThinkingTokens
					// Collect every model other than the one requested, not just the last one a
					// (non-deterministic) map iteration happens to land on -- a single pipeline run
					// can fall back to more than one distinct model across its ~13 agent calls, and
					// silently dropping all but one would defeat the whole point of this check
					// (catching fallback-masked "successes").
					var fallbacks []string
					for servedModel := range usage.ModelsUsed {
						if servedModel != cfg.Model {
							fallbacks = append(fallbacks, servedModel)
						}
					}
					sort.Strings(fallbacks)
					out.ServedByOther = strings.Join(fallbacks, ",")
					out.Report = agents.RunMechanicalQualityChecks(orch.LastArchitectJSON(), validBlocks)
					if out.Report.ParseError != "" {
						log.Printf("  ⚠️  %.1fs | mechanical-check parse error: %s", latency, out.Report.ParseError)
					} else {
						log.Printf("  ✅ %.1fs | defects=%d | in=%d out=%d thinking=%d", latency, out.Report.TotalDefects(), out.InputTokens, out.OutputTokens, out.ThinkingTokens)
					}
					if out.ServedByOther != "" {
						log.Printf("  ⚠️  silently served by fallback model %s instead of requested %s", out.ServedByOther, cfg.Model)
					}
				}
				orch.Close()
				outcomes = append(outcomes, out)
			}
			fmt.Fprint(reportFile, renderCellRow(cfg, qName, outcomes))
			reportFile.Sync()
		}
	}

	log.Printf("🏁 Report written to %s", reportPath)
}

// renderCellRow aggregates one (config, query) cell's reps into a single markdown table row.
func renderCellRow(cfg thinkingConfig, qName string, outcomes []runOutcome) string {
	var okCount int
	var sumLatency, sumIn, sumOut, sumThinking, sumDefects float64
	var sumUnverified, sumCaptureFmt, sumCabinet, sumRanges, sumCritic float64
	var issues []string
	for _, r := range outcomes {
		if r.Error != "" {
			issues = append(issues, "error: "+truncate(r.Error, 60))
			continue
		}
		if r.Report.ParseError != "" {
			issues = append(issues, "parse-error")
			continue
		}
		okCount++
		sumLatency += r.LatencySec
		sumIn += float64(r.InputTokens)
		sumOut += float64(r.OutputTokens)
		sumThinking += float64(r.ThinkingTokens)
		sumDefects += float64(r.Report.TotalDefects())
		sumUnverified += float64(r.Report.UnverifiedBlocks)
		sumCaptureFmt += float64(r.Report.CaptureFormattingMismatches)
		sumCabinet += float64(r.Report.IncompleteCabinetBlocks)
		sumRanges += float64(r.Report.LeftoverValueRanges)
		sumCritic += float64(r.Report.CriticIssues)
		if r.ServedByOther != "" {
			issues = append(issues, "fallback->"+r.ServedByOther)
		}
	}
	issueCol := "-"
	if len(issues) > 0 {
		issueCol = strings.Join(issues, "; ")
	}
	if okCount == 0 {
		return fmt.Sprintf("| %s | %s | %s | 0/%d | - | - | - | - | - | - | - | - | - | - | %s |\n",
			cfg.ModelLabel, cfg.BudgetLabel, qName, len(outcomes), issueCol)
	}
	n := float64(okCount)
	return fmt.Sprintf("| %s | %s | %s | %d/%d | %.1f | %.0f | %.0f | %.0f | %.2f | %.1f | %.1f | %.1f | %.1f | %.1f | %s |\n",
		cfg.ModelLabel, cfg.BudgetLabel, qName, okCount, len(outcomes),
		sumLatency/n, sumIn/n, sumOut/n, sumThinking/n, sumDefects/n,
		sumUnverified/n, sumCaptureFmt/n, sumCabinet/n, sumRanges/n, sumCritic/n, issueCol)
}

func toSet(csv string) map[string]bool {
	set := make(map[string]bool)
	for _, s := range strings.Split(csv, ",") {
		set[strings.TrimSpace(s)] = true
	}
	return set
}

func filterConfigs(configs []thinkingConfig, keep func(thinkingConfig) bool) []thinkingConfig {
	var out []thinkingConfig
	for _, c := range configs {
		if keep(c) {
			out = append(out, c)
		}
	}
	return out
}

// truncate flattens s to a single line, escapes markdown table delimiters, and shortens it to
// at most n runes total, including the trailing ellipsis (not bytes -- error text can carry
// multi-byte UTF-8 like curly quotes or em-dashes, and a byte-index slice can split one and
// corrupt the output).
func truncate(s string, n int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "|", "-")
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	if n <= 0 {
		return ""
	}
	return string(runes[:n-1]) + "…"
}
