package config

import (
	"encoding/json"
	"os"
)

type AppConfig struct {
	SingleAmpMode      bool   `json:"single_amp_mode"`
	AllowCloudCaptures bool   `json:"allow_cloud_captures"`
	ProjectID          string `json:"project_id"`
	BucketName         string `json:"bucket_name"`
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
