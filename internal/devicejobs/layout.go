package devicejobs

import (
	"errors"
	"fmt"
	"strings"
)

// SceneLetters names the device's eight scenes, A-H, by index 0-7.
const SceneLetters = "ABCDEFGH"

// GuitarScenes puts one guitar of a preset on two device scenes: its Scene A values (the
// rhythm tone) on Rhythm and its Scene B values (the lead tone) on Lead. Scenes are indexes 0-7.
type GuitarScenes struct {
	Guitar string
	Rhythm int
	Lead   int
}

// SceneLayout says which guitars go into one slot and onto which scenes. One or two guitars;
// scenes nobody takes are written with the first guitar's rhythm values (G6: the device
// keeps an unwritten scene's old value, so every scene is written). Nil, or one guitar on
// scenes A and B, is the original single-guitar push.
type SceneLayout struct {
	Guitars []GuitarScenes
}

// ParseScene reads a scene letter "A".."H" (case-insensitive) as an index 0-7.
func ParseScene(s string) (int, bool) {
	s = strings.ToUpper(strings.TrimSpace(s))
	if len(s) != 1 {
		return 0, false
	}
	i := strings.Index(SceneLetters, s)
	return i, i >= 0
}

// Letter is the scene letter for index i.
func Letter(i int) string {
	if i < 0 || i >= len(SceneLetters) {
		return "?"
	}
	return SceneLetters[i : i+1]
}

// DefaultLayout is one guitar on Scenes A (rhythm) and B (lead).
func DefaultLayout(guitar string) SceneLayout {
	return SceneLayout{Guitars: []GuitarScenes{{Guitar: guitar, Rhythm: 0, Lead: 1}}}
}

// IsDefault reports whether this is the original one-guitar A/B push, which keeps its
// original payload and plan byte for byte.
func (l SceneLayout) IsDefault() bool {
	return len(l.Guitars) == 1 && l.Guitars[0].Rhythm == 0 && l.Guitars[0].Lead == 1
}

// Validate checks the layout's shape: one or two distinct guitars, every scene in 0-7 and no
// scene given to two roles.
func (l SceneLayout) Validate() error {
	if n := len(l.Guitars); n < 1 || n > 2 {
		return fmt.Errorf("a scene layout holds one or two guitars, got %d", n)
	}
	used := map[int]string{}
	for i, g := range l.Guitars {
		if strings.TrimSpace(g.Guitar) == "" {
			return errors.New("a scene layout guitar has no name")
		}
		if i > 0 && g.Guitar == l.Guitars[0].Guitar {
			return errors.New("the second guitar must be a different guitar from the first")
		}
		for _, role := range []struct {
			name string
			s    int
		}{{"rhythm", g.Rhythm}, {"lead", g.Lead}} {
			if role.s < 0 || role.s >= len(SceneLetters) {
				return fmt.Errorf("%s scene %d is not one of A-H", role.name, role.s)
			}
			if prev, dup := used[role.s]; dup {
				return fmt.Errorf("Scene %s is given to both %s and %s %s: each scene takes one tone", Letter(role.s), prev, g.Guitar, role.name)
			}
			used[role.s] = g.Guitar + " " + role.name
		}
	}
	return nil
}

// SceneSource names where one scene's values come from: guitar index and whether it is that
// guitar's lead (Scene B) value.
type SceneSource struct {
	Guitar int
	Lead   bool
}

// Sources returns, for each of the eight scenes, which guitar's rhythm or lead value it
// takes. A scene nobody takes takes the first guitar's rhythm value.
func (l SceneLayout) Sources() [8]SceneSource {
	var out [8]SceneSource
	for gi, g := range l.Guitars {
		if g.Rhythm >= 0 && g.Rhythm < 8 {
			out[g.Rhythm] = SceneSource{Guitar: gi}
		}
		if g.Lead >= 0 && g.Lead < 8 {
			out[g.Lead] = SceneSource{Guitar: gi, Lead: true}
		}
	}
	return out
}

// Used reports whether a scene is given to a tone.
func (l SceneLayout) Used(scene int) bool {
	for _, g := range l.Guitars {
		if g.Rhythm == scene || g.Lead == scene {
			return true
		}
	}
	return false
}

// Label is the guitars' names joined, for the payload's informational guitar field.
func (l SceneLayout) Label() string {
	names := make([]string, len(l.Guitars))
	for i, g := range l.Guitars {
		names[i] = g.Guitar
	}
	return strings.Join(names, " + ")
}

// PlanLayout is the readable scene assignment of a non-default layout, for the dry run.
type PlanLayout struct {
	Scenes [8]PlanScene `json:"scenes"`
}

// PlanScene is what one device scene holds.
type PlanScene struct {
	Letter string `json:"letter"`
	Guitar string `json:"guitar,omitempty"` // empty on an unused scene
	Role   string `json:"role"`             // "rhythm", "lead" or "unused"
}
