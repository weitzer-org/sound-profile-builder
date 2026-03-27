package agents

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/google/generative-ai-go/genai"
	"github.com/weitzer-org/sound-builder/internal/storage"
	"google.golang.org/api/option"
)

// ContextKey establishes standard context keys for the agents package
type ContextKey string

const (
	MockModeKey ContextKey = "mock_mode"
)

// TokenUsage rigidly aggregates exact API volume metadata
type TokenUsage struct {
	InputTokens  int32
	OutputTokens int32
	ModelsUsed   map[string]int
	mu           sync.Mutex
}

// OrchestratorService defines the methods available on the ADK Orchestrator
type OrchestratorService interface {
	RunPipeline(ctx context.Context, prompt string, constraints map[string]interface{}) (string, *TokenUsage, error)
	RefineChat(ctx context.Context, p *storage.Preset, userMessage string) (string, *TokenUsage, error)
	Close()
}

// Orchestrator manages the 12-agent pipeline through 4 execution phases
type Orchestrator struct {
	client *genai.Client
	Usage  *TokenUsage
}

// NewOrchestrator initializes the Gemini ADK client
func NewOrchestrator(ctx context.Context, apiKey string, opts ...option.ClientOption) (*Orchestrator, error) {
	allOpts := append([]option.ClientOption{option.WithAPIKey(apiKey)}, opts...)
	client, err := genai.NewClient(ctx, allOpts...)
	if err != nil {
		return nil, err
	}
	// Initialize concurrency-safe Token Tracker
	usage := &TokenUsage{
		ModelsUsed: make(map[string]int),
	}

	return &Orchestrator{client: client, Usage: usage}, nil
}

// RunPipeline takes the user's prompt and routes it through the 12 agents
func (o *Orchestrator) RunPipeline(ctx context.Context, prompt string, constraints map[string]interface{}) (string, *TokenUsage, error) {
	if mockVal, ok := ctx.Value(MockModeKey).(bool); ok && mockVal {
		if mockOutput, err := readMockFile("testdata/e2e_mocks/architect_generate.json"); err == nil {
			return mockOutput, o.Usage, nil
		}
	}

	// Apply global application architecture limits dynamically into the root user prompt stream
	singleAmpMode, ok := constraints["single_amp_mode"].(bool)
	if ok && singleAmpMode {
		prompt = "[GLOBAL USER CONFIG FLAG: SINGLE AMP ONLY. The generated preset MUST route through one single amplifier block.]\n\n" + prompt
	}

	log.Printf("Starting ADK Pipeline for prompt: %s\n", prompt)

	execID := fmt.Sprintf("%d", time.Now().UnixNano())
	gcs, err := storage.NewGCSClient(ctx)
	if err != nil {
		log.Printf("Warning: failed to init GCS client for logging: %v", err)
	} else {
		defer gcs.Close()
	}

	logToGCS := func(agentNum string, result string) {
		if gcs != nil {
			err := gcs.WriteFile(ctx, "weitzer-sound-builder", fmt.Sprintf("logs/%s/%s.json", execID, agentNum), []byte(result))
			if err != nil {
				log.Printf("Failed to log %s to GCS: %v", agentNum, err)
			}
		}
	}

	// Phase 1: Research & Reality (Concurrent Goroutines)
	var wg1 sync.WaitGroup
	var toneResult, sonicResult, scrapeResult string
	var err1, err2, err3 error

	wg1.Add(3)

	go func() {
		defer wg1.Done()
		sysPrompt, _ := LoadPrompt("1_tone_historian")
		toneResult, err1 = o.RunAgent(ctx, "Tone Historian", sysPrompt+"\n\nUser Request: "+prompt)
		logToGCS("1_tone_historian", toneResult)
	}()

	go func() {
		defer wg1.Done()
		sysPrompt, _ := LoadPrompt("2_sonic_profiler")
		sonicResult, err2 = o.RunAgent(ctx, "Sonic Profiler", sysPrompt+"\n\nUser Request: "+prompt)
		logToGCS("2_sonic_profiler", sonicResult)
	}()

	go func() {
		defer wg1.Done()
		sysPrompt, _ := LoadPrompt("3_community_scraper")
		scrapeResult, err3 = o.RunAgent(ctx, "Community Scraper", sysPrompt+"\n\nUser Request: "+prompt)
		logToGCS("3_community_scraper", scrapeResult)
	}()

	wg1.Wait()

	if err1 != nil || err2 != nil || err3 != nil {
		return "", o.Usage, fmt.Errorf("Phase 1 failures: Historian=%v, Profiler=%v, Scraper=%v", err1, err2, err3)
	}

	log.Printf("Phase 1 Complete.")

	// Download dictionary for Agent 4
	dictJSON := "{}"
	if gcs != nil {
		dictBytes, err := gcs.ReadFile(ctx, "weitzer-sound-builder", "coros_map.json")
		if err == nil {
			dictJSON = string(dictBytes)
		}
	}

	// Phase 2: Sourcing & Verification (Sequential)
	// Agent 4: CorOS Librarian
	sysPrompt4, _ := LoadPrompt("4_coros_librarian")
	context4 := fmt.Sprintf("%s\n\nTone History: %s\nScraper: %s\nDictionary: %s\nConstraints: %v", sysPrompt4, toneResult, scrapeResult, dictJSON, constraints)
	librarianResult, err4 := o.RunAgent(ctx, "CorOS Librarian", context4)
	if err4 != nil {
		return "", o.Usage, fmt.Errorf("Phase 2 Librarian failure: %v", err4)
	}
	logToGCS("4_coros_librarian", librarianResult)

	// Agent 5: Cloud Navigator
	allowCloudCaptures, ok := constraints["allow_cloud_captures"].(bool)
	var navigatorResult string
	var err5 error
	
	if ok && !allowCloudCaptures {
		// Bypass API processing to enforce strict configuration rules and prevent hallucinations
		navigatorResult = "User explicitly disabled Cloud Sourcing. Do not map any cloud captures."
		logToGCS("5_cloud_navigator", navigatorResult)
	} else {
		sysPrompt5, _ := LoadPrompt("5_cloud_navigator")
		context5 := fmt.Sprintf("%s\n\nLibrarian Output: %s", sysPrompt5, librarianResult)
		navigatorResult, err5 = o.RunAgent(ctx, "Cloud Navigator", context5)
		if err5 != nil {
			return "", o.Usage, fmt.Errorf("Phase 2 Navigator failure: %v", err5)
		}
		logToGCS("5_cloud_navigator", navigatorResult)
	}

	log.Printf("Phase 2 Complete.")

	// Phase 3: Physics & Calculation
	var wg3 sync.WaitGroup
	var acousticianResult, techResult, fohResult string
	var err6, err7, err8 error

	wg3.Add(3)
	go func() {
		defer wg3.Done()
		p, _ := LoadPrompt("6_acoustician")
		acousticianResult, err6 = o.RunAgent(ctx, "Acoustician", fmt.Sprintf("%s\n\nSonic Profiler: %s\nConstraints: %v", p, sonicResult, constraints))
		logToGCS("6_acoustician", acousticianResult)
	}()
	go func() {
		defer wg3.Done()
		p, _ := LoadPrompt("7_transducer_tech")
		techResult, err7 = o.RunAgent(ctx, "Transducer Tech", fmt.Sprintf("%s\n\nLibrarian Output: %s", p, librarianResult))
		logToGCS("7_transducer_tech", techResult)
	}()
	go func() {
		defer wg3.Done()
		p, _ := LoadPrompt("8_foh_optimizer")
		fohResult, err8 = o.RunAgent(ctx, "FOH Optimizer", fmt.Sprintf("%s\n\nConstraints: %v (Standard FRFR FOH context applied)", p, constraints))
		logToGCS("8_foh_optimizer", fohResult)
	}()
	wg3.Wait()

	if err6 != nil || err7 != nil || err8 != nil {
		return "", o.Usage, fmt.Errorf("Phase 3 failures: %v, %v, %v", err6, err7, err8)
	}
	log.Printf("Phase 3 Complete.")

	// Phase 4: Assembly & Output
	var wg4 sync.WaitGroup
	var mixResult, mapResult, dspResult string
	var err9, err10, err11 error

	wg4.Add(3)
	go func() {
		defer wg4.Done()
		p, _ := LoadPrompt("9_mix_engineer")
		mixResult, err9 = o.RunAgent(ctx, "Mix Engineer", fmt.Sprintf("%s\n\nTone Result: %s", p, toneResult))
		logToGCS("9_mix_engineer", mixResult)
	}()
	go func() {
		defer wg4.Done()
		p, _ := LoadPrompt("10_control_mapper")
		mapResult, err10 = o.RunAgent(ctx, "Control Mapper", fmt.Sprintf("%s\n\nPrompt: %s\nLibrarian Output: %s\nNavigator Output: %s\nAcoustician Output: %s", p, prompt, librarianResult, navigatorResult, acousticianResult))
		logToGCS("10_control_mapper", mapResult)
	}()
	go func() {
		defer wg4.Done()
		p, _ := LoadPrompt("11_dsp_dispatcher")
		dspResult, err11 = o.RunAgent(ctx, "DSP Dispatcher", fmt.Sprintf("%s\n\nLibrarian Output: %s\nNavigator Output: %s", p, librarianResult, navigatorResult))
		logToGCS("11_dsp_dispatcher", dspResult)
	}()
	wg4.Wait()

	if err9 != nil || err10 != nil || err11 != nil {
		return "", o.Usage, fmt.Errorf("Phase 4 failures: %v, %v, %v", err9, err10, err11)
	}
	log.Printf("Phase 4 Complete.")

	// Agent 12: Architect & Evaluator formats the final breakdown
	architectPrompt := "Evaluate the impact of the pipeline and format the final HTML table based strictly on the following aggregated data payload:\n\n"
	architectPrompt += fmt.Sprintf("Constraints: %v\n\nTone: %s\nSonic: %s\nScraper: %s\nLibrarian: %s\nNavigator: %s\nAcoustician: %s\nTransducer: %s\nFOH: %s\nMix: %s\nMap: %s\nDSP: %s",
		constraints, toneResult, sonicResult, scrapeResult, librarianResult, navigatorResult, acousticianResult, techResult, fohResult, mixResult, mapResult, dspResult)

	sysPrompt12, _ := LoadPrompt("12_architect")
	finalResult, err12 := o.RunAgent(ctx, "Architect & Evaluator", sysPrompt12+"\n\n"+architectPrompt)
	if err12 != nil {
		return "", o.Usage, fmt.Errorf("Architect failure: %v", err12)
	}
	logToGCS("12_architect", finalResult)

	return finalResult, o.Usage, nil
}

// RunAgent executes a prompt using Gemini 3.1 Pro Preview with fallback logic to Gemini 2.5 Pro
func (o *Orchestrator) RunAgent(ctx context.Context, agentRole string, prompt string) (string, error) {
	
	// 1. Attempt generation with Gemini 3.1 Pro Preview (Primary)
	// TODO: Evaluate if ALL 12 agents actually require gemini-3.1-pro-preview. Given its strict capacity limits, we should benchmark if less demanding agents (like Librarian or Formatter) can run efficiently on gemini-2.5-flash or gemini-2.5-pro to save global quota.
	modelName := "gemini-3.1-pro-preview"
	model := o.client.GenerativeModel(modelName)
	
	ctx1, cancel1 := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel1()

	resp, err := model.GenerateContent(ctx1, genai.Text(prompt))
	if err != nil {
		log.Printf("[%s] Gemini 3.1 Pro Preview failed or rate-limited: %v. Falling back to Gemini 2.5 Pro.", agentRole, err)
		
		// 2. Fallback to Gemini 2.5 Pro
		modelName = "gemini-2.5-pro"
		fallbackModel := o.client.GenerativeModel(modelName)
		
		ctx2, cancel2 := context.WithTimeout(ctx, 3*time.Minute)
		defer cancel2()
		
		resp, err = fallbackModel.GenerateContent(ctx2, genai.Text(prompt))
		if err != nil {
			return "", fmt.Errorf("[%s] Fallback model also failed: %v", agentRole, err)
		}
	}

	if len(resp.Candidates) == 0 {
		return "", fmt.Errorf("[%s] No response candidates from LLM", agentRole)
	}

	// Safely extract and accumulate generated API metrics natively 
	if resp.UsageMetadata != nil {
		log.Printf("AGENT_TOKEN_LOG: Agent=%s In=%d Out=%d", agentRole, resp.UsageMetadata.PromptTokenCount, resp.UsageMetadata.CandidatesTokenCount)
		o.Usage.mu.Lock()
		o.Usage.InputTokens += resp.UsageMetadata.PromptTokenCount
		o.Usage.OutputTokens += resp.UsageMetadata.CandidatesTokenCount
		o.Usage.ModelsUsed[modelName]++
		o.Usage.mu.Unlock()
	}

	return fmt.Sprintf("%v", resp.Candidates[0].Content.Parts[0]), nil
}

// Close explicitly frees the Gemini rest connection
func (o *Orchestrator) Close() {
	if o.client != nil {
		o.client.Close()
	}
}

// RefineChat bypasses the 12-agent pipeline and submits the user's feedback to the Architect utilizing conversational history
func (o *Orchestrator) RefineChat(ctx context.Context, p *storage.Preset, userMessage string) (string, *TokenUsage, error) {
	if mockVal, ok := ctx.Value(MockModeKey).(bool); ok && mockVal {
		if mockOutput, err := readMockFile("testdata/e2e_mocks/architect_refine.json"); err == nil {
			return mockOutput, o.Usage, nil
		}
	}
	log.Printf("Starting ADK Refinement Chat for feedback: %s\n", userMessage)

	sysPrompt, _ := LoadPrompt("12_architect")
	
	refinementPrompt := fmt.Sprintf(`
You are being called in REFINEMENT mode. The user is asking a question or requesting a change to an existing generated preset.
You MUST output the following exact JSON schema:

{
  "conversational_response": "Your conversational answer to the user. Describe what you changed, or answer their question.",
  "dsp_matrix_updated": true, /* Set to true ONLY if you made changes to the matrix, false otherwise */
  "final_html_payload": "YOUR_FULL_HTML_TABLE_HERE", /* The *entire* updated HTML table matrix (only if dsp_matrix_updated is true) */
  "agent_impact": ["Bullet point describing impact"]
}

EXISTING HTML PAYLOAD:
%s
`, p.Payload)

	historyText := "\n\nCHAT HISTORY (Most recent last):\n"
	for _, msg := range p.ChatHistory {
		if msg.Role == "user" {
			historyText += "USER: " + msg.Content + "\n"
		} else {
			historyText += "ARCHITECT: " + msg.Content + "\n"
		}
	}
	historyText += "USER: " + userMessage + "\n"

	finalResult, err := o.RunAgent(ctx, "Refinement Architect", sysPrompt+"\n\n"+refinementPrompt+historyText)
	if err != nil {
		return "", o.Usage, fmt.Errorf("Refinement failure: %v", err)
	}

	return finalResult, o.Usage, nil
}
