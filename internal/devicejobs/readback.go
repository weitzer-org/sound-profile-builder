package devicejobs

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"unicode/utf8"

	"github.com/weitzer-org/sound-builder/internal/storage"
)

// The qc-device-readback/1 payload (contract docs/design/device-job-contract.md section 9.1):
// the preset open on the unit, as the home agent reads it, in the agent's positional form.
// The Python reference is qc_agent.jobs.readback_payload; testdata/devicejobs/readback.json
// and readback_roundtrip/*.json pin both sides.
//
// Everything in it is untrusted device text and numbers. ParseReadbackPayload checks the
// shape only (types, counts, ranges); what the numbers mean is the importer's job.

// ReadbackFormat is the format string the agent writes in Result.Readback.Format.
const ReadbackFormat = "qc-device-readback/1"

// Limits of the payload, from the device's own geometry (4 rows of 8 columns, 8 scenes) and
// the agent's text bound. Anything beyond them is refused, not truncated.
const (
	ReadbackScenes      = 8
	ReadbackRows        = 4
	ReadbackCols        = 8
	MaxReadbackParams   = 256
	MaxReadbackModelID  = 1<<31 - 1
	MaxReadbackStringRn = 192 // runes: a capture's 64-hex key, a separator and its name
)

// ReadbackPayload is the payload object.
type ReadbackPayload struct {
	Setlist ReadbackSetlist `json:"setlist"`
	Chains  []ReadbackChain `json:"chains"`
	// Bypass is bypass[row][col][scene] (true = bypassed), or nil when the unit sent none.
	Bypass [][][]bool `json:"bypass"`
}

// ReadbackSetlist says where the open preset sits: Folder is "My Presets" or "other".
type ReadbackSetlist struct {
	Folder   string `json:"folder"`
	Position *int   `json:"position"`
}

// ReadbackChain is one grid row.
type ReadbackChain struct {
	Row     int              `json:"row"`
	InPort  *int             `json:"in_port"`
	OutPort *int             `json:"out_port"`
	Blocks  []ReadbackPlaced `json:"blocks"`
}

// ReadbackPlaced is a placed block with every parameter the device returned: the model's
// own list plus one trailing slot (plan 7.2). (ReadbackBlock is the result's envelope.)
type ReadbackPlaced struct {
	Col     int             `json:"col"`
	ModelID int             `json:"model_id"`
	Params  []ReadbackParam `json:"params"`
}

// ReadbackParam is one parameter slot: its position, whether the unit has it scene-assigned,
// and its eight scene values (A-H). A value is a JSON number, string, boolean or null; use
// Float and Text.
type ReadbackParam struct {
	Index     int   `json:"index"`
	SceneMode bool  `json:"scene_mode"`
	Values    []any `json:"values"`
}

// Float returns scene s's value when it is a finite number.
func (p ReadbackParam) Float(s int) (float64, bool) {
	if s < 0 || s >= len(p.Values) {
		return 0, false
	}
	n, ok := p.Values[s].(json.Number)
	if !ok {
		return 0, false
	}
	f, err := n.Float64()
	if err != nil || f != f || f > 1.7976931348623157e308 || f < -1.7976931348623157e308 {
		return 0, false
	}
	return f, true
}

// Text returns scene s's value when it is a string.
func (p ReadbackParam) Text(s int) (string, bool) {
	if s < 0 || s >= len(p.Values) {
		return "", false
	}
	t, ok := p.Values[s].(string)
	return t, ok
}

// ParseReadbackPayload decodes and shape-checks a read_preset payload. It rejects unknown
// fields (a new agent field means a new format version), trailing data, wrong types, and
// counts or indexes outside the device's geometry. The error text is for logs and the
// tool's refusal message and never echoes payload text.
func ParseReadbackPayload(raw json.RawMessage) (*ReadbackPayload, error) {
	if len(raw) == 0 {
		return nil, errors.New("readback payload is empty")
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	dec.UseNumber() // for the []any values; typed ints below still decode as ints
	var p ReadbackPayload
	if err := dec.Decode(&p); err != nil {
		return nil, fmt.Errorf("readback payload does not match %s: %s", ReadbackFormat, shortErr(err))
	}
	if _, err := dec.Token(); err != io.EOF {
		return nil, errors.New("readback payload has data after the object")
	}
	if err := p.validate(); err != nil {
		return nil, err
	}
	return &p, nil
}

// shortErr drops the offending value some decoder errors quote.
func shortErr(err error) string {
	var ute *json.UnmarshalTypeError
	if errors.As(err, &ute) {
		return fmt.Sprintf("field %q has the wrong type", ute.Field)
	}
	var se *json.SyntaxError
	if errors.As(err, &se) {
		return "invalid JSON"
	}
	return "unknown or malformed field"
}

func (p *ReadbackPayload) validate() error {
	if p.Chains == nil {
		return errors.New("readback payload: chains is missing")
	}
	if len(p.Chains) > ReadbackRows {
		return fmt.Errorf("readback payload: %d chains, at most %d", len(p.Chains), ReadbackRows)
	}
	rows := map[int]bool{}
	for _, ch := range p.Chains {
		if err := validateChain(ch, rows); err != nil {
			return err
		}
	}
	if p.Bypass != nil {
		if err := validateBypass(p.Bypass); err != nil {
			return err
		}
	}
	if p.Setlist.Folder != "My Presets" && p.Setlist.Folder != "other" {
		return errors.New(`readback payload: setlist.folder must be "My Presets" or "other"`)
	}
	return nil
}

func validateChain(ch ReadbackChain, rows map[int]bool) error {
	if ch.Row < 0 || ch.Row >= ReadbackRows || rows[ch.Row] {
		return errors.New("readback payload: chain rows must be unique values 0-3")
	}
	rows[ch.Row] = true
	if ch.Blocks == nil || len(ch.Blocks) > ReadbackCols {
		return errors.New("readback payload: a chain needs a blocks array of at most 8")
	}
	cols := map[int]bool{}
	for _, b := range ch.Blocks {
		if err := validateBlock(b, cols); err != nil {
			return err
		}
	}
	return nil
}

func validateBlock(b ReadbackPlaced, cols map[int]bool) error {
	if b.Col < 0 || b.Col >= ReadbackCols || cols[b.Col] {
		return errors.New("readback payload: block columns must be unique values 0-7")
	}
	cols[b.Col] = true
	if b.ModelID < 0 || b.ModelID > MaxReadbackModelID {
		return errors.New("readback payload: model_id out of range")
	}
	if b.Params == nil || len(b.Params) > MaxReadbackParams {
		return fmt.Errorf("readback payload: a block needs a params array of at most %d", MaxReadbackParams)
	}
	idx := map[int]bool{}
	for _, pr := range b.Params {
		if err := validateParam(pr, idx); err != nil {
			return err
		}
	}
	return nil
}

func validateParam(pr ReadbackParam, idx map[int]bool) error {
	if pr.Index < 0 || pr.Index >= MaxReadbackParams || idx[pr.Index] {
		return errors.New("readback payload: parameter indexes must be unique values in range")
	}
	idx[pr.Index] = true
	if len(pr.Values) != ReadbackScenes {
		return fmt.Errorf("readback payload: a parameter has %d values, want %d (scenes A-H)", len(pr.Values), ReadbackScenes)
	}
	for _, v := range pr.Values {
		switch x := v.(type) {
		case nil, bool, json.Number:
		case string:
			if utf8.RuneCountInString(x) > MaxReadbackStringRn {
				return errors.New("readback payload: a text value is too long")
			}
		default:
			return errors.New("readback payload: a parameter value must be a number, string, boolean or null")
		}
	}
	return nil
}

func validateBypass(b [][][]bool) error {
	if len(b) != ReadbackRows {
		return fmt.Errorf("readback payload: bypass has %d rows, want %d", len(b), ReadbackRows)
	}
	for _, row := range b {
		if len(row) != ReadbackCols {
			return fmt.Errorf("readback payload: a bypass row has %d columns, want %d", len(row), ReadbackCols)
		}
		for _, col := range row {
			if len(col) != ReadbackScenes {
				return fmt.Errorf("readback payload: a bypass column has %d scenes, want %d", len(col), ReadbackScenes)
			}
		}
	}
	return nil
}

// ---- the importer's seam ----

// ReadbackMeta is what the tool knows about a read besides the payload.
type ReadbackMeta struct {
	JobID    string
	Slot     string // as the agent reported it ("" when none)
	Name     string // preset name as the agent reported it, untrusted
	Firmware string
}

// ImportedPreset is the importer's result: the draft's chain, and an account of every
// control it could not convert. All strings are already sanitized.
type ImportedPreset struct {
	Preset      storage.StructuredPreset
	Guitar      string
	Name        string
	Blocks      int
	Converted   int
	AtDefault   int
	Unconverted []Unconverted // capped; UnconvertedCount is the true number
	// UnconvertedCount counts every item left out, including those past the cap.
	UnconvertedCount int
	Notes            []string
}

// Unconverted is one thing the importer left out, with the reason.
type Unconverted struct {
	Block  string `json:"block"`
	Param  string `json:"param,omitempty"`
	Reason string `json:"reason"`
}

// ReadbackImporter turns a read-back into a StructuredPreset. internal/deviceexport
// implements it; the tool depends only on this interface.
type ReadbackImporter interface {
	ImportReadback(ctx context.Context, rb *ReadbackPayload, meta ReadbackMeta) (*ImportedPreset, error)
}
