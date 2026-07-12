package agents

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/weitzer-org/sound-builder/internal/storage"
	"google.golang.org/genai"
)

// ContextKey establishes standard context keys for the agents package
type ContextKey string

const (
	MockModeKey ContextKey = "mock_mode"
)

//go:embed coros_map.json
var embeddedCorosMap []byte

//go:embed user_captures.json
var embeddedUserCaptures []byte

// AgentUsage captures per-agent token, model, call-count, and latency data for one
// pipeline run so before/after quality-vs-cost comparisons don't have to be reconstructed
// from log lines.
type AgentUsage struct {
	InputTokens  int32
	OutputTokens int32
	Model        string
	Calls        int
	LatencyMs    int64
}

// TokenUsage rigidly aggregates exact API volume metadata
type TokenUsage struct {
	InputTokens    int32
	OutputTokens   int32
	ModelsUsed     map[string]int
	PerAgent       map[string]*AgentUsage
	TotalLatencyMs int64
	mu             sync.Mutex
}

// recordAgentUsage accumulates one agent call's token/latency data under agentRole.
func (u *TokenUsage) recordAgentUsage(agentRole, model string, inputTokens, outputTokens int32, latencyMs int64) {
	u.mu.Lock()
	defer u.mu.Unlock()
	if u.PerAgent == nil {
		u.PerAgent = make(map[string]*AgentUsage)
	}
	a, ok := u.PerAgent[agentRole]
	if !ok {
		a = &AgentUsage{}
		u.PerAgent[agentRole] = a
	}
	a.InputTokens += inputTokens
	a.OutputTokens += outputTokens
	a.Model = model
	a.Calls++
	a.LatencyMs += latencyMs
}

// OrchestratorService defines the methods available on the ADK Orchestrator
type OrchestratorService interface {
	RunPipeline(ctx context.Context, prompt string, constraints map[string]interface{}, agentConfig map[string]string, onProgress func(phase string)) (string, *TokenUsage, error)
	RefineChat(ctx context.Context, p *storage.Preset, userMessage string) (string, *TokenUsage, error)
	Close()
}

// Orchestrator manages the 12-agent pipeline through 4 execution phases
type Orchestrator struct {
	client         *genai.Client
	Usage          *TokenUsage
	gcs            storage.Client
	AgentModels    map[string]string // Added for per-agent model configuration
	useOpenLLM     bool
	openLLMClient  *OpenLLMClient
	openLLMTimeout time.Duration
}

// NewOrchestrator initializes the Gemini ADK client or the Open-LLM REST client
func NewOrchestrator(ctx context.Context, apiKey string, gcs storage.Client, opts ...ClientOption) (*Orchestrator, error) {
	isMock := os.Getenv("MOCK_MODE") == "true"
	if mockVal, ok := ctx.Value(MockModeKey).(bool); ok && mockVal {
		isMock = true
	}
	if isMock {
		return &Orchestrator{client: nil, Usage: &TokenUsage{ModelsUsed: make(map[string]int)}, gcs: gcs}, nil
	}

	useOpenLLM := os.Getenv("USE_OPENLLM") == "true"
	
	// Build Open-LLM client (always available for hybrid "openllm:" prefixed routing)
	url := os.Getenv("OPENLLM_API_URL")
	if url == "" {
		url = "https://open-llm-gateway-710019748844.us-central1.run.app/v1"
	}
	key := ""
	if useOpenLLM {
		key = apiKey
	}
	if key == "" {
		key = os.Getenv("OPENLLM_API_KEY")
	}
	model := os.Getenv("OPENLLM_MODEL")
	if model == "" {
		model = "active-model"
	}
	timeout := 10 * time.Minute
	if tStr := os.Getenv("OPENLLM_TIMEOUT"); tStr != "" {
		if parsedT, err := time.ParseDuration(tStr); err == nil {
			timeout = parsedT
		} else {
			log.Printf("Warning: Invalid OPENLLM_TIMEOUT '%s', defaulting to 10 minutes", tStr)
		}
	}
	openLLMTimeout := timeout
	openLLMClient := NewOpenLLMClient(url, key, model, timeout)

	var client *genai.Client
	var err error
	// Instantiate Gemini client if not exclusively using Open-LLM or if a key is available for dynamic side-by-side execution
	if !useOpenLLM || apiKey != "" || os.Getenv("GEMINI_API_KEY") != "" {
		geminiKey := apiKey
		if geminiKey == "" {
			geminiKey = os.Getenv("GEMINI_API_KEY")
		}
		// Only build if we have a real non-empty credential key (excluding fallback flags)
		if geminiKey != "" && geminiKey != "mock-api-key" {
			cc := &genai.ClientConfig{
				APIKey:  geminiKey,
				Backend: genai.BackendGeminiAPI, // plain Gemini API key auth; never Vertex AI, no GCP project required
			}
			for _, opt := range opts {
				opt(cc)
			}
			client, err = genai.NewClient(ctx, cc)
			if err != nil {
				// Don't crash if open-llm is active but just warn, since we might only use open-llm!
				if useOpenLLM {
					log.Printf("Warning: Failed to initialize optional Gemini client: %v", err)
				} else {
					return nil, err
				}
			}
		}
	}

	// Initialize concurrency-safe Token Tracker
	usage := &TokenUsage{
		ModelsUsed: make(map[string]int),
	}

	return &Orchestrator{
		client:         client,
		Usage:          usage,
		gcs:            gcs,
		useOpenLLM:     useOpenLLM,
		openLLMClient:  openLLMClient,
		openLLMTimeout: openLLMTimeout,
	}, nil
}

// RunPipeline takes the user's prompt and routes it through the 12 agents
func (o *Orchestrator) RunPipeline(ctx context.Context, prompt string, constraints map[string]interface{}, agentConfig map[string]string, onProgress func(phase string)) (string, *TokenUsage, error) {
	pipelineStart := time.Now()
	defer func() {
		o.Usage.mu.Lock()
		o.Usage.TotalLatencyMs = time.Since(pipelineStart).Milliseconds()
		o.Usage.mu.Unlock()
	}()

	isMock := os.Getenv("MOCK_MODE") == "true"
	if mockVal, ok := ctx.Value(MockModeKey).(bool); ok && mockVal {
		isMock = true
	}

	if isMock {
		if mockOutput, err := readMockFile("testdata/e2e_mocks/architect_generate.json"); err == nil {
			return mockOutput, o.Usage, nil
		} else {
			log.Printf("Warning: Failed to read mock file testdata/e2e_mocks/architect_generate.json: %v", err)
		}
	}

	// Apply global application architecture limits dynamically into the root user prompt stream
	singleAmpMode, ok := constraints["single_amp_mode"].(bool)
	if ok && singleAmpMode {
		prompt = "[GLOBAL USER CONFIG FLAG: SINGLE AMP ONLY. The generated preset MUST route through one single amplifier block.]\n\n" + prompt
	}

	allowPaidPlugins, ok := constraints["allow_paid_plugins"].(bool)
	if ok && !allowPaidPlugins {
		prompt = "[GLOBAL USER CONFIG FLAG: NO PAID PLUGINS. The generated preset MUST NOT use any paid plugins.]\n\n" + prompt
	} else if ok && allowPaidPlugins {
		availablePlugins, ok := constraints["available_plugins"].([]string)
		if ok && len(availablePlugins) > 0 {
			prompt = fmt.Sprintf("[GLOBAL USER CONFIG FLAG: AVAILABLE PLUGINS: %s. The generated preset may use these plugins if appropriate.]\n\n", strings.Join(availablePlugins, ", ")) + prompt
		}
	}

	effectiveAllowFactoryCaptures := true
	if v, ok := constraints["allow_factory_captures"].(bool); ok {
		effectiveAllowFactoryCaptures = v
	}
	if !effectiveAllowFactoryCaptures {
		prompt = "[GLOBAL USER CONFIG FLAG: NO FACTORY CAPTURES. The generated preset MUST NOT use any factory captures.]\n\n" + prompt
	}

	effectiveAllowUserCaptures := true
	if v, ok := constraints["allow_user_captures"].(bool); ok {
		effectiveAllowUserCaptures = v
	}
	if !effectiveAllowUserCaptures {
		prompt = "[GLOBAL USER CONFIG FLAG: NO USER CAPTURES. The generated preset MUST NOT use any entries from the User Capture Library.]\n\n" + prompt
	}

	if v, ok := constraints["favor_captures"].(bool); ok && v {
		prompt = "[GLOBAL USER CONFIG FLAG: FAVOR CAPTURES. The generated preset should actively prioritize using official Factory Neural Captures (tagged with is_capture: true) over built-in algorithmic component models when possible.]\n\n" + prompt
	}

	if v, ok := constraints["favor_cloud_captures"].(bool); ok && v {
		prompt = "[GLOBAL USER CONFIG FLAG: FAVOR CLOUD CAPTURES. The generated preset should actively prioritize using entries from the User Capture Library (the user's own downloaded Cortex Cloud captures) over built-in algorithmic component models and factory captures when possible.]\n\n" + prompt
	}

	log.Printf("Starting ADK Pipeline for prompt: %s\n", prompt)

	execID := fmt.Sprintf("%d", time.Now().UnixNano())
	gcs := o.gcs

	logToGCS := func(agentNum string, result string) {
		if os.Getenv("LOCAL_AGENT_LOGGING") == "true" {
			dirName := execID
			if customDir := os.Getenv("LOCAL_AGENT_LOGGING_DIR"); customDir != "" {
				dirName = customDir
			}
			os.MkdirAll("eval_results/diagnostics/"+dirName, 0755)
			filePath := "eval_results/diagnostics/" + dirName + "/" + agentNum + ".json"
			err := os.WriteFile(filePath, []byte(result), 0644)
			if err != nil {
				log.Printf("Failed to log %s to local file: %v", agentNum, err)
			}
		}
		if gcs != nil {
			go func(aNum, res string) {
				// Detach context to ensure logging finishes even if main pipeline is cancelled
				bgCtx := context.WithoutCancel(ctx)
				bucket := os.Getenv("GCS_BUCKET_NAME")
				if bucket == "" {
					bucket = os.Getenv("S3_BUCKET") // track the active S3/R2 bucket when set
				}
				if bucket == "" {
					bucket = "weitzer-sound-builder" // Default fallback
				}
				err := gcs.WriteFile(bgCtx, bucket, fmt.Sprintf("logs/%s/%s.json", execID, aNum), []byte(res))
				if err != nil {
					log.Printf("Failed to log %s to GCS: %v", aNum, err)
				}
			}(agentNum, result)
		}
	}

	if onProgress != nil {
		onProgress("Phase 1/4: Analyzing Tone Context & Physics...")
	}

	// Phase 1: Research & Reality (Concurrent Goroutines)
	var wg1 sync.WaitGroup
	var toneResult, sonicResult, scrapeResult string
	var err1, err2, err3 error

	wg1.Add(3)

	go func() {
		defer wg1.Done()
		version := agentConfig["1_tone_historian"]
		if version == "off" {
			toneResult = "Tone Historian skipped by configuration."
			return
		}
		sysPrompt, _ := o.loadPromptForAgent("1_tone_historian", version)
		toneResult, err1 = o.RunAgentSplit(ctx, "Tone Historian", sysPrompt, "User Request: "+prompt)
		logToGCS("1_tone_historian", toneResult)
	}()

	go func() {
		defer wg1.Done()
		version := agentConfig["2_sonic_profiler"]
		if version == "off" {
			sonicResult = "Sonic Profiler skipped by configuration."
			return
		}
		sysPrompt, _ := o.loadPromptForAgent("2_sonic_profiler", version)
		sonicResult, err2 = o.RunAgentSplit(ctx, "Sonic Profiler", sysPrompt, "User Request: "+prompt)
		logToGCS("2_sonic_profiler", sonicResult)
	}()

	go func() {
		defer wg1.Done()
		version := agentConfig["3_community_scraper"]
		if version == "off" {
			scrapeResult = "Community Scraper skipped by configuration."
			return
		}
		sysPrompt, _ := o.loadPromptForAgent("3_community_scraper", version)
		scrapeResult, err3 = o.RunAgentSplit(ctx, "Community Scraper", sysPrompt, "User Request: "+prompt)
		logToGCS("3_community_scraper", scrapeResult)
	}()

	wg1.Wait()

	if err1 != nil || err2 != nil || err3 != nil {
		return "", o.Usage, fmt.Errorf("Phase 1 failures: Historian=%v, Profiler=%v, Scraper=%v", err1, err2, err3)
	}

	log.Printf("Phase 1 Complete.")

	// Read embedded dictionary for Agent 4
	dictJSON := string(embeddedCorosMap)
	if !effectiveAllowFactoryCaptures {
		var fullMap map[string]map[string]interface{}
		if err := json.Unmarshal(embeddedCorosMap, &fullMap); err == nil {
			filteredMap := make(map[string]map[string]interface{})
			for k, v := range fullMap {
				if isCap, _ := v["is_capture"].(bool); !isCap {
					filteredMap[k] = v
				}
			}
			if b, err := json.Marshal(filteredMap); err == nil {
				dictJSON = string(b)
			}
		}
	}
	ampMenu := GetCategorizedAmplifiers(effectiveAllowFactoryCaptures)

	userCapturesJSON := "[]"
	if effectiveAllowUserCaptures {
		userCapturesJSON = GetUserCapturesJSON()
	}

	if onProgress != nil {
		onProgress("Phase 2/4: Mapping to Native CorOS Effects...")
	}

	// Phase 2: Sourcing & Verification (Sequential)
	// Agent 4: CorOS Librarian
	var librarianResult string
	var err4 error
	version4 := agentConfig["4_coros_librarian"]
	if version4 == "off" {
		librarianResult = "CorOS Librarian skipped by configuration."
	} else {
		sysPrompt4, _ := o.loadPromptForAgent("4_coros_librarian", version4)
		userContext4 := fmt.Sprintf("Tone History: %s\nScraper: %s\nDictionary: %s\nAmplifier Archetype Menu:\n%s\nUser Capture Library: %s\nConstraints: %v", toneResult, scrapeResult, dictJSON, ampMenu, userCapturesJSON, constraints)
		librarianResult, err4 = o.RunAgentSplit(ctx, "CorOS Librarian", sysPrompt4, userContext4)
		if err4 != nil {
			return "", o.Usage, fmt.Errorf("Phase 2 Librarian failure: %v", err4)
		}
		logToGCS("4_coros_librarian", librarianResult)
	}

	// Agent 5: Cloud Navigator
	allowCloudCaptures, ok := constraints["allow_cloud_captures"].(bool)
	var navigatorResult string
	var err5 error
	
	version5 := agentConfig["5_cloud_navigator"]
	if version5 == "off" {
		navigatorResult = "Cloud Navigator skipped by configuration."
	} else if ok && !allowCloudCaptures {
		// Bypass API processing to enforce strict configuration rules and prevent hallucinations
		navigatorResult = "User explicitly disabled Cloud Sourcing. Do not map any cloud captures."
		logToGCS("5_cloud_navigator", navigatorResult)
	} else {
		sysPrompt5, _ := o.loadPromptForAgent("5_cloud_navigator", version5)
		userContext5 := fmt.Sprintf("Librarian Output: %s\nUser Capture Library: %s", librarianResult, userCapturesJSON)
		navigatorResult, err5 = o.RunAgentSplit(ctx, "Cloud Navigator", sysPrompt5, userContext5)
		if err5 != nil {
			return "", o.Usage, fmt.Errorf("Phase 2 Navigator failure: %v", err5)
		}
		logToGCS("5_cloud_navigator", navigatorResult)
	}

	log.Printf("Phase 2 Complete.")

	if onProgress != nil {
		onProgress("Phase 3/4: Calculating FOH & Room EQ Balances...")
	}

	// Phase 3: Physics & Calculation
	var wg3 sync.WaitGroup
	var acousticianResult, techResult, fohResult string
	var err6, err7, err8 error

	wg3.Add(3)
	go func() {
		defer wg3.Done()
		version := agentConfig["6_acoustician"]
		if version == "off" {
			acousticianResult = "Acoustician skipped by configuration."
			return
		}
		p, _ := o.loadPromptForAgent("6_acoustician", version)
		acousticianResult, err6 = o.RunAgentSplit(ctx, "Acoustician", p, fmt.Sprintf("Sonic Profiler: %s\nConstraints: %v", sonicResult, constraints))
		logToGCS("6_acoustician", acousticianResult)
	}()
	go func() {
		defer wg3.Done()
		version := agentConfig["7_transducer_tech"]
		if version == "off" {
			techResult = "Transducer Tech skipped by configuration."
			return
		}
		p, _ := o.loadPromptForAgent("7_transducer_tech", version)
		techResult, err7 = o.RunAgentSplit(ctx, "Transducer Tech", p, fmt.Sprintf("Librarian Output: %s", librarianResult))
		logToGCS("7_transducer_tech", techResult)
	}()
	go func() {
		defer wg3.Done()
		version := agentConfig["8_foh_optimizer"]
		if version == "off" {
			fohResult = "FOH Optimizer skipped by configuration."
			return
		}
		p, _ := o.loadPromptForAgent("8_foh_optimizer", version)
		fohResult, err8 = o.RunAgentSplit(ctx, "FOH Optimizer", p, fmt.Sprintf("Constraints: %v (Standard FRFR FOH context applied)", constraints))
		logToGCS("8_foh_optimizer", fohResult)
	}()
	wg3.Wait()

	if err6 != nil || err7 != nil || err8 != nil {
		return "", o.Usage, fmt.Errorf("Phase 3 failures: %v, %v, %v", err6, err7, err8)
	}
	log.Printf("Phase 3 Complete.")

	if onProgress != nil {
		onProgress("Phase 4/4: Constructing Final DSP Routing Matrix...")
	}

	// Phase 4: Assembly & Output
	var wg4 sync.WaitGroup
	var mixResult, mapResult, dspResult string
	var err9, err10, err11 error

	wg4.Add(3)
	go func() {
		defer wg4.Done()
		version := agentConfig["9_mix_engineer"]
		if version == "off" {
			mixResult = "Mix Engineer skipped by configuration."
			return
		}
		p, _ := o.loadPromptForAgent("9_mix_engineer", version)
		mixResult, err9 = o.RunAgentSplit(ctx, "Mix Engineer", p, fmt.Sprintf("Tone Result: %s", toneResult))
		logToGCS("9_mix_engineer", mixResult)
	}()
	go func() {
		defer wg4.Done()
		version := agentConfig["10_control_mapper"]
		if version == "off" {
			mapResult = "Control Mapper skipped by configuration."
			return
		}
		p, _ := o.loadPromptForAgent("10_control_mapper", version)
		mapResult, err10 = o.RunAgentSplit(ctx, "Control Mapper", p, fmt.Sprintf("Prompt: %s\nLibrarian Output: %s\nNavigator Output: %s\nAcoustician Output: %s", prompt, librarianResult, navigatorResult, acousticianResult))
		logToGCS("10_control_mapper", mapResult)
	}()
	go func() {
		defer wg4.Done()
		version := agentConfig["11_dsp_dispatcher"]
		if version == "off" {
			dspResult = "DSP Dispatcher skipped by configuration."
			return
		}
		p, _ := o.loadPromptForAgent("11_dsp_dispatcher", version)
		dspResult, err11 = o.RunAgentSplit(ctx, "DSP Dispatcher", p, fmt.Sprintf("Librarian Output: %s\nNavigator Output: %s", librarianResult, navigatorResult))
		logToGCS("11_dsp_dispatcher", dspResult)
	}()
	wg4.Wait()

	if err9 != nil || err10 != nil || err11 != nil {
		return "", o.Usage, fmt.Errorf("Phase 4 failures: %v, %v, %v", err9, err10, err11)
	}
	log.Printf("Phase 4 Complete.")

	if onProgress != nil {
		onProgress("Finalizing: Rendering Interactive Workspace UI...")
	}

	// Agent 12: Architect & Evaluator formats the final breakdown
	architectPrompt := "Evaluate the impact of the pipeline and format the final HTML table based strictly on the following aggregated data payload:\n\n"
	architectPrompt += fmt.Sprintf("Constraints: %v\n\nTone: %s\nSonic: %s\nScraper: %s\nLibrarian: %s\nNavigator: %s\nAcoustician: %s\nTransducer: %s\nFOH: %s\nMix: %s\nMap: %s\nDSP: %s",
		constraints, toneResult, sonicResult, scrapeResult, librarianResult, navigatorResult, acousticianResult, techResult, fohResult, mixResult, mapResult, dspResult)

	var finalResult string
	var err12 error
	version12 := agentConfig["12_architect"]
	if version12 == "off" {
		finalResult = "Architect skipped by configuration."
	} else {
		sysPrompt12, _ := o.loadPromptForAgent("12_architect", version12)
		finalResult, err12 = o.RunAgentSplit(ctx, "Architect & Evaluator", sysPrompt12, architectPrompt)
		if err12 != nil {
			return "", o.Usage, fmt.Errorf("Architect failure: %v", err12)
		}
		logToGCS("12_architect", finalResult)

		if rendered, err := injectRenderedHTML(finalResult); err == nil {
			finalResult = rendered
		} else {
			return "", o.Usage, fmt.Errorf("Architect returned unparseable JSON: %v", err)
		}
	}

	if len(GetValidNativeBlocks()) > 0 {
		effectiveBlocks := BuildEffectiveValidBlocks(effectiveAllowFactoryCaptures, effectiveAllowUserCaptures)
		finalResult = ApplyFuzzyCorrection(finalResult, effectiveBlocks)
	}

	return finalResult, o.Usage, nil
}

// getFallbackChain generates a progressive chronological backup pathway list for standard model components
func getFallbackChain(primaryModel string) []string {
	switch primaryModel {
	case "gemini-3.1-pro-preview":
		return []string{"gemini-3.5-flash", "gemini-3-flash-preview", "gemini-2.5-pro"}
	case "gemini-3.5-flash":
		return []string{"gemini-3-flash-preview", "gemini-2.5-pro"}
	case "gemini-3-flash-preview":
		return []string{"gemini-2.5-pro"}
	case "gemini-2.5-pro":
		return nil
	default:
		return []string{"gemini-2.5-pro"}
	}
}

// RunAgent redirects to RunAgentSplit with empty system instructions
func (o *Orchestrator) RunAgent(ctx context.Context, agentRole string, prompt string) (string, error) {
	return o.RunAgentSplit(ctx, agentRole, "", prompt)
}

// RunAgentSplit executes a prompt with separate system and user prompts using the configured backend (Gemini or Open-LLM)
func (o *Orchestrator) RunAgentSplit(ctx context.Context, agentRole string, systemPrompt, userPrompt string) (string, error) {
	skip := false
	switch agentRole {
	case "Tone Historian": skip = os.Getenv("ABLATE_AGENT_1") == "true"
	case "Sonic Profiler": skip = os.Getenv("ABLATE_AGENT_2") == "true"
	case "Community Scraper": skip = os.Getenv("ABLATE_AGENT_3") == "true"
	case "CorOS Librarian": skip = os.Getenv("ABLATE_AGENT_4") == "true"
	case "Cloud Navigator": skip = os.Getenv("ABLATE_AGENT_5") == "true"
	case "Acoustician": skip = os.Getenv("ABLATE_AGENT_6") == "true"
	case "Transducer Tech": skip = os.Getenv("ABLATE_AGENT_7") == "true"
	case "FOH Optimizer": skip = os.Getenv("ABLATE_AGENT_8") == "true"
	case "Mix Engineer": skip = os.Getenv("ABLATE_AGENT_9") == "true"
	case "Control Mapper": skip = os.Getenv("ABLATE_AGENT_10") == "true"
	case "DSP Dispatcher": skip = os.Getenv("ABLATE_AGENT_11") == "true"
	case "Architect & Evaluator", "Refinement Architect": skip = os.Getenv("ABLATE_AGENT_12") == "true"
	}

	if skip {
		log.Printf("[%s] ABLATION TRIGGERED. Skipping LLM call.", agentRole)
		return fmt.Sprintf("Ablated Output for %s.", agentRole), nil
	}

	key := ""
	switch agentRole {
	case "Tone Historian": key = "1_tone_historian"
	case "Sonic Profiler": key = "2_sonic_profiler"
	case "Community Scraper": key = "3_community_scraper"
	case "CorOS Librarian": key = "4_coros_librarian"
	case "Cloud Navigator": key = "5_cloud_navigator"
	case "Acoustician": key = "6_acoustician"
	case "Transducer Tech": key = "7_transducer_tech"
	case "FOH Optimizer": key = "8_foh_optimizer"
	case "Mix Engineer": key = "9_mix_engineer"
	case "Control Mapper": key = "10_control_mapper"
	case "DSP Dispatcher": key = "11_dsp_dispatcher"
	case "Architect & Evaluator", "Refinement Architect": key = "12_architect"
	}

	modelName := os.Getenv("TARGET_MODEL")
	if modelName == "" {
		if o.useOpenLLM {
			modelName = o.openLLMClient.Model
		} else {
			// Apply production routing defaults (Agent 1 & 12 use 3.1 Pro, others use 3.5 Flash)
			if key == "1_tone_historian" || key == "12_architect" {
				modelName = "gemini-3.1-pro-preview"
			} else {
				modelName = "gemini-3.5-flash"
			}
		}
	}

	if key != "" && o.AgentModels != nil {
		if specificModel, ok := o.AgentModels[key]; ok {
			modelName = specificModel
		}
	}

	// Determine if this specific request targets the Open-LLM backend
	isPrefixOpenLLM := strings.HasPrefix(strings.ToLower(modelName), "openllm:")
	isOpenLLMRoute := o.useOpenLLM || isPrefixOpenLLM

	actualModelName := modelName
	if isPrefixOpenLLM {
		actualModelName = modelName[8:] // Strip "openllm:"
		if actualModelName == "" {
			actualModelName = o.openLLMClient.Model
		}
	}

	// 1. Open-LLM Branch
	if isOpenLLMRoute {
		if o.openLLMClient == nil {
			return "", fmt.Errorf("[%s] Open-LLM client not initialized", agentRole)
		}
		timeout := o.openLLMTimeout
		if timeout == 0 {
			timeout = 10 * time.Minute
		}
		ctx1, cancel1 := context.WithTimeout(ctx, timeout)
		defer cancel1()

		log.Printf("[%s] Dispatching Open-LLM request (Model: %s, Timeout: %s)...", agentRole, actualModelName, timeout)
		content, promptTokens, completionTokens, err := o.openLLMClient.Generate(ctx1, systemPrompt, userPrompt, actualModelName)
		if err != nil {
			// Check if this error is due to reaching the context window limits
			errStr := err.Error()
			lower := strings.ToLower(errStr)
			isContextLimit := strings.Contains(lower, "context length") || 
							   strings.Contains(lower, "context_length") || 
							   strings.Contains(lower, "maximum context") || 
							   strings.Contains(lower, "context window") ||
							   (strings.Contains(lower, "context") && strings.Contains(lower, "limit"))

			if isContextLimit {
				log.Printf("⚠️ WARNING: [%s] Open-LLM hit context window max: %v. Falling back to default production model: gemini-3.1-pro-preview", agentRole, err)
				if o.client == nil {
					return "", fmt.Errorf("[%s] Open-LLM hit context limit, but Gemini fallback client is not initialized (missing API key)", agentRole)
				}
				modelName = "gemini-3.1-pro-preview"
				// Fall through to standard Gemini execution path below!
			} else {
				return "", fmt.Errorf("[%s] Open-LLM failure: %w", agentRole, err)
			}
		} else {
			// Accumulate metrics natively
			o.Usage.mu.Lock()
			o.Usage.InputTokens += promptTokens
			o.Usage.OutputTokens += completionTokens
			o.Usage.ModelsUsed[modelName]++ // Keep full modelName in stats to identify openllm custom calls
			o.Usage.mu.Unlock()

			log.Printf("AGENT_TOKEN_LOG (OpenLLM): Agent=%s In=%d Out=%d", agentRole, promptTokens, completionTokens)
			return content, nil
		}
	}

	// 2. Standard Gemini Branch.
	// systemPrompt and userPrompt are now sent as a real system instruction + user turn
	// (via GenerateContentConfig.SystemInstruction) instead of being concatenated into one
	// user-role text blob, which is what the old SDK's single genai.Text(prompt) call forced.
	genConfig := o.buildGenerationConfig(agentRole, key)
	if systemPrompt != "" {
		genConfig.SystemInstruction = &genai.Content{Parts: []*genai.Part{genai.NewPartFromText(systemPrompt)}}
	}
	content := userPrompt
	if content == "" {
		// Degenerate case: only systemPrompt was provided. Don't echo it back as the user
		// turn too -- it's already set as SystemInstruction above, and for a prompt the
		// size of 12_architect_v3.md that would double input tokens for no reason.
		content = "Please generate the response based on the system instructions."
	}
	contents := []*genai.Content{genai.NewContentFromText(content, genai.RoleUser)}

	// Build the complete fallback sequence list starting with primary model target
	candidates := append([]string{modelName}, getFallbackChain(modelName)...)

	var lastErr error
	var resp *genai.GenerateContentResponse
	var finalModelName string
	var err error

	callStart := time.Now()
	for _, mName := range candidates {
		if o.client == nil {
			return "", fmt.Errorf("[%s] Gemini client is nil, cannot invoke model %s", agentRole, mName)
		}

		ctx1, cancel1 := context.WithTimeout(ctx, 3*time.Minute)
		resp, err = o.client.Models.GenerateContent(ctx1, mName, contents, genConfig)
		cancel1()

		if err == nil {
			finalModelName = mName
			break
		}

		log.Printf("[%s] Model %s failed or rate-limited: %v. Retrying next fallback...", agentRole, mName, err)
		lastErr = err
	}
	latencyMs := time.Since(callStart).Milliseconds()

	if finalModelName == "" {
		return "", fmt.Errorf("[%s] All fallback models failed: %w", agentRole, lastErr)
	}

	if len(resp.Candidates) == 0 {
		return "", fmt.Errorf("[%s] No response candidates from LLM", agentRole)
	}

	// Safely extract and accumulate generated API metrics natively
	if resp.UsageMetadata != nil {
		log.Printf("AGENT_TOKEN_LOG: Agent=%s In=%d Out=%d Model=%s LatencyMs=%d", agentRole, resp.UsageMetadata.PromptTokenCount, resp.UsageMetadata.CandidatesTokenCount, finalModelName, latencyMs)
		o.Usage.mu.Lock()
		o.Usage.InputTokens += resp.UsageMetadata.PromptTokenCount
		o.Usage.OutputTokens += resp.UsageMetadata.CandidatesTokenCount
		o.Usage.ModelsUsed[finalModelName]++
		o.Usage.mu.Unlock()
		o.Usage.recordAgentUsage(agentRole, finalModelName, resp.UsageMetadata.PromptTokenCount, resp.UsageMetadata.CandidatesTokenCount, latencyMs)
	}

	return resp.Text(), nil
}

// buildGenerationConfig assembles the per-agent GenerationConfig: temperature, search
// grounding (agents 1 & 3 only), and structured JSON output. Agent 12 (Architect) uses the
// raw-JSON-Schema path (ResponseJsonSchema) because its payload has guitar-name-keyed
// dynamic objects the typed OpenAPI-subset Schema can't express; every other agent uses the
// typed ResponseSchema.
func (o *Orchestrator) buildGenerationConfig(agentRole, key string) *genai.GenerateContentConfig {
	cfg := &genai.GenerateContentConfig{
		Temperature: agentTemperature(key),
		Tools:       agentSearchTool(key),
	}
	if key == "12_architect" {
		cfg.ResponseMIMEType = "application/json"
		if agentRole == "Refinement Architect" {
			cfg.ResponseJsonSchema = architectRefineJSONSchema()
		} else {
			cfg.ResponseJsonSchema = architectGenerationJSONSchema()
		}
		return cfg
	}
	if schema := agentResponseSchema(key); schema != nil {
		cfg.ResponseMIMEType = "application/json"
		cfg.ResponseSchema = schema
	}
	return cfg
}

// loadPromptForAgent loads the prompt for a given agent, falling back to flash version if applicable and available.
func (o *Orchestrator) loadPromptForAgent(key string, version string) (string, error) {
	model := "gemini-3.1-pro-preview" // default
	if o.AgentModels != nil {
		if m, ok := o.AgentModels[key]; ok {
			model = m
		}
	}

	if strings.Contains(model, "flash") {
		promptName := key + "_flash"
		prompt, err := LoadPrompt(promptName, version)
		if err == nil {
			return prompt, nil
		}
		// Fallback to standard prompt if flash prompt not found
	}

	return LoadPrompt(key, version)
}

// Close explicitly frees the Gemini rest connection
func (o *Orchestrator) Close() {
	// google.golang.org/genai's Client is a plain REST client with no persistent
	// connection/stream and no Close method (unlike the old SDK's gRPC-backed client).
}

// RefineChat bypasses the 12-agent pipeline and submits the user's feedback to the Architect utilizing conversational history
func (o *Orchestrator) RefineChat(ctx context.Context, p *storage.Preset, userMessage string) (string, *TokenUsage, error) {
	if mockVal, ok := ctx.Value(MockModeKey).(bool); ok && mockVal {
		if mockOutput, err := readMockFile("testdata/e2e_mocks/architect_refine.json"); err == nil {
			return mockOutput, o.Usage, nil
		}
	}
	log.Printf("Starting ADK Refinement Chat for feedback: %s\n", userMessage)

	sysPrompt, _ := o.loadPromptForAgent("12_architect", "v3")

	refinementPrompt := fmt.Sprintf(`
You are being called in REFINEMENT mode. The user is asking a question or requesting a change to an existing generated preset.

CRITICAL REFINE INSTRUCTION: If your refinement includes swapping a piece of gear (e.g. changing the Amplifier, Cabinet, or an Effect), you MUST meticulously update the actual "model" field on that block in structured_payload, and its "rationale" field. Do NOT just update the rationale text and leave the old model name in place!

You do NOT produce an HTML table. final_html_payload is rendered automatically from your structured_payload after you return it.

You MUST output the following exact JSON schema:

{
  "conversational_response": "Your conversational answer to the user. Describe what you changed, or answer their question.",
  "builder_statement": "Provide a short and concise statement on what you did during this refinement. Focus on the core tone and gear choices. Do NOT explain the differences between the guitars. IMPORTANT: If you do NOT make changes to the matrix (i.e. you are just answering a question), leave this field completely empty.",
  "dsp_matrix_updated": true, /* Set to true ONLY if you made changes to the matrix, false otherwise */
  "structured_payload": {
    "guitars": {
      "Fender Telecaster Single Coil": [
        { "id": "block-1", "type": "Amplifier", "model": "...", "rationale": "Why this block/model was chosen.", "parameters": [ {"name": "Gain", "type": "slider", "value": "7.0", "value_b": "8.5"} ] }
      ]
    }
  }, /* The *entire* updated structured JSON data for ALL guitars explicitly (only if dsp_matrix_updated is true). This is CRITICAL for persistence! Every block needs "rationale"; only add "value_b" on a parameter when Scene B (Lead) genuinely differs from Scene A (Rhythm) -- omit it otherwise. */
  "agent_impact": ["Bullet point describing impact"]
}

EXISTING STRUCTURED PAYLOAD:
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

	// TODO: Test which agents should additionally be invoked for the refinement flow for optimal results.
	// Currently we rely entirely on the standalone Refinement Architect editor agent, but pulling in context from the Mix Engineer or Librarian may yield higher fidelity modifications.
	finalResult, err := o.RunAgentSplit(ctx, "Refinement Architect", sysPrompt, refinementPrompt+historyText)
	if err != nil {
		return "", o.Usage, fmt.Errorf("Refinement failure: %v", err)
	}

	if rendered, err := injectRenderedHTML(finalResult); err == nil {
		finalResult = rendered
	} else {
		return "", o.Usage, fmt.Errorf("Refinement Architect returned unparseable JSON: %v", err)
	}

	validBlocks := GetValidNativeBlocks()
	if len(validBlocks) > 0 {
		finalResult = ApplyFuzzyCorrection(finalResult, validBlocks)
	}

	return finalResult, o.Usage, nil
}
