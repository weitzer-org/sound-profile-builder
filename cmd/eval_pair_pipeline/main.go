// eval_pair_pipeline runs the real 13-agent pipeline for one prompt-order condition and
// records everything needed to compare it, run by run, with another condition:
// scores from the owner-approved checklist (on the Librarian's output AND the final
// preset), mechanical quality defects, token use (incl. cached), cost and latency.
//
// Pairing: pass the same -phase1-dir to both conditions. Whichever condition runs first
// writes each (query, sample)'s Phase 1 output there; the second reuses it byte for byte,
// so Phase 1's sampling noise cannot masquerade as a difference between conditions. Run the
// "new" condition first.
//
// The "old" condition has no switch in production code: build this tool from a worktree
// where buildLibrarianContext's format string is reverted to per-run-text-first (see
// docs/plans/2026-11-librarian-quality-plan.md, workstream 3).
//
// Makes live, PAID Gemini calls. -dry-run shows the plan; -max-usd is a hard ceiling.
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/weitzer-org/sound-builder/internal/agents"
	"github.com/weitzer-org/sound-builder/internal/config"
	"github.com/weitzer-org/sound-builder/internal/evalfixtures"
	"github.com/weitzer-org/sound-builder/internal/evalscore"
)

// capture records agent outputs and per-agent usage. Safe for concurrent use.
type capture struct {
	mu        sync.Mutex
	librarian string
	usage     map[string]agents.AgentUsage
}

func (c *capture) OnPhase(string)                         {}
func (c *capture) OnAgentStart(string, string, string)    {}
func (c *capture) OnAttempt(string, string, int64, error) {}
func (c *capture) OnInfo(string)                          {}
func (c *capture) OnAgentDone(role, model, output string, u agents.AgentUsage, err error) {
	if err != nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if role == "CorOS Librarian" {
		c.librarian = output
	}
	if c.usage == nil {
		c.usage = map[string]agents.AgentUsage{}
	}
	prev := c.usage[role]
	prev.InputTokens += u.InputTokens
	prev.CachedInputTokens += u.CachedInputTokens
	prev.OutputTokens += u.OutputTokens
	prev.ThinkingTokens += u.ThinkingTokens
	prev.Model = u.Model
	c.usage[role] = prev
}

type unit struct {
	query  string
	sample int
}

func main() {
	var (
		condition = flag.String("condition", "", "label for this condition, e.g. new or old (required)")
		out       = flag.String("out", "", "output dir (default eval_results/runs/pair-<condition>-<UTC timestamp>)")
		p1Dir     = flag.String("phase1-dir", "", "shared frozen Phase 1 dir (required)")
		samples   = flag.Int("samples", 5, "samples per query")
		queryList = flag.String("queries", "", "comma-separated golden query names (default: all 13)")
		workers   = flag.Int("workers", 2, "parallel pipelines (each with its own orchestrator)")
		maxUSD    = flag.Float64("max-usd", 45, "abort when estimated spend exceeds this")
		dryRun    = flag.Bool("dry-run", false, "print the plan only; no API calls")
	)
	flag.Parse()
	if *condition == "" || *p1Dir == "" {
		log.Fatal("-condition and -phase1-dir are required")
	}
	order := evalfixtures.GoldenQueryOrder()
	if *queryList != "" {
		order = strings.Split(*queryList, ",")
	}
	if *out == "" {
		*out = filepath.Join("eval_results", "runs", "pair-"+*condition+"-"+time.Now().UTC().Format("20060102-150405"))
	}
	n := len(order) * *samples
	fmt.Printf("plan: condition=%q, %d queries x %d samples = %d full pipelines (production routing). rough cost ~$%.0f (unmeasured; ~$0.31/pipeline on 2026-11-04). out=%s\n", *condition, len(order), *samples, n, float64(n)*0.31, *out)
	if *dryRun {
		return
	}

	key := os.Getenv("GEMINI_API_KEY")
	if key == "" {
		log.Fatal("GEMINI_API_KEY must be set")
	}
	cfg, err := config.LoadConfig("config.json")
	if err != nil {
		log.Fatal(err)
	}
	scorer, err := evalscore.New()
	if err != nil {
		log.Fatal(err)
	}
	for _, d := range []string{*out, *p1Dir} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			log.Fatal(err)
		}
	}
	validBlocks := agents.BuildEffectiveValidBlocks(true, true)
	queries := evalfixtures.GoldenQueries()
	constraints := evalfixtures.DefaultConstraints()
	ctx := context.Background()

	// Resume: completed runs are kept; failed ones are retried.
	manifest := filepath.Join(*out, "manifest.jsonl")
	done := map[unit]bool{}
	var spent float64
	if f, err := os.Open(manifest); err == nil {
		sc := bufio.NewScanner(f)
		sc.Buffer(make([]byte, 1<<20), 1<<24)
		for sc.Scan() {
			var r evalscore.PipelineRun
			if json.Unmarshal(sc.Bytes(), &r) == nil && r.OK {
				done[unit{r.Query, r.Sample}] = true
				spent += r.CostUSD
			}
		}
		f.Close()
		log.Printf("resuming: %d completed runs", len(done))
	}
	mf, err := os.OpenFile(manifest, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		log.Fatal(err)
	}
	defer mf.Close()

	var mu sync.Mutex
	aborted := false
	ch := make(chan unit)
	var wg sync.WaitGroup
	for w := 0; w < *workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			orch, err := agents.NewOrchestrator(ctx, key, nil)
			if err != nil {
				log.Fatal(err)
			}
			for u := range ch {
				mu.Lock()
				stop := aborted
				mu.Unlock()
				if stop {
					continue
				}
				rec := runOne(ctx, orch, cfg, scorer, validBlocks, queries[u.query], constraints, *condition, *p1Dir, *out, u)
				mu.Lock()
				spent += rec.CostUSD
				b, _ := json.Marshal(rec)
				mf.Write(append(b, '\n'))
				if spent > *maxUSD {
					aborted = true
					log.Printf("ABORT: estimated spend $%.2f exceeded -max-usd=%.0f", spent, *maxUSD)
				}
				log.Printf("%s %s s%d ok=%v lat=%.0fs in=%d cached=%d spend=$%.2f", *condition, u.query, u.sample, rec.OK, rec.LatencySec, rec.In, rec.Cached, spent)
				mu.Unlock()
			}
		}()
	}
	for _, q := range order {
		for s := 0; s < *samples; s++ {
			if !done[unit{q, s}] {
				ch <- unit{q, s}
			}
		}
	}
	close(ch)
	wg.Wait()
	fmt.Printf("done. manifest: %s (estimated spend $%.2f, aborted=%v)\n", manifest, spent, aborted)
}

func runOne(ctx context.Context, orch *agents.Orchestrator, cfg *config.AppConfig, scorer *evalscore.Scorer, validBlocks map[string]bool,
	query string, constraints map[string]interface{}, condition, p1Dir, out string, u unit) evalscore.PipelineRun {

	rec := evalscore.PipelineRun{Condition: condition, Query: u.query, Sample: u.sample}
	p1Path := filepath.Join(p1Dir, fmt.Sprintf("%s_s%d.json", u.query, u.sample))
	rctx := ctx
	var gen agents.Phase1Result
	if b, err := os.ReadFile(p1Path); err == nil {
		var p agents.Phase1Result
		if json.Unmarshal(b, &p) == nil && p.Tone != "" {
			rctx = agents.WithPhase1Override(rctx, p)
			rec.Phase1Reused = true
		}
	}
	if !rec.Phase1Reused {
		rctx = agents.WithPhase1Capture(rctx, &gen)
	}
	cap := &capture{}
	rctx = agents.WithRunObserver(rctx, cap)

	orch.Usage = &agents.TokenUsage{ModelsUsed: map[string]int{}}
	start := time.Now()
	html, usage, err := orch.RunPipeline(rctx, query, constraints, cfg.AgentPrompts, nil)
	rec.LatencySec = time.Since(start).Seconds()

	cap.mu.Lock()
	rec.PerAgent = map[string]evalscore.AgentTokens{}
	for role, au := range cap.usage {
		cost := agents.EstimateCostUSD(au.Model, au.InputTokens, au.CachedInputTokens, au.OutputTokens, au.ThinkingTokens)
		rec.PerAgent[role] = evalscore.AgentTokens{Model: au.Model, In: au.InputTokens, Cached: au.CachedInputTokens, Out: au.OutputTokens, Thinking: au.ThinkingTokens, CostUSD: cost}
		rec.CostUSD += cost
		if !evalscore.Phase1Roles[role] {
			rec.CostExP1USD += cost
		}
	}
	librarian := cap.librarian
	cap.mu.Unlock()
	if usage != nil {
		rec.In, rec.Cached, rec.Out, rec.Thinking = usage.InputTokens, usage.CachedInputTokens, usage.OutputTokens, usage.ThinkingTokens
		rec.ModelsUsed = map[string]int{}
		for k, v := range usage.ModelsUsed {
			rec.ModelsUsed[k] = v
		}
	}
	if err != nil {
		rec.Err = err.Error()
		return rec
	}
	rec.OK = true

	// Persist the Phase 1 output so the other condition can reuse it.
	if !rec.Phase1Reused && gen.Tone != "" {
		b, _ := json.MarshalIndent(gen, "", "  ")
		_ = os.WriteFile(p1Path, b, 0o644)
	}

	dir := filepath.Join(out, fmt.Sprintf("s%d", u.sample))
	_ = os.MkdirAll(dir, 0o755)
	rec.HTMLPath = filepath.Join(dir, u.query+".html")
	_ = os.WriteFile(rec.HTMLPath, []byte(html), 0o644)
	if librarian != "" {
		rec.LibrarianPath = filepath.Join(dir, u.query+".librarian.json")
		_ = os.WriteFile(rec.LibrarianPath, []byte(librarian), 0o644)
		if names, mand, perr := evalscore.LibrarianNames(librarian); perr == nil {
			if sc, serr := scorer.Score(u.query, names, mand); serr == nil {
				rec.LibScore = &sc
			}
		}
	}
	if sc, serr := scorer.Score(u.query, evalscore.PresetModels(html), nil); serr == nil {
		rec.PresetScore = &sc
	}
	mech := agents.RunMechanicalQualityChecks(orch.LastArchitectJSON(), validBlocks)
	rec.MechParseErr, rec.MechDefects = mech.ParseError, mech.TotalDefects()
	return rec
}
