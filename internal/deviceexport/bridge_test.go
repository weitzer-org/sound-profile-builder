package deviceexport

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/weitzer-org/sound-builder/internal/devicejobs"
	"github.com/weitzer-org/sound-builder/internal/devicejobs/devicejobstest"
)

// TestBridgeWithRealExporter runs P3's dry run and confirm with this package's exporter:
// a pushable preset gets a confirm token and is queued with the exact payload; one with
// unresolved items gets no token and its confirm is refused.
func TestBridgeWithRealExporter(t *testing.T) {
	const key = "000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f"
	kr, err := devicejobs.NewKeyring(key, "")
	if err != nil {
		t.Fatal(err)
	}
	c := devicejobstest.NewMemClient()
	now := time.Now().UTC().Truncate(time.Second)
	b := devicejobs.NewBridge(c, &devicejobs.Config{Bucket: "jobs", Keys: kr}, testExporter(t))
	b.Now = func() time.Time { return now }
	agent := devicejobstest.NewFakeAgent(c, "jobs", kr)
	agent.Now = b.Now
	if err := agent.Heartbeat(-10*time.Second, "idle", nil); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	sp := loadPreset(t, "pushable_g3")
	req := devicejobs.PushRequest{PresetID: "p-1", Guitar: "Gibson ES-339 Humbuckers", Slot: "20A", Name: "G3 shape"}
	plan, err := b.PlanPush(ctx, sp, req)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Token == "" || len(plan.Export.Unresolved) != 0 {
		t.Fatalf("pushable preset: token %q, unresolved %+v", plan.Token, plan.Export.Unresolved)
	}
	id, _, err := b.ConfirmPush(ctx, sp, req, plan.Token)
	if err != nil {
		t.Fatal(err)
	}
	pend, err := b.Store.PendingJobs(ctx, now)
	if err != nil || len(pend) != 1 {
		t.Fatalf("pending jobs %d, err %v", len(pend), err)
	}
	var args devicejobs.PushArgs
	if err := json.Unmarshal(pend[0].Args, &args); err != nil {
		t.Fatalf("job %s args: %v", id, err)
	}
	var want bytes.Buffer
	if err := json.Compact(&want, plan.Export.Payload); err != nil {
		t.Fatal(err)
	}
	if args.Export.Format != Format || !bytes.Equal(args.Export.Payload, want.Bytes()) {
		t.Fatalf("queued export differs from the dry run's")
	}

	blocked := loadPreset(t, "05_Clapton")
	req2 := devicejobs.PushRequest{PresetID: "p-2", Guitar: "Gibson ES-339 Humbuckers", Slot: "20B", Name: "Clapton"}
	plan2, err := b.PlanPush(ctx, blocked, req2)
	if err != nil {
		t.Fatal(err)
	}
	if plan2.Token != "" || len(plan2.Export.Unresolved) == 0 {
		t.Fatalf("unresolved preset got a token")
	}
	// A token made for the pushable plan can't be reused for this one.
	_, _, err = b.ConfirmPush(ctx, blocked, req2, plan.Token)
	var ue *devicejobs.UserError
	if !errors.As(err, &ue) || !strings.Contains(ue.Msg, "confirm_token") {
		t.Fatalf("confirm of an unresolved plan: %v", err)
	}
}
