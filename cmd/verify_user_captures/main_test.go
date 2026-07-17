package main

import "testing"

func TestFilterUnverified(t *testing.T) {
	captures := []userCapture{
		{Name: "a", DescriptionVerified: false},
		{Name: "b", DescriptionVerified: true},
		{Name: "c", DescriptionVerified: false},
		{Name: "d", DescriptionVerified: true},
	}

	got := filterUnverified(captures)

	if len(got) != 2 {
		t.Fatalf("Expected 2 unverified captures, got %d: %v", len(got), got)
	}
	names := map[string]bool{}
	for _, c := range got {
		names[c.Name] = true
	}
	if !names["a"] || !names["c"] {
		t.Errorf("Expected unverified captures 'a' and 'c', got %v", got)
	}
	if names["b"] || names["d"] {
		t.Errorf("Expected already-verified captures 'b'/'d' to be excluded, got %v", got)
	}
}
