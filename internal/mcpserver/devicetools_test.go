package mcpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/weitzer-org/sound-builder/internal/devicejobs"
	"github.com/weitzer-org/sound-builder/internal/devicejobs/devicejobstest"
)

const (
	devJobsBucket = "qc2-device-jobs-test"
	// Test key: bytes 0x00..0x1f, the contract's vector key. Not a secret.
	devKeyHex = "000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f"
	devGuitar = "Fender Telecaster Single Coil"
)

var deviceToolNames = []string{deviceStatusToolName, deviceJobToolName, deviceSlotsToolName, deviceReadToolName, devicePushToolName}

type deviceRig struct {
	h      *svcHarness
	cs     *mcp.ClientSession
	bridge *devicejobs.Bridge
	agent  *devicejobstest.FakeAgent
	exp    *devicejobstest.StaticExporter
	client *devicejobstest.MemClient
}

// newDeviceRig serves the MCP routes WithDevice over a fake jobs bucket, a
// fixture exporter and a fake home agent. clock nil means real time.
func newDeviceRig(t *testing.T, clock func() time.Time) *deviceRig {
	t.Helper()
	return newDeviceRigWith(t, clock)
}

// newDeviceRigWith is newDeviceRig plus extra server options (e.g. WithImporter).
func newDeviceRigWith(t *testing.T, clock func() time.Time, extra ...Option) *deviceRig {
	t.Helper()
	kr, err := devicejobs.NewKeyring(devKeyHex, "")
	if err != nil {
		t.Fatal(err)
	}
	h := newTestService(t)
	c := devicejobstest.NewMemClient()
	exp := &devicejobstest.StaticExporter{Exp: devicejobstest.FixtureExport()}
	b := devicejobs.NewBridge(c, &devicejobs.Config{Bucket: devJobsBucket, Keys: kr}, exp)
	b.PollGap = 10 * time.Millisecond
	a := devicejobstest.NewFakeAgent(c, devJobsBucket, kr)
	if clock != nil {
		b.Now, a.Now = clock, clock
	}
	mux := http.NewServeMux()
	Mount(mux, h.svc, Config{Token: testToken}, nil, append([]Option{WithDevice(b)}, extra...)...)
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)
	hc := &http.Client{Transport: itBearer{token: testToken, base: http.DefaultTransport}}
	cs, err := itConnect(t, ts.URL+"/mcp", hc)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cs.Close() })
	return &deviceRig{h: h, cs: cs, bridge: b, agent: a, exp: exp, client: c}
}

func (r *deviceRig) online(t *testing.T) {
	t.Helper()
	if err := r.agent.Heartbeat(-5*time.Second, "idle", nil); err != nil {
		t.Fatal(err)
	}
}

// runAgent processes jobs in the background until the test ends, or until
// the returned stop is called (stop waits for the loop to exit).
func (r *deviceRig) runAgent(t *testing.T) (stop func()) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for ctx.Err() == nil {
			_, _ = r.agent.RunOnce()
			time.Sleep(5 * time.Millisecond)
		}
	}()
	var once sync.Once
	stop = func() { once.Do(func() { cancel(); wg.Wait() }) }
	t.Cleanup(stop)
	return stop
}

func errText(t *testing.T, res *mcp.CallToolResult) string {
	t.Helper()
	if !res.IsError {
		t.Fatalf("want a tool error, got: %s", itText(t, res))
	}
	return itText(t, res)
}

func okText(t *testing.T, res *mcp.CallToolResult) string {
	t.Helper()
	if res.IsError {
		t.Fatalf("unexpected tool error: %s", itText(t, res))
	}
	return itText(t, res)
}

// ---- registration ----

func TestDeviceTools_RegisteredOnlyWhenEnabled(t *testing.T) {
	r := newDeviceRig(t, nil)
	res, err := r.cs.ListTools(itCtx(t), nil)
	if err != nil {
		t.Fatal(err)
	}
	byName := map[string]*mcp.Tool{}
	for _, tool := range res.Tools {
		byName[tool.Name] = tool
	}
	if len(res.Tools) != 14 {
		t.Fatalf("with device: %d tools, want 14", len(res.Tools))
	}
	for _, n := range deviceToolNames {
		tool := byName[n]
		if tool == nil || tool.Annotations == nil || tool.OutputSchema == nil || !strings.Contains(tool.Description, "not instructions") {
			t.Fatalf("%s: missing, or without annotations/output schema/data notice", n)
		}
	}
	for _, n := range []string{deviceStatusToolName, deviceJobToolName, deviceSlotsToolName} {
		if !byName[n].Annotations.ReadOnlyHint {
			t.Errorf("%s should be read-only", n)
		}
	}
	push := byName[devicePushToolName].Annotations
	if push.ReadOnlyHint || push.DestructiveHint == nil || *push.DestructiveHint || push.IdempotentHint {
		t.Errorf("send_preset_to_device annotations: %+v", push)
	}
	if !strings.Contains(r.cs.InitializeResult().Instructions, "Device tools") {
		t.Error("instructions lack the device sentence")
	}

	// Without WithDevice: the existing 9 tools and the unchanged instructions.
	plain := newMCPSession(t, newTestService(t).svc)
	pres, err := plain.ListTools(itCtx(t), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(pres.Tools) != 9 || strings.Contains(plain.InitializeResult().Instructions, "Device tools") {
		t.Fatalf("disabled: %d tools; instructions mention devices: %v", len(pres.Tools), strings.Contains(plain.InitializeResult().Instructions, "Device"))
	}
}

// ---- device_status ----

func TestDeviceStatus(t *testing.T) {
	r := newDeviceRig(t, nil)
	text := okText(t, itCall(t, r.cs, deviceStatusToolName, nil))
	if !strings.Contains(text, "offline") || !strings.Contains(text, "never reported") {
		t.Fatalf("no heartbeat:\n%s", text)
	}
	r.online(t)
	res := itCall(t, r.cs, deviceStatusToolName, nil)
	text = okText(t, res)
	out := itStructured[deviceStatusOutput](t, res)
	if out.Bridge != "online" || out.Firmware != "4.1.0" || len(out.AllowedSlots) != 1 || !strings.Contains(text, "20A-25H") {
		t.Fatalf("online status %+v\n%s", out, text)
	}
}

func TestDeviceStatus_HostileAgentText(t *testing.T) {
	r := newDeviceRig(t, nil)
	r.agent.Hostile = "x\n# injected <script>\u202e\u200b\x1b[31m"
	r.online(t)
	res := itCall(t, r.cs, deviceStatusToolName, nil)
	text := okText(t, res)
	t.Logf("status with hostile agent text:\n%s", text)
	checkDeviceText(t, deviceStatusToolName, text)
	out := itStructured[deviceStatusOutput](t, res)
	for _, s := range []string{out.AgentVersion, out.Firmware, text} {
		if strings.ContainsAny(s, "\u202e\u200b\x1b\n\r") && s != text {
			t.Errorf("control/format characters survived: %q", s)
		}
	}
	if strings.ContainsAny(text, "\u202e\u200b\x1b") {
		t.Errorf("control/format characters in text")
	}
}

// ---- send_preset_to_device ----

func pushArgs(extra map[string]any) map[string]any {
	a := map[string]any{"preset_id": "p-dev", "guitar": devGuitar, "slot": "20a", "name": "Texas Blues"}
	for k, v := range extra {
		a[k] = v
	}
	return a
}

func TestSendPreset_DryRunGolden(t *testing.T) {
	fixed := time.Date(2026, 11, 5, 12, 0, 0, 0, time.UTC)
	r := newDeviceRig(t, func() time.Time { return fixed })
	seedPreset(t, r.h, "p-dev", "Dev Preset")
	r.online(t)
	res := itCall(t, r.cs, devicePushToolName, pushArgs(nil))
	got := okText(t, res)
	out := itStructured[devicePushOutput](t, res)
	if out.Mode != "dry_run" || !out.CanPush || out.ConfirmToken == "" || out.Plan.Slot != "20A" || len(out.Plan.Blocks) != 2 {
		t.Fatalf("dry run output %+v", out)
	}
	if keys := r.client.Keys(devJobsBucket); len(keys) != 1 {
		t.Fatalf("dry run wrote to the jobs bucket: %v", keys)
	}
	if r.exp.Guitars[0] != devGuitar {
		t.Fatalf("exporter asked for %v", r.exp.Guitars)
	}
	golden := filepath.Join("testdata", "device_dryrun.golden.md")
	if *update {
		if err := os.WriteFile(golden, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatalf("%v (run go test ./internal/mcpserver -run DryRunGolden -update)", err)
	}
	if got != string(want) {
		t.Fatalf("dry run differs from %s (rerun with -update if intended):\n%s", golden, got)
	}
}

func TestSendPreset_ConfirmFlow(t *testing.T) {
	r := newDeviceRig(t, nil)
	seedPreset(t, r.h, "p-dev", "Dev Preset")
	r.online(t)
	stopAgent := r.runAgent(t)

	dry := itStructured[devicePushOutput](t, itCall(t, r.cs, devicePushToolName, pushArgs(nil)))
	res := itCall(t, r.cs, devicePushToolName, pushArgs(map[string]any{"dry_run": false, "confirm_token": dry.ConfirmToken, "wait_seconds": 5}))
	text := okText(t, res)
	out := itStructured[devicePushOutput](t, res)
	if out.Mode != "queued" || out.Job == nil || out.Job.State != devicejobs.StatusSucceeded || out.Job.Push == nil || out.Job.Push.Slot != "20A" {
		t.Fatalf("confirm output %+v\n%s", out, text)
	}
	if ex := r.agent.ExecutedJobs(); len(ex) != 1 || ex[0].Kind != devicejobs.KindPushPreset {
		t.Fatalf("agent executed %v", ex)
	}
	// A repeat confirm names the same job and queues nothing.
	again := itStructured[devicePushOutput](t, itCall(t, r.cs, devicePushToolName,
		pushArgs(map[string]any{"dry_run": false, "confirm_token": dry.ConfirmToken, "wait_seconds": 0})))
	if again.Mode != "existing" || again.Job.JobID != out.Job.JobID {
		t.Fatalf("repeat confirm %+v", again)
	}
	// Deterministic "never twice": stop the background agent, then give it
	// one more full pass over the bucket.
	stopAgent()
	if _, err := r.agent.RunOnce(); err != nil {
		t.Fatal(err)
	}
	if n := len(r.agent.ExecutedJobs()); n != 1 {
		t.Fatalf("job executed %d times", n)
	}
	// get_device_job reports the same result.
	jo := itStructured[deviceJobOutput](t, itCall(t, r.cs, deviceJobToolName, map[string]any{"job_id": out.Job.JobID}))
	if jo.State != devicejobs.StatusSucceeded || jo.Kind != devicejobs.KindPushPreset {
		t.Fatalf("get_device_job %+v", jo)
	}
}

func TestSendPreset_Refusals(t *testing.T) {
	r := newDeviceRig(t, nil)
	seedPreset(t, r.h, "p-dev", "Dev Preset")
	dry := itStructured[devicePushOutput](t, itCall(t, r.cs, devicePushToolName, pushArgs(nil)))
	if dry.BridgeOnline || !strings.Contains(dry.BridgeReason, "never reported") {
		t.Fatalf("offline dry run %+v", dry)
	}
	cases := []struct {
		name string
		args map[string]any
		want string
	}{
		{"token with dry run", pushArgs(map[string]any{"confirm_token": dry.ConfirmToken}), "only used with dry_run=false"},
		{"confirm without token", pushArgs(map[string]any{"dry_run": false}), "needs the confirm_token"},
		{"offline confirm", pushArgs(map[string]any{"dry_run": false, "confirm_token": dry.ConfirmToken}), "Not queued"},
		{"bad slot", pushArgs(map[string]any{"slot": "33A"}), "bank 1-32"},
		{"bad slot letter", pushArgs(map[string]any{"slot": "20I"}), "bank 1-32"},
		{"name too long", pushArgs(map[string]any{"name": strings.Repeat("x", 33)}), "1-32 printable ASCII"},
		{"name with emoji", pushArgs(map[string]any{"name": "Blues 🎸"}), "1-32 printable ASCII"},
		{"name with newline", pushArgs(map[string]any{"name": "a\nb"}), "1-32 printable ASCII"},
		{"unknown guitar", pushArgs(map[string]any{"guitar": "Strat"}), "Gibson ES-339 Humbuckers"},
		{"bad preset id", pushArgs(map[string]any{"preset_id": "../x"}), "preset_id must be"},
		{"missing preset", pushArgs(map[string]any{"preset_id": "nope"}), "does not exist"},
		{"wait too long", pushArgs(map[string]any{"wait_seconds": 500}), "wait_seconds"},
	}
	// Every case above runs with the bridge offline; none changes the rig.
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			text := errText(t, itCall(t, r.cs, devicePushToolName, c.args))
			if !strings.Contains(text, c.want) {
				t.Fatalf("error %q, want it to contain %q", text, c.want)
			}
		})
	}
	// Online from here on, so the token check (not the offline check) is
	// what refuses the forged token.
	r.online(t)
	t.Run("forged token", func(t *testing.T) {
		text := errText(t, itCall(t, r.cs, devicePushToolName,
			pushArgs(map[string]any{"dry_run": false, "confirm_token": "c1.9999999999.00000000000000000000000000000000"})))
		if !strings.Contains(text, "does not match") {
			t.Fatalf("forged token: %q", text)
		}
	})
	if keys := r.client.Keys(devJobsBucket); len(keys) != 1 || keys[0] != devicejobs.HeartbeatKey {
		t.Fatalf("a refused call wrote jobs: %v", keys)
	}
}

func TestSendPreset_PresetChangedAndUnavailableExport(t *testing.T) {
	r := newDeviceRig(t, nil)
	seedPreset(t, r.h, "p-dev", "Dev Preset")
	r.online(t)
	dry := itStructured[devicePushOutput](t, itCall(t, r.cs, devicePushToolName, pushArgs(nil)))
	e := devicejobstest.FixtureExport()
	e.Payload = []byte(strings.Replace(string(e.Payload), `"wire":0.3,`, `"wire":0.31,`, 1))
	r.exp.Set(e)
	text := errText(t, itCall(t, r.cs, devicePushToolName, pushArgs(map[string]any{"dry_run": false, "confirm_token": dry.ConfirmToken})))
	if !strings.Contains(text, "preset changed") {
		t.Fatalf("changed export: %s", text)
	}
	e.Unresolved = []devicejobs.Unresolved{{Block: "Spring <M>", Param: "Mix", Reason: "unmeasured bound\n# x"}}
	r.exp.Set(e)
	res := itCall(t, r.cs, devicePushToolName, pushArgs(nil))
	text = okText(t, res)
	if out := itStructured[devicePushOutput](t, res); out.CanPush || out.ConfirmToken != "" || !strings.Contains(text, "cannot be pushed") {
		t.Fatalf("unresolved plan %+v\n%s", out, text)
	}
	if strings.Contains(text, "\n# x") || strings.Contains(text, "<M>") {
		t.Fatalf("unresolved text not escaped:\n%s", text)
	}
	r.bridge.Exporter = devicejobs.UnavailableExporter{}
	text = errText(t, itCall(t, r.cs, devicePushToolName, pushArgs(nil)))
	if !strings.Contains(text, "not available yet") {
		t.Fatalf("stub exporter: %s", text)
	}
}

// ---- reads ----

func TestDeviceReads(t *testing.T) {
	r := newDeviceRig(t, nil)
	if text := errText(t, itCall(t, r.cs, deviceSlotsToolName, nil)); !strings.Contains(text, "Not queued") {
		t.Fatalf("offline read: %s", text)
	}
	r.online(t)

	// Slow agent: the call returns a queued job; get_device_job finishes it.
	res := itCall(t, r.cs, deviceSlotsToolName, map[string]any{"wait_seconds": 0})
	text := okText(t, res)
	q := itStructured[deviceJobOutput](t, res)
	if q.State != devicejobs.StateQueued || !strings.Contains(text, "get_device_job") {
		t.Fatalf("queued read %+v\n%s", q, text)
	}
	r.runAgent(t)
	res = itCall(t, r.cs, deviceJobToolName, map[string]any{"job_id": q.JobID, "wait_seconds": 5})
	jo := itStructured[deviceJobOutput](t, res)
	if jo.State != devicejobs.StatusSucceeded || jo.Slots == nil || len(jo.Slots.Slots) != 3 {
		t.Fatalf("slots %+v", jo)
	}
	if text := okText(t, res); !strings.Contains(text, "Empty slots: `20A`") || !strings.Contains(text, "created by this app") {
		t.Fatalf("slots text:\n%s", text)
	}

	// Agent answering within the wait.
	res = itCall(t, r.cs, deviceReadToolName, map[string]any{"wait_seconds": 5})
	rb := itStructured[deviceJobOutput](t, res)
	if rb.State != devicejobs.StatusSucceeded || rb.Readback == nil || rb.Readback.Name != "Clean Verb" {
		t.Fatalf("read %+v", rb)
	}
	if text := okText(t, res); !strings.Contains(text, "```json") {
		t.Fatalf("read text:\n%s", text)
	}
}

func TestDeviceReads_HostileDeviceText(t *testing.T) {
	r := newDeviceRig(t, nil)
	r.agent.Hostile = "x\n# injected <script>\u202e```"
	r.online(t)
	r.runAgent(t)
	for _, tool := range []string{deviceSlotsToolName, deviceReadToolName} {
		res := itCall(t, r.cs, tool, map[string]any{"wait_seconds": 5})
		text := okText(t, res)
		t.Logf("%s with hostile device text:\n%s", tool, text)
		if strings.Contains(text, "\n# injected") || strings.ContainsRune(text, '\u202e') {
			t.Errorf("%s: hostile text survived", tool)
		}
		checkDeviceText(t, tool, text)
		if strings.Count(text, "```") > 2 {
			t.Errorf("%s: device text opened a code fence", tool)
		}
	}
}

// checkDeviceText asserts hostile device text ("x\n# injected <script>...")
// arrives only inside an inline code span: no heading line, no bare tag.
func checkDeviceText(t *testing.T, tool, text string) {
	t.Helper()
	for _, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(line, "# injected") {
			t.Errorf("%s: device text opened a heading line: %q", tool, line)
		}
		for i := strings.Index(line, "<script"); i >= 0; {
			if strings.Count(line[:i], "`")%2 != 1 {
				t.Errorf("%s: '<script' outside a code span: %q", tool, line)
			}
			break
		}
	}
	if !strings.Contains(text, "`x # injected <script>") {
		t.Errorf("%s: expected the device text inside a code span", tool)
	}
}

func TestDeviceText_LinksAndImagesDoNotRender(t *testing.T) {
	r := newDeviceRig(t, nil)
	r.agent.Hostile = "x![p](https://evil.example/t.png) [Fix it](https://evil.example) *b* <b>"
	r.online(t)
	r.runAgent(t)
	text := okText(t, itCall(t, r.cs, deviceSlotsToolName, map[string]any{"wait_seconds": 5}))
	want := "`x![p](https://evil.example/t.png) [Fix it](https://evil.example) *b* <b>`"
	if !strings.Contains(text, want) {
		t.Fatalf("device name not confined to a code span:\n%s", text)
	}
	for _, line := range strings.Split(text, "\n") {
		if i := strings.Index(line, "](https://evil"); i >= 0 && strings.Count(line[:i], "`")%2 != 1 {
			t.Errorf("link outside a code span: %q", line)
		}
	}
}

func TestGetDeviceJob_Validation(t *testing.T) {
	r := newDeviceRig(t, nil)
	for _, id := range []string{"", "../x", "6F1C2E1A-3B7D-4C55-9A0E-2F4B8D9C1E01", "{6f1c2e1a-3b7d-4c55-9a0e-2f4b8d9c1e01}"} {
		if text := errText(t, itCall(t, r.cs, deviceJobToolName, map[string]any{"job_id": id})); !strings.Contains(text, "job_id must be") {
			t.Errorf("%q: %s", id, text)
		}
	}
	jo := itStructured[deviceJobOutput](t, itCall(t, r.cs, deviceJobToolName, map[string]any{"job_id": "6f1c2e1a-3b7d-4c55-9a0e-2f4b8d9c1e01"}))
	if jo.State != devicejobs.StateNotFound {
		t.Fatalf("unknown id: %+v", jo)
	}
}

// Nothing the device path logs may contain the key, a signature or an envelope.
func TestDeviceTools_NoSecretsInLogs(t *testing.T) {
	var buf bytes.Buffer
	prev := log.Writer()
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(prev) })
	r := newDeviceRig(t, nil)
	seedPreset(t, r.h, "p-dev", "Dev Preset")
	r.online(t)
	r.runAgent(t)
	dry := itStructured[devicePushOutput](t, itCall(t, r.cs, devicePushToolName, pushArgs(nil)))
	itCall(t, r.cs, devicePushToolName, pushArgs(map[string]any{"dry_run": false, "confirm_token": dry.ConfirmToken, "wait_seconds": 5}))
	itCall(t, r.cs, deviceSlotsToolName, map[string]any{"wait_seconds": 5})
	// Force an internal error path too.
	r.client.SetFailList(true)
	itCall(t, r.cs, deviceStatusToolName, nil)
	logs := buf.String()
	for _, secret := range []string{devKeyHex, devKeyHex[:16], `"sig"`, dry.ConfirmToken} {
		if strings.Contains(logs, secret) {
			t.Fatalf("logs contain %q:\n%s", secret, logs)
		}
	}
}

func TestSanitizeDeviceTextBoundsWork(t *testing.T) {
	long := strings.Repeat("ab ", 1<<20) // 3 MiB
	if got := sanitizeDeviceText(long, 10); got != "ab ab ab a…" {
		t.Fatalf("got %q", got)
	}
	// A cut through a multi-byte rune never leaks an invalid sequence.
	if got := sanitizeDeviceText(strings.Repeat("é", 100), 2); got != "éé…" {
		t.Fatalf("got %q", got)
	}
	allocs := testing.AllocsPerRun(5, func() { _ = sanitizeDeviceText(long, 200) })
	if allocs > 50 {
		t.Fatalf("%v allocations for a 3 MiB input", allocs)
	}
}

func TestSanitizeJSONValueDepth(t *testing.T) {
	deep := strings.Repeat("[", 200) + `"x"` + strings.Repeat("]", 200)
	out, err := json.Marshal(sanitizeJSONValue(json.RawMessage(deep)))
	if err != nil || !strings.Contains(string(out), "nested too deeply") || strings.Count(string(out), "[") > devicejobs.MaxDepth {
		t.Fatalf("got %s (%v)", out, err)
	}
}
