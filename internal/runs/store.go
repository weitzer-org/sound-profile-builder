package runs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"regexp"
	"time"

	"github.com/weitzer-org/sound-builder/internal/storage"
)

// DefaultAbandonAfter is how long a "running" record may go without an update
// before a reader reports it abandoned. The longest benchmarked run is ~166 s
// and every agent finish writes the record, so a healthy run never goes this
// long without a write.
const DefaultAbandonAfter = 10 * time.Minute

// ErrInvalidID is returned for run IDs that could escape the runs/ prefix.
var ErrInvalidID = errors.New("invalid run id")

// ErrNotFound is returned by Get when no record exists for the ID.
var ErrNotFound = errors.New("run not found")

// IDs are interpolated into object keys, and MCP clients will supply them, so
// only a conservative alphabet is accepted (no slashes, dots or spaces).
var idPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9-]{0,79}$`)

// ValidID reports whether id is safe to use in an object key.
func ValidID(id string) bool { return idPattern.MatchString(id) }

// Store persists RunRecords (and per-agent prompt logs) through storage.Client.
type Store struct {
	client storage.Client
	bucket string
	// AbandonAfter is the staleness threshold used by Get. Zero means
	// DefaultAbandonAfter.
	AbandonAfter time.Duration
	now          func() time.Time
}

// NewStore returns a Store over client/bucket. RUN_ABANDON_AFTER (a Go
// duration such as "15m") overrides the default threshold.
func NewStore(client storage.Client, bucket string) *Store {
	s := &Store{client: client, bucket: bucket, AbandonAfter: DefaultAbandonAfter, now: time.Now}
	if v := os.Getenv("RUN_ABANDON_AFTER"); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			s.AbandonAfter = d
		} else {
			log.Printf("runs: ignoring invalid RUN_ABANDON_AFTER %q", v)
		}
	}
	return s
}

func (s *Store) abandonAfter() time.Duration {
	if s.AbandonAfter > 0 {
		return s.AbandonAfter
	}
	return DefaultAbandonAfter
}

func recordKey(id string) string { return fmt.Sprintf("runs/%s.json", id) }

// Put writes rec verbatim (UpdatedAt is the caller's responsibility).
func (s *Store) Put(ctx context.Context, rec *RunRecord) error {
	if !ValidID(rec.ID) {
		return ErrInvalidID
	}
	data, err := json.Marshal(rec)
	if err != nil {
		return err
	}
	return s.client.WriteFile(ctx, s.bucket, recordKey(rec.ID), data)
}

// Get reads a record. A record still marked running whose UpdatedAt is older
// than the abandon threshold is returned as abandoned, and that verdict is
// written back once so later readers see it without recomputing.
func (s *Store) Get(ctx context.Context, id string) (*RunRecord, error) {
	if !ValidID(id) {
		return nil, ErrInvalidID
	}
	data, err := s.client.ReadFile(ctx, s.bucket, recordKey(id))
	if err != nil {
		return nil, fmt.Errorf("%w: %s: %v", ErrNotFound, id, err)
	}
	var rec RunRecord
	if err := json.Unmarshal(data, &rec); err != nil {
		return nil, fmt.Errorf("decode run %s: %w", id, err)
	}
	if rec.Status == StatusRunning && s.now().Sub(rec.UpdatedAt) > s.abandonAfter() {
		now := s.now().UTC()
		rec.Status = StatusAbandoned
		if rec.Error == "" {
			rec.Error = fmt.Sprintf("run made no progress for over %s and was presumed stopped (server restart or auto-stop); start it again", s.abandonAfter())
		}
		rec.UpdatedAt = now
		rec.CompletedAt = &now
		rec.Events = append(rec.Events, RunEvent{Seq: len(rec.Events) + 1, TS: now, Type: EventInfo, Msg: "marked abandoned: " + rec.Error})
		if err := s.Put(ctx, &rec); err != nil {
			log.Printf("runs: failed to persist abandoned verdict for %s: %v", id, err)
		}
	}
	return &rec, nil
}

// WriteLog stores a per-agent artifact at logs/<run_id>/<name>.
func (s *Store) WriteLog(ctx context.Context, runID, name string, data []byte) error {
	if !ValidID(runID) {
		return ErrInvalidID
	}
	return s.client.WriteFile(ctx, s.bucket, fmt.Sprintf("logs/%s/%s", runID, name), data)
}
