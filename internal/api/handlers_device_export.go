package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"

	"github.com/weitzer-org/sound-builder/internal/agents"
	"github.com/weitzer-org/sound-builder/internal/deviceexport"
	"github.com/weitzer-org/sound-builder/internal/presetsvc"
)

// DeviceExporter is the device export (plan P2) configured from the app config: the
// user's plugins come from allow_paid_plugins/available_plugins, and capture names from
// coros_map.json and user_captures.json. cmd/server shares it with the MCP server and the
// device bridge, so all three build the same export.
func (s *Server) DeviceExporter() (*deviceexport.Exporter, error) {
	var owned map[string]bool
	if cfg := s.appConfig; cfg != nil {
		owned = deviceexport.OwnedPluginSet(cfg.AllowPaidPlugins, cfg.AvailablePlugins)
	}
	return deviceexport.NewEmbedded(deviceexport.Options{IsCapture: isCaptureName, OwnedPlugins: owned})
}

// isCaptureName reports names coros_map.json or user_captures.json mark as captures.
func isCaptureName(name string) bool {
	isCap, ok := agents.GetValidNativeBlocks()[name]
	return ok && isCap
}

// handleDeviceExport serves GET /api/preset/device_export?id=<preset>&guitar=<name>: one
// guitar's device export as a JSON download. It is returned even when items are
// unresolved; the payload lists them, and only send_preset_to_device can push.
func (s *Server) handleDeviceExport() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", http.MethodGet)
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			return
		}
		id := r.URL.Query().Get("id")
		guitar := r.URL.Query().Get("guitar")
		if id == "" {
			http.Error(w, "Missing id", http.StatusBadRequest)
			return
		}
		p, err := s.store.Get(r.Context(), id)
		if err != nil {
			http.Error(w, "Preset not found", http.StatusNotFound)
			return
		}
		sp, ok := presetsvc.DecodeStructured(p.Payload)
		if !ok {
			http.Error(w, "This preset has no structured block data to export", http.StatusBadRequest)
			return
		}
		if guitar == "" && len(sp.Guitars) == 1 {
			for g := range sp.Guitars {
				guitar = g
			}
		}
		if _, ok := sp.Guitars[guitar]; !ok {
			http.Error(w, "Unknown guitar for this preset", http.StatusNotFound)
			return
		}
		exp, err := s.DeviceExporter()
		if err != nil {
			log.Printf("device export: %v", err)
			http.Error(w, "Device export unavailable", http.StatusServiceUnavailable)
			return
		}
		e, err := exp.Export(r.Context(), sp, guitar)
		if err != nil {
			log.Printf("device export of preset %s: %v", id, err)
			http.Error(w, "Device export failed", http.StatusInternalServerError)
			return
		}
		var body bytes.Buffer
		if err := json.Indent(&body, e.Payload, "", "  "); err != nil {
			http.Error(w, "Device export failed", http.StatusInternalServerError)
			return
		}
		body.WriteByte('\n')
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, deviceExportFilename(id, guitar)))
		w.Header().Set("Cache-Control", "no-store")
		_, _ = w.Write(body.Bytes())
	}
}

var nonSlug = regexp.MustCompile(`[^a-z0-9]+`)

// slug is a lower-case ASCII [a-z0-9-] form of s, at most n bytes; "x" when nothing is left.
func slug(s string, n int) string {
	out := strings.Trim(nonSlug.ReplaceAllString(strings.ToLower(s), "-"), "-")
	if len(out) > n {
		out = strings.Trim(out[:n], "-")
	}
	if out == "" {
		return "x"
	}
	return out
}

// deviceExportFilename is built only from [a-z0-9-], so preset text never reaches the
// Content-Disposition header as is.
func deviceExportFilename(id, guitar string) string {
	return "qc2-device-" + slug(id, 40) + "-" + slug(guitar, 40) + ".json"
}

// deviceExportLinksHTML is the "Download device JSON" link for each guitar of a
// structured preset.
func deviceExportLinksHTML(id string, guitars []string) string {
	if len(guitars) == 0 {
		return ""
	}
	sorted := append([]string(nil), guitars...)
	sort.Strings(sorted)
	var b strings.Builder
	b.WriteString(`<div class="device-export-links" style="display: flex; flex-wrap: wrap; gap: 0.5rem; align-items: center; margin-bottom: 1rem; font-size: 0.85rem; color: var(--text-sub);">`)
	b.WriteString(`<span>Quad Cortex:</span>`)
	for _, g := range sorted {
		href := "/api/preset/device_export?id=" + url.QueryEscape(id) + "&guitar=" + url.QueryEscape(g)
		fmt.Fprintf(&b, `<a class="device-export-link" href="%s" download style="padding: 0.25rem 0.6rem; border: 1px solid var(--border); border-radius: 6px; color: var(--accent); text-decoration: none;">Download device JSON (%s)</a>`,
			html.EscapeString(href), html.EscapeString(g))
	}
	b.WriteString(`</div>`)
	return b.String()
}
