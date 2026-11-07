// Package devicejobstest holds test doubles for the device bridge: an
// in-memory storage.Client with the S3 backend's listing semantics, a fixture
// Exporter, and a FakeAgent that plays the home agent by the contract's rules.
package devicejobstest

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/weitzer-org/sound-builder/internal/devicejobs"
	"github.com/weitzer-org/sound-builder/internal/storage"
)

// MemClient is a mutex-guarded in-memory storage.Client. Like the S3 and GCS
// backends (s3.go, gcs.go), ListFiles skips the object whose key equals the
// prefix, so code that probes existence by listing an exact key fails here
// exactly as it would in production.
type MemClient struct {
	mu        sync.Mutex
	objs      map[string]map[string][]byte
	failList  bool
	failRead  bool
	failWrite bool
	failNextN int
	limited   int
}

// FailNextReads makes the next n ReadFile/ReadFileLimited calls fail, then
// succeeds again: a deterministic transient storage error.
func (c *MemClient) FailNextReads(n int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.failNextN = n
}

// SetFailRead makes every later ReadFile/ReadFileLimited call fail, as a
// transient storage error would (the object still lists as present).
func (c *MemClient) SetFailRead(fail bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.failRead = fail
}

// SetFailList makes every later ListFiles call fail (storage-error tests).
func (c *MemClient) SetFailList(fail bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.failList = fail
}

// SetFailWrite makes every later WriteFile call fail (storage-error tests).
func (c *MemClient) SetFailWrite(fail bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.failWrite = fail
}

// NewMemClient returns an empty client.
func NewMemClient() *MemClient { return &MemClient{objs: map[string]map[string][]byte{}} }

func (c *MemClient) ReadFile(_ context.Context, bucket, object string) ([]byte, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.failRead {
		return nil, fmt.Errorf("read failed (transient)")
	}
	if c.failNextN > 0 {
		c.failNextN--
		return nil, fmt.Errorf("read failed (transient)")
	}
	d, ok := c.objs[bucket][object]
	if !ok {
		return nil, fmt.Errorf("object not found: %s/%s", bucket, object)
	}
	return append([]byte(nil), d...), nil
}

func (c *MemClient) WriteFile(_ context.Context, bucket, object string, data []byte) error {
	c.mu.Lock()
	failing := c.failWrite
	c.mu.Unlock()
	if failing {
		return errors.New("memclient: write failed")
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.objs[bucket] == nil {
		c.objs[bucket] = map[string][]byte{}
	}
	c.objs[bucket][object] = append([]byte(nil), data...)
	return nil
}

func (c *MemClient) ListFiles(_ context.Context, bucket, prefix string) ([]string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.failList {
		return nil, fmt.Errorf("list failed")
	}
	var names []string
	for n := range c.objs[bucket] {
		if strings.HasPrefix(n, prefix) && n != prefix {
			names = append(names, n)
		}
	}
	sort.Strings(names)
	return names, nil
}

func (c *MemClient) DeleteFile(_ context.Context, bucket, object string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.objs[bucket], object)
	return nil
}

func (c *MemClient) Close() {}

// ReadFileLimited implements storage.LimitedReader like the S3 and GCS
// backends do, and counts its calls.
func (c *MemClient) ReadFileLimited(ctx context.Context, bucket, object string, max int64) ([]byte, error) {
	c.mu.Lock()
	c.limited++
	c.mu.Unlock()
	d, err := c.ReadFile(ctx, bucket, object)
	if err != nil {
		return nil, err
	}
	if int64(len(d)) > max {
		return nil, fmt.Errorf("%w (%d bytes)", storage.ErrTooLarge, max)
	}
	return d, nil
}

// LimitedReads counts ReadFileLimited calls.
func (c *MemClient) LimitedReads() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.limited
}

// Keys lists a bucket's object keys.
func (c *MemClient) Keys(bucket string) []string {
	names, _ := c.ListFiles(context.Background(), bucket, "")
	return names
}

// StaticExporter returns a fixed export (or error) for every call, and
// records the guitars it was asked for.
type StaticExporter struct {
	mu      sync.Mutex
	Exp     *devicejobs.Export
	Err     error
	Guitars []string
}

func (s *StaticExporter) Export(_ context.Context, _ storage.StructuredPreset, guitar string) (*devicejobs.Export, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Guitars = append(s.Guitars, guitar)
	if s.Err != nil {
		return nil, s.Err
	}
	e := *s.Exp
	return &e, nil
}

// Set swaps the export (to simulate the preset changing between calls).
func (s *StaticExporter) Set(e *devicejobs.Export) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Exp = e
}

// FixtureExport is a small export in the plan §4 shape: two blocks, one
// parameter that differs between scenes.
func FixtureExport() *devicejobs.Export {
	return &devicejobs.Export{
		Format: "qc2-device-export/1",
		Payload: json.RawMessage(`{"format":"qc2-device-export/1","rows":[{"row":0,"in_port":1,"out_port":19,"blocks":[` +
			`{"col":0,"model_id":5011,"model":"Jewel","params":[{"index":0,"name":"COMP","display":3,"wire":0.3,"scene_b":{"display":5,"wire":0.5}}],"bypass":{"A":false,"B":false}},` +
			`{"col":1,"model_id":1201,"model":"US Double Nrm","params":[{"index":2,"name":"Gain","display":4.5,"wire":0.45}],"bypass":{"A":false,"B":false}}]}],` +
			`"scenes":[{"index":0,"label":"Rhythm"},{"index":1,"label":"Lead"}],"unresolved":[]}`),
		Summary: []devicejobs.PlanBlock{
			{Row: 0, Col: 0, Model: "Jewel", ModelID: 5011, Params: []devicejobs.PlanParam{{Name: "COMP", A: "3", B: "5"}}},
			{Row: 0, Col: 1, Model: "US Double Nrm", ModelID: 1201, Params: []devicejobs.PlanParam{{Name: "Gain", A: "4.5"}}},
		},
	}
}

// FakeAgent plays the home agent: it checks pending jobs with the contract's
// reference check (devicejobs.CheckPendingJob), keeps a ledger, writes signed
// results and heartbeats, and deletes processed jobs.
type FakeAgent struct {
	Client *MemClient
	Bucket string
	Keys   *devicejobs.Keyring
	Now    func() time.Time

	// Hostile, when set, is used for every agent-sourced string (slot names,
	// messages, firmware) to test sanitizing.
	Hostile string
	// AllowedSlots goes into heartbeats (default "20A-25H").
	AllowedSlots []string
	// ClaimBucket, when set, replaces the bucket named inside heartbeats and
	// results (objects still go to Bucket), to test the bucket binding.
	ClaimBucket string
	// ReadbackPayload, when set, is the qc-device-readback/1 payload every read_preset
	// answers with (default: a small legacy-shaped object that is not a valid payload).
	ReadbackPayload json.RawMessage
	// ReadbackName and ReadbackSlot override the preset name and slot a read answers
	// with (defaults "Clean Verb" and "1A"); Hostile still wins for the name.
	ReadbackName string
	ReadbackSlot string
	// LibraryPayload, when set, is the qc-device-library/1 payload every sync_device_library
	// answers with (default: one valid V2 factory capture).
	LibraryPayload json.RawMessage

	mu       sync.Mutex
	ledger   map[string]bool
	Outcomes map[string]devicejobs.Outcome // object key -> outcome of the check
	Executed []*devicejobs.Job
}

// NewFakeAgent builds an agent over the same client and bucket as QC-2.
func NewFakeAgent(c *MemClient, bucket string, keys *devicejobs.Keyring) *FakeAgent {
	return &FakeAgent{Client: c, Bucket: bucket, Keys: keys, Now: time.Now,
		ledger: map[string]bool{}, Outcomes: map[string]devicejobs.Outcome{}, AllowedSlots: []string{"20A-25H"}}
}

func (a *FakeAgent) now() time.Time { return a.Now().UTC().Truncate(time.Second) }

func (a *FakeAgent) claimBucket() string {
	if a.ClaimBucket != "" {
		return a.ClaimBucket
	}
	return a.Bucket
}

func (a *FakeAgent) text(s string) string {
	if a.Hostile != "" {
		return a.Hostile
	}
	return s
}

// Heartbeat writes a signed heartbeat dated at Now()+offset with the given state.
func (a *FakeAgent) Heartbeat(offset time.Duration, state string, current *string) error {
	t := a.now().Add(offset)
	fw := a.text("4.1.0")
	present := true
	hb := devicejobs.Heartbeat{
		V: 1, Type: devicejobs.TypeHeartbeat, Bucket: a.claimBucket(), AgentVersion: "0.1.0",
		StartedAt: devicejobs.FormatTime(t.Add(-time.Hour)), WrittenAt: devicejobs.FormatTime(t),
		IdleExitAt: devicejobs.FormatTime(t.Add(30 * time.Minute)), PollIntervalS: 5, State: state,
		CurrentJobID: current, Unit: devicejobs.UnitStatus{Present: &present, Firmware: &fw},
		ExpectedFirmware: "4.1.0", AllowedSlots: a.AllowedSlots, Setlist: "My Presets",
	}
	if a.Hostile != "" {
		hb.AgentVersion = a.Hostile
	}
	return a.writeSigned(devicejobs.HeartbeatKey, devicejobs.TagHeartbeat, hb)
}

func (a *FakeAgent) writeSigned(key, tag string, v any) error {
	p, err := json.Marshal(v)
	if err != nil {
		return err
	}
	env, err := a.Keys.Seal(tag, p)
	if err != nil {
		return err
	}
	return a.Client.WriteFile(context.Background(), a.Bucket, key, env)
}

// RunOnce processes every pending job once, in key order, and returns how
// many it executed.
func (a *FakeAgent) RunOnce() (int, error) {
	ctx := context.Background()
	names, err := a.Client.ListFiles(ctx, a.Bucket, devicejobs.PendingPrefix)
	if err != nil {
		return 0, err
	}
	n := 0
	for _, key := range names {
		data, err := a.Client.ReadFile(ctx, a.Bucket, key)
		if err != nil {
			continue
		}
		now := a.now()
		a.mu.Lock()
		seen := func(id string) bool {
			if a.ledger[id] {
				return true
			}
			_, err := a.Client.ReadFile(ctx, a.Bucket, devicejobs.DoneKey(id))
			return err == nil
		}
		j, cerr := devicejobs.CheckPendingJob(a.Keys, key, a.Bucket, now, data, seen)
		o := devicejobs.OutcomeOf(cerr)
		a.Outcomes[key] = o
		if o == devicejobs.OutcomeBadKey {
			a.mu.Unlock()
			continue
		}
		if j != nil {
			a.ledger[j.JobID] = true
		}
		a.mu.Unlock()
		switch o {
		case devicejobs.OutcomeOK:
			// Record before publishing the result, so anyone who sees the
			// result also sees the execution.
			a.mu.Lock()
			a.Executed = append(a.Executed, j)
			a.mu.Unlock()
			if err := a.writeSigned(devicejobs.DoneKey(j.JobID), devicejobs.TagResult, a.result(j, now)); err != nil {
				return n, err
			}
			n++
		case devicejobs.OutcomeExpired:
			r := a.base(j, now, devicejobs.StatusExpired)
			r.ReasonCode = "expired"
			if err := a.writeSigned(devicejobs.DoneKey(j.JobID), devicejobs.TagResult, r); err != nil {
				return n, err
			}
		}
		_ = a.Client.DeleteFile(ctx, a.Bucket, key)
	}
	return n, nil
}

func (a *FakeAgent) base(j *devicejobs.Job, now time.Time, status string) devicejobs.Result {
	fw := a.text("4.1.0")
	return devicejobs.Result{
		V: 1, Type: devicejobs.TypeResult, JobID: j.JobID, Bucket: a.claimBucket(), Kind: j.Kind, Status: status,
		Message: a.text("done"), StartedAt: devicejobs.FormatTime(now), FinishedAt: devicejobs.FormatTime(now),
		AgentVersion: "0.1.0", Firmware: &fw,
	}
}

func (a *FakeAgent) result(j *devicejobs.Job, now time.Time) devicejobs.Result {
	r := a.base(j, now, devicejobs.StatusSucceeded)
	switch j.Kind {
	case devicejobs.KindPushPreset:
		var args devicejobs.PushArgs
		_ = json.Unmarshal(j.Args, &args)
		cpu := 31.5
		rb := 0.3
		r.Push = &devicejobs.PushResult{Slot: args.Slot, Name: args.Name, CPULoad: &cpu,
			Diff: []devicejobs.DiffItem{{Row: 0, Col: 0, ParamIndex: 0, Expected: 0.3, Readback: &rb, OK: true}}}
	case devicejobs.KindListSlots:
		name := a.text("Clean Verb")
		r.Slots = &devicejobs.SlotsResult{Setlist: "My Presets", Slots: []devicejobs.SlotInfo{
			{Slot: "1A", State: "occupied", Name: &name},
			{Slot: "20A", State: "empty"},
			{Slot: "20B", State: "occupied", Name: &name, Owned: true},
		}}
	case devicejobs.KindSyncLibrary:
		payload := a.LibraryPayload
		if payload == nil {
			payload = json.RawMessage(`{"captures":[{"name":"CA John's","key":"` + strings.Repeat("ab", 32) +
				`","version":2,"kind":"factory","folder":"Factory Captures V2","cloud":false}],"skipped":0}`)
		}
		r.Library = &devicejobs.LibraryBlock{Format: devicejobs.LibraryFormat, Payload: payload}
	case devicejobs.KindReadPreset:
		def := "Clean Verb"
		if a.ReadbackName != "" {
			def = a.ReadbackName
		}
		name := a.text(def)
		slot := "1A"
		if a.ReadbackSlot != "" {
			slot = a.ReadbackSlot
		}
		payload := a.ReadbackPayload
		if payload == nil {
			payload = json.RawMessage(fmt.Sprintf(`{"rows":[{"row":0,"blocks":[{"col":0,"model_id":5011,"label":%q}]}]}`, a.text("Jewel")))
		}
		r.Readback = &devicejobs.ReadbackBlock{Format: devicejobs.ReadbackFormat, Slot: &slot, Name: &name, Payload: payload}
	}
	return r
}

// ExecutedJobs is a copy of the executed jobs (safe while the agent runs in
// another goroutine).
func (a *FakeAgent) ExecutedJobs() []*devicejobs.Job {
	a.mu.Lock()
	defer a.mu.Unlock()
	return append([]*devicejobs.Job(nil), a.Executed...)
}

// Ledger reports whether the agent has taken job id.
func (a *FakeAgent) Ledger(id string) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.ledger[id]
}
