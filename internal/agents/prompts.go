package agents

import (
	"embed"
	"fmt"
)

//go:embed prompts/*.md
var PromptFS embed.FS

// LoadPrompt fetches the markdown text representation of the system/system-instructions for a given agent.
// Any version other than "" or "off" is loaded from a "_<version>.md" sibling file (e.g. "v2" -> "_v2.md",
// "v3" -> "_v3.md"), so every prompt change stays versioned and A/B-able via the existing agentConfig
// override mechanism instead of being edited in place on the base file.
func LoadPrompt(agentFileName string, version string) (string, error) {
	if version != "" && version != "off" {
		versionedPath := fmt.Sprintf("prompts/%s_%s.md", agentFileName, version)
		bytes, err := PromptFS.ReadFile(versionedPath)
		if err == nil {
			return string(bytes), nil
		}
		// Fallback to the base (v1) prompt if this specific version file doesn't exist.
	}

	bytes, err := PromptFS.ReadFile(fmt.Sprintf("prompts/%s.md", agentFileName))
	if err != nil {
		return "", fmt.Errorf("failed to load prompt file %s: %w", agentFileName, err)
	}
	return string(bytes), nil
}
