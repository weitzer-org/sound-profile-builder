package config

import (
	"os"
	"testing"
)

func TestLoadConfig_Success(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "config_*.json")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	content := `{"single_amp_mode": true, "allow_cloud_captures": false, "allow_paid_plugins": true, "allow_factory_captures": true, "available_plugins": ["Cory Wong"]}`
	if _, err := tmpFile.Write([]byte(content)); err != nil {
		t.Fatalf("Failed to write to temp file: %v", err)
	}
	tmpFile.Close()

	cfg, err := LoadConfig(tmpFile.Name())
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if !cfg.SingleAmpMode {
		t.Error("Expected SingleAmpMode to be true")
	}
	if cfg.AllowCloudCaptures {
		t.Error("Expected AllowCloudCaptures to be false")
	}
	if !cfg.AllowPaidPlugins {
		t.Error("Expected AllowPaidPlugins to be true")
	}
	if !cfg.AllowFactoryCaptures {
		t.Error("Expected AllowFactoryCaptures to be true")
	}
	if len(cfg.AvailablePlugins) != 1 || cfg.AvailablePlugins[0] != "Cory Wong" {
		t.Errorf("Expected AvailablePlugins to be ['Cory Wong'], got %v", cfg.AvailablePlugins)
	}
}

func TestLoadConfig_FileNotFound(t *testing.T) {
	_, err := LoadConfig("nonexistent_file.json")
	if err == nil {
		t.Error("Expected error for non-existent file, got nil")
	}
}

func TestLoadConfig_InvalidJSON(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "config_invalid_*.json")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	content := `{invalid json`
	if _, err := tmpFile.Write([]byte(content)); err != nil {
		t.Fatalf("Failed to write to temp file: %v", err)
	}
	tmpFile.Close()

	_, err = LoadConfig(tmpFile.Name())
	if err == nil {
		t.Error("Expected error for invalid JSON, got nil")
	}
}
