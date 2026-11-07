package agents

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/weitzer-org/sound-builder/internal/storage"
)

// refineRequest runs RefineChat for p against a fake Gemini and returns the Architect's
// outgoing prompt text.
func refineRequest(t *testing.T, ctx context.Context, p *storage.Preset) (string, error) {
	t.Helper()
	var mu sync.Mutex
	var last string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		mu.Lock()
		last = string(b)
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(mockGeminiResponse))
	}))
	defer srv.Close()
	orch, err := NewOrchestrator(ctx, "fake-key", nil, WithEndpoint(srv.URL), WithHTTPClient(srv.Client()))
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := orch.RefineChat(ctx, p, "less fizz", true, true, NewPluginGate(false, nil)); err != nil {
		return "", err
	}
	mu.Lock()
	defer mu.Unlock()
	return requestPromptText(t, last), nil
}

func devicePreset(origin *storage.PresetOrigin) *storage.Preset {
	sp := storage.StructuredPreset{Guitars: map[string][]storage.EffectBlock{"From the unit": {
		{ID: "b1", Type: "Drive", Model: "Green 808"},
		{ID: "b2", Type: "Cabinet", Model: "412 Brit 60B V30 95"},
	}}}
	payload, _ := json.Marshal(map[string]any{"structured": sp, "legacy_html": map[string]string{}})
	return &storage.Preset{Payload: string(payload), Origin: origin,
		ChatHistory: []storage.ChatMessage{{Role: "user", Content: "hi"}}}
}

var (
	v6Only   = map[string]string{"12_architect": "v6"}
	v6Device = map[string]string{"12_architect": "v6", DeviceEditPromptKey: "v1"}
)

// The Architect's context for every preset whose origin is not device-readback is byte for
// byte what it was before the section existed, whatever the prompt versions say. The
// reference is the same preset with no origin and no section configured; the one-off check
// against the pre-change code is in the commit message.
func TestDeviceEditContextOnlyForDeviceDrafts(t *testing.T) {
	base := context.Background()
	off := WithPromptVersions(base, v6Only)
	on := WithPromptVersions(base, v6Device)

	want, err := refineRequest(t, off, devicePreset(nil))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(want, "Quad Cortex (device draft)") {
		t.Fatal("the reference context must not carry the section")
	}
	device := &storage.PresetOrigin{Kind: storage.OriginDeviceReadback, JobID: "j", Unconverted: 3}

	for name, c := range map[string]struct {
		ctx    context.Context
		origin *storage.PresetOrigin
	}{
		"no origin, section on":              {on, nil},
		"other origin kind, section on":      {on, &storage.PresetOrigin{Kind: "something-else", Unconverted: 3}},
		"empty origin kind, section on":      {on, &storage.PresetOrigin{}},
		"device draft, section off (absent)": {off, device},
		"device draft, section off":          {WithPromptVersions(base, map[string]string{"12_architect": "v6", DeviceEditPromptKey: "off"}), device},
		"device draft, no versions at all":   {base, device},
	} {
		got, err := refineRequest(t, c.ctx, devicePreset(c.origin))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		wantHere := want
		if name == "device draft, no versions at all" { // production default version, no section
			if wantHere, err = refineRequest(t, base, devicePreset(nil)); err != nil {
				t.Fatal(err)
			}
		}
		if got != wantHere {
			t.Errorf("%s: the Architect's context changed (%d bytes vs %d)", name, len(got), len(wantHere))
		}
	}
}

// A device draft with the section on gets the section and nothing else: removing the section
// text gives back the ordinary context exactly.
func TestDeviceEditContextIsAppendOnly(t *testing.T) {
	base := context.Background()
	want, err := refineRequest(t, WithPromptVersions(base, v6Only), devicePreset(nil))
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range []int{0, 4} {
		got, err := refineRequest(t, WithPromptVersions(base, v6Device),
			devicePreset(&storage.PresetOrigin{Kind: storage.OriginDeviceReadback, JobID: "j", Unconverted: n}))
		if err != nil {
			t.Fatal(err)
		}
		sec, err := deviceEditContext(WithPromptVersions(base, v6Device),
			devicePreset(&storage.PresetOrigin{Kind: storage.OriginDeviceReadback, Unconverted: n}))
		if err != nil || sec == "" {
			t.Fatalf("section: %q, %v", sec, err)
		}
		if !strings.Contains(got, "# Editing a preset that lives on the Quad Cortex (device draft)") {
			t.Fatal("device draft with the section on has no section")
		}
		if strings.Count(got, sec) != 1 || strings.Replace(got, sec, "", 1) != want {
			t.Errorf("unconverted=%d: the context differs from the ordinary one by more than the section", n)
		}
		if has := strings.Contains(got, "left 4 control(s) or block(s)"); has != (n == 4) {
			t.Errorf("unconverted=%d: count sentence present = %v", n, has)
		}
	}
}

func TestDeviceEditContextMissingFileIsAnError(t *testing.T) {
	ctx := WithPromptVersions(context.Background(), map[string]string{"12_architect": "v6", DeviceEditPromptKey: "v99"})
	_, err := refineRequest(t, ctx, devicePreset(&storage.PresetOrigin{Kind: storage.OriginDeviceReadback}))
	if err == nil || !strings.Contains(err.Error(), "v99") {
		t.Fatalf("a configured section version with no file must fail the refine, got %v", err)
	}
	// ...but only a device draft ever looks for it.
	if _, err := refineRequest(t, ctx, devicePreset(nil)); err != nil {
		t.Fatalf("an ordinary preset must not care: %v", err)
	}
}

// config.json is production: the section stays off until the eval passes.
func TestDeviceEditContextOffInConfig(t *testing.T) {
	b, err := os.ReadFile("../../config.json")
	if err != nil {
		t.Fatal(err)
	}
	var c struct {
		AgentPrompts map[string]string `json:"agent_prompts"`
	}
	if err := json.Unmarshal(b, &c); err != nil {
		t.Fatal(err)
	}
	if v, ok := c.AgentPrompts[DeviceEditPromptKey]; ok && v != "off" && v != "" {
		t.Fatalf("config.json turns on %s=%q; it stays off until the E2 eval passes (docs/design/qc-preset-editing.md 8.1)", DeviceEditPromptKey, v)
	}
	if _, err := PromptFS.ReadFile("prompts/" + DeviceEditPromptKey + "_v1.md"); err != nil {
		t.Fatal(err)
	}
}
