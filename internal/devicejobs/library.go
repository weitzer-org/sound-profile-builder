package devicejobs

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"
	"unicode/utf8"
)

// The qc-device-library/1 payload (contract docs/design/device-job-contract.md section 9.2):
// the capture files on the unit, as the home agent's sync_device_library job lists them.
// The Python reference is qc_agent.captures.library_payload; testdata/devicejobs/library.json
// pins both sides.
//
// Everything in it is untrusted device text. ParseLibraryPayload checks the shape and
// refuses anything that sanitising would change, so a stored name is the very string the
// agent compared on the unit (a hostile name can't collide with a good one after clean-up).

// LibraryFormat is the format string the agent writes in Result.Library.Format.
const LibraryFormat = "qc-device-library/1"

// Limits of the payload. The agent leaves a bad entry out and counts it in Skipped; it
// fails the whole job rather than truncate a list that is too long, because a cut list
// would make a capture on the unit look missing.
const (
	MaxLibraryCaptures = 5000
	MaxLibraryName     = 64 // the agent's capture-name bound (export_validator._CAPTURE_NAME_RE)
	MaxLibraryFolder   = 40
	MaxLibraryBytes    = 2 << 20
)

// Capture kinds in a library entry: the folder the file sits in is a factory folder or not.
const (
	CaptureKindFactory = "factory"
	CaptureKindUser    = "user"
)

var (
	libKeyRe  = regexp.MustCompile(`^[0-9a-f]{64}$`)
	libNameRe = regexp.MustCompile("^[\\x20-\\x7e]{1,64}$")
)

// LibraryPayload is the payload object.
type LibraryPayload struct {
	Captures []LibraryCapture `json:"captures"`
	// Skipped counts capture files left out because their name, key or folder was not
	// something the bridge could compare exactly (not printable ASCII, changed by
	// clean-up, too long) and favorites views.
	Skipped int `json:"skipped"`
}

// LibraryCapture is one capture file on the unit.
type LibraryCapture struct {
	Name string `json:"name"`
	Key  string `json:"key"` // 64 lowercase hex characters
	// Version is 1 or 2 when the bridge could tell (a factory capture's from the nearest
	// "Factory Captures V1/V2" folder at or above its own, a user capture's from its
	// ProductData.version "1"/"2" taken literally), else null.
	Version *int   `json:"version"`
	Kind    string `json:"kind"`
	Folder  string `json:"folder"`
	// Cloud is true when the file sits in a Cortex Cloud (or plugin) folder: listed, but
	// not a file on the unit, so a push refuses it.
	Cloud bool `json:"cloud"`
}

// ParseLibraryPayload decodes and shape-checks a sync_device_library payload. It rejects
// unknown fields, trailing data, wrong types, counts over MaxLibraryCaptures and any text
// that is not printable ASCII (names) or that SanitizeText would change (folders). The
// error text is for logs and the tool's refusal message and never echoes payload text.
func ParseLibraryPayload(raw json.RawMessage) (*LibraryPayload, error) {
	if len(raw) == 0 {
		return nil, errors.New("library payload is empty")
	}
	if len(raw) > MaxLibraryBytes {
		return nil, fmt.Errorf("library payload is larger than %d bytes", MaxLibraryBytes)
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	var p LibraryPayload
	if err := dec.Decode(&p); err != nil {
		return nil, fmt.Errorf("library payload does not match %s: %s", LibraryFormat, shortErr(err))
	}
	if _, err := dec.Token(); err != io.EOF {
		return nil, errors.New("library payload has data after the object")
	}
	if p.Captures == nil {
		return nil, errors.New("library payload: captures is missing")
	}
	if len(p.Captures) > MaxLibraryCaptures {
		return nil, fmt.Errorf("library payload: %d captures, at most %d", len(p.Captures), MaxLibraryCaptures)
	}
	if p.Skipped < 0 || p.Skipped > 1<<20 {
		return nil, errors.New("library payload: skipped out of range")
	}
	for _, c := range p.Captures {
		if err := c.Validate(); err != nil {
			return nil, err
		}
	}
	return &p, nil
}

// Validate checks one entry as ParseLibraryPayload does.
func (c LibraryCapture) Validate() error {
	switch {
	case !libNameRe.MatchString(c.Name) || strings.TrimSpace(c.Name) != c.Name || strings.Contains(c.Name, "  ") || strings.Contains(c.Name, "`"):
		return errors.New("library payload: a capture name is not clean printable ASCII")
	case !libKeyRe.MatchString(c.Key):
		return errors.New("library payload: a capture key is not 64 lowercase hex characters")
	case c.Version != nil && *c.Version != 1 && *c.Version != 2:
		return errors.New("library payload: version must be 1, 2 or null")
	case c.Kind != CaptureKindFactory && c.Kind != CaptureKindUser:
		return errors.New(`library payload: kind must be "factory" or "user"`)
	case c.Folder == "" || utf8.RuneCountInString(c.Folder) > MaxLibraryFolder || SanitizeText(c.Folder, MaxLibraryFolder) != c.Folder:
		return errors.New("library payload: a folder name is empty, too long or not clean text")
	}
	return nil
}
