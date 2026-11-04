package mcpserver

import (
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/weitzer-org/sound-builder/internal/storage"
)

var update = flag.Bool("update", false, "rewrite testdata/preset.golden.md from current output")

// findRepoRoot walks up from the test's working directory to the directory holding go.mod.
func findRepoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found above test working directory")
		}
		dir = parent
	}
}

func TestRenderPresetMarkdownGolden(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join(findRepoRoot(t), "testdata", "e2e_mocks", "architect_generate.json"))
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		BuilderStatement  string                   `json:"builder_statement"`
		StructuredPayload storage.StructuredPreset `json:"structured_payload"`
	}
	if err := json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}

	p := &storage.Preset{Name: "Golden Preset", BuilderStatement: fixture.BuilderStatement}
	got := RenderPresetMarkdown(p, fixture.StructuredPayload)

	const golden = "testdata/preset.golden.md"
	if *update {
		if err := os.WriteFile(golden, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		t.Logf("updated %s", golden)
		return
	}
	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatalf("read golden (run with -update to create): %v", err)
	}
	if got != string(want) {
		t.Errorf("output differs from %s (run with -update after reviewing)\n--- got ---\n%s", golden, got)
	}
}

var (
	allowedHeading  = regexp.MustCompile(`^#{2,3} `)
	unescapedPipe   = regexp.MustCompile(`(^|[^\\])\|`)
	tableRowPrefix  = "|"
	expectedColumns = 6 // leading pipe + 4 inner separators + trailing pipe for 5 columns
)

// assertStructure checks that LLM text did not create extra Markdown structure:
// only our own ## / ### headings start lines with #, and every table row is one
// line with the expected number of unescaped column separators.
func assertStructure(t *testing.T, out string) {
	t.Helper()
	if strings.Contains(out, "<") || strings.Contains(out, ">") {
		t.Errorf("raw < or > in output:\n%s", out)
	}
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(line, "#") && !allowedHeading.MatchString(line) {
			t.Errorf("unexpected heading-like line %q", line)
		}
		if strings.HasPrefix(line, tableRowPrefix) {
			if n := len(unescapedPipe.FindAllString(line, -1)); n != expectedColumns {
				t.Errorf("table row has %d unescaped pipes (want %d): %q", n, expectedColumns, line)
			}
		}
	}
}

func TestRenderNeutralisesHostileFields(t *testing.T) {
	tests := []struct {
		name        string
		preset      *storage.Preset
		sp          storage.StructuredPreset
		mustContain []string
	}{
		{
			name: "pipes, newlines, heading in rationale, script in name",
			preset: &storage.Preset{
				Name:             "<script>alert(1)</script>",
				BuilderStatement: "Stmt\n# Injected heading",
			},
			sp: storage.StructuredPreset{Guitars: map[string][]storage.EffectBlock{
				"Guitar\n## evil": {{
					Type:      "Overdrive\n| x",
					Model:     "Tu|be\nAmp",
					Rationale: "line one\n# Heading\nmore",
					Parameters: []storage.BlockParameter{{
						Name: "Gain|x", Value: "a|b\n# p", Unit: "dB",
					}},
				}},
			}},
			mustContain: []string{
				"## &lt;script&gt;alert(1)&lt;/script&gt;",
				"Stmt # Injected heading",
				"### Guitar ## evil",
				"Tu\\|be Amp",
				"Gain\\|x a\\|b # pdB",
				"- Tu\\|be Amp: line one # Heading more",
				"Overdrive \\| x",
			},
		},
		{
			name:   "statement and name that start block constructs",
			preset: &storage.Preset{Name: "1. first", BuilderStatement: "# Injected\n- list\n> quote"},
			sp:     storage.StructuredPreset{},
			mustContain: []string{
				"## 1\\. first",
				"\\# Injected - list &gt; quote",
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			out := RenderPresetMarkdown(tc.preset, tc.sp)
			assertStructure(t, out)
			for _, want := range tc.mustContain {
				if !strings.Contains(out, want) {
					t.Errorf("output missing %q\n%s", want, out)
				}
			}
			if strings.Contains(out, "\n# ") || strings.Contains(out, "\n- list") {
				t.Errorf("LLM text started a new block:\n%s", out)
			}
		})
	}
}

func TestRenderNilPresetAndEmptyGuitars(t *testing.T) {
	if got, want := RenderPresetMarkdown(nil, storage.StructuredPreset{}), "##\n"; got != want {
		t.Errorf("nil preset: got %q want %q", got, want)
	}
	p := &storage.Preset{Name: "Only Name", BuilderStatement: "Just a statement."}
	want := "## Only Name\n\nJust a statement.\n"
	if got := RenderPresetMarkdown(p, storage.StructuredPreset{Guitars: map[string][]storage.EffectBlock{}}); got != want {
		t.Errorf("empty guitars: got %q want %q", got, want)
	}
}

func TestRenderEstimateAndValueB(t *testing.T) {
	block := func(params ...storage.BlockParameter) storage.StructuredPreset {
		return storage.StructuredPreset{Guitars: map[string][]storage.EffectBlock{
			"G": {{Type: "Amplifier", Model: "Tweed", Parameters: params}},
		}}
	}
	p := &storage.Preset{Name: "P"}

	t.Run("no estimate means no asterisk and no footnote", func(t *testing.T) {
		out := RenderPresetMarkdown(p, block(
			storage.BlockParameter{Name: "Gain", Value: "5", ValueB: "7", Unit: "dB", Basis: "confirmed_range"},
		))
		if !strings.Contains(out, "| Gain 5dB | Gain 7dB |") {
			t.Errorf("missing separate A and B columns: %s", out)
		}
		if strings.Contains(out, "*") || strings.Contains(out, "estimated value") {
			t.Errorf("unexpected estimate marker:\n%s", out)
		}
	})

	t.Run("estimate adds asterisk and footnote", func(t *testing.T) {
		out := RenderPresetMarkdown(p, block(
			storage.BlockParameter{Name: "Gain", Value: "5", ValueB: "7", Unit: "dB", Basis: "estimate"},
			storage.BlockParameter{Name: "Drive", Value: "3", ValueB: "3", Basis: "estimate"},
			storage.BlockParameter{Name: "Bass", Value: "4", ValueB: ""},
		))
		for _, want := range []string{"| Gain 5dB*; Drive 3*; Bass 4 | Gain 7dB*; Drive 3*; Bass 4 |", "\\* = estimated value"} {
			if !strings.Contains(out, want) {
				t.Errorf("missing %q:\n%s", want, out)
			}
		}
		if strings.Contains(out, "Bass 4*") {
			t.Errorf("non-estimate parameter got asterisk:\n%s", out)
		}
	})

	t.Run("identical scenes show same as A, never an arrow", func(t *testing.T) {
		out := RenderPresetMarkdown(p, block(storage.BlockParameter{Name: "Drive", Value: "3", ValueB: "3"}))
		if strings.Contains(out, "→") || !strings.Contains(out, "| Drive 3 | same as A |") {
			t.Errorf("want same-as-A column:\n%s", out)
		}
	})
}

func TestRenderDeterministic(t *testing.T) {
	guitars := map[string][]storage.EffectBlock{}
	for _, g := range []string{"Strat", "Les Paul", "Tele", "SG", "Jazzmaster", "335"} {
		guitars[g] = []storage.EffectBlock{
			{Type: "Amplifier", Model: "Plexi", Rationale: "why " + g, Parameters: []storage.BlockParameter{
				{Name: "Gain", Value: "6", Basis: "estimate"}, {Name: "Bass", Value: "4"},
			}},
			{Type: "Overdrive", Model: "Tube Screamer", Parameters: []storage.BlockParameter{{Name: "Drive", Value: "2"}}},
		}
	}
	p := &storage.Preset{Name: "Det", BuilderStatement: "Stable."}
	sp := storage.StructuredPreset{Guitars: guitars}

	first := RenderPresetMarkdown(p, sp)
	for i := 0; i < 20; i++ {
		if got := RenderPresetMarkdown(p, sp); got != first {
			t.Fatalf("run %d differs from first run", i+1)
		}
	}
}
