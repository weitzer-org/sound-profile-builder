package presetsvc

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/weitzer-org/sound-builder/internal/agents"
	"github.com/weitzer-org/sound-builder/internal/runs"
	"github.com/weitzer-org/sound-builder/internal/storage"
)

// presetPayload is the on-disk {structured, legacy_html} envelope. Field order
// matters: it fixes the key order of the marshalled payload.
type presetPayload struct {
	Structured storage.StructuredPreset `json:"structured"`
	LegacyHTML map[string]string        `json:"legacy_html"`
}

// StartRefine loads the preset, creates a run record and launches the
// refinement in a detached goroutine, returning the run ID immediately.
// Errors: *LookupError (preset not found), *CredentialsError, ErrBusy.
func (s *Service) StartRefine(ctx context.Context, presetID, message string, mock bool) (string, error) {
	return s.startRefine(ctx, presetID, message, mock, runs.SourceUI)
}

// StartRefineFrom is StartRefine with an explicit run source ("ui" | "mcp").
func (s *Service) StartRefineFrom(ctx context.Context, presetID, message string, mock bool, source string) (string, error) {
	return s.startRefine(ctx, presetID, message, mock, source)
}

func (s *Service) startRefine(ctx context.Context, presetID, message string, mock bool, source string) (string, error) {
	p, err := s.d.Store.Get(context.WithoutCancel(ctx), presetID)
	if err != nil {
		return "", &LookupError{Err: err}
	}
	apiKey, err := s.apiKey(ctx)
	if err != nil {
		return "", err
	}
	if !s.tryAcquire() {
		return "", ErrBusy
	}

	// One refinement per preset at a time (see ErrPresetBusy). Re-read the preset
	// once the guard is held so the run starts from the latest saved state, not
	// from whatever it was before an earlier refinement finished.
	s.mu.Lock()
	if _, busy := s.refining[presetID]; busy {
		s.mu.Unlock()
		s.release()
		return "", ErrPresetBusy
	}
	s.refining[presetID] = struct{}{}
	s.mu.Unlock()
	if p, err = s.d.Store.Get(context.WithoutCancel(ctx), presetID); err != nil {
		s.endRefine(presetID)
		s.release() // the run never started, so give its slot back too
		return "", &LookupError{Err: err}
	}

	cfg := s.config()
	versions := map[string]string{}
	if cfg != nil {
		for k, v := range cfg.AgentPrompts {
			versions[k] = v
		}
	}
	if source == "" {
		source = runs.SourceUI
	}
	runID := newRunID("chat-")
	lr, runCtx := s.startRun(ctx, runs.RunRecord{
		ID:             runID,
		Kind:           runs.KindRefine,
		Source:         source,
		Status:         runs.StatusRunning,
		Phase:          "Refining preset...",
		UserPrompt:     message,
		Constraints:    map[string]any{},
		PromptVersions: versions,
		ParentPresetID: presetID,
	}, isMock(mock))

	go s.runRefine(runCtx, lr, runID, presetID, apiKey, p, message)
	return runID, nil
}

// endRefine releases a preset's in-flight refinement guard.
func (s *Service) endRefine(presetID string) {
	s.mu.Lock()
	delete(s.refining, presetID)
	s.mu.Unlock()
}

// runRefine is the former body of handleChatPreset after credential lookup.
func (s *Service) runRefine(ctx context.Context, lr *liveRun, runID, presetID, apiKey string, p *storage.Preset, userMessage string) {
	defer s.release()
	defer s.endRefine(presetID)
	defer func() {
		if r := recover(); r != nil {
			log.Printf("Refinement panic for %s: %v", runID, r)
			s.fail(lr, ErrKindInternal, fmt.Sprintf("Internal error: %v", r))
		}
	}()

	orch, err := s.d.OrchMaker(ctx, apiKey)
	if err != nil {
		log.Printf("Failed to initialize Orchestrator service: %v", err)
		s.fail(lr, ErrKindInit, fmt.Sprintf("Failed to initialize ADK: %v", err))
		return
	}
	defer orch.Close()

	allowFactoryCapturesForFlag := true
	allowUserCapturesForFlag := true
	var pluginGate agents.PluginGate // inactive (no plugin filtering) without a config
	refineCtx := ctx
	if cfg := s.config(); cfg != nil {
		allowFactoryCapturesForFlag = cfg.AllowFactoryCaptures
		allowUserCapturesForFlag = cfg.AllowUserCaptures
		pluginGate = agents.NewPluginGate(cfg.AllowPaidPlugins, cfg.AvailablePlugins)
		// RefineChat uses the configured Architect prompt version (it used to be fixed at v4).
		refineCtx = agents.WithPromptVersions(ctx, cfg.AgentPrompts)
	}

	// Run the chat refinement loop
	jsonResponse, _, err := orch.RefineChat(refineCtx, p, userMessage, allowFactoryCapturesForFlag, allowUserCapturesForFlag, pluginGate)
	if err != nil {
		s.fail(lr, ErrKindExec, err.Error())
		return
	}

	// Strip markdown if exists
	jsonResponse = strings.TrimSpace(jsonResponse)
	jsonResponse = strings.TrimPrefix(jsonResponse, "```json")
	jsonResponse = strings.TrimPrefix(jsonResponse, "```")
	jsonResponse = strings.TrimSuffix(jsonResponse, "```")
	jsonResponse = strings.TrimSpace(jsonResponse)

	// Sanitize raw literal quotes within string properties to prevent decoding crashes
	jsonResponse = SanitizeJSONQuotes(jsonResponse)

	var archResp struct {
		ConversationalResponse string                   `json:"conversational_response"`
		BuilderStatement       string                   `json:"builder_statement"`
		DSPMatrixUpdated       bool                     `json:"dsp_matrix_updated"`
		StructuredPayload      storage.StructuredPreset `json:"structured_payload"`
		FinalHTMLPayload       map[string]string        `json:"final_html_payload"`
		AgentImpact            []string                 `json:"agent_impact"`
	}

	if err := json.Unmarshal([]byte(jsonResponse), &archResp); err != nil {
		log.Printf("Failed to unmarshal chat architect: %v, raw: %s", err, jsonResponse)
		s.fail(lr, ErrKindSerialize, fmt.Sprintf("Serialization Error: %v", err))
		return
	}

	effectiveBlocksForFlag := agents.BuildEffectiveValidBlocksFor(agents.ValidBlockFilter{
		AllowFactoryCaptures: allowFactoryCapturesForFlag,
		AllowUserCaptures:    allowUserCapturesForFlag,
		Plugins:              pluginGate,
	})
	agents.FlagCaptureFormattingMismatches(&archResp.StructuredPayload, effectiveBlocksForFlag)
	agents.FlagIncompleteCabinetBlocks(&archResp.StructuredPayload)
	agents.FlagLeftoverValueRanges(&archResp.StructuredPayload)
	agents.FlagUnverifiedStructuredBlocks(&archResp.StructuredPayload, effectiveBlocksForFlag)
	// After FlagUnverifiedStructuredBlocks: it checks the snapped names and skips unverified ones.
	agents.FlagCatalogParamMismatches(&archResp.StructuredPayload, effectiveBlocksForFlag)
	// RefineChat corrected the rendered tables against the unfiltered block set (it isn't
	// told which plugins the user owns). Re-apply the correction with this run's set so the
	// saved HTML flags the same blocks the structured payload does, e.g. an unowned plugin
	// device. Safe to repeat: capture labels are re-derived and flags aren't doubled.
	for guitar, html := range archResp.FinalHTMLPayload {
		archResp.FinalHTMLPayload[guitar] = agents.ApplyFuzzyCorrection(html, effectiveBlocksForFlag)
	}

	// Append user message
	p.ChatHistory = append(p.ChatHistory, storage.ChatMessage{Role: "user", Content: userMessage})

	// Append architect response including impacts
	assistantContent := archResp.ConversationalResponse
	if archResp.DSPMatrixUpdated && len(archResp.AgentImpact) > 0 {
		assistantContent += "\n\n**Impact:**\n"
		for _, imp := range archResp.AgentImpact {
			assistantContent += "- " + imp + "\n"
		}
	}

	p.ChatHistory = append(p.ChatHistory, storage.ChatMessage{Role: "model", Content: assistantContent})

	if archResp.DSPMatrixUpdated && archResp.BuilderStatement != "" {
		p.BuilderStatement = archResp.BuilderStatement
	}

	// dspSaved tracks whether p.Payload was actually replaced below, so a
	// reply never claims the matrix changed when the response had no guitars
	// or failed to marshal and the old payload was kept.
	dspSaved := false
	if archResp.DSPMatrixUpdated {
		if len(archResp.StructuredPayload.Guitars) > 0 {
			// Preserve the {structured, legacy_html} envelope (same shape
			// generation writes) so the just-rendered HTML -- including
			// the per-block Rationale and Scene B values injectRenderedHTML just
			// computed -- survives the refinement instead of being dropped, which
			// would otherwise silently fall back to the legacy renderer's
			// rationale-blind, single-scene table on the next view.
			payloadBytes, err := json.Marshal(presetPayload{
				Structured: archResp.StructuredPayload,
				LegacyHTML: archResp.FinalHTMLPayload,
			})
			if err != nil {
				log.Printf("Failed to marshal structured payload: %v", err)
			} else {
				p.Payload = string(payloadBytes)
				dspSaved = true
			}

			// Auto-Capture Learned Rule
			m := &storage.Memory{
				Artist:     p.Name, // Topic context
				Critique:   userMessage,
				Action:     strings.Join(archResp.AgentImpact, "; "),
				BasePreset: p.ID,
			}
			if err := s.d.Memories.Save(ctx, m); err != nil {
				log.Printf("Failed to save learned rule: %v", err)
			} else {
				log.Printf("Successfully saved learned rule for %s", p.Name)
			}
		} else {
			log.Printf("Warning: Agent returned empty StructuredPayload despite DSPMatrixUpdated=true. Skipping overwrite.")
		}
	}

	p.RunIDs = append(p.RunIDs, runID)

	// Save the state. A failed save must not report success: the run would claim a
	// change that isn't stored, and the caller would render an unsaved preset.
	if err := s.d.Store.Save(ctx, p); err != nil {
		log.Printf("Failed to save refined preset %s: %v", p.ID, err)
		s.fail(lr, ErrKindStorage, fmt.Sprintf("Storage Error: %v", err))
		return
	}

	lr.mu.Lock()
	lr.reply, lr.dspUpdated = archResp.ConversationalResponse, dspSaved
	lr.mu.Unlock()
	s.succeed(lr, p)
}
