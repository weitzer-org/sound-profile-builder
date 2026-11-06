package mcpserver

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/weitzer-org/sound-builder/internal/deviceexport"
	"github.com/weitzer-org/sound-builder/internal/storage"
)

func newExportSession(t *testing.T, h *svcHarness, withExporter bool) *mcp.ClientSession {
	t.Helper()
	var opts []Option
	if withExporter {
		exp, err := deviceexport.NewEmbedded(deviceexport.Options{})
		if err != nil {
			t.Fatal(err)
		}
		opts = append(opts, WithExporter(exp))
	}
	mux := http.NewServeMux()
	Mount(mux, h.svc, Config{Token: testToken}, nil, opts...)
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)
	hc := &http.Client{Transport: itBearer{token: testToken, base: http.DefaultTransport}}
	cs, err := itConnect(t, ts.URL+"/mcp", hc)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cs.Close() })
	return cs
}

const hostileText = "x\n# injected <script>"

func seedDevicePreset(t *testing.T, h *svcHarness, id string, guitars map[string][]storage.EffectBlock) {
	t.Helper()
	raw, err := json.Marshal(storage.StructuredPreset{Guitars: guitars})
	if err != nil {
		t.Fatal(err)
	}
	if err := h.store.Save(context.Background(), &storage.Preset{ID: id, Name: "Device test", Payload: string(raw)}); err != nil {
		t.Fatal(err)
	}
}

func pushableChain() []storage.EffectBlock {
	return []storage.EffectBlock{
		{Model: "Myth Drive", Parameters: []storage.BlockParameter{
			{Name: "Gain", Value: "2", ValueB: "8", Basis: "estimate"},
			{Name: "Bypass", Value: "Off", ValueB: "On", Basis: "estimate"}}},
		{Model: "Brit 2203", Parameters: []storage.BlockParameter{{Name: "Output", Value: "-10 dB", Basis: "estimate"}}},
	}
}

func TestGetPresetDevice(t *testing.T) {
	h := newTestService(t)
	cs := newExportSession(t, h, true)
	seedDevicePreset(t, h, "p-dev1", map[string][]storage.EffectBlock{"Tele": pushableChain()})

	res := itCall(t, cs, getToolName, map[string]any{"preset_id": "p-dev1", "format": "device"})
	text := okText(t, res)
	for _, want := range []string{
		"Device export of guitar Tele: qc2-device-export/1",
		"Nothing is unresolved",
		"| row 0 col 0 | Myth Drive (id 1) | GAIN | 2 | 8 | Active/Bypassed |",
		"| row 0 col 1 | Brit 2203 (id 1001) | OUTPUT | \\-10 dB | \\-10 dB | Active/Active |",
		"Scenes C-H get Scene A's values",
		"```json\n{\"format\":\"qc2-device-export/1\"",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("missing %q in:\n%s", want, text)
		}
	}
	var out getOutput
	raw, _ := json.Marshal(res.StructuredContent)
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatal(err)
	}
	if out.Device == nil || !out.Device.CanPush || out.Device.Guitar != "Tele" || len(out.Device.Blocks) != 2 ||
		len(out.Device.PlanSHA256) != 64 || out.Format != formatDevice {
		t.Fatalf("structured output: %+v", out.Device)
	}
	if out.Device.Payload["format"] != deviceexport.Format {
		t.Fatalf("payload: %+v", out.Device.Payload)
	}
}

func TestGetPresetDeviceGuitarChoice(t *testing.T) {
	h := newTestService(t)
	cs := newExportSession(t, h, true)
	seedDevicePreset(t, h, "p-dev2", map[string][]storage.EffectBlock{"Tele": pushableChain(), "ES-339": pushableChain()})

	if msg := errText(t, itCall(t, cs, getToolName, map[string]any{"preset_id": "p-dev2", "format": "device"})); !strings.Contains(msg, "'ES-339', 'Tele'") {
		t.Errorf("two guitars, none chosen: %s", msg)
	}
	if msg := errText(t, itCall(t, cs, getToolName, map[string]any{"preset_id": "p-dev2", "format": "device", "guitar": "Strat"})); !strings.Contains(msg, "exactly one of") {
		t.Errorf("unknown guitar: %s", msg)
	}
	okText(t, itCall(t, cs, getToolName, map[string]any{"preset_id": "p-dev2", "format": "device", "guitar": "ES-339"}))
	if msg := errText(t, itCall(t, cs, getToolName, map[string]any{"preset_id": "p-dev2", "guitar": "ES-339"})); !strings.Contains(msg, "only used with format=device") {
		t.Errorf("guitar with markdown: %s", msg)
	}
}

func TestGetPresetDeviceUnavailable(t *testing.T) {
	h := newTestService(t)
	cs := newExportSession(t, h, false)
	seedDevicePreset(t, h, "p-dev3", map[string][]storage.EffectBlock{"Tele": pushableChain()})
	if msg := errText(t, itCall(t, cs, getToolName, map[string]any{"preset_id": "p-dev3", "format": "device"})); !strings.Contains(msg, "not available") {
		t.Errorf("no exporter: %s", msg)
	}
}

// TestGetPresetDeviceHostile puts hostile text in the guitar, a block name and a value,
// all of which reach the plan table, the unresolved list and the JSON.
func TestGetPresetDeviceHostile(t *testing.T) {
	h := newTestService(t)
	cs := newExportSession(t, h, true)
	chain := []storage.EffectBlock{
		{Model: hostileText + " | ``` |", Parameters: []storage.BlockParameter{}},
		{Model: "Myth Drive", Parameters: []storage.BlockParameter{{Name: "Gain " + hostileText, Value: hostileText, Basis: "estimate"}}},
	}
	seedDevicePreset(t, h, "p-dev4", map[string][]storage.EffectBlock{hostileText: chain})
	text := okText(t, itCall(t, cs, getToolName, map[string]any{"preset_id": "p-dev4", "format": "device", "guitar": hostileText}))
	t.Logf("hostile device export:\n%s", text)
	for _, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(line, "# injected") {
			t.Errorf("hostile text opened a heading line: %q", line)
		}
		if strings.HasPrefix(line, "```") && line != "```json" && line != "```" {
			t.Errorf("hostile text opened a fence: %q", line)
		}
	}
	if strings.Contains(text, "<script") {
		t.Errorf("raw '<' from preset text")
	}
	if strings.Count(text, "\n```") != 2 {
		t.Errorf("want exactly one fenced block")
	}
}
