package api

import (
	"regexp"
	"testing"

	"github.com/weitzer-org/sound-builder/internal/storage"
)

func TestGuitarTabID(t *testing.T) {
	for in, want := range map[string]string{
		"Gibson ES-339 Humbuckers":      "gibson-es-339-humbuckers", // unchanged from the old slug
		"Fender Telecaster Single Coil": "fender-telecaster-single-coil",
		"A/B":                           "ab", // "/" was already dropped
		"x'); alert(1);//":              "x-alert1",
		`a"><img src=x onerror=1>`:      "aimg-srcx-onerror1",
		"a&#39;);alert(1);('":           "a39alert1",
		"!!!":                           "guitar",
		"":                              "guitar",
	} {
		if got := guitarTabID(in); got != want {
			t.Errorf("guitarTabID(%q) = %q, want %q", in, got, want)
		}
	}
}

var (
	onclickTabRe = regexp.MustCompile(`onclick="switchTab\(this, '([^']*)'\)"`)
	paneIDRe     = regexp.MustCompile(`id="(tab-[^"]*)"`)
)

// A hostile guitar key must not break out of the onclick/id attributes in the
// editable workspace (structured mode) or the static one. The key still appears,
// HTML-escaped, as the button's visible label, which is inert text.
func TestWorkspaceHTMLTabIDCannotBreakOut(t *testing.T) {
	const evil = `a');alert(1);('`
	payload := `{"guitars":{"` + evil + `":[{"id":"b1","type":"Amplifier","model":"Plexi","parameters":[{"name":"Gain","type":"slider","value":"4"}]}]}}`
	p := &storage.Preset{ID: "p1", Name: "Saved", Payload: payload}
	for _, static := range []bool{false, true} {
		out := renderTweakingWorkspaceHTML(p, false, static)
		clicks := onclickTabRe.FindAllStringSubmatch(out, -1)
		panes := paneIDRe.FindAllStringSubmatch(out, -1)
		if len(clicks) != 1 || len(panes) != 1 {
			t.Fatalf("static=%v: want one tab button and one pane, got %d and %d", static, len(clicks), len(panes))
		}
		if clicks[0][1] != "tab-aalert1" || panes[0][1] != "tab-aalert1" {
			t.Errorf("static=%v: onclick target %q / pane id %q, want tab-aalert1 for both", static, clicks[0][1], panes[0][1])
		}
	}
}
