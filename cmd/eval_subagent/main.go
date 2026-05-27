package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/weitzer-org/sound-builder/internal/agents"
	"github.com/weitzer-org/sound-builder/internal/config"
	"github.com/weitzer-org/sound-builder/internal/storage"
)

type EvalRunResult struct {
	Query       string
	ModelName   string
	Content     string
	LatencySec  float64
	InputTokens int32
	OutputTokens int32
	Error       string
}

func main() {
	log.Println("🪐 Starting Tone Historian Subagent Multi-Model Benchmark Suite...")

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

	// 3. Load target system instructions for Agent 1: Tone Historian
	sysPrompt, err := agents.LoadPrompt("1_tone_historian", "")
	if err != nil {
		log.Fatalf("Failed to load Tone Historian system prompt: %v", err)
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
		"11_Mayer_Lead":        "John Mayer Trio Lead. Smooth Two-Rock/Dumble platform, mid-scooped clean with a subtle drive push.",
		"12_Bonamassa":        "Joe Bonamassa modern blues lead features, smooth tube drive into a Dumble style amplifier.",
		"13_Hard_Rock_Blues":   "hard rock blues from the 1960s - 1980s",
	}

	// Models to evaluate side-by-side
	models := []struct {
		Label string
		Model string
	}{
		{Label: "Gemini 3.1 Pro", Model: "gemini-3.1-pro-preview"},
		{Label: "Gemini 2.5 Flash", Model: "gemini-2.5-flash"},
		{Label: "Gemini 3.0 Flash", Model: "gemini-3-flash-preview"},
		{Label: "Gemini 3.5 Flash", Model: "gemini-3.5-flash"},
		{Label: "Open-LLM L4 Gateway", Model: "openllm:active-model"},
	}

	results := []EvalRunResult{}

	// Let's run queries sequentially to avoid quotas limits and log cleanly
	queryNamesOrdered := []string{
		"01_SRV_Clean", "02_Chicago_Blues", "03_British_Invasion", "04_Southern_Rock",
		"05_Clapton", "06_Gilmour", "07_Edge", "08_EVH",
		"09_BB_King", "10_Slash", "11_Mayer_Lead", "12_Bonamassa",
		"13_Hard_Rock_Blues",
	}

	firstOpenLLMCall := true

	for _, name := range queryNamesOrdered {
		query := evalQueries[name]
		log.Printf("\n=============================================================")
		log.Printf("▶ EVALUATING SCENARIO: %s (%s)", name, query)
		log.Printf("=============================================================")

		for _, mSpec := range models {
			log.Printf(" -> Executing on: %s...", mSpec.Label)

			if mSpec.Label == "Open-LLM L4 Gateway" && firstOpenLLMCall {
				log.Println("⚠️ NOTE: This is the very first Open-LLM request! If the GPU container is cold, this step may trigger a serverless cold start of up to 4.5 minutes (270 seconds). Please sit back while regional SafeTensors are loaded...")
				firstOpenLLMCall = false
			}

			// Clear statistics by instantiating a fresh tracker
			orch.Usage = &agents.TokenUsage{
				ModelsUsed: make(map[string]int),
			}

			// Configure specific agent target model
			orch.AgentModels = map[string]string{
				"1_tone_historian": mSpec.Model,
			}

			start := time.Now()
			res, err := orch.RunAgentSplit(ctx, "Tone Historian", sysPrompt, "User Request: "+query)
			duration := time.Since(start).Seconds()

			runResult := EvalRunResult{
				Query:      query,
				ModelName:  mSpec.Label,
				LatencySec: duration,
			}

			if err != nil {
				log.Printf("❌ Failed: %v", err)
				runResult.Error = err.Error()
			} else {
				log.Printf("✅ Success in %.2fs | Tokens: In %d, Out %d", duration, orch.Usage.InputTokens, orch.Usage.OutputTokens)
				runResult.Content = res
				runResult.InputTokens = orch.Usage.InputTokens
				runResult.OutputTokens = orch.Usage.OutputTokens
			}

			results = append(results, runResult)
		}
	}

	// 4. Write the results report to a dynamic markdown file
	reportPath := "/usr/local/google/home/benweitzer/.gemini/jetski/brain/7b6eb7d7-c05e-4c9f-a88f-9b308083dfff/tone_historian_benchmark_report.md"
	log.Printf("\nGenerating comparison report at: %s...", reportPath)
	generateReport(reportPath, queryNamesOrdered, evalQueries, results)
	log.Println("🏁 BENCHMARK COMPLETE! Report successfully generated.")
}

func generateReport(path string, queryNames []string, queries map[string]string, results []EvalRunResult) {
	// Find all unique model names present in the results (maintaining the order they appear)
	var modelLabels []string
	seenModels := make(map[string]bool)
	for _, r := range results {
		if !seenModels[r.ModelName] {
			seenModels[r.ModelName] = true
			modelLabels = append(modelLabels, r.ModelName)
		}
	}

	var sb strings.Builder
	sb.WriteString("# 🪐 Tone Historian Subagent - Multi-Model Quality Benchmark Report\n\n")
	sb.WriteString("An end-to-end quality and performance analysis comparing multiple Google Gemini models and our decoupled **Open-LLM Scale-to-Zero GPU Gateway** (Qwen 2.5/Llama 3.1 active set) executing the Tone Historian (Agent 1) instructions.\n\n")

	sb.WriteString("## 📊 Performance Summary Matrix\n\n")
	
	// Print Table Header
	sb.WriteString("| Scenario | Metric")
	for _, label := range modelLabels {
		sb.WriteString(" | " + label)
	}
	sb.WriteString(" |\n| :--- | :---")
	for range modelLabels {
		sb.WriteString(" | :---")
	}
	sb.WriteString(" |\n")

	// Map findings by query & model
	runs := make(map[string]map[string]EvalRunResult)
	for _, r := range results {
		if _, ok := runs[r.Query]; !ok {
			runs[r.Query] = make(map[string]EvalRunResult)
		}
		runs[r.Query][r.ModelName] = r
	}

	for _, name := range queryNames {
		query := queries[name]
		cleanName := strings.ReplaceAll(name, "_", " ")

		// Row A: Latencies
		sb.WriteString(fmt.Sprintf("| **%s** | Latency", cleanName))
		for _, label := range modelLabels {
			r := runs[query][label]
			sb.WriteString(fmt.Sprintf(" | %.2fs", r.LatencySec))
		}
		sb.WriteString(" |\n")

		// Row B: Tokens
		sb.WriteString("| | Tokens (In/Out)")
		for _, label := range modelLabels {
			r := runs[query][label]
			sb.WriteString(fmt.Sprintf(" | %d / %d", r.InputTokens, r.OutputTokens))
		}
		sb.WriteString(" |\n")

		// Row C: Status
		sb.WriteString("| | Status")
		for _, label := range modelLabels {
			r := runs[query][label]
			status := "OK"
			if r.Error != "" {
				status = "Error"
			}
			sb.WriteString(" | " + status)
		}
		sb.WriteString(" |\n")
	}

	sb.WriteString("\n---\n\n")
	sb.WriteString("## 🔍 Detailed Scenario Outputs & Prose Comparison\n\n")
	sb.WriteString("Use the carousel cards below to compare the qualitative depth, specific gear references, pickup staging insights, and technical details of the text outputs generated by each model.\n\n")

	for _, name := range queryNames {
		query := queries[name]
		cleanName := strings.ReplaceAll(name, "_", " ")
		sb.WriteString(fmt.Sprintf("### 🎸 Scenario: %s\n", cleanName))
		sb.WriteString(fmt.Sprintf("> **Target Query:** *%s*\n\n", query))

		sb.WriteString("````carousel\n")
		
		for i, label := range modelLabels {
			r := runs[query][label]
			slideMarker := ""
			if i > 0 {
				slideMarker = "<!-- slide -->\n"
			}
			sb.WriteString(slideMarker)
			sb.WriteString(fmt.Sprintf("### %s\n", label))
			sb.WriteString(fmt.Sprintf("* **Latency:** %.2fs | **Tokens:** %d In / %d Out\n\n", r.LatencySec, r.InputTokens, r.OutputTokens))
			if r.Error != "" {
				sb.WriteString(fmt.Sprintf("❌ **Error:** %s\n", r.Error))
			} else {
				sb.WriteString(r.Content)
			}
			sb.WriteString("\n")
		}
		sb.WriteString("````\n\n")
		sb.WriteString("---\n\n")
	}

	err := os.WriteFile(path, []byte(sb.String()), 0644)
	if err != nil {
		log.Printf("Failed to write report file: %v", err)
	}
}
