package mcpserver

import (
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"golang.org/x/net/html"

	"github.com/weitzer-org/sound-builder/internal/config"
	"github.com/weitzer-org/sound-builder/internal/presetsvc"
)

// mcpOnlyGenerateFields are generate_preset parameters with no control on the
// UI's tone prompt form. Everything else in generateInput must be on the form.
var mcpOnlyGenerateFields = map[string]bool{
	"wait_seconds":         true, // MCP waits for the run; the UI polls
	"mock":                 true, // UI uses ?mock=true in the query string
	"guitars":              true,
	"single_amp_mode":      true,
	"allow_cloud_captures": true,
}

// uiPromptForm is one input of #gen-prompt-form.
type uiPromptForm struct {
	names    map[string]bool // every name= on the form
	checkbox map[string]bool // checkbox name -> checked in the markup
}

// parseUIPromptForm reads web/templates/index.html and returns the controls of
// the form with id gen-prompt-form. It uses a real HTML parser, not a regex.
func parseUIPromptForm(t *testing.T) uiPromptForm {
	t.Helper()
	f, err := os.Open(filepath.Join(findRepoRoot(t), "web", "templates", "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	doc, err := html.Parse(f)
	if err != nil {
		t.Fatal(err)
	}

	attr := func(n *html.Node, key string) (string, bool) {
		for _, a := range n.Attr {
			if a.Key == key {
				return a.Val, true
			}
		}
		return "", false
	}

	var form *html.Node
	var find func(*html.Node)
	find = func(n *html.Node) {
		if form != nil {
			return
		}
		if n.Type == html.ElementNode && n.Data == "form" {
			if id, _ := attr(n, "id"); id == "gen-prompt-form" {
				form = n
				return
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			find(c)
		}
	}
	find(doc)
	if form == nil {
		t.Fatal("form#gen-prompt-form not found in web/templates/index.html; update this test if the form was renamed")
	}

	out := uiPromptForm{names: map[string]bool{}, checkbox: map[string]bool{}}
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && (n.Data == "input" || n.Data == "select" || n.Data == "textarea") {
			if name, ok := attr(n, "name"); ok && name != "" {
				out.names[name] = true
				if typ, _ := attr(n, "type"); typ == "checkbox" {
					_, checked := attr(n, "checked")
					out.checkbox[name] = checked
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(form)
	return out
}

func generateInputTags() map[string]bool {
	tags := map[string]bool{}
	rt := reflect.TypeOf(generateInput{})
	for i := 0; i < rt.NumField(); i++ {
		if name, _, _ := strings.Cut(rt.Field(i).Tag.Get("json"), ","); name != "" && name != "-" {
			tags[name] = true
		}
	}
	return tags
}

// TestGeneratePresetCoversUIPromptForm fails when the UI tone prompt form gains
// (or renames) a field that generate_preset does not accept, so the two stay in
// step. Fix by adding the field to generateInput and presetsvc.GenerateOptions.
// There is deliberately no UI-only allowlist: MCP must expose every UI option.
func TestGeneratePresetCoversUIPromptForm(t *testing.T) {
	form := parseUIPromptForm(t)
	tags := generateInputTags()

	var missing []string
	for name := range form.names {
		if !tags[name] {
			missing = append(missing, name)
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		t.Errorf("the UI form #gen-prompt-form has fields that generate_preset (generateInput) lacks: %v. "+
			"Add them to generateInput and presetsvc.GenerateOptions so MCP exposes every UI option.", missing)
	}

	// The allowlist must not rot: every entry is a real generateInput field
	// that is genuinely absent from the form.
	for name := range mcpOnlyGenerateFields {
		if !tags[name] {
			t.Errorf("allowlisted MCP-only field %q is not a generateInput json tag", name)
		}
		if form.names[name] {
			t.Errorf("%q is now on the UI form; remove it from mcpOnlyGenerateFields", name)
		}
	}
	// And every generateInput field is either on the form or allowlisted.
	for name := range tags {
		if !form.names[name] && !mcpOnlyGenerateFields[name] {
			t.Errorf("generateInput field %q is neither on the UI form nor in mcpOnlyGenerateFields; "+
				"declare it MCP-only on purpose or add the UI control", name)
		}
	}
	if len(form.checkbox) < 5 {
		t.Errorf("found %d checkboxes in the form, expected at least 5; did the markup change?", len(form.checkbox))
	}
}

// TestGeneratePresetDefaultsMatchUI checks that omitting a checkbox parameter
// over MCP gives the value the UI form submits by default (its checked state),
// under the config the app ships (config.json). The UI always submits every
// checkbox (a hidden "off" input), so its default is its markup state; MCP
// omits the parameter and gets presetsvc's default.
func TestGeneratePresetDefaultsMatchUI(t *testing.T) {
	form := parseUIPromptForm(t)
	cfg, err := config.LoadConfig(filepath.Join(findRepoRoot(t), "config.json"))
	if err != nil {
		t.Fatalf("load config.json: %v", err)
	}

	got := presetsvc.BuildConstraints(cfg, presetsvc.GenerateOptions{})
	for name, checked := range form.checkbox {
		v, ok := got[name].(bool)
		if !ok {
			t.Errorf("constraint %q is missing or not a bool when the MCP parameter is omitted (value %v)", name, got[name])
			continue
		}
		if v != checked {
			t.Errorf("checkbox %q is checked=%v in the UI but omitting the MCP parameter gives %v under config.json", name, checked, v)
		}
	}
}

// TestGenerateInputSchemaDescribesUIFields guards the model-facing wording: the
// UI-backed options keep the UI's labels, and MCP-only ones say so.
func TestGenerateInputSchemaDescribesUIFields(t *testing.T) {
	rt := reflect.TypeOf(generateInput{})
	for _, c := range []struct{ field, want string }{
		{"AllowFactoryCaptures", "Allow Factory Captures"},
		{"AllowUserCaptures", "Use My Capture Library"},
		{"AllowPaidPlugins", "Allow Paid Plugins"},
		{"FavorCaptures", "Favor Captures Over Models"},
		{"FavorCloudCaptures", "Favor My Capture Library"},
		{"Guitars", "MCP-only"},
		{"SingleAmpMode", "MCP-only"},
		{"AllowCloudCaptures", "MCP-only"},
	} {
		f, _ := rt.FieldByName(c.field)
		if d := f.Tag.Get("jsonschema"); !strings.Contains(d, c.want) {
			t.Errorf("%s description %q should contain %q", c.field, d, c.want)
		}
	}
	f, _ := rt.FieldByName("Guitars")
	d := f.Tag.Get("jsonschema")
	for _, want := range []string{"ONLY guitars the user actually named", "OMIT", "never add or invent"} {
		if !strings.Contains(d, want) {
			t.Errorf("guitars description %q must contain %q", d, want)
		}
	}
}
