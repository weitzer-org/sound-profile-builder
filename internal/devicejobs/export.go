package devicejobs

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/weitzer-org/sound-builder/internal/storage"
)

// Exporter turns one guitar's chain of a stored preset into a device export.
// P2 (internal/deviceexport) implements it; until then production uses
// UnavailableExporter and tests use fixtures. P3 treats the payload as
// opaque: it only checks the envelope (format, JSON object, size), hashes it
// for the confirm token, and carries it in the push job.
type Exporter interface {
	Export(ctx context.Context, sp storage.StructuredPreset, guitar string) (*Export, error)
}

// Export is a versioned, opaque device export plus a human-readable summary.
type Export struct {
	// Format names the payload schema, e.g. "qc2-device-export/1".
	Format string
	// Payload is the export itself: a JSON object, at most MaxExportBytes.
	Payload json.RawMessage
	// Summary is the exporter's own readable plan, so the dry run can show
	// exactly what will be written without P3 parsing the payload.
	Summary []PlanBlock
	// Unresolved lists everything the exporter could not map exactly. A push
	// needs none.
	Unresolved []Unresolved
	// Stomps is the exporter's footswitch layout for the dry run (design doc
	// qc-stomp-layout.md S2). Optional and display-only: the assignments that are
	// pushed are the payload's own "stomps" array, which the plan hash covers.
	Stomps *PlanStomps
}

// PlanStomps is the readable stomp layout: where each block goes, and what was left
// alone and why. All switch behaviour is UNVERIFIED until gate G7.
type PlanStomps struct {
	Assigned []PlanStomp   `json:"assigned"`
	Skipped  []PlanSkipped `json:"skipped"`
	Warnings []string      `json:"warnings"`
}

// PlanStomp is one block with its switch.
type PlanStomp struct {
	Row    int    `json:"row"`
	Col    int    `json:"col"`
	Model  string `json:"model"`
	Role   string `json:"role"`
	Switch string `json:"switch"`
}

// PlanSkipped is a placed block that gets no switch.
type PlanSkipped struct {
	Row    int    `json:"row"`
	Col    int    `json:"col"`
	Model  string `json:"model"`
	Reason string `json:"reason"`
}

// PlanBlock is one block of the readable plan.
type PlanBlock struct {
	Row     int         `json:"row"`
	Col     int         `json:"col"`
	Model   string      `json:"model"`
	ModelID int         `json:"model_id"`
	Params  []PlanParam `json:"params"`
	BypassA bool        `json:"bypass_a"`
	BypassB bool        `json:"bypass_b"`
	// AtDefault names controls the preset sets to the device default, which are
	// therefore not written.
	AtDefault []string `json:"at_default,omitempty"`
}

// PlanParam is one parameter write: display values for Scenes A and B.
type PlanParam struct {
	Name string `json:"name"`
	A    string `json:"a"`
	B    string `json:"b,omitempty"`
}

// Unresolved is one item the exporter refused to map.
type Unresolved struct {
	Block  string `json:"block"`
	Param  string `json:"param,omitempty"`
	Reason string `json:"reason"`
}

// ErrExportUnavailable is what the production exporter returns until P2 lands.
var ErrExportUnavailable = errors.New("device export not available yet")

// UnavailableExporter is the production Exporter until P2 lands.
type UnavailableExporter struct{}

// Export always returns ErrExportUnavailable.
func (UnavailableExporter) Export(context.Context, storage.StructuredPreset, string) (*Export, error) {
	return nil, ErrExportUnavailable
}

// Validate checks the parts of an export P3 relies on.
func (e *Export) Validate() error {
	if !fmtRe.MatchString(e.Format) {
		return fmt.Errorf("export format %q is not <name>/<version>", truncate(e.Format, 40))
	}
	p := bytes.TrimSpace(e.Payload)
	if len(p) == 0 || p[0] != '{' || !json.Valid(p) {
		return errors.New("export payload must be a JSON object")
	}
	if len(p) > MaxExportBytes {
		return fmt.Errorf("export payload is %d bytes, over the %d limit", len(p), MaxExportBytes)
	}
	return nil
}

// compactPayload returns the payload in compact form, the form that is
// hashed and carried in the job.
func (e *Export) compactPayload() ([]byte, error) {
	var b bytes.Buffer
	if err := json.Compact(&b, e.Payload); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

// PlanSHA256 is SHA-256(format || "\n" || compact payload), lowercase hex. It
// binds a confirm token to the exact export the user saw in the dry run.
func (e *Export) PlanSHA256() (string, error) {
	p, err := e.compactPayload()
	if err != nil {
		return "", err
	}
	h := sha256.New()
	h.Write([]byte(e.Format))
	h.Write([]byte{'\n'})
	h.Write(p)
	return hex.EncodeToString(h.Sum(nil)), nil
}
