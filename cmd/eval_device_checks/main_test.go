package main

import "testing"

func TestCountBlocks(t *testing.T) {
	raw := "```json\n" + `{"structured_payload":{"guitars":{"A":[{"id":"1"},{"id":"2"}],"B":[{"id":"3"}]}}}` + "\n```"
	if n := countBlocks(raw); n != 3 {
		t.Errorf("countBlocks = %d, want 3", n)
	}
	if n := countBlocks("not json"); n != 0 {
		t.Errorf("countBlocks(garbage) = %d, want 0", n)
	}
}
