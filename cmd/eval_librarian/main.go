// eval_librarian is a paired, Librarian-only quality A/B: does putting the static
// catalog FIRST in the CorOS Librarian's prompt (the cache-friendly order) change what
// the Librarian picks, compared with the old per-run-text-first order?
//
// Design (see docs/plans/2026-11-librarian-quality-plan.md):
//   - Phase 1 (Tone Historian, Sonic Profiler, Community Scraper) is run ONCE per
//     (query, sample) and frozen on disk, so both orders see byte-identical inputs.
//   - Each (query, Phase 1 sample) is a unit; each unit gets N repeats per order.
//   - Output is scored by the owner-approved checklist
//     (internal/evalfixtures/librarian_checklist.json) -- deterministic, no LLM judge.
//   - The decision rule (non-inferiority margin on the hard-rule pass fraction) is fixed
//     in code, before any data exists. See internal/evalscore/compare.go.
//
// This makes live, PAID Gemini calls. Use -dry-run to see the plan first. -max-usd is a
// hard ceiling on estimated spend.
//
//	go run ./cmd/eval_librarian -dry-run
//	GEMINI_API_KEY=... go run ./cmd/eval_librarian -out eval_results/runs/librarian-1
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/weitzer-org/sound-builder/internal/agents"
	"github.com/weitzer-org/sound-builder/internal/config"
	"github.com/weitzer-org/sound-builder/internal/evalfixtures"
	"github.com/weitzer-org/sound-builder/internal/evalscore"
)

type job struct {
	query string
	p1    int
	order string
	rep   int
}

func (j job) key() string { return fmt.Sprintf("%s|%d|%s|%d", j.query, j.p1, j.order, j.rep) }

func main() {
	var (
		samples   = flag.Int("reps", 5, "Librarian repeats per (query, Phase 1 sample, order)")
		p1Samples = flag.Int("phase1-samples", 2, "independent Phase 1 samples per query")
		queryList = flag.String("queries", "", "comma-separated golden query names (default: all 13)")
		out       = flag.String("out", "", "output dir (default eval_results/runs/librarian-<UTC timestamp>)")
		p1Dir     = flag.String("phase1-dir", "", "frozen Phase 1 dir (default <out>/phase1); reused if files exist")
		workers   = flag.Int("workers", 3, "parallel workers (each has its own orchestrator)")
		maxUSD    = flag.Float64("max-usd", 30, "abort when estimated spend exceeds this")
		margin    = flag.Float64("margin", 0.05, "non-inferiority margin on hard-rule pass fraction")
		dryRun    = flag.Bool("dry-run", false, "print the plan and a rough cost estimate; make no API calls")
	)
	flag.Parse()

	order := evalfixtures.GoldenQueryOrder()
	if *queryList != "" {
		order = strings.Split(*queryList, ",")
	}
	if *out == "" {
		*out = filepath.Join("eval_results", "runs", "librarian-"+time.Now().UTC().Format("20060102-150405"))
	}
	if *p1Dir == "" {
		*p1Dir = filepath.Join(*out, "phase1")
	}
	nLib := len(order) * *p1Samples * 2 * *samples
	nP1 := len(order) * *p1Samples
	// Rough per-call estimates from the 2026-11-04 runs (~38K input tokens for the
	// Librarian, a few thousand for Phase 1). Unmeasured for this tool: an estimate only.
	est := float64(nLib)*0.08 + float64(nP1)*0.05
	fmt.Printf("plan: %d queries x %d Phase 1 samples x 2 orders x %d reps = %d Librarian calls, %d Phase 1 runs\n", len(order), *p1Samples, *samples, nLib, nP1)
	fmt.Printf("rough cost estimate: ~$%.0f (unmeasured; ceiling -max-usd=%.0f). out=%s\n", est, *maxUSD, *out)
	if *dryRun {
		return
	}

	key := os.Getenv("GEMINI_API_KEY")
	if key == "" {
		log.Fatal("GEMINI_API_KEY must be set")
	}
	cfg, err := config.LoadConfig("config.json")
	if err != nil {
		log.Fatalf("config: %v", err)
	}
	scorer, err := evalscore.New()
	if err != nil {
		log.Fatalf("scorer: %v", err)
	}
	if err := os.MkdirAll(*p1Dir, 0o755); err != nil {
		log.Fatal(err)
	}
	queries := evalfixtures.GoldenQueries()
	constraints := evalfixtures.DefaultConstraints()
	ctx := context.Background()

	newOrch := func() *agents.Orchestrator {
		o, err := agents.NewOrchestrator(ctx, key, nil)
		if err != nil {
			log.Fatalf("orchestrator: %v", err)
		}
		return o
	}

	// 1. Freeze Phase 1.
	frozen := map[string]agents.Phase1Result{}
	p1orch := newOrch()
	for _, q := range order {
		for p := 0; p < *p1Samples; p++ {
			path := filepath.Join(*p1Dir, fmt.Sprintf("%s_p%d.json", q, p))
			if b, err := os.ReadFile(path); err == nil {
				var r agents.Phase1Result
				if json.Unmarshal(b, &r) == nil && r.Tone != "" && r.LibrarianStatic != "" {
					frozen[fmt.Sprintf("%s|%d", q, p)] = r
					continue
				}
			}
			var got agents.Phase1Result
			var lastErr error
			for attempt := 0; attempt < 3; attempt++ {
				pctx := agents.WithStopAfterPhase1(agents.WithPhase1Capture(ctx, &got))
				_, _, lastErr = p1orch.RunPipeline(pctx, queries[q], constraints, cfg.AgentPrompts, nil)
				if errors.Is(lastErr, agents.ErrStoppedAfterPhase1) {
					lastErr = nil
					break
				}
			}
			if lastErr != nil {
				log.Printf("phase1 failed for %s p%d: %v (unit skipped)", q, p, lastErr)
				continue
			}
			b, _ := json.MarshalIndent(got, "", "  ")
			if err := os.WriteFile(path, b, 0o644); err != nil {
				log.Fatal(err)
			}
			frozen[fmt.Sprintf("%s|%d", q, p)] = got
			log.Printf("phase1 frozen: %s p%d", q, p)
		}
	}

	// 2. Build the job list, interleaving orders so time-of-day drift hits both equally.
	runsPath := filepath.Join(*out, "runs.jsonl")
	done := map[string]bool{}
	var prior []evalscore.LibrarianRun
	if f, err := os.Open(runsPath); err == nil {
		sc := bufio.NewScanner(f)
		sc.Buffer(make([]byte, 1<<20), 1<<24)
		for sc.Scan() {
			var r evalscore.LibrarianRun
			if json.Unmarshal(sc.Bytes(), &r) == nil && r.Err == "" {
				prior = append(prior, r)
				done[job{r.Query, r.Phase1, r.Order, r.Rep}.key()] = true
			}
		}
		f.Close()
		log.Printf("resuming: %d completed runs found in %s", len(prior), runsPath)
	}
	var jobs []job
	for _, q := range order {
		for p := 0; p < *p1Samples; p++ {
			if _, ok := frozen[fmt.Sprintf("%s|%d", q, p)]; !ok {
				continue
			}
			for rep := 0; rep < *samples; rep++ {
				orders := []string{"new", "old"}
				if rep%2 == 1 {
					orders = []string{"old", "new"}
				}
				for _, o := range orders {
					j := job{q, p, o, rep}
					if !done[j.key()] {
						jobs = append(jobs, j)
					}
				}
			}
		}
	}
	log.Printf("%d Librarian calls to run", len(jobs))

	// 3. Run.
	rf, err := os.OpenFile(runsPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		log.Fatal(err)
	}
	defer rf.Close()
	var mu sync.Mutex
	runs := append([]evalscore.LibrarianRun{}, prior...)
	spent := 0.0
	for _, r := range prior {
		spent += r.CostUSD
	}
	aborted := false
	ch := make(chan job)
	var wg sync.WaitGroup
	for w := 0; w < *workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			o := newOrch()
			for j := range ch {
				mu.Lock()
				stop := aborted
				mu.Unlock()
				if stop {
					continue
				}
				p1 := frozen[fmt.Sprintf("%s|%d", j.query, j.p1)]
				var userCtx string
				if j.order == "new" {
					userCtx = agents.LibrarianContextNewOrder(p1.LibrarianStatic, p1.Tone, p1.Scrape, constraints)
				} else {
					userCtx = agents.LibrarianContextOldOrder(p1.LibrarianStatic, p1.Tone, p1.Scrape, constraints)
				}
				before := snapshot(o)
				start := time.Now()
				output, err := o.RunLibrarian(ctx, cfg.AgentPrompts, userCtx)
				rec := evalscore.LibrarianRun{Query: j.query, Phase1: j.p1, Order: j.order, Rep: j.rep, Latency: time.Since(start).Milliseconds()}
				after := snapshot(o)
				rec.In, rec.Cached, rec.Out, rec.Thinking = after.in-before.in, after.cached-before.cached, after.out-before.out, after.think-before.think
				rec.CostUSD = agents.EstimateCostUSD(after.model, rec.In, rec.Cached, rec.Out, rec.Thinking)
				if err != nil {
					rec.Err = err.Error()
				} else if names, mand, perr := evalscore.LibrarianNames(output); perr != nil {
					rec.ParseOK = false
				} else {
					rec.ParseOK, rec.Names = true, names
					if sc, serr := scorer.Score(j.query, names, mand); serr == nil {
						rec.Score = &sc
					}
					_ = os.MkdirAll(filepath.Join(*out, "outputs"), 0o755)
					_ = os.WriteFile(filepath.Join(*out, "outputs", fmt.Sprintf("%s_p%d_%s_r%d.json", j.query, j.p1, j.order, j.rep)), []byte(output), 0o644)
				}
				mu.Lock()
				runs = append(runs, rec)
				spent += rec.CostUSD
				b, _ := json.Marshal(rec)
				rf.Write(append(b, '\n'))
				if spent > *maxUSD {
					aborted = true
					log.Printf("ABORT: estimated spend $%.2f exceeded -max-usd=%.0f", spent, *maxUSD)
				}
				n := len(runs)
				mu.Unlock()
				if n%10 == 0 {
					log.Printf("progress: %d runs, est. spend $%.2f", n, spent)
				}
			}
		}()
	}
	for _, j := range jobs {
		ch <- j
	}
	close(ch)
	wg.Wait()

	// 4. Report.
	sort.SliceStable(runs, func(i, j int) bool { return runs[i].Query < runs[j].Query })
	cmp := evalscore.CompareLibrarian(runs, *margin)
	md := cmp.Markdown() + fmt.Sprintf("\nEstimated total spend: $%.2f (token counts x geminiPriceTable; cached-input rates unverified for most models)\n", spent)
	if aborted {
		md = "**ABORTED at the spend ceiling: results below are partial.**\n\n" + md
	}
	if err := os.WriteFile(filepath.Join(*out, "summary.md"), []byte(md), 0o644); err != nil {
		log.Fatal(err)
	}
	fmt.Println(md)
}

type usageSnap struct {
	in, cached, out, think int32
	model                  string
}

func snapshot(o *agents.Orchestrator) usageSnap {
	u := o.Usage
	s := usageSnap{in: u.InputTokens, cached: u.CachedInputTokens, out: u.OutputTokens, think: u.ThinkingTokens}
	if a := u.PerAgent["CorOS Librarian"]; a != nil {
		s.model = a.Model
	}
	return s
}
