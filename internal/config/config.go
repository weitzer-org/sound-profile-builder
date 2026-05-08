package config

import (
	"encoding/json"
	"os"
)

type AppConfig struct {
	SingleAmpMode      bool              `json:"single_amp_mode"`
	AllowCloudCaptures bool              `json:"allow_cloud_captures"`
	AllowFactoryCaptures bool              `json:"allow_factory_captures"`
	AllowPaidPlugins   bool              `json:"allow_paid_plugins"`
	AvailablePlugins   []string          `json:"available_plugins"`
	ProjectID          string            `json:"project_id"`
	BucketName         string            `json:"bucket_name"`
	AgentPrompts       map[string]string `json:"agent_prompts"`
	AgentModels        map[string]string `json:"agent_models"`
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

	if envBucket := os.Getenv("GCS_BUCKET"); envBucket != "" {
		cfg.BucketName = envBucket
	}
	if envProject := os.Getenv("GOOGLE_CLOUD_PROJECT"); envProject != "" {
		cfg.ProjectID = envProject
	}

	return &cfg, nil
}
