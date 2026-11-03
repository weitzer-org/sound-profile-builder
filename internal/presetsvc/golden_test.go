package presetsvc

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/weitzer-org/sound-builder/internal/agents"
	"github.com/weitzer-org/sound-builder/internal/runs"
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

	// The whole intro, agent-impact list included: it is what the original handler
	// wrote (verified by diffing the original and new servers' output), and the
	// impact list is the part with real logic (agent names, prompt versions).
	const wantIntro = "Preset structure successfully laid out based on your requirements.\n" +
		"<i><ul><li><strong>Tone Historian (v1):</strong> Agent 1 (Tone Historian): Did stuff based on Make it brighter.</li></ul></i><br><br>" +
		"\n\t\t\t\t<div class=\"card\" style=\"margin-top: 1rem; font-family: monospace; color: #a1a1aa;\">\n" +
		"\t\t\t\t\t<strong>Pipeline AI Processing Tokens:</strong> Input: 0 | Output: 0 | Models: \n" +
		"\t\t\t\t</div>"
	if got := p.ChatHistory[1].Content; got != wantIntro {
		t.Errorf("draft intro changed.\n got: %q\nwant: %q", got, wantIntro)
	}
}

// nilUsageOrch reports no TokenUsage, as an orchestrator without usage data would.
type nilUsageOrch struct{ *fakeOrch }

func (o nilUsageOrch) RunPipeline(ctx context.Context, p string, c map[string]interface{}, a map[string]string, f func(string)) (string, *agents.TokenUsage, error) {
	out, _, err := o.fakeOrch.RunPipeline(ctx, p, c, a, f)
	return out, nil, err
}

// A nil TokenUsage used to nil-dereference (tokenUsage.ModelsUsed was read before
// the nil check); the run must complete with just no stats card.
func TestGenerateWithNilTokenUsageCompletes(t *testing.T) {
	h := newHarness(t)
	h.svc.d.OrchMaker = func(context.Context, string) (agents.OrchestratorService, error) {
		return nilUsageOrch{h.orch}, nil
	}
	ctx := context.Background()

	runID := generate(t, h, GenerateOptions{Mock: true})
	rec, err := h.svc.WaitRun(ctx, runID, 10*time.Second)
	if err != nil || rec.Status != runs.StatusComplete {
		t.Fatalf("run = %+v, err = %v; want complete", rec, err)
	}
	p, err := h.store.Get(ctx, rec.PresetID)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(p.ChatHistory[1].Content, "Pipeline AI Processing Tokens") {
		t.Error("stats card rendered although no token usage was reported")
	}
}
