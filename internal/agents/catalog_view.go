package agents

// Device controls for display and editing: the exported face of catalog_params.go's merged
// per-block parameter lists, used by the web editor and the read-only preset views.

import (
	"math"
	"strconv"
	"strings"

	"github.com/weitzer-org/sound-builder/internal/storage"
)

// DeviceControl is one device control of a block.
type DeviceControl struct {
	Name  string // as the catalog names it, with MIC n / IR n slot prefixes
	Units string
	// Min/Max are nil when the catalog only gives firmware symbols for them.
	Min, Max, Default *float64
	// Steps are the switch positions, when the device names them.
	Steps []string
	// Switch: a discrete control. Pan: L..R labelled ends (display scale unverified).
	Switch, Pan bool
	// MinOff/MaxOff: that end of the range turns the control off.
	MinOff, MaxOff bool
	// Slider reports a numeric range the editor can offer as a slider: known bounds, not
	// a pan, not a unitless 0..1 float whose display scale is unverified, and not a
	// switch without named positions.
	Slider bool
}

// DeviceBlockControls are the controls of one block model.
type DeviceBlockControls struct {
	Model    string
	Controls []DeviceControl
	dev      *deviceBlock
}

// DeviceControlsFor returns the device controls of a block model, or false when the name
// isn't a catalog model or family name, names a capture (captures have no catalog
// controls; a coros_map.json or user capture always wins over a catalog name), or carries
// the unverified annotation (FlagUnverifiedStructuredBlocks found it isn't a block this
// preset can use, so no device controls are claimed for it, matching the parameter check).
// A capture annotation on the name is ignored.
func DeviceControlsFor(model string) (*DeviceBlockControls, bool) {
	name := stripCaptureAnnotation(model)
	if strings.HasSuffix(model, unverifiedSuffix) {
		return nil, false
	}
	if GetValidNativeBlocks()[name] {
		return nil, false // a capture
	}
	dev, ok := deviceBlockFor(name)
	if !ok {
		return nil, false
	}
	out := &DeviceBlockControls{Model: dev.Name, dev: dev, Controls: make([]DeviceControl, len(dev.Params))}
	for i, p := range dev.Params {
		out.Controls[i] = DeviceControl{
			Name: p.Label, Units: p.Units, Min: p.Min, Max: p.Max, Default: p.Default,
			Steps: p.StepNames, Switch: p.Switch, Pan: p.Pan, MinOff: p.MinOff, MaxOff: p.MaxOff,
			Slider: p.rangeCheckable() && !p.Switch,
		}
	}
	return out, true
}

// Index returns the position in Controls of the control a written parameter name refers
// to (ignoring case, spaces and inter-letter hyphens, plus the check's aliases), or -1.
func (d *DeviceBlockControls) Index(paramName string) int {
	dp, ok := d.dev.lookupParam(paramName)
	if !ok {
		return -1
	}
	for i, p := range d.dev.Params {
		if p == dp {
			return i
		}
	}
	return -1
}

// ParseValue reads a written value as a number in the control's own units: "250 ms" and
// "0.25 s" are both 250 on a ms control, and "OFF" is the end of the range labelled OFF.
// False for text, a value in a unit of another dimension, or a unit it doesn't know.
func (c DeviceControl) ParseValue(v string) (float64, bool) {
	v = strings.TrimSpace(v)
	if strings.EqualFold(v, "OFF") {
		switch {
		case c.MinOff && c.Min != nil:
			return *c.Min, true
		case c.MaxOff && c.Max != nil:
			return *c.Max, true
		}
		return 0, false
	}
	m := valuePattern.FindStringSubmatch(v)
	if m == nil {
		return 0, false
	}
	x, err := strconv.ParseFloat(m[1], 64)
	if err != nil {
		return 0, false
	}
	if m[2] == "" || c.Units == "" {
		if m[2] != "" && c.Units == "" {
			return 0, false
		}
		return x, true
	}
	wDim, wFactor, wOK := unitDimension(m[2])
	cDim, cFactor, cOK := unitDimension(c.Units)
	if !wOK || !cOK || wDim != cDim {
		return 0, false
	}
	return x * wFactor / cFactor, true
}

// defaultStep is the named position the device starts at, if any.
func (c DeviceControl) defaultStep() string {
	if len(c.Steps) == 0 || c.Default == nil || c.Min == nil {
		return ""
	}
	if i := int(math.Round(*c.Default - *c.Min)); i >= 0 && i < len(c.Steps) {
		return c.Steps[i]
	}
	return ""
}

// IsDefault reports whether a written value is the device default for this control.
func (c DeviceControl) IsDefault(v string) bool {
	if s := c.defaultStep(); s != "" {
		return strings.EqualFold(strings.TrimSpace(v), s)
	}
	if c.Default == nil {
		return false
	}
	x, ok := c.ParseValue(v)
	return ok && math.Abs(x-*c.Default) < 1e-9
}

// FormatValue writes a number in the control's units, or "OFF" at an end labelled OFF.
func (c DeviceControl) FormatValue(x float64) string {
	if (c.MinOff && c.Min != nil && x <= *c.Min) || (c.MaxOff && c.Max != nil && x >= *c.Max) {
		return "OFF"
	}
	s := formatNum(math.Round(x*1000) / 1000)
	if c.Units != "" {
		s += " " + c.Units
	}
	return s
}

// DefaultValue is the control's device default as a written value ("On", "5", "35 %",
// "OFF"), or "" when the catalog gives none.
func (c DeviceControl) DefaultValue() string {
	if s := c.defaultStep(); s != "" {
		return s
	}
	if c.Default == nil {
		return ""
	}
	return c.FormatValue(*c.Default)
}

// ValueWithUnit is a parameter's value with its separately stored unit put back
// ("0.25" + "s" -> "0.25 s"), so it parses in the right units. Only a bare number gets
// the unit; text ("OFF") or a value that already carries a unit is returned as is.
func ValueWithUnit(value, unit string) string {
	unit = strings.TrimSpace(unit)
	if unit == "" {
		return value
	}
	if m := valuePattern.FindStringSubmatch(value); m != nil && m[2] == "" {
		return strings.TrimSpace(value) + " " + unit
	}
	return value
}

// SplitDefaults splits a block's parameters for the read-only views, so that every device
// control of a catalog block appears exactly once: as a listed parameter, or by name in the
// "at device default" list. shown are the parameters to list: everything except those set to
// the device default in both scenes. atDefault names, in the device's order, every device
// control the block doesn't set to another value: its kept_defaults, parameters equal to the
// default in both scenes, and controls the preset never mentions (the device leaves those at
// their default too; the parameter check notes them as not considered). For a block whose
// model has no catalog controls (a capture, an unknown name), every parameter is shown and
// atDefault is nil.
func SplitDefaults(b storage.EffectBlock) (shown []storage.BlockParameter, atDefault []string) {
	d, ok := DeviceControlsFor(b.Model)
	if !ok {
		return b.Parameters, nil
	}
	set := make([]bool, len(d.Controls)) // listed with a non-default value: shown, so not "at default"
	for _, p := range b.Parameters {
		i := d.Index(p.Name)
		if i >= 0 && d.Controls[i].IsDefault(ValueWithUnit(p.Value, p.Unit)) &&
			(p.ValueB == "" || d.Controls[i].IsDefault(ValueWithUnit(p.ValueB, p.Unit))) {
			continue
		}
		if i >= 0 {
			set[i] = true
		}
		shown = append(shown, p)
	}
	for i, c := range d.Controls {
		if !set[i] {
			atDefault = append(atDefault, c.Name)
		}
	}
	return shown, atDefault
}
