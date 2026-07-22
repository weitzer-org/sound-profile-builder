package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/weitzer-org/sound-builder/internal/agents"
	"github.com/weitzer-org/sound-builder/internal/config"
)

// eval_full_pipeline benchmarks gemini-3.6-flash as a candidate replacement for one
// production model tier at a time, running the real 12(+1)-agent pipeline (RunPipeline)
// so every agent gets the actual upstream context it gets in production -- unlike
// cmd/eval_subagent, which can only isolate the handful of agents whose input doesn't
// depend on another agent's output.
//
// Each scenario is a complete per-agent routing map, not a single model swapped in
// uniformly everywhere: production splits agents into a "Pro tier" (1, 12) on
// gemini-3.1-pro-preview and a "Flash tier" (2-11, 13) on gemini-3.5-flash (see
// internal/agents/orchestrator.go's production routing defaults), so a candidate run
// only ever changes one tier at a time, exactly like it would if adopted.
type PipelineRunResult struct {
	Query         string
	ScenarioName  string
	ScenarioLabel string
	LatencySec    float64
	InputTokens   int32
	OutputTokens  int32
	ModelsUsed    map[string]int
	HTMLPath      string
	Error         string
}

type RoutingScenario struct {
	Label       string
	FolderName  string
	AgentModels map[string]string
}

func main() {
	log.Println("🪐 Starting Full Pipeline gemini-3.6-flash Candidate Routing Benchmark...")

	ctx := context.Background()
	cfg, err := config.LoadConfig("config.json")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// 1. Fetch Credentials. Reads GEMINI_API_KEY directly (matching how the app itself
	// authenticates locally, see cmd/server/main.go's localSecretFetcher, and how cmd/eval
	// does it) rather than requiring a GCP project + Secret Manager access.
	geminiKey := os.Getenv("GEMINI_API_KEY")
	if geminiKey == "" {
		log.Fatalf("GEMINI_API_KEY must be set to run live evals")
	}

	// 2. Instantiate the Orchestrator (Gemini only -- Open-LLM isn't part of this comparison)
	orch, err := agents.NewOrchestrator(ctx, geminiKey, nil)
	if err != nil {
		log.Fatalf("Failed to initialize Orchestrator: %v", err)
	}
	defer orch.Close()

	// Production tier split (internal/agents/orchestrator.go RunAgentSplit's routing
	// defaults). 13_critic is Flash-tier: Preset Critic runs inside RunPipeline right
	// after the Architect and otherwise falls through to the same gemini-3.5-flash default
	// as agents 2-11.
	proTierKeys := []string{"1_tone_historian", "12_architect"}
	flashTierKeys := []string{
		"2_sonic_profiler", "3_community_scraper", "4_coros_librarian",
		"5_cloud_navigator", "6_acoustician", "7_transducer_tech",
		"8_foh_optimizer", "9_mix_engineer", "10_control_mapper",
		"11_dsp_dispatcher", "13_critic",
	}

	const (
		prodProModel   = "gemini-3.1-pro-preview"
		prodFlashModel = "gemini-3.5-flash"
		candidateModel = "gemini-3.6-flash"
	)

	buildRoutingMap := func(proModel, flashModel string) map[string]string {
		m := make(map[string]string)
		for _, k := range proTierKeys {
			m[k] = proModel
		}
		for _, k := range flashTierKeys {
			m[k] = flashModel
		}
		return m
	}

	scenarios := []RoutingScenario{
		{
			Label:       "Baseline (Prod Routing)",
			FolderName:  "baseline",
			AgentModels: buildRoutingMap(prodProModel, prodFlashModel),
		},
		{
			Label:       fmt.Sprintf("Flash-Tier Candidate (%s)", candidateModel),
			FolderName:  "flash-tier-candidate",
			AgentModels: buildRoutingMap(prodProModel, candidateModel),
		},
		{
			Label:       fmt.Sprintf("Pro-Tier Candidate (%s)", candidateModel),
			FolderName:  "pro-tier-candidate",
			AgentModels: buildRoutingMap(candidateModel, prodFlashModel),
		},
	}

	// The 13 Standard Archetype Evaluation Queries
	evalQueries := map[string]string{
		"01_SRV_Clean":        "Clean funk blues tone. Stevie Ray Vaughan style with high headroom. Wants to push it with a TS808.",
		"02_Chicago_Blues":    "Chicago Blues style. Warm Chess Records style overdrive into a small combo amp. Slightly gritty but clean platform.",
		"03_British_Invasion": "Early British Invasion tone. Vox AC30/JTM45 chime and edge of breakup. Punchy mids, sparkle.",
		"04_Southern_Rock":    "Southern Rock slide style. Dual lead humbuckers into a cranked American Tweed amp. Singing sustain.",
		"05_Clapton":          "Vintage Cream-era Clapton tone. Rolled-off Les Paul tone knobs into a cranked Marshall.",
		"06_Gilmour":          "David Gilmour preset using a Hiwatt Custom 100, Ram's Head Big Muff, WEM 4x12, and a massive Plate Reverb.",
		"07_Edge":             "The Edge style chime. 1964 Vox AC30 edge-of-breakup with rhythmic dotted-eighth delays.",
		"08_EVH":              "Van Halen Brown Sound. Hot-rodded 1968 Marshall Plexi, variac sag, plate reverb.",
		"09_BB_King":          "BB King Lucile tone. High-headroom American Twin Reverb clean platform.",
		"10_Slash":            "Guns N' Roses Slash lead. Les Paul neck pickup into a hot JCM800 with standard delay.",
		"11_Mayer_Lead":       "John Mayer Trio Lead. Smooth Two-Rock/Dumble platform, mid-scooped clean with a subtle drive push.",
		"12_Bonamassa":        "Joe Bonamassa modern blues lead features, smooth tube drive into a Dumble style amplifier.",
		"13_Hard_Rock_Blues":  "hard rock blues from the 1960s - 1980s",
	}

	constraints := map[string]interface{}{
		"single_amp_mode":        true,
		"allow_cloud_captures":   false,
		"allow_factory_captures": true,
		"favor_captures":         true,
		"guitars":                []string{"Gibson ES-339 Humbuckers", "Fender Telecaster Single Coil"},
	}

	results := []PipelineRunResult{}

	queryNamesOrdered := []string{
		"01_SRV_Clean", "02_Chicago_Blues", "03_British_Invasion", "04_Southern_Rock",
		"05_Clapton", "06_Gilmour", "07_Edge", "08_EVH",
		"09_BB_King", "10_Slash", "11_Mayer_Lead", "12_Bonamassa",
		"13_Hard_Rock_Blues",
	}

	baseOutDir := "eval_results/full_pipeline"

	// Sequential execution to avoid API rate limits across the ~39-pipeline run matrix
	// (3 scenarios x 13 scenarios-worth of queries x 12 agent calls each).
	for _, scenario := range scenarios {
		log.Printf("\n🚀 STARTING ROUTING SCENARIO: %s", scenario.Label)

		modelDir := filepath.Join(baseOutDir, scenario.FolderName)
		if err := os.MkdirAll(modelDir, 0755); err != nil {
			log.Fatalf("Failed to create folder: %v", err)
		}

		orch.AgentModels = scenario.AgentModels

		for _, name := range queryNamesOrdered {
			query := evalQueries[name]
			log.Printf("\nScenario: %s | Routing: %s", name, scenario.Label)

			// Clear statistics
			orch.Usage = &agents.TokenUsage{
				ModelsUsed: make(map[string]int),
			}

			start := time.Now()
			htmlResult, tokenUsage, err := orch.RunPipeline(ctx, query, constraints, cfg.AgentPrompts, nil)
			duration := time.Since(start).Seconds()

			runResult := PipelineRunResult{
				Query:         query,
				ScenarioName:  name,
				ScenarioLabel: scenario.Label,
				LatencySec:    duration,
			}

			if err != nil {
				log.Printf(" ❌ Failed pipeline run: %v", err)
				runResult.Error = err.Error()
			} else {
				log.Printf("  Success in %.2fs | Input Tokens: %d, Output Tokens: %d", duration, tokenUsage.InputTokens, tokenUsage.OutputTokens)
				runResult.InputTokens = tokenUsage.InputTokens
				runResult.OutputTokens = tokenUsage.OutputTokens

				runResult.ModelsUsed = make(map[string]int)
				for k, v := range tokenUsage.ModelsUsed {
					runResult.ModelsUsed[k] = v
				}

				htmlFileName := fmt.Sprintf("%s.html", name)
				htmlFilePath := filepath.Join(modelDir, htmlFileName)
				err = os.WriteFile(htmlFilePath, []byte(htmlResult), 0644)
				if err != nil {
					log.Printf("  Failed to save HTML output: %v", err)
				} else {
					runResult.HTMLPath = htmlFilePath
				}
			}

			results = append(results, runResult)
		}
	}

	reportPath := filepath.Join(baseOutDir, "full_pipeline_gemini36_benchmark_report.md")
	log.Printf("\nGenerating full-pipeline summary report at: %s...", reportPath)
	generateReport(reportPath, queryNamesOrdered, evalQueries, results, scenarios)
	log.Println("🏁 FULL BENCHMARK SUITE COMPLETE! Walkthrough report written successfully.")
	log.Println("Next: run cmd/judge_compare against baseline vs each candidate folder under " + baseOutDir + " (judge model stays gemini-3.1-pro-preview for both comparisons, including the Pro-Tier one where it's also a candidate).")
}

func generateReport(path string, queryNames []string, queries map[string]string, results []PipelineRunResult, scenarios []RoutingScenario) {
	var sb strings.Builder
	sb.WriteString("# 🪐 Full Pipeline - gemini-3.6-flash Candidate Routing Benchmark Report\n\n")
	sb.WriteString("An end-to-end performance and qualitative benchmark comparing production routing (Pro tier: gemini-3.1-pro-preview on agents 1/12, Flash tier: gemini-3.5-flash on agents 2-11/13) against two gemini-3.6-flash candidate routings, one tier swapped at a time.\n\n")

	sb.WriteString("## 📊 Telemetry Summary Matrix\n\n")

	sb.WriteString("| Scenario | Metric")
	for _, s := range scenarios {
		sb.WriteString(" | " + s.Label)
	}
	sb.WriteString(" |\n| :--- | :---")
	for range scenarios {
		sb.WriteString(" | :---")
	}
	sb.WriteString(" |\n")

	runs := make(map[string]map[string]PipelineRunResult)
	for _, r := range results {
		if _, ok := runs[r.Query]; !ok {
			runs[r.Query] = make(map[string]PipelineRunResult)
		}
		runs[r.Query][r.ScenarioLabel] = r
	}

	type stats struct {
		TotalRuns     int
		TotalLatency  float64
		TotalInput    int64
		TotalOutput   int64
		FallbackCount int
	}
	scenarioStats := make(map[string]*stats)
	expectedModelsByScenario := make(map[string]map[string]bool)
	for _, s := range scenarios {
		scenarioStats[s.Label] = &stats{}
		expected := make(map[string]bool)
		for _, m := range s.AgentModels {
			expected[m] = true
		}
		expectedModelsByScenario[s.Label] = expected
	}

	for _, name := range queryNames {
		query := queries[name]
		cleanName := strings.ReplaceAll(name, "_", " ")

		sb.WriteString(fmt.Sprintf("| **%s** | Pipeline Latency", cleanName))
		for _, s := range scenarios {
			r := runs[query][s.Label]
			if r.Error != "" {
				sb.WriteString(" | *Failed* ")
			} else {
				sb.WriteString(fmt.Sprintf(" | %.2fs", r.LatencySec))
				scenarioStats[s.Label].TotalRuns++
				scenarioStats[s.Label].TotalLatency += r.LatencySec
				scenarioStats[s.Label].TotalInput += int64(r.InputTokens)
				scenarioStats[s.Label].TotalOutput += int64(r.OutputTokens)
			}
		}
		sb.WriteString(" |\n")

		sb.WriteString("| | Accrued Tokens (In/Out)")
		for _, s := range scenarios {
			r := runs[query][s.Label]
			if r.Error != "" {
				sb.WriteString(" | - ")
			} else {
				sb.WriteString(fmt.Sprintf(" | %d / %d", r.InputTokens, r.OutputTokens))
			}
		}
		sb.WriteString(" |\n")

		sb.WriteString("| | Fallback Events / Notes")
		for _, s := range scenarios {
			r := runs[query][s.Label]
			if r.Error != "" {
				sb.WriteString(fmt.Sprintf(" | Error: %s", strings.ReplaceAll(r.Error, "|", "-")))
			} else {
				fallbackText := "None"
				expected := expectedModelsByScenario[s.Label]
				unexpectedCount := 0
				for m, count := range r.ModelsUsed {
					if !expected[m] {
						unexpectedCount += count
					}
				}
				if unexpectedCount > 0 {
					fallbackText = fmt.Sprintf("⚠️ Yes (%d agent calls fell back to a non-target model)", unexpectedCount)
					scenarioStats[s.Label].FallbackCount++
				}
				sb.WriteString(" | " + fallbackText)
			}
		}
		sb.WriteString(" |\n")

		sb.WriteString("| | Local HTML Matrix Page")
		for _, s := range scenarios {
			r := runs[query][s.Label]
			if r.HTMLPath != "" {
				absoluteURI := "file://" + r.HTMLPath
				sb.WriteString(fmt.Sprintf(" | [View %s Output](%s)", s.Label, absoluteURI))
			} else {
				sb.WriteString(" | N/A ")
			}
		}
		sb.WriteString(" |\n")
	}

	sb.WriteString("\n---\n\n")
	sb.WriteString("## 📉 Performance & Resource Averages Summary\n\n")
	sb.WriteString("| Scenario | Avg Pipeline Latency | Avg Input Tokens | Avg Output Tokens | Queries With Fallback Events |\n")
	sb.WriteString("| :--- | :--- | :--- | :--- | :--- |\n")

	for _, s := range scenarios {
		st := scenarioStats[s.Label]
		if st.TotalRuns > 0 {
			avgLat := st.TotalLatency / float64(st.TotalRuns)
			avgIn := float64(st.TotalInput) / float64(st.TotalRuns)
			avgOut := float64(st.TotalOutput) / float64(st.TotalRuns)
			sb.WriteString(fmt.Sprintf("| **%s** | %.2fs | %.0f | %.0f | %d |\n", s.Label, avgLat, avgIn, avgOut, st.FallbackCount))
		} else {
			sb.WriteString(fmt.Sprintf("| **%s** | *No successful runs* | - | - | - |\n", s.Label))
		}
	}

	sb.WriteString("\n\n> [!NOTE]\n")
	sb.WriteString("> **Understanding Fallback Events:** a scenario's routing map only targets its own tier's candidate/production models; if any agent call in `ModelsUsed` shows a model outside that set, `getFallbackChain` (orchestrator.go) kicked in after the primary target failed or was rate-limited for that step.\n")
	sb.WriteString("\n> **Judging quality:** run `cmd/judge_compare` with `DIR_A`/`DIR_B` pointed at `baseline` vs each candidate folder here, `LABEL_A`/`LABEL_B` set accordingly. The judge model stays `gemini-3.1-pro-preview` for both comparisons -- including the Pro-Tier one, where it is itself one of the two candidates -- to keep the grading rubric identical across both runs; that conflict of interest is a known, accepted tradeoff, not an oversight.\n")

	err := os.WriteFile(path, []byte(sb.String()), 0644)
	if err != nil {
		log.Printf("Failed to write report file: %v", err)
	}
}
