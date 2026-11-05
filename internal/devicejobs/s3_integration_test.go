package devicejobs_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/weitzer-org/sound-builder/internal/devicejobs"
	"github.com/weitzer-org/sound-builder/internal/devicejobs/devicejobstest"
	"github.com/weitzer-org/sound-builder/internal/storage"
)

// TestS3Integration runs the queue against the real S3 client (storage.S3Client)
// when DEVICEJOBS_S3_TEST_BUCKET names an existing, empty, throwaway bucket on
// the endpoint in S3_ENDPOINT (MinIO, moto or a test R2 bucket). It checks the
// part the in-memory client can only imitate: existence probes via ListFiles,
// which skips a key equal to the prefix on S3. Skipped otherwise.
func TestS3Integration(t *testing.T) {
	bucket := os.Getenv("DEVICEJOBS_S3_TEST_BUCKET")
	if bucket == "" {
		t.Skip("set DEVICEJOBS_S3_TEST_BUCKET (and S3_ENDPOINT, S3_ACCESS_KEY_ID, S3_SECRET_ACCESS_KEY) to run")
	}
	ctx := context.Background()
	s3c, err := storage.NewS3Client(ctx)
	if err != nil {
		t.Fatal(err)
	}
	kr, _ := devicejobs.NewKeyring(keyA, "")
	cfg := &devicejobs.Config{Bucket: bucket, Keys: kr}
	b := devicejobs.NewBridge(s3c, cfg, &devicejobstest.StaticExporter{Exp: devicejobstest.FixtureExport()})
	b.PollGap = 50 * time.Millisecond

	// Write a heartbeat through the real client.
	hb := devicejobstest.NewFakeAgent(devicejobstest.NewMemClient(), bucket, kr)
	mem := hb.Client
	if err := hb.Heartbeat(-5*time.Second, "idle", nil); err != nil {
		t.Fatal(err)
	}
	data, _ := mem.ReadFile(ctx, bucket, devicejobs.HeartbeatKey)
	if err := s3c.WriteFile(ctx, bucket, devicejobs.HeartbeatKey, data); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s3c.DeleteFile(context.Background(), bucket, devicejobs.HeartbeatKey) })

	plan, err := b.PlanPush(ctx, sp(), req)
	if err != nil || plan.Token == "" || !plan.Bridge.Online {
		t.Fatalf("plan %+v err %v", plan, err)
	}
	id, existing, err := b.ConfirmPush(ctx, sp(), req, plan.Token)
	if err != nil || existing {
		t.Fatalf("confirm: %v existing=%v", err, existing)
	}
	t.Cleanup(func() {
		for _, k := range []string{devicejobs.PendingKey(id), devicejobs.DoneKey(id)} {
			_ = s3c.DeleteFile(context.Background(), bucket, k)
		}
	})
	// The existence probe must see the pending object on real S3.
	if _, existing, err := b.ConfirmPush(ctx, sp(), req, plan.Token); err != nil || !existing {
		t.Fatalf("repeat confirm on S3: existing=%v err=%v (ListFiles prefix semantics?)", existing, err)
	}
	if v, err := b.Job(ctx, id); err != nil || v.State != devicejobs.StateQueued {
		t.Fatalf("state %+v err %v", v, err)
	}
	if v, err := b.Job(ctx, "11111111-2222-4333-8444-555555555555"); err != nil || v.State != devicejobs.StateNotFound {
		t.Fatalf("missing job on S3: %+v err %v", v, err)
	}
}
