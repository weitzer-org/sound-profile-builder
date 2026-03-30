package agents

import (
	"encoding/json"
	"fmt"
	"os"
)

type geminiMockResp struct {
	Candidates []struct {
		Content struct {
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
		} `json:"content"`
	} `json:"candidates"`
}

func readMockFile(path string) (string, error) {
	b, err := os.ReadFile("../../" + path) // since process runs in root
	if err != nil {
		b, err = os.ReadFile(path) // fallback if ran from root
		if err != nil {
			return "", err
		}
	}
	var resp geminiMockResp
	if err := json.Unmarshal(b, &resp); err != nil {
		return "", err
	}
	if len(resp.Candidates) > 0 && len(resp.Candidates[0].Content.Parts) > 0 {
		return resp.Candidates[0].Content.Parts[0].Text, nil
	}
	return "", fmt.Errorf("no mock candidates found")
}
