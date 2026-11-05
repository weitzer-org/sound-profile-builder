// eval_architect_ab compares two Architect prompt versions (default v4 vs v5) on the golden
// query set. Each run is the real full pipeline (RunPipeline) with config.json's models and
// prompt pins; only agent_prompts["12_architect"] differs between the arms. Each output is
// scored with RunMechanicalQualityChecks, the same deterministic checks production runs.
//
//	go run ./cmd/eval_architect_ab                      # dry run: prints the plan only
//	go run ./cmd/eval_architect_ab -dry-run=false -reps=1
//
// The arms are interleaved per query and rep (A, B, A, B, ...), so drift in the API over the
// run hits both alike. Upstream agents run fresh in every run, so the two arms don't see
// identical Librarian picks: compare averages over the query set, not single queries.
//
// Model fallback is off (EVAL_NO_FALLBACK=true, unless -allow-fallback), so a failing call
// fails its run instead of being silently scored on another model; failed runs are listed
// and left out of the averages.
//
// The catalog counts are reported per kind. Read CabConvention on its own: v4's prompt
// always writes a cab "Blend", which the device doesn't have, so it counts against v4 by
// construction.
//
// Output (under -out/<timestamp>/): runs.jsonl (one line per run, written as it finishes),
// <query>_<arm>_rep<n>.json (the raw Architect output) and report.md.
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
	"time"

	"github.com/weitzer-org/sound-builder/internal/agents"
	"github.com/weitzer-org/sound-builder/internal/config"
	"github.com/weitzer-org/sound-builder/internal/evalfixtures"
)

type runLine struct {
	Query          string                         `json:"query"`
	Arm            string                         `json:"arm"`
	Rep            int                            `json:"rep"`
	Error          string                         `json:"error,omitempty"`
	LatencySec     float64                        `json:"latency_sec"`
	InputTokens    int32                          `json:"input_tokens"`
	OutputTokens   int32                          `json:"output_tokens"`
	ThinkingTokens int32                          `json:"thinking_tokens"`
	ModelsUsed     map[string]int                 `json:"models_used,omitempty"`
	Report         agents.MechanicalQualityReport `json:"report"`
}

func main() {
	dryRun := flag.Bool("dry-run", true, "print the plan and exit without calling Gemini; pass -dry-run=false to spend real money")
	armA := flag.String("a", "v4", "Architect prompt version for arm A")
	armB := flag.String("b", "v5", "Architect prompt version for arm B")
	reps := flag.Int("reps", 1, "repetitions per (query, arm)")
	queriesFlag := flag.String("queries", "all", "comma-separated golden-set query names, or all")
	configPath := flag.String("config", "config.json", "app config (models, prompt pins, plugins, capture toggles)")
	outDir := flag.String("out", "eval_results/architect_ab", "output directory")
	allowFallback := flag.Bool("allow-fallback", false, "let a failing call fall back to another model (off by default: such runs aren't comparable)")
	flag.Parse()

	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		log.Fatalf("load %s: %v", *configPath, err)
	}
	golden := evalfixtures.GoldenQueries()
	queries := evalfixtures.GoldenQueryOrder()
	if *queriesFlag != "all" {
		queries = nil
		for _, q := range strings.Split(*queriesFlag, ",") {
			q = strings.TrimSpace(q)
			if _, ok := golden[q]; !ok {
				log.Fatalf("unknown query %q (see evalfixtures.GoldenQueryOrder)", q)
			}
			queries = append(queries, q)
		}
	}
	arms := []string{*armA, *armB}
	total := len(queries) * len(arms) * *reps
	log.Printf("Plan: %d queries x %d arms (%s) x %d reps = %d full-pipeline runs (~12-13 agent calls each)",
		len(queries), len(arms), strings.Join(arms, " vs "), *reps, total)
	log.Printf("Queries: %s", strings.Join(queries, ", "))
	log.Printf("Prompt pins from %s (12_architect overridden per arm): %v", *configPath, cfg.AgentPrompts)
	if *dryRun {
		log.Println("DRY RUN -- no Gemini calls made. Pass -dry-run=false to execute this plan.")
		return
	}
	if !*allowFallback {
		os.Setenv(agents.EvalNoFallbackEnv, "true")
	}
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		log.Fatal("GEMINI_API_KEY must be set for a live run")
	}

	// The same constraints the app builds from config.json, with the two standard eval
	// guitars, and the ground truth those constraints imply.
	constraints := map[string]interface{}{
		"single_amp_mode":        cfg.SingleAmpMode,
		"allow_cloud_captures":   cfg.AllowCloudCaptures,
		"allow_factory_captures": cfg.AllowFactoryCaptures,
		"allow_user_captures":    cfg.AllowUserCaptures,
		"favor_captures":         cfg.FavorCaptures,
		"favor_cloud_captures":   cfg.FavorCloudCaptures,
		"allow_paid_plugins":     cfg.AllowPaidPlugins,
		"available_plugins":      cfg.AvailablePlugins,
		"guitars":                evalfixtures.DefaultConstraints()["guitars"],
	}
	validBlocks := agents.BuildEffectiveValidBlocksFor(agents.ValidBlockFilter{
		AllowFactoryCaptures: cfg.AllowFactoryCaptures,
		AllowUserCaptures:    cfg.AllowUserCaptures,
		Plugins:              agents.PluginGateFromConstraints(constraints),
	})

	dir := filepath.Join(*outDir, time.Now().Format("2006-01-02-150405"))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		log.Fatal(err)
	}
	jsonl, err := os.Create(filepath.Join(dir, "runs.jsonl"))
	if err != nil {
		log.Fatal(err)
	}
	defer jsonl.Close()

	ctx := context.Background()
	var lines []runLine
	n := 0
	for _, q := range queries {
		for rep := 1; rep <= *reps; rep++ {
			for _, arm := range arms {
				n++
				log.Printf("[%d/%d] %s | %s | rep %d", n, total, q, arm, rep)
				line := runOnce(ctx, apiKey, cfg, arm, golden[q], constraints, validBlocks)
				line.Query, line.Arm, line.Rep = q, arm, rep
				if line.Error != "" {
					log.Printf("  error: %s", line.Error)
				} else {
					log.Printf("  defects=%d (device %d) checked blocks=%d", line.Report.TotalDefects(), line.Report.Catalog.Defects(), line.Report.Catalog.CheckedBlocks)
				}
				b, _ := json.Marshal(line)
				jsonl.Write(append(b, '\n'))
				jsonl.Sync()
				lines = append(lines, line)
			}
		}
	}
	report := renderReport(lines, arms)
	if err := os.WriteFile(filepath.Join(dir, "report.md"), []byte(report), 0o644); err != nil {
		log.Fatal(err)
	}
	fmt.Print(report)
	log.Printf("Wrote %s", dir)
}

func runOnce(ctx context.Context, apiKey string, cfg *config.AppConfig, arm, query string, constraints map[string]interface{}, validBlocks map[string]bool) runLine {
	var line runLine
	orch, err := agents.NewOrchestrator(ctx, apiKey, nil)
	if err != nil {
		line.Error = err.Error()
		return line
	}
	defer orch.Close()
	orch.AgentModels = cfg.AgentModels
	prompts := make(map[string]string, len(cfg.AgentPrompts)+1)
	for k, v := range cfg.AgentPrompts {
		prompts[k] = v
	}
	prompts["12_architect"] = arm

	start := time.Now()
	_, usage, err := orch.RunPipeline(ctx, query, constraints, prompts, nil)
	line.LatencySec = time.Since(start).Seconds()
	if err != nil {
		line.Error = err.Error()
		return line
	}
	line.InputTokens, line.OutputTokens, line.ThinkingTokens = usage.InputTokens, usage.OutputTokens, usage.ThinkingTokens
	line.ModelsUsed = usage.ModelsUsed
	line.Report = agents.RunMechanicalQualityChecks(orch.LastArchitectJSON(), validBlocks)
	return line
}

// renderReport averages each arm over its successful, parsed runs.
func renderReport(lines []runLine, arms []string) string {
	var sb strings.Builder
	sb.WriteString("# Architect A/B\n\nAverages per successful run (lower is better). Device* columns are the catalog parameter check; CabConv counts v4-style Blend on cabs.\n\n")
	sb.WriteString("| Arm | OK/Runs | Defects | Unverified | CaptureFmt | Cabinet | Ranges | Critic | DevUnknownName | DevRange | DevSwitch | DevUnit | DevBasis | CabConv | Checked blocks | In tok | Out+think tok |\n")
	sb.WriteString("|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|\n")
	for _, arm := range arms {
		var ok, runs int
		var sum [16]float64
		for _, l := range lines {
			if l.Arm != arm {
				continue
			}
			runs++
			if l.Error != "" || l.Report.ParseError != "" {
				continue
			}
			ok++
			r := l.Report
			for i, v := range []int{r.TotalDefects(), r.UnverifiedBlocks, r.CaptureFormattingMismatches, r.IncompleteCabinetBlocks,
				r.LeftoverValueRanges, r.CriticIssues, r.Catalog.UnknownNames, r.Catalog.OutOfRange, r.Catalog.BadSwitchValues,
				r.Catalog.UnitMismatches, r.Catalog.BasisOverclaims, r.Catalog.CabConvention, r.Catalog.CheckedBlocks,
				int(l.InputTokens), int(l.OutputTokens + l.ThinkingTokens)} {
				sum[i] += float64(v)
			}
		}
		fmt.Fprintf(&sb, "| %s | %d/%d |", arm, ok, runs)
		for i := 0; i < 15; i++ {
			if ok == 0 {
				sb.WriteString(" - |")
				continue
			}
			fmt.Fprintf(&sb, " %.2f |", sum[i]/float64(ok))
		}
		sb.WriteString("\n")
	}
	var failed []string
	for _, l := range lines {
		if l.Error != "" || l.Report.ParseError != "" {
			failed = append(failed, fmt.Sprintf("%s/%s/rep%d: %s%s", l.Query, l.Arm, l.Rep, l.Error, l.Report.ParseError))
		}
	}
	sort.Strings(failed)
	if len(failed) > 0 {
		sb.WriteString("\nFailed or unparsed runs (excluded from the averages):\n\n- " + strings.Join(failed, "\n- ") + "\n")
	}
	return sb.String()
}
