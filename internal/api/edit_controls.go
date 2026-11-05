package api

// Edit-view controls for one effect block. A block whose model is in the device catalog
// shows every device control: the ones the preset sets, with their values, and the rest at
// the device default (marked "default"), each as the control the device has -- a dropdown
// of its positions, a slider over its real range, or a text box where the range or display
// scale isn't known. Parameters that aren't catalog controls (Bypass, the cab's Mic 1/Mic 2
// choice, names the device doesn't have) and blocks outside the catalog keep the generic
// controls.

import (
	"encoding/json"
	"fmt"
	"html"
	"math"
	"strings"

	"github.com/weitzer-org/sound-builder/internal/agents"
	"github.com/weitzer-org/sound-builder/internal/storage"
)

// editTarget identifies where an edit is saved.
type editTarget struct {
	presetID, guitar, blockID, tabID string
}

// hxVals renders the update request's fields for an hx-vals attribute.
func (t editTarget) hxVals(paramName, unit string) string {
	vals := map[string]string{"preset_id": t.presetID, "guitar": t.guitar, "block_id": t.blockID, "param_name": paramName}
	if unit != "" {
		vals["unit"] = unit // a slider posts a bare number; the handler appends the unit
	}
	b, _ := json.Marshal(vals)
	return html.EscapeString(string(b))
}

// sliderLiveScript updates the live value shown above a slider (the last element of the
// label row just before it) as it moves. It is one constant for every slider: nothing
// preset-specific is ever interpolated into it, because parameter names and block ids are
// LLM-written and HTML escaping doesn't protect a JS string inside an event attribute (the
// browser decodes &#39; back to ' before the script runs). The unit comes from the input's
// data-unit attribute, which is HTML-escaped data and is assigned as text.
const sliderLiveScript = `this.previousElementSibling.lastElementChild.innerText = this.value + (this.dataset.unit || '')`

// blockControlsHTML renders the controls for one block.
func blockControlsHTML(t editTarget, block storage.EffectBlock) string {
	var sb strings.Builder
	ctl, ok := agents.DeviceControlsFor(block.Model)
	if !ok {
		for _, param := range block.Parameters {
			sb.WriteString(genericParamHTML(t, param))
		}
		return sb.String()
	}
	// Each device control shows the preset's parameter for it, if any, else its default.
	paramFor := make(map[int]int, len(block.Parameters))
	var extra []storage.BlockParameter
	for j, param := range block.Parameters {
		i := ctl.Index(param.Name)
		if _, dup := paramFor[i]; i < 0 || dup {
			extra = append(extra, param)
			continue
		}
		paramFor[i] = j
	}
	for _, param := range extra {
		if leadingParam(param.Name) {
			sb.WriteString(genericParamHTML(t, param))
		}
	}
	for i, c := range ctl.Controls {
		if j, ok := paramFor[i]; ok {
			sb.WriteString(deviceControlHTML(t, c, block.Parameters[j].Name, block.Parameters[j].Value, false))
		} else {
			sb.WriteString(deviceControlHTML(t, c, c.Name, c.DefaultValue(), true))
		}
	}
	for _, param := range extra {
		if !leadingParam(param.Name) {
			sb.WriteString(genericParamHTML(t, param))
		}
	}
	return sb.String()
}

// leadingParam reports the parameters shown before a block's device controls: Bypass,
// and a cab's Mic 1/Mic 2 microphone choice, which its per-mic controls depend on.
func leadingParam(name string) bool {
	switch strings.ToLower(strings.ReplaceAll(strings.TrimSpace(name), " ", "")) {
	case "bypass", "mic1", "mic2":
		return true
	}
	return false
}

const (
	paramGroupStyle = `display: flex; flex-direction: column; gap: 0.5rem;`
	labelRowStyle   = `display: flex; justify-content: space-between; gap: 0.5rem; font-size: 0.9rem; color: var(--text-sub);`
	textInputStyle  = `padding: 0.5rem; background: rgba(0,0,0,0.2); border: 1px solid var(--border); border-radius: 4px; color: white; font-size: 0.9rem;`
)

// controlLabel is a control's label, with a muted "default" tag for one the preset
// leaves at the device default.
func controlLabel(name string, atDefault bool) string {
	label := html.EscapeString(name)
	if atDefault {
		label += ` <span class="param-default-tag" style="font-size: 0.75rem; opacity: 0.6; font-style: italic;">default</span>`
	}
	return label
}

// deviceControlHTML renders one device control. name is what an edit saves under: the
// preset's own parameter name when it has one, else the device's control name.
func deviceControlHTML(t editTarget, c agents.DeviceControl, name, value string, atDefault bool) string {
	label := controlLabel(c.Name, atDefault)
	if len(c.Steps) > 0 {
		return selectHTML(t, name, label, value, c.Steps)
	}
	if c.Slider {
		if x, ok := c.ParseValue(value); ok {
			lo, hi := *c.Min, *c.Max
			x = math.Max(lo, math.Min(hi, x))
			return sliderHTML(label, c.FormatValue(x), t.hxVals(name, c.Units),
				formatFloat(lo), formatFloat(hi), formatFloat(sliderStep(c)), formatFloat(x), unitSuffix(c.Units))
		}
	}
	return textInputHTML(t, name, label, value)
}

// sliderStep is a sensible increment for a control's range.
func sliderStep(c agents.DeviceControl) float64 {
	span := *c.Max - *c.Min
	switch {
	case c.Units == "dB":
		return 0.5
	case span <= 2:
		return 0.01
	case span <= 20:
		return 0.1
	}
	return 1
}

// unitSuffix is the unit shown after a slider's live value (" dB"), or "".
func unitSuffix(units string) string {
	if units == "" {
		return ""
	}
	return " " + units
}

// sliderHTML renders a range slider with its label row and live value. label is HTML
// (it may carry the "default" tag) and hxVals is already escaped; every other argument
// is plain text and escaped here.
func sliderHTML(label, display, hxVals, min, max, step, value, unit string) string {
	return fmt.Sprintf(`
							<div class="param-group" style="%[1]s">
								<div style="%[2]s"><span>%[3]s</span><span>%[4]s</span></div>
								<input type="range" name="value" hx-post="/api/preset/update_parameter" hx-trigger="change" hx-vals="%[5]s" min="%[6]s" max="%[7]s" step="%[8]s" value="%[9]s" data-unit="%[10]s" style="width: 100%%; cursor: pointer;" oninput="%[11]s">
							</div>`, paramGroupStyle, labelRowStyle, label, html.EscapeString(display), hxVals,
		html.EscapeString(min), html.EscapeString(max), html.EscapeString(step), html.EscapeString(value),
		html.EscapeString(unit), html.EscapeString(sliderLiveScript))
}

func formatFloat(x float64) string {
	return strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.4f", x), "0"), ".")
}

func selectHTML(t editTarget, name, label, value string, options []string) string {
	var opts strings.Builder
	found := false
	for _, o := range options {
		sel := ""
		if strings.EqualFold(strings.TrimSpace(value), o) {
			sel, found = " selected", true
		}
		fmt.Fprintf(&opts, `<option value="%[1]s"%[2]s>%[1]s</option>`, html.EscapeString(o), sel)
	}
	if !found && strings.TrimSpace(value) != "" {
		// Keep a value that isn't one of the positions visible rather than silently
		// showing (and on the next change, saving over it with) the first option.
		fmt.Fprintf(&opts, `<option value="%[1]s" selected>%[1]s</option>`, html.EscapeString(value))
	}
	return fmt.Sprintf(`
							<div class="param-group" style="%[1]s">
								<label style="font-size: 0.9rem; color: var(--text-sub);">%[2]s</label>
								<select name="value" hx-post="/api/preset/update_parameter" hx-trigger="change" hx-vals="%[3]s" style="%[4]s">%[5]s</select>
							</div>`, paramGroupStyle, label, t.hxVals(name, ""), textInputStyle, opts.String())
}

func textInputHTML(t editTarget, name, label, value string) string {
	return fmt.Sprintf(`
							<div class="param-group" style="%[1]s">
								<label style="font-size: 0.9rem; color: var(--text-sub);">%[2]s</label>
								<input type="text" name="value" hx-post="/api/preset/update_parameter" hx-trigger="keyup delay:500ms" hx-vals="%[3]s" value="%[4]s" style="%[5]s">
							</div>`, paramGroupStyle, label, t.hxVals(name, ""), html.EscapeString(value), textInputStyle)
}

// genericParamHTML renders a parameter with no catalog control: a 0-10 slider for a bare
// number (the dial convention), a dropdown for an on/off toggle, else a text box.
func genericParamHTML(t editTarget, param storage.BlockParameter) string {
	label := html.EscapeString(param.Name)
	if param.Type == "slider" && param.Unit == "" && isBareSliderNumber(param.Value) {
		// Sliders are hard-coded 0-10, which only makes sense for a true dimensionless
		// 0-10 value (Gain/Tone/Volume). A param carrying a real-world unit -- in Unit, or
		// (as the live pipeline usually writes it) inside Value, e.g. "+3.0 dB" -- can't
		// be shown on that range, so it gets a text box instead.
		v := strings.TrimSpace(param.Value)
		return sliderHTML(label, v, t.hxVals(param.Name, ""), "0", "10", "0.1", v, "")
	}
	if param.Type == "toggle" {
		// A dropdown, not a checkbox: an unticked checkbox posts no value at all, which
		// saved an empty string, and the old check only recognised lower-case "on".
		return selectHTML(t, param.Name, label, param.Value, []string{"Off", "On"})
	}
	value := param.Value
	if param.Unit != "" {
		value = param.Value + " " + param.Unit
	}
	return textInputHTML(t, param.Name, label, value)
}
