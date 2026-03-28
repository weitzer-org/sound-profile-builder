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
					<button hx-get="/api/preset/copy_ui?id=%[3]s" hx-target="#main-workspace" style="flex: 1; padding: 0.5rem; font-size: 0.9rem; background: var(--bg-dark); border: 1px solid var(--border); border-radius: 8px; color: white; cursor: pointer; transition: background 0.2s;" onmouseover="this.style.background='var(--border)'" onmouseout="this.style.background='var(--bg-dark)'">Copy</button>
					<button id="delete-btn-%[3]s" hx-post="/api/preset/delete" hx-vals='{"id":"%[3]s"}' hx-trigger="confirmed" hx-target="#preset-list-container" onclick="if(this.dataset.confirmed) { htmx.trigger(this, 'confirmed'); } else { this.dataset.confirmed = 'true'; this.innerText = 'Confirm?'; this.style.background = '#7f1d1d'; setTimeout(() => { this.dataset.confirmed = ''; this.innerText = 'Delete'; this.style.background = '#ef4444'; }, 3000); }" style="flex: 1; padding: 0.5rem; font-size: 0.9rem; background: #ef4444; border: 1px solid #b91c1c; border-radius: 8px; color: white; cursor: pointer; transition: background 0.2s;">Delete</button>
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

func (s *Server) handleCopyPresetUI() http.HandlerFunc {
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
		w.Write([]byte(renderTweakingWorkspaceHTML(p, true)))
	}
}

func (s *Server) handleCopyPreset() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, "Bad form config", http.StatusBadRequest)
			return
		}

		id := r.FormValue("id")
		ctx := r.Context()
		p, err := s.store.Get(ctx, id)
		if err != nil {
			http.Error(w, "Preset not found", http.StatusNotFound)
			return
		}

		newName := strings.TrimSpace(r.FormValue("new_name"))
		if newName == "" {
			newName = p.Name + " (Copy)"
		}

		// Save an exact replica
		pCopy := &storage.Preset{
			Name:             newName,
			Payload:          p.Payload,
			BuilderStatement: p.BuilderStatement,
		}
		
		if len(p.ChatHistory) > 0 {
			pCopy.ChatHistory = append([]storage.ChatMessage{}, p.ChatHistory...)
		}
		
		if err := s.store.Save(ctx, pCopy); err != nil {
			http.Error(w, "Failed to duplicate", http.StatusInternalServerError)
			return
		}

		// Return the cloned workspace and command sidebar to refresh
		w.Header().Set("Content-Type", "text/html")
		w.Header().Set("HX-Trigger", "presetCopied")
		w.Write([]byte(renderTweakingWorkspaceHTML(pCopy, false)))
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
		`, renderPresetList(presets), name, renderTweakingWorkspaceHTML(p, false))
		
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(oobResponse))
	}
}

// renderTweakingWorkspaceHTML constructs the Side-by-Side editing view for a Preset
func renderTweakingWorkspaceHTML(p *storage.Preset, isCopyMode bool) string {
	// TODO: Make the conversational response from the agent more visible in the UI. 
	// Currently, it gets hidden inside the "View ADK Processing Log" accordion. We should explore
	// showing the latest message prominently, especially if the matrix wasn't updated.
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
				<form hx-post="/api/preset/rename" hx-target="#main-workspace" style="display:flex; gap:0.5rem; margin:0; flex: 1; align-items: center;" autocomplete="off">
					<input type="hidden" name="id" value="%[1]s">
					<input type="text" name="preset_name" placeholder="Enter custom name..." required style="flex: 1; min-width: 300px; padding: 0.75rem 1rem; background: rgba(15,23,42,0.8); border: 1px solid var(--accent); border-radius: 8px; font-size: 1.25rem; color: white; font-weight: 500; outline: none; transition: border-color 0.2s, box-shadow 0.2s;" onfocus="this.style.boxShadow='0 0 0 2px rgba(99,102,241,0.5)'" onblur="this.style.boxShadow='none'">
					<button type="submit" style="width: auto; padding: 0.75rem 1.5rem; font-size: 1rem; font-weight: 600; background: var(--success); border-radius: 8px; border: none; color: white; cursor: pointer; transition: opacity 0.2s;" onhover="this.style.opacity='0.9'">Finalize Save</button>
				</form>
				<button id="discard-btn-%[1]s" hx-post="/api/preset/delete_draft" hx-vals='{"id":"%[1]s"}' hx-trigger="confirmed" hx-target="body" onclick="if(this.dataset.confirmed) { htmx.trigger(this, 'confirmed'); } else { this.dataset.confirmed = 'true'; this.innerText = 'Confirm Discard?'; this.style.background = '#7f1d1d'; setTimeout(() => { this.dataset.confirmed = ''; this.innerText = 'Discard / Exit'; this.style.background = '#ef4444'; }, 3000); }" style="width: auto; padding: 0.75rem 1.5rem; font-size: 1rem; font-weight: 600; background: #ef4444; border: 1px solid #b91c1c; border-radius: 8px; color: white; cursor: pointer; transition: background 0.2s;">Discard / Exit</button>
			</div>
		`, p.ID)
	} else {
		headerHtml = fmt.Sprintf(`
			<div style="display: flex; justify-content: space-between; align-items: center; width: 100%%; gap: 1rem;">
				<div style="flex: 1; display: flex; align-items: center;">
					<h2 id="preset-title-%[2]s" style="font-size: 1.5rem; font-weight: 600; margin: 0; color: white; display: flex; align-items: center; gap: 1rem;">
						%[1]s
					</h2>
					<form id="rename-form-%[2]s" hx-post="/api/preset/rename" hx-target="#main-workspace" style="display: none; gap: 0.5rem; flex: 1; margin: 0; align-items: center;" autocomplete="off">
						<input type="hidden" name="id" value="%[2]s">
						<input type="text" name="preset_name" placeholder="Rename..." required style="flex: 1; min-width: 300px; padding: 0.5rem 1rem; font-size: 1.25rem; background: rgba(0,0,0,0.4); border: 1px solid var(--accent); border-radius: 8px; color: white; font-weight: 500; outline: none; transition: box-shadow 0.2s;" onfocus="this.style.boxShadow='0 0 0 2px rgba(99,102,241,0.5)'" onblur="this.style.boxShadow='none'">
						<button type="submit" style="padding: 0.5rem 1rem; font-size: 1rem; font-weight: 600; background: var(--success); border: none; border-radius: 8px; color: white; cursor: pointer;">Save</button>
						<button type="button" onclick="document.getElementById('rename-form-%[2]s').style.display='none'; document.getElementById('title-actions-%[2]s').style.display='flex';" style="padding: 0.5rem 1rem; font-size: 1rem; font-weight: 600; background: var(--bg-dark); border: 1px solid var(--border); border-radius: 8px; color: white; cursor: pointer;">Cancel</button>
					</form>
				</div>
				<div id="title-actions-%[2]s" style="display: flex; gap: 0.5rem; align-items: center;">
					<button type="button" onclick="document.getElementById('rename-form-%[2]s').style.display='flex'; this.parentElement.style.display='none'; document.querySelector('#rename-form-%[2]s input[name=preset_name]').value = '%[1]s';" style="width: auto; padding: 0.5rem 1.25rem; font-size: 1rem; font-weight: 500; background: var(--accent); border: none; border-radius: 8px; color: white; cursor: pointer; transition: opacity 0.2s;" onmouseover="this.style.opacity='0.9'" onmouseout="this.style.opacity='1'">Rename</button>
					<button onclick="window.location.reload()" style="width: auto; padding: 0.5rem 1.25rem; font-size: 1rem; font-weight: 500; background: var(--bg-dark); border: 1px solid var(--border); border-radius: 8px; color: white; cursor: pointer; transition: background 0.2s;" onmouseover="this.style.background='var(--border)'" onmouseout="this.style.background='var(--bg-dark)'">Back / Exit</button>
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

	refinementSummaryHtml := ""
	if len(p.ChatHistory) > 0 {
		lastMsg := p.ChatHistory[len(p.ChatHistory)-1]
		if lastMsg.Role == "architect" {
			summaryText := html.EscapeString(lastMsg.Content)
			summaryText = strings.ReplaceAll(summaryText, "\n", "<br>")
			summaryText = strings.ReplaceAll(summaryText, "**Impact:**", "<strong style='display:inline-block; margin-top: 0.5rem;'>Impact:</strong>")
			refinementSummaryHtml = fmt.Sprintf(`
				<div style="padding: 1rem; background: rgba(16, 185, 129, 0.1); border-left: 4px solid var(--success); border-radius: 0 8px 8px 0; font-size: 0.95rem; color: #e2e8f0; line-height: 1.5;">
					<strong style="color: var(--success); display: block; margin-bottom: 0.5rem;">Latest Refinement Result:</strong>
					%s
				</div>
			`, summaryText)
		}
	}

	controlPanelHtml := ""
	if isCopyMode {
		controlPanelHtml = fmt.Sprintf(`
		<div class="card" style="padding: 1.5rem; margin-bottom: 1.5rem; border-radius: 12px; display: flex; flex-direction: column; gap: 1rem; border: 2px solid var(--accent);">
			<h3 style="margin: 0; font-size: 1.25rem; color: var(--text-main);">Duplicate Preset</h3>
			<form hx-post="/api/preset/copy" hx-target="#workspace-wrapper" hx-swap="outerHTML" style="display: flex; gap: 0.75rem; align-items: flex-start;" autocomplete="off">
				<input type="hidden" name="id" value="%[1]s">
				<div style="flex: 1; display: flex; flex-direction: column; gap: 0.5rem;">
					<input type="text" name="new_name" placeholder="Enter name for the duplicate..." required style="flex: 1; padding: 0.85rem 1rem; border-radius: 8px; background: rgba(15,23,42,0.5); color: white; border: 1px solid rgba(255,255,255,0.2); font-family: inherit; font-size: 1.25rem; font-weight: 500; outline: none; transition: box-shadow 0.2s;" onfocus="this.style.boxShadow='0 0 0 2px rgba(99,102,241,0.5)'" onblur="this.style.boxShadow='none'">
					<div style="font-size: 0.85rem; color: rgba(255,255,255,0.6);">
						Creates an exact replica of this routing matrix and chat history.
					</div>
				</div>
				<button type="submit" style="width: auto; height: 50px; padding: 0 1.5rem; border-radius: 8px; font-weight: 600; font-size: 1rem; background: var(--success); border: none; color: white; cursor: pointer;">
					Confirm Duplicate
				</button>
			</form>
		</div>
		`, p.ID)
	} else {
		controlPanelHtml = fmt.Sprintf(`
		<div class="card" style="padding: 1.5rem; margin-bottom: 1.5rem; border-radius: 12px; display: flex; flex-direction: column; gap: 1rem;">
			<h3 style="margin: 0; font-size: 1.1rem; color: var(--text-main);">Adjust Preset Instructions</h3>
			%s
			<!-- TODO: Display the initial generation prompt (p.Prompt) somewhere in this area to provide context on what was originally requested -->
			<form hx-post="/api/preset/chat" hx-target="#workspace-wrapper" hx-swap="outerHTML" style="display: flex; gap: 0.75rem; align-items: flex-end;" autocomplete="off" hx-sync="this:drop" hx-disabled-elt="this, #chat-input, button[type='submit']">
				<input type="hidden" name="id" value="%s">
				<div style="flex: 1; display: flex; flex-direction: column; gap: 0.5rem;">
					<textarea name="message" id="chat-input" placeholder="e.g., Make the amp darker..." style="resize: none; overflow-y: hidden; min-height: 48px; padding: 0.85rem 1rem; border-radius: 8px; background: rgba(15,23,42,0.5); color: white; border: 1px solid rgba(255,255,255,0.2); font-family: inherit; font-size: 0.95rem; line-height: 1.4;" rows="1" oninput="this.style.height = ''; this.style.height = this.scrollHeight + 'px'" onkeydown="if(event.key === 'Enter' && !event.shiftKey) { event.preventDefault(); this.form.dispatchEvent(new Event('submit', {cancelable: true, bubbles: true})); }" required></textarea>
					<div style="font-size: 0.85rem; color: rgba(255,255,255,0.8); line-height: 1.4; padding-top: 0.25rem;">
						<span style="color: var(--accent); font-weight: 500;">Builder Statement:</span> %s
					</div>
				</div>
				<!-- TODO: Add a button to trigger a full 12-agent "re-run" of the pipeline, passing in the current preset state and chat history as context to completely overhaul the tone from scratch rather than just tweaking it. -->
				<!-- TODO: Implement a visual pulsing border on the DSP Matrix container when a refinement is in-progress, and a green success flash when complete to make the system state more obvious to the user. -->
				<button id="chat-submit-btn" type="submit" style="width: auto; height: 48px; padding: 0 1.25rem; border-radius: 8px;">
					<span class="spinner"></span>
					<span class="btn-text">Adjust</span>
				</button>
			</form>
		</div>
		`, refinementSummaryHtml, p.ID, html.EscapeString(p.BuilderStatement))
	}

	return fmt.Sprintf(`
	<div id="workspace-wrapper" class="workspace-wrapper">
		<div class="card" style="padding: 1rem 1.5rem; margin-bottom: 1.5rem; border-radius: 12px;">
			%s
		</div>
		
		%s

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
	`, headerHtml, controlPanelHtml, matrixContainerHtml, historyHtml)
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
		w.Write([]byte(renderTweakingWorkspaceHTML(p, false)))
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

		if r.FormValue("mock") == "true" || os.Getenv("MOCK_MODE") == "true" {
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
			BuilderStatement       string            `json:"builder_statement"`
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

		if archResp.DSPMatrixUpdated && archResp.BuilderStatement != "" {
			p.BuilderStatement = archResp.BuilderStatement
		}

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

		finalDOM := renderTweakingWorkspaceHTML(p, false)

		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(finalDOM))
	}
}
