package api

import (
	"context"
	"errors"
	"fmt"
	"html"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"

	"github.com/weitzer-org/sound-builder/internal/agents"
	"github.com/weitzer-org/sound-builder/internal/config"
	"github.com/weitzer-org/sound-builder/internal/presetsvc"
	"github.com/weitzer-org/sound-builder/internal/runs"
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
	// svc owns generation/refinement runs, the concurrency cap and the run log.
	svc *presetsvc.Service
	// tasks is a render-side override consulted by handleTaskStatus before it
	// asks svc: an entry here wins. Nothing in production writes to it any more
	// (runs live in svc); it remains because existing tests drive
	// handleTaskStatus through it.
	tasks   map[string]*TaskState
	tasksMu sync.RWMutex
	// rateLimiters holds one token bucket per client IP, shared by every
	// rateLimitMiddleware-wrapped route (see ratelimit.go). Swept
	// periodically (rateLimiterSweepOnce) so it doesn't grow unbounded.
	rateLimiters         map[string]*rateLimiterEntry
	rateLimitersMu       sync.Mutex
	rateLimiterSweepOnce sync.Once
}

// NewServer initializes a new Server and its routes.
func NewServer(store *storage.PresetStore, memoryStore *storage.MemoryStore, client storage.Client, smFetcher storage.SecretFetcher, orchMaker func(ctx context.Context, apiKey string) (agents.OrchestratorService, error), appConfig *config.AppConfig) *Server {
	s := &Server{
		mux:          http.NewServeMux(),
		store:        store,
		memoryStore:  memoryStore,
		client:       client,
		smFetcher:    smFetcher,
		orchMaker:    orchMaker,
		appConfig:    appConfig,
		tasks:        make(map[string]*TaskState),
		rateLimiters: make(map[string]*rateLimiterEntry),
	}
	bucket := ""
	if store != nil {
		bucket = store.Bucket()
	}
	// The dependencies are read through closures, not copied, so a later
	// reassignment of s.smFetcher / s.orchMaker / s.appConfig (tests do this)
	// is seen by the service.
	s.svc = presetsvc.New(presetsvc.Deps{
		Store:    store,
		Memories: memoryStore,
		Client:   client,
		Bucket:   bucket,
		Secrets:  func() storage.SecretFetcher { return s.smFetcher },
		OrchMaker: func(ctx context.Context, apiKey string) (agents.OrchestratorService, error) {
			return s.orchMaker(ctx, apiKey)
		},
		Config: func() *config.AppConfig { return s.appConfig },
	})
	s.routes()
	return s
}

// Service returns the preset service backing the generate/refine handlers.
func (s *Server) Service() *presetsvc.Service { return s.svc }

// Handler returns the server's root handler, for callers that manage their own
// http.Server (graceful shutdown).
func (s *Server) Handler() http.Handler { return s.mux }

// routes registers all the HTTP handlers for the application.
func (s *Server) routes() {
	// PWA Manifest and Service Worker (Public endpoints)
	s.mux.HandleFunc("/manifest.json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		http.ServeFile(w, r, "web/static/manifest.json")
	})
	s.mux.HandleFunc("/sw.js", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/javascript")
		http.ServeFile(w, r, "web/static/sw.js")
	})
	// Serve /static/ assets (icons, images)
	fs := http.FileServer(http.Dir("web/static"))
	s.mux.Handle("/static/", http.StripPrefix("/static/", fs))

	// Public Routes
	s.mux.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			s.handleServeLogin()(w, r)
		} else if r.Method == http.MethodPost {
			s.handleProcessLogin()(w, r)
		} else {
			w.Header().Set("Allow", "GET, POST")
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		}
	})

	// HTMX Frontend Routes
	s.mux.Handle("/", s.authMiddleware(s.handleIndex()))

	// Gemini/OpenLLM-backed routes: metered API calls behind a shared
	// password, rate-limited per client IP so a looping client (or a leaked
	// session) can't run up the bill. Generous burst -- this stops a
	// runaway loop, it isn't meant to police normal interactive retries.
	llmRateLimit := s.rateLimitMiddleware(rate.Every(2*time.Second), 10)

	// API Routes (to be protected by Secret Manager Password)
	s.mux.Handle("/api/preset/generate", s.authMiddleware(llmRateLimit(s.handleGeneratePreset())))
	s.mux.Handle("/api/preset/status", s.authMiddleware(s.handleTaskStatus()))

	// Preset Management Routes
	s.mux.Handle("/api/presets", s.authMiddleware(s.handleGetPresets()))
	s.mux.Handle("/api/preset/save", s.authMiddleware(s.handleSavePreset()))
	s.mux.Handle("/api/preset/delete", s.authMiddleware(s.handleDeletePreset()))
	s.mux.Handle("/api/preset/copy_ui", s.authMiddleware(s.handleCopyPresetUI()))
	s.mux.Handle("/api/preset/copy", s.authMiddleware(s.handleCopyPreset()))
	s.mux.Handle("/api/preset/view", s.authMiddleware(s.handleViewPreset()))
	s.mux.Handle("/api/preset/chat", s.authMiddleware(llmRateLimit(s.handleChatPreset())))
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

// formOn reports whether any submitted value of a checkbox field is "on".
func formOn(vals []string) bool {
	for _, v := range vals {
		if v == "on" {
			return true
		}
	}
	return false
}

// handleGeneratePreset kicks off the Gemini ADK Phase 1 Orchestrator.
func (s *Server) handleGeneratePreset() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", http.MethodPost)
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

		// A checkbox absent from the form means "not specified" (the service
		// applies its default); present means on iff one of its values is "on".
		opts := presetsvc.GenerateOptions{Mock: r.FormValue("mock") == "true", Source: runs.SourceUI}
		for field, dst := range map[string]**bool{
			"allow_factory_captures": &opts.AllowFactoryCaptures,
			"allow_user_captures":    &opts.AllowUserCaptures,
			"allow_paid_plugins":     &opts.AllowPaidPlugins,
			"favor_captures":         &opts.FavorCaptures,
			"favor_cloud_captures":   &opts.FavorCloudCaptures,
		} {
			if vals, ok := r.Form[field]; ok {
				v := formOn(vals)
				*dst = &v
			}
		}

		taskID, err := s.svc.StartGenerate(r.Context(), prompt, opts)
		if err != nil {
			var credErr *presetsvc.CredentialsError
			switch {
			case errors.As(err, &credErr):
				http.Error(w, credErr.Public, http.StatusInternalServerError)
			case errors.Is(err, presetsvc.ErrBusy):
				w.Header().Set("Retry-After", "30")
				http.Error(w, "Another generation is already running. Please try again in a moment.", http.StatusTooManyRequests)
			default:
				log.Printf("Failed to start generation: %v", err)
				http.Error(w, "Failed to start generation", http.StatusInternalServerError)
			}
			return
		}

		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(fmt.Sprintf(`
			<div id="gen-progress-area" hx-get="/api/preset/status?id=%s&scope=gen" hx-trigger="every 2s" hx-swap="outerHTML">
				<div class="progress-panel" style="padding: 0.75rem 1rem; display: flex; flex-direction: row; align-items: center; gap: 0.75rem;">
					<span class="spinner" style="display:inline-block;"></span>
					<span style="color: white; font-size: 0.95rem;">Current: <span style="color: var(--accent);">Initializing ADK Pipeline...</span></span>
				</div>
			</div>
			<button id="gen-submit-btn" style="display: none;" hx-swap-oob="true"></button>
		`, taskID)))
	}
}

// taskView resolves a task ID to the state handleTaskStatus renders. The
// tasks override map wins; otherwise the run comes from the preset service,
// from memory first and then from its persisted record.
func (s *Server) taskView(ctx context.Context, id string) (TaskState, bool) {
	s.tasksMu.RLock()
	task, ok := s.tasks[id]
	var copied TaskState
	if ok {
		copied = *task
	}
	s.tasksMu.RUnlock()
	if ok {
		return copied, true
	}

	if res, ok := s.svc.Result(id); ok {
		return s.viewFromRecord(res.Record, res.Preset, strings.HasPrefix(id, "chat-")), true
	}

	rec, err := s.svc.Run(ctx, id)
	if err != nil {
		return TaskState{}, false
	}
	var preset *storage.Preset
	if rec.Status == runs.StatusComplete && rec.PresetID != "" && s.store != nil {
		preset, _ = s.store.Get(ctx, rec.PresetID)
	}
	return s.viewFromRecord(*rec, preset, strings.HasPrefix(id, "chat-")), true
}

func (s *Server) viewFromRecord(rec runs.RunRecord, preset *storage.Preset, isChat bool) TaskState {
	switch rec.Status {
	case runs.StatusRunning:
		return TaskState{Status: "running", Phase: rec.Phase}
	case runs.StatusComplete:
		if preset == nil {
			return TaskState{Status: "error", Error: "Result is no longer available; open it from your library."}
		}
		// Generated drafts render interactive; refinements render static,
		// exactly as handleChatPreset's own response does.
		return TaskState{Status: "complete", Result: renderTweakingWorkspaceHTML(preset, false, isChat)}
	default: // error, abandoned
		return TaskState{Status: "error", Error: rec.Error}
	}
}

func (s *Server) handleTaskStatus() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.URL.Query().Get("id")
		if id == "" {
			http.Error(w, "Missing task ID", http.StatusBadRequest)
			return
		}

		view, ok := s.taskView(r.Context(), id)
		if !ok {
			http.Error(w, "Task not found", http.StatusNotFound)
			return
		}
		task := &view

		w.Header().Set("Content-Type", "text/html")
		isChat := strings.HasPrefix(id, "chat-")
		scope := r.URL.Query().Get("scope")
		if scope == "" {
			if isChat {
				scope = "lib"
			} else {
				scope = "gen"
			}
		}

		btnID := fmt.Sprintf("%s-submit-btn", scope)
		if isChat {
			btnID = fmt.Sprintf("%s-chat-submit-btn", scope)
		}

		if task.Status == "running" {
			areaID := fmt.Sprintf("%s-progress-area", scope)
			if isChat {
				areaID = fmt.Sprintf("%s-chat-progress-area", scope)
			}
			w.Write([]byte(fmt.Sprintf(`
				<div id="%s" hx-get="/api/preset/status?id=%s&scope=%s" hx-trigger="every 2s" hx-swap="outerHTML">
					<div class="progress-panel" style="padding: 0.75rem 1rem; display: flex; flex-direction: row; align-items: center; gap: 0.75rem;">
						<span class="spinner" style="display:inline-block;"></span>
						<span style="color: white; font-size: 0.95rem;">Current: <span style="color: var(--accent);">%s</span></span>
					</div>
				</div>
			`, areaID, id, scope, html.EscapeString(task.Phase))))
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
				<div id="%[2]s-chat-progress-area"></div>
				<button id="%[2]s-chat-submit-btn" hx-swap-oob="true" style="padding: 0.85rem 1.5rem; border-radius: 8px; background: var(--accent); color: white; border: none; font-weight: 600; font-size: 0.95rem; cursor: pointer; transition: all 0.2s; height: 48px; display: flex; align-items: center; justify-content: center; gap: 0.5rem; min-width: 100px;">
					<span class="btn-text">Adjust</span>
				</button>
				%[1]s
			`, task.Result, scope)))
			return
		}

		w.Write([]byte(fmt.Sprintf(`
			<div id="%[2]s-progress-area"></div>
			<button id="%[2]s-submit-btn" hx-swap-oob="true" style="padding: 0.85rem 1.5rem; border-radius: 8px; background: var(--accent); color: white; border: none; font-weight: 600; font-size: 0.95rem; cursor: pointer; transition: all 0.2s; height: 48px; display: flex; align-items: center; justify-content: center; gap: 0.5rem; min-width: 100px;">
				<span class="btn-text">Spin Up ADK Pipeline</span>
			</button>
			%[1]s
		`, task.Result, scope)))
		return

	}
}
