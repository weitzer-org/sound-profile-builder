package devicelib

import (
	"context"
	"encoding/json"
	"errors"
	"sort"

	"github.com/weitzer-org/sound-builder/internal/devicejobs"
)

// SlotsObject is the stored summary of the most recent succeeded list_slots job.
const (
	SlotsObject = "device/slots.json"
	SlotsFormat = "qc2-device-slots/1"
	maxSlots    = 256
)

// SlotListing is what the dry run knows about the unit's slots: which ones the bridge last saw
// occupied, and when. Names are deliberately not kept (device text, and the dry run does not need
// them). It can be stale: the unit may have changed since, and the bridge decides at push time.
type SlotListing struct {
	Format   string   `json:"format"`
	ListedAt string   `json:"listed_at"` // contract timestamp of the listing job's result
	JobID    string   `json:"job_id"`
	Occupied []string `json:"occupied"` // normalized slots, sorted
}

// NewSlotListing builds a listing from a succeeded list_slots result. Slots that are not valid My Presets
// slot names are dropped.
func NewSlotListing(r *devicejobs.SlotsResult, jobID, listedAt string) *SlotListing {
	l := &SlotListing{Format: SlotsFormat, ListedAt: listedAt, JobID: jobID, Occupied: []string{}}
	seen := map[string]bool{}
	for _, s := range r.Slots {
		slot, ok := devicejobs.NormalizeSlot(s.Slot)
		if ok && s.State == "occupied" && !seen[slot] {
			seen[slot] = true
			l.Occupied = append(l.Occupied, slot)
		}
	}
	sort.Strings(l.Occupied)
	return l
}

// IsOccupied reports whether the listing showed the slot occupied.
func (l *SlotListing) IsOccupied(slot string) bool {
	for _, s := range l.Occupied {
		if s == slot {
			return true
		}
	}
	return false
}

func (l *SlotListing) validate() error {
	bad := errors.New("the stored slot listing is malformed")
	if l.Format != SlotsFormat || len(l.Occupied) > maxSlots {
		return bad
	}
	if _, err := devicejobs.ParseTime(l.ListedAt); err != nil {
		return bad
	}
	for _, s := range l.Occupied {
		if n, ok := devicejobs.NormalizeSlot(s); !ok || n != s {
			return bad
		}
	}
	return nil
}

// Slots returns the stored listing, or nil when none has been stored.
func (s *Store) Slots(ctx context.Context) (*SlotListing, error) {
	var l SlotListing
	ok, err := s.readJSON(ctx, SlotsObject, &l)
	if err != nil || !ok {
		return nil, err
	}
	if err := l.validate(); err != nil {
		return nil, err
	}
	return &l, nil
}

// SaveSlots replaces the stored listing.
func (s *Store) SaveSlots(ctx context.Context, l *SlotListing) error {
	data, err := json.Marshal(l)
	if err != nil {
		return err
	}
	return s.client.WriteFile(ctx, s.bucket, SlotsObject, data)
}
