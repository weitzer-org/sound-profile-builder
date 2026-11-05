package devicejobs

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
)

// Payload type strings and the payload version (contract §7, §9, §10).
const (
	PayloadVersion = 1
	TypeJob        = "qc2-device-job"
	TypeResult     = "qc2-device-result"
	TypeHeartbeat  = "qc2-device-heartbeat"
)

// Job kinds.
const (
	KindPushPreset = "push_preset"
	KindListSlots  = "list_slots"
	KindReadPreset = "read_preset"
)

// Result statuses.
const (
	StatusSucceeded = "succeeded"
	StatusFailed    = "failed"
	StatusRefused   = "refused"
	StatusExpired   = "expired"
)

// Job lifetime rules (contract §7-§8).
const (
	DefaultJobTTL   = 15 * time.Minute
	MaxJobLifetime  = 24 * time.Hour
	MaxCreatedAhead = 5 * time.Minute
)

// MaxExportBytes is QC-2's own cap on an export payload (Export.Validate).
// Agents do not measure it separately; the whole-job cap bounds them.
const MaxExportBytes = 256 << 10

// TimeLayout is the only timestamp form in the contract: UTC, whole seconds, "Z".
const TimeLayout = "2006-01-02T15:04:05Z"

var (
	timeRe = regexp.MustCompile(`^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}Z$`)
	// JobIDRe is a lowercase canonical UUID (any version).
	JobIDRe   = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
	pendingRe = regexp.MustCompile(`^jobs/pending/([0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12})\.json$`)
	slotRe    = regexp.MustCompile(`^(?:[1-9]|[12][0-9]|3[0-2])[A-H]$`)
	bucketRe  = regexp.MustCompile(`^[a-z0-9][a-z0-9.-]{1,61}[a-z0-9]$`)
	fmtRe     = regexp.MustCompile(`^[a-z0-9][a-z0-9.-]{0,40}/[0-9]{1,4}$`)
)

// FormatTime renders t in the contract's timestamp form.
func FormatTime(t time.Time) string { return t.UTC().Format(TimeLayout) }

// ParseTime parses a contract timestamp, rejecting any other form.
func ParseTime(s string) (time.Time, error) {
	if !timeRe.MatchString(s) {
		return time.Time{}, fmt.Errorf("timestamp %q is not YYYY-MM-DDTHH:MM:SSZ", truncate(s, 40))
	}
	t, err := time.Parse(TimeLayout, s)
	if err != nil {
		return time.Time{}, err
	}
	if t.Year() < 1 {
		return time.Time{}, fmt.Errorf("timestamp %q is before year 1", s)
	}
	return t, nil
}

// Object keys in the jobs bucket (contract §3).
const (
	PendingPrefix = "jobs/pending/"
	DonePrefix    = "jobs/done/"
	HeartbeatKey  = "agent/heartbeat.json"
	LibraryKey    = "agent/library.json"    // reserved (v2)
	OwnedSlotsKey = "jobs/owned_slots.json" // agent-written, informational
)

// PendingKey and DoneKey build object keys; id must already satisfy JobIDRe.
func PendingKey(id string) string { return PendingPrefix + id + ".json" }
func DoneKey(id string) string    { return DonePrefix + id + ".json" }

// ValidJobID reports whether id is a lowercase canonical UUID.
func ValidJobID(id string) bool { return JobIDRe.MatchString(id) }

// NormalizeSlot upper-cases and validates a My Presets slot such as "20A".
func NormalizeSlot(s string) (string, bool) {
	s = strings.ToUpper(strings.TrimSpace(s))
	return s, slotRe.MatchString(s)
}

// ValidBucketName applies the S3/R2 bucket-name shape.
func ValidBucketName(b string) bool { return bucketRe.MatchString(b) }

// ValidDeviceName reports whether name is 1-32 printable ASCII characters with
// no leading or trailing space (contract §7; the device's own limit is UNVERIFIED).
func ValidDeviceName(name string) bool {
	if name == "" || len(name) > 32 || strings.TrimSpace(name) != name {
		return false
	}
	for i := 0; i < len(name); i++ {
		if name[i] < 0x20 || name[i] > 0x7E {
			return false
		}
	}
	return true
}

// ---- job ----

// Job is the signed job payload (contract §7). Unknown fields are an error.
type Job struct {
	V         int             `json:"v"`
	Type      string          `json:"type"`
	JobID     string          `json:"job_id"`
	Bucket    string          `json:"bucket"`
	Kind      string          `json:"kind"`
	CreatedAt string          `json:"created_at"`
	ExpiresAt string          `json:"expires_at"`
	Args      json.RawMessage `json:"args"`
}

// PushArgs are a push_preset job's args.
type PushArgs struct {
	Slot   string       `json:"slot"`
	Name   string       `json:"name"`
	Source PushSource   `json:"source"`
	Export ExportRecord `json:"export"`
}

// PushSource names the stored preset the export came from (informational).
type PushSource struct {
	PresetID string `json:"preset_id"`
	Guitar   string `json:"guitar"`
}

// ExportRecord is the device export carried in a push job: a versioned,
// opaque JSON object defined by P2 (internal/deviceexport).
type ExportRecord struct {
	Format  string          `json:"format"`
	Payload json.RawMessage `json:"payload"`
}

// ReadArgs are a read_preset job's args. Slot must be null in v1.
type ReadArgs struct {
	Slot *string `json:"slot"`
}

// object requires raw to be a JSON object whose keys are exactly keys
// (case-sensitive: Go's struct decoding would fold case, Python's would not).
func object(raw json.RawMessage, keys ...string) (map[string]json.RawMessage, error) {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || raw[0] != '{' {
		return nil, errors.New("not a JSON object")
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, errors.New("not a JSON object")
	}
	if len(m) != len(keys) {
		return nil, fmt.Errorf("want exactly the keys %v", keys)
	}
	for _, k := range keys {
		if _, ok := m[k]; !ok {
			return nil, fmt.Errorf("missing %q", k)
		}
	}
	return m, nil
}

// str requires raw to be a JSON string (null is not a string).
func str(raw json.RawMessage, field string) (string, error) {
	var s string
	if len(raw) == 0 || raw[0] != '"' || json.Unmarshal(raw, &s) != nil {
		return "", fmt.Errorf("%s must be a string", field)
	}
	return s, nil
}

// ParseJob strictly parses and shape-checks a verified job payload (contract
// §7): nesting depth, no duplicate keys, no unpaired surrogate escapes, the
// exact key set at every level (case-sensitive), every field's JSON type
// (null is never a string), known type/kind/version, contract timestamps, and
// args valid for the kind. It does not check bindings, replay or time against
// a clock; CheckPendingJob does.
func ParseJob(payload []byte) (*Job, error) {
	if err := checkStrictJSON(payload); err != nil {
		return nil, fail(OutcomeBadPayload, "%v", err)
	}
	if hasLoneSurrogate(payload) {
		return nil, fail(OutcomeBadPayload, "unpaired surrogate escape")
	}
	m, err := object(payload, "v", "type", "job_id", "bucket", "kind", "created_at", "expires_at", "args")
	if err != nil {
		return nil, fail(OutcomeBadPayload, "%v", err)
	}
	if string(m["v"]) != "1" {
		return nil, fail(OutcomeBadPayload, "v must be the integer 1")
	}
	j := Job{V: 1, Args: m["args"]}
	for _, f := range []struct {
		name string
		dst  *string
	}{{"type", &j.Type}, {"job_id", &j.JobID}, {"bucket", &j.Bucket}, {"kind", &j.Kind}, {"created_at", &j.CreatedAt}, {"expires_at", &j.ExpiresAt}} {
		if *f.dst, err = str(m[f.name], f.name); err != nil {
			return nil, fail(OutcomeBadPayload, "%v", err)
		}
	}
	switch {
	case j.Type != TypeJob:
		return nil, fail(OutcomeBadPayload, "type must be %q", TypeJob)
	case !ValidJobID(j.JobID):
		return nil, fail(OutcomeBadPayload, "job_id must be a lowercase canonical UUID")
	case !ValidBucketName(j.Bucket):
		return nil, fail(OutcomeBadPayload, "bad bucket name")
	}
	if _, err := ParseTime(j.CreatedAt); err != nil {
		return nil, fail(OutcomeBadPayload, "created_at: %v", err)
	}
	if _, err := ParseTime(j.ExpiresAt); err != nil {
		return nil, fail(OutcomeBadPayload, "expires_at: %v", err)
	}
	if err := checkArgs(j.Kind, j.Args); err != nil {
		return nil, fail(OutcomeBadPayload, "args: %v", err)
	}
	return &j, nil
}

func checkArgs(kind string, args json.RawMessage) error {
	switch kind {
	case KindListSlots:
		_, err := object(args)
		return err
	case KindReadPreset:
		m, err := object(args, "slot")
		if err != nil {
			return err
		}
		if string(m["slot"]) == "null" {
			return nil
		}
		s, err := str(m["slot"], "slot")
		if err != nil {
			return err
		}
		if !slotRe.MatchString(s) {
			return errors.New("bad slot")
		}
	case KindPushPreset:
		m, err := object(args, "slot", "name", "source", "export")
		if err != nil {
			return err
		}
		slot, err := str(m["slot"], "slot")
		if err != nil {
			return err
		}
		if !slotRe.MatchString(slot) {
			return errors.New("bad slot")
		}
		name, err := str(m["name"], "name")
		if err != nil {
			return err
		}
		if !ValidDeviceName(name) {
			return errors.New("bad name")
		}
		src, err := object(m["source"], "preset_id", "guitar")
		if err != nil {
			return fmt.Errorf("source: %v", err)
		}
		for _, f := range []string{"preset_id", "guitar"} {
			if _, err := str(src[f], "source."+f); err != nil {
				return err
			}
		}
		exp, err := object(m["export"], "format", "payload")
		if err != nil {
			return fmt.Errorf("export: %v", err)
		}
		format, err := str(exp["format"], "export.format")
		if err != nil {
			return err
		}
		if !fmtRe.MatchString(format) {
			return errors.New("bad export format")
		}
		if p := bytes.TrimSpace(exp["payload"]); len(p) == 0 || p[0] != '{' {
			return errors.New("export.payload must be a JSON object")
		}
	default:
		return fmt.Errorf("unknown kind %q", truncate(kind, 40))
	}
	return nil
}

// Times returns the job's parsed created_at and expires_at.
func (j *Job) Times() (created, expires time.Time) {
	created, _ = ParseTime(j.CreatedAt)
	expires, _ = ParseTime(j.ExpiresAt)
	return
}

// ---- result ----

// Result is the agent's signed result payload (contract §9). QC-2 reads it
// leniently (unknown fields ignored); every string in it is untrusted.
type Result struct {
	V            int            `json:"v"`
	Type         string         `json:"type"`
	JobID        string         `json:"job_id"`
	Bucket       string         `json:"bucket"`
	Kind         string         `json:"kind"`
	Status       string         `json:"status"`
	ReasonCode   string         `json:"reason_code"`
	Message      string         `json:"message"`
	StartedAt    string         `json:"started_at"`
	FinishedAt   string         `json:"finished_at"`
	AgentVersion string         `json:"agent_version"`
	Firmware     *string        `json:"firmware"`
	Push         *PushResult    `json:"push"`
	Slots        *SlotsResult   `json:"slots"`
	Readback     *ReadbackBlock `json:"readback"`
}

// PushResult reports a push's read-back comparison.
type PushResult struct {
	Slot       string     `json:"slot"`
	Name       string     `json:"name"`
	Mismatches int        `json:"mismatches"`
	CPULoad    *float64   `json:"cpu_load"`
	Diff       []DiffItem `json:"diff"`
}

// DiffItem compares one written parameter with its read-back value.
type DiffItem struct {
	Row        int      `json:"row"`
	Col        int      `json:"col"`
	ParamIndex int      `json:"param_index"`
	Expected   float64  `json:"expected"`
	Readback   *float64 `json:"readback"`
	OK         bool     `json:"ok"`
}

// SlotsResult is a list_slots answer.
type SlotsResult struct {
	Setlist string     `json:"setlist"`
	Slots   []SlotInfo `json:"slots"`
}

// SlotInfo is one My Presets slot.
type SlotInfo struct {
	Slot  string  `json:"slot"`
	State string  `json:"state"` // "empty" | "occupied"
	Name  *string `json:"name"`
	Owned bool    `json:"owned"`
}

// ReadbackBlock is a read_preset answer. Payload is opaque until P5 defines
// "qc-device-readback/1".
type ReadbackBlock struct {
	Format  string          `json:"format"`
	Slot    *string         `json:"slot"`
	Name    *string         `json:"name"`
	Payload json.RawMessage `json:"payload"`
}

// Reason codes an agent may put in a result.
var ReasonCodes = map[string]bool{
	"": true, "firmware_mismatch": true, "unit_absent": true, "unit_busy": true,
	"slot_not_allowed": true, "slot_not_empty": true, "validation": true,
	"unsupported_kind": true, "unsupported_arg": true, "device_error": true,
	"readback_mismatch": true, "interrupted": true, "expired": true,
}

// ---- heartbeat ----

// Heartbeat is the agent's signed liveness record (contract §10).
type Heartbeat struct {
	V                int        `json:"v"`
	Type             string     `json:"type"`
	Bucket           string     `json:"bucket"`
	AgentVersion     string     `json:"agent_version"`
	StartedAt        string     `json:"started_at"`
	WrittenAt        string     `json:"written_at"`
	IdleExitAt       string     `json:"idle_exit_at"`
	PollIntervalS    int        `json:"poll_interval_s"`
	State            string     `json:"state"` // idle | busy | exiting
	CurrentJobID     *string    `json:"current_job_id"`
	LastJob          *LastJob   `json:"last_job"`
	Unit             UnitStatus `json:"unit"`
	ExpectedFirmware string     `json:"expected_firmware"`
	AllowedSlots     []string   `json:"allowed_slots"`
	Setlist          string     `json:"setlist"`
}

// LastJob summarises the agent's most recent job.
type LastJob struct {
	JobID      string `json:"job_id"`
	Status     string `json:"status"`
	FinishedAt string `json:"finished_at"`
}

// UnitStatus is what the agent last saw of the Quad Cortex.
type UnitStatus struct {
	Present        *bool   `json:"present"`
	Firmware       *string `json:"firmware"`
	FirmwareSeenAt *string `json:"firmware_seen_at"`
}
