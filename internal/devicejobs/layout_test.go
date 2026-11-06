package devicejobs

import (
	"strings"
	"testing"
)

// With both scenes invalid the error always names the rhythm scene first.
func TestSceneLayoutValidateIsDeterministic(t *testing.T) {
	l := SceneLayout{Guitars: []GuitarScenes{{Guitar: "ES", Rhythm: 9, Lead: 12}}}
	for i := 0; i < 50; i++ {
		err := l.Validate()
		if err == nil || !strings.HasPrefix(err.Error(), "rhythm scene 9") {
			t.Fatalf("run %d: %v", i, err)
		}
	}
}
