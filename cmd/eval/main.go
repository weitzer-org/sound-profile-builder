package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"sync"
	"sync/atomic"

	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/option"
	"github.com/weitzer-org/sound-builder/internal/agents"
	"github.com/weitzer-org/sound-builder/internal/storage"
)

const qc2MonolithicPrompt = `System Instructions: Quad Cortex Systems Engineer (Generalized)

Role: You are "QC-2," a Quad Cortex Systems Engineer. Your goal is to create technical, gig-ready guitar presets using the Cortex Control Desktop App. You must prioritize physics-accurate gain staging, signal-to-noise ratio, and hardware constraints.

1. Hardware & "Physics First" Protocol
User Interface: Cortex Control (Mac Desktop).
Speaker Profile: QSC CP12 (Active 12" PA Speaker).

2. Terminology & UI Verification (Strict Adherence)
Input Logic (Global vs. Grid):
Global Input Gate (Circle 1): Uses Threshold (dB). Default assumption unless specified.
Adaptive Gate (Grid Block): Uses Noise Reduction (%). If the user mentions "%", pivot immediately to this block.
Factory Model Verification: Before recommending any Amp, Cab, or Pedal, you must verify it exists in the current Quad Cortex OS (CorOS) device list.
Pseudonym Rule: If a user asks for a vintage amp (e.g., Fender 5E3) not explicitly named, use the official Neural DSP pseudonym (e.g., "US DLX 58" or "US Tweed Basslad") and state: "Closest Available Model: [Model Name]". Do not invent model names.
Parameter Validity: Only list parameters that physically appear on the QC screen for that specific model.
Non-Master Volume Amps: For vintage circuits (Tweed, Plexi, AC30), do not list a "Master Volume" if the model lacks one. Instruct the user to use Lane Output Level for overall loudness.

3. Gain Staging & Pickup Compensation
The Headroom Rule:
To Increase Loudness (SPL): Raise Lane Output Level or Amp Block Level.
To Increase Drive/Tone: Raise Amp Block Volume or Input Gain.
Pickup Output Compensation Strategy:
Adapting Vintage/Single-Coil Tone for Humbucker User: Reduce Amp Gain/Volume by 30-50%. Lower Input Block Gain to -3.0dB or -6.0dB to prevent digital clipping/fuzz.
Adapting Humbucker Tone for Single-Coil User: Increase Input Block Gain by +3.0dB or add a Booster block.
Trigger: Always verify: "Are your pickups Vintage Output, Medium, or High Output?" before finalizing gain stages.

4. Organization Standard (Split-Bank Matrix)
Row 1 (Scenes A-D): Single Coil Profile (Telecaster). Focus on noise management and body boost.
Row 2 (Scenes E-H): Humbucker Profile (Gibson/Strat). Focus on clarity (lower Gate threshold) and definition.
Scene Functions:
A/E: Rhythm (Tight, -1.5dB output relative to Lead).
B/F: Lead (Mid-boosted, +1.5dB output).
C/G: Dry/Comping (No Reverb/Delay).
D/H: Ambient/FX.

5. Execution Protocol: The "Chameleon" Strategy
Goal: When adapting a reference tone for a specific guitar type, use the Parametric-8 EQ block rather than just gain.
Band 1 (Body): Low Shelf/Peak around 200Hz to add weight to single coils.
Band 8 (Twang): Low Pass (LPF) around 3kHz-5kHz to tame pick attack.
Output Format: Present all preset builds in this exact table format:
Table A: Main Signal Chain
Mark Scene-Specific changes clearly with "(Right-Click > Assign)".
Block CategoryModel NameRhythm Settings (Sc A/E)Lead Settings (Sc B/F)Physics/RationaleInput/Gate[Name]Thresh/Red: [X]Thresh/Red: [X][Why this setting?]Pre-FX[Name][Settings][Settings][Purpose]Amp[Real QC Name]Vol: [X]Vol: [X][Tube Taper Logic]Cab[Name][Mic/Dist][Mic/Dist][Speaker Physics]Post-FX[Name]Mix: [X]%Mix: [X]%[Spatial Goal]

6. Troubleshooting & Refinement Tree
If the user reports the tone is "Too Distorted" or "Too Fuzzy," follow this strict order of operations:
Input Pad: Lower Input Block Gain to -6.0dB (Simulates rolling off guitar volume).
Amp Gain: Reduce Amp Volume/Drive knobs by 2.0 increments.
Tube Sag: If the amp sounds "broken/farty," reduce Bass parameters (essential for Tweed/Rectifier circuits).
Output Compensation: Compensate for the volume loss by raising the Lane Output Level. Never use a compressor to fix gain issues.

7. Deep Research Trigger
If the user asks for a preset based on a specific Artist, Song, or Genre:
Identify Target: (e.g., "The Thrill Is Gone").
Hunt for Analog Specs: Console, Preamp, Mic, Amp Settings, Speaker Type.
Map to Quad Cortex: Translate analog findings into verified QC Block Models.

8. Preset Registry Protocol (Session Memory)
Goal: Maintain a full parameter history of every preset created in this session to allow for instant recall or modification.
Format: At the end of every successful build, append the full configuration to the "Session Library" below.
Session Library (Active Presets)
1. Preset Name: "Spoonful - ES339"
Target: Howlin' Wolf / Hubert Sumlin (1960).
Guitar: ES-339 (Humbuckers) w/ Pick.
Physics Goal: Clean/Edge-of-breakup rhythm + Fuzz/Sag lead without using pedals.
Full Configuration:
Block 1 (Adaptive Gate): Noise Red [Rhy: 40% / Lead: 15%], Thresh [-60dB / -65dB], Decay [100ms / 250ms].
Block 2 (EQ-8): HPF [90Hz], Band 6 [0.0dB], LPF [Rhy: 4200Hz / Lead: 4500Hz] (Simulates thumb attack).
Block 3 (Amp - US Tweed Basslad Jumped): Vol Norm [2.0 / 2.2], Vol Bright [2.5 / 3.2], Bass [2.5], Mid [6.0 / 7.0], Treble [7.0 / 6.5], Presence [6.0], Output Level [+7.0dB / +8.5dB].
Block 4 (Cab - 410 Basslad PR10): Mic A (Dyn 57, Pos 0.5, Dist 1.0"), Mic B (Ribbon 121, Pos 0.8, Dist 5.0"), Mix [A: 0dB, B: -4dB].
Block 5 (Tape Delay): Mix [15% / 22%], Time [110ms], Fdbk [15%], Drive [35%], HP [150Hz], LP [2500Hz].
Block 6 (Room Reverb): Mix [12%], Decay [0.8s], HP [120Hz], LP [3500Hz].`

func main() {
	ctx := context.Background()

	// The 12-Point Blues Evaluation Suite
	evalQueries := map[string]string{
		"01_SRV_Clean":     "Stevie Ray Vaughan 'Texas Flood' tone. Vintage single coil Strat. Needs clean Tube Screamer edge.",
		"02_Albert_King":   "Albert King 'Born Under A Bad Sign'. Medium humbuckers. Needs solid-state Acoustic amp replication.",
		"03_John_Mayer":    "John Mayer 'Gravity'. Low-output strat pickups. Dumble amp + Klon Centaur.",
		"04_Muddy_Waters":  "Muddy Waters edge-of-fuzz. Telecaster middle position. Needs a pushed 5E3 equivalent without flub.",
		"05_Howlin_Wolf":   "Howlin' Wolf / Hubert Sumlin tone. Les Paul with P90s. Filter the noise and map high-mid compensation.",
		"06_Gary_Moore":    "Gary Moore 'Still Got The Blues'. High output humbuckers. Marshall JTM45 driven heavily by a Guv'nor.",
		"07_Buddy_Guy":     "Buddy Guy 'Sweet Home Chicago'. Vintage Strat single coils. Needs Bassman with extreme treble control.",
		"08_Joe_Bonamassa": "Joe Bonamassa 'Sloe Gin'. ES-335 humbuckers. Multi-amp blend synthesis constraint.",
		"09_Robben_Ford":   "Robben Ford 'Talk To Your Daughter'. Fender Esprit humbuckers. Dumble ODS clean-to-overdrive morphing.",
		"10_Freddie_King":  "Freddie King 'Hideaway'. ES-345. Muted acoustic snap simulation (fast slapback) and Varitone filter.",
		"11_Jimi_Hendrix":  "Jimi Hendrix 'Red House'. Strat neck pickup. Fuzz Face padded front end to simulate rolled-off volume.",
		"12_Derek_Trucks":  "Derek Trucks 'Midnight in Harlem'. SG open E slide. Zero pedals, cranked Super Reverb tube sag.",
	}

	// 1. Fetch Secure Credentials
	smClient, err := storage.NewSecretManagerClient(ctx)
	if err != nil {
		log.Fatalf("Failed to init Secret Manager: %v", err)
	}
	defer smClient.Close()

	apiKey, err := smClient.GetPassword(ctx, "710019748844", "gsr-gemini-api-key")
	if err != nil {
		log.Fatalf("Failed to fetch API key: %v", err)
	}

	var wg sync.WaitGroup
	var totalMultiInput, totalMultiOutput, totalMonoInput, totalMonoOutput atomic.Int64
	
	// Max 3 concurrent execution pipelines to avoid immediate quota bans 
	sem := make(chan struct{}, 3)

	for name, query := range evalQueries {
		wg.Add(1)
		sem <- struct{}{}
		
		go func(name, query string) {
			defer wg.Done()
			defer func() { <-sem }()
		log.Printf("\n=============================================")
		log.Printf("▶ EXECUTING EVAL: %s", name)
		log.Printf("=============================================")

		// 2. RUN A: The Multi-Agent Orchestrator Pipeline
		log.Println(" -> Phase 1: 12-Agent Orchestrator...")
		orch, err := agents.NewOrchestrator(ctx, apiKey)
		if err != nil {
			log.Fatalf("Failed to init orchestrator: %v", err)
		}

		constraints := map[string]interface{}{
			"single_amp_mode":      true,
			"allow_cloud_captures": false,
		}

		multiAgentResult, usage, err := orch.RunPipeline(ctx, query, constraints)
		if err != nil {
			log.Printf("❌ Multi-Agent Pipeline failed for %s: %v", name, err)
		} else {
			log.Printf("✅ MULTI-AGENT SUCCESS | Tokens: In %d, Out %d", usage.InputTokens, usage.OutputTokens)
			totalMultiInput.Add(int64(usage.InputTokens))
			totalMultiOutput.Add(int64(usage.OutputTokens))
			err = os.WriteFile(fmt.Sprintf("%s_multi.html", name), []byte(multiAgentResult), 0644)
			if err != nil { log.Printf("File err: %v", err) }
		}
		orch.Close()

		// 3. RUN B: The Single Monolithic QC-2 Prompt
		log.Println(" -> Phase 2: Monolithic QC-2 LLM...")
		client, err := genai.NewClient(ctx, option.WithAPIKey(apiKey))
		if err != nil {
			log.Fatalf("Failed to create direct genai client: %v", err)
		}

		model := client.GenerativeModel("gemini-3.1-pro-preview")
		model.SystemInstruction = &genai.Content{
			Parts: []genai.Part{genai.Text(qc2MonolithicPrompt)},
		}

		resp, err := model.GenerateContent(ctx, genai.Text(query))
		if err != nil {
			log.Printf("❌ Monolithic generation failed for %s: %v", name, err)
		} else {
			monolithicResult := fmt.Sprintf("%v", resp.Candidates[0].Content.Parts[0])
			usageMono := resp.UsageMetadata
			
			log.Printf("✅ MONO SUCCESS | Tokens: In %d, Out %d", usageMono.PromptTokenCount, usageMono.CandidatesTokenCount)
			totalMonoInput.Add(int64(usageMono.PromptTokenCount))
			totalMonoOutput.Add(int64(usageMono.CandidatesTokenCount))
			
			err = os.WriteFile(fmt.Sprintf("%s_mono.md", name), []byte(monolithicResult), 0644)
			if err != nil { log.Printf("File err: %v", err) }
		}
		client.Close()
		}(name, query)
	}

	wg.Wait()

	log.Println("\n=============================================")
	log.Println("🏁 EVALUATION SUITE COMPLETE")
	log.Println("Wrote 24 resulting HTML and MD files to workspace.")
	log.Printf("📊 MULTI-AGENT TOTAL TOKENS: Input: %d, Output: %d", totalMultiInput.Load(), totalMultiOutput.Load())
	log.Printf("📊 MONOLITHIC  TOTAL TOKENS: Input: %d, Output: %d", totalMonoInput.Load(), totalMonoOutput.Load())
	log.Println("=============================================")
}
