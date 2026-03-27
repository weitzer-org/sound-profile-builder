package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/weitzer-org/sound-builder/internal/agents"
	"github.com/weitzer-org/sound-builder/internal/storage"
)

// bucketName is the default bucket used for everything right now
const presetBucketName = "weitzer-sound-builder"

// getPresetStore initializes an ephemeral Store connection
func getPresetStore(r *http.Request) (*storage.PresetStore, *storage.GCSClient, error) {
	ctx := r.Context()
	gcs, err := storage.NewGCSClient(ctx)
	if err != nil {
		return nil, nil, err
	}
	store := storage.NewPresetStore(gcs, presetBucketName)
	return store, gcs, nil
}

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
					<form hx-post="/api/preset/edit" hx-target="#result" hx-indicator="#submit-btn" style="display:flex; gap: 0.5rem;">
						<input type="hidden" name="id" value="%s">
						<input type="text" name="feedback" placeholder="Instruct LLM tweak..." style="flex:2; padding: 0.5rem; font-size: 0.9rem;">
						<button type="submit" style="flex:1; padding: 0.5rem; font-size: 0.9rem; background: var(--success);"><small>Refine</small></button>
					</form>
				</div>
			</li>`, p.Name, p.UpdatedAt, p.ID, p.ID, p.ID)
	}
	html += `</ul>`
	return html
}

func (s *Server) handleGetPresets() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		store, gcs, err := getPresetStore(r)
		if err != nil {
			http.Error(w, "Failed to connect to storage", http.StatusInternalServerError)
			return
		}
		defer gcs.Close()

		presets, err := store.List(r.Context())
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

		name := r.FormValue("name")
		payload := r.FormValue("payload")
		if name == "" {
			name = "Unnamed Preset"
		}

		store, gcs, err := getPresetStore(r)
		if err != nil {
			http.Error(w, "Storage conn error", http.StatusInternalServerError)
			return
		}
		defer gcs.Close()

		preset := &storage.Preset{
			Name:    name,
			Payload: payload,
		}

		if err := store.Save(r.Context(), preset); err != nil {
			log.Printf("Failed saving preset: %v", err)
			http.Error(w, "Failed saving preset", http.StatusInternalServerError)
			return
		}

		// Reload the list
		presets, _ := store.List(r.Context())
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(renderPresetList(presets)))
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

		store, gcs, err := getPresetStore(r)
		if err != nil {
			http.Error(w, "Storage conn error", http.StatusInternalServerError)
			return
		}
		defer gcs.Close()

		if err := store.Delete(r.Context(), id); err != nil {
			log.Printf("Failed deleting preset: %v", err)
			http.Error(w, "Failed deleting preset", http.StatusInternalServerError)
			return
		}

		// Reload the list
		presets, _ := store.List(r.Context())
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
		store, gcs, err := getPresetStore(r)
		if err != nil {
			http.Error(w, "Storage conn error", http.StatusInternalServerError)
			return
		}
		defer gcs.Close()

		p, err := store.Get(r.Context(), id)
		if err != nil {
			http.Error(w, "Preset not found", http.StatusNotFound)
			return
		}

		// Save a stripped copy
		pCopy := &storage.Preset{
			Name:    p.Name + " (Copy)",
			Payload: p.Payload,
		}
		
		if err := store.Save(r.Context(), pCopy); err != nil {
			http.Error(w, "Failed to duplicate", http.StatusInternalServerError)
			return
		}

		// Reload the list
		presets, _ := store.List(r.Context())
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(renderPresetList(presets)))
	}
}

func (s *Server) handleEditPreset() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, "Bad form input", http.StatusBadRequest)
			return
		}

		id := r.FormValue("id")
		feedback := r.FormValue("feedback")
		if id == "" || feedback == "" {
			w.Write([]byte(`<div class="grid-matrix" style="color:#ef4444;">Missing ID or feedback string.</div>`))
			return
		}

		ctx := r.Context()
		store, gcs, err := getPresetStore(r)
		if err != nil {
			w.Write([]byte(fmt.Sprintf(`<div class="grid-matrix">Storage Error: %v</div>`, err)))
			return
		}
		defer gcs.Close()

		p, err := store.Get(ctx, id)
		if err != nil {
			w.Write([]byte(fmt.Sprintf(`<div class="grid-matrix">Lookup Error: %v</div>`, err)))
			return
		}

		smClient, err := storage.NewSecretManagerClient(ctx)
		if err != nil {
			w.Write([]byte(fmt.Sprintf(`<div class="grid-matrix">Secrets Error: %v</div>`, err)))
			return
		}
		defer smClient.Close()

		apiKey, err := smClient.GetPassword(ctx, "710019748844", "gsr-gemini-api-key")
		if err != nil {
			w.Write([]byte(fmt.Sprintf(`<div class="grid-matrix">API Key Auth Error: %v</div>`, err)))
			return
		}

		orch, err := agents.NewOrchestrator(ctx, apiKey)
		if err != nil {
			w.Write([]byte(fmt.Sprintf(`<div class="grid-matrix">ADK Error: %v</div>`, err)))
			return
		}
		defer orch.Close()

		// Run the focused feedback refinement loop 
		// We will define this in orchestrator.go next
		w.Header().Set("Content-Type", "text/html")
		htmlPayload, tokenUsage, err := orch.RefinePreset(ctx, p.Payload, feedback)
		if err != nil {
			w.Write([]byte(fmt.Sprintf(`<div class="grid-matrix" style="color: #ef4444; border-color: #ef4444;">Execution Error: %v</div>`, err)))
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
			w.Write([]byte(fmt.Sprintf(`<div class="grid-matrix" style="color: #ef4444; border-color: #ef4444;">Serialization Error: %v</div>`, err)))
			return
		}

		// We save the updated preset payload dynamically!
		p.Payload = archResp.FinalHtmlPayload
		// Save the edit!
		store.Save(ctx, p)

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
				<strong>Refinement Processing Tokens:</strong> Input: %d | Output: %d | Models: %s
			</div>`, tokenUsage.InputTokens, tokenUsage.OutputTokens, strings.TrimSuffix(modelsList, ", "))
		}

		finalDOM := fmt.Sprintf(`
			<div class="card">
				%s
			</div>
			<div class="card">
				<h2>Feedback Architect Log (Refined)</h2>
				%s
			</div>
			%s
			<div style="margin-top: 1rem; color: var(--success); text-align:center;">
				<strong>Edit Applied and Saved!</strong> 
				<button hx-get="/api/presets" hx-target="#preset-list-container" style="padding: 0.5rem; margin-left: 1rem; background: var(--bg-card); color:var(--text-sub); border: 1px solid var(--border);">Refresh Presets List</button>
			</div>
		`, archResp.FinalHtmlPayload, impactsHtml, tokenStatsHtml)

		w.Write([]byte(finalDOM))
	}
}
