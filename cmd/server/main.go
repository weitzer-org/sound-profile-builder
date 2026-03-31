package main

import (
	"context"
	"log"
	"os"

	"github.com/weitzer-org/sound-builder/internal/agents"
	"github.com/weitzer-org/sound-builder/internal/api"
	"github.com/weitzer-org/sound-builder/internal/config"
	"github.com/weitzer-org/sound-builder/internal/storage"
)

type localSecretFetcher struct {
	uiPassword string
}

func (l *localSecretFetcher) GetPassword(ctx context.Context, projectID, secretName string) (string, error) {
	if secretName == "spb-login-pw" {
		return l.uiPassword, nil
	}
	if key := os.Getenv("GEMINI_API_KEY"); key != "" {
		return key, nil
	}
	return "mock-api-key", nil
}

func (l *localSecretFetcher) Close() {}

func main() {
	log.Println("Starting QC-2 Multi-Agent Modeler Backend...")

	ctx := context.Background()
	gcsClient, err := storage.NewGCSClient(ctx)
	if err != nil {
		log.Fatalf("Failed to initialize GCS client: %v", err)
	}
	defer gcsClient.Close()

	var smFetcher storage.SecretFetcher
	mockPassword := os.Getenv("MOCK_PASSWORD")
	if mockPassword != "" {
		log.Println("Using Local Secret Fetcher for UI Authentication")
		smFetcher = &localSecretFetcher{uiPassword: mockPassword}
	} else {
		smClient, err := storage.NewSecretManagerClient(ctx)
		if err != nil {
			log.Fatalf("Failed to initialize Secret Manager client: %v", err)
		}
		defer smClient.Close()
		smFetcher = smClient
	}

	cfg, err := config.LoadConfig("config.json")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	store := storage.NewPresetStore(gcsClient, cfg.BucketName)
	memoryStore := storage.NewMemoryStore(gcsClient, cfg.BucketName)
	orchMaker := func(ic context.Context, key string) (agents.OrchestratorService, error) {
		return agents.NewOrchestrator(ic, key, gcsClient)
	}

	// Initialize Server
	server := api.NewServer(store, memoryStore, gcsClient, smFetcher, orchMaker, cfg)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8081"
	}

	// Start server
	if err := server.Start(":" + port); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
