package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/weitzer-org/sound-builder/internal/agents"
	"github.com/weitzer-org/sound-builder/internal/config"
	"github.com/weitzer-org/sound-builder/internal/storage"
)

func main() {
	ctx := context.Background()

	// 1. Fetch Secure Credentials
	smClient, err := storage.NewSecretManagerClient(ctx)
	if err != nil {
		log.Fatalf("Failed to init Secret Manager: %v", err)
	}
	defer smClient.Close()

	cfg, err := config.LoadConfig("config.json")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	apiKey, err := smClient.GetPassword(ctx, cfg.ProjectID, "gsr-gemini-api-key")
	if err != nil {
		log.Fatalf("Failed to fetch API key: %v", err)
	}

	orch, err := agents.NewOrchestrator(ctx, apiKey)
	if err != nil {
		log.Fatalf("Failed to init orchestrator: %v", err)
	}
	defer orch.Close()

	evals := []struct {
		Name     string
		File     string
		Question string
	}{
		{"01_SRV_Clean", "eval_results/01_SRV_Clean_multi.html", "Why did you pick that specific cabinet style for this blues tone?"},
		{"02_Metal_Rhythm", "eval_results/02_Metal_Rhythm_multi.html", "Why did you pick the Green overdrive pedal instead of just using the amp's gain?"},
		{"03_Ambient_Lead", "eval_results/03_Ambient_Lead_multi.html", "Why did you choose that particular delay and plate reverb timing?"},
		{"04_Classic_Rock", "eval_results/04_Classic_Rock_multi.html", "Why did you choose to cut the low shelf pre-EQ so aggressively for the Gibson?"},
	}

	for _, e := range evals {
		log.Printf("\n=============================================")
		log.Printf("▶ EXECUTING REFINEMENT EVAL: %s", e.Name)
		log.Printf("Question: %s", e.Question)

		data, err := os.ReadFile(e.File)
		if err != nil {
			log.Printf("Skipping %s, file read error: %v", e.Name, err)
			continue
		}

		dataString := strings.TrimPrefix(string(data), "```json\n")
		dataString = strings.TrimSuffix(dataString, "\n```\n")
		dataString = strings.TrimSuffix(dataString, "\n```")

		var archResp struct {
			FinalHtmlPayload map[string]string `json:"final_html_payload"`
		}
		if err := json.Unmarshal([]byte(dataString), &archResp); err != nil {
			log.Printf("Skipping %s, JSON parse error: %v", e.Name, err)
			continue
		}

		payloadBytes, _ := json.Marshal(archResp.FinalHtmlPayload)
		p := &storage.Preset{
			Name:    e.Name,
			Payload: string(payloadBytes),
			ChatHistory: []storage.ChatMessage{
				{Role: "user", Content: "Generate preset."},
				{Role: "model", Content: "Preset created."},
			},
		}

		result, _, err := orch.RefineChat(ctx, p, e.Question)
		if err != nil {
			log.Printf("Refinement failed for %s: %v", e.Name, err)
			continue
		}

		outPath := fmt.Sprintf("eval_results/%s_refine.json", e.Name)
		os.WriteFile(outPath, []byte(result), 0644)
		log.Printf("Saved refinement response to %s", outPath)
	}
}
