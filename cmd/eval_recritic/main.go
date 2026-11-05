// eval_recritic is plan §6.2.1: re-run ONE fixed Critic model over every scoreable output in a
// results directory and record a defect count per run, so scenarios with different pipeline
// Critics (S5, A1: Argon) are scored by the same reviewer.
//
//	go run ./cmd/eval_recritic -root <batch>                  # dry run: prints the call count, spends nothing
//	EVAL_NO_FALLBACK=true go run ./cmd/eval_recritic -root <batch> -dry-run=false
//
// One Critic call per output. Results go to <root>/recritic.jsonl (resumable: outputs already
// answered successfully are skipped). The pipeline's own Critic, outputs and calls.json are never
// touched. cmd/critic_probe is a different, older tool (hard-coded gemini-3.1-pro-preview, no
// served-model check) and is left as it was.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"sort"
	"strings"

	"github.com/weitzer-org/sound-builder/internal/agents"
	"github.com/weitzer-org/sound-builder/internal/config"
	"github.com/weitzer-org/sound-builder/internal/evalcheck"
	"github.com/weitzer-org/sound-builder/internal/evalcritic"
)

const criticKey = "13_critic"

func main() {
	root := flag.String("root", "", "results directory written by eval_full_pipeline")
	dryRun := flag.Bool("dry-run", true, "print the call count and exit without calling Gemini; pass -dry-run=false to spend money")
	scen := flag.String("scenarios", "", "comma-separated scenario IDs to re-critic (default: all in the batch)")
	model := flag.String("model", "", "Critic model to pin (default: S0's 13_critic from manifest.json)")
	served := flag.String("expect-served", "", "served model id the pin must report (default: S0's pin, e.g. gemini-3.6-flash for gemini-3.5-flash)")
	includeUnscoreable := flag.Bool("include-unscoreable", false, "also re-critic runs that failed integrity (their output may come from a fallback model; off by default)")
	legacy := flag.Bool("legacy", false, "root is a pre-manifest directory (e.g. the July eval_results/full_pipeline): list <scenario>/<query>.html; served-model integrity CANNOT be verified there")
	maxCalls := flag.Int("max-calls", 300, "hard cap on Critic calls in one live invocation")
	flag.Parse()
	if *root == "" {
		log.Fatal("-root is required")
	}
	load := evalcheck.LoadBatch
	if *legacy {
		load = evalcheck.LoadLegacy
	}
	b, err := load(*root)
	if err != nil {
		log.Fatal(err)
	}
	if len(b.Problems) > 0 {
		log.Fatalf("batch has structural problems, refusing: %s", strings.Join(b.Problems, "; "))
	}
	cfg, src := evalcritic.Resolve(b.Manifest, *model, *served)
	var only map[string]bool
	if *scen != "" {
		only = map[string]bool{}
		for _, s := range strings.Split(*scen, ",") {
			only[strings.TrimSpace(s)] = true
		}
	}
	outs, skipped := evalcritic.Plan(b, only, *includeUnscoreable)
	resPath := evalcritic.ResultsPath(*root)
	done, err := evalcritic.LoadDone(resPath)
	if err != nil {
		log.Fatal(err)
	}
	var todo []evalcritic.Output
	perScen := map[string]int{}
	for _, o := range outs {
		if _, ok := done[o.Key]; ok {
			continue
		}
		todo = append(todo, o)
		perScen[o.Key.Scenario]++
	}

	if b.Legacy {
		fmt.Println("LEGACY directory: no calls files, so which model produced each output is UNVERIFIED.")
	}
	fmt.Printf("Re-critic: model %s (expected served id %s; source: %s)\n", cfg.Model, cfg.ExpectServe, src)
	fmt.Printf("Outputs to review: %d (already done: %d; skipped as unscoreable/no output: %d)\n", len(todo), len(done), len(skipped))
	var ids []string
	for id := range perScen {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		fmt.Printf("  %-4s %3d Critic calls\n", id, perScen[id])
	}
	for _, s := range skipped {
		fmt.Printf("  skipped %s: %s\n", s.Key, s.Reason)
	}
	fmt.Printf("TOTAL: %d Critic calls (one per output). Cost is NOT estimated here: no smoke measurement of a Critic-only call exists yet.\n", len(todo))
	if *dryRun {
		fmt.Println("DRY RUN -- no Gemini calls made. Pass -dry-run=false to execute.")
		return
	}

	if len(todo) > *maxCalls {
		log.Fatalf("%d calls exceeds -max-calls %d; raise it deliberately or narrow with -scenarios", len(todo), *maxCalls)
	}
	if !agents.EvalNoFallbackEnabled() {
		log.Fatalf("%s=true is required: otherwise a failed Critic call is silently served by a fallback model and the re-critic stops being uniform", agents.EvalNoFallbackEnv)
	}
	if os.Getenv("TARGET_MODEL") != "" {
		log.Fatal("TARGET_MODEL is set and would override the pinned Critic model; unset it")
	}
	key := os.Getenv("GEMINI_API_KEY")
	if key == "" {
		log.Fatal("GEMINI_API_KEY must be set for a live run")
	}
	cc, err := config.LoadConfig("config.json")
	if err != nil {
		log.Fatalf("config: %v", err)
	}
	version := cc.AgentPrompts[criticKey] // same version the pipeline's Critic uses ("" = base prompt)
	sys, err := loadCriticPrompt(cfg.Model, version)
	if err != nil {
		log.Fatal(err)
	}
	ctx := context.Background()
	orch, err := agents.NewOrchestrator(ctx, key, nil)
	if err != nil {
		log.Fatalf("orchestrator: %v", err)
	}
	defer orch.Close()
	orch.AgentModels = map[string]string{criticKey: cfg.Model}
	call := func(ctx context.Context, input string) (string, []agents.CallRecord, error) {
		orch.Usage = &agents.TokenUsage{ModelsUsed: map[string]int{}}
		raw, err := orch.RunAgentSplit(ctx, "Preset Critic", sys, input)
		return raw, orch.Usage.CallsSnapshot(), err
	}
	failed := 0
	for i, o := range todo { // sequential on purpose (rate limits)
		log.Printf("[%d/%d] re-critic %s", i+1, len(todo), o.Key)
		res := evalcritic.Review(ctx, cfg, o, call)
		if b.Manifest != nil {
			res.BatchID = b.Manifest.BatchID
		}
		if err := evalcritic.AppendResult(resPath, res); err != nil {
			log.Fatalf("write %s: %v", resPath, err)
		}
		if !res.OK {
			failed++
			log.Printf("   NOT SCORED: %s", res.Error)
		} else {
			log.Printf("   defects=%d unmatched=%d served=%s %dms", res.Defects, res.Unmatched, res.ServedModel, res.LatencyMs)
		}
	}
	fmt.Printf("Done: %d reviewed, %d not scored (see %s; not-scored outputs are excluded from means, never counted as 0).\n", len(todo)-failed, failed, resPath)
}

// loadCriticPrompt mirrors Orchestrator.loadPromptForAgent: a Flash model prefers
// <key>_flash.md when it exists, otherwise the base prompt (today only 13_critic.md exists).
func loadCriticPrompt(model, version string) (string, error) {
	if strings.Contains(model, "flash") {
		if p, err := agents.LoadPrompt(criticKey+"_flash", version); err == nil {
			return p, nil
		}
	}
	return agents.LoadPrompt(criticKey, version)
}
