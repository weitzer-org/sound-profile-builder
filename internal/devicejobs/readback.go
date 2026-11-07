package devicejobs

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
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

// ReadbackFormat is the first read-back format: its setlist says only "My Presets" or "other".
// QC-2 keeps parsing it (stored snapshots and agents from before contract revision 2).
const ReadbackFormat = "qc-device-readback/1"

// ReadbackFormatV2 adds the open collection's key and factory flag to the setlist, and a
// position in any regular collection (contract section 9.1, revision 2026-11-07). Its shape is
// otherwise identical to /1.
const ReadbackFormatV2 = "qc-device-readback/2"

// IsReadbackFormat reports whether format is a read-back format this server parses.
func IsReadbackFormat(format string) bool {
	return format == ReadbackFormat || format == ReadbackFormatV2
}

// MaxFolderKeyLen bounds a collection key in a /2 read-back.
const MaxFolderKeyLen = 128

// ValidFolderKey reports whether k is a collection key a /2 read-back may carry: 1-128
// printable ASCII characters (0x20-0x7E), no backtick, no leading or trailing space, no double
// space. Such a key is already unchanged by SanitizeText (contract section 11), so it can be
// compared byte for byte with what the unit reports. Anything else is sent as null by the agent.
func ValidFolderKey(k string) bool {
	if k == "" || len(k) > MaxFolderKeyLen || k[0] == ' ' || k[len(k)-1] == ' ' || strings.Contains(k, "  ") {
		return false
	}
	for i := 0; i < len(k); i++ {
		if c := k[i]; c < 0x20 || c > 0x7E || c == '`' {
			return false
		}
	}
	return true
}

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

// MaxReadbackBytes bounds the payload before it is decoded: the result envelope is already
// read through MaxResultBytes, and the agent caps its own output below this, so a larger
// payload is not from this agent. It keeps the decoder's allocation proportional (defence in
// depth for a public parser).
const MaxReadbackBytes = 512 << 10

// ReadbackPayload is the payload object.
type ReadbackPayload struct {
	Setlist ReadbackSetlist `json:"setlist"`
	Chains  []ReadbackChain `json:"chains"`
	// Bypass is bypass[row][col][scene] (true = bypassed), or nil when the unit sent none.
	Bypass [][][]bool `json:"bypass"`
}

// ReadbackSetlist says where the open preset sits: Folder is "My Presets" or "other".
// FolderKey and IsFactory are only in a /2 read-back (nil in /1); FolderKey is also nil in /2
// when the unit's key did not pass ValidFolderKey.
type ReadbackSetlist struct {
	Folder    string  `json:"folder"`
	FolderKey *string `json:"folder_key,omitempty"`
	Position  *int    `json:"position"`
	IsFactory *bool   `json:"is_factory,omitempty"`
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

// ParseReadbackPayload parses a qc-device-readback/1 payload (ParseReadback with ReadbackFormat).
func ParseReadbackPayload(raw json.RawMessage) (*ReadbackPayload, error) {
	return ParseReadback(ReadbackFormat, raw)
}

// ParseReadback decodes and shape-checks a read_preset payload of the given format. It rejects
// an unknown format, unknown fields (a new agent field means a new format version), a setlist
// whose key set is not exactly the format's, trailing data, wrong types, and counts or indexes
// outside the device's geometry. The error text is for logs and the tool's refusal message and
// never echoes payload text.
func ParseReadback(format string, raw json.RawMessage) (*ReadbackPayload, error) {
	if !IsReadbackFormat(format) {
		return nil, errors.New("unknown read-back format")
	}
	if len(raw) == 0 {
		return nil, errors.New("readback payload is empty")
	}
	if len(raw) > MaxReadbackBytes {
		return nil, fmt.Errorf("readback payload is larger than %d bytes", MaxReadbackBytes)
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	dec.UseNumber() // for the []any values; typed ints below still decode as ints
	var p ReadbackPayload
	if err := dec.Decode(&p); err != nil {
		return nil, fmt.Errorf("readback payload does not match %s: %s", format, shortErr(err))
	}
	if _, err := dec.Token(); err != io.EOF {
		return nil, errors.New("readback payload has data after the object")
	}
	if err := checkSetlistKeys(format, raw); err != nil {
		return nil, err
	}
	if err := p.validate(format); err != nil {
		return nil, err
	}
	return &p, nil
}

// checkSetlistKeys requires the setlist's exact key set for the format: the struct decoder
// accepts any subset, and /2's keys must not appear in a /1 payload (or be missing from a /2).
func checkSetlistKeys(format string, raw json.RawMessage) error {
	var top struct {
		Setlist map[string]json.RawMessage `json:"setlist"`
	}
	if err := json.Unmarshal(raw, &top); err != nil || top.Setlist == nil {
		return errors.New("readback payload: setlist must be an object")
	}
	want := []string{"folder", "position"}
	if format == ReadbackFormatV2 {
		want = append(want, "folder_key", "is_factory")
	}
	if len(top.Setlist) != len(want) {
		return fmt.Errorf("readback payload: setlist must have exactly the keys %v", want)
	}
	for _, k := range want {
		if _, ok := top.Setlist[k]; !ok {
			return fmt.Errorf("readback payload: setlist must have exactly the keys %v", want)
		}
	}
	return nil
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

func (p *ReadbackPayload) validate(format string) error {
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
	if format == ReadbackFormatV2 {
		return validateSetlistV2(p.Setlist)
	}
	return nil
}

// validateSetlistV2: is_factory is a boolean; folder_key is null or a ValidFolderKey; position
// is null or 0-255, and non-null only for a non-factory preset whose collection key was sent.
func validateSetlistV2(s ReadbackSetlist) error {
	if s.IsFactory == nil {
		return errors.New("readback payload: setlist.is_factory must be a boolean")
	}
	if s.FolderKey != nil && !ValidFolderKey(*s.FolderKey) {
		return errors.New("readback payload: setlist.folder_key is not a valid collection key")
	}
	if s.Position != nil {
		if *s.Position < 0 || *s.Position >= SlotCount {
			return errors.New("readback payload: setlist.position must be 0-255")
		}
		if *s.IsFactory || s.FolderKey == nil {
			return errors.New("readback payload: setlist.position must be null for a factory preset or an unknown collection")
		}
	}
	return nil
}

// SlotCount is the number of slots in a regular preset collection (32 banks x A-H).
const SlotCount = 256

// SlotLabel names a 0-based position the way the unit shows it: bank p/8+1, letter A-H
// ("" when p is out of range). The inverse of slotIndex.
func SlotLabel(p int) string {
	if p < 0 || p >= SlotCount {
		return ""
	}
	return strconv.Itoa(p/8+1) + string(rune('A'+p%8))
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
