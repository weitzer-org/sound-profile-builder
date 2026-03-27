package config

import (
	"encoding/json"
	"os"
)

type AppConfig struct {
	SingleAmpMode      bool `json:"single_amp_mode"`
	AllowCloudCaptures bool `json:"allow_cloud_captures"`
}

func LoadConfig(path string) (*AppConfig, error) {
	bytes, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg AppConfig
	if err := json.Unmarshal(bytes, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}
