package stomplayout

import (
	"strings"
	"testing"
)

func TestEmbeddedLayout(t *testing.T) {
	l := testLayout(t)
	want := map[string]string{"comp": "A", "boost": "B", "drive": "C", "drive2": "D", "delay": "E", "mod": "F", "reverb": "G"}
	for slot, sw := range want {
		if got, ok := l.SwitchFor(slot); !ok || got != sw {
			t.Errorf("%s -> %q (%v), want %s", slot, got, ok, sw)
		}
	}
	if !l.IsReserved("H") || len(l.Reserved) != 1 {
		t.Errorf("reserved = %v", l.Reserved)
	}
	if got := l.Summary(); got != "A=comp B=boost C=drive D=drive2 E=delay F=mod G=reverb" {
		t.Errorf("summary %q", got)
	}
}

func TestParseLayoutRefusals(t *testing.T) {
	for _, tc := range []struct {
		name, json, wantErr string
	}{
		{"switch outside A-H", `{"version":1,"switches":{"I":"comp"},"reserved":[]}`, "outside A-H"},
		{"lower-case switch", `{"version":1,"switches":{"a":"comp"},"reserved":[]}`, "outside A-H"},
		{"two-letter switch", `{"version":1,"switches":{"AB":"comp"},"reserved":[]}`, "outside A-H"},
		{"role on two switches", `{"version":1,"switches":{"A":"comp","B":"comp"},"reserved":["H"]}`, "two switches"},
		{"reserved switch has a role", `{"version":1,"switches":{"A":"comp","H":"reverb"},"reserved":["H"]}`, "reserved switch H"},
		{"unknown role", `{"version":1,"switches":{"A":"flanger"},"reserved":["H"]}`, "unknown role"},
		{"drive2 without drive", `{"version":1,"switches":{"D":"drive2"},"reserved":["H"]}`, "needs a \"drive\""},
		{"reserved outside A-H", `{"version":1,"switches":{"A":"comp"},"reserved":["Z"]}`, "outside A-H"},
		{"reserved twice", `{"version":1,"switches":{"A":"comp"},"reserved":["H","H"]}`, "twice"},
		{"duplicate switch key", `{"version":1,"switches":{"A":"comp","A":"boost"},"reserved":["H"]}`, "appears twice"},
		{"no roles", `{"version":1,"switches":{},"reserved":["H"]}`, "no switch has a role"},
		{"missing switches", `{"version":1,"reserved":["H"]}`, "missing"},
		{"role not a string", `{"version":1,"switches":{"A":1},"reserved":[]}`, "not a string"},
		{"wrong version", `{"version":2,"switches":{"A":"comp"},"reserved":[]}`, "version"},
		{"unknown field", `{"version":1,"switches":{"A":"comp"},"reserved":[],"extra":1}`, "unknown field"},
		{"trailing data", `{"version":1,"switches":{"A":"comp"},"reserved":[]} {}`, "trailing"},
		{"not json", `nope`, "stomp layout"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseLayout([]byte(tc.json))
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("err = %v, want containing %q", err, tc.wantErr)
			}
		})
	}
}

func TestParseLayoutAcceptsPartialAndOtherSwitches(t *testing.T) {
	l, err := ParseLayout([]byte(`{"version":1,"switches":{"G":"comp","A":"reverb"},"reserved":["H"]}`))
	if err != nil {
		t.Fatal(err)
	}
	if sw, _ := l.SwitchFor("comp"); sw != "G" {
		t.Errorf("comp on %q", sw)
	}
	if _, ok := l.SwitchFor("boost"); ok {
		t.Error("boost should have no switch")
	}
}
