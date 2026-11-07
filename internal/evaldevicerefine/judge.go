package evaldevicerefine

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/weitzer-org/sound-builder/internal/agents"
	"github.com/weitzer-org/sound-builder/internal/presetsvc"
	"github.com/weitzer-org/sound-builder/internal/storage"
)

// The judge is the one non-deterministic metric. It sees the draft, the request and two
// candidate answers labelled X and Y, never the arm names, with the order fixed per triple
// by a hash (so a rerun asks the same question) and recorded so the mapping can be audited.

// XIsA reports whether arm A is shown as candidate X for this triple.
func XIsA(t Triple) bool {
	h := sha256.Sum256([]byte(fmt.Sprintf("judge-order/%s/%s/%d", t.Draft, t.Request, t.Rep)))
	return h[0]&1 == 0
}

// JudgeVerdict is the judge's structured answer.
type JudgeVerdict struct {
	X         JudgeScore `json:"x"`
	Y         JudgeScore `json:"y"`
	Rationale string     `json:"rationale"`
}

// JudgeRecord is one line of judge.jsonl.
type JudgeRecord struct {
	Triple
	XIsA    bool          `json:"x_is_a"`
	Verdict *JudgeVerdict `json:"verdict,omitempty"`
	Error   string        `json:"error,omitempty"`
	Raw     string        `json:"raw,omitempty"`
}

// Scores returns the verdict mapped back to arms.
func (r JudgeRecord) Scores() (a, b *JudgeScore) {
	if r.Verdict == nil {
		return nil, nil
	}
	x, y := r.Verdict.X, r.Verdict.Y
	if r.XIsA {
		return &x, &y
	}
	return &y, &x
}

// ParseVerdict reads the judge's JSON, with the same fence stripping the refine path uses.
func ParseVerdict(text string) (*JudgeVerdict, error) {
	var v JudgeVerdict
	if err := json.Unmarshal([]byte(strings.TrimSpace(agents.StripJSONFences(text))), &v); err != nil {
		return nil, err
	}
	for _, s := range []int{v.X.Addressed, v.Y.Addressed} {
		if s < 0 || s > 2 {
			return nil, fmt.Errorf("addressed must be 0, 1 or 2, got %d", s)
		}
	}
	return &v, nil
}

// RenderChain lists a chain one block per line, values as the Architect wrote them.
func RenderChain(chain []storage.EffectBlock) string {
	var b strings.Builder
	for _, blk := range chain {
		fmt.Fprintf(&b, "- %s | %s", blk.ID, blk.Model)
		for _, p := range blk.Parameters {
			fmt.Fprintf(&b, " | %s=%s", p.Name, p.Value)
			if p.ValueB != "" && p.ValueB != p.Value {
				fmt.Fprintf(&b, " (scene B %s)", p.ValueB)
			}
		}
		b.WriteString("\n")
	}
	return b.String()
}

// RenderCandidate shows one refine output to the judge: the reply and the chain it leaves.
func RenderCandidate(d *Draft, raw string) string {
	var env refineEnvelope
	clean := presetsvc.SanitizeJSONQuotes(strings.TrimSpace(agents.StripJSONFences(raw)))
	if err := json.Unmarshal([]byte(clean), &env); err != nil {
		return "(unreadable answer)"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Reply: %s\n", env.ConversationalResponse)
	if !env.DSPMatrixUpdated || len(env.StructuredPayload.Guitars) == 0 {
		b.WriteString("Chain after the answer: unchanged from the original.\n")
		return b.String()
	}
	if chain, err := chainOf(env.StructuredPayload); err == nil {
		b.WriteString("Chain after the answer:\n" + RenderChain(chain))
	}
	return b.String()
}

// JudgePrompt asks for a blind comparison of two answers to one request.
func JudgePrompt(d *Draft, req Request, x, y string) string {
	return fmt.Sprintf(`You are an expert guitarist and Quad Cortex programmer judging two assistants' answers to the same request about an existing preset.

The preset (as it was, one block per line, values for scene A and, where different, scene B):
%s
The user's request:
%s

Candidate X:
%s
Candidate Y:
%s
For EACH candidate score:
- addressed: 0 = the request is not addressed (or the answer makes unrelated changes), 1 = partly, 2 = clearly addressed.
- tone_ok: true if the resulting settings are musically reasonable for the request (no absurd values, nothing that would obviously make the tone worse).
A candidate that changes nothing can still score if it explains well why nothing should change, but a request that can be met should be addressed by an actual change. Do not prefer a candidate for being longer. Respond ONLY with JSON:
{"x": {"addressed": 0-2, "tone_ok": true|false}, "y": {"addressed": 0-2, "tone_ok": true|false}, "rationale": "..."}
`, RenderChain(d.Chain), req.Message, x, y)
}
