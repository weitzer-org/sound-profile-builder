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

	"google.golang.org/genai"
)

type Judgement struct {
	Preference string  `json:"preference"`
	Rationale  string  `json:"rationale"`
	Confidence float64 `json:"confidence"`
}

func main() {
	ctx := context.Background()

	// 1. Fetch Credentials. Reads GEMINI_API_KEY directly rather than requiring a GCP
	// project + Secret Manager access.
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		log.Fatalf("GEMINI_API_KEY must be set to run the judge")
	}

	client, err := genai.NewClient(ctx, &genai.ClientConfig{APIKey: apiKey, Backend: genai.BackendGeminiAPI})
	if err != nil {
		log.Fatalf("GenAI client failed: %v", err)
	}

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

		genConfig := &genai.GenerateContentConfig{
			ResponseMIMEType: "application/json",
			ResponseSchema: &genai.Schema{
				Type: genai.TypeObject,
				Properties: map[string]*genai.Schema{
					"preference": {Type: genai.TypeString, Enum: []string{"A", "B", "Equal"}},
					"rationale":  {Type: genai.TypeString},
					"confidence": {Type: genai.TypeNumber},
				},
				Required: []string{"preference", "rationale", "confidence"},
			},
		}
		resp, err := client.Models.GenerateContent(ctx, "gemini-3.1-pro-preview", []*genai.Content{genai.NewContentFromText(prompt, genai.RoleUser)}, genConfig)
		if err != nil {
			log.Printf("Failed to judge %s: %v", basename, err)
			continue
		}

		resultText := resp.Text()
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
