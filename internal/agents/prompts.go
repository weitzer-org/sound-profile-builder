package agents

import (
	"embed"
	"fmt"
	"os"
)

//go:embed prompts/*.md
var PromptFS embed.FS

// LoadPrompt fetches the markdown text representation of the system/system-instructions for a given agent
func LoadPrompt(agentFileName string) (string, error) {
	version := os.Getenv("PROMPT_VERSION")
	if version == "v2" {
		v2Path := fmt.Sprintf("prompts/%s_v2.md", agentFileName)
		bytes, err := PromptFS.ReadFile(v2Path)
		if err == nil {
			return string(bytes), nil
		}
		// Fallback to v1 if v2 file doesn't exist
	}

	bytes, err := PromptFS.ReadFile(fmt.Sprintf("prompts/%s.md", agentFileName))
	if err != nil {
		return "", fmt.Errorf("failed to load prompt file %s: %w", agentFileName, err)
	}
	return string(bytes), nil
}
