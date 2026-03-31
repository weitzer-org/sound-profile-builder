package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"

	"github.com/weitzer-org/sound-builder/internal/agents"
	"github.com/weitzer-org/sound-builder/internal/config"
	"github.com/weitzer-org/sound-builder/internal/storage"
)

// Server holds the dependencies for our HTTP server.
type Server struct {
	mux         *http.ServeMux
	store       *storage.PresetStore
	client      storage.Client
	smFetcher   storage.SecretFetcher
	orchMaker   func(ctx context.Context, apiKey string) (agents.OrchestratorService, error)
	appConfig   *config.AppConfig
	apiKeyCache string
	apiKeyMu    sync.RWMutex
}

// NewServer initializes a new Server and its routes.
func NewServer(store *storage.PresetStore, client storage.Client, smFetcher storage.SecretFetcher, orchMaker func(ctx context.Context, apiKey string) (agents.OrchestratorService, error), appConfig *config.AppConfig) *Server {
	s := &Server{
		mux:       http.NewServeMux(),
		store:     store,
		client:    client,
		smFetcher: smFetcher,
		orchMaker: orchMaker,
		appConfig: appConfig,
	}
	s.routes()
	return s
}

// routes registers all the HTTP handlers for the application.
func (s *Server) routes() {
	// Public Routes
	s.mux.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			s.handleServeLogin()(w, r)
		} else if r.Method == http.MethodPost {
			s.handleProcessLogin()(w, r)
		} else {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		}
	})

	// HTMX Frontend Routes
	s.mux.Handle("/", s.authMiddleware(s.handleIndex()))

	// API Routes (to be protected by Secret Manager Password)
	s.mux.Handle("/api/preset/generate", s.authMiddleware(s.handleGeneratePreset()))
	
	// Preset Management Routes
	s.mux.Handle("/api/presets", s.authMiddleware(s.handleGetPresets()))
	s.mux.Handle("/api/preset/save", s.authMiddleware(s.handleSavePreset()))
	s.mux.Handle("/api/preset/delete", s.authMiddleware(s.handleDeletePreset()))
	s.mux.Handle("/api/preset/copy_ui", s.authMiddleware(s.handleCopyPresetUI()))
	s.mux.Handle("/api/preset/copy", s.authMiddleware(s.handleCopyPreset()))
	s.mux.Handle("/api/preset/view", s.authMiddleware(s.handleViewPreset()))
	s.mux.Handle("/api/preset/chat", s.authMiddleware(s.handleChatPreset()))
	s.mux.Handle("/api/preset/rename", s.authMiddleware(s.handleRenamePreset()))
	s.mux.Handle("/api/preset/delete_draft", s.authMiddleware(s.handleDeleteDraftPreset()))
}

// Start begins listening on the specified address.
func (s *Server) Start(addr string) error {
	return http.ListenAndServe(addr, s.mux)
}

// handleIndex serves the main HTMX dashboard.
func (s *Server) handleIndex() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "web/templates/index.html")
	}
}

// handleGeneratePreset kicks off the Gemini ADK Phase 1 Orchestrator.
func (s *Server) handleGeneratePreset() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			return
		}

		ctx := context.WithoutCancel(r.Context())

		// 1. Fetch from cache or explicitly fetch the exact Gemini API Key from GCP Secret Manager
		s.apiKeyMu.RLock()
		apiKey := s.apiKeyCache
		s.apiKeyMu.RUnlock()

		if apiKey == "" {
			s.apiKeyMu.Lock()
			if s.apiKeyCache == "" {
				key, err := s.smFetcher.GetPassword(ctx, "710019748844", "gsr-gemini-api-key")
				if err != nil {
					s.apiKeyMu.Unlock()
					log.Printf("Failed to fetch API key: %v", err)
					http.Error(w, "Missing Secure AI Credentials", http.StatusInternalServerError)
					return
				}
				s.apiKeyCache = key
			}
			apiKey = s.apiKeyCache
			s.apiKeyMu.Unlock()
		}

		// 2. Initialize the Orchestrator with the fetched API Key
		orch, err := s.orchMaker(ctx, apiKey)
		if err != nil {
			log.Printf("Failed to initialize Orchestrator: %v", err)
			http.Error(w, "Failed to initialize ADK", http.StatusInternalServerError)
			return
		}
		defer orch.Close()

		// 3. Extract payload from the incoming HTMX Form Request
		if err := r.ParseForm(); err != nil {
			log.Printf("Failed to parse ADK form: %v", err)
			http.Error(w, "Failed to parse generation request form", http.StatusBadRequest)
			return
		}

		if r.FormValue("mock") == "true" || os.Getenv("MOCK_MODE") == "true" {
			ctx = context.WithValue(ctx, agents.MockModeKey, true)
		}

		// Use globally loaded config
		cfg := s.appConfig
		if cfg == nil {
			log.Printf("Warning: Global config missing, using defaults")
			cfg = &config.AppConfig{SingleAmpMode: false, AllowCloudCaptures: true}
		}

		prompt := r.FormValue("prompt")
		guitars := []string{"Gibson ES-339 Humbuckers", "Fender Telecaster Single Coil"}
		
		constraints := map[string]interface{}{
			"guitars": guitars,
			"single_amp_mode": cfg.SingleAmpMode,
			"allow_cloud_captures": cfg.AllowCloudCaptures,
		}

		// 4. Trigger the multi-agent generation pipeline
		w.Header().Set("Content-Type", "text/html")

		htmlPayload, tokenUsage, err := orch.RunPipeline(ctx, prompt, constraints)
		if err != nil {
			log.Printf("Orchestrator generation failure: %v", err)
			w.Write([]byte(fmt.Sprintf(`<div class="grid-matrix" style="color: #ef4444; border-color: #ef4444;">Pipeline Execution Error: %v</div>`, err)))
			return
		}

		// Strip potential markdown wrapper blocks injected by the LLM
		htmlPayload = strings.TrimSpace(htmlPayload)
		htmlPayload = strings.TrimPrefix(htmlPayload, "```json")
		htmlPayload = strings.TrimPrefix(htmlPayload, "```")
		htmlPayload = strings.TrimSuffix(htmlPayload, "```")
		htmlPayload = strings.TrimSpace(htmlPayload)

		// Unmarshal the Architect's JSON block
		var archResp struct {
			BuilderStatement string            `json:"builder_statement"`
			FinalHtmlPayload map[string]string `json:"final_html_payload"`
			AgentImpact      []string          `json:"agent_impact"`
		}

		if err := json.Unmarshal([]byte(htmlPayload), &archResp); err != nil {
			log.Printf("Failed to decode architect output: %v - Raw payload: %s", err, htmlPayload)
			w.Write([]byte(fmt.Sprintf(`<div class="grid-matrix" style="color: #ef4444; border-color: #ef4444;">Serialization Error: %v</div>`, err)))
			return
		}

		// Rebuild cleanly into an HTMX-ready block
		impactsHtml := "<ul>"
		for _, imp := range archResp.AgentImpact {
			impactsHtml += "<li>" + imp + "</li>"
		}
		impactsHtml += "</ul>"

		modelsList := ""
		for m, count := range tokenUsage.ModelsUsed {
			modelsList += fmt.Sprintf("%s (%d), ", m, count)
		}
		
		tokenStatsHtml := ""
		if tokenUsage != nil {
			tokenStatsHtml = fmt.Sprintf(`
			<div class="card" style="margin-top: 1rem; font-family: monospace; color: #a1a1aa;">
				<strong>Pipeline AI Processing Tokens:</strong> Input: %d | Output: %d | Models: %s
			</div>`, tokenUsage.InputTokens, tokenUsage.OutputTokens, strings.TrimSuffix(modelsList, ", "))
		}

		initialAgentIntro := fmt.Sprintf(`<i>%s</i><br><br>%s`, impactsHtml, tokenStatsHtml)

		payloadBytes, err := json.Marshal(archResp.FinalHtmlPayload)
		if err != nil {
			log.Printf("Failed to marshal final html payload map: %v", err)
			w.Write([]byte(fmt.Sprintf(`<div class="grid-matrix" style="color: #ef4444;">Payload Serialization Error: %v</div>`, err)))
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
		}

		if err := s.store.Save(ctx, draftPreset); err != nil {
			log.Printf("Failed to save draft preset: %v", err)
			w.Write([]byte(fmt.Sprintf(`<div class="grid-matrix" style="color: #ef4444;">Storage Error: %v</div>`, err)))
			return
		}

		// Reload the list to trigger the newly added Draft Preset display (optional, but good for UI consistency)
		presets := cleanUpOldDrafts(ctx, s.store)
		
		// Return pure DOM swapping the MAIN workspace alongside updating the sidebar
		finalDOM := fmt.Sprintf(`
			<div id="preset-list-container" hx-swap-oob="true">
				%s
			</div>
			<div id="main-workspace" hx-swap-oob="true">
				%s
			</div>
		`, renderPresetList(presets, false), renderTweakingWorkspaceHTML(draftPreset, false))

		w.Write([]byte(finalDOM))
	}
}
