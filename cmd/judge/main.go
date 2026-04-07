package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/generative-ai-go/genai"
	"github.com/weitzer-org/sound-builder/internal/storage"
	"google.golang.org/api/option"
)

type Judgement struct {
	Preference string  `json:"preference"`
	Rationale  string  `json:"rationale"`
	Confidence float64 `json:"confidence"`
}

func main() {
	ctx := context.Background()

	// 1. Fetch Secure Credentials
	smClient, err := storage.NewSecretManagerClient(ctx)
	if err != nil {
		log.Fatalf("Failed to init Secret Manager: %v", err)
	}
	defer smClient.Close()

	// Hardcode or load config to get project ID
	// For simplicity, let's try to load config.json or use fallback.
	projectID := "710019748844"
	apiKey, err := smClient.GetPassword(ctx, projectID, "gsr-gemini-api-key")
	if err != nil {
		log.Fatalf("Failed to fetch API key: %v", err)
	}

	client, err := genai.NewClient(ctx, option.WithAPIKey(apiKey))
	if err != nil {
		log.Fatalf("GenAI client failed: %v", err)
	}
	defer client.Close()

	baselineDir := os.Getenv("DIR_A")
	if baselineDir == "" {
		baselineDir = "eval_results"
	}
	labelA := filepath.Base(baselineDir)
	if labelA == "eval_results" || labelA == "." {
		labelA = "Baseline"
	}

	var ablationPath string
	var labelB string
	dirB := os.Getenv("DIR_B")
	if dirB != "" {
		ablationPath = dirB
		labelB = filepath.Base(dirB)
	} else {
		ablationDir := os.Getenv("ABLATION_SUBDIR")
		if ablationDir == "" {
			log.Fatalf("ABLATION_SUBDIR env var is required when DIR_B is not set")
		}
		ablationPath = filepath.Join(baselineDir, "ablation", ablationDir)
		labelB = "Ablation (" + ablationDir + ")"
	}

	files, err := filepath.Glob(filepath.Join(ablationPath, "*.html"))
	if err != nil {
		log.Fatalf("Failed to find ablated files: %v", err)
	}

	if len(files) == 0 {
		log.Printf("No files found in %s", ablationPath)
		return
	}

	for _, ablatedFile := range files {
		basename := filepath.Base(ablatedFile)
		baselineFile := filepath.Join(baselineDir, basename)

		if _, err := os.Stat(baselineFile); os.IsNotExist(err) {
			log.Printf("Skipping %s, baseline not found", basename)
			continue
		}

		ablatedData, _ := ioutil.ReadFile(ablatedFile)
		baselineData, _ := ioutil.ReadFile(baselineFile)

		isAblatedA := rand.Float32() > 0.5
		var aData, bData []byte
		if isAblatedA {
			aData, bData = ablatedData, baselineData
		} else {
			aData, bData = baselineData, ablatedData
		}

		prompt := fmt.Sprintf(`You are a master guitar tone judge. Compare Preset A and Preset B for a %s recreation.
Analyze the signal chain, settings, and output quality.
Tell me which one is better and why.

Preset A:
%s

Preset B:
%s

Respond ONLY in JSON format:
{
  "preference": "A" or "B" or "Equal",
  "rationale": "...",
  "confidence": 0.0-1.0
}
`, strings.ReplaceAll(basename, "_multi.html", ""), string(aData), string(bData))

		model := client.GenerativeModel("gemini-3.1-pro-preview")
		resp, err := model.GenerateContent(ctx, genai.Text(prompt))
		if err != nil {
			log.Printf("Failed to judge %s: %v", basename, err)
			continue
		}

		resultText := fmt.Sprintf("%v", resp.Candidates[0].Content.Parts[0])
		log.Printf("\n⚖️ --- JUDGEMENT FOR %s ---", basename)
		log.Println(resultText)

		// Unmarshal to verify JSON
		var judge Judgement
		cleanJSON := strings.TrimSpace(resultText)
		cleanJSON = strings.TrimPrefix(cleanJSON, "```json")
		cleanJSON = strings.TrimSuffix(cleanJSON, "```")
		cleanJSON = strings.TrimSpace(cleanJSON)
		
		if err := json.Unmarshal([]byte(cleanJSON), &judge); err != nil {
			log.Printf("Failed to parse judge JSON: %v", err)
			continue
		}

		winner := labelA
		if (judge.Preference == "A" && isAblatedA) || (judge.Preference == "B" && !isAblatedA) {
			winner = labelB
		} else if judge.Preference == "Equal" {
			winner = "Equal"
		}

		log.Printf("🏆 WINNER: %v (Confidence: %.2f)", winner, judge.Confidence)
	}
}
