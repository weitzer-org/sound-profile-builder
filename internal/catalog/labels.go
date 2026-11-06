package catalog

import (
	"fmt"
	"regexp"
	"strings"
)

// Parameter labels shared by the Architect's parameter context (internal/agents) and the
// device export (internal/deviceexport), so the name a preset was written with is the
// name the export resolves.

// LabelledParam is one user-facing parameter of a model with the label presets use for it.
type LabelledParam struct {
	// Label is the catalog name, prefixed with its slot ("MIC 1 LEVEL") when the model
	// repeats the name once per mic or IR slot (dual-mic cabs, dual IR loaders).
	Label string
	// Key is ParamNameKey(Label).
	Key   string
	Param Param
}

// LabelParams returns a model's user-facing parameters in catalog order, internal ones
// (IsInternalParam) left out. A name the model repeats is numbered by occurrence: the
// first LEVEL of a dual-mic cab is "MIC 1 LEVEL", the second "MIC 2 LEVEL".
func LabelParams(m *Model) []LabelledParam {
	total := map[string]int{}
	for _, p := range m.Params {
		if !IsInternalParam(p) {
			total[p.Name]++
		}
	}
	seen := map[string]int{}
	out := make([]LabelledParam, 0, len(m.Params))
	for _, p := range m.Params {
		if IsInternalParam(p) {
			continue
		}
		seen[p.Name]++
		label := p.Name
		if total[p.Name] > 1 {
			label = fmt.Sprintf("%s %d %s", SlotPrefix(m.Category), seen[p.Name], p.Name)
		}
		out = append(out, LabelledParam{Label: label, Key: ParamNameKey(label), Param: p})
	}
	return out
}

// internalParamNames are catalog parameters that aren't user controls: the per-slot
// bypass/phase switches and the IR file index. Lower-case in the device's own XML.
var internalParamNames = map[string]bool{"bypass": true, "phi": true, "ir selector": true}

// internalParamTypes are read-only meters and placeholders.
var internalParamTypes = map[string]bool{"string": true, "grMeter": true, "empty": true}

// IsInternalParam reports a catalog parameter that isn't a user control.
func IsInternalParam(p Param) bool {
	return internalParamTypes[p.Type] || internalParamNames[p.Name]
}

// ContinuousParamTypes are the types converted display <-> wire by range and skew
// (Param.ToWire/ToDisplay). "fader" is there by owner decision (2026-11-06) as a labelled
// guess, not a measurement; a type not listed is refused by default.
var ContinuousParamTypes = map[string]bool{"float": true, "fader": true}

// DiscreteParamTypes are the catalog parameter types with discrete positions.
var DiscreteParamTypes = map[string]bool{"switch": true, "rotarySwitch": true, "toggleButton": true, "comboBox": true}

var interLetterHyphen = regexp.MustCompile(`([A-Z])-([A-Z])`)

// ParamNameKey normalizes a parameter name for comparison: upper case, a hyphen between
// two letters dropped ("PRE-DELAY" = "PRE DELAY"), spaces and underscores dropped ("65 Hz"
// = "65Hz"). Other punctuation is kept, so "OCT +1" and "OCT -1" stay distinct.
// TestParamNameKeyHasNoCollisions (internal/agents) checks that no two parameters of one
// model share a key.
func ParamNameKey(name string) string {
	s := strings.ToUpper(strings.TrimSpace(name))
	s = interLetterHyphen.ReplaceAllString(s, "$1$2")
	return paramKeyStripper.Replace(s)
}

// paramKeyStripper drops the separators ParamNameKey ignores. Built once: a Replacer
// compiles its lookup table at construction.
var paramKeyStripper = strings.NewReplacer(" ", "", "_", "", "\t", "")

// SlotPrefix is how a repeated per-slot parameter is labelled: "MIC 1 LEVEL" on cabs,
// "IR 1 LEVEL" on dual IR loaders.
func SlotPrefix(category string) string {
	if category == "IRLoaders" {
		return "IR"
	}
	return "MIC"
}

// ParamNameAliases are spellings presets use for a control the device names differently,
// keyed and valued by ParamNameKey. An alias only applies when its target exists on that
// model.
var ParamNameAliases = map[string][]string{
	"TIME":    {"DELAYTIME"},
	"LOWCUT":  {"HPF", "HIGHPASS", "HIPASS", "HPFFREQ"},
	"HIGHCUT": {"LPF", "LOWPASS", "LPFFREQ"},
}

// UnitDimension maps a written or catalog unit to its dimension and the factor to its base
// unit (Hz, s), so "0.25 s" compares with a "ms" range. Unknown units return ok=false.
func UnitDimension(u string) (dim string, factor float64, ok bool) {
	switch strings.ToLower(strings.TrimSpace(u)) {
	case "db":
		return "level", 1, true
	case "hz":
		return "frequency", 1, true
	case "khz":
		return "frequency", 1000, true
	case "ms":
		return "time", 0.001, true
	case "s", "sec", "secs", "seconds":
		return "time", 1, true
	case "%":
		return "percent", 1, true
	}
	return "", 0, false
}

// cabIRControlPattern matches (in ParamNameKey form) the controls an IR cab has and a Neural
// Capture doesn't: the mic choice, per-mic position/distance/level/pan, the cab's filters and its
// output trim.
var cabIRControlPattern = regexp.MustCompile(`^(?:MIC[12][A-Z]*|HPF|LPF|HPFFREQ|LPFFREQ|HIGHCUT|LOWCUT|OUTPUTVOLUME|OUTPUTLEVEL|BLEND)$`)

// IsCabIRControl reports whether key (a ParamNameKey) is an IR cab control that a Neural Capture
// standing in a cab slot doesn't have. The one list shared by the generate-time check and the
// device export.
func IsCabIRControl(key string) bool { return cabIRControlPattern.MatchString(key) }
