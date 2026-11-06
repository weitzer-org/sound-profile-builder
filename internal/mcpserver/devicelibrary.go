package mcpserver

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/weitzer-org/sound-builder/internal/devicejobs"
	"github.com/weitzer-org/sound-builder/internal/devicelib"
)

// Library sync (plan P4b). sync_device_library queues a read-only job; the home agent lists
// the capture files on the unit (the same directory read the push uses) and QC-2 stores the
// list through storage.Client. The dry run of send_preset_to_device then resolves each
// capture against it. Cab mic strings are collected only from presets the user asks to read
// (read_device_preset): whether a stored preset can be read without loading it on the unit
// is UNVERIFIED, so the bridge never reads one. Everything from the unit is untrusted text:
// parsed strictly, sanitised, bounded, and rendered only in code spans.

const deviceSyncToolName = "sync_device_library"

// DeviceLibrary is the server's side of the sync: where the lists are stored and how a
// read-back is mined for cab mics. Without it the sync tool is not registered and reads
// record nothing.
type DeviceLibrary struct {
	Store     *devicelib.Store
	Harvester *devicelib.Harvester
	// Now is injectable for tests.
	Now func() time.Time
}

func (l *DeviceLibrary) now() time.Time {
	if l.Now != nil {
		return l.Now().UTC()
	}
	return time.Now().UTC()
}

type deviceLibraryOut struct {
	Status         string `json:"status"` // stored | not_stored
	Reason         string `json:"reason,omitempty"`
	SyncedAt       string `json:"synced_at,omitempty"`
	Total          int    `json:"total"`
	Factory        int    `json:"factory"`
	User           int    `json:"user"`
	V1             int    `json:"v1"`
	V2             int    `json:"v2"`
	UnknownVersion int    `json:"unknown_version"`
	CloudOnly      int    `json:"cloud_only"`
	Skipped        int    `json:"skipped"`
}

type deviceMicsOut struct {
	NewlyRecorded int `json:"newly_recorded"`
	Ignored       int `json:"ignored"`
}

// storeSync stores the library a finished sync_device_library job returned. It is
// idempotent (the same job writes the same list) and never replaces a newer sync with an
// older job's late answer.
func (l *DeviceLibrary) storeSync(ctx context.Context, v *devicejobs.JobView) *deviceLibraryOut {
	no := func(why string) *deviceLibraryOut { return &deviceLibraryOut{Status: "not_stored", Reason: why} }
	r := v.Result
	if r == nil || r.Kind != devicejobs.KindSyncLibrary {
		return nil
	}
	if r.Status != devicejobs.StatusSucceeded {
		return no("the sync did not succeed, so the stored capture list is unchanged")
	}
	if r.Library == nil || r.Library.Format != devicejobs.LibraryFormat {
		return no("the bridge's answer had no capture list in a format this server reads, so nothing was stored")
	}
	p, err := devicejobs.ParseLibraryPayload(r.Library.Payload)
	if err != nil {
		return no("the capture list was rejected (" + err.Error() + "), so nothing was stored")
	}
	at := l.now()
	if t, err := devicejobs.ParseTime(r.FinishedAt); err == nil && !t.After(at.Add(devicejobs.MaxCreatedAhead)) {
		at = t
	}
	lib := devicelib.NewLibrary(p, v.JobID, dptr(r.Firmware), at)
	cur, err := l.Store.Library(ctx)
	if err != nil {
		log.Printf("device library: reading the stored list: %v", err)
		return no("the stored capture list could not be read, so nothing was stored")
	}
	if cur != nil && cur.JobID != lib.JobID {
		if old, err := devicejobs.ParseTime(cur.SyncedAt); err == nil && old.After(at) {
			return no("a newer sync is already stored; this older answer was not used")
		}
	}
	if err := l.Store.SaveLibrary(ctx, lib); err != nil {
		log.Printf("device library: storing the list: %v", err)
		return no("storing the capture list failed")
	}
	c := lib.Count()
	return &deviceLibraryOut{Status: "stored", SyncedAt: lib.SyncedAt, Total: c.Total, Factory: c.Factory, User: c.User,
		V1: c.V1, V2: c.V2, UnknownVersion: c.UnknownVersion, CloudOnly: c.Cloud, Skipped: lib.Skipped}
}

// storeSlots keeps which slots the most recent succeeded list_slots job showed occupied, so a dry run can warn
// (F9). Best effort: a failure is logged and changes nothing the user sees. A late older answer never replaces
// a newer listing.
func (l *DeviceLibrary) storeSlots(ctx context.Context, v *devicejobs.JobView) {
	r := v.Result
	if l == nil || r == nil || r.Kind != devicejobs.KindListSlots || r.Status != devicejobs.StatusSucceeded || r.Slots == nil {
		return
	}
	at, err := devicejobs.ParseTime(r.FinishedAt)
	if err != nil || at.After(l.now().Add(devicejobs.MaxCreatedAhead)) {
		return
	}
	cur, err := l.Store.Slots(ctx)
	if err != nil {
		log.Printf("device slots: reading the stored listing: %v", err)
		cur = nil // a malformed record is replaced
	}
	if cur != nil {
		if cur.JobID == v.JobID {
			return // this job's listing is already stored
		}
		if old, err := devicejobs.ParseTime(cur.ListedAt); err == nil && old.After(at) {
			return
		}
	}
	if err := l.Store.SaveSlots(ctx, devicelib.NewSlotListing(r.Slots, v.JobID, r.FinishedAt)); err != nil {
		log.Printf("device slots: storing the listing: %v", err)
	}
}

// slotOccupiedNote is the dry run's warning when the last stored listing showed the slot occupied, or "".
// It never blocks the confirm token: the listing can be stale, and the bridge decides.
func (l *DeviceLibrary) slotOccupiedNote(ctx context.Context, slot string) string {
	if l == nil {
		return ""
	}
	cur, err := l.Store.Slots(ctx)
	if err != nil {
		log.Printf("device slots: reading the stored listing: %v", err)
		return ""
	}
	if cur == nil || !cur.IsOccupied(slot) {
		return ""
	}
	return fmt.Sprintf("slot %s was occupied when the bridge last listed the slots (%s). The bridge refuses to write into an occupied slot (slot_not_empty); "+
		"that listing may be out of date, and the bridge checks the unit again when it pushes. Pick an empty slot (list_device_slots) or free this one on the unit first",
		slot, cur.ListedAt)
}

// recordMics records the cab mic strings of a finished read_preset job. The user asked for
// the read, so this is the only source of the mic list.
func (l *DeviceLibrary) recordMics(ctx context.Context, v *devicejobs.JobView) *deviceMicsOut {
	r := v.Result
	if r == nil || r.Kind != devicejobs.KindReadPreset || r.Status != devicejobs.StatusSucceeded || r.Readback == nil ||
		r.Readback.Format != devicejobs.ReadbackFormat {
		return nil
	}
	p, err := devicejobs.ParseReadbackPayload(r.Readback.Payload)
	if err != nil {
		return nil // the read-back itself is reported as unusable elsewhere
	}
	seen, ignored := l.Harvester.Harvest(p)
	added, err := l.Store.AddMics(ctx, seen, l.now())
	if err != nil {
		log.Printf("device library: recording cab mics: %v", err)
		return &deviceMicsOut{Ignored: ignored}
	}
	return &deviceMicsOut{NewlyRecorded: added, Ignored: ignored}
}

func renderLibrary(b *strings.Builder, o *deviceLibraryOut) {
	if o == nil {
		return
	}
	if o.Status != "stored" {
		fmt.Fprintf(b, "- Capture list: **not stored**: %s.\n", sanitizeDeviceText(o.Reason, 200))
		return
	}
	fmt.Fprintf(b, "- Capture list stored (synced %s): %d captures, %d factory and %d user; %d V1, %d V2, %d with no V1/V2 marker the bridge trusts",
		o.SyncedAt, o.Total, o.Factory, o.User, o.V1, o.V2, o.UnknownVersion)
	if o.CloudOnly > 0 {
		fmt.Fprintf(b, "; %d listed only as Cortex Cloud items (not files on the unit)", o.CloudOnly)
	}
	if o.Skipped > 0 {
		fmt.Fprintf(b, "; %d left out because their names could not be compared exactly", o.Skipped)
	}
	b.WriteString(".\n  send_preset_to_device's dry run now checks each capture against this list; the bridge still re-checks the unit when it pushes.\n")
}

func renderMics(b *strings.Builder, o *deviceMicsOut) {
	if o == nil {
		return
	}
	fmt.Fprintf(b, "- Cab mics seen in this preset: %d new recorded", o.NewlyRecorded)
	if o.Ignored > 0 {
		fmt.Fprintf(b, ", %d left out (not a variation of the cab's own default)", o.Ignored)
	}
	b.WriteString(". A later dry run can write a recorded mic exactly instead of leaving the cab on its default.\n")
}

// syncDevicePreset is sync_device_library: queue the read-only job, wait, store the answer.
func syncDeviceLibrary(b *devicejobs.Bridge, lib *DeviceLibrary) mcp.ToolHandlerFor[deviceReadInput, deviceJobOutput] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in deviceReadInput) (*mcp.CallToolResult, deviceJobOutput, error) {
		wait, err := deviceWait(in.WaitSeconds, 60) // the directory listing alone takes ~15 s
		if err != nil {
			return nil, deviceJobOutput{}, err
		}
		id, err := b.QueueRead(ctx, devicejobs.KindSyncLibrary)
		if err != nil {
			return nil, deviceJobOutput{}, deviceError(deviceSyncToolName, err)
		}
		v, err := b.Wait(ctx, id, wait)
		if err != nil {
			return nil, deviceJobOutput{}, deviceError(deviceSyncToolName, err)
		}
		out := jobOutput(v)
		out.Library = lib.storeSync(ctx, v)
		var t strings.Builder
		t.WriteString(deviceDataNotice + "\n\n")
		renderJob(&t, out)
		if !devicejobs.Terminal(out.State) {
			fmt.Fprintf(&t, "\nThe bridge hasn't answered yet: call get_device_job with job_id `%s` in this same turn; it stores the list when the job has finished.", id)
		}
		return textResult(strings.TrimRight(t.String(), "\n")), out, nil
	}
}
