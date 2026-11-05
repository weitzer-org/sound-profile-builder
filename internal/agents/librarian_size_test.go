package agents

import (
	"os"
	"testing"

	"github.com/weitzer-org/sound-builder/internal/config"
)

// TestLibrarianContextSize logs how big the Dictionary and Amplifier Archetype Menu are
// for the shipped config.json, the two catalog-derived inputs that grow with the device
// catalog. It asserts nothing about the size; run it with -v to compare before and after
// a change to what the Librarian is offered (it only fails if an input comes back empty). Tokens are estimated as bytes/4, a rough
// rule of thumb, not Gemini's tokenizer.
func TestLibrarianContextSize(t *testing.T) {
	cfg, err := config.LoadConfig("../../config.json")
	if err != nil {
		t.Skipf("config.json not loadable: %v", err)
	}
	dict, menu := librarianInputs(cfg.AllowFactoryCaptures, NewPluginGate(cfg.AllowPaidPlugins, cfg.AvailablePlugins))
	if len(dict) == 0 || len(menu) == 0 {
		t.Fatalf("empty Librarian input: Dictionary %d bytes, amp menu %d bytes", len(dict), len(menu))
	}
	prompt, err := os.ReadFile("prompts/4_coros_librarian_v2.md")
	if err != nil {
		t.Fatalf("read Librarian prompt: %v", err)
	}
	t.Logf("paid plugins allowed: %v, owned: %v; factory captures allowed: %v", cfg.AllowPaidPlugins, cfg.AvailablePlugins, cfg.AllowFactoryCaptures)
	for _, part := range []struct {
		name string
		n    int
	}{
		{"Dictionary", len(dict)},
		{"Amplifier Archetype Menu", len(menu)},
		{"Librarian v2 prompt file", len(prompt)},
	} {
		t.Logf("%-26s %7d bytes  ~%6d tokens", part.name, part.n, part.n/4)
	}
}
