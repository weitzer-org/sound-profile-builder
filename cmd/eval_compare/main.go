package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/weitzer-org/sound-builder/internal/agents"
	"github.com/weitzer-org/sound-builder/internal/config"
	"github.com/weitzer-org/sound-builder/internal/evalfixtures"
)

// eval_compare (Tier-0 side) runs the same golden prompt set as the baseline worktree
// tool, using the new orchestrator's built-in TokenUsage.PerAgent instrumentation
// directly (no log-parsing needed) plus phase-boundary latency via onProgress, and saves
// full output text for the LLM-judge quality pass.

type runResult struct {
	Query          string                        `json:"query"`
	TotalInput     int32                         `json:"total_input_tokens"`
	TotalOutput    int32                         `json:"total_output_tokens"`
	ModelsUsed     map[string]int                `json:"models_used"`
	TotalLatencyMs int64                         `json:"total_latency_ms"`
	PhaseLatencyMs map[string]int64              `json:"phase_latency_ms"`
	PerAgent       map[string]*agents.AgentUsage `json:"per_agent"`
	Error          string                        `json:"error,omitempty"`
}

func main() {
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		log.Fatalf("GEMINI_API_KEY must be set")
	}

	cfg, err := config.LoadConfig("config.json")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}
	log.Printf("Using agent_prompts (version pins): %v", cfg.AgentPrompts)

	// Shared with every other cmd/eval_* tool -- previously a locally duplicated 12-query
	// copy that had drifted from the other tools' 13-query set (missing "13_Hard_Rock_Blues").
	queries := evalfixtures.GoldenQueries
	order := evalfixtures.GoldenQueryOrder

	outDir := os.Getenv("EVAL_OUT_DIR")
	if outDir == "" {
		outDir = "/tmp/qc2-eval-full/v2"
	}
	os.MkdirAll(outDir, 0755)

	ctx := context.Background()
	results := []runResult{}

	for _, name := range order {
		query := queries[name]
		log.Printf("=== TIER-0 RUN: %s ===", name)

		orch, err := agents.NewOrchestrator(ctx, apiKey, nil)
		if err != nil {
			log.Fatalf("Failed to init orchestrator: %v", err)
		}
		orch.AgentModels = cfg.AgentModels

		constraints := evalfixtures.DefaultConstraints()

		var phaseEvents []struct {
			Phase string
			AtMs  int64
		}
		start := time.Now()
		onProgress := func(phase string) {
			phaseEvents = append(phaseEvents, struct {
				Phase string
				AtMs  int64
			}{phase, time.Since(start).Milliseconds()})
		}

		output, usage, err := orch.RunPipeline(ctx, query, constraints, cfg.AgentPrompts, onProgress)
		orch.Close()

		rr := runResult{Query: name, PhaseLatencyMs: map[string]int64{}}
		if err != nil {
			rr.Error = err.Error()
			log.Printf("❌ %s failed: %v", name, err)
			results = append(results, rr)
			continue
		}

		rr.TotalInput = usage.InputTokens
		rr.TotalOutput = usage.OutputTokens
		rr.ModelsUsed = usage.ModelsUsed
		rr.TotalLatencyMs = usage.TotalLatencyMs
		rr.PerAgent = usage.PerAgent
		for _, pe := range phaseEvents {
			rr.PhaseLatencyMs[pe.Phase] = pe.AtMs
		}

		os.WriteFile(filepath.Join(outDir, name+".json"), []byte(output), 0644)
		log.Printf("✅ %s: total in=%d out=%d latency=%dms", name, rr.TotalInput, rr.TotalOutput, rr.TotalLatencyMs)
		results = append(results, rr)
	}

	out, _ := json.MarshalIndent(results, "", "  ")
	os.WriteFile(filepath.Join(outDir, "_summary.json"), out, 0644)
	fmt.Println(string(out))
}
