package catalog

import (
	"fmt"
	"math"
)

// Display <-> wire conversion for continuous parameters.
//
// The unit stores a parameter as a wire value in 0..1 and shows a display value in the
// parameter's own range, related by a power-law taper:
//
//	wire    = ((v - min) / (max - min)) ^ skew
//	display = min + (max - min) * wire ^ (1 / skew)
//
// What has been checked on the owner's unit (CorOS 4.1.0, plan 7.1 G4, screen readings):
// skew 3.8018 (amp OUTPUT, -60..+12 dB), skew 4.9594844 (cab LEVEL, -40..+6 dB), skew 0.3
// (Low-High Cut HPF FREQ, 20..20000 Hz), and absent skew = linear (0..10 dials). Skew 0.6
// and LOG_SKEW parameters are NOT confirmed; they convert with the same formula but have no
// device evidence.
//
// Conversions refuse instead of clamping: an out-of-range or NaN input, a parameter type
// whose mapping has not been measured, or an unmeasured bound returns an error, never a
// nearby valid number.

// Bounds returns the parameter's numeric range with symbolic bounds resolved through
// FirmwareBounds. It fails when a bound is missing, unmeasured or unknown, or the range is
// empty or inverted.
func (p Param) Bounds() (lo, hi float64, err error) {
	lo, err = resolveBound(p.Min, p.MinSymbol, "min")
	if err != nil {
		return 0, 0, fmt.Errorf("param %q: %w", p.Name, err)
	}
	hi, err = resolveBound(p.Max, p.MaxSymbol, "max")
	if err != nil {
		return 0, 0, fmt.Errorf("param %q: %w", p.Name, err)
	}
	if !(lo < hi) {
		return 0, 0, fmt.Errorf("param %q: range %g..%g is empty or inverted", p.Name, lo, hi)
	}
	return lo, hi, nil
}

func resolveBound(num *float64, symbol, which string) (float64, error) {
	if num != nil {
		if math.IsNaN(*num) || math.IsInf(*num, 0) {
			return 0, fmt.Errorf("%s bound is not finite", which)
		}
		return *num, nil
	}
	if symbol == "" {
		return 0, fmt.Errorf("%s bound is absent", which)
	}
	if why, ok := UnmeasuredBounds[symbol]; ok {
		return 0, fmt.Errorf("%s bound %s is unmeasured: %s", which, symbol, why)
	}
	b, ok := FirmwareBounds[symbol]
	if !ok {
		return 0, fmt.Errorf("%s bound %s is unknown", which, symbol)
	}
	return b.Value, nil
}

// skewOrLinear is the taper exponent: Skew, or 1 when the XML gave none.
func (p Param) skewOrLinear() (float64, error) {
	if p.Skew == nil {
		return 1, nil
	}
	s := *p.Skew
	if math.IsNaN(s) || math.IsInf(s, 0) || s <= 0 {
		return 0, fmt.Errorf("param %q: skew %g must be a finite number > 0", p.Name, s)
	}
	return s, nil
}

// convertible checks what both directions need before touching a number.
func (p Param) convertible() (lo, hi, skew float64, err error) {
	// Only plain floats have a measured taper. Switches, combo boxes, rotary switches and
	// strings map steps to wire values by a rule nobody has checked on the unit.
	if p.Type != "float" {
		return 0, 0, 0, fmt.Errorf("param %q: type %q has no measured wire mapping", p.Name, p.Type)
	}
	if lo, hi, err = p.Bounds(); err != nil {
		return 0, 0, 0, err
	}
	if skew, err = p.skewOrLinear(); err != nil {
		return 0, 0, 0, err
	}
	return lo, hi, skew, nil
}

// ToWire converts a display value to the 0..1 wire value. It refuses NaN, infinities,
// values outside the parameter's range, non-float parameters and unmeasured bounds.
func (p Param) ToWire(display float64) (float64, error) {
	if math.IsNaN(display) || math.IsInf(display, 0) {
		return 0, fmt.Errorf("param %q: display value %v is not a finite number", p.Name, display)
	}
	lo, hi, skew, err := p.convertible()
	if err != nil {
		return 0, err
	}
	if display < lo || display > hi {
		return 0, fmt.Errorf("param %q: display value %g is outside %g..%g", p.Name, display, lo, hi)
	}
	return math.Pow((display-lo)/(hi-lo), skew), nil
}

// ToDisplay converts a wire value (0..1) to the display value. It refuses NaN, infinities
// (four factory presets store NaN), values outside 0..1, non-float parameters and
// unmeasured bounds.
func (p Param) ToDisplay(wire float64) (float64, error) {
	if math.IsNaN(wire) || math.IsInf(wire, 0) {
		return 0, fmt.Errorf("param %q: wire value %v is not a finite number", p.Name, wire)
	}
	lo, hi, skew, err := p.convertible()
	if err != nil {
		return 0, err
	}
	if wire < 0 || wire > 1 {
		return 0, fmt.Errorf("param %q: wire value %g is outside 0..1", p.Name, wire)
	}
	return lo + (hi-lo)*math.Pow(wire, 1/skew), nil
}

// DeviceParamCount is the length of the params array the device returns for this model:
// every parameter in the XML list (clones= resolved, hidden ones included), plus one
// trailing slot. Checked on the owner's unit (CorOS 4.1.0, plan 7.2) for five blocks, among
// them one cab cloning model 12000; only that one clone family was checked, not all 196
// cloned models. What the trailing slot is remains unknown.
func (m Model) DeviceParamCount() int { return m.ParamCount + 1 }

// ValidParamIndex reports whether a parameter index may be addressed: it must be below
// ParamCount. The trailing device slot (index ParamCount) is NOT valid: nothing may read or
// write it until it is understood. Hidden parameters keep their index and stay addressable
// by position, though they are not listed in Params.
func (m Model) ValidParamIndex(index int) bool {
	return index >= 0 && index < m.ParamCount
}
