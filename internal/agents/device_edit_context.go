package agents

import (
	"context"
	"fmt"

	"github.com/weitzer-org/sound-builder/internal/storage"
)

// DeviceEditPromptKey is the agent_prompts key that turns on the edit-aware refine context
// for drafts read back from the Quad Cortex (preset-editing plan, sections 3.3 and 8.1).
// Its value names a section file, prompts/12_architect_device_edit_<version>.md. Unset or
// "off" (the config.json default) means a device draft is refined exactly like any preset.
// The section is appended to the Architect's per-run context, never to its system prompt, so
// the Architect prompt files stay as they are.
const DeviceEditPromptKey = "12_architect_device_edit"

// deviceEditVersion is the configured section version, or "" when the section is off.
func deviceEditVersion(ctx context.Context) string {
	if v := PromptVersionsFrom(ctx)[DeviceEditPromptKey]; v != "" && v != "off" {
		return v
	}
	return ""
}

// IsDeviceDraft reports whether p was imported from the unit's open preset.
func IsDeviceDraft(p *storage.Preset) bool {
	return p != nil && p.Origin != nil && p.Origin.Kind == storage.OriginDeviceReadback
}

// deviceEditContext returns the section to append to RefineChat's context: "" for every
// preset that is not a device draft and for every run that has not turned the section on.
// A configured version whose file is missing is an error (never a silent fallback to the
// ordinary context, which would make an eval arm measure the wrong thing).
func deviceEditContext(ctx context.Context, p *storage.Preset) (string, error) {
	if !IsDeviceDraft(p) {
		return "", nil
	}
	v := deviceEditVersion(ctx)
	if v == "" {
		return "", nil
	}
	b, err := PromptFS.ReadFile(fmt.Sprintf("prompts/%s_%s.md", DeviceEditPromptKey, v))
	if err != nil {
		return "", fmt.Errorf("device edit context %s %s: %w", DeviceEditPromptKey, v, err)
	}
	s := "\n\n" + string(b)
	if n := p.Origin.Unconverted; n > 0 {
		s += fmt.Sprintf("\nThe import left %d control(s) or block(s) of the unit's preset out of this draft. They are locked: leave them alone.\n", n)
	}
	return s, nil
}
