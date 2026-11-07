package evaldevicerefine

import (
	"strings"

	"github.com/weitzer-org/sound-builder/internal/agents"
	"github.com/weitzer-org/sound-builder/internal/config"
	"github.com/weitzer-org/sound-builder/internal/deviceexport"
	"github.com/weitzer-org/sound-builder/internal/devicejobs"
)

// NewExporter builds the exporter the server builds (api.Server.DeviceExporter) without the
// storage-backed device library: the user's plugins from cfg (nil = none), capture names from
// coros_map.json and user_captures.json, device text through devicejobs.SanitizeText.
func NewExporter(cfg *config.AppConfig) (*deviceexport.Exporter, error) {
	var owned map[string]bool
	if cfg != nil {
		owned = deviceexport.OwnedPluginSet(cfg.AllowPaidPlugins, cfg.AvailablePlugins)
	}
	return deviceexport.NewEmbedded(deviceexport.Options{
		CaptureSource: agents.CaptureSource, OwnedPlugins: owned, StompCaptures: agents.StompCaptureLookup(),
		LookupCapture: lookupCapture, Sanitize: devicejobs.SanitizeText,
	})
}

// lookupCapture is api.lookupCapture's rule: an exact name ignoring case; a user entry wins,
// then the lexicographically first factory spelling.
func lookupCapture(deviceName string) (name, source string, ok bool) {
	want := strings.TrimSpace(deviceName)
	if want == "" {
		return "", "", false
	}
	for n, isCap := range agents.GetValidNativeBlocks() {
		if !isCap || !strings.EqualFold(n, want) {
			continue
		}
		if agents.IsUserCaptureName(n) {
			if source != "user" || n < name {
				name, source, ok = n, "user", true
			}
		} else if source != "user" && (!ok || n < name) {
			name, source, ok = n, "factory", true
		}
	}
	return name, source, ok
}
