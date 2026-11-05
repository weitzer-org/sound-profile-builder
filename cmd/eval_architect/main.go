package main

// eval_architect is Phase 1B of docs/eval/gemini-model-upgrade-test-plan.md: known-regression
// repros on the Architect (agent 12) with everything upstream held identical.
//
// Stage "freeze": run agents 1-11 ONCE per query on baseline models through the real RunPipeline
// (so every prompt is built by production code), capture the exact Architect system+user prompt
// and every upstream output, and stop (context cancelled) the instant the Architect would be
// called. No Architect or Critic money is spent. Files are reused by later runs; an existing
// valid frozen file is never re-spent unless -refreeze.
//
// Stage "run": for each (model, query, rep) call ONLY the Architect with the frozen prompts, then
// the Critic on baseline (gemini-3.5-flash), apply the Critic as production does, and score with
// internal/evalcheck. The control model (first of -models) runs first; if it does not pass R1/R2
// the check or the freeze is broken and the tool STOPS before spending on candidates.
//
// Safeguards: -dry-run defaults to true; live runs require EVAL_NO_FALLBACK=true.

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"google.golang.org/genai"

	"github.com/weitzer-org/sound-builder/internal/agents"
	"github.com/weitzer-org/sound-builder/internal/config"
	"github.com/weitzer-org/sound-builder/internal/evalcheck"
	"github.com/weitzer-org/sound-builder/internal/evalfixtures"
)

// Frozen is the on-disk freeze of one query.
type Frozen struct {
	Query           string              `json:"query"`
	QueryText       string              `json:"query_text"`
	FrozenAt        string              `json:"frozen_at"`
	Routing         map[string]string   `json:"routing"`
	ArchitectPrompt string              `json:"architect_prompt_version"`
	ArchSystem      string              `json:"architect_system_prompt"`
	ArchUser        string              `json:"architect_user_prompt"`
	Upstream        []UpstreamOutput    `json:"upstream"`
	Calls           []agents.CallRecord `json:"upstream_calls"`
	IntegrityOK     bool                `json:"integrity_ok"`
	Issues          []string            `json:"integrity_issues,omitempty"`
}

func frozenPath(dir, q string) string { return filepath.Join(dir, q+".json") }

func loadFrozen(dir, q string) (*Frozen, error) {
	b, err := os.ReadFile(frozenPath(dir, q))
	if err != nil {
		return nil, err
	}
	var f Frozen
	if err := json.Unmarshal(b, &f); err != nil {
		return nil, err
	}
	if !f.IntegrityOK || f.ArchUser == "" || f.ArchSystem == "" {
		return nil, fmt.Errorf("%s is not a valid freeze (integrity_ok=%v)", frozenPath(dir, q), f.IntegrityOK)
	}
	return &f, nil
}

func main() {
	stage := flag.String("stage", "all", "freeze | run | all")
	queriesFlag := flag.String("queries", strings.Join(RuleQueries(), ","), "queries to freeze and run (R1: 07_Edge; R2: 09_BB_King, 05_Clapton)")
	modelsFlag := flag.String("models", strings.Join([]string{modelProBaseline, modelArgon, modelFlash38}, ","), "Architect models; the FIRST is the control")
	reps := flag.Int("reps", 5, "repetitions per (model, query)")
	dryRun := flag.Bool("dry-run", true, "print the exact call grid and exit; pass -dry-run=false to spend money")
	refreeze := flag.Bool("refreeze", false, "re-run the upstream freeze even if a valid frozen file exists")
	freezeAttempts := flag.Int("freeze-attempts", 2, "max pipeline attempts per query when an upstream agent fails")
	frozenDir := flag.String("frozen-dir", filepath.Join("eval_results", "phase1", "architect", "frozen"), "where frozen upstream files live")
	outDir := flag.String("out", "", "run output directory (default eval_results/phase1/architect/<UTC timestamp>); one with runs.jsonl is refused")
	maxCalls := flag.Int("max-calls", 0, "hard cap on live API calls for this invocation (0 = the planned maximum); refuses to start above it")
	noStop := flag.Bool("no-stop-on-control-failure", false, "keep spending on candidates even if the control fails (results are then NOT conclusions)")
	archPrompt := flag.String("architect-prompt", "", "Architect prompt version to test instead of the frozen production one (e.g. v5); the frozen upstream user prompt is unchanged")
	thinkingBudget := flag.Int("thinking-budget", -1, "ThinkingBudget for every call this tool makes (-1 = unset, per-agent defaults apply); used to check the budget is enforced")
	thinkingLevel := flag.String("thinking-level", "", "ThinkingLevel for the Architect call only (MINIMAL|LOW|MEDIUM|HIGH); empty = provider default, as in production")
	allowFallback := flag.Bool("allow-fallback", false, "permit a live run without EVAL_NO_FALLBACK=true (NOT scoreable)")
	flag.Parse()

	if *reps < 1 || *freezeAttempts < 1 {
		log.Fatal("-reps and -freeze-attempts must be >= 1")
	}
	if *stage != "freeze" && *stage != "run" && *stage != "all" {
		log.Fatalf("-stage must be freeze, run or all")
	}
	golden := evalfixtures.GoldenQueries()
	var queries []string
	for _, q := range strings.Split(*queriesFlag, ",") {
		if q = strings.TrimSpace(q); q != "" {
			if _, ok := golden[q]; !ok {
				log.Fatalf("unknown query %q", q)
			}
			queries = append(queries, q)
		}
	}
	var models []string
	for _, m := range strings.Split(*modelsFlag, ",") {
		if m = strings.TrimSpace(m); m != "" {
			models = append(models, m)
		}
	}
	if len(queries) == 0 || len(models) == 0 {
		log.Fatal("need at least one query and one model")
	}
	control := models[0]

	// ---- plan ----
	var toFreeze []string
	for _, q := range queries {
		if _, err := loadFrozen(*frozenDir, q); err != nil || *refreeze {
			toFreeze = append(toFreeze, q)
		}
	}
	const upstreamAgents = 10 // agents 1-11 minus the bypassed Cloud Navigator, one attempt each
	freezeMin, freezeMax := 0, 0
	if *stage != "run" {
		freezeMin, freezeMax = len(toFreeze)*upstreamAgents, len(toFreeze)*upstreamAgents*(*freezeAttempts)
	}
	runJobs := 0
	if *stage != "freeze" {
		runJobs = len(models) * len(queries) * (*reps)
	}
	fmt.Printf("Plan (stage=%s): queries=%v models=%v (control=%s) reps=%d\n", *stage, queries, models, control, *reps)
	if *stage != "run" {
		fmt.Printf("  freeze: %d of %d queries need freezing %v (already frozen: reused, 0 calls)\n", len(toFreeze), len(queries), toFreeze)
		fmt.Printf("          = %d upstream calls (10 API-calling upstream agents x %d queries); up to %d if every query needs its %d-attempt retry\n", freezeMin, len(toFreeze), freezeMax, *freezeAttempts)
	}
	if *stage != "freeze" {
		fmt.Printf("  run:    %d models x %d queries x %d reps = %d Architect calls, + up to %d Critic calls (one per successful Architect call; Critic on %s)\n", len(models), len(queries), *reps, runJobs, runJobs, modelFlashBaseline)
	}
	plannedMax := freezeMax + 2*runJobs
	fmt.Printf("  TOTAL: %d expected if every call succeeds, %d planned-maximum API calls\n", freezeMin+runJobs+runJobs, plannedMax)
	if *dryRun {
		fmt.Println("DRY RUN -- no Gemini calls made. Pass -dry-run=false to execute.")
		return
	}
	if *maxCalls > 0 && plannedMax > *maxCalls {
		log.Fatalf("planned maximum %d calls is above -max-calls %d; refusing to start", plannedMax, *maxCalls)
	}
	if !agents.EvalNoFallbackEnabled() && !*allowFallback {
		log.Fatalf("%s=true is required for live runs. Set it, or pass -allow-fallback (NOT scoreable).", agents.EvalNoFallbackEnv)
	}
	if set := agents.AblationEnvSet(); len(set) > 0 {
		log.Fatalf("%v set: those agents would return a stub without calling a model; unset them for a live eval", set)
	}
	if os.Getenv("TARGET_MODEL") != "" {
		log.Fatal("TARGET_MODEL is set; unset it")
	}
	key := os.Getenv("GEMINI_API_KEY")
	if key == "" {
		log.Fatal("GEMINI_API_KEY must be set to run live evals")
	}
	cfg, err := config.LoadConfig("config.json")
	if err != nil {
		log.Fatalf("load config: %v", err)
	}
	ctx := context.Background()
	orch, err := agents.NewOrchestrator(ctx, key, nil)
	if err != nil {
		log.Fatalf("init orchestrator: %v", err)
	}
	defer orch.Close()
	if *thinkingBudget >= 0 {
		b := int32(*thinkingBudget)
		orch.ThinkingBudget = &b
	}

	// ---- freeze ----
	if *stage != "run" {
		if err := os.MkdirAll(*frozenDir, 0o755); err != nil {
			log.Fatal(err)
		}
		for _, q := range toFreeze {
			if err := freezeQuery(ctx, orch, cfg, *frozenDir, q, golden[q], *freezeAttempts); err != nil {
				log.Fatalf("freeze %s: %v", q, err)
			}
		}
	}
	if *stage == "freeze" {
		return
	}

	// ---- run ----
	frozen := map[string]*Frozen{}
	for _, q := range queries {
		f, err := loadFrozen(*frozenDir, q)
		if err != nil {
			log.Fatalf("no usable freeze for %s (run -stage freeze): %v", q, err)
		}
		frozen[q] = f
	}
	if *outDir == "" {
		*outDir = filepath.Join("eval_results", "phase1", "architect", time.Now().UTC().Format("20060102T150405Z"))
	}
	runsPath := filepath.Join(*outDir, "runs.jsonl")
	if _, err := os.Stat(runsPath); err == nil {
		log.Fatalf("%s already exists; pick a fresh -out", runsPath)
	}
	if err := os.MkdirAll(*outDir, 0o755); err != nil {
		log.Fatal(err)
	}
	rf, err := os.OpenFile(runsPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		log.Fatal(err)
	}
	defer rf.Close()

	valid := agents.BuildEffectiveValidBlocks(true, true)
	var results []RunResult
	jobs := BuildJobs(models, queries, *reps)
	stopped := false
	for i, j := range jobs {
		// Gate: once the control's runs are all done, require it to pass before any candidate spend.
		if !*noStop && j.Model != control && (i == 0 || jobs[i-1].Model == control) {
			ok, why := ControlOK(JudgeFor(results, []string{control}, *reps, queries), control)
			if !ok {
				log.Printf("STOP: control %s did not pass R1/R2 -- the check or the freeze is broken, or the control has the defect. No candidate was run. %v", control, why)
				stopped = true
				break
			}
		}
		res := runOne(ctx, orch, cfg, valid, frozen[j.Query], j, *outDir, *archPrompt, genai.ThinkingLevel(strings.ToUpper(*thinkingLevel)))
		results = append(results, res)
		if b, err := json.Marshal(res); err == nil {
			fmt.Fprintln(rf, string(b))
		}
		log.Printf("[%d/%d] %s %s rep%d arch_ok=%v finish=%s served=%s scoreable=%v defect=%v", i+1, len(jobs), j.Model, j.Query, j.Rep, res.ArchSuccess, res.ArchFinish, res.ArchServed, res.Scoreable, res.CaptureDefect)
	}
	md := Markdown(results, models, *reps, queries)
	if stopped {
		md = "**STOPPED: the control did not pass; no candidate conclusions.**\n\n" + md
	}
	if err := os.WriteFile(filepath.Join(*outDir, "summary.md"), []byte(md), 0o644); err != nil {
		log.Printf("write summary: %v", err)
	}
	fmt.Println(md)
	if stopped {
		os.Exit(2)
	}
}

func freezeQuery(ctx context.Context, orch *agents.Orchestrator, cfg *config.AppConfig, dir, name, queryText string, attempts int) error {
	routing := BaselineRouting()
	var lastIssues []string
	for a := 1; a <= attempts; a++ {
		log.Printf("freeze %s attempt %d/%d ...", name, a, attempts)
		cctx, cancel := context.WithCancel(ctx)
		obs := &freezeObserver{cancel: cancel}
		orch.AgentModels = routing
		orch.Usage = &agents.TokenUsage{ModelsUsed: map[string]int{}}
		_, _, runErr := orch.RunPipeline(agents.WithRunObserver(cctx, obs), queryText, evalfixtures.DefaultConstraints(), cfg.AgentPrompts, nil)
		cancel()
		var upCalls []agents.CallRecord
		for _, c := range orch.Usage.CallsSnapshot() {
			if c.Agent != architectRole { // the cancelled Architect attempt is expected and never reached the API
				upCalls = append(upCalls, c)
			}
		}
		ok, issues, _ := evalcheck.AssessCalls(upCalls, routing, pins)
		if !obs.Captured() {
			ok = false
			issues = append(issues, fmt.Sprintf("pipeline never reached the Architect: %v", runErr))
		}
		if missing := missingUpstreamCalls(upCalls); len(missing) > 0 {
			ok = false
			issues = append(issues, fmt.Sprintf("no successful model call recorded for: %s", strings.Join(missing, ", ")))
		}
		if missing := missingUpstream(obs.Upstream); len(missing) > 0 {
			ok = false
			issues = append(issues, fmt.Sprintf("missing upstream outputs from: %s", strings.Join(missing, ", ")))
		}
		if !ok {
			lastIssues = issues
			log.Printf("freeze %s attempt %d unusable: %s", name, a, strings.Join(issues, "; "))
			continue
		}
		f := Frozen{Query: name, QueryText: queryText, FrozenAt: time.Now().UTC().Format(time.RFC3339), Routing: routing,
			ArchitectPrompt: cfg.AgentPrompts[architectKey], ArchSystem: obs.ArchSystem, ArchUser: obs.ArchUser,
			Upstream: obs.Upstream, Calls: upCalls, IntegrityOK: true}
		b, err := json.MarshalIndent(f, "", "  ")
		if err != nil {
			return err
		}
		return os.WriteFile(frozenPath(dir, name), b, 0o644)
	}
	return fmt.Errorf("no usable freeze after %d attempt(s): %s", attempts, strings.Join(lastIssues, "; "))
}

// requiredUpstream are the agents that always call the API. Cloud Navigator (5) is bypassed with
// a canned string when allow_cloud_captures is false (it is in evalfixtures.DefaultConstraints),
// so it makes no call and has no frozen output; its text is still inside the frozen Architect prompt.
var requiredUpstream = []string{"Tone Historian", "Sonic Profiler", "Community Scraper", "CorOS Librarian",
	"Acoustician", "Transducer Tech", "FOH Optimizer", "Mix Engineer", "Control Mapper", "DSP Dispatcher"}

// missingUpstreamCalls returns the required upstream agents with no successful model call. An
// output alone is not enough: an ablated agent (ABLATE_AGENT_N=true) produces a stub output with
// no call, which missingUpstream would accept.
func missingUpstreamCalls(calls []agents.CallRecord) []string {
	ok := map[string]bool{}
	for _, c := range calls {
		if c.Success {
			ok[c.Agent] = true
		}
	}
	var miss []string
	for _, r := range requiredUpstream {
		if !ok[r] {
			miss = append(miss, r)
		}
	}
	return miss
}

func missingUpstream(got []UpstreamOutput) []string {
	have := map[string]bool{}
	for _, u := range got {
		have[u.Role] = true
	}
	var miss []string
	for _, r := range requiredUpstream {
		if !have[r] {
			miss = append(miss, r)
		}
	}
	return miss
}

func runOne(ctx context.Context, orch *agents.Orchestrator, cfg *config.AppConfig, valid map[string]bool, fz *Frozen, j Job, outDir, archPrompt string, level genai.ThinkingLevel) RunResult {
	res := RunResult{Model: j.Model, Query: j.Query, Rep: j.Rep, ArchPromptVersion: cfg.AgentPrompts[architectKey], ThinkingLevel: string(level)}
	routing := map[string]string{architectKey: j.Model, criticKey: modelFlashBaseline}
	orch.AgentModels = routing
	orch.Usage = &agents.TokenUsage{ModelsUsed: map[string]int{}}

	sys, err := orch.PromptForAgent(architectKey, cfg.AgentPrompts[architectKey])
	res.PromptMatches = err == nil && sys == fz.ArchSystem
	if err != nil || !res.PromptMatches {
		// Production would send a different system prompt for this model than the one frozen. Use
		// the frozen one so the only variable is the model, and flag it.
		sys = fz.ArchSystem
	}
	if archPrompt != "" {
		// Deliberate prompt experiment, loaded through the same production selector.
		p, perr := orch.PromptForAgent(architectKey, archPrompt)
		if perr != nil || p == fz.ArchSystem {
			res.ArchError = fmt.Sprintf("architect prompt %q unusable or identical to the frozen one: %v", archPrompt, perr)
			return res
		}
		sys, res.ArchPromptVersion, res.PromptMatches = p, archPrompt, false
	}
	orch.ThinkingLevel = level // Architect call only; the Critic below stays on production settings
	out, aerr := orch.RunAgentSplit(ctx, architectRole, sys, fz.ArchUser)
	orch.ThinkingLevel = ""
	archCalls := orch.Usage.CallsSnapshot()
	if len(archCalls) > 0 {
		c := archCalls[len(archCalls)-1]
		res.ArchServed, res.ArchFinish, res.ArchSuccess = c.ServedModel, c.FinishReason, c.Success
		res.ArchLatencyMs, res.ArchInTokens, res.ArchOutTokens, res.ArchThinkTokens = c.LatencyMs, c.InputTokens, c.OutputTokens, c.ThinkingTokens
		res.ArchError = c.Error
	}
	if aerr != nil {
		res.ArchSuccess = false
		if res.ArchError == "" {
			res.ArchError = aerr.Error()
		}
	}
	res.ArchIntegrityOK = len(archCalls) == 1 && archCalls[0].AttemptModel == j.Model && archCalls[0].Success &&
		agents.ServedModelMatches(expectedServed(j.Model), archCalls[0].ServedModel)

	final := out
	if aerr == nil {
		csys, cerr := orch.PromptForAgent(criticKey, cfg.AgentPrompts[criticKey])
		if cerr == nil {
			cout, critErr := orch.RunAgentSplit(ctx, criticRole, csys, out)
			if critErr == nil {
				final = agents.ApplyCriticFindings(out, cout)
			}
		} else {
			res.CriticError = cerr.Error()
		}
	}
	all := orch.Usage.CallsSnapshot()
	for _, c := range all {
		if c.Agent == criticRole {
			res.CriticServed, res.CriticFinish, res.CriticSuccess = c.ServedModel, c.FinishReason, c.Success
			if c.Error != "" {
				res.CriticError = c.Error
			}
		}
	}
	ok, issues, mt := evalcheck.AssessCalls(all, routing, pins)
	res.IntegrityIssues, res.MaxTokens = issues, mt
	_ = ok
	res.CriticIntegrity = res.CriticSuccess && !criticIssue(issues)

	if aerr == nil {
		rep := evalcheck.Check(final, valid)
		res.Report = &rep
		res.Scoreable = res.ArchIntegrityOK && rep.ParseError == ""
		res.CaptureDefect = res.Scoreable && Defect(rep)
	}

	dir := filepath.Join(outDir, strings.ReplaceAll(j.Model, "/", "_"), j.Query)
	if err := os.MkdirAll(dir, 0o755); err == nil {
		base := filepath.Join(dir, fmt.Sprintf("rep%d", j.Rep))
		if aerr == nil {
			res.OutputFile = base + ".final.json"
			_ = os.WriteFile(base+".architect.json", []byte(out), 0o644)
			_ = os.WriteFile(res.OutputFile, []byte(final), 0o644)
		}
		res.CallsFile = base + ".calls.json"
		if b, err := json.MarshalIndent(all, "", "  "); err == nil {
			_ = os.WriteFile(res.CallsFile, b, 0o644)
		}
	}
	return res
}

func criticIssue(issues []string) bool {
	for _, s := range issues {
		if strings.HasPrefix(s, criticRole) {
			return true
		}
	}
	return false
}

func expectedServed(model string) string {
	if p, ok := pins[model]; ok {
		return p
	}
	return model
}
