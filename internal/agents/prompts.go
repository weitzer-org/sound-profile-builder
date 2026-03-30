package agents

import (
	"embed"
	"fmt"
)

//go:embed prompts/*.md
var PromptFS embed.FS

// LoadPrompt fetches the markdown text representation of the system/system-instructions for a given agent
func LoadPrompt(agentFileName string) (string, error) {
	bytes, err := PromptFS.ReadFile(fmt.Sprintf("prompts/%s.md", agentFileName))
	if err != nil {
		return "", fmt.Errorf("failed to load prompt file %s: %w", agentFileName, err)
	}
	return string(bytes), nil
}
