package devicejobs

import (
	"strings"
	"testing"
)

func TestSanitizeText(t *testing.T) {
	cases := []struct{ in, want string }{
		{"Clean Verb", "Clean Verb"},
		{"a\x00b‮c​d", "abcd"},
		{"x`y```z", "x'y'''z"},
		{"line one\n\n# line   two\t", "line one # line two"},
		{"x![p](https://evil.example/t.png)", "x![p](https://evil.example/t.png)"}, // inert in a code span; not this layer's job
	}
	for _, c := range cases {
		if got := SanitizeText(c.in, 200); got != c.want {
			t.Errorf("SanitizeText(%q) = %q, want %q", c.in, got, c.want)
		}
	}
	if got := SanitizeText(strings.Repeat("é", 100), 10); got != strings.Repeat("é", 10)+"…" {
		t.Errorf("cut = %q", got)
	}
	if got := SanitizeText(strings.Repeat("a", 1<<20), 8); got != "aaaaaaaa…" {
		t.Errorf("long input = %q", got)
	}
	if got := SanitizeText("\xff\xfe", 8); got != "" {
		t.Errorf("invalid UTF-8 = %q", got)
	}
}

func TestParseReadbackPayloadAcceptsMinimal(t *testing.T) {
	p, err := ParseReadbackPayload([]byte(`{"setlist":{"folder":"other","position":null},"chains":[],"bypass":null}`))
	if err != nil || len(p.Chains) != 0 || p.Bypass != nil {
		t.Fatalf("p=%+v err=%v", p, err)
	}
	for _, bad := range []string{``, `null`, `[]`, `{"setlist":{"folder":"other","position":null},"chains":[],"bypass":null} x`} {
		if _, err := ParseReadbackPayload([]byte(bad)); err == nil {
			t.Errorf("%q was accepted", bad)
		}
	}
}
