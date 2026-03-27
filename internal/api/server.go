package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/weitzer-org/sound-builder/internal/agents"
	"github.com/weitzer-org/sound-builder/internal/config"
	"github.com/weitzer-org/sound-builder/internal/storage"
)

// Server holds the dependencies for our HTTP server.
type Server struct {
	mux *http.ServeMux
}

// NewServer initializes a new Server and its routes.
func NewServer() *Server {
	s := &Server{
		mux: http.NewServeMux(),
	}
	s.routes()
	return s
}

// routes registers all the HTTP handlers for the application.
func (s *Server) routes() {
	// HTMX Frontend Routes
	s.mux.HandleFunc("/", s.handleIndex())

	// API Routes (to be protected by Secret Manager Password)
	s.mux.HandleFunc("/api/preset/generate", s.handleGeneratePreset())
	
	// Preset Management Routes
	s.mux.HandleFunc("/api/presets", s.handleGetPresets())
	s.mux.HandleFunc("/api/preset/save", s.handleSavePreset())
	s.mux.HandleFunc("/api/preset/delete", s.handleDeletePreset())
	s.mux.HandleFunc("/api/preset/copy", s.handleCopyPreset())
	s.mux.HandleFunc("/api/preset/edit", s.handleEditPreset())
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

		ctx := r.Context()

		// 1. Explicitly fetch the exact Gemini API Key from GCP Secret Manager
		smClient, err := storage.NewSecretManagerClient(ctx)
		if err != nil {
			log.Printf("Failed to init Secret Manager: %v", err)
			http.Error(w, "Failed to initialize Secret Manager", http.StatusInternalServerError)
			return
		}
		defer smClient.Close()

		apiKey, err := smClient.GetPassword(ctx, "710019748844", "gsr-gemini-api-key")
		if err != nil {
			log.Printf("Failed to fetch API key: %v", err)
			http.Error(w, "Missing Secure AI Credentials", http.StatusInternalServerError)
			return
		}

		// 2. Initialize the Orchestrator with the fetched API Key
		orch, err := agents.NewOrchestrator(ctx, apiKey)
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

		// Load config file globally
		cfg, err := config.LoadConfig("config.json")
		if err != nil {
			log.Printf("Warning: Could not load config.json, using defaults: %v", err)
			cfg = &config.AppConfig{SingleAmpMode: false, AllowCloudCaptures: true}
		}

		prompt := r.FormValue("prompt")
		guitars := r.Form["guitar"]
		
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
			FinalHtmlPayload string   `json:"final_html_payload"`
			AgentImpact      []string `json:"agent_impact"`
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

		finalDOM := fmt.Sprintf(`
			<div class="card">
				%s
				
				<!-- Trigger Button -->
				<button type="button" onclick="document.getElementById('saveModal').classList.add('active')" style="display:block; width:100%%; margin-top:2rem; padding:1rem; background:var(--success); font-size:1.1rem; border-radius:8px; cursor:pointer; color:white; border:none; font-weight:bold;">Save Custom Preset</button>
				
				<!-- Modal Overlay -->
				<div id="saveModal" class="modal-overlay">
					<div class="modal-content">
						<div class="modal-header">
							<h3>Save Custom Preset</h3>
							<button type="button" class="modal-close" onclick="document.getElementById('saveModal').classList.remove('active')">&times;</button>
						</div>
						<form hx-post="/api/preset/save" hx-target="#preset-list-container" onsubmit="document.getElementById('saveModal').classList.remove('active')">
							<div class="form-group" style="text-align: left; margin-bottom: 0;">
								<label style="color:#cbd5e1; margin-bottom:0.5rem; font-size:0.9rem;">Preset Name</label>
								<input type="text" name="preset_name" placeholder="Enter preset name..." autocomplete="off" required style="width:100%%; background: rgba(15,23,42,0.5); border: 1px solid rgba(255,255,255,0.2);">
							</div>
							<input type="hidden" name="payload" value='%s'>
							<div class="modal-actions">
								<button type="button" class="btn-cancel" onclick="document.getElementById('saveModal').classList.remove('active')" style="padding:0.75rem; border-radius:8px; border:none; cursor:pointer; font-weight:600;">Cancel</button>
								<button type="submit" class="btn-save" style="padding:0.75rem; border-radius:8px; border:none; cursor:pointer; font-weight:600;">Save to Cloud</button>
							</div>
						</form>
					</div>
				</div>
			</div>
			<div class="card">
				<h2>Agent Architecture Log</h2>
				%s
			</div>
			%s
		`, archResp.FinalHtmlPayload, archResp.FinalHtmlPayload, impactsHtml, tokenStatsHtml)

		// 5. Output pure DOM back directly to the #result target!
		w.Write([]byte(finalDOM))
	}
}
