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

	queries := map[string]string{
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
	}
	order := []string{
		"01_SRV_Clean", "02_Chicago_Blues", "03_British_Invasion", "04_Southern_Rock",
		"05_Clapton", "06_Gilmour", "07_Edge", "08_EVH",
		"09_BB_King", "10_Slash", "11_Mayer_Lead", "12_Bonamassa",
	}

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

		constraints := map[string]interface{}{
			"single_amp_mode":        true,
			"allow_cloud_captures":   false,
			"allow_factory_captures": true,
			"favor_captures":         true,
			"guitars":                []string{"Gibson ES-339 Humbuckers", "Fender Telecaster Single Coil"},
		}

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
