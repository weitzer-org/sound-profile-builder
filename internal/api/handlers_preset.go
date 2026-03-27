package api

import (
	"encoding/json"
	"fmt"
	"html"
	"log"
	"net/http"
	"strings"

	"github.com/weitzer-org/sound-builder/internal/storage"
)

// bucketName is the default bucket used for everything right now
const presetBucketName = "weitzer-sound-builder"

// renderPresetList returning HTML fragment for HTMX
func renderPresetList(presets []*storage.Preset) string {
	if len(presets) == 0 {
		return `<p class="subtitle" style="font-size:0.9rem;">No presets saved yet.</p>`
	}

	html := `<ul style="list-style-type: none; padding: 0;">`
	for _, p := range presets {
		html += fmt.Sprintf(`
			<li style="margin-bottom: 1rem; border-bottom: 1px solid var(--border); padding-bottom: 1rem;">
				<h3 style="margin: 0 0 0.5rem 0; font-size: 1.1rem;">%s</h3>
				<span style="font-size: 0.8rem; color: var(--text-sub);">Saved: %s</span>
				<div style="margin-top: 0.5rem; display: flex; gap: 0.5rem;">
					<button hx-post="/api/preset/copy" hx-vals='{"id":"%s"}' hx-target="#preset-list-container" style="flex: 1; padding: 0.5rem; font-size: 0.9rem; background: var(--bg-dark); border: 1px solid var(--border);">Copy</button>
					<button hx-post="/api/preset/delete" hx-vals='{"id":"%s"}' hx-target="#preset-list-container" style="flex: 1; padding: 0.5rem; font-size: 0.9rem; background: #ef4444; border: 1px solid #b91c1c;">Delete</button>
				</div>
				<div style="margin-top: 0.5rem;">
					<button hx-get="/api/preset/view?id=%s" hx-target="#main-workspace" style="width: 100%%; padding: 0.5rem; font-size: 0.9rem; background: var(--success);">Adjust preset</button>
				</div>
			</li>`, p.Name, p.UpdatedAt, p.ID, p.ID, p.ID)
	}
	html += `</ul>`
	return html
}

func (s *Server) handleGetPresets() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		presets, err := s.store.List(r.Context())
		if err != nil {
			http.Error(w, "Failed to list presets", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(renderPresetList(presets)))
	}
}

func (s *Server) handleSavePreset() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, "Bad form config", http.StatusBadRequest)
			return
		}

		name := r.FormValue("preset_name")
		payload := r.FormValue("payload")
		if name == "" {
			name = "Unnamed Preset"
		}

		preset := &storage.Preset{
			Name:    name,
			Payload: payload,
		}

		if err := s.store.Save(r.Context(), preset); err != nil {
			log.Printf("Failed saving preset: %v", err)
			http.Error(w, "Failed saving preset", http.StatusInternalServerError)
			return
		}

		// Reload the list
		presets, _ := s.store.List(r.Context())
		
		w.Header().Set("Content-Type", "text/html")
		w.Header().Set("HX-Trigger", "presetSaved")
		
		oobResponse := fmt.Sprintf(`
			<div id="preset-list-container" hx-swap-oob="true">
				%s
			</div>
			<div id="toast-container" hx-swap-oob="beforeend:body">
				<div class="toast show">Successfully saved "%s"!</div>
			</div>
		`, renderPresetList(presets), name)
		
		w.Write([]byte(oobResponse))
	}
}

func (s *Server) handleDeletePreset() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, "Bad form config", http.StatusBadRequest)
			return
		}

		id := r.FormValue("id")
		if id == "" {
			http.Error(w, "Missing ID", http.StatusBadRequest)
			return
		}

		if err := s.store.Delete(r.Context(), id); err != nil {
			log.Printf("Failed deleting preset: %v", err)
			http.Error(w, "Failed deleting preset", http.StatusInternalServerError)
			return
		}

		// Reload the list
		presets, _ := s.store.List(r.Context())
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(renderPresetList(presets)))
	}
}

func (s *Server) handleCopyPreset() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, "Bad form config", http.StatusBadRequest)
			return
		}

		id := r.FormValue("id")
		p, err := s.store.Get(r.Context(), id)
		if err != nil {
			http.Error(w, "Preset not found", http.StatusNotFound)
			return
		}

		// Save a stripped copy
		pCopy := &storage.Preset{
			Name:    p.Name + " (Copy)",
			Payload: p.Payload,
		}
		
		if err := s.store.Save(r.Context(), pCopy); err != nil {
			http.Error(w, "Failed to duplicate", http.StatusInternalServerError)
			return
		}

		// Reload the list
		presets, _ := s.store.List(r.Context())
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(renderPresetList(presets)))
	}
}

func (s *Server) handleDeleteDraftPreset() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, "Bad form config", http.StatusBadRequest)
			return
		}

		id := r.FormValue("id")
		if id == "" {
			http.Error(w, "Missing ID", http.StatusBadRequest)
			return
		}

		if err := s.store.Delete(r.Context(), id); err != nil {
			log.Printf("Failed deleting draft preset: %v", err)
			http.Error(w, "Failed deleting draft preset", http.StatusInternalServerError)
			return
		}

		// Just reload the whole page via a JS redirect since we are discarding the current workspace state
		w.Header().Set("HX-Redirect", "/")
		w.WriteHeader(http.StatusOK)
	}
}

func (s *Server) handleRenamePreset() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, "Bad form config", http.StatusBadRequest)
			return
		}

		id := r.FormValue("id")
		name := r.FormValue("preset_name")
		if id == "" || name == "" {
			http.Error(w, "Missing ID or name", http.StatusBadRequest)
			return
		}

		ctx := r.Context()
		p, err := s.store.Get(ctx, id)
		if err != nil {
			http.Error(w, "Preset not found", http.StatusNotFound)
			return
		}

		p.Name = name
		if err := s.store.Save(ctx, p); err != nil {
			log.Printf("Failed to rename preset: %v", err)
			http.Error(w, "Failed renaming preset", http.StatusInternalServerError)
			return
		}

		// Reload the list AND replace the workspace header simultaneously
		presets, _ := s.store.List(ctx)
		oobResponse := fmt.Sprintf(`
			<div id="preset-list-container" hx-swap-oob="true">
				%s
			</div>
			<div id="toast-container" hx-swap-oob="beforeend:body">
				<div class="toast show">Successfully saved "%s"!</div>
			</div>
			%s
		`, renderPresetList(presets), name, renderTweakingWorkspaceHTML(p))
		
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(oobResponse))
	}
}

// renderTweakingWorkspaceHTML constructs the Side-by-Side editing view for a Preset
func renderTweakingWorkspaceHTML(p *storage.Preset) string {
	// Render chat history
	historyHtml := ""
	for _, msg := range p.ChatHistory {
		if msg.Role == "user" {
			historyHtml += fmt.Sprintf(`
			<div style="align-self: flex-end; background: var(--accent); color: white; padding: 0.75rem 1rem; border-radius: 12px 12px 0 12px; max-width: 80%%;">
				<b>You:</b><br>%s
			</div>`, html.EscapeString(msg.Content))
		} else {
			historyHtml += fmt.Sprintf(`
			<div style="align-self: flex-start; background: #334155; color: white; padding: 0.75rem 1rem; border-radius: 12px 12px 12px 0; max-width: 80%%;">
				<b>Agent process:</b><br>%s
			</div>`, strings.ReplaceAll(html.EscapeString(msg.Content), "\n", "<br>"))
		}
	}

	headerHtml := ""
	if p.Name == "Draft Preset" {
		headerHtml = fmt.Sprintf(`
			<div style="display: flex; gap: 0.5rem; align-items: center;">
				<form hx-post="/api/preset/rename" hx-target="#main-workspace" style="display:flex; gap:0.5rem; margin:0;" autocomplete="off">
					<input type="hidden" name="id" value="%s">
					<input type="text" name="preset_name" placeholder="Enter custom name..." required style="padding: 0.4rem; background: rgba(15,23,42,0.5); border: 1px solid rgba(255,255,255,0.2);">
					<button type="submit" style="width: auto; padding: 0.4rem 0.8rem; font-size: 0.8rem; background: var(--success);">Finalize Save</button>
				</form>
				<button hx-post="/api/preset/delete_draft" hx-vals='{"id":"%s"}' hx-target="body" style="width: auto; padding: 0.4rem 0.8rem; font-size: 0.8rem; background: #ef4444; border: 1px solid #b91c1c;">Discard / Exit</button>
			</div>
		`, p.ID, p.ID)
	} else {
		headerHtml = fmt.Sprintf(`
			<h2 style="font-size: 1.1rem; margin: 0; color: var(--text-sub);">Preset: <span style="color:white;">%s</span></h2>
			<button onclick="window.location.reload()" style="width: auto; padding: 0.4rem 0.8rem; font-size: 0.8rem; background: var(--bg-dark); border: 1px solid var(--border);">Back / Exit</button>
		`, html.EscapeString(p.Name))
	}

	return fmt.Sprintf(`
	<div class="tweaking-workspace" style="display: grid; grid-template-columns: 1fr 1fr; gap: 1rem;">
		<!-- Left: Matrix -->
		<div class="card" style="padding: 1.5rem; margin-bottom: 0;">
			<h2 style="font-size: 1.25rem; margin-top: 0;">Live DSP Matrix</h2>
			<div id="live-matrix-container" style="zoom: 0.8; overflow-x: auto;">
				%s
			</div>
		</div>
		<!-- Right: Chat Thread -->
		<div class="card" style="padding: 1.5rem; display: flex; flex-direction: column; height: 650px; margin-bottom: 0;">
			<div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 1rem;">
				%s
			</div>
			
			<div id="chat-thread" style="flex: 1; overflow-y: auto; display: flex; flex-direction: column; gap: 1rem; padding-right: 0.5rem; margin-bottom: 1rem;">
				<div style="align-self: flex-start; background: #334155; color: white; padding: 0.75rem 1rem; border-radius: 12px 12px 12px 0; max-width: 80%%;">
					<b>Agent process:</b><br>Adjustments ready. How do you want to modify this preset?
				</div>
				%s
			</div>

			<form hx-post="/api/preset/chat" hx-target="#chat-thread" hx-swap="beforeend" hx-on::after-request="this.reset()" style="display: flex; gap: 0.5rem;" autocomplete="off">
				<input type="hidden" name="id" value="%s">
				<input type="text" name="message" id="chat-input" placeholder="e.g., Make the amp darker..." style="flex: 1;" required>
				<button type="submit" style="width: auto; padding: 0 1rem; border-radius: 8px;">
					<span class="htmx-indicator spinner-small"></span><span class="btn-text">Adjust</span>
				</button>
			</form>
		</div>
	</div>
	`, p.Payload, headerHtml, historyHtml, p.ID)
}

func (s *Server) handleViewPreset() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.URL.Query().Get("id")
		if id == "" {
			http.Error(w, "Missing ID", http.StatusBadRequest)
			return
		}

		ctx := r.Context()
		p, err := s.store.Get(ctx, id)
		if err != nil {
			w.Write([]byte(fmt.Sprintf(`<div class="grid-matrix">Lookup Error: %v</div>`, err)))
			return
		}

		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(renderTweakingWorkspaceHTML(p)))
	}
}

func (s *Server) handleChatPreset() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, "Bad form input", http.StatusBadRequest)
			return
		}

		id := r.FormValue("id")
		userMessage := r.FormValue("message")
		if id == "" || userMessage == "" {
			w.Write([]byte(`<div style="color:#ef4444;">Missing ID or message.</div>`))
			return
		}

		ctx := r.Context()
		p, err := s.store.Get(ctx, id)
		if err != nil {
			w.Write([]byte(fmt.Sprintf(`<div style="color:#ef4444;">Lookup Error: %v</div>`, err)))
			return
		}

		apiKey, err := s.smFetcher.GetPassword(ctx, "710019748844", "gsr-gemini-api-key")
		if err != nil {
			w.Write([]byte(fmt.Sprintf(`<div style="color:#ef4444;">Auth Error: %v</div>`, err)))
			return
		}

		orch, err := s.orchMaker(ctx, apiKey)
		if err != nil {
			w.Write([]byte(fmt.Sprintf(`<div style="color:#ef4444;">ADK Error: %v</div>`, err)))
			return
		}
		defer orch.Close()

		// Run the chat refinement loop
		jsonResponse, _, err := orch.RefineChat(ctx, p, userMessage)
		if err != nil {
			w.Write([]byte(fmt.Sprintf(`<div class="toast show" style="background:#ef4444;">Execution Error: %v</div>`, err)))
			return
		}

		// Strip markdown if exists
		jsonResponse = strings.TrimSpace(jsonResponse)
		jsonResponse = strings.TrimPrefix(jsonResponse, "```json")
		jsonResponse = strings.TrimPrefix(jsonResponse, "```")
		jsonResponse = strings.TrimSuffix(jsonResponse, "```")
		jsonResponse = strings.TrimSpace(jsonResponse)

		var archResp struct {
			ConversationalResponse string   `json:"conversational_response"`
			DSPMatrixUpdated       bool     `json:"dsp_matrix_updated"`
			FinalHTMLPayload       string   `json:"final_html_payload"`
			AgentImpact            []string `json:"agent_impact"`
		}

		if err := json.Unmarshal([]byte(jsonResponse), &archResp); err != nil {
			log.Printf("Failed to unmarshal chat architect: %v, raw: %s", err, jsonResponse)
			w.Write([]byte(fmt.Sprintf(`<div class="toast show" style="background:#ef4444;">Serialization Error! Check server logs.</div>`)))
			return
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

		var oobHtml string
		if archResp.DSPMatrixUpdated {
			p.Payload = archResp.FinalHTMLPayload
			// Build OOB HTML for Live Matrix
			oobHtml = fmt.Sprintf(`
			<div id="live-matrix-container" hx-swap-oob="true" style="zoom: 0.8; overflow-x: auto;">
				%s
			</div>
			`, p.Payload)
		}

		// Save the state
		s.store.Save(ctx, p)

		userBubble := fmt.Sprintf(`
		<div style="align-self: flex-end; background: var(--accent); color: white; padding: 0.75rem 1rem; border-radius: 12px 12px 0 12px; max-width: 80%%%%;">
			<b>You:</b><br>%s
		</div>`, html.EscapeString(userMessage))

		safeAssis := html.EscapeString(assistantContent)
		safeAssis = strings.ReplaceAll(safeAssis, "\n", "<br>")
		
		assistantBubble := fmt.Sprintf(`
		<div style="align-self: flex-start; background: #334155; color: white; padding: 0.75rem 1rem; border-radius: 12px 12px 12px 0; max-width: 80%%%%;">
			<b>Architect:</b><br>%s
		</div>`, safeAssis)

		finalDOM := userBubble + assistantBubble + oobHtml

		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(finalDOM))
	}
}
