package api

import (
	"fmt"
	"html"
	"log"
	"net/http"
	"strings"
)

// handleGetMemories returns HTML fragments for the Memory Rules tab
func (s *Server) handleGetMemories() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		memories, err := s.memoryStore.List(ctx)
		if err != nil {
			log.Printf("Failed to list memories: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`<p class="subtitle" style="font-size:0.9rem;">Failed to load rules.</p>`))
			return
		}

		if len(memories) == 0 {
			w.Write([]byte(`<p class="subtitle" style="font-size:0.9rem;">No learned rules yet.</p>`))
			return
		}

		var sb strings.Builder
		sb.WriteString(`<ul style="list-style-type: none; padding: 0;">`)
		for _, m := range memories {
			fmt.Fprintf(&sb, `
				<li style="margin-bottom: 1.5rem; border-bottom: 1px solid var(--border); padding-bottom: 1rem;">
					<div style="display: flex; justify-content: space-between; align-items: flex-start;">
						<div>
							<h3 style="margin: 0 0 0.5rem 0; font-size: 1.1rem; color: var(--text-main);">%[1]s</h3>
							<p style="margin: 0 0 0.25rem 0; font-size: 0.9rem; color: var(--text-sub);">Critique: %[2]s</p>
							<p style="margin: 0; font-size: 0.9rem; color: var(--success);">Action: %[3]s</p>
						</div>
						<button hx-delete="/api/memory/delete?id=%[4]s" hx-target="#sidebar-content" style="width: auto; padding: 0.4rem; font-size: 0.8rem; background: rgba(239, 68, 68, 0.1); color: #ef4444; border: 1px solid rgba(239, 68, 68, 0.2); border-radius: 4px; cursor: pointer;">Delete</button>
					</div>
				</li>`, html.EscapeString(m.Artist), html.EscapeString(m.Critique), html.EscapeString(m.Action), m.ID)
		}
		sb.WriteString(`</ul>`)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(sb.String()))
	}
}

// handleDeleteMemory deletes a rule and re-renders the list
func (s *Server) handleDeleteMemory() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		id := r.URL.Query().Get("id")
		if id == "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		if err := s.memoryStore.Delete(ctx, id); err != nil {
			log.Printf("Failed to delete memory %s: %v", id, err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		// Re-render the list to update UI via HTMX
		memories, err := s.memoryStore.List(ctx)
		if err != nil {
			log.Printf("Failed to list memories after delete: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		if len(memories) == 0 {
			w.Write([]byte(`<p class="subtitle" style="font-size:0.9rem;">No learned rules yet.</p>`))
			return
		}

		var sb strings.Builder
		sb.WriteString(`<ul style="list-style-type: none; padding: 0;">`)
		for _, m := range memories {
			fmt.Fprintf(&sb, `
				<li style="margin-bottom: 1.5rem; border-bottom: 1px solid var(--border); padding-bottom: 1rem;">
					<div style="display: flex; justify-content: space-between; align-items: flex-start;">
						<div>
							<h3 style="margin: 0 0 0.5rem 0; font-size: 1.1rem; color: var(--text-main);">%[1]s</h3>
							<p style="margin: 0 0 0.25rem 0; font-size: 0.9rem; color: var(--text-sub);">Critique: %[2]s</p>
							<p style="margin: 0; font-size: 0.9rem; color: var(--success);">Action: %[3]s</p>
						</div>
						<button hx-delete="/api/memory/delete?id=%[4]s" hx-target="#sidebar-content" style="width: auto; padding: 0.4rem; font-size: 0.8rem; background: rgba(239, 68, 68, 0.1); color: #ef4444; border: 1px solid rgba(239, 68, 68, 0.2); border-radius: 4px; cursor: pointer;">Delete</button>
					</div>
				</li>`, html.EscapeString(m.Artist), html.EscapeString(m.Critique), html.EscapeString(m.Action), m.ID)
		}
		sb.WriteString(`</ul>`)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(sb.String()))
	}
}
