// Package devicelib holds what QC-2 knows about the contents of the owner's Quad Cortex:
// the capture list a sync_device_library job reported (plan P4b) and the cab mic strings
// seen in presets the user asked to read. Both are device-sourced and untrusted: they are
// parsed strictly, re-sanitised before they are stored, and only ever compared for exact
// equality. They never feed a write decision on their own: the home agent re-resolves
// every capture against a fresh listing of the unit before it places one.
package devicelib

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/weitzer-org/sound-builder/internal/devicejobs"
	"github.com/weitzer-org/sound-builder/internal/storage"
)

// Object names in QC-2's main bucket.
const (
	LibraryObject  = "device/library.json"
	MicsObject     = "device/cab_mics.json"
	LibraryFormat  = "qc2-device-library/1"
	MicsFormat     = "qc2-cab-mics/1"
	maxStoredBytes = 1 << 20
)

// Library is the stored capture list.
type Library struct {
	Format   string    `json:"format"`
	SyncedAt string    `json:"synced_at"` // contract timestamp
	JobID    string    `json:"job_id"`
	Firmware string    `json:"firmware,omitempty"`
	Skipped  int       `json:"skipped"`
	Captures []Capture `json:"captures"`
}

// Capture is one capture file on the unit.
type Capture struct {
	Name    string `json:"name"`
	Key     string `json:"key"`
	Version int    `json:"version"` // 1, 2, or 0 when the bridge could not tell
	Kind    string `json:"kind"`
	Folder  string `json:"folder"`
	Cloud   bool   `json:"cloud"`
}

// NewLibrary builds the stored form from a parsed payload. The strings are sanitised again
// (they were checked clean by ParseLibraryPayload; this keeps the invariant local) and a
// capture whose name or folder the clean-up would change is dropped and counted as skipped.
func NewLibrary(p *devicejobs.LibraryPayload, jobID, firmware string, at time.Time) *Library {
	l := &Library{
		Format: LibraryFormat, SyncedAt: devicejobs.FormatTime(at), JobID: jobID,
		Firmware: devicejobs.SanitizeText(firmware, 32), Skipped: p.Skipped, Captures: make([]Capture, 0, len(p.Captures)),
	}
	for _, c := range p.Captures {
		folder := devicejobs.SanitizeText(c.Folder, devicejobs.MaxLibraryFolder)
		if devicejobs.SanitizeText(c.Name, devicejobs.MaxLibraryName) != c.Name || folder != c.Folder {
			l.Skipped++
			continue
		}
		v := 0
		if c.Version != nil {
			v = *c.Version
		}
		l.Captures = append(l.Captures, Capture{Name: c.Name, Key: c.Key, Version: v, Kind: c.Kind, Folder: folder, Cloud: c.Cloud})
	}
	return l
}

// Counts summarises a library for the tool's answer.
type Counts struct {
	Total, Factory, User, V1, V2, UnknownVersion, Cloud int
}

// Count tallies the list.
func (l *Library) Count() Counts {
	var c Counts
	for _, e := range l.Captures {
		c.Total++
		if e.Kind == devicejobs.CaptureKindFactory {
			c.Factory++
		} else {
			c.User++
		}
		switch e.Version {
		case 1:
			c.V1++
		case 2:
			c.V2++
		default:
			c.UnknownVersion++
		}
		if e.Cloud {
			c.Cloud++
		}
	}
	return c
}

// Store reads and writes the two documents through storage.Client (the main bucket).
type Store struct {
	client storage.Client
	bucket string
}

// NewStore binds a storage client and the main bucket name.
func NewStore(client storage.Client, bucket string) *Store {
	return &Store{client: client, bucket: bucket}
}

// readJSON reports whether the object exists and decodes it into v. storage.Client has no
// not-found error, so existence comes from a listing, as in devicejobs.Store: the S3 and GCS
// backends' ListFiles skip a key equal to the prefix, so the prefix is the key minus ".json".
func (s *Store) readJSON(ctx context.Context, object string, v any) (bool, error) {
	names, err := s.client.ListFiles(ctx, s.bucket, strings.TrimSuffix(object, ".json"))
	if err != nil {
		return false, err
	}
	found := false
	for _, n := range names {
		if n == object {
			found = true
		}
	}
	if !found {
		return false, nil
	}
	data, err := s.client.ReadFile(ctx, s.bucket, object)
	if err != nil {
		return false, err
	}
	if len(data) > maxStoredBytes {
		return false, fmt.Errorf("%s is over its size limit", object)
	}
	if err := json.Unmarshal(data, v); err != nil {
		return false, fmt.Errorf("%s is not valid: %w", object, err)
	}
	return true, nil
}

// Library returns the stored capture list, or nil when no sync has been stored.
func (s *Store) Library(ctx context.Context) (*Library, error) {
	var l Library
	ok, err := s.readJSON(ctx, LibraryObject, &l)
	if err != nil || !ok {
		return nil, err
	}
	if l.Format != LibraryFormat {
		return nil, errors.New("the stored capture list has an unknown format")
	}
	return &l, nil
}

// SaveLibrary replaces the stored list: the newest sync is the unit's current truth.
func (s *Store) SaveLibrary(ctx context.Context, l *Library) error {
	data, err := json.Marshal(l)
	if err != nil {
		return err
	}
	return s.client.WriteFile(ctx, s.bucket, LibraryObject, data)
}

// Mics is the stored per-cab list of mic strings seen on the unit.
type Mics struct {
	Format    string              `json:"format"`
	UpdatedAt string              `json:"updated_at"`
	Cabs      map[string][]string `json:"cabs"` // catalog model id (decimal) -> sorted distinct strings
}

// Limits on the stored mic list, so a stream of reads can't grow it without bound.
const (
	MaxMicCabs    = 400
	MaxMicsPerCab = 64
)

// For returns the mic strings seen for a catalog cab model id.
func (m *Mics) For(modelID int) []string {
	if m == nil {
		return nil
	}
	return m.Cabs[fmt.Sprint(modelID)]
}

// Total is the number of stored strings.
func (m *Mics) Total() int {
	n := 0
	if m != nil {
		for _, v := range m.Cabs {
			n += len(v)
		}
	}
	return n
}

// Mics returns the stored list, or nil when none has been stored.
func (s *Store) Mics(ctx context.Context) (*Mics, error) {
	var m Mics
	ok, err := s.readJSON(ctx, MicsObject, &m)
	if err != nil || !ok {
		return nil, err
	}
	if m.Format != MicsFormat {
		return nil, errors.New("the stored cab mic list has an unknown format")
	}
	return &m, nil
}

// AddMics unions seen (model id -> strings) into the stored list and returns how many
// strings were new. Nothing is written when nothing is new.
func (s *Store) AddMics(ctx context.Context, seen map[int][]string, at time.Time) (int, error) {
	if len(seen) == 0 {
		return 0, nil
	}
	cur, err := s.Mics(ctx)
	if err != nil {
		return 0, err
	}
	if cur == nil {
		cur = &Mics{Format: MicsFormat, Cabs: map[string][]string{}}
	}
	added := 0
	for id, list := range seen {
		k := fmt.Sprint(id)
		have := map[string]bool{}
		for _, v := range cur.Cabs[k] {
			have[v] = true
		}
		for _, v := range list {
			if have[v] || len(cur.Cabs[k]) >= MaxMicsPerCab {
				continue
			}
			if _, ok := cur.Cabs[k]; !ok && len(cur.Cabs) >= MaxMicCabs {
				continue
			}
			cur.Cabs[k] = append(cur.Cabs[k], v)
			have[v] = true
			added++
		}
		sort.Strings(cur.Cabs[k])
	}
	if added == 0 {
		return 0, nil
	}
	cur.UpdatedAt = devicejobs.FormatTime(at)
	data, err := json.Marshal(cur)
	if err != nil {
		return 0, err
	}
	return added, s.client.WriteFile(ctx, s.bucket, MicsObject, data)
}

// Snapshot is what an export reads once per build: the library, the alias table and the
// mic list. Library and Mics are nil when nothing has been synced or seen.
type Snapshot struct {
	Library *Library
	Aliases *Aliases
	Mics    *Mics
}

// Source loads a Snapshot. A storage failure is returned so the caller can say the list
// could not be read, rather than treating it as "not synced".
type Source struct {
	Store   *Store
	Aliases *Aliases
}

// Snapshot reads the library and mic list.
func (s *Source) Snapshot(ctx context.Context) (*Snapshot, error) {
	l, err := s.Store.Library(ctx)
	if err != nil {
		return nil, err
	}
	m, err := s.Store.Mics(ctx)
	if err != nil {
		return nil, err
	}
	return &Snapshot{Library: l, Aliases: s.Aliases, Mics: m}, nil
}
