package agents

import (
	"testing"

	"github.com/weitzer-org/sound-builder/internal/stomplayout"
)

// The real library through the real classifier: "Strymon Sunset BOOST" is a drive entry
// whose name says boost; a full_rig entry gets no switch; a name not in the library is
// unclassified. Uses a Neural Capture slot id from the catalog (14001), which is a test
// assumption about how a capture block is identified: UNVERIFIED on the device.
func TestStompCaptureLookupRealLibrary(t *testing.T) {
	c, err := stomplayout.NewEmbeddedClassifier(StompCaptureLookup())
	if err != nil {
		t.Fatal(err)
	}
	const capID = 14001
	for _, tc := range []struct {
		name string
		want stomplayout.Role
		un   bool
	}{
		{"Strymon Sunset BOOST", stomplayout.RoleBoost, false},
		{"TR KOTyellow Boost SC", stomplayout.RoleBoost, false},
		{"Boss Blues Driver Waza", stomplayout.RoleDrive, false},
		{"Sears Silvertone", stomplayout.RoleNone, false}, // full_rig
		{"Not In The Library", stomplayout.RoleNone, true},
	} {
		cl := c.Classify(stomplayout.Block{ModelID: capID, CaptureName: tc.name})
		if cl.Role != tc.want || cl.Unclassified != tc.un {
			t.Errorf("%s: role=%q unclassified=%v, want %q/%v", tc.name, cl.Role, cl.Unclassified, tc.want, tc.un)
		}
	}
	if _, ok := StompCaptureLookup()("1964 Fender Super Reverb"); !ok {
		t.Error("eval-preset capture missing from the library")
	}
}
