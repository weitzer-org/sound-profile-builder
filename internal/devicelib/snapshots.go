package devicelib

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/weitzer-org/sound-builder/internal/devicejobs"
)

// Snapshots (preset-editing plan E1, section 3.1): every succeeded read_preset result QC-2
// parses is kept raw as device/snapshots/<job_id>.json in the main bucket, whether or not
// the user asked to import it. An imported draft's Origin.JobID names its snapshot, so a
// refine never loses the baseline an edit is computed against.
//
// The payload is the qc-device-readback/1 object exactly as the bridge signed it. It is
// stored raw on purpose: the edit plan's compare-and-swap needs the unit's exact strings
// (preset name, string parameters). Sanitising happens on every output instead. Everything
// in it is untrusted device data, re-parsed strictly on every load.
const (
	SnapshotPrefix = "device/snapshots/"
	SnapshotFormat = "qc2-device-snapshot/1"
	// maxSnapshotBytes bounds a stored snapshot: the payload cap plus the envelope.
	maxSnapshotBytes = devicejobs.MaxReadbackBytes + 4<<10
)

// ReadSnapshot is one stored read of the unit's open preset.
type ReadSnapshot struct {
	Format   string `json:"format"`
	JobID    string `json:"job_id"`
	Slot     string `json:"slot"`     // as the agent reported it ("" when none)
	Name     string `json:"name"`     // preset name as the agent reported it, untrusted
	Firmware string `json:"firmware"` // "" when none
	ReadAt   string `json:"read_at"`  // contract timestamp of the result's finish
	// ReadbackFormat is the payload's format string (always devicejobs.ReadbackFormat here).
	ReadbackFormat string          `json:"readback_format"`
	Payload        json.RawMessage `json:"payload"`
}

// SnapshotObject is the object key for a job id.
func SnapshotObject(jobID string) string { return SnapshotPrefix + jobID + ".json" }

// NewReadSnapshot builds a snapshot from a succeeded read_preset result. It checks the
// payload with ParseReadbackPayload, so only a payload this server can read again is
// stored.
func NewReadSnapshot(v *devicejobs.JobView) (*ReadSnapshot, error) {
	if v == nil || v.Result == nil || v.State != devicejobs.StatusSucceeded {
		return nil, errors.New("not a succeeded job")
	}
	r := v.Result
	if r.Kind != devicejobs.KindReadPreset || r.Readback == nil {
		return nil, errors.New("not a read of the open preset")
	}
	if !devicejobs.ValidJobID(v.JobID) || r.JobID != v.JobID {
		return nil, errors.New("the result does not belong to this job")
	}
	if r.Readback.Format != devicejobs.ReadbackFormat {
		return nil, fmt.Errorf("read-back format is not %s", devicejobs.ReadbackFormat)
	}
	if _, err := devicejobs.ParseReadbackPayload(r.Readback.Payload); err != nil {
		return nil, err
	}
	if _, err := devicejobs.ParseTime(r.FinishedAt); err != nil {
		return nil, errors.New("the result has no valid finish time")
	}
	s := &ReadSnapshot{
		Format: SnapshotFormat, JobID: v.JobID, ReadAt: r.FinishedAt, ReadbackFormat: r.Readback.Format,
		Payload: append(json.RawMessage(nil), r.Readback.Payload...),
	}
	if r.Readback.Slot != nil {
		s.Slot = *r.Readback.Slot
	}
	if r.Readback.Name != nil {
		s.Name = *r.Readback.Name
	}
	if r.Firmware != nil {
		s.Firmware = *r.Firmware
	}
	return s, nil
}

// Validate checks a snapshot loaded from storage (an object the bucket's writers could
// have altered): format, job id, time, and the payload's strict shape.
func (s *ReadSnapshot) Validate(wantJobID string) error {
	bad := errors.New("the stored snapshot is malformed")
	if s.Format != SnapshotFormat || s.JobID != wantJobID || !devicejobs.ValidJobID(s.JobID) || s.ReadbackFormat != devicejobs.ReadbackFormat {
		return bad
	}
	if _, err := devicejobs.ParseTime(s.ReadAt); err != nil {
		return bad
	}
	if len(s.Slot) > 8 || len([]rune(s.Name)) > 128 || len([]rune(s.Firmware)) > 64 {
		return bad
	}
	if _, err := devicejobs.ParseReadbackPayload(s.Payload); err != nil {
		return bad
	}
	return nil
}

// Readback decodes the stored payload.
func (s *ReadSnapshot) Readback() (*devicejobs.ReadbackPayload, error) {
	return devicejobs.ParseReadbackPayload(s.Payload)
}

// SaveSnapshot stores a snapshot unless one already exists for the job. A job id names one
// read forever, so a second write would carry identical bytes; skipping it keeps a stored
// snapshot from ever being replaced. The existence check is not atomic with the write: two
// concurrent callers both write the same content, which is harmless. It reports whether
// this call wrote.
func (s *Store) SaveSnapshot(ctx context.Context, snap *ReadSnapshot) (bool, error) {
	if err := snap.Validate(snap.JobID); err != nil {
		return false, err
	}
	var existing ReadSnapshot
	ok, err := s.readJSON(ctx, SnapshotObject(snap.JobID), &existing)
	if err != nil {
		return false, err
	}
	if ok {
		return false, nil
	}
	data, err := json.Marshal(snap)
	if err != nil {
		return false, err
	}
	if len(data) > maxSnapshotBytes {
		return false, errors.New("the snapshot is over its size limit")
	}
	return true, s.client.WriteFile(ctx, s.bucket, SnapshotObject(snap.JobID), data)
}

// ReadSnapshot returns the stored snapshot of a read job, or nil when none is stored. A
// stored object that fails validation is an error, never "not there".
func (s *Store) ReadSnapshot(ctx context.Context, jobID string) (*ReadSnapshot, error) {
	if !devicejobs.ValidJobID(jobID) || strings.ContainsAny(jobID, "/.") {
		return nil, errors.New("not a job id")
	}
	var snap ReadSnapshot
	ok, err := s.readJSON(ctx, SnapshotObject(jobID), &snap)
	if err != nil || !ok {
		return nil, err
	}
	if err := snap.Validate(jobID); err != nil {
		return nil, err
	}
	return &snap, nil
}
