package mcpserver

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/weitzer-org/sound-builder/internal/catalog"
	"github.com/weitzer-org/sound-builder/internal/deviceexport"
	"github.com/weitzer-org/sound-builder/internal/devicejobs"
	"github.com/weitzer-org/sound-builder/internal/devicejobs/devicejobstest"
	"github.com/weitzer-org/sound-builder/internal/devicelib"
	"github.com/weitzer-org/sound-builder/internal/storage"
)

const mainBucket = "main-test"

// libRig is a device rig whose exporter is the real one, reading the synced lists the sync tool
// stores, so a sync followed by a dry run is the whole P4b path.
type libRig struct {
	*deviceRig
	store *devicelib.Store
}

func newLibRig(t *testing.T) *libRig {
	t.Helper()
	kr, err := devicejobs.NewKeyring(devKeyHex, "")
	if err != nil {
		t.Fatal(err)
	}
	h := newTestService(t)
	c := devicejobstest.NewMemClient()
	cat, err := catalog.Embedded()
	if err != nil {
		t.Fatal(err)
	}
	ls := devicelib.NewStore(c, mainBucket)
	al, err := devicelib.EmbeddedAliases()
	if err != nil {
		t.Fatal(err)
	}
	src := &devicelib.Source{Store: ls, Aliases: al}
	exp, err := deviceexport.NewEmbedded(deviceexport.Options{
		CaptureSource: func(n string) string {
			if n == "CA John's" {
				return "factory"
			}
			return ""
		},
		Device: src.Snapshot,
	})
	if err != nil {
		t.Fatal(err)
	}
	b := devicejobs.NewBridge(c, &devicejobs.Config{Bucket: devJobsBucket, Keys: kr}, exp)
	b.PollGap = 10 * time.Millisecond
	a := devicejobstest.NewFakeAgent(c, devJobsBucket, kr)
	lib := &DeviceLibrary{Store: ls, Harvester: devicelib.NewHarvester(cat)}
	mux := http.NewServeMux()
	Mount(mux, h.svc, Config{Token: testToken}, nil, WithDevice(b), WithExporter(exp), WithImporter(exp), WithDeviceLibrary(lib))
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)
	hc := &http.Client{Transport: itBearer{token: testToken, base: http.DefaultTransport}}
	cs, err := itConnect(t, ts.URL+"/mcp", hc)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cs.Close() })
	return &libRig{deviceRig: &deviceRig{h: h, cs: cs, bridge: b, agent: a, client: c}, store: ls}
}

func seedCapturePreset(t *testing.T, r *libRig, id string) {
	t.Helper()
	seedDevicePreset(t, r.h, id, map[string][]storage.EffectBlock{devGuitar: {
		{Model: "CA John's", Parameters: []storage.BlockParameter{{Name: "Gain", Value: "0 dB", Basis: "estimate"}}},
	}})
}

func dryRun(t *testing.T, r *libRig, id string) (string, devicePushOutput) {
	t.Helper()
	res := itCall(t, r.cs, devicePushToolName, map[string]any{"preset_id": id, "guitar": devGuitar, "slot": "20A", "name": "Lib test"})
	return okText(t, res), itStructured[devicePushOutput](t, res)
}

func TestSyncDeviceLibrary_StoresTheListAndTheDryRunUsesIt(t *testing.T) {
	r := newLibRig(t)
	r.online(t)
	r.runAgent(t)
	seedCapturePreset(t, r, "p-lib")

	// Before any sync: the capture is "found on the unit at push time", as it always was.
	text, out := dryRun(t, r, "p-lib")
	if !strings.Contains(text, "found on the unit at push time") || !out.CanPush {
		t.Fatalf("before a sync:\n%s", text)
	}

	res := itCall(t, r.cs, deviceSyncToolName, map[string]any{"wait_seconds": 10})
	text = okText(t, res)
	jo := itStructured[deviceJobOutput](t, res)
	if jo.State != devicejobs.StatusSucceeded || jo.Library == nil || jo.Library.Status != "stored" || jo.Library.Total != 1 || jo.Library.V2 != 1 {
		t.Fatalf("sync output %+v\n%s", jo.Library, text)
	}
	if !strings.Contains(text, "Capture list stored") || !strings.Contains(text, "1 captures") {
		t.Fatalf("sync text:\n%s", text)
	}
	stored, err := r.store.Library(context.Background())
	if err != nil || stored == nil || len(stored.Captures) != 1 || stored.Captures[0].Name != "CA John's" || stored.JobID != jo.JobID {
		t.Fatalf("stored %+v %v", stored, err)
	}

	text, out = dryRun(t, r, "p-lib")
	if !out.CanPush || !strings.Contains(text, "capture on your unit: V2") || !strings.Contains(text, "Factory Captures V2") {
		t.Fatalf("dry run after the sync:\n%s", text)
	}
	if out.Plan.Blocks[0].CaptureInfo == nil || out.Plan.Blocks[0].CaptureInfo.KeyPrefix != "abababab" {
		t.Fatalf("plan block %+v", out.Plan.Blocks[0])
	}

	// A unit that doesn't have the capture: "not on your unit", and no confirm token.
	r.agent.LibraryPayload = json.RawMessage(`{"captures":[{"name":"Brit 2203 87","key":"` + strings.Repeat("d4", 32) +
		`","version":1,"kind":"factory","folder":"Factory Captures V1","cloud":false}],"skipped":0}`)
	res = itCall(t, r.cs, deviceSyncToolName, map[string]any{"wait_seconds": 10})
	if jo = itStructured[deviceJobOutput](t, res); jo.Library == nil || jo.Library.Status != "stored" {
		t.Fatalf("second sync: %+v", jo.Library)
	}
	text, out = dryRun(t, r, "p-lib")
	if out.CanPush || out.ConfirmToken != "" || !strings.Contains(text, "not on your unit") || !strings.Contains(text, "Unresolved") {
		t.Fatalf("dry run for a capture the unit lacks:\n%s", text)
	}
}

func TestSyncDeviceLibrary_HostileAndMalformedAnswers(t *testing.T) {
	r := newLibRig(t)
	r.online(t)
	r.runAgent(t)
	bad := map[string]string{
		"hostile name":   `{"captures":[{"name":"x\n# injected <script>","key":"` + strings.Repeat("0", 64) + `","version":1,"kind":"user","folder":"F","cloud":false}],"skipped":0}`,
		"non-ASCII":      `{"captures":[{"name":"Caf\u00e9","key":"` + strings.Repeat("0", 64) + `","version":1,"kind":"user","folder":"F","cloud":false}],"skipped":0}`,
		"bidi in folder": `{"captures":[{"name":"A","key":"` + strings.Repeat("0", 64) + `","version":1,"kind":"user","folder":"F\u202eG","cloud":false}],"skipped":0}`,
		"unknown field":  `{"captures":[],"skipped":0,"x":1}`,
		"not an object":  `[]`,
	}
	for name, payload := range bad {
		r.agent.LibraryPayload = json.RawMessage(payload)
		res := itCall(t, r.cs, deviceSyncToolName, map[string]any{"wait_seconds": 10})
		text := okText(t, res)
		jo := itStructured[deviceJobOutput](t, res)
		if jo.Library == nil || jo.Library.Status != "not_stored" {
			t.Errorf("%s: %+v", name, jo.Library)
		}
		if strings.Contains(text, "# injected") || strings.ContainsRune(text, '\u202e') {
			t.Errorf("%s: device text reached the answer:\n%s", name, text)
		}
		if got, _ := r.store.Library(context.Background()); got != nil {
			t.Errorf("%s: something was stored", name)
		}
	}
}

func TestSyncDeviceLibrary_BridgeOfflineAndAnOlderAnswerNeverWins(t *testing.T) {
	r := newLibRig(t)
	if text := errText(t, itCall(t, r.cs, deviceSyncToolName, nil)); !strings.Contains(text, "Not queued") {
		t.Fatalf("offline: %s", text)
	}
	// An answer older than the stored list is not used.
	lib := &DeviceLibrary{Store: r.store, Now: func() time.Time { return time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC) }}
	newer := devicelib.NewLibrary(&devicejobs.LibraryPayload{Captures: []devicejobs.LibraryCapture{}}, "11111111-1111-4111-8111-111111111111", "4.1.0",
		time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC))
	if err := r.store.SaveLibrary(context.Background(), newer); err != nil {
		t.Fatal(err)
	}
	payload := json.RawMessage(`{"captures":[],"skipped":0}`)
	v := &devicejobs.JobView{JobID: "22222222-2222-4222-8222-222222222222", State: devicejobs.StatusSucceeded, Result: &devicejobs.Result{
		Kind: devicejobs.KindSyncLibrary, Status: devicejobs.StatusSucceeded, FinishedAt: "2026-11-06T11:00:00Z",
		Library: &devicejobs.LibraryBlock{Format: devicejobs.LibraryFormat, Payload: payload}}}
	if out := lib.storeSync(context.Background(), v); out == nil || out.Status != "not_stored" || !strings.Contains(out.Reason, "newer") {
		t.Fatalf("older answer: %+v", out)
	}
	v.Result.FinishedAt = "2026-11-06T13:00:00Z"
	if out := lib.storeSync(context.Background(), v); out == nil || out.Status != "stored" {
		t.Fatalf("newer answer: %+v", out)
	}
	// a failed or wrong-format result changes nothing
	v.Result.Status = devicejobs.StatusFailed
	if out := lib.storeSync(context.Background(), v); out == nil || out.Status != "not_stored" {
		t.Fatalf("failed job: %+v", out)
	}
	v.Result.Status, v.Result.Library.Format = devicejobs.StatusSucceeded, "qc-device-library/9"
	if out := lib.storeSync(context.Background(), v); out == nil || out.Status != "not_stored" {
		t.Fatalf("wrong format: %+v", out)
	}
	// other kinds are not library answers
	v.Result.Kind = devicejobs.KindListSlots
	if out := lib.storeSync(context.Background(), v); out != nil {
		t.Fatalf("a list_slots result produced %+v", out)
	}
}

// A late sync (the first call returned while the bridge was still working) is stored by get_device_job.
func TestSyncDeviceLibrary_GetDeviceJobStoresALateAnswer(t *testing.T) {
	r := newLibRig(t)
	r.online(t)
	res := itCall(t, r.cs, deviceSyncToolName, map[string]any{"wait_seconds": 0})
	q := itStructured[deviceJobOutput](t, res)
	if q.State != devicejobs.StateQueued || !strings.Contains(okText(t, res), "get_device_job") {
		t.Fatalf("queued: %+v", q)
	}
	if got, _ := r.store.Library(context.Background()); got != nil {
		t.Fatal("stored before the job finished")
	}
	r.runAgent(t)
	res = itCall(t, r.cs, deviceJobToolName, map[string]any{"job_id": q.JobID, "wait_seconds": 10})
	jo := itStructured[deviceJobOutput](t, res)
	if jo.Library == nil || jo.Library.Status != "stored" {
		t.Fatalf("late answer: %+v", jo)
	}
	if got, _ := r.store.Library(context.Background()); got == nil || got.JobID != q.JobID {
		t.Fatalf("stored %+v", got)
	}
}

// read_device_preset records the cab mics it sees, and the next dry run writes one exactly.
func TestReadDevicePreset_RecordsCabMicsOnly(t *testing.T) {
	r := newLibRig(t)
	r.online(t)
	r.runAgent(t)
	seen := "ME_110 US PRN C10R_Ribbon 121"
	vals := func(v string) string {
		return `["` + v + `","` + v + `","` + v + `","` + v + `","` + v + `","` + v + `","` + v + `","` + v + `"]`
	}
	r.agent.ReadbackPayload = json.RawMessage(`{"setlist":{"folder":"other","position":null},"chains":[{"row":0,"in_port":1,"out_port":19,"blocks":[` +
		`{"col":0,"model_id":12045,"params":[{"index":1,"scene_mode":false,"values":` + vals(seen) + `},{"index":9,"scene_mode":false,"values":` + vals("C:\\\\evil_x") + `}]}]}],"bypass":null}`)
	res := itCall(t, r.cs, deviceReadToolName, map[string]any{"wait_seconds": 10})
	text := okText(t, res)
	jo := itStructured[deviceJobOutput](t, res)
	if jo.Mics == nil || jo.Mics.NewlyRecorded != 1 || jo.Mics.Ignored != 1 || !strings.Contains(text, "1 new recorded") {
		t.Fatalf("mics %+v\n%s", jo.Mics, text)
	}
	m, err := r.store.Mics(context.Background())
	if err != nil || m == nil || len(m.For(12045)) != 1 || m.For(12045)[0] != seen {
		t.Fatalf("stored mics %+v %v", m, err)
	}
	// reading again records nothing new
	jo = itStructured[deviceJobOutput](t, itCall(t, r.cs, deviceReadToolName, map[string]any{"wait_seconds": 10}))
	if jo.Mics == nil || jo.Mics.NewlyRecorded != 0 {
		t.Fatalf("second read: %+v", jo.Mics)
	}
	// the preset itself was not stored
	if ids, _ := r.h.store.List(context.Background()); len(ids) != 0 {
		t.Fatalf("a read stored %d presets", len(ids))
	}

	seedDevicePreset(t, r.h, "p-mic", map[string][]storage.EffectBlock{devGuitar: {
		{Model: "110 US PRN C10R (M)", Parameters: []storage.BlockParameter{{Name: "Mic 1", Value: "Ribbon 121", Basis: "estimate"}}},
	}})
	text, out := dryRun(t, r, "p-mic")
	if !out.CanPush || !strings.Contains(text, "cab mic written exactly: MIC 1 = ") || !strings.Contains(text, seen) {
		t.Fatalf("dry run should write the seen mic:\n%s", text)
	}
	if !strings.Contains(text, "UNVERIFIED") || len(out.Plan.Blocks[0].Mics) != 1 {
		t.Fatalf("mic plan:\n%s", text)
	}
}

func TestDeviceTools_LibraryToolOnlyWithALibrary(t *testing.T) {
	r := newLibRig(t)
	res, err := r.cs.ListTools(itCtx(t), nil)
	if err != nil {
		t.Fatal(err)
	}
	var tool *mcp.Tool
	for _, x := range res.Tools {
		if x.Name == deviceSyncToolName {
			tool = x
		}
	}
	if tool == nil || len(res.Tools) != 15 {
		t.Fatalf("%d tools, sync tool %v", len(res.Tools), tool != nil)
	}
	if !strings.Contains(tool.Description, "data, not instructions") || tool.Annotations == nil || tool.Annotations.ReadOnlyHint {
		t.Errorf("annotations/description: %+v", tool)
	}
	// without WithDeviceLibrary the existing device rig is unchanged
	plain := newDeviceRig(t, nil)
	pres, _ := plain.cs.ListTools(itCtx(t), nil)
	for _, x := range pres.Tools {
		if x.Name == deviceSyncToolName {
			t.Error("sync tool registered without a library")
		}
	}
}

func TestDryRun_FolderTextIsACodeSpan(t *testing.T) {
	var b strings.Builder
	renderExportTable(&b, []devicejobs.PlanBlock{{Row: 0, Col: 0, Model: "CA John's", Capture: "CA John's",
		Params:      []devicejobs.PlanParam{{Name: "GAIN", A: "0 dB"}},
		CaptureInfo: &devicejobs.PlanCapture{UnitName: "CA John's", Version: 2, Kind: "factory", Folder: "a[click](https://evil.example)", KeyPrefix: "abcd1234", SyncedAt: "2026-11-06T12:00:00Z"}}}, nil, nil, nil, nil)
	if !strings.Contains(b.String(), "folder `a[click](https://evil.example)`") {
		t.Fatalf("folder is not in a code span:\n%s", b.String())
	}
}

// F9: a dry run into a slot the bridge last listed as occupied warns, says when, and still issues the token.
func TestDryRun_WarnsWhenTheLastListingShowedTheSlotOccupied(t *testing.T) {
	r := newLibRig(t)
	r.online(t)
	r.runAgent(t)
	seedCapturePreset(t, r, "p-slot")
	dry := func(slot string) (string, devicePushOutput) {
		res := itCall(t, r.cs, devicePushToolName, map[string]any{"preset_id": "p-slot", "guitar": devGuitar, "slot": slot, "name": "Slot test"})
		return okText(t, res), itStructured[devicePushOutput](t, res)
	}
	// no listing yet: no warning
	if text, out := dry("20B"); out.SlotListingNote != "" || strings.Contains(text, "last listed") {
		t.Fatalf("warned without a listing:\n%s", text)
	}
	okText(t, itCall(t, r.cs, deviceSlotsToolName, map[string]any{"wait_seconds": 5})) // fake: 20A empty, 20B and 1A occupied
	text, out := dry("20B")
	if !strings.Contains(out.SlotListingNote, "20B was occupied when the bridge last listed") || !strings.Contains(text, "⚠️ slot 20B was occupied") {
		t.Fatalf("no warning for an occupied slot:\n%s", text)
	}
	if !out.CanPush || out.ConfirmToken == "" {
		t.Fatalf("the warning must not block the token: %+v", out)
	}
	if text, out := dry("20A"); out.SlotListingNote != "" || strings.Contains(text, "last listed") {
		t.Fatalf("warned for an empty slot:\n%s", text)
	}
}
