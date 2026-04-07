package agents

import (
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
	return string(b), nil
}
