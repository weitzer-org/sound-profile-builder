package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"os"
	"path/filepath"

	"google.golang.org/genai"
)

// judge_compare runs a blind pairwise LLM-judge comparison between the baseline
// (pre-Tier-0) and Tier-0 pipeline outputs for every prompt in the golden set, using a
// rubric specifically aimed at what Tier 0 changed: real search grounding (vs. fabricated
// "history"/"community consensus"), and schema-enforced structural correctness. The judge
// never sees which side is which.

type Judgement struct {
	Preference          string  `json:"preference"`
	GroundingNotes       string  `json:"grounding_notes"`
	StructuralNotes      string  `json:"structural_notes"`
	Rationale            string  `json:"rationale"`
	Confidence           float64 `json:"confidence"`
}

func main() {
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		log.Fatalf("GEMINI_API_KEY must be set")
	}

	client, err := genai.NewClient(context.Background(), &genai.ClientConfig{APIKey: apiKey, Backend: genai.BackendGeminiAPI})
	if err != nil {
		log.Fatalf("GenAI client failed: %v", err)
	}
	ctx := context.Background()

	order := []string{
		"01_SRV_Clean", "02_Chicago_Blues", "03_British_Invasion", "04_Southern_Rock",
		"05_Clapton", "06_Gilmour", "07_Edge", "08_EVH",
		"09_BB_King", "10_Slash", "11_Mayer_Lead", "12_Bonamassa",
	}

	genConfig := &genai.GenerateContentConfig{
		ResponseMIMEType: "application/json",
		ResponseSchema: &genai.Schema{
			Type: genai.TypeObject,
			Properties: map[string]*genai.Schema{
				"preference":       {Type: genai.TypeString, Enum: []string{"A", "B", "Equal"}},
				"grounding_notes":  {Type: genai.TypeString, Description: "Does either preset show signs of citing/using real, specific historical or community facts vs. generic/fabricated-sounding claims?"},
				"structural_notes": {Type: genai.TypeString, Description: "Any structural/schema issues: missing fields, malformed JSON, inconsistent block naming, value ranges/hedging."},
				"rationale":        {Type: genai.TypeString},
				"confidence":       {Type: genai.TypeNumber},
			},
			Required: []string{"preference", "grounding_notes", "structural_notes", "rationale", "confidence"},
		},
	}

	winCount := map[string]int{"baseline": 0, "tier0": 0, "Equal": 0}
	type row struct {
		Name    string
		Winner  string
		Conf    float64
		Ground  string
		Struct  string
		Rationale string
	}
	var rows []row

	for _, name := range order {
		baselineData, err := os.ReadFile(filepath.Join("/tmp/qc2-eval-full/baseline", name+".json"))
		if err != nil {
			log.Printf("skip %s: %v", name, err)
			continue
		}
		tier0Data, err := os.ReadFile(filepath.Join("/tmp/qc2-eval-full/v2", name+".json"))
		if err != nil {
			log.Printf("skip %s: %v", name, err)
			continue
		}

		isTier0A := rand.Float32() > 0.5
		var aData, bData []byte
		var aLabel, bLabel string
		if isTier0A {
			aData, bData = tier0Data, baselineData
			aLabel, bLabel = "tier0", "baseline"
		} else {
			aData, bData = baselineData, tier0Data
			aLabel, bLabel = "baseline", "tier0"
		}

		prompt := fmt.Sprintf(`You are a master guitar tone/gear judge evaluating two AI-generated Quad Cortex presets for the same target: %q.

Both are JSON objects with a builder_statement, an HTML preset table (final_html_payload), a structured_payload block list, and an agent_impact log explaining what each of 11 upstream reasoning agents contributed.

Evaluate on: (1) plausibility and specificity of the tonal/historical reasoning — does it read like it's grounded in real, specific facts about the gear/artist/era, or generic/hand-wavy claims that could apply to almost any similar tone; (2) internal consistency — do the parameter choices and rationale actually support each other; (3) structural correctness — valid, complete JSON; sensible block list; no missing/malformed fields; (4) overall usefulness as an actual preset a guitarist could load and tweak.

Preset A:
%s

Preset B:
%s

Respond with your structured judgement.`, name, string(aData), string(bData))

		resp, err := client.Models.GenerateContent(ctx, "gemini-3.1-pro-preview", []*genai.Content{genai.NewContentFromText(prompt, genai.RoleUser)}, genConfig)
		if err != nil {
			log.Printf("judge failed for %s: %v", name, err)
			continue
		}

		var j Judgement
		if err := json.Unmarshal([]byte(resp.Text()), &j); err != nil {
			log.Printf("failed to parse judge output for %s: %v\nraw: %s", name, err, resp.Text())
			continue
		}

		winner := "Equal"
		if j.Preference == "A" {
			winner = aLabel
		} else if j.Preference == "B" {
			winner = bLabel
		}
		winCount[winner]++
		rows = append(rows, row{name, winner, j.Confidence, j.GroundingNotes, j.StructuralNotes, j.Rationale})
		log.Printf("⚖️ %s -> %s (conf %.2f)", name, winner, j.Confidence)
	}

	fmt.Println("\n=== JUDGE RESULTS ===")
	fmt.Printf("baseline wins: %d | tier0 wins: %d | equal: %d\n\n", winCount["baseline"], winCount["tier0"], winCount["Equal"])
	for _, r := range rows {
		fmt.Printf("--- %s ---\nWinner: %s (confidence %.2f)\nGrounding: %s\nStructural: %s\nRationale: %s\n\n", r.Name, r.Winner, r.Conf, r.Ground, r.Struct, r.Rationale)
	}

	out, _ := json.MarshalIndent(rows, "", "  ")
	os.WriteFile("/tmp/qc2-judge-results.json", out, 0644)
}
