package catalog

// Symbolic parameter bounds.
//
// ModelRepo.xml gives some parameter ranges as names (MIN_CABSIM_DB, MAX_EQ_DB, ...) whose
// numbers live in the unit's firmware. The values below were measured by others, not
// derived here, so each entry records how far it is trusted. Nothing resolves a name that
// is not in this table.
//
// Attribution: the values marked EvidencePyQuadCortex are data points (plain numbers, no
// code) taken from the published measurements of pyquadcortex
// (github.com/stokes-audio/pyquadcortex, protocol/units.py FIRMWARE_CONSTANTS), which is
// MIT licensed. They were measured on that author's CorOS 4.0.1 unit. No pyquadcortex code
// is used. The qc-mcp snapshot and the web editor list the same numbers, but they cite
// pyquadcortex, so that is not independent confirmation.
//
// Evidence on our own unit (CorOS 4.1.0) is the Phase 0 G4 screen readings, plan 7.1.

// Evidence says how a firmware constant is known.
type Evidence string

const (
	// EvidenceConfirmed410: read from the owner's screen on CorOS 4.1.0.
	EvidenceConfirmed410 Evidence = "confirmed on CorOS 4.1.0"
	// EvidencePyQuadCortex: pyquadcortex's 4.0.1 measurement (MIT), data only, NOT
	// confirmed on 4.1.0.
	EvidencePyQuadCortex Evidence = "pyquadcortex 4.0.1 measurement (MIT), data only, NOT confirmed on 4.1.0"
)

// FirmwareBound is one resolved symbolic bound.
type FirmwareBound struct {
	Value    float64
	Evidence Evidence
	// Note is the specific evidence, e.g. the G4 points.
	Note string
}

const cabsimG4 = "confirmed at two points (G4): cab LEVEL, skew 4.9594844, wire 0.01 -> -21.8 dB and wire 1.0 -> +6.0 dB. " +
	"The -21.8 point fits the minimum and the skew as a pair; it does not pin either alone, so -40 at wire 0 is not separately confirmed."

// FirmwareBounds maps a symbolic bound name to its value.
var FirmwareBounds = map[string]FirmwareBound{
	"MIN_CABSIM_DB": {-40, EvidenceConfirmed410, cabsimG4},
	"MAX_CABSIM_DB": {6, EvidenceConfirmed410, cabsimG4},

	"MIN_EQ_DB": {-12, EvidencePyQuadCortex, ""},
	"MAX_EQ_DB": {12, EvidencePyQuadCortex, ""},

	"MIN_MIXER_DB": {-40, EvidencePyQuadCortex, ""},
	"MAX_MIXER_DB": {12, EvidencePyQuadCortex, ""},

	"MIN_FXLOOP_OUT_GAIN_DB": {-40, EvidencePyQuadCortex, ""},
	"MAX_FXLOOP_OUT_GAIN_DB": {0, EvidencePyQuadCortex, ""},

	"MIN_FXLOOP_IN_GAIN_DB": {-40, EvidencePyQuadCortex, ""},
	"MAX_FXLOOP_IN_GAIN_DB": {12, EvidencePyQuadCortex, ""},

	"MIN_TEMPO": {40, EvidencePyQuadCortex, ""},
	"MAX_TEMPO": {240, EvidencePyQuadCortex, ""},

	"MIN_EQ_FREQ": {20, EvidencePyQuadCortex, ""},
	"MAX_EQ_FREQ": {20000, EvidencePyQuadCortex, ""},
}

// UnmeasuredBounds lists symbolic bounds nobody has measured. ToWire and ToDisplay refuse
// a parameter that uses one, on purpose.
var UnmeasuredBounds = map[string]string{
	"MIN_INPUT_TRIM": "unmeasured (pyquadcortex leaves it out on purpose)",
	"MAX_INPUT_TRIM": "unmeasured (pyquadcortex leaves it out on purpose)",
}

// Device evidence for the push path (internal/deviceexport). The display-to-wire formula
// and the clone expansion are only trusted on the unit where they have been checked; the
// export refuses everything else rather than guessing. Add an entry only with a measurement
// recorded in docs/design/qc-device-bridge-plan.md section 7.

// ConfirmedSkews are the numeric taper exponents whose display-to-wire mapping was read on
// the unit's screen (CorOS 4.1.0, plan 7.1 G4). A symbolic LOG_SKEW (stored as 0.3) is not
// covered by the 0.3 entry: see Param.SkewSymbol.
var ConfirmedSkews = map[float64]string{
	1:         "G4: GAIN wire 0.45 = 4.5 and MASTER wire 0.3 = 3.0 on a 0..10 dial (linear)",
	0.3:       "G4: Low-High Cut HPF FREQ wire 0.25 = 217 Hz (numeric skew 0.3, 20..20000 Hz)",
	3.8018:    "G4: Brit 2203 OUTPUT wire 0.25 = -10.0 dB (-60..+12 dB)",
	4.9594844: "G4: cab LEVEL wire 0.01 = -21.8 dB and wire 1.0 = +6.0 dB (-40..+6 dB)",
}

// VerifiedCloneBases are the clones= base models whose expanded parameter layout (our
// catalog's indexes) was matched against the device's own parameter array (plan 7.2, G1).
// Every other clone base's indexes are unverified.
var VerifiedCloneBases = map[int]string{
	12000: "G1 (CorOS 4.1.0): 110 US PRN C10R (M), id 12045, a dual-mic cab cloning 12000; N = 22 = full list + 1, IR names at 1 and 9",
}
