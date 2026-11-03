package presetsvc

import (
	"context"
	"sort"
	"time"

	"github.com/weitzer-org/sound-builder/internal/storage"
)

// CleanUpOldDrafts returns the current presets but permanently deletes any "Draft Preset" entries beyond the 3 newest.
// (Moved verbatim from internal/api/handlers_preset.go, which keeps a thin wrapper for its own callers.)
func CleanUpOldDrafts(ctx context.Context, store *storage.PresetStore) []*storage.Preset {
	presets, err := store.List(ctx)
	if err != nil {
		return presets
	}

	type timedPreset struct {
		p *storage.Preset
		t time.Time
	}
	var timed []timedPreset
	for _, p := range presets {
		t, err := time.Parse(time.RFC3339, p.UpdatedAt)
		if err != nil {
			t = time.Time{}
		}
		timed = append(timed, timedPreset{p: p, t: t})
	}

	sort.Slice(timed, func(i, j int) bool {
		if timed[i].t.Equal(timed[j].t) {
			return timed[i].p.Name < timed[j].p.Name
		}
		return timed[i].t.After(timed[j].t)
	})

	var sortedPresets []*storage.Preset
	for _, tp := range timed {
		sortedPresets = append(sortedPresets, tp.p)
	}
	presets = sortedPresets

	draftCount := 0
	var finalPresets []*storage.Preset
	var stale []string
	for _, p := range presets {
		if p.Name == "Draft Preset" {
			draftCount++
			if draftCount > 3 {
				stale = append(stale, p.ID)
				continue
			}
		}
		finalPresets = append(finalPresets, p)
	}
	if len(stale) > 0 {
		// One background goroutine deletes them in turn: the response isn't blocked,
		// and a large backlog can't fan out into a burst of concurrent storage calls.
		// Best-effort and idempotent, so a stop mid-way just leaves the rest for the
		// next cleanup.
		go func(ids []string) {
			for _, id := range ids {
				store.Delete(context.Background(), id)
			}
		}(stale)
	}
	return finalPresets
}
