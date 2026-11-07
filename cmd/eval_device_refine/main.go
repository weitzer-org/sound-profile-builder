// eval_device_refine is the E2 A/B eval of docs/design/qc-preset-editing.md section 8.1: on
// drafts imported from the Quad Cortex, does the edit-aware refine context (arm B) produce
// changes the bridge can apply, without losing tone quality, compared with today's refine
// (arm A)?
//
// Modes (-mode):
//
//	run    refine every (draft, request, arm, rep): Gemini calls, resumable, outputs saved
//	judge  blind pairwise judge over the saved outputs of triples usable in both arms: Gemini calls
//	score  re-score saved outputs and print the report: NO Gemini calls, rerunnable any time
//
// -dry-run prints the run matrix and the number of Gemini calls the mode would make and makes
// none. The paid modes also refuse to start unless -confirm-calls equals the printed expected
// count, so a spend is always a deliberate second step.
//
//	go run ./cmd/eval_device_refine -dry-run -drafts 01_SRV_Clean,05_Clapton -reps 1
//	go run ./cmd/eval_device_refine -mode run -out eval_results/device_refine/pilot -drafts ... -reps 1 -confirm-calls 16
//	go run ./cmd/eval_device_refine -mode score -out eval_results/device_refine/pilot
//
// Corpus (-corpus, default the round-trip fixtures): *.json files that are either a fixture
// (testdata/devicejobs/readback_roundtrip) or a stored snapshot (device/snapshots/<job_id>.json,
// qc2-device-snapshot/1) of the owner's own reads. Arm A is the production refine (config.json's
// agent_prompts with the edit-aware key removed); arm B adds that key. Scoring is by
// deviceedit.Plan against the draft's snapshot; see internal/evaldevicerefine.
package main

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
	"sync"
	"time"

	"google.golang.org/genai"

	"github.com/weitzer-org/sound-builder/internal/agents"
	"github.com/weitzer-org/sound-builder/internal/config"
	"github.com/weitzer-org/sound-builder/internal/evalcheck"
	"github.com/weitzer-org/sound-builder/internal/evalcost"
	er "github.com/weitzer-org/sound-builder/internal/evaldevicerefine"
)

type opts struct {
	mode, out, configPath, sectionVersion, judgeModel string
	corpus, drafts, requests, arms                    string
	reps, parallel, confirm                           int
	dryRun, retryFailed, allowFallback                bool
}

func main() {
	var o opts
	flag.StringVar(&o.mode, "mode", "run", "run | judge | score")
	flag.StringVar(&o.out, "out", "eval_results/device_refine", "output directory (runs.jsonl, outputs/, judge.jsonl, report.md)")
	flag.StringVar(&o.configPath, "config", "config.json", "config.json: production prompt pins, plugin ownership")
	flag.StringVar(&o.sectionVersion, "section-version", "v1", "edit-aware section version for arm B (prompts/12_architect_device_edit_<v>.md)")
	flag.StringVar(&o.judgeModel, "judge-model", "gemini-3.1-pro-preview", "model for -mode judge (cmd/judge's)")
	flag.StringVar(&o.corpus, "corpus", "testdata/devicejobs/readback_roundtrip", "comma-separated directories of fixtures or snapshots")
	flag.StringVar(&o.drafts, "drafts", "", "comma-separated draft names to use (default all)")
	flag.StringVar(&o.requests, "requests", strings.Join(er.RequestIDs(), ","), "comma-separated request ids")
	flag.StringVar(&o.arms, "arms", "A,B", "comma-separated arms")
	flag.IntVar(&o.reps, "reps", 2, "repetitions per (draft, request, arm)")
	flag.IntVar(&o.parallel, "parallel", 3, "concurrent Gemini calls")
	flag.IntVar(&o.confirm, "confirm-calls", 0, "paid modes: must equal the expected call count the dry run prints")
	flag.BoolVar(&o.dryRun, "dry-run", false, "print the matrix and expected Gemini calls; make none")
	flag.BoolVar(&o.retryFailed, "retry-failed", false, "run mode: run again the runs whose last attempt failed")
	flag.BoolVar(&o.allowFallback, "allow-fallback", false, "run mode: do not require EVAL_NO_FALLBACK=true (a fallback model could then answer)")
	flag.Parse()

	cfg, err := config.LoadConfig(o.configPath)
	if err != nil {
		log.Fatalf("load %s: %v", o.configPath, err)
	}
	exp, err := er.NewExporter(cfg)
	if err != nil {
		log.Fatal(err)
	}
	ctx := context.Background()
	all, err := er.LoadCorpus(ctx, splitList(o.corpus), exp)
	if err != nil {
		log.Fatal(err)
	}
	drafts := pick(all, splitList(o.drafts))
	reqIDs := splitList(o.requests)
	for _, id := range reqIDs {
		if _, ok := er.RequestByID(id); !ok {
			log.Fatalf("unknown request %q (have %s)", id, strings.Join(er.RequestIDs(), ", "))
		}
	}
	arms := splitList(o.arms)
	for _, a := range arms {
		if a != er.ArmA && a != er.ArmB {
			log.Fatalf("unknown arm %q", a)
		}
	}
	names := make([]string, len(drafts))
	for i, d := range drafts {
		names[i] = d.Name
	}
	matrix := er.Matrix(names, reqIDs, arms, o.reps)

	switch o.mode {
	case "run":
		if o.dryRun {
			dryRun(o, drafts, matrix, nil)
			return
		}
		paid(o, len(todo(o, matrix)))
		runMode(ctx, o, cfg, drafts, matrix)
	case "judge":
		scorer := er.NewScorer(exp)
		var pairs []er.Pair
		for _, p := range pairsOf(o, scorer, all) {
			if p.A.Judge == nil || p.B.Judge == nil { // not judged yet
				pairs = append(pairs, p)
			}
		}
		if o.dryRun {
			dryRun(o, drafts, matrix, pairs)
			return
		}
		paid(o, len(pairs))
		judgeMode(ctx, o, all, pairs)
	case "score":
		scoreMode(o, er.NewScorer(exp), all)
	default:
		log.Fatalf("unknown -mode %q", o.mode)
	}
}

func splitList(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func pick(all []*er.Draft, names []string) []*er.Draft {
	if len(names) == 0 {
		return all
	}
	by := map[string]*er.Draft{}
	for _, d := range all {
		by[d.Name] = d
	}
	var out []*er.Draft
	for _, n := range names {
		d, ok := by[n]
		if !ok {
			var have []string
			for k := range by {
				have = append(have, k)
			}
			sort.Strings(have)
			log.Fatalf("no draft %q in the corpus (have %s)", n, strings.Join(have, ", "))
		}
		out = append(out, d)
	}
	return out
}

// ---- dry run and the spend guard ----

func dryRun(o opts, drafts []*er.Draft, matrix []er.Run, pairs []er.Pair) {
	fmt.Printf("DRY RUN, mode %s: no Gemini call is made.\n\n", o.mode)
	fmt.Printf("Drafts (%d): ", len(drafts))
	for _, d := range drafts {
		fmt.Printf("%s[%d blocks, %d unconverted] ", d.Name, len(d.Chain), d.Unconverted)
	}
	fmt.Printf("\nRequests (%d): %s\nArms: %s\nReps: %d\n\n", len(splitList(o.requests)), o.requests, o.arms, o.reps)
	fmt.Println("| Draft | Request | Arm | Rep | Already done |\n|---|---|---|--:|:-:|")
	done := doneSet(o)
	for _, r := range matrix {
		mark := ""
		if done[r] {
			mark = "yes"
		}
		fmt.Printf("| %s | %s | %s | %d | %s |\n", r.Draft, r.Request, r.Arm, r.Rep, mark)
	}
	switch o.mode {
	case "run":
		n := len(todo(o, matrix))
		fmt.Printf("\nRefine runs in the matrix: %d. Still to run (not already done in %s): %d.\n", len(matrix), o.out, n)
		fmt.Printf("Expected Gemini calls for -mode run: %d (one RefineChat call each; EVAL_NO_FALLBACK=true is required so there are no fallback calls, but the SDK's own retries on a transient error are not counted here, so this is a floor).\n", n)
		fmt.Printf("To start: -confirm-calls %d. Cost per run for this prompt is UNMEASURED.\n", n)
		nj := 0
		for _, r := range matrix {
			if r.Arm == er.ArmA {
				nj++
			}
		}
		fmt.Printf("\nA later -mode judge would make at most %d calls (one per triple usable in both arms; the exact count is printed by its own dry run once the runs exist).\n", nj)
	case "judge":
		fmt.Printf("\nTriples usable in both arms in %s: %d. Expected Gemini calls for -mode judge: %d (one per triple). To start: -confirm-calls %d.\n", o.out, len(pairs), len(pairs), len(pairs))
	}
}

func paid(o opts, expected int) {
	if expected == 0 {
		log.Println("Nothing to do.")
		os.Exit(0)
	}
	if o.confirm != expected {
		log.Fatalf("this would make %d Gemini calls; pass -confirm-calls %d only once the owner has approved that spend (see -dry-run)", expected, expected)
	}
}

// ---- run mode ----

func versionsFor(cfg *config.AppConfig, arm, sectionVersion string) map[string]string {
	v := map[string]string{}
	for k, val := range cfg.AgentPrompts {
		v[k] = val
	}
	delete(v, agents.DeviceEditPromptKey) // arm A is today's refine whatever config.json says later
	if arm == er.ArmB {
		v[agents.DeviceEditPromptKey] = sectionVersion
	}
	return v
}

func outputPath(out string, r er.Run) string {
	return filepath.Join(out, "outputs", r.Draft, r.Request, r.Arm, fmt.Sprintf("rep%d.json", r.Rep))
}

func recordOf(rec evalcheck.RunRecord) er.Run {
	d, q, _ := strings.Cut(rec.Query, "/")
	return er.Run{Triple: er.Triple{Draft: d, Request: q, Rep: rec.Rep}, Arm: rec.Scenario}
}

func readRecords(out string) []evalcheck.RunRecord {
	recs, err := evalcheck.ReadRunRecords(filepath.Join(out, "runs.jsonl"))
	if err != nil && !os.IsNotExist(err) {
		log.Fatal(err)
	}
	return recs
}

// doneSet is the runs that already have a record (a failed one counts as done unless
// -retry-failed, because it already cost money).
func doneSet(o opts) map[er.Run]bool {
	done := map[er.Run]bool{}
	for _, rec := range readRecords(o.out) {
		// The latest record decides, as in scoreRuns: a failure after a success is retried with -retry-failed.
		done[recordOf(rec)] = rec.Success || !o.retryFailed
	}
	return done
}

func todo(o opts, matrix []er.Run) []er.Run {
	done := doneSet(o)
	var out []er.Run
	for _, r := range matrix {
		if !done[r] {
			out = append(out, r)
		}
	}
	return out
}

func runMode(ctx context.Context, o opts, cfg *config.AppConfig, drafts []*er.Draft, matrix []er.Run) {
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		log.Fatal("GEMINI_API_KEY must be set for -mode run")
	}
	if set := agents.AblationEnvSet(); len(set) > 0 {
		log.Fatalf("refusing to run with %v set: an ablated agent would be scored as a real one", set)
	}
	if !agents.EvalNoFallbackEnabled() && !o.allowFallback {
		log.Fatal("set EVAL_NO_FALLBACK=true so a failing model fails visibly instead of a fallback model answering (or pass -allow-fallback)")
	}
	byName := map[string]*er.Draft{}
	for _, d := range drafts {
		byName[d.Name] = d
	}
	if err := os.MkdirAll(o.out, 0o755); err != nil {
		log.Fatal(err)
	}
	f, err := os.OpenFile(filepath.Join(o.out, "runs.jsonl"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	gate := agents.NewPluginGate(cfg.AllowPaidPlugins, cfg.AvailablePlugins)
	expected := map[string]string{"12_architect": agents.DefaultModelFor("12_architect")}
	var mu sync.Mutex
	sem := make(chan struct{}, max(o.parallel, 1))
	var wg sync.WaitGroup
	for _, r := range todo(o, matrix) {
		r := r
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			d := byName[r.Draft]
			req, _ := er.RequestByID(r.Request)
			runCtx, cancel := context.WithTimeout(agents.WithPromptVersions(ctx, versionsFor(cfg, r.Arm, o.sectionVersion)), 6*time.Minute)
			defer cancel()
			start := time.Now()
			orch, err := agents.NewOrchestrator(runCtx, apiKey, nil)
			rec := evalcheck.RunRecord{Scenario: r.Arm, Query: r.Triple.Key(), Rep: r.Rep}
			var out string
			if err == nil {
				out, _, err = orch.RefineChat(runCtx, d.Preset(), req.Message, cfg.AllowFactoryCaptures, cfg.AllowUserCaptures, gate)
			}
			rec.LatencySec = time.Since(start).Seconds()
			rec.Success = err == nil
			if err != nil {
				rec.Error = err.Error()
			}
			if orch != nil {
				defer orch.Close()
				calls := orch.Usage.CallsSnapshot()
				rec.IntegrityOK, rec.IntegrityIssues, rec.MaxTokensByAgent = evalcheck.AssessCalls(calls, expected, nil)
				rec.CallsFile = strings.TrimSuffix(outputPath(o.out, r), ".json") + ".calls.json"
				writeJSON(rec.CallsFile, calls)
			}
			if err == nil {
				rec.OutputFile = outputPath(o.out, r)
				if werr := os.MkdirAll(filepath.Dir(rec.OutputFile), 0o755); werr != nil {
					log.Printf("mkdir for %s: %v", rec.OutputFile, werr)
				} else if werr := os.WriteFile(rec.OutputFile, []byte(out), 0o644); werr != nil {
					log.Printf("write %s: %v", rec.OutputFile, werr)
				}
			}
			line, _ := json.Marshal(rec)
			mu.Lock()
			fmt.Fprintln(f, string(line))
			mu.Unlock()
			log.Printf("%s %s arm %s rep %d: success=%v integrity_ok=%v %.0fs %v", r.Draft, r.Request, r.Arm, r.Rep, rec.Success, rec.IntegrityOK, rec.LatencySec, err)
		}()
	}
	wg.Wait()
	log.Printf("Done. Re-score with: go run ./cmd/eval_device_refine -mode score -out %s (no Gemini calls)", o.out)
}

func writeJSON(path string, v any) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		log.Printf("mkdir %s: %v", path, err)
		return
	}
	b, _ := json.MarshalIndent(v, "", "  ")
	if err := os.WriteFile(path, b, 0o644); err != nil {
		log.Printf("write %s: %v", path, err)
	}
}

// ---- scoring (no Gemini) ----

// scoreRuns scores every record in o.out against the corpus. The latest record of a run wins.
func scoreRuns(o opts, s *er.Scorer, all []*er.Draft) []er.RunScore {
	byName := map[string]*er.Draft{}
	for _, d := range all {
		byName[d.Name] = d
	}
	latest := map[er.Run]evalcheck.RunRecord{}
	for _, rec := range readRecords(o.out) {
		latest[recordOf(rec)] = rec
	}
	var runs []er.Run
	for r := range latest {
		runs = append(runs, r)
	}
	sort.Slice(runs, func(i, j int) bool {
		a, b := runs[i], runs[j]
		if a.Draft != b.Draft {
			return a.Draft < b.Draft
		}
		if a.Request != b.Request {
			return a.Request < b.Request
		}
		if a.Rep != b.Rep {
			return a.Rep < b.Rep // numeric: rep 10 sorts after rep 2
		}
		return a.Arm < b.Arm
	})
	judged := judgeByRun(o.out)
	var out []er.RunScore
	for _, r := range runs {
		rec := latest[r]
		d, ok := byName[r.Draft]
		req, rok := er.RequestByID(r.Request)
		if !ok || !rok {
			log.Printf("skipping %s: not in the loaded corpus/requests", r.Triple.Key())
			continue
		}
		rs := er.RunScore{Run: r, Success: rec.Success, IntegrityOK: rec.IntegrityOK, Error: rec.Error}
		if rec.Success && rec.OutputFile != "" {
			raw, err := os.ReadFile(rec.OutputFile)
			if err != nil {
				log.Fatalf("%s: %v", rec.OutputFile, err)
			}
			rs.Metrics = s.Score(d, req, string(raw))
		}
		rs.Judge = judged[r]
		out = append(out, rs)
	}
	return out
}

func pairsOf(o opts, s *er.Scorer, all []*er.Draft) []er.Pair {
	pairs, _ := er.Pairs(scoreRuns(o, s, all))
	return pairs
}

func scoreMode(o opts, s *er.Scorer, all []*er.Draft) {
	runs := scoreRuns(o, s, all)
	var sb strings.Builder
	er.Report(&sb, runs)
	costReport(&sb, o, runs)
	fmt.Print(sb.String())
	if err := os.WriteFile(filepath.Join(o.out, "report.md"), []byte(sb.String()), 0o644); err != nil {
		log.Printf("write report: %v", err)
	}
	writeJSON(filepath.Join(o.out, "scores.json"), runs)
}

// costReport prices each arm's recorded calls with evalcost (the same price table
// cmd/eval_cost and the usage records use). Models with no official price are UNKNOWN, never $0.
func costReport(w *strings.Builder, o opts, runs []er.RunScore) {
	fmt.Fprint(w, "\n## Cost of the recorded refine calls (each run's latest attempt; failed ones included)\n\n| Arm | Runs | Calls | Input tokens | Output tokens | Thinking tokens | Known USD | Status |\n|---|--:|--:|--:|--:|--:|--:|---|\n")
	recs := map[er.Run]evalcheck.RunRecord{}
	for _, rec := range readRecords(o.out) {
		recs[recordOf(rec)] = rec
	}
	for _, arm := range er.Arms {
		var n int
		var tot evalcost.RunCost
		tot.Unpriced = map[string]evalcost.ModelTokens{}
		for _, rs := range runs {
			if rs.Arm != arm {
				continue
			}
			rec := recs[rs.Run]
			if rec.CallsFile == "" {
				continue
			}
			b, err := os.ReadFile(rec.CallsFile)
			if err != nil {
				continue
			}
			var calls []agents.CallRecord
			if json.Unmarshal(b, &calls) != nil {
				continue
			}
			c := evalcost.CostOfCalls(calls, evalcost.BasisRequested)
			n++
			tot.Calls += c.Calls
			tot.KnownUSD += c.KnownUSD
			tot.Input += c.Input
			tot.Output += c.Output
			tot.Thinking += c.Thinking
			for m, t := range c.Unpriced {
				x := tot.Unpriced[m]
				x.Calls += t.Calls
				x.Input += t.Input
				x.Output += t.Output
				x.Thinking += t.Thinking
				tot.Unpriced[m] = x
			}
		}
		fmt.Fprintf(w, "| %s | %d | %d | %d | %d | %d | $%.4f | %s |\n", arm, n, tot.Calls, tot.Input, tot.Output, tot.Thinking, tot.KnownUSD, tot.Status())
	}
	fmt.Fprint(w, "\nA run retried with -retry-failed overwrites its calls file (the path is per run, not per attempt), so an earlier attempt's tokens are not in this table: add them by hand if a retry happened. Known USD excludes any model with no official price (Status PARTIAL or UNKNOWN then makes it a lower bound). `cmd/usage_report` reads the same spend from the usage records when the run recorded to a bucket; this tool runs with no bucket, so the calls files above are the record.\n")
}

// ---- judge mode ----

func judgeFile(out string) string { return filepath.Join(out, "judge.jsonl") }

// judgeByRun maps each run to its judge score from judge.jsonl (the last record per triple).
func judgeByRun(out string) map[er.Run]*er.JudgeScore {
	b, err := os.ReadFile(judgeFile(out))
	if err != nil {
		return nil
	}
	m := map[er.Run]*er.JudgeScore{}
	for _, line := range strings.Split(string(b), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var jr er.JudgeRecord
		if json.Unmarshal([]byte(line), &jr) != nil {
			continue
		}
		a, bb := jr.Scores()
		if a == nil {
			delete(m, er.Run{Triple: jr.Triple, Arm: er.ArmA})
			delete(m, er.Run{Triple: jr.Triple, Arm: er.ArmB})
			continue
		}
		m[er.Run{Triple: jr.Triple, Arm: er.ArmA}] = a
		m[er.Run{Triple: jr.Triple, Arm: er.ArmB}] = bb
	}
	return m
}

func judgeMode(ctx context.Context, o opts, all []*er.Draft, pairs []er.Pair) {
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		log.Fatal("GEMINI_API_KEY must be set for -mode judge")
	}
	client, err := genai.NewClient(ctx, &genai.ClientConfig{APIKey: apiKey, Backend: genai.BackendGeminiAPI})
	if err != nil {
		log.Fatalf("genai client: %v", err)
	}
	byName := map[string]*er.Draft{}
	for _, d := range all {
		byName[d.Name] = d
	}
	recs := map[er.Run]evalcheck.RunRecord{}
	for _, rec := range readRecords(o.out) {
		recs[recordOf(rec)] = rec
	}
	f, err := os.OpenFile(judgeFile(o.out), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()
	cfg := &genai.GenerateContentConfig{
		ResponseMIMEType: "application/json",
		ResponseSchema: &genai.Schema{Type: genai.TypeObject, Required: []string{"x", "y", "rationale"}, Properties: map[string]*genai.Schema{
			"x":         candidateSchema(),
			"y":         candidateSchema(),
			"rationale": {Type: genai.TypeString},
		}},
	}
	for _, p := range pairs {
		d := byName[p.Draft]
		req, _ := er.RequestByID(p.Request)
		rawA, errA := os.ReadFile(recs[p.A.Run].OutputFile)
		rawB, errB := os.ReadFile(recs[p.B.Run].OutputFile)
		if errA != nil || errB != nil {
			log.Fatalf("%s: cannot read saved outputs: %v %v", p.Triple.Key(), errA, errB)
		}
		xIsA := er.XIsA(p.Triple)
		x, y := er.RenderCandidate(d, string(rawA)), er.RenderCandidate(d, string(rawB))
		if !xIsA {
			x, y = y, x
		}
		jr := er.JudgeRecord{Triple: p.Triple, XIsA: xIsA}
		resp, err := client.Models.GenerateContent(ctx, o.judgeModel,
			[]*genai.Content{genai.NewContentFromText(er.JudgePrompt(d, req, x, y), genai.RoleUser)}, cfg)
		if err != nil {
			jr.Error = err.Error()
		} else {
			jr.Raw = resp.Text()
			if v, perr := er.ParseVerdict(jr.Raw); perr != nil {
				jr.Error = perr.Error()
			} else {
				jr.Verdict = v
			}
		}
		line, _ := json.Marshal(jr)
		fmt.Fprintln(f, string(line))
		log.Printf("judged %s#%d: err=%q", p.Triple.Key(), p.Rep, jr.Error)
	}
	log.Printf("Done. Merge into the report with -mode score -out %s", o.out)
}

func candidateSchema() *genai.Schema {
	return &genai.Schema{Type: genai.TypeObject, Required: []string{"addressed", "tone_ok"}, Properties: map[string]*genai.Schema{
		"addressed": {Type: genai.TypeInteger},
		"tone_ok":   {Type: genai.TypeBoolean},
	}}
}
