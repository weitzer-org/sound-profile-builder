package agents

import (
	"sync"

	"github.com/weitzer-org/sound-builder/internal/stomplayout"
)

var (
	stompCaptureTypes     map[string]string
	stompCaptureTypesOnce sync.Once
)

// StompCaptureLookup adapts the embedded user_captures.json to stomplayout.CaptureLookup:
// an exact name match returns the entry's block_type. Factory captures (coros_map.json) are
// deliberately not included (docs/design/qc-stomp-layout.md section 4.1). It lives here so
// internal/stomplayout never imports this package.
func StompCaptureLookup() stomplayout.CaptureLookup {
	stompCaptureTypesOnce.Do(func() {
		stompCaptureTypes = make(map[string]string)
		for _, c := range GetUserCaptures() {
			if c.Name == "" {
				continue
			}
			if _, dup := stompCaptureTypes[c.Name]; !dup {
				stompCaptureTypes[c.Name] = c.BlockType
			}
		}
	})
	return func(name string) (string, bool) {
		bt, ok := stompCaptureTypes[name]
		return bt, ok
	}
}
