package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"google.golang.org/genai"
)

// critic_probe is a cheap, one-off validation step for the Tier 2 critic-agent ablation
// question: before spending a full live-eval round generating fresh presets with a critic
// agent wired into the pipeline, run the proposed critic prompt directly against the 12
// Architect outputs already sitting on disk from the last validated Tier 1 eval round (no
// new generation, just 12 small critic-only calls) to see whether it finds anything real.
// If it doesn't, the full ablation isn't worth running; if it does, that's the concrete
// evidence needed to justify the bigger investment.

type Issue struct {
	Guitar   string `json:"guitar"`
	BlockID  string `json:"block_id"`
	Issue    string `json:"issue"`
	Severity string `json:"severity"`
}

type CriticResult struct {
	Issues []Issue `json:"issues"`
}

const criticPrompt = `You are the Preset Critic, a fresh, skeptical second reader of an already-assembled Quad Cortex guitar preset. You do NOT re-derive the preset. Your only job is to catch internal inconsistencies between what the preset SAYS (builder_statement, block rationales) and what the preset's structured data actually DOES.

Check ONLY these two things:
1. Scene-state consistency: for each block with a "Bypass" parameter, does the actual Bypass value/value_b match what the builder_statement or that block's own rationale claims is active/inactive in Scene A (Rhythm) vs Scene B (Lead)? Flag any block where the prose says one thing and the Bypass data says the opposite.
2. Prose-data gear consistency: does every specific piece of gear or effect the builder_statement or a block's rationale explicitly names actually appear as a real block in structured_payload, and vice versa (no block whose rationale contradicts its own listed Model field)?

Do not flag anything else -- not tonal quality, not historical accuracy, not parameter value plausibility, not gear choice, not the basis field, not formatting. Those are out of scope for this check. Return an empty issues array if nothing is genuinely wrong -- do not invent an issue just to have something to report.

Preset to review:
%s`

func main() {
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		log.Fatalf("GEMINI_API_KEY must be set")
	}
	dir := os.Getenv("PROBE_DIR")
	if dir == "" {
		dir = "/tmp/qc2-eval-full/tier1"
	}

	client, err := genai.NewClient(context.Background(), &genai.ClientConfig{APIKey: apiKey, Backend: genai.BackendGeminiAPI})
	if err != nil {
		log.Fatalf("GenAI client failed: %v", err)
	}
	ctx := context.Background()

	genConfig := &genai.GenerateContentConfig{
		Temperature:      func() *float32 { t := float32(0.1); return &t }(),
		ResponseMIMEType: "application/json",
		ResponseSchema: &genai.Schema{
			Type: genai.TypeObject,
			Properties: map[string]*genai.Schema{
				"issues": {
					Type: genai.TypeArray,
					Items: &genai.Schema{
						Type: genai.TypeObject,
						Properties: map[string]*genai.Schema{
							"guitar":   {Type: genai.TypeString},
							"block_id": {Type: genai.TypeString},
							"issue":    {Type: genai.TypeString},
							"severity": {Type: genai.TypeString, Enum: []string{"high", "medium"}},
						},
						Required: []string{"guitar", "block_id", "issue", "severity"},
					},
				},
			},
			Required: []string{"issues"},
		},
	}

	files, err := filepath.Glob(filepath.Join(dir, "*.json"))
	if err != nil {
		log.Fatalf("glob failed: %v", err)
	}

	totalIn, totalOut := int32(0), int32(0)
	totalIssues := 0
	for _, f := range files {
		base := filepath.Base(f)
		if base == "_summary.json" {
			continue
		}
		data, err := os.ReadFile(f)
		if err != nil {
			log.Printf("skip %s: %v", base, err)
			continue
		}

		prompt := fmt.Sprintf(criticPrompt, string(data))
		resp, err := client.Models.GenerateContent(ctx, "gemini-3.1-pro-preview", []*genai.Content{genai.NewContentFromText(prompt, genai.RoleUser)}, genConfig)
		if err != nil {
			log.Printf("critic failed for %s: %v", base, err)
			continue
		}
		if resp.UsageMetadata != nil {
			totalIn += resp.UsageMetadata.PromptTokenCount
			totalOut += resp.UsageMetadata.CandidatesTokenCount
		}

		var result CriticResult
		if err := json.Unmarshal([]byte(resp.Text()), &result); err != nil {
			log.Printf("failed to parse critic output for %s: %v\nraw: %s", base, err, resp.Text())
			continue
		}

		fmt.Printf("=== %s ===\n", base)
		if len(result.Issues) == 0 {
			fmt.Println("  (no issues found)")
		}
		for _, iss := range result.Issues {
			fmt.Printf("  [%s] %s / %s: %s\n", iss.Severity, iss.Guitar, iss.BlockID, iss.Issue)
			totalIssues++
		}
	}

	fmt.Printf("\n=== SUMMARY ===\nTotal issues found: %d\nTotal tokens: in=%d out=%d\n", totalIssues, totalIn, totalOut)
}
