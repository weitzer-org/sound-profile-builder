package agents

import (
	"strings"
	"testing"

	"github.com/weitzer-org/sound-builder/internal/storage"
)

func TestDeviceControlParseAndDefault(t *testing.T) {
	d, ok := DeviceControlsFor("Analog Delay")
	if !ok {
		t.Fatal("Analog Delay not found")
	}
	ctl := func(name string) DeviceControl { return d.Controls[d.Index(name)] }
	if x, ok := ctl("Delay Time").ParseValue("0.25 s"); !ok || x != 250 {
		t.Errorf("0.25 s on DELAY TIME = %v, %v; want 250", x, ok)
	}
	if _, ok := ctl("MIX").ParseValue("3 dB"); ok {
		t.Error("dB on a % control should not parse")
	}
	if !ctl("TRAILS").IsDefault("on") || ctl("TRAILS").IsDefault("Off") {
		t.Error("TRAILS default is On (case-insensitive)")
	}
	if !ctl("MIX").IsDefault("35 %") || !ctl("MIX").IsDefault("35") || ctl("MIX").IsDefault("36 %") {
		t.Error("MIX default is 35 %")
	}
	if got := ctl("MIX").DefaultValue(); got != "35 %" {
		t.Errorf("MIX DefaultValue = %q", got)
	}
	eq, _ := DeviceControlsFor("Graphic-9")
	lpf := eq.Controls[eq.Index("LPF")]
	if got := lpf.DefaultValue(); got != "OFF" {
		t.Errorf("Graphic-9 LPF default should display OFF, got %q", got)
	}
	if !lpf.IsDefault("OFF") {
		t.Error(`"OFF" should read as the LPF's OFF end`)
	}
	if _, ok := DeviceControlsFor("Some Unknown Block"); ok {
		t.Error("unknown name should have no controls")
	}
	if _, ok := DeviceControlsFor("US DLX 58"); ok {
		t.Error("a capture (US DLX 58) should have no catalog controls")
	}
}

func TestSplitDefaults(t *testing.T) {
	b := storage.EffectBlock{Type: "Delay", Model: "Analog Delay",
		Parameters: []storage.BlockParameter{
			{Name: "MIX", Value: "35 %", ValueB: "40 %"}, // differs in B: shown
			{Name: "FEEDBACK", Value: "25 %"},            // equals default in both: folded
			{Name: "DELAY TIME", Value: "350 ms"},        // changed: shown
			{Name: "TRAILS", Value: "Off"},               // changed and also (wrongly) kept: shown only
			{Name: "Bypass", Value: "Off"},               // not a catalog control: shown
		},
		KeptDefaults: []string{"WIDTH", "PING PONG", "TRAILS", "NOT A CONTROL"},
	}
	shown, atDefault := SplitDefaults(b)
	var names []string
	for _, p := range shown {
		names = append(names, p.Name)
	}
	if got := strings.Join(names, ","); got != "MIX,DELAY TIME,TRAILS,Bypass" {
		t.Errorf("shown = %s", got)
	}
	if got := strings.Join(atDefault, ","); got != "FEEDBACK,PING PONG,WIDTH" {
		t.Errorf("atDefault = %s (want device order, no TRAILS, no unknown names)", got)
	}
	capture := storage.EffectBlock{Model: "US DLX 58 (Factory Capture)", Parameters: []storage.BlockParameter{{Name: "Gain", Value: "+1 dB"}}}
	if shown, atDefault := SplitDefaults(capture); len(shown) != 1 || atDefault != nil {
		t.Errorf("a capture keeps every parameter: %v %v", shown, atDefault)
	}
}
