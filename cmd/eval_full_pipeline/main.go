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
	"github.com/weitzer-org/sound-builder/internal/storage"
)

type PipelineRunResult struct {
	Query        string
	ScenarioName string
	ModelLabel   string
	LatencySec   float64
	InputTokens  int32
	OutputTokens int32
	ModelsUsed   map[string]int
	HTMLPath     string
	Error        string
}

func main() {
	log.Println("🪐 Starting Full 12-Agent Pipeline Multi-Model Benchmark Suite...")

	ctx := context.Background()
	cfg, err := config.LoadConfig("config.json")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// 1. Initialize Credentials dynamically from Secret Manager
	smClient, err := storage.NewSecretManagerClient(ctx)
	if err != nil {
		log.Fatalf("Failed to initialize Secret Manager: %v", err)
	}
	defer smClient.Close()

	log.Println(" -> Fetching credentials from Google Secret Manager...")
	geminiKey, err := smClient.GetPassword(ctx, cfg.ProjectID, "gsr-gemini-api-key")
	if err != nil {
		log.Fatalf("Failed to fetch Gemini API key: %v", err)
	}

	openLLMKey, err := smClient.GetPassword(ctx, cfg.ProjectID, "open-llm-api-auth-secret")
	if err != nil {
		log.Printf("Warning: Failed to fetch open-llm-api-auth-secret: %v. Relying on OPENLLM_API_KEY env variable.", err)
	} else {
		os.Setenv("OPENLLM_API_KEY", openLLMKey)
	}

	// 2. Instantiate main Orchestrator in hybrid mode (both keys active)
	orch, err := agents.NewOrchestrator(ctx, geminiKey, nil)
	if err != nil {
		log.Fatalf("Failed to initialize Orchestrator: %v", err)
	}
	defer orch.Close()

	// Direct all 12 sequential/concurrent sub-agent steps to our target models
	agentKeys := []string{
		"1_tone_historian", "2_sonic_profiler", "3_community_scraper",
		"4_coros_librarian", "5_cloud_navigator", "6_acoustician",
		"7_transducer_tech", "8_foh_optimizer", "9_mix_engineer",
		"10_control_mapper", "11_dsp_dispatcher", "12_architect",
	}

	// The 12 Standard Archetype Evaluation Queries
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
		"11_Mayer_Lead":        "John Mayer Trio Lead. Smooth Two-Rock/Dumble platform, mid-scooped clean with a subtle drive push.",
		"12_Bonamassa":        "Joe Bonamassa modern blues lead features, smooth tube drive into a Dumble style amplifier.",
		"13_Hard_Rock_Blues":   "hard rock blues from the 1960s - 1980s",
	}

	// Chronological & Performance Models lineup to evaluate side-by-side
	models := []struct {
		Label      string
		ModelName  string
		FolderName string
	}{
		{Label: "Gemini 3.1 Pro", ModelName: "gemini-3.1-pro-preview", FolderName: "gemini-3.1-pro"},
		{Label: "Gemini 2.5 Pro", ModelName: "gemini-2.5-pro", FolderName: "gemini-2.5-pro"},
		{Label: "Gemini 2.5 Flash", ModelName: "gemini-2.5-flash", FolderName: "gemini-2.5-flash"},
		{Label: "Gemini 3.0 Flash", ModelName: "gemini-3-flash-preview", FolderName: "gemini-3.0-flash"},
		{Label: "Gemini 3.5 Flash", ModelName: "gemini-3.5-flash", FolderName: "gemini-3.5-flash"},
		{Label: "Open-LLM L4 Gateway", ModelName: "openllm:active-model", FolderName: "open-llm-gateway"},
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
	firstOpenLLMCall := true

	// Standard sequential execution to prevent API rate limits across the massive 72-pipeline runs (864 individual LLM requests!)
	for _, mSpec := range models {
		log.Printf("\n🚀 STARTING MODEL RUN MATRIX: %s", mSpec.Label)

		// Create target folder
		modelDir := filepath.Join(baseOutDir, mSpec.FolderName)
		if err := os.MkdirAll(modelDir, 0755); err != nil {
			log.Fatalf("Failed to create folder: %v", err)
		}

		// Configure all 12 steps to target this model
		modelMap := make(map[string]string)
		for _, key := range agentKeys {
			modelMap[key] = mSpec.ModelName
		}
		orch.AgentModels = modelMap

		for _, name := range queryNamesOrdered {
			query := evalQueries[name]
			log.Printf("\nScenario: %s | Model: %s", name, mSpec.Label)

			if mSpec.Label == "Open-LLM L4 Gateway" && firstOpenLLMCall {
				log.Println("⚠️ NOTE: This is the first Open-LLM pipeline request! Standard serverless GPU cold boot may wait up to 4.5 minutes to load FUSE SafeTensors weights...")
				firstOpenLLMCall = false
			}

			// Clear statistics
			orch.Usage = &agents.TokenUsage{
				ModelsUsed: make(map[string]int),
			}

			start := time.Now()
			htmlResult, tokenUsage, err := orch.RunPipeline(ctx, query, constraints, cfg.AgentPrompts, nil)
			duration := time.Since(start).Seconds()

			runResult := PipelineRunResult{
				Query:        query,
				ScenarioName: name,
				ModelLabel:   mSpec.Label,
				LatencySec:   duration,
			}

			if err != nil {
				log.Printf(" ❌ Failed pipeline run: %v", err)
				runResult.Error = err.Error()
			} else {
				log.Printf("  Success in %.2fs | Input Tokens: %d, Output Tokens: %d", duration, tokenUsage.InputTokens, tokenUsage.OutputTokens)
				runResult.InputTokens = tokenUsage.InputTokens
				runResult.OutputTokens = tokenUsage.OutputTokens

				// Copy models used map
				runResult.ModelsUsed = make(map[string]int)
				for k, v := range tokenUsage.ModelsUsed {
					runResult.ModelsUsed[k] = v
				}

				// Save the HTML output locally
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

	// Generate Report Matrix
	reportPath := "/usr/local/google/home/benweitzer/.gemini/jetski/brain/7b6eb7d7-c05e-4c9f-a88f-9b308083dfff/full_pipeline_benchmark_report.md"
	log.Printf("\nGenerating full-pipeline summary report at: %s...", reportPath)
	generateReport(reportPath, queryNamesOrdered, evalQueries, results, models)
	log.Println("🏁 FULL BENCHMARK SUITE COMPLETE! Walkthrough report written successfully.")
}

func generateReport(path string, queryNames []string, queries map[string]string, results []PipelineRunResult, models []struct{ Label, ModelName, FolderName string }) {
	var sb strings.Builder
	sb.WriteString("# 🪐 Full 12-Agent Pipeline - Multi-Model Quality & Performance Benchmark Report\n\n")
	sb.WriteString("An end-to-end performance and qualitative quality benchmark analysis evaluating our complete **12-Agent Sound Builder Pipeline** across 6 distinct Google Gemini and Open-LLM model targets.\n\n")

	sb.WriteString("## 📊 Telemetry Summary Matrix\n\n")
	
	// Print Table Header
	sb.WriteString("| Scenario | Metric")
	for _, m := range models {
		sb.WriteString(" | " + m.Label)
	}
	sb.WriteString(" |\n| :--- | :---")
	for range models {
		sb.WriteString(" | :---")
	}
	sb.WriteString(" |\n")

	// Map findings by query & model
	runs := make(map[string]map[string]PipelineRunResult)
	for _, r := range results {
		if _, ok := runs[r.Query]; !ok {
			runs[r.Query] = make(map[string]PipelineRunResult)
		}
		runs[r.Query][r.ModelLabel] = r
	}

	// Summary Statistics Accumulator
	type stats struct {
		TotalRuns     int
		TotalLatency  float64
		TotalInput    int64
		TotalOutput   int64
		FallbackCount int
	}
	modelStats := make(map[string]*stats)
	for _, m := range models {
		modelStats[m.Label] = &stats{}
	}

	for _, name := range queryNames {
		query := queries[name]
		cleanName := strings.ReplaceAll(name, "_", " ")

		// Row 1: Latencies
		sb.WriteString(fmt.Sprintf("| **%s** | Pipeline Latency", cleanName))
		for _, m := range models {
			r := runs[query][m.Label]
			if r.Error != "" {
				sb.WriteString(" | *Failed* ")
			} else {
				sb.WriteString(fmt.Sprintf(" | %.2fs", r.LatencySec))
				modelStats[m.Label].TotalRuns++
				modelStats[m.Label].TotalLatency += r.LatencySec
				modelStats[m.Label].TotalInput += int64(r.InputTokens)
				modelStats[m.Label].TotalOutput += int64(r.OutputTokens)
			}
		}
		sb.WriteString(" |\n")

		// Row 2: Tokens
		sb.WriteString("| | Accrued Tokens (In/Out)")
		for _, m := range models {
			r := runs[query][m.Label]
			if r.Error != "" {
				sb.WriteString(" | - ")
			} else {
				sb.WriteString(fmt.Sprintf(" | %d / %d", r.InputTokens, r.OutputTokens))
			}
		}
		sb.WriteString(" |\n")

		// Row 3: Fallback Detection
		sb.WriteString("| | Fallback Events / Notes")
		for _, m := range models {
			r := runs[query][m.Label]
			if r.Error != "" {
				sb.WriteString(fmt.Sprintf(" | Error: %s", strings.ReplaceAll(r.Error, "|", "-")))
			} else {
				// Detect if gemini-3.1-pro-preview was used in Open-LLM run (meaning fallback happened!)
				fallbackText := "None"
				if m.Label == "Open-LLM L4 Gateway" {
					proCount := r.ModelsUsed["gemini-3.1-pro-preview"]
					if proCount > 0 {
						fallbackText = fmt.Sprintf("⚠️ Yes (%d steps fallback)", proCount)
						modelStats[m.Label].FallbackCount++
					}
				}
				sb.WriteString(" | " + fallbackText)
			}
		}
		sb.WriteString(" |\n")

		// Row 4: HTML output link
		sb.WriteString("| | Local HTML Matrix Page")
		for _, m := range models {
			r := runs[query][m.Label]
			if r.HTMLPath != "" {
				// Convert to dynamic local link scheme path
				absoluteURI := "file://" + r.HTMLPath
				sb.WriteString(fmt.Sprintf(" | [View %s Output](%s)", m.Label, absoluteURI))
			} else {
				sb.WriteString(" | N/A ")
			}
		}
		sb.WriteString(" |\n")
	}

	sb.WriteString("\n---\n\n")
	sb.WriteString("## 📉 Performance & Resource Averages Summary\n\n")
	sb.WriteString("| Model Target | Avg Pipeline Latency | Avg Input Tokens | Avg Output Tokens | Total Fallback Events |\n")
	sb.WriteString("| :--- | :--- | :--- | :--- | :--- |\n")
	
	for _, m := range models {
		st := modelStats[m.Label]
		if st.TotalRuns > 0 {
			avgLat := st.TotalLatency / float64(st.TotalRuns)
			avgIn := float64(st.TotalInput) / float64(st.TotalRuns)
			avgOut := float64(st.TotalOutput) / float64(st.TotalRuns)
			sb.WriteString(fmt.Sprintf("| **%s** | %.2fs | %.0f | %.0f | %d |\n", m.Label, avgLat, avgIn, avgOut, st.FallbackCount))
		} else {
			sb.WriteString(fmt.Sprintf("| **%s** | *No successful runs* | - | - | - |\n", m.Label))
		}
	}

	sb.WriteString("\n\n> [!NOTE]\n")
	sb.WriteString("> **Understanding Fallback Events:** When Open-LLM hits context limit sizes, it triggers a warning log and automatically falls back to standard Gemini 3.1 Pro (our default production model) for that specific agent step. This guarantees continuous and successful pipeline runs while letting us log exactly where context boundary issues are occurring.\n")

	err := os.WriteFile(path, []byte(sb.String()), 0644)
	if err != nil {
		log.Printf("Failed to write report file: %v", err)
	}
}
