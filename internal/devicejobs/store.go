package devicejobs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/weitzer-org/sound-builder/internal/storage"
)

// Job states as QC-2 reports them. The last four are a verified result's
// status; the rest are derived.
const (
	StateQueued     = "queued"
	StateRunning    = "running"
	StateNotFound   = "not_found"
	StateUnverified = "unverified" // a result object exists but fails verification
	StateUnclaimed  = "expired_unclaimed"
)

// HeartbeatMaxAge and HeartbeatMaxAhead bound a fresh heartbeat (contract §10).
const (
	HeartbeatMaxAge   = 2 * time.Minute
	HeartbeatMaxAhead = time.Minute
)

// unclaimedGrace is how long after expires_at a never-picked-up job is still
// reported as queued (the agent may be writing its "expired" result).
const unclaimedGrace = 2 * time.Minute

// ErrJobExists is returned by Enqueue when the id is already pending or done.
var ErrJobExists = errors.New("a job with this id already exists")

// Store reads and writes the jobs bucket through storage.Client. It never
// deletes: the agent clears jobs/pending/ (contract §2).
type Store struct {
	client storage.Client
	bucket string
	keys   *Keyring
}

// NewStore binds a storage client to the jobs bucket.
func NewStore(client storage.Client, cfg *Config) *Store {
	return &Store{client: client, bucket: cfg.Bucket, keys: cfg.Keys}
}

// Bucket is the jobs bucket name (it is carried in every job).
func (s *Store) Bucket() string { return s.bucket }

// exists reports whether object exists. storage.Client has no stat call and
// no not-found error, and the S3 and GCS backends' ListFiles skip a key equal
// to the prefix, so list the key minus its ".json" and look for an exact match.
func (s *Store) exists(ctx context.Context, object string) (bool, error) {
	names, err := s.client.ListFiles(ctx, s.bucket, strings.TrimSuffix(object, ".json"))
	if err != nil {
		return false, err
	}
	for _, n := range names {
		if n == object {
			return true, nil
		}
	}
	return false, nil
}

// readLimited reads object and enforces a size cap. found=false means the
// object does not exist; a read error on an existing object is an error.
func (s *Store) readLimited(ctx context.Context, object string, max int) (data []byte, found bool, err error) {
	data, rerr := s.client.ReadFile(ctx, s.bucket, object)
	if rerr != nil {
		ok, lerr := s.exists(ctx, object)
		if lerr != nil {
			return nil, false, fmt.Errorf("reading %s: %w (listing: %v)", object, rerr, lerr)
		}
		if !ok {
			return nil, false, nil
		}
		return nil, true, fmt.Errorf("reading %s: %w", object, rerr)
	}
	if len(data) > max {
		return nil, true, fmt.Errorf("%s is %d bytes, over the %d limit", object, len(data), max)
	}
	return data, true, nil
}

// NewJob builds a job with the contract's lifetime.
func (s *Store) NewJob(id, kind string, args any, now time.Time) (*Job, error) {
	raw, err := json.Marshal(args)
	if err != nil {
		return nil, err
	}
	now = now.UTC().Truncate(time.Second)
	return &Job{
		V: PayloadVersion, Type: TypeJob, JobID: id, Bucket: s.bucket, Kind: kind,
		CreatedAt: FormatTime(now), ExpiresAt: FormatTime(now.Add(DefaultJobTTL)), Args: raw,
	}, nil
}

// Enqueue signs j and writes it to jobs/pending/<id>.json. It refuses an id
// that is already pending or done; the check is best effort (storage.Client
// has no conditional write), and the agent's ledger is what guarantees a job
// runs at most once.
func (s *Store) Enqueue(ctx context.Context, j *Job) error {
	payload, err := json.Marshal(j)
	if err != nil {
		return err
	}
	// Self-check: QC-2 must never write a job its own agent check rejects.
	if _, err := ParseJob(payload); err != nil {
		return fmt.Errorf("refusing to enqueue an invalid job: %w", err)
	}
	if len(payload) > MaxJobBytes-1024 {
		return fmt.Errorf("job is %d bytes, over the limit", len(payload))
	}
	for _, k := range []string{PendingKey(j.JobID), DoneKey(j.JobID)} {
		ok, err := s.exists(ctx, k)
		if err != nil {
			return err
		}
		if ok {
			return ErrJobExists
		}
	}
	env, err := s.keys.Seal(TagJob, payload)
	if err != nil {
		return err
	}
	return s.client.WriteFile(ctx, s.bucket, PendingKey(j.JobID), env)
}

// maxPendingScan bounds how many pending objects one call reads, so a flood
// of junk objects (written with the agent's bucket token) can't make every
// tool call download them all. QC-2 itself never queues more than
// MaxPendingJobs.
const maxPendingScan = 4 * MaxPendingJobs

// PendingJobs returns the verifiable pending jobs among the first
// maxPendingScan objects. Objects that fail verification are skipped (the
// agent deletes them).
func (s *Store) PendingJobs(ctx context.Context) ([]*Job, error) {
	names, err := s.client.ListFiles(ctx, s.bucket, PendingPrefix)
	if err != nil {
		return nil, err
	}
	if len(names) > maxPendingScan {
		names = names[:maxPendingScan]
	}
	var out []*Job
	for _, n := range names {
		m := pendingRe.FindStringSubmatch(n)
		if m == nil {
			continue
		}
		data, found, err := s.readLimited(ctx, n, MaxJobBytes)
		if err != nil || !found {
			continue
		}
		payload, err := s.keys.Open(TagJob, data)
		if err != nil {
			continue
		}
		j, err := ParseJob(payload)
		if err != nil || j.JobID != m[1] {
			continue
		}
		out = append(out, j)
	}
	return out, nil
}

// HeartbeatView is QC-2's judgement of agent/heartbeat.json.
type HeartbeatView struct {
	Online bool
	// Reason explains Online=false in words for the user (never raw agent text).
	Reason string
	// Heartbeat is the verified heartbeat, or nil. Its strings are untrusted.
	Heartbeat *Heartbeat
	Age       time.Duration
}

// Heartbeat reads and judges the agent's heartbeat (contract §10).
func (s *Store) Heartbeat(ctx context.Context, now time.Time) (HeartbeatView, error) {
	data, found, err := s.readLimited(ctx, HeartbeatKey, MaxHeartbeatBytes)
	if err != nil {
		if found {
			return HeartbeatView{Reason: "the bridge's heartbeat could not be read"}, nil
		}
		return HeartbeatView{}, err
	}
	if !found {
		return HeartbeatView{Reason: "the bridge has never reported in: start it on the PC (the \"Start QC bridge\" shortcut)"}, nil
	}
	payload, err := s.keys.Open(TagHeartbeat, data)
	if err != nil {
		if OutcomeOf(err) == OutcomeUnknownKID {
			return HeartbeatView{Reason: "the bridge signs with a key this server does not know (DEVICE_JOB_KEY differs between the server and the PC)"}, nil
		}
		return HeartbeatView{Reason: "the bridge's heartbeat failed verification"}, nil
	}
	var hb Heartbeat
	if err := json.Unmarshal(payload, &hb); err != nil || hb.V != PayloadVersion || hb.Type != TypeHeartbeat {
		return HeartbeatView{Reason: "the bridge's heartbeat is malformed"}, nil
	}
	if hb.Bucket != s.bucket {
		return HeartbeatView{Reason: "the bridge is configured for a different jobs bucket"}, nil
	}
	written, err := ParseTime(hb.WrittenAt)
	if err != nil {
		return HeartbeatView{Reason: "the bridge's heartbeat is malformed"}, nil
	}
	v := HeartbeatView{Heartbeat: &hb, Age: now.Sub(written)}
	switch {
	case v.Age < -HeartbeatMaxAhead:
		v.Reason = fmt.Sprintf("the bridge PC's clock is ahead of the server by about %d s; fix the PC's clock", int(-v.Age.Seconds()))
	case v.Age > HeartbeatMaxAge:
		v.Reason = "the bridge is not running (no heartbeat in the last 2 minutes): switch on the Quad Cortex and start the bridge on the PC"
	case hb.State == "exiting":
		v.Reason = "the bridge has shut down: start it again on the PC"
	default:
		v.Online = true
	}
	return v, nil
}

// JobView is a job's state as QC-2 reports it.
type JobView struct {
	JobID string
	State string
	// Job is the pending job, when it is still pending (QC-2's own, verified).
	Job *Job
	// Result is the verified result, when State is a result status.
	Result *Result
}

// JobState derives a job's state from the bucket (and the heartbeat, for
// "running"). hb may be nil.
func (s *Store) JobState(ctx context.Context, id string, hb *Heartbeat, now time.Time) (*JobView, error) {
	if !ValidJobID(id) {
		return nil, errors.New("invalid job id")
	}
	v := &JobView{JobID: id}
	data, found, err := s.readLimited(ctx, DoneKey(id), MaxResultBytes)
	if err != nil && !found {
		return nil, err
	}
	if found {
		if err != nil {
			v.State = StateUnverified
			return v, nil
		}
		r, ok := s.openResult(data, id)
		if !ok {
			v.State = StateUnverified
			return v, nil
		}
		v.State, v.Result = r.Status, r
		return v, nil
	}
	data, found, err = s.readLimited(ctx, PendingKey(id), MaxJobBytes)
	if err != nil && !found {
		return nil, err
	}
	if !found {
		v.State = StateNotFound
		return v, nil
	}
	if err == nil {
		if payload, oerr := s.keys.Open(TagJob, data); oerr == nil {
			if j, perr := ParseJob(payload); perr == nil && j.JobID == id {
				v.Job = j
			}
		}
	}
	if v.Job == nil {
		// Not a job QC-2 wrote; the agent will delete it unanswered.
		v.State = StateNotFound
		return v, nil
	}
	_, expires := v.Job.Times()
	switch {
	case hb != nil && hb.CurrentJobID != nil && *hb.CurrentJobID == id:
		v.State = StateRunning
	case now.After(expires.Add(unclaimedGrace)):
		v.State = StateUnclaimed
	default:
		v.State = StateQueued
	}
	return v, nil
}

func (s *Store) openResult(data []byte, id string) (*Result, bool) {
	payload, err := s.keys.Open(TagResult, data)
	if err != nil {
		return nil, false
	}
	var r Result
	if err := json.Unmarshal(payload, &r); err != nil {
		return nil, false
	}
	if r.V != PayloadVersion || r.Type != TypeResult || r.JobID != id || r.Bucket != s.bucket {
		return nil, false
	}
	switch r.Kind { // shown to the model: only the contract's values
	case KindPushPreset, KindListSlots, KindReadPreset:
	default:
		return nil, false
	}
	switch r.Status {
	case StatusSucceeded, StatusFailed, StatusRefused, StatusExpired:
	default:
		return nil, false
	}
	if !ReasonCodes[r.ReasonCode] {
		r.ReasonCode = "unknown"
	}
	return &r, true
}

// Terminal reports whether state is final.
func Terminal(state string) bool {
	switch state {
	case StatusSucceeded, StatusFailed, StatusRefused, StatusExpired, StateUnverified, StateUnclaimed, StateNotFound:
		return true
	}
	return false
}
