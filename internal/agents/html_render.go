package agents

import (
	"encoding/json"
	"html"
	"sort"
	"strings"

	"github.com/weitzer-org/sound-builder/internal/storage"
)

// injectRenderedHTML strips any markdown fencing the model/gateway might still add despite
// structured-output enforcement (defensive -- matters most for the Open-LLM routing branch,
// which isn't schema-constrained), parses structured_payload.guitars if present, and injects
// a freshly Go-rendered final_html_payload built from that exact data, so the two can never
// disagree. If structured_payload.guitars is absent or empty (e.g. a RefineChat turn that
// made no preset change), the input is returned unchanged aside from fence-stripping. A
// failure to parse the outer envelope at all is returned as an error -- that's a genuine
// schema-compliance failure, not a "nothing to render" case.
func injectRenderedHTML(raw string) (string, error) {
	clean := strings.TrimSpace(raw)
	clean = strings.TrimPrefix(clean, "```json")
	clean = strings.TrimPrefix(clean, "```")
	clean = strings.TrimSuffix(clean, "```")
	clean = strings.TrimSpace(clean)

	var envelope map[string]json.RawMessage
	if err := json.Unmarshal([]byte(clean), &envelope); err != nil {
		return clean, err
	}

	rawStructured, ok := envelope["structured_payload"]
	if !ok {
		return clean, nil
	}

	var sp struct {
		Guitars map[string][]storage.EffectBlock `json:"guitars"`
	}
	if err := json.Unmarshal(rawStructured, &sp); err != nil || len(sp.Guitars) == 0 {
		return clean, nil
	}

	htmlBytes, err := json.Marshal(renderAllGuitarsHTML(sp.Guitars))
	if err != nil {
		return clean, nil
	}
	envelope["final_html_payload"] = htmlBytes

	out, err := json.Marshal(envelope)
	if err != nil {
		return clean, err
	}
	return string(out), nil
}

// renderPresetHTML builds the read-only preview table for one guitar from its
// structured block list, replacing what the Architect used to generate directly as free
// text. Deriving it from the same validated data the app persists means the table can
// never drift from structured_payload the way independently-generated HTML could.
func renderPresetHTML(blocks []storage.EffectBlock) string {
	var sb strings.Builder
	sb.WriteString(`<table class='grid-matrix' style='width: 100%; border-collapse: collapse;'>`)
	sb.WriteString(`<thead><tr>`)
	sb.WriteString(`<th style='border-bottom: 2px solid #52525b; padding: 12px; text-align: left;'>Effect Type & Name</th>`)
	sb.WriteString(`<th style='border-bottom: 2px solid #52525b; padding: 12px; text-align: left;'>Scene A (Rhythm)</th>`)
	sb.WriteString(`<th style='border-bottom: 2px solid #52525b; padding: 12px; text-align: left;'>Scene B (Lead)</th>`)
	sb.WriteString(`</tr></thead><tbody>`)

	for _, block := range blocks {
		title := block.Type
		if block.Model != "" {
			title = block.Type + ": " + block.Model
		}

		var sceneA, sceneB []string
		for _, p := range block.Parameters {
			valA := p.Value
			valB := p.ValueB
			if valB == "" {
				valB = valA
			}
			if p.Unit != "" {
				valA += " " + p.Unit
				valB += " " + p.Unit
			}
			sceneA = append(sceneA, html.EscapeString(p.Name+": "+valA))
			sceneB = append(sceneB, html.EscapeString(p.Name+": "+valB))
		}

		sb.WriteString(`<tr><td style='padding: 12px; border-bottom: 1px solid #3f3f46;'>`)
		sb.WriteString(html.EscapeString(title))
		if block.Rationale != "" {
			sb.WriteString(`<br/><div style='font-size: 0.85em; color: #94a3b8; white-space: normal; max-width: 300px; line-height: 1.4; margin-top: 4px;'><em>Rationale: `)
			sb.WriteString(html.EscapeString(block.Rationale))
			sb.WriteString(`</em></div>`)
		}
		sb.WriteString(`</td><td style='padding: 12px; border-bottom: 1px solid #3f3f46;'>`)
		sb.WriteString(strings.Join(sceneA, "<br/>"))
		sb.WriteString(`</td><td style='padding: 12px; border-bottom: 1px solid #3f3f46;'>`)
		sb.WriteString(strings.Join(sceneB, "<br/>"))
		sb.WriteString(`</td></tr>`)
	}

	sb.WriteString(`</tbody></table>`)
	return sb.String()
}

// renderAllGuitarsHTML builds final_html_payload (guitar name -> table HTML) for every
// guitar in a structured preset.
func renderAllGuitarsHTML(guitars map[string][]storage.EffectBlock) map[string]string {
	out := make(map[string]string, len(guitars))
	names := make([]string, 0, len(guitars))
	for name := range guitars {
		names = append(names, name)
	}
	sort.Strings(names) // deterministic map iteration order, not load-bearing beyond that
	for _, name := range names {
		out[name] = renderPresetHTML(guitars[name])
	}
	return out
}
