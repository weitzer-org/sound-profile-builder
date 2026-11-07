package devicejobs_test

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/weitzer-org/sound-builder/internal/devicejobs"
	"github.com/weitzer-org/sound-builder/internal/devicejobs/devicejobstest"
)

// lateDone hides jobs/done/ from the first read and listing of it, as if the agent wrote the result (and deleted the
// pending job) between JobState's read of done and its read of pending.
type lateDone struct {
	*devicejobstest.MemClient
	hidden atomic.Int32
}

func (c *lateDone) hide(object string) bool {
	return strings.HasPrefix(object, "jobs/done/") && c.hidden.Add(1) <= 2 // the read and its existence check
}

func (c *lateDone) ReadFileLimited(ctx context.Context, bucket, object string, max int64) ([]byte, error) {
	if c.hide(object) {
		return nil, errors.New("not there yet")
	}
	return c.MemClient.ReadFileLimited(ctx, bucket, object, max)
}

func (c *lateDone) ListFiles(ctx context.Context, bucket, prefix string) ([]string, error) {
	if strings.HasPrefix(prefix, "jobs/done/") && c.hidden.Load() < 2 {
		c.hidden.Add(1)
		return nil, nil
	}
	return c.MemClient.ListFiles(ctx, bucket, prefix)
}

// A result that lands after JobState read done and before it read pending is the job's result, not "no such job".
func TestJobStateSeesAResultThatLandsBetweenItsReads(t *testing.T) {
	r := newRig(t)
	r.online(t)
	id, err := r.bridge.QueueRead(context.Background(), devicejobs.KindReadPreset)
	if err != nil {
		t.Fatal(err)
	}
	if n, err := r.agent.RunOnce(); err != nil || n != 1 { // done written, pending deleted
		t.Fatalf("agent ran %d: %v", n, err)
	}
	late := &lateDone{MemClient: r.client}
	st := devicejobs.NewStore(late, &devicejobs.Config{Bucket: jobsBucket, Keys: r.bridge.Keys})
	v, err := st.JobState(context.Background(), id, nil, *r.clock)
	if err != nil {
		t.Fatal(err)
	}
	if v.State == devicejobs.StateNotFound || v.Result == nil {
		t.Fatalf("state = %q, result %v: the finished job was reported as missing", v.State, v.Result)
	}
	if v.State != devicejobs.StatusSucceeded {
		t.Errorf("state = %q, want %q", v.State, devicejobs.StatusSucceeded)
	}
}
