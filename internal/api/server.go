package api

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/weitzer-org/sound-builder/internal/agents"
	"github.com/weitzer-org/sound-builder/internal/config"
	"github.com/weitzer-org/sound-builder/internal/storage"
)

// Server holds the dependencies for our HTTP server.
type TaskState struct {
	Status string // "running", "complete", "error"
	Phase  string
	Result string // HTML for workspace
	Error  string
}

type Server struct {
	mux         *http.ServeMux
	store       *storage.PresetStore
	memoryStore *storage.MemoryStore
	client      storage.Client
	smFetcher   storage.SecretFetcher
	orchMaker   func(ctx context.Context, apiKey string) (agents.OrchestratorService, error)
	appConfig   *config.AppConfig
	apiKeyCache string
	apiKeyMu    sync.RWMutex
	tasks       map[string]*TaskState
	tasksMu     sync.RWMutex
}

// NewServer initializes a new Server and its routes.
func NewServer(store *storage.PresetStore, memoryStore *storage.MemoryStore, client storage.Client, smFetcher storage.SecretFetcher, orchMaker func(ctx context.Context, apiKey string) (agents.OrchestratorService, error), appConfig *config.AppConfig) *Server {
	s := &Server{
		mux:         http.NewServeMux(),
		store:       store,
		memoryStore: memoryStore,
		client:      client,
		smFetcher:   smFetcher,
		orchMaker:   orchMaker,
		appConfig:   appConfig,
		tasks:       make(map[string]*TaskState),
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
	s.mux.Handle("/api/preset/status", s.authMiddleware(s.handleTaskStatus()))
	
	// Preset Management Routes
	s.mux.Handle("/api/presets", s.authMiddleware(s.handleGetPresets()))
	s.mux.Handle("/api/preset/save", s.authMiddleware(s.handleSavePreset()))
	s.mux.Handle("/api/preset/delete", s.authMiddleware(s.handleDeletePreset()))
	s.mux.Handle("/api/preset/copy_ui", s.authMiddleware(s.handleCopyPresetUI()))
	s.mux.Handle("/api/preset/copy", s.authMiddleware(s.handleCopyPreset()))
	s.mux.Handle("/api/preset/view", s.authMiddleware(s.handleViewPreset()))
	s.mux.Handle("/api/preset/chat", s.authMiddleware(s.handleChatPreset()))
	s.mux.Handle("/api/preset/rename", s.authMiddleware(s.handleRenamePreset()))
	s.mux.Handle("/api/preset/update_parameter", s.authMiddleware(s.handleUpdateParameter()))
	s.mux.Handle("/api/preset/remove_block", s.authMiddleware(s.handleRemoveBlock()))
	s.mux.Handle("/api/preset/delete_draft", s.authMiddleware(s.handleDeleteDraftPreset()))

	// Memory Rules Routes
	s.mux.Handle("/api/memories", s.authMiddleware(s.handleGetMemories()))
	s.mux.Handle("/api/memory/delete", s.authMiddleware(s.handleDeleteMemory()))
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

		if err := r.ParseForm(); err != nil {
			log.Printf("Failed to parse ADK form: %v", err)
			http.Error(w, "Failed to parse generation request form", http.StatusBadRequest)
			return
		}

		prompt := r.FormValue("prompt")
		if prompt == "" {
			http.Error(w, "Prompt is required", http.StatusBadRequest)
			return
		}

		cfg := s.appConfig
		if cfg == nil {
			cfg = &config.AppConfig{SingleAmpMode: false, AllowCloudCaptures: true}
		}

		constraints := map[string]interface{}{
			"guitars":              []string{"Gibson ES-339 Humbuckers", "Fender Telecaster Single Coil"},
			"single_amp_mode":      cfg.SingleAmpMode,
			"allow_cloud_captures": cfg.AllowCloudCaptures,
		}

		ctx := context.WithoutCancel(r.Context())
		if r.FormValue("mock") == "true" || os.Getenv("MOCK_MODE") == "true" {
			ctx = context.WithValue(ctx, agents.MockModeKey, true)
		}

		s.apiKeyMu.RLock()
		apiKey := s.apiKeyCache
		s.apiKeyMu.RUnlock()

		if apiKey == "" {
			s.apiKeyMu.Lock()
			if s.apiKeyCache == "" {
				projectID := "710019748844"
				if s.appConfig != nil && s.appConfig.ProjectID != "" {
					projectID = s.appConfig.ProjectID
				}
				secretName := os.Getenv("GEMINI_API_KEY_NAME")
				if secretName == "" {
					secretName = "gsr-gemini-api-key"
				}
				key, err := s.smFetcher.GetPassword(ctx, projectID, secretName)
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

		taskID := fmt.Sprintf("gen-%d", time.Now().UnixNano())

		s.tasksMu.Lock()
		s.tasks[taskID] = &TaskState{
			Status: "running",
			Phase:  "Initializing ADK Pipeline...",
		}
		s.tasksMu.Unlock()

		go func() {
			orch, err := s.orchMaker(ctx, apiKey)
			if err != nil {
				log.Printf("Failed to initialize Orchestrator: %v", err)
				s.updateTaskError(taskID, fmt.Sprintf("Failed to initialize ADK: %v", err))
				return
			}
			defer orch.Close()

			htmlPayload, tokenUsage, err := orch.RunPipeline(ctx, prompt, constraints, func(phase string) {
				s.tasksMu.Lock()
				if task, ok := s.tasks[taskID]; ok {
					task.Phase = phase
				}
				s.tasksMu.Unlock()
			})

			if err != nil {
				log.Printf("Orchestrator generation failure: %v", err)
				s.updateTaskError(taskID, fmt.Sprintf("Pipeline Execution Error: %v", err))
				return
			}

			htmlPayload = strings.TrimSpace(htmlPayload)
			htmlPayload = strings.TrimPrefix(htmlPayload, "```json")
			htmlPayload = strings.TrimPrefix(htmlPayload, "```")
			htmlPayload = strings.TrimSuffix(htmlPayload, "```")
			htmlPayload = strings.TrimSpace(htmlPayload)

			var archResp struct {
				BuilderStatement  string                  `json:"builder_statement"`
				FinalHTMLPayload  map[string]string       `json:"final_html_payload"`
				StructuredPayload storage.StructuredPreset `json:"structured_payload"`
				AgentImpact       []string                `json:"agent_impact"`
			}

			if err := json.Unmarshal([]byte(htmlPayload), &archResp); err != nil {
				log.Printf("Failed to decode architect output: %v", err)
				s.updateTaskError(taskID, fmt.Sprintf("Serialization Error: %v", err))
				return
			}

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

			combinedPayload := map[string]interface{}{
				"structured":  archResp.StructuredPayload,
				"legacy_html": archResp.FinalHTMLPayload,
			}
			payloadBytes, err := json.Marshal(combinedPayload)
			if err != nil {
				s.updateTaskError(taskID, fmt.Sprintf("Payload Serialization Error: %v", err))
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
				s.updateTaskError(taskID, fmt.Sprintf("Storage Error: %v", err))
				return
			}

			cleanUpOldDrafts(ctx, s.store)

			s.tasksMu.Lock()
			if task, ok := s.tasks[taskID]; ok {
				task.Status = "complete"
				task.Result = renderTweakingWorkspaceHTML(draftPreset, false)
			}
			s.tasksMu.Unlock()
		}()

		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(fmt.Sprintf(`
			<div id="progress-area" hx-get="/api/preset/status?id=%s" hx-trigger="every 2s" hx-swap="outerHTML">
				<div class="progress-panel" style="padding: 0.75rem 1rem; display: flex; flex-direction: row; align-items: center; gap: 0.75rem;">
					<span class="spinner" style="display:inline-block;"></span>
					<span style="color: white; font-size: 0.95rem;">Current: <span style="color: var(--accent);">Initializing ADK Pipeline...</span></span>
				</div>
			</div>
			<button id="submit-btn" style="display: none;" hx-swap-oob="true"></button>
		`, taskID)))
	}
}

func (s *Server) handleTaskStatus() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.URL.Query().Get("id")
		if id == "" {
			http.Error(w, "Missing task ID", http.StatusBadRequest)
			return
		}

		s.tasksMu.RLock()
		task, ok := s.tasks[id]
		s.tasksMu.RUnlock()

		if !ok {
			http.Error(w, "Task not found", http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "text/html")

		isChat := strings.HasPrefix(id, "chat-")
		btnID := "submit-btn"
		if isChat {
			btnID = "chat-submit-btn"
		}

		if task.Status == "running" {
			if isChat {
				w.Write([]byte(fmt.Sprintf(`
					<button id="%s" hx-get="/api/preset/status?id=%s" hx-trigger="every 2s" hx-target="#%s" hx-swap="outerHTML" class="htmx-request">
						<span class="spinner" style="display:inline-block;"></span>
						<span class="btn-text">%s</span>
					</button>
				`, btnID, id, btnID, html.EscapeString(task.Phase))))
				return
			}


			areaID := "progress-area"
			if isChat {
				areaID = "chat-progress-area"
			}
			w.Write([]byte(fmt.Sprintf(`
				<div id="%s" hx-get="/api/preset/status?id=%s" hx-trigger="every 2s" hx-swap="outerHTML">
					<div class="progress-panel" style="padding: 0.75rem 1rem; display: flex; flex-direction: row; align-items: center; gap: 0.75rem;">
						<span class="spinner" style="display:inline-block;"></span>
						<span style="color: white; font-size: 0.95rem;">Current: <span style="color: var(--accent);">%s</span></span>
					</div>
				</div>
			`, areaID, id, html.EscapeString(task.Phase))))
			return
		}

		if task.Status == "error" {
			w.Write([]byte(fmt.Sprintf(`
				<button id="%s" class="error">
					Error: %s
				</button>
			`, btnID, html.EscapeString(task.Error))))
			return
		}

		if isChat {
			w.Write([]byte(fmt.Sprintf(`
				<div id="chat-progress-area"></div>
				<button id="chat-submit-btn" hx-swap-oob="true" style="padding: 0.85rem 1.5rem; border-radius: 8px; background: var(--accent); color: white; border: none; font-weight: 600; font-size: 0.95rem; cursor: pointer; transition: all 0.2s; height: 48px; display: flex; align-items: center; justify-content: center; gap: 0.5rem; min-width: 100px;">
					<span class="spinner"></span>
					<span class="btn-text">Adjust</span>
				</button>
				<div id="workspace-wrapper" hx-swap-oob="true">
					%s
				</div>
			`, task.Result)))
			return
		}

		w.Write([]byte(fmt.Sprintf(`
			<div id="progress-area"></div>
			<button id="submit-btn" hx-swap-oob="true">
				<span class="spinner"></span>
				<span class="btn-text">Spin Up ADK Pipeline</span>
			</button>
			<div id="generator-workspace" hx-swap-oob="true">
				%s
			</div>
		`, task.Result)))
		return


	}
}

func (s *Server) updateTaskError(taskID, errMsg string) {
	s.tasksMu.Lock()
	if task, ok := s.tasks[taskID]; ok {
		task.Status = "error"
		task.Error = errMsg
	}
	s.tasksMu.Unlock()
}
