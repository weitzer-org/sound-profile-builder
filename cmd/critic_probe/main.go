package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/weitzer-org/sound-builder/internal/agents"
	"google.golang.org/genai"
)

// critic_probe is a cheap, one-off/repeatable validation step for the Preset Critic agent:
// run it directly against Architect outputs already sitting on disk (no new pipeline
// generation) to check its findings without spending a full live-eval round. Originally
// built to answer the Tier 2 critic-agent ablation question (does a dedicated critic catch
// anything a deterministic check can't); kept as a standing tool for validating prompt
// changes to prompts/13_critic.md the same cheap way -- e.g. it caught its own
// Bypass-semantics bug this way before that fix ever reached a live eval.
//
// Loads the real prompts/13_critic.md via agents.LoadPrompt rather than duplicating the
// checklist as a Go string constant, specifically so this tool and the live pipeline can
// never drift out of sync the way an earlier hardcoded copy briefly did.

// criticResult wraps the shared agents.CriticIssue type (the same struct the live pipeline
// parses the critic's response into) rather than redeclaring the per-issue field shape
// here, so a field change in the real type can't silently leave the probe parsing a stale
// shape. The one-field wrapper is trivial and package-local only because agents' own
// response wrapper is unexported.
type criticResult struct {
	Issues []agents.CriticIssue `json:"issues"`
}

const criticPromptTemplate = "%s\n\nPreset to review:\n%s"

func main() {
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		log.Fatalf("GEMINI_API_KEY must be set")
	}
	dir := os.Getenv("PROBE_DIR")
	if dir == "" {
		dir = "/tmp/qc2-eval-full/tier1"
	}

	criticSystemPrompt, err := agents.LoadPrompt("13_critic", "")
	if err != nil {
		log.Fatalf("failed to load prompts/13_critic.md: %v", err)
	}

	client, err := genai.NewClient(context.Background(), &genai.ClientConfig{APIKey: apiKey, Backend: genai.BackendGeminiAPI})
	if err != nil {
		log.Fatalf("GenAI client failed: %v", err)
	}
	ctx := context.Background()

	temp := float32(0.1)
	genConfig := &genai.GenerateContentConfig{
		Temperature:      &temp,
		ResponseMIMEType: "application/json",
		ResponseSchema:   agents.AgentResponseSchema("13_critic"),
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

		prompt := fmt.Sprintf(criticPromptTemplate, criticSystemPrompt, string(data))
		resp, err := client.Models.GenerateContent(ctx, "gemini-3.1-pro-preview", []*genai.Content{genai.NewContentFromText(prompt, genai.RoleUser)}, genConfig)
		if err != nil {
			log.Printf("critic failed for %s: %v", base, err)
			continue
		}
		if resp.UsageMetadata != nil {
			totalIn += resp.UsageMetadata.PromptTokenCount
			totalOut += resp.UsageMetadata.CandidatesTokenCount
		}

		var result criticResult
		if err := json.Unmarshal([]byte(agents.StripJSONFences(resp.Text())), &result); err != nil {
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
