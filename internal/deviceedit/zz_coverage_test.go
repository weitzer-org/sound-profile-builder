package deviceedit

import "testing"

// Every item code has a case above (run after the other tests in this package).
func TestEveryReasonHasACase(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	for _, c := range Codes() {
		if !seen[c] && !unreachable[c] {
			t.Errorf("no test case produced %q", c)
		}
	}
	if len(Codes()) != len(reasons) {
		t.Error("codes and reasons differ")
	}
}
