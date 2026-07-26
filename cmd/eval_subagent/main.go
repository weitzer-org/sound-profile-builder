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
	"github.com/weitzer-org/sound-builder/internal/evalfixtures"
)

// eval_subagent benchmarks gemini-3.6-flash as a candidate replacement against each
// agent's current production model, one agent at a time. Only agents whose input is a
// direct function of the user query (no upstream agent output required) are covered here --
// Tone Historian, Sonic Profiler, and Community Scraper. Every other agent (4-11, 13)
// consumes prior agents' outputs as context and can't be meaningfully isolated this way;
// those are covered instead by cmd/eval_full_pipeline's mixed-routing scenarios.
const candidateModel = "gemini-3.6-flash"

type EvalRunResult struct {
	Query        string
	ModelLabel   string
	Content      string
	LatencySec   float64
	InputTokens  int32
	OutputTokens int32
	Error        string
}

type AgentSpec struct {
	Role            string // RunAgentSplit agentRole
	Key             string // agentConfig/AgentModels key
	BaselineModel   string // current production model for this agent
	BuildUserPrompt func(query string) string
}

func main() {
	log.Println("🪐 Starting gemini-3.6-flash Candidate Subagent Benchmark Suite...")

	ctx := context.Background()

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

	agentSpecs := []AgentSpec{
		{
			Role:          "Tone Historian",
			Key:           "1_tone_historian",
			BaselineModel: "gemini-3.1-pro-preview", // Pro tier
			BuildUserPrompt: func(query string) string {
				return "User Request: " + query
			},
		},
		{
			Role:          "Sonic Profiler",
			Key:           "2_sonic_profiler",
			BaselineModel: "gemini-3.5-flash", // Flash tier
			BuildUserPrompt: func(query string) string {
				return fmt.Sprintf("User Request: %s\nQC Block Parameter Vocabulary: %s", query, agents.GetQCSonicProfilerSchemaJSON())
			},
		},
		{
			Role:          "Community Scraper",
			Key:           "3_community_scraper",
			BaselineModel: "gemini-3.5-flash", // Flash tier
			BuildUserPrompt: func(query string) string {
				return "User Request: " + query
			},
		},
	}

	// The 13 Standard Archetype Evaluation Queries (shared with every other cmd/eval_* tool)
	evalQueries := evalfixtures.GoldenQueries()
	queryNamesOrdered := evalfixtures.GoldenQueryOrder()

	reportDir := "eval_results/subagent"
	if err := os.MkdirAll(reportDir, 0755); err != nil {
		log.Fatalf("Failed to create report directory: %v", err)
	}

	for _, spec := range agentSpecs {
		log.Printf("\n=============================================================")
		log.Printf("▶ AGENT: %s (baseline: %s, candidate: %s)", spec.Role, spec.BaselineModel, candidateModel)
		log.Printf("=============================================================")

		sysPrompt, err := agents.LoadPrompt(spec.Key, "")
		if err != nil {
			log.Fatalf("Failed to load system prompt for %s: %v", spec.Role, err)
		}

		modelVariants := []struct {
			Label string
			Model string
		}{
			{Label: fmt.Sprintf("Baseline (%s)", spec.BaselineModel), Model: spec.BaselineModel},
			{Label: fmt.Sprintf("Candidate (%s)", candidateModel), Model: candidateModel},
		}

		results := []EvalRunResult{}

		for _, name := range queryNamesOrdered {
			query := evalQueries[name]
			log.Printf(" -> Scenario: %s", name)

			for _, mVariant := range modelVariants {
				log.Printf("    Executing on: %s...", mVariant.Label)

				// Clear statistics by instantiating a fresh tracker
				orch.Usage = &agents.TokenUsage{
					ModelsUsed: make(map[string]int),
				}

				orch.AgentModels = map[string]string{
					spec.Key: mVariant.Model,
				}

				start := time.Now()
				res, err := orch.RunAgentSplit(ctx, spec.Role, sysPrompt, spec.BuildUserPrompt(query))
				duration := time.Since(start).Seconds()

				runResult := EvalRunResult{
					Query:      query,
					ModelLabel: mVariant.Label,
					LatencySec: duration,
				}

				if err != nil {
					log.Printf("    ❌ Failed: %v", err)
					runResult.Error = err.Error()
				} else {
					log.Printf("    ✅ Success in %.2fs | Tokens: In %d, Out %d", duration, orch.Usage.InputTokens, orch.Usage.OutputTokens)
					runResult.Content = res
					runResult.InputTokens = orch.Usage.InputTokens
					runResult.OutputTokens = orch.Usage.OutputTokens
				}

				results = append(results, runResult)
			}
		}

		reportPath := filepath.Join(reportDir, spec.Key+"_gemini36_benchmark_report.md")
		log.Printf("\nGenerating comparison report at: %s...", reportPath)
		generateReport(reportPath, spec.Role, queryNamesOrdered, evalQueries, results)
	}

	log.Println("🏁 BENCHMARK COMPLETE! Reports written to " + reportDir)
}

func generateReport(path string, agentRole string, queryNames []string, queries map[string]string, results []EvalRunResult) {
	// Find all unique model labels present in the results (maintaining the order they appear)
	var modelLabels []string
	seenModels := make(map[string]bool)
	for _, r := range results {
		if !seenModels[r.ModelLabel] {
			seenModels[r.ModelLabel] = true
			modelLabels = append(modelLabels, r.ModelLabel)
		}
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# 🪐 %s - gemini-3.6-flash Candidate Benchmark Report\n\n", agentRole))
	sb.WriteString(fmt.Sprintf("Baseline-vs-candidate quality and performance comparison for the **%s** agent.\n\n", agentRole))

	sb.WriteString("## 📊 Performance Summary Matrix\n\n")

	sb.WriteString("| Scenario | Metric")
	for _, label := range modelLabels {
		sb.WriteString(" | " + label)
	}
	sb.WriteString(" |\n| :--- | :---")
	for range modelLabels {
		sb.WriteString(" | :---")
	}
	sb.WriteString(" |\n")

	runs := make(map[string]map[string]EvalRunResult)
	for _, r := range results {
		if _, ok := runs[r.Query]; !ok {
			runs[r.Query] = make(map[string]EvalRunResult)
		}
		runs[r.Query][r.ModelLabel] = r
	}

	for _, name := range queryNames {
		query := queries[name]
		cleanName := strings.ReplaceAll(name, "_", " ")

		sb.WriteString(fmt.Sprintf("| **%s** | Latency", cleanName))
		for _, label := range modelLabels {
			r := runs[query][label]
			sb.WriteString(fmt.Sprintf(" | %.2fs", r.LatencySec))
		}
		sb.WriteString(" |\n")

		sb.WriteString("| | Tokens (In/Out)")
		for _, label := range modelLabels {
			r := runs[query][label]
			sb.WriteString(fmt.Sprintf(" | %d / %d", r.InputTokens, r.OutputTokens))
		}
		sb.WriteString(" |\n")

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
	sb.WriteString("Use the carousel cards below to compare the qualitative depth, specific gear references, and technical details of each variant's output.\n\n")

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
