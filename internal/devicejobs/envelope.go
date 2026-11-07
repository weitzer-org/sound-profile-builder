package devicejobs

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"regexp"
	"strconv"
	"unicode/utf8"
)

// Domain tags. Each message type is signed under its own tag, so a valid
// signature for one type never verifies as another (contract §5).
const (
	TagJob = "qc2-device-job/v1"
	// TagJobV2 signs a version 2 job (edit_preset, contract section 7.1). The v1 kinds stay under
	// TagJob, byte for byte.
	TagJobV2      = "qc2-device-job/v2"
	TagResult     = "qc2-device-result/v1"
	TagHeartbeat  = "qc2-device-heartbeat/v1"
	TagOwnedSlots = "qc2-device-owned-slots/v1" // reserved for the agent's informational copy
)

// EnvelopeVersion is the only envelope "v" this code reads or writes.
const EnvelopeVersion = 1

// Size caps on whole stored objects (contract §3).
const (
	MaxJobBytes       = 512 << 10
	MaxResultBytes    = 2 << 20 // a sync_device_library result carries up to MaxLibraryCaptures entries
	MaxHeartbeatBytes = 8 << 10
)

// Outcome names a verification result. The strings are shared with the Python
// agent through testdata/devicejobs/vectors.json ("expect").
type Outcome string

const (
	OutcomeOK           Outcome = "ok"
	OutcomeBadKey       Outcome = "bad_object_key" // object key is not jobs/pending/<uuid>.json
	OutcomeTooLarge     Outcome = "too_large"
	OutcomeBadEnvelope  Outcome = "bad_envelope"
	OutcomeUnknownKID   Outcome = "unknown_kid"
	OutcomeBadSignature Outcome = "bad_signature"
	OutcomeBadPayload   Outcome = "bad_payload"
	OutcomeMisrouted    Outcome = "misrouted"
	OutcomeReplay       Outcome = "replay"
	OutcomeExpired      Outcome = "expired"
)

// VerifyError carries the Outcome of a failed check. Its message never
// includes key material, signatures or payload text.
type VerifyError struct {
	Outcome Outcome
	Detail  string
}

func (e *VerifyError) Error() string {
	if e.Detail == "" {
		return string(e.Outcome)
	}
	return string(e.Outcome) + ": " + e.Detail
}

func fail(o Outcome, format string, args ...any) error {
	return &VerifyError{Outcome: o, Detail: fmt.Sprintf(format, args...)}
}

// OutcomeOf returns err's Outcome, OutcomeOK for nil, and "" for an error that
// is not a VerifyError.
func OutcomeOf(err error) Outcome {
	if err == nil {
		return OutcomeOK
	}
	var ve *VerifyError
	if errors.As(err, &ve) {
		return ve.Outcome
	}
	return ""
}

var (
	kidRe = regexp.MustCompile(`^[0-9a-f]{8}$`)
	sigRe = regexp.MustCompile(`^[0-9a-f]{64}$`)
)

// envelope is the stored object: exactly these four keys.
type envelope struct {
	V       int    `json:"v"`
	KID     string `json:"kid"`
	Payload string `json:"payload"`
	Sig     string `json:"sig"`
}

func mac(key []byte, tag string, payload []byte) []byte {
	m := hmac.New(sha256.New, key)
	m.Write([]byte(tag))
	m.Write([]byte{'\n'})
	m.Write(payload)
	return m.Sum(nil)
}

// Sign returns the lowercase-hex HMAC-SHA256 of tag || "\n" || payload under
// key. Exposed for the test vectors; callers use Seal.
func Sign(key []byte, tag string, payload []byte) string {
	return hex.EncodeToString(mac(key, tag, payload))
}

// Seal wraps payload (JSON text produced by the caller) in a signed envelope
// under the current key. payload is carried as an exact string, so verifiers
// never re-serialize anything.
func (k *Keyring) Seal(tag string, payload []byte) ([]byte, error) {
	if !utf8.Valid(payload) || !json.Valid(payload) {
		return nil, errors.New("devicejobs: payload must be valid UTF-8 JSON")
	}
	return json.Marshal(envelope{
		V: EnvelopeVersion, KID: k.cur.kid, Payload: string(payload), Sig: Sign(k.cur.key, tag, payload),
	})
}

// Open verifies an envelope under tag and returns the signed payload bytes.
// Checks, in contract §5 order: strict envelope shape, known kid, then the MAC
// in constant time. The payload is not parsed here.
func (k *Keyring) Open(tag string, data []byte) ([]byte, error) {
	env, err := parseEnvelope(data)
	if err != nil {
		return nil, err
	}
	key, ok := k.lookup(env.KID)
	if !ok {
		return nil, fail(OutcomeUnknownKID, "kid %s is not DEVICE_JOB_KEY or DEVICE_JOB_KEY_NEXT", env.KID)
	}
	want := mac(key, tag, []byte(env.Payload))
	got, _ := hex.DecodeString(env.Sig) // sigRe already guarantees valid hex
	if !hmac.Equal(want, got) {
		return nil, fail(OutcomeBadSignature, "")
	}
	return []byte(env.Payload), nil
}

// OpenJob verifies a job envelope and strictly parses its payload (contract section 8, steps
// 3-6). The MAC is computed under both job tags and both are compared in constant time; the tag
// that verifies decides the payload version (TagJob: v 1, the v1 kinds; TagJobV2: v 2,
// edit_preset). Neither verifying is bad_signature. Domain separation means at most one can.
func (k *Keyring) OpenJob(data []byte) (*Job, error) {
	env, err := parseEnvelope(data)
	if err != nil {
		return nil, err
	}
	key, ok := k.lookup(env.KID)
	if !ok {
		return nil, fail(OutcomeUnknownKID, "kid %s is not DEVICE_JOB_KEY or DEVICE_JOB_KEY_NEXT", env.KID)
	}
	got, _ := hex.DecodeString(env.Sig)
	v1 := hmac.Equal(mac(key, TagJob, []byte(env.Payload)), got)
	v2 := hmac.Equal(mac(key, TagJobV2, []byte(env.Payload)), got)
	switch {
	case v1:
		return ParseJob([]byte(env.Payload))
	case v2:
		return ParseJobV2([]byte(env.Payload))
	}
	return nil, fail(OutcomeBadSignature, "")
}

// JobTag is the tag a job of version v is signed under.
func JobTag(v int) string {
	if v == PayloadVersionEdit {
		return TagJobV2
	}
	return TagJob
}

// parseEnvelope enforces the strict envelope rules: valid UTF-8, one JSON
// object with exactly v/kid/payload/sig and no duplicates, v the integer 1,
// kid and sig lowercase hex of the right length, payload a string with no
// unpaired UTF-16 surrogate escape.
func parseEnvelope(data []byte) (*envelope, error) {
	if !utf8.Valid(data) {
		return nil, fail(OutcomeBadEnvelope, "not valid UTF-8")
	}
	if err := checkStrictJSON(data); err != nil {
		return nil, fail(OutcomeBadEnvelope, "%v", err)
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fail(OutcomeBadEnvelope, "not a JSON object")
	}
	if len(raw) != 4 {
		return nil, fail(OutcomeBadEnvelope, "envelope must have exactly the keys v, kid, payload, sig")
	}
	for _, f := range []string{"v", "kid", "payload", "sig"} {
		if _, ok := raw[f]; !ok {
			return nil, fail(OutcomeBadEnvelope, "missing %q", f)
		}
	}
	if string(raw["v"]) != "1" {
		return nil, fail(OutcomeBadEnvelope, "v must be the integer 1")
	}
	var env envelope
	env.V = 1
	if err := json.Unmarshal(raw["kid"], &env.KID); err != nil || !kidRe.MatchString(env.KID) {
		return nil, fail(OutcomeBadEnvelope, "kid must be 8 lowercase hex characters")
	}
	if err := json.Unmarshal(raw["sig"], &env.Sig); err != nil || !sigRe.MatchString(env.Sig) {
		return nil, fail(OutcomeBadEnvelope, "sig must be 64 lowercase hex characters")
	}
	if len(raw["payload"]) == 0 || raw["payload"][0] != '"' {
		return nil, fail(OutcomeBadEnvelope, "payload must be a JSON string")
	}
	if hasLoneSurrogate(raw["payload"]) {
		// Go would decode it to U+FFFD and Python would fail to encode it:
		// reject it outright so both sides agree.
		return nil, fail(OutcomeBadEnvelope, "payload contains an unpaired surrogate escape")
	}
	if err := json.Unmarshal(raw["payload"], &env.Payload); err != nil {
		return nil, fail(OutcomeBadEnvelope, "payload must be a JSON string")
	}
	return &env, nil
}

// hasLoneSurrogate reports whether a raw JSON string literal contains a \uD800-
// \uDFFF escape that is not a high surrogate immediately followed by a low one.
func hasLoneSurrogate(lit []byte) bool {
	hexVal := func(b []byte) (int, bool) {
		if len(b) < 4 {
			return 0, false
		}
		var v int
		for _, c := range b[:4] {
			v <<= 4
			switch {
			case c >= '0' && c <= '9':
				v |= int(c - '0')
			case c >= 'a' && c <= 'f':
				v |= int(c-'a') + 10
			case c >= 'A' && c <= 'F':
				v |= int(c-'A') + 10
			default:
				return 0, false
			}
		}
		return v, true
	}
	for i := 0; i < len(lit); i++ {
		if lit[i] != '\\' {
			continue
		}
		if i+1 >= len(lit) {
			return false
		}
		if lit[i+1] != 'u' {
			i++ // skip the escaped character (handles \\ and \")
			continue
		}
		v, ok := hexVal(lit[i+2:])
		if !ok {
			return false // malformed; json.Unmarshal rejects it
		}
		switch {
		case v >= 0xD800 && v <= 0xDBFF:
			j := i + 6
			if j+1 < len(lit) && lit[j] == '\\' && lit[j+1] == 'u' {
				if w, ok := hexVal(lit[j+2:]); ok && w >= 0xDC00 && w <= 0xDFFF {
					i = j + 5
					continue
				}
			}
			return true
		case v >= 0xDC00 && v <= 0xDFFF:
			return true
		}
		i += 5
	}
	return false
}

// checkStrictJSON requires exactly one JSON value with no duplicate object keys
// at any depth. Go's decoder (like Python's) silently keeps the last duplicate,
// so duplicates are rejected explicitly: two parsers must never disagree about
// what a signed document says.
func checkStrictJSON(data []byte) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	if err := walkValue(dec, 1); err != nil {
		return err
	}
	if _, err := dec.Token(); err != io.EOF {
		return errors.New("trailing data after the JSON value")
	}
	return nil
}

// MaxDepth bounds JSON nesting in every signed document: an envelope is depth
// 1, and the deepest legitimate job (a push export) is about a dozen. Without
// it, a 512 KiB run of "[" costs Go hundreds of MB of stack and makes Python
// raise RecursionError (contract §5).
const MaxDepth = 32

func walkValue(dec *json.Decoder, depth int) error {
	tok, err := dec.Token()
	if err != nil {
		return errors.New("invalid JSON")
	}
	d, ok := tok.(json.Delim)
	if !ok {
		// Every number must be a finite float64: Go would otherwise accept
		// 1e400 inside an opaque object that Python rejects (or vice versa).
		if n, isNum := tok.(json.Number); isNum {
			if f, err := strconv.ParseFloat(string(n), 64); math.IsInf(f, 0) || (err != nil && !errors.Is(err, strconv.ErrRange)) {
				return fmt.Errorf("number %s is out of range", truncate(string(n), 20))
			}
		}
		return nil
	}
	if depth > MaxDepth {
		return fmt.Errorf("nested deeper than %d", MaxDepth)
	}
	switch d {
	case '{':
		seen := map[string]bool{}
		for dec.More() {
			kt, err := dec.Token()
			if err != nil {
				return errors.New("invalid JSON")
			}
			key, _ := kt.(string)
			if seen[key] {
				return fmt.Errorf("duplicate key %q", truncate(key, 40))
			}
			seen[key] = true
			if err := walkValue(dec, depth+1); err != nil {
				return err
			}
		}
	case '[':
		for dec.More() {
			if err := walkValue(dec, depth+1); err != nil {
				return err
			}
		}
	}
	if _, err := dec.Token(); err != nil { // closing delimiter
		return errors.New("invalid JSON")
	}
	return nil
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}
