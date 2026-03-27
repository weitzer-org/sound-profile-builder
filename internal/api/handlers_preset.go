package api

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"log"
	"net/http"
	"strings"

	"github.com/weitzer-org/sound-builder/internal/agents"
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
				<h3 style="margin: 0 0 0.5rem 0; font-size: 1.1rem;">%[1]s</h3>
				<span style="font-size: 0.8rem; color: var(--text-sub);">Saved: %[2]s</span>
				<div style="margin-top: 0.5rem; display: flex; gap: 0.5rem;">
					<div id="copy-container-%[3]s" style="flex: 1; display: flex;">
						<button type="button" onclick="document.getElementById('copy-form-%[3]s').style.display='flex'; this.style.display='none'; document.getElementById('delete-btn-%[3]s').style.display='none';" style="width: 100%%; padding: 0.5rem; font-size: 0.9rem; background: var(--bg-dark); border: 1px solid var(--border); color: white; cursor: pointer;">Copy</button>
						<form id="copy-form-%[3]s" hx-post="/api/preset/copy" hx-target="#preset-list-container" style="display: none; width: 100%%; gap: 0.25rem; align-items: stretch; margin: 0;" autocomplete="off">
							<input type="hidden" name="id" value="%[3]s">
							<input type="text" name="new_name" placeholder="Copy Name..." required style="flex: 1; min-width: 0; padding: 0.3rem; font-size: 0.8rem; background: rgba(0,0,0,0.2); border: 1px solid var(--border); color: white; box-sizing: border-box;">
							<button type="submit" style="padding: 0 0.5rem; font-size: 0.8rem; background: var(--success); border: none; color: white; cursor: pointer;">✔</button>
							<button type="button" onclick="document.getElementById('copy-form-%[3]s').style.display='none'; document.getElementById('copy-form-%[3]s').previousElementSibling.style.display='block'; document.getElementById('delete-btn-%[3]s').style.display='block';" style="padding: 0 0.5rem; font-size: 0.8rem; background: var(--bg-dark); border: 1px solid var(--border); color: white; cursor: pointer;">✖</button>
						</form>
					</div>
					<button id="delete-btn-%[3]s" hx-post="/api/preset/delete" hx-vals='{"id":"%[3]s"}' hx-trigger="confirmed" hx-target="#preset-list-container" onclick="if(this.dataset.confirmed) { htmx.trigger(this, 'confirmed'); } else { this.dataset.confirmed = 'true'; this.innerText = 'Confirm?'; this.style.background = '#7f1d1d'; setTimeout(() => { this.dataset.confirmed = ''; this.innerText = 'Delete'; this.style.background = '#ef4444'; }, 3000); }" style="flex: 1; padding: 0.5rem; font-size: 0.9rem; background: #ef4444; border: 1px solid #b91c1c; color: white; cursor: pointer; transition: background 0.2s;">Delete</button>
				</div>
				<div style="margin-top: 0.5rem;">
					<button hx-get="/api/preset/view?id=%[3]s" hx-target="#main-workspace" style="width: 100%%; padding: 0.5rem; font-size: 0.9rem; background: var(--success); color: white; border: none; cursor: pointer;">Adjust preset</button>
				</div>
			</li>`, p.Name, p.UpdatedAt, p.ID)
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

		newName := r.FormValue("new_name")
		if newName == "" {
			newName = r.Header.Get("HX-Prompt")
		}
		if newName == "" {
			newName = p.Name + " (Copy)"
		}

		// Save a stripped copy
		pCopy := &storage.Preset{
			Name:    newName,
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
		if name == "" {
			name = r.Header.Get("HX-Prompt")
		}
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
	// Render chat history logs as a collapsible accordion
	historyHtml := ""
	if len(p.ChatHistory) > 0 {
		var logItems string
		for _, msg := range p.ChatHistory {
			if msg.Role == "model" {
				logItems += fmt.Sprintf(`<div style="margin-bottom: 1rem; padding-bottom: 1rem; border-bottom: 1px solid rgba(255,255,255,0.1);"><b>Agent process:</b><br>%s</div>`, strings.ReplaceAll(msg.Content, "\n", "<br>"))
			} else {
				logItems += fmt.Sprintf(`<div style="margin-bottom: 1rem; padding-bottom: 1rem; border-bottom: 1px solid rgba(255,255,255,0.1); color: var(--accent);"><b>You requested:</b><br>%s</div>`, html.EscapeString(msg.Content))
			}
		}
		
		historyHtml = fmt.Sprintf(`
		<details style="margin-top: 1.5rem; background: rgba(30, 41, 59, 0.5); border: 1px solid var(--border); border-radius: 8px;">
			<summary style="padding: 1rem; cursor: pointer; font-weight: 500; outline: none; user-select: none;">View ADK Processing Log</summary>
			<div style="padding: 1rem; border-top: 1px solid var(--border); font-size: 0.9rem; color: var(--text-sub); max-height: 400px; overflow-y: auto;">
				%s
			</div>
		</details>
		`, logItems)
	}

	headerHtml := ""
	if p.Name == "Draft Preset" {
		headerHtml = fmt.Sprintf(`
			<div style="display: flex; gap: 0.5rem; align-items: center; width: 100%%; justify-content: space-between;">
				<form hx-post="/api/preset/rename" hx-target="#main-workspace" style="display:flex; gap:0.5rem; margin:0; flex: 1;" autocomplete="off">
					<input type="hidden" name="id" value="%s">
					<input type="text" name="preset_name" placeholder="Enter custom name..." required style="padding: 0.5rem 1rem; background: rgba(15,23,42,0.5); border: 1px solid rgba(255,255,255,0.2); border-radius: 8px; font-size: 1rem; width: 300px;">
					<button type="submit" style="width: auto; padding: 0.5rem 1rem; font-size: 0.9rem; background: var(--success); border-radius: 8px;">Finalize Save</button>
				</form>
				<button hx-post="/api/preset/delete_draft" hx-confirm="Are you sure you want to discard this draft?" hx-vals='{"id":"%s"}' hx-target="body" style="width: auto; padding: 0.5rem 1rem; font-size: 0.9rem; background: #ef4444; border: 1px solid #b91c1c; border-radius: 8px;">Discard / Exit</button>
			</div>
		`, p.ID, p.ID)
	} else {
		headerHtml = fmt.Sprintf(`
			<div style="display: flex; justify-content: space-between; align-items: center; width: 100%%;">
				<h2 style="font-size: 1.25rem; margin: 0; color: var(--text-sub);">Preset: <span style="color:white;">%[1]s</span></h2>
				<div style="display: flex; gap: 0.5rem;">
					<div id="rename-container-%[2]s" style="display: flex;">
						<button type="button" onclick="document.getElementById('rename-form-%[2]s').style.display='flex'; this.style.display='none';" style="width: auto; padding: 0.5rem 1rem; font-size: 0.9rem; background: var(--accent); border: 1px solid var(--border); border-radius: 8px; color: white; cursor: pointer;">Rename</button>
						<form id="rename-form-%[2]s" hx-post="/api/preset/rename" hx-target="#main-workspace" style="display: none; gap: 0.5rem; flex: 1; margin: 0;" autocomplete="off">
							<input type="hidden" name="id" value="%[2]s">
							<input type="text" name="preset_name" placeholder="Rename..." required style="width: 300px; padding: 0.5rem; font-size: 0.9rem; background: rgba(0,0,0,0.2); border: 1px solid var(--border); border-radius: 8px; color: white; box-sizing: border-box;">
							<button type="submit" style="padding: 0 0.75rem; font-size: 0.9rem; background: var(--success); border: none; border-radius: 8px; color: white; cursor: pointer;">Save</button>
							<button type="button" onclick="document.getElementById('rename-form-%[2]s').style.display='none'; document.getElementById('rename-form-%[2]s').previousElementSibling.style.display='block';" style="padding: 0 0.75rem; font-size: 0.9rem; background: var(--bg-dark); border: 1px solid var(--border); border-radius: 8px; color: white; cursor: pointer;">Cancel</button>
						</form>
					</div>
					<button onclick="window.location.reload()" style="width: auto; padding: 0.5rem 1rem; font-size: 0.9rem; background: var(--bg-dark); border: 1px solid var(--border); border-radius: 8px; color: white; cursor: pointer;">Back / Exit</button>
				</div>
			</div>
		`, html.EscapeString(p.Name), p.ID)
	}

	// Parse payload into map of guitar variations
	var matrices map[string]string
	if err := json.Unmarshal([]byte(p.Payload), &matrices); err != nil {
		// Fallback for older presets that are just plain HTML strings
		matrices = map[string]string{"Legacy Format": p.Payload}
	}

	tabsHtml := `<div class="tabs-header" style="display: flex; gap: 0.5rem; margin-bottom: 1rem; overflow-x: auto; padding-bottom: 0.5rem;">`
	contentHtml := `<div class="tabs-content">`
	
	first := true
	for guitarName, matrixHTML := range matrices {
		activeClass := ""
		displayStyle := "display: none;"
		if first {
			activeClass = "active"
			displayStyle = "display: block;"
			first = false
		}
		
		safeId := strings.ReplaceAll(strings.ToLower(guitarName), " ", "-")
		safeId = strings.ReplaceAll(safeId, "/", "")

		tabsHtml += fmt.Sprintf(`
			<button class="tab-btn %s" onclick="switchTab(this, 'tab-%s')" style="white-space: nowrap; padding: 0.5rem 1rem; background: var(--bg-dark); border: 1px solid var(--border); border-radius: 8px; color: var(--text-sub); cursor: pointer;">%s</button>
		`, activeClass, safeId, html.EscapeString(guitarName))
		
		contentHtml += fmt.Sprintf(`
			<div id="tab-%s" class="tab-pane" style="%s">
				%s
			</div>
		`, safeId, displayStyle, matrixHTML)
	}
	
	tabsHtml += `</div>`
	contentHtml += `</div>`

	tabScript := `
		<script>
			function switchTab(btn, paneId) {
				const container = btn.closest('.card');
				container.querySelectorAll('.tab-btn').forEach(b => {
					b.classList.remove('active');
					b.style.color = 'var(--text-sub)';
					b.style.borderColor = 'var(--border)';
				});
				container.querySelectorAll('.tab-pane').forEach(p => p.style.display = 'none');
				
				btn.classList.add('active');
				btn.style.color = 'var(--text-main)';
				btn.style.borderColor = 'var(--accent)';
				container.querySelector('#' + paneId).style.display = 'block';
			}
			// Init active tab
			document.querySelectorAll('.tab-btn.active').forEach(b => {
				b.style.color = 'var(--text-main)';
				b.style.borderColor = 'var(--accent)';
			});
		</script>
	`
	matrixContainerHtml := tabsHtml + contentHtml + tabScript

	return fmt.Sprintf(`
	<div id="workspace-wrapper" class="workspace-wrapper">
		<div class="card" style="padding: 1rem 1.5rem; margin-bottom: 1.5rem; border-radius: 12px;">
			%s
		</div>
		
		<div class="card" style="padding: 1.5rem; margin-bottom: 1.5rem; border-radius: 12px; display: flex; flex-direction: column; gap: 1rem;">
			<h3 style="margin: 0; font-size: 1.1rem; color: var(--text-main);">Adjust Preset Instructions</h3>
			<!-- TODO: Display the initial generation prompt (p.Prompt) somewhere in this area to provide context on what was originally requested -->
			<form hx-post="/api/preset/chat" hx-target="#workspace-wrapper" hx-swap="outerHTML" style="display: flex; gap: 0.75rem; align-items: flex-end;" autocomplete="off" hx-sync="this:drop" hx-disabled-elt="this, #chat-input, button[type='submit']">
				<input type="hidden" name="id" value="%s">
				<div style="flex: 1; display: flex; flex-direction: column; gap: 0.5rem;">
					<textarea name="message" id="chat-input" placeholder="e.g., Make the amp darker..." style="resize: none; overflow-y: hidden; min-height: 48px; padding: 0.85rem 1rem; border-radius: 8px; background: rgba(15,23,42,0.5); color: white; border: 1px solid rgba(255,255,255,0.2); font-family: inherit; font-size: 0.95rem; line-height: 1.4;" rows="1" oninput="this.style.height = ''; this.style.height = this.scrollHeight + 'px'" onkeydown="if(event.key === 'Enter' && !event.shiftKey) { event.preventDefault(); this.form.dispatchEvent(new Event('submit', {cancelable: true, bubbles: true})); }" required></textarea>
					<div style="font-size: 0.8rem; color: rgba(255,255,255,0.5);">
						<span style="color: var(--accent);">Refinement Mode:</span> Bypasses the 11-agent scraping pipeline. A single <b>Architect Subagent</b> directly ingests your feedback and rewrites the live Matrix in ~15s.
					</div>
				</div>
				<button id="chat-submit-btn" type="submit" style="width: auto; height: 48px; padding: 0 1.25rem; border-radius: 8px;">
					<span class="spinner"></span>
					<span class="btn-text">Adjust</span>
				</button>
			</form>
		</div>

		<div class="tweaking-workspace" style="display: flex; flex-direction: column;">
			<div class="card" style="padding: 1.5rem; margin-bottom: 0; border-radius: 12px;">
				<h2 style="font-size: 1.25rem; margin-top: 0; margin-bottom: 1rem;">Live DSP Matrix</h2>
				<!-- TODO: Parse the matrix HTML or instruct the LLM to emit badges next to each effect indicating whether it is a native algorithm, 1P Capture, or 3P Capture. -->
				<div id="live-matrix-container" style="zoom: 0.8;">
					%s
				</div>
				%s
			</div>
		</div>
	</div>
	`, headerHtml, p.ID, matrixContainerHtml, historyHtml)
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

		ctx := context.WithoutCancel(r.Context())
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

		if r.FormValue("mock") == "true" {
			ctx = context.WithValue(ctx, agents.MockModeKey, true)
		}

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
			ConversationalResponse string            `json:"conversational_response"`
			DSPMatrixUpdated       bool              `json:"dsp_matrix_updated"`
			FinalHTMLPayload       map[string]string `json:"final_html_payload"`
			AgentImpact            []string          `json:"agent_impact"`
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

		if archResp.DSPMatrixUpdated {
			payloadBytes, err := json.Marshal(archResp.FinalHTMLPayload)
			if err != nil {
				log.Printf("Failed to marshal final html payload map: %v", err)
			} else {
				p.Payload = string(payloadBytes)
			}
		}

		// Save the state
		s.store.Save(ctx, p)

		finalDOM := renderTweakingWorkspaceHTML(p)

		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(finalDOM))
	}
}
