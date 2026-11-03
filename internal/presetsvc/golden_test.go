package presetsvc

import (
	"context"
	"testing"
	"time"
)

// The draft preset's first model chat message embeds HTML (agent impacts plus a
// token-stats card) that the HTMX UI renders verbatim and that is persisted.
// It moved here from handleGeneratePreset and must stay byte-identical, raw
// string whitespace included: a nesting-level change once shifted its tabs.
func TestGenerateDraftChatHistoryIntroIsByteStable(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()

	runID := generate(t, h, GenerateOptions{Mock: true})
	rec, err := h.svc.WaitRun(ctx, runID, 10*time.Second)
	if err != nil || rec.PresetID == "" {
		t.Fatalf("run did not complete: err=%v rec=%+v", err, rec)
	}
	p, err := h.store.Get(ctx, rec.PresetID)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.ChatHistory) != 2 {
		t.Fatalf("chat history has %d messages, want 2", len(p.ChatHistory))
	}

	const wantTail = "<br><br>\n\t\t\t\t<div class=\"card\" style=\"margin-top: 1rem; font-family: monospace; color: #a1a1aa;\">\n" +
		"\t\t\t\t\t<strong>Pipeline AI Processing Tokens:</strong> Input: 0 | Output: 0 | Models: \n" +
		"\t\t\t\t</div>"
	got := p.ChatHistory[1].Content
	if len(got) < len(wantTail) || got[len(got)-len(wantTail):] != wantTail {
		t.Errorf("token-stats HTML changed.\n got tail: %q\nwant tail: %q", got[max(0, len(got)-len(wantTail)-20):], wantTail)
	}
}
