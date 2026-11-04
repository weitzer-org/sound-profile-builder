package main

// eval_subagent is the Phase 1A screen of docs/eval/gemini-model-upgrade-test-plan.md: it runs the
// three agents whose input is a direct function of the user query (1 Tone Historian, 2 Sonic
// Profiler, 3 Community Scraper) on each agent's production model and on the candidate models,
// over the golden queries, WITHOUT running the pipeline. Every other agent consumes upstream
// output and is covered by cmd/eval_full_pipeline (agents 4-11, 13) or cmd/eval_architect (12).
//
// Per call it records the served model, finish reason, whether the response carried Google
// Search grounding evidence, schema validity (against the exact typed schema production sends),
// truncation, latency and tokens, then reports rates per (agent, model).
//
// Safeguards: -dry-run defaults to true and prints the exact call grid; live runs require
// EVAL_NO_FALLBACK=true so a failing candidate fails visibly instead of being answered by a
// fallback model (plan principle 3).

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/weitzer-org/sound-builder/internal/agents"
	"github.com/weitzer-org/sound-builder/internal/config"
	"github.com/weitzer-org/sound-builder/internal/evalcheck"
	"github.com/weitzer-org/sound-builder/internal/evalfixtures"
)

func main() {
	agentsFlag := flag.String("agents", "1,2,3", "independent agents to screen: 1,2,3 (or prompt keys)")
	modelsFlag := flag.String("models", "default", "candidate models for every selected agent, comma-separated; \"default\" = the plan's per-agent candidates. The agent's baseline is always included.")
	queriesFlag := flag.String("queries", "all", "comma-separated golden query names (or all 13)")
	reps := flag.Int("reps", 2, "repetitions per (agent, model, query)")
	dryRun := flag.Bool("dry-run", true, "print the exact call grid and exit without calling Gemini; pass -dry-run=false to spend money")
	parallel := flag.Int("parallel", 1, "concurrent calls (one orchestrator per worker); latency numbers are only comparable within one run")
	maxCalls := flag.Int("max-calls", 0, "hard cap on live calls (0 = the grid size); refuses to start if the grid is larger")
	allowFallback := flag.Bool("allow-fallback", false, "permit a live run without EVAL_NO_FALLBACK=true (results are NOT scoreable)")
	outDir := flag.String("out", "", "output directory (default eval_results/phase1/subagent/<UTC timestamp>); one that already has calls.jsonl is refused")
	flag.Parse()

	if *reps < 1 || *parallel < 1 {
		log.Fatal("-reps and -parallel must be >= 1")
	}
	specs, err := SelectAgents(Specs(), *agentsFlag)
	if err != nil {
		log.Fatal(err)
	}
	golden := evalfixtures.GoldenQueries()
	queries, err := SelectQueries(evalfixtures.GoldenQueryOrder(), golden, *queriesFlag)
	if err != nil {
		log.Fatal(err)
	}
	jobs := BuildGrid(specs, *modelsFlag, queries, *reps)
	perAgent, total := GridCounts(jobs)

	fmt.Printf("Plan: %d agent(s) x %d queries x %d reps\n", len(specs), len(queries), *reps)
	for _, sp := range specs {
		ms := ModelsFor(sp, *modelsFlag)
		fmt.Printf("  agent %d %-18s models=%v  %d x %d x %d = %d calls\n", sp.Num, sp.Role, ms, len(ms), len(queries), *reps, perAgent[sp.Key])
	}
	fmt.Printf("TOTAL: %d API calls (one attempt each; EVAL_NO_FALLBACK disables retries)\n", total)
	if *dryRun {
		fmt.Println("DRY RUN -- no Gemini calls made. Pass -dry-run=false to execute.")
		return
	}
	if *maxCalls > 0 && total > *maxCalls {
		log.Fatalf("grid is %d calls, above -max-calls %d; refusing to start", total, *maxCalls)
	}
	if !agents.EvalNoFallbackEnabled() && !*allowFallback {
		log.Fatalf("%s=true is required for live runs. Set it, or pass -allow-fallback (results will NOT be scoreable).", agents.EvalNoFallbackEnv)
	}
	if os.Getenv("TARGET_MODEL") != "" {
		log.Fatal("TARGET_MODEL is set; unset it so it cannot interfere with per-agent routing")
	}
	key := os.Getenv("GEMINI_API_KEY")
	if key == "" {
		log.Fatal("GEMINI_API_KEY must be set to run live evals")
	}
	cfg, err := config.LoadConfig("config.json")
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	if *outDir == "" {
		*outDir = filepath.Join("eval_results", "phase1", "subagent", time.Now().UTC().Format("20060102T150405Z"))
	}
	callsPath := filepath.Join(*outDir, "calls.jsonl")
	if _, err := os.Stat(callsPath); err == nil {
		log.Fatalf("%s already exists; pick a fresh -out", callsPath)
	}
	if err := os.MkdirAll(*outDir, 0o755); err != nil {
		log.Fatal(err)
	}
	f, err := os.OpenFile(callsPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	ctx := context.Background()
	jobCh := make(chan Job)
	var mu sync.Mutex
	var results []CallResult
	var wg sync.WaitGroup
	done := 0
	for w := 0; w < *parallel; w++ {
		orch, err := agents.NewOrchestrator(ctx, key, nil)
		if err != nil {
			log.Fatalf("init orchestrator: %v", err)
		}
		wg.Add(1)
		go func(orch *agents.Orchestrator) {
			defer wg.Done()
			defer orch.Close()
			for j := range jobCh {
				res := runJob(ctx, orch, j, golden[j.Query], cfg.AgentPrompts[j.Spec.Key])
				mu.Lock()
				results = append(results, res)
				done++
				if b, err := json.Marshal(res); err == nil {
					fmt.Fprintln(f, string(b))
				} else {
					log.Printf("marshal result: %v", err)
				}
				log.Printf("[%d/%d] agent %d %-22s %-26s rep%d success=%v schema=%v finish=%s grounded=%v served=%s %.1fs",
					done, total, j.Spec.Num, j.Query, j.Model, j.Rep, res.Success, res.SchemaValid, res.FinishReason, res.Grounded, res.ServedModel, float64(res.LatencyMs)/1000)
				mu.Unlock()
			}
		}(orch)
	}
	for _, j := range jobs {
		jobCh <- j
	}
	close(jobCh)
	wg.Wait()

	stats := Aggregate(results)
	md := Markdown(stats)
	if err := os.WriteFile(filepath.Join(*outDir, "summary.md"), []byte(md), 0o644); err != nil {
		log.Printf("write summary.md: %v", err)
	}
	if b, err := json.MarshalIndent(stats, "", "  "); err == nil {
		if err := os.WriteFile(filepath.Join(*outDir, "summary.json"), b, 0o644); err != nil {
			log.Printf("write summary.json: %v", err)
		}
	}
	fmt.Println(md)
	log.Printf("Done. Raw per-call records: %s", callsPath)
}

// runJob executes one call exactly as RunPipeline would for that agent (production prompt
// selection, generation config, tools and caps) and scores it.
func runJob(ctx context.Context, orch *agents.Orchestrator, j Job, query, promptVersion string) CallResult {
	res := CallResult{Agent: j.Spec.Role, AgentKey: j.Spec.Key, Model: j.Model, Baseline: j.Baseline, Query: j.Query, Rep: j.Rep, PromptVersion: promptVersion}
	orch.Usage = &agents.TokenUsage{ModelsUsed: map[string]int{}}
	orch.AgentModels = map[string]string{j.Spec.Key: j.Model}
	sys, err := orch.PromptForAgent(j.Spec.Key, promptVersion)
	if err != nil {
		res.Error = "load prompt: " + err.Error()
		return res
	}
	out, runErr := orch.RunAgentSplit(ctx, j.Spec.Role, sys, j.Spec.BuildUserPrompt(query))
	calls := orch.Usage.CallsSnapshot()
	res.CallCount = len(calls)
	if len(calls) > 0 {
		c := calls[len(calls)-1]
		res.ServedModel, res.FinishReason, res.Success = c.ServedModel, c.FinishReason, c.Success
		res.Grounded, res.SearchQueries = c.Grounded, c.SearchQueries
		res.LatencyMs, res.InputTokens, res.OutputTokens, res.ThinkTokens = c.LatencyMs, c.InputTokens, c.OutputTokens, c.ThinkingTokens
		res.Error = c.Error
		res.Truncated = c.FinishReason == "MAX_TOKENS"
		res.ServedOK = c.AttemptModel == j.Model && agents.ServedModelMatches(ExpectedServed(j.Model), c.ServedModel)
	}
	if runErr != nil {
		res.Success = false
		if res.Error == "" {
			res.Error = runErr.Error()
		}
		return res
	}
	res.Output = out
	res.SchemaIssues = evalcheck.ValidateAgainstSchema(out, agents.AgentResponseSchema(j.Spec.Key))
	res.SchemaValid = len(res.SchemaIssues) == 0
	if len(res.SchemaIssues) > 5 {
		res.SchemaIssues = append(res.SchemaIssues[:5], fmt.Sprintf("... %d more", len(res.SchemaIssues)-5))
	}
	return res
}
