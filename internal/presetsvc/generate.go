package presetsvc

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/weitzer-org/sound-builder/internal/agents"
	"github.com/weitzer-org/sound-builder/internal/config"
	"github.com/weitzer-org/sound-builder/internal/runs"
	"github.com/weitzer-org/sound-builder/internal/storage"
)

// impactAgentNames maps prompt keys to the agent names that appear in the
// Architect's agent_impact bullets, so each bullet can be tagged with the
// prompt version that produced it. Deliberately the original 11 generation
// agents only (not agents.AgentRoleKeys): adding the Architect/Critic here would
// change the saved chat-history HTML.
var impactAgentNames = map[string]string{
	"1_tone_historian":    "Tone Historian",
	"2_sonic_profiler":    "Sonic Profiler",
	"3_community_scraper": "Community Scraper",
	"4_coros_librarian":   "CorOS Librarian",
	"5_cloud_navigator":   "Cloud Navigator",
	"6_acoustician":       "Acoustician",
	"7_transducer_tech":   "Transducer Tech",
	"8_foh_optimizer":     "FOH Optimizer",
	"9_mix_engineer":      "Mix Engineer",
	"10_control_mapper":   "Control Mapper",
	"11_dsp_dispatcher":   "DSP Dispatcher",
}

// pick resolves an optional override: nil means def.
func pick(v *bool, def bool) bool {
	if v == nil {
		return def
	}
	return *v
}

// DefaultGuitars is the guitar pair every UI run uses, and what an MCP run gets
// when it names none. A fresh slice each call, so callers cannot alias it.
func DefaultGuitars() []string {
	return []string{"Gibson ES-339 Humbuckers", "Fender Telecaster Single Coil"}
}

// buildConstraints turns GenerateOptions plus config into the constraints map
// RunPipeline consumes. (Moved from handleGeneratePreset; same keys and value
// types, since the pipeline and its tests type-assert them.)
func buildConstraints(cfg *config.AppConfig, o GenerateOptions) map[string]interface{} {
	guitars := DefaultGuitars()
	if len(o.Guitars) > 0 {
		guitars = append([]string(nil), o.Guitars...)
	}
	return map[string]interface{}{
		"guitars": guitars,
		// Request wins, config is only the default. (Not a ceiling like the
		// allow_* flags below: config.json ships allow_cloud_captures=false, so
		// a ceiling would make the MCP parameter impossible to turn on.)
		"single_amp_mode":        pick(o.SingleAmpMode, cfg.SingleAmpMode),
		"allow_cloud_captures":   pick(o.AllowCloudCaptures, cfg.AllowCloudCaptures),
		"allow_factory_captures": cfg.AllowFactoryCaptures && pick(o.AllowFactoryCaptures, true),
		"allow_user_captures":    cfg.AllowUserCaptures && pick(o.AllowUserCaptures, true),
		"favor_captures":         pick(o.FavorCaptures, cfg.FavorCaptures),
		"favor_cloud_captures":   pick(o.FavorCloudCaptures, cfg.FavorCloudCaptures),
		"allow_paid_plugins":     cfg.AllowPaidPlugins && pick(o.AllowPaidPlugins, true),
		"available_plugins":      cfg.AvailablePlugins,
	}
}

// BuildConstraints exposes buildConstraints so other packages' tests (the MCP
// UI-parity test) can check what a given option set resolves to.
func BuildConstraints(cfg *config.AppConfig, o GenerateOptions) map[string]interface{} {
	return buildConstraints(cfg, o)
}

// StartGenerate creates a run record, launches the pipeline in a detached
// goroutine and returns the run ID immediately. Errors: *CredentialsError when
// the API key cannot be resolved, ErrBusy when the concurrency cap is reached.
func (s *Service) StartGenerate(ctx context.Context, prompt string, o GenerateOptions) (string, error) {
	cfg := s.config()
	if cfg == nil {
		cfg = &config.AppConfig{
			SingleAmpMode:        true,
			AllowCloudCaptures:   false,
			AllowFactoryCaptures: true,
			AllowUserCaptures:    true,
			AllowPaidPlugins:     true,
		}
	}
	constraints := buildConstraints(cfg, o)

	apiKey, err := s.apiKey(ctx)
	if err != nil {
		return "", err
	}
	if !s.tryAcquire() {
		return "", ErrBusy
	}

	source := o.Source
	if source == "" {
		source = runs.SourceUI
	}
	saved := make(map[string]any, len(constraints))
	for k, v := range constraints {
		saved[k] = v
	}
	versions := make(map[string]string, len(cfg.AgentPrompts))
	for k, v := range cfg.AgentPrompts {
		versions[k] = v
	}
	runID := newRunID("gen-")
	mock := isMock(o.Mock)
	lr, runCtx := s.startRun(ctx, runs.RunRecord{
		ID:             runID,
		Kind:           runs.KindGenerate,
		Source:         source,
		Status:         runs.StatusRunning,
		Phase:          "Initializing ADK Pipeline...",
		UserPrompt:     prompt,
		Constraints:    saved,
		PromptVersions: versions,
	}, mock)

	go s.runGenerate(runCtx, lr, runID, apiKey, prompt, constraints, cfg)
	return runID, nil
}

// runGenerate is the former body of handleGeneratePreset's goroutine.
func (s *Service) runGenerate(ctx context.Context, lr *liveRun, runID, apiKey, prompt string, constraints map[string]interface{}, cfg *config.AppConfig) {
	defer s.release()
	defer func() {
		if r := recover(); r != nil {
			log.Printf("Generation panic for %s: %v", runID, r)
			s.fail(lr, ErrKindInternal, fmt.Sprintf("Internal error: %v", r))
		}
	}()

	orch, err := s.d.OrchMaker(ctx, apiKey)
	if err != nil {
		log.Printf("Failed to initialize Orchestrator: %v", err)
		s.fail(lr, ErrKindInit, fmt.Sprintf("Failed to initialize ADK: %v", err))
		return
	}
	defer orch.Close()

	htmlPayload, tokenUsage, err := orch.RunPipeline(ctx, prompt, constraints, cfg.AgentPrompts, lr.rec.OnPhase)

	if err != nil {
		log.Printf("Orchestrator generation failure: %v", err)
		s.fail(lr, ErrKindExec, fmt.Sprintf("Pipeline Execution Error: %v", err))
		return
	}

	htmlPayload = strings.TrimSpace(htmlPayload)
	htmlPayload = strings.TrimPrefix(htmlPayload, "```json")
	htmlPayload = strings.TrimPrefix(htmlPayload, "```")
	htmlPayload = strings.TrimSuffix(htmlPayload, "```")
	htmlPayload = strings.TrimSpace(htmlPayload)

	// Sanitize raw literal quotes within string properties to prevent decoding crashes
	htmlPayload = SanitizeJSONQuotes(htmlPayload)

	var archResp struct {
		BuilderStatement  string                   `json:"builder_statement"`
		FinalHTMLPayload  map[string]string        `json:"final_html_payload"`
		StructuredPayload storage.StructuredPreset `json:"structured_payload"`
		AgentImpact       []string                 `json:"agent_impact"`
	}

	if err := json.Unmarshal([]byte(htmlPayload), &archResp); err != nil {
		log.Printf("Failed to decode architect output: %v", err)
		s.fail(lr, ErrKindSerialize, fmt.Sprintf("Serialization Error: %v", err))
		return
	}

	allowFactoryCapturesForFlag := true
	if v, ok := constraints["allow_factory_captures"].(bool); ok {
		allowFactoryCapturesForFlag = v
	}
	allowUserCapturesForFlag := true
	if v, ok := constraints["allow_user_captures"].(bool); ok {
		allowUserCapturesForFlag = v
	}
	effectiveBlocksForFlag := agents.BuildEffectiveValidBlocks(allowFactoryCapturesForFlag, allowUserCapturesForFlag)
	agents.FlagCaptureFormattingMismatches(&archResp.StructuredPayload, effectiveBlocksForFlag)
	agents.FlagIncompleteCabinetBlocks(&archResp.StructuredPayload)
	agents.FlagLeftoverValueRanges(&archResp.StructuredPayload)
	agents.FlagUnverifiedStructuredBlocks(&archResp.StructuredPayload, effectiveBlocksForFlag)

	impactsHtml := "<ul>"
	for _, imp := range archResp.AgentImpact {
		processedImp := imp
		// Find which agent this line belongs to and append version
		for key, name := range impactAgentNames {
			if strings.Contains(imp, name) {
				version := "v1"
				if v, ok := cfg.AgentPrompts[key]; ok && v != "" {
					version = v
				}
				if !strings.Contains(imp, "v2") && !strings.Contains(imp, "v1") {
					newHeader := fmt.Sprintf("<strong>Agent %s (%s):</strong>", name, version)
					processedImp = strings.Replace(imp, "<strong>Agent", newHeader, 1)
					// If the LLM didn't use the strong Agent format, just append it
					if processedImp == imp {
						processedImp = fmt.Sprintf("<strong>%s (%s):</strong> %s", name, version, imp)
					}
				}
			}
		}
		impactsHtml += "<li>" + processedImp + "</li>"
	}
	impactsHtml += "</ul>"

	// A nil TokenUsage (an orchestrator with no usage to report) just omits the
	// stats card. The raw-string whitespace is persisted into the draft's chat
	// history and rendered verbatim, so keep it byte-identical to the original
	// handler's.
	tokenStatsHtml := ""
	if tokenUsage != nil {
		modelsList := ""
		for m, count := range tokenUsage.ModelsUsed {
			modelsList += fmt.Sprintf("%s (%d), ", m, count)
		}
		tokenStatsHtml = fmt.Sprintf(`
				<div class="card" style="margin-top: 1rem; font-family: monospace; color: #a1a1aa;">
					<strong>Pipeline AI Processing Tokens:</strong> Input: %d | Output: %d | Models: %s
				</div>`, tokenUsage.InputTokens, tokenUsage.OutputTokens, strings.TrimSuffix(modelsList, ", "))
	}

	initialAgentIntro := fmt.Sprintf(`<i>%s</i><br><br>%s`, impactsHtml, tokenStatsHtml)

	// Deliberately a map, not presetPayload: json.Marshal sorts map keys, so this
	// persists {"legacy_html":…,"structured":…}, byte-for-byte what the original
	// handler wrote. presetPayload (refine) writes structured first. Don't unify
	// without a migration decision; both orders decode identically.
	combinedPayload := map[string]interface{}{
		"structured":  archResp.StructuredPayload,
		"legacy_html": archResp.FinalHTMLPayload,
	}
	payloadBytes, err := json.Marshal(combinedPayload)
	if err != nil {
		s.fail(lr, ErrKindSerialize, fmt.Sprintf("Payload Serialization Error: %v", err))
		return
	}

	draftPreset := &storage.Preset{
		Name:             "Draft Preset",
		Payload:          string(payloadBytes),
		BuilderStatement: archResp.BuilderStatement,
		ChatHistory: []storage.ChatMessage{
			{Role: "user", Content: prompt},
			{Role: "model", Content: "Preset structure successfully laid out based on your requirements.\n" + initialAgentIntro},
		},
		RunIDs: []string{runID},
	}

	if err := s.d.Store.Save(ctx, draftPreset); err != nil {
		s.fail(lr, ErrKindStorage, fmt.Sprintf("Storage Error: %v", err))
		return
	}

	bgCtx := context.WithoutCancel(ctx)
	go CleanUpOldDrafts(bgCtx, s.d.Store)

	s.succeed(lr, draftPreset)
}
