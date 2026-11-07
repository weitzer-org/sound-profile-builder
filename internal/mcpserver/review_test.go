package mcpserver

import (
	"context"
	"strings"
	"testing"

	"github.com/weitzer-org/sound-builder/internal/storage"
)

func importReviewDraft(t *testing.T, h *svcHarness, id string, origin storage.PresetOrigin) {
	t.Helper()
	sp := storage.StructuredPreset{Guitars: map[string][]storage.EffectBlock{"From the unit": {
		{ID: "b1", Type: "Drive", Model: "Green 808"},
	}}}
	if _, _, err := h.svc.ImportDraft(context.Background(), id, sp, origin, "imported"); err != nil {
		t.Fatal(err)
	}
}

// refine_preset(review_goal) is the one-call "review my unit preset": it sends the canned
// minimal-edits request for a draft read from the unit, and points at the edit dry run.
func TestRefinePresetReviewGoal(t *testing.T) {
	h := newMockService(t)
	cs := newMCPSession(t, h.svc)
	importReviewDraft(t, h, "device-j1", storage.PresetOrigin{Kind: storage.OriginDeviceReadback, JobID: "j1"})
	importReviewDraft(t, h, "plain-1", storage.PresetOrigin{Kind: "other"})

	res := itCall(t, cs, refineToolName, map[string]any{"preset_id": "device-j1", "review_goal": "a cleaner blues rhythm", "mock": true, "wait_seconds": 30})
	if res.IsError {
		t.Fatalf("review: %s", itText(t, res))
	}
	if out := itStructured[runOutput](t, res); out.Status != "complete" {
		t.Fatalf("status = %q", out.Status)
	}
	if text := itText(t, res); !strings.Contains(text, "edit_device_preset") || !strings.Contains(text, "nothing on the unit has changed") {
		t.Errorf("a refined device draft should point at the edit dry run:\n%.600s", text)
	}
	p, err := h.store.Get(context.Background(), "device-j1")
	if err != nil {
		t.Fatal(err)
	}
	if len(p.ChatHistory) == 0 || p.ChatHistory[0].Content != reviewMessage("a cleaner blues rhythm") {
		t.Errorf("chat history = %+v, want the canned review request first", p.ChatHistory)
	}
	for _, want := range []string{"a cleaner blues rhythm", "minimal edits", "id, model and order"} {
		if !strings.Contains(p.ChatHistory[0].Content, want) {
			t.Errorf("canned request lacks %q: %s", want, p.ChatHistory[0].Content)
		}
	}

	for name, args := range map[string]map[string]any{
		"both set":        {"preset_id": "device-j1", "message": "x", "review_goal": "y"},
		"not a device":    {"preset_id": "plain-1", "review_goal": "y"},
		"neither":         {"preset_id": "device-j1"},
		"too long a goal": {"preset_id": "device-j1", "review_goal": strings.Repeat("a", maxReviewGoalRunes+1)},
		"unknown preset":  {"preset_id": "missing-preset", "review_goal": "y"},
	} {
		args["mock"] = true
		if r := itCall(t, cs, refineToolName, args); !r.IsError {
			t.Errorf("%s: want a tool error, got %s", name, itText(t, r))
		}
	}
	// An ordinary refine of an ordinary preset has no device hint.
	r := itCall(t, cs, refineToolName, map[string]any{"preset_id": "plain-1", "message": "less gain", "mock": true, "wait_seconds": 30})
	if r.IsError || strings.Contains(itText(t, r), "edit_device_preset") {
		t.Errorf("ordinary refine: error=%v text has device hint=%v", r.IsError, strings.Contains(itText(t, r), "edit_device_preset"))
	}
}
