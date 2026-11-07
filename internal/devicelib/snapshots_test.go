package devicelib

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/weitzer-org/sound-builder/internal/devicejobs"
	"github.com/weitzer-org/sound-builder/internal/devicejobs/devicejobstest"
)

const snapJob = "33333333-3333-4333-8333-333333333333"

func snapView(payload string) *devicejobs.JobView {
	slot, name, fw := "3A", "Texas Blues", "4.1.0"
	return &devicejobs.JobView{JobID: snapJob, State: devicejobs.StatusSucceeded, Result: &devicejobs.Result{
		JobID: snapJob, Kind: devicejobs.KindReadPreset, Status: devicejobs.StatusSucceeded,
		FinishedAt: "2026-11-07T10:00:00Z", Firmware: &fw,
		Readback: &devicejobs.ReadbackBlock{Format: devicejobs.ReadbackFormat, Slot: &slot, Name: &name, Payload: json.RawMessage(payload)},
	}}
}

const snapPayload = `{"setlist":{"folder":"My Presets","position":2},"chains":[{"row":0,"in_port":1,"out_port":19,"blocks":[]}],"bypass":null}`

func TestSnapshotRoundTripAndWriteOnce(t *testing.T) {
	ctx := context.Background()
	c := devicejobstest.NewMemClient()
	st := NewStore(c, "main")
	if s, err := st.ReadSnapshot(ctx, snapJob); err != nil || s != nil {
		t.Fatalf("empty store: %v %v", s, err)
	}
	snap, err := NewReadSnapshot(snapView(snapPayload))
	if err != nil {
		t.Fatal(err)
	}
	if wrote, err := st.SaveSnapshot(ctx, snap); err != nil || !wrote {
		t.Fatalf("first save: %v %v", wrote, err)
	}
	got, err := st.ReadSnapshot(ctx, snapJob)
	if err != nil || got == nil || got.Slot != "3A" || got.Name != "Texas Blues" || got.Firmware != "4.1.0" || string(got.Payload) != snapPayload {
		t.Fatalf("loaded %+v %v", got, err)
	}
	// Write-once: a second save changes nothing.
	other := *snap
	other.Name = "Changed"
	if wrote, err := st.SaveSnapshot(ctx, &other); err != nil || wrote {
		t.Fatalf("second save: %v %v", wrote, err)
	}
	if again, _ := st.ReadSnapshot(ctx, snapJob); again.Name != "Texas Blues" {
		t.Errorf("snapshot was replaced: %q", again.Name)
	}
}

func TestSnapshotRefusals(t *testing.T) {
	ctx := context.Background()
	st := NewStore(devicejobstest.NewMemClient(), "main")
	if _, err := NewReadSnapshot(snapView(`{"chains":"x"}`)); err == nil {
		t.Error("a payload that fails the strict parse was accepted")
	}
	bad := snapView(snapPayload)
	bad.State = devicejobs.StatusFailed
	if _, err := NewReadSnapshot(bad); err == nil {
		t.Error("a failed job produced a snapshot")
	}
	bad = snapView(snapPayload)
	bad.Result.Kind = devicejobs.KindListSlots
	if _, err := NewReadSnapshot(bad); err == nil {
		t.Error("a non-read job produced a snapshot")
	}
	bad = snapView(snapPayload)
	bad.Result.JobID = "44444444-4444-4444-8444-444444444444"
	if _, err := NewReadSnapshot(bad); err == nil {
		t.Error("a result for another job produced a snapshot")
	}
	for _, id := range []string{"", "../x", "a/b", "not-a-uuid"} {
		if _, err := st.ReadSnapshot(ctx, id); err == nil {
			t.Errorf("job id %q accepted", id)
		}
	}
}

// A stored object the bucket's writers altered is an error, never "not there" and never trusted.
func TestStoredSnapshotIsRevalidated(t *testing.T) {
	ctx := context.Background()
	c := devicejobstest.NewMemClient()
	st := NewStore(c, "main")
	for name, obj := range map[string]string{
		"wrong job":   `{"format":"qc2-device-snapshot/1","job_id":"44444444-4444-4444-8444-444444444444","readback_format":"qc-device-readback/1","read_at":"2026-11-07T10:00:00Z","payload":` + snapPayload + `}`,
		"bad payload": `{"format":"qc2-device-snapshot/1","job_id":"` + snapJob + `","readback_format":"qc-device-readback/1","read_at":"2026-11-07T10:00:00Z","payload":{"chains":"x"}}`,
		"bad format":  `{"format":"other","job_id":"` + snapJob + `","readback_format":"qc-device-readback/1","read_at":"2026-11-07T10:00:00Z","payload":` + snapPayload + `}`,
		"long name":   `{"format":"qc2-device-snapshot/1","job_id":"` + snapJob + `","name":"` + strings.Repeat("x", 300) + `","readback_format":"qc-device-readback/1","read_at":"2026-11-07T10:00:00Z","payload":` + snapPayload + `}`,
		"not json":    `nope`,
	} {
		if err := c.WriteFile(ctx, "main", SnapshotObject(snapJob), []byte(obj)); err != nil {
			t.Fatal(err)
		}
		if s, err := st.ReadSnapshot(ctx, snapJob); err == nil {
			t.Errorf("%s: loaded %+v", name, s)
		}
	}
}
