package evalcheck

import (
	"strings"
	"testing"

	"github.com/weitzer-org/sound-builder/internal/agents"
)

func TestValidateAgainstSchema_Agent1(t *testing.T) {
	s := agents.AgentResponseSchema("1_tone_historian")
	good := "```json\n" + `{"amp_type":"Vox AC30","pedal_names":["Tube Screamer"],"speaker_type":"Celestion Blue","mic_type":"SM57"}` + "\n```"
	if got := ValidateAgainstSchema(good, s); len(got) != 0 {
		t.Fatalf("valid output flagged: %v", got)
	}
	cases := map[string]string{
		"not json":      `{"amp_type": "x"`,
		"missing req":   `{"amp_type":"x","pedal_names":[],"speaker_type":"y"}`,
		"null req":      `{"amp_type":"x","pedal_names":[],"speaker_type":"y","mic_type":null}`,
		"wrong type":    `{"amp_type":5,"pedal_names":[],"speaker_type":"y","mic_type":"z"}`,
		"bad item type": `{"amp_type":"x","pedal_names":[1],"speaker_type":"y","mic_type":"z"}`,
		"array top":     `[]`,
		"empty":         ``,
	}
	for name, in := range cases {
		if got := ValidateAgainstSchema(in, s); len(got) == 0 {
			t.Errorf("%s: expected violations, got none", name)
		}
	}
}

func TestValidateAgainstSchema_EnumAndNumber(t *testing.T) {
	s := agents.AgentResponseSchema("2_sonic_profiler")
	ok := `{"eq_profile":"flat","saturation_style":"hard clipping","reverb_type":"plate","noise_gate_target_db":-60}`
	if got := ValidateAgainstSchema(ok, s); len(got) != 0 {
		t.Fatalf("valid flagged: %v", got)
	}
	bad := `{"eq_profile":"boomy","saturation_style":"x","reverb_type":"y","noise_gate_target_db":"-60 dB"}`
	got := strings.Join(ValidateAgainstSchema(bad, s), "|")
	if !strings.Contains(got, "not in enum") || !strings.Contains(got, "expected integer") {
		t.Fatalf("expected enum and integer violations, got %q", got)
	}
}

func TestValidateAgainstSchema_NilSchema(t *testing.T) {
	if got := ValidateAgainstSchema(`{}`, nil); len(got) != 1 {
		t.Fatalf("got %v", got)
	}
}
