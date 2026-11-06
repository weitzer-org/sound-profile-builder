// eval_architect_ab compares two Architect prompt versions (default v5 vs v6) on the golden
// query set. Each run is the real full pipeline (RunPipeline) with config.json's models and
// prompt pins; only agent_prompts["12_architect"] differs between the arms. Each output is
// scored with RunMechanicalQualityChecks, the same deterministic checks production runs.
//
//	go run ./cmd/eval_architect_ab                      # dry run: prints the plan only
//	go run ./cmd/eval_architect_ab -dry-run=false -reps=1
//	go run ./cmd/eval_architect_ab -a "" -dry-run=false   # arm B alone, half the calls
//
// The arms are interleaved per query and rep (A, B, A, B, ...), so drift in the API over the
// run hits both alike. Upstream agents run fresh in every run, so the two arms don't see
// identical Librarian picks: compare averages over the query set, not single queries.
//
// Model fallback is off (EVAL_NO_FALLBACK=true, unless -allow-fallback), so a failing call
// fails its run instead of being silently scored on another model; failed runs are listed
// and left out of the averages.
//
// The catalog counts are reported per kind. Read CabConvention on its own: a pre-v6 prompt
// always writes a cab "Blend", which the device doesn't have, so it counts against it by
// construction. Unconsidered (controls neither set nor kept at default) is only measured
// for output that uses kept_defaults, so it is 0 for pre-v6 output by construction, not by merit.
//
// Output (under -out/<timestamp>/): runs.jsonl (one line per run, written as it finishes),
// <query>_<arm>_rep<n>.json (the raw Architect output) and report.md. report.md ends with a
// pan table: what each run wrote for the pan controls (center / kept / range minimum / other).
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
	"strconv"
	"strings"
	"time"

	"github.com/weitzer-org/sound-builder/internal/agents"
	"github.com/weitzer-org/sound-builder/internal/config"
	"github.com/weitzer-org/sound-builder/internal/evalfixtures"
	"github.com/weitzer-org/sound-builder/internal/storage"
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
	// Raw is the Architect's output, saved next to runs.jsonl as <query>_<arm>_rep<n>.json
	// (not repeated in runs.jsonl).
	Raw string `json:"-"`
	// Pans is what the run wrote for each pan control (see panStats).
	Pans PanStats `json:"pans"`
}

func main() {
	dryRun := flag.Bool("dry-run", true, "print the plan and exit without calling Gemini; pass -dry-run=false to spend real money")
	armA := flag.String("a", "v5", `Architect prompt version for arm A; "" skips arm A and runs arm B alone`)
	armB := flag.String("b", "v6", "Architect prompt version for arm B")
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
	if *armA == "" { // -a "" runs arm B only (half the cost)
		arms = []string{*armB}
	}
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
				if line.Raw != "" {
					name := fmt.Sprintf("%s_%s_rep%d.json", q, arm, rep)
					if err := os.WriteFile(filepath.Join(dir, name), []byte(line.Raw), 0o644); err != nil {
						log.Printf("  could not save %s: %v", name, err)
					}
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
	line.Raw = orch.LastArchitectJSON()
	line.Report = agents.RunMechanicalQualityChecks(line.Raw, validBlocks)
	line.Pans = panStats(line.Raw)
	return line
}

// reportColumns is the number of averaged columns in the report table, after Arm and OK/Runs.
const reportColumns = 17

// reportValues is one run's values for the report table's averaged columns, in order.
func reportValues(l runLine) [reportColumns]int {
	r := l.Report
	return [reportColumns]int{r.TotalDefects(), r.UnverifiedBlocks, r.CaptureFormattingMismatches, r.IncompleteCabinetBlocks,
		r.LeftoverValueRanges, r.CriticIssues, r.Catalog.UnknownNames, r.Catalog.OutOfRange, r.Catalog.BadSwitchValues,
		r.Catalog.UnitMismatches, r.Catalog.BasisOverclaims, r.Catalog.UnrecognizedBlocks, r.Catalog.CabConvention,
		r.Catalog.UnconsideredParams, r.Catalog.CheckedBlocks, int(l.InputTokens), int(l.OutputTokens + l.ThinkingTokens)}
}

// renderReport averages each arm over its successful, parsed runs.
func renderReport(lines []runLine, arms []string) string {
	var sb strings.Builder
	sb.WriteString("# Architect A/B\n\nAverages per successful run (lower is better). Device* columns are the catalog parameter check; CabConv counts pre-v6 Blend on cabs.\n\n")
	sb.WriteString("| Arm | OK/Runs | Defects | Unverified | CaptureFmt | Cabinet | Ranges | Critic | DevUnknownName | DevRange | DevSwitch | DevUnit | DevBasis | DevUnrecognized | CabConv | Unconsidered | Checked blocks | In tok | Out+think tok |\n")
	sb.WriteString("|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|\n")
	for _, arm := range arms {
		var ok, runs int
		var sum [reportColumns]float64
		for _, l := range lines {
			if l.Arm != arm {
				continue
			}
			runs++
			if l.Error != "" || l.Report.ParseError != "" {
				continue
			}
			ok++
			for i, v := range reportValues(l) {
				sum[i] += float64(v)
			}
		}
		fmt.Fprintf(&sb, "| %s | %d/%d |", arm, ok, runs)
		for i := range sum {
			if ok == 0 {
				sb.WriteString(" - |")
				continue
			}
			fmt.Fprintf(&sb, " %.2f |", sum[i]/float64(ok))
		}
		sb.WriteString("\n")
	}
	sb.WriteString(renderPanTable(lines, arms))
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

// PanStats counts what one run wrote for the pan controls (L..R labelled ends) of the blocks
// the device catalog covers. The pan's scale is the catalog's; how the unit displays it is
// unverified. A pan is counted once, under the first line that applies:
//
//	Default   written, and equal to the catalog default (center)
//	Kept      named in the block's kept_defaults and not written
//	AtMin     written as the range minimum when that is not the default: the old failure, a
//	          0 meant as center that is hard left on a 0..10 pan
//	Other     written as another number on the range
//	OutOfRange / Text   written outside the range, or as text (Center, "L30" ...)
type PanStats struct {
	Default, Kept, AtMin, Other, OutOfRange, Text int
	// Written lists each written pan as "model: name = value (catalog range, default)".
	Written []string `json:"written,omitempty"`
}

func (p PanStats) total() int {
	return p.Default + p.Kept + p.AtMin + p.Other + p.OutOfRange + p.Text
}

func panStats(raw string) PanStats {
	var ps PanStats
	var env struct {
		SP storage.StructuredPreset `json:"structured_payload"`
	}
	if json.Unmarshal([]byte(agents.StripJSONFences(raw)), &env) != nil {
		return ps
	}
	for _, blocks := range env.SP.Guitars {
		for _, b := range blocks {
			ctl, ok := agents.DeviceControlsFor(b.Model)
			if !ok {
				continue
			}
			written := map[int]bool{}
			for _, bp := range b.Parameters {
				i := ctl.Index(bp.Name)
				if i < 0 || !ctl.Controls[i].Pan || written[i] {
					continue // not a pan, or repeated: each pan is counted once
				}
				written[i] = true
				c := ctl.Controls[i]
				ps.Written = append(ps.Written, fmt.Sprintf("%s: %s = %s", b.Model, bp.Name, bp.Value))
				x, err := strconv.ParseFloat(strings.TrimSpace(bp.Value), 64)
				switch {
				case err != nil || c.Min == nil || c.Max == nil || c.Default == nil:
					ps.Text++
				case x < *c.Min || x > *c.Max:
					ps.OutOfRange++
				case x == *c.Default:
					ps.Default++
				case x == *c.Min:
					ps.AtMin++
				default:
					ps.Other++
				}
			}
			for _, name := range b.KeptDefaults {
				if i := ctl.Index(name); i >= 0 && ctl.Controls[i].Pan && !written[i] {
					ps.Kept++
					written[i] = true
				}
			}
		}
	}
	return ps
}

// renderPanTable sums PanStats per arm.
func renderPanTable(lines []runLine, arms []string) string {
	var sb strings.Builder
	sb.WriteString("\nPan controls on catalog-covered blocks (each pan counted once; the unit's display of a pan is unverified):\n\n")
	sb.WriteString("| Pan arm | Pans | Default (center) | Kept at default | Written at range min | Other number | Out of range | Text |\n|---|---|---|---|---|---|---|---|\n")
	for _, arm := range arms {
		var t PanStats
		for _, l := range lines {
			if l.Arm != arm || l.Error != "" || l.Report.ParseError != "" {
				continue
			}
			t.Default += l.Pans.Default
			t.Kept += l.Pans.Kept
			t.AtMin += l.Pans.AtMin
			t.Other += l.Pans.Other
			t.OutOfRange += l.Pans.OutOfRange
			t.Text += l.Pans.Text
		}
		fmt.Fprintf(&sb, "| %s | %d | %d | %d | %d | %d | %d | %d |\n", arm, t.total(), t.Default, t.Kept, t.AtMin, t.Other, t.OutOfRange, t.Text)
	}
	return sb.String()
}
