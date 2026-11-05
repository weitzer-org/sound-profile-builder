package main

import (
	"strings"
	"testing"
)

func TestStrip(t *testing.T) {
	in := `{"builder_statement":"x","final_html_payload":"<table>⚠️ Critic: bad</table>","structured_payload":{"guitars":{"G":[
	 {"id":"a","type":"Amplifier","model":"M","rationale":"Real reason. ⚠️ Critic: The rationale claims X.","parameters":[]},
	 {"id":"b","type":"Drive","model":"D","rationale":"Own ⚠️ warning kept.","parameters":[]}]}}}`
	out, n, err := Strip([]byte(in))
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	if n != 1 || strings.Contains(s, "Critic:") || strings.Contains(s, "final_html_payload") {
		t.Fatalf("n=%d out=%s", n, s)
	}
	if !strings.Contains(s, "Real reason.") || !strings.Contains(s, "warning kept") || !strings.Contains(s, "builder_statement") {
		t.Fatalf("lost content: %s", s)
	}
}
