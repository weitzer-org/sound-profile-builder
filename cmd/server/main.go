package main

import (
	"context"
	"log"
	"os"

	"github.com/weitzer-org/sound-builder/internal/agents"
	"github.com/weitzer-org/sound-builder/internal/api"
	"github.com/weitzer-org/sound-builder/internal/storage"
)

func main() {
	log.Println("Starting QC-2 Multi-Agent Modeler Backend...")

	ctx := context.Background()
	gcsClient, err := storage.NewGCSClient(ctx)
	if err != nil {
		log.Fatalf("Failed to initialize GCS client: %v", err)
	}
	defer gcsClient.Close()

	smClient, err := storage.NewSecretManagerClient(ctx)
	if err != nil {
		log.Fatalf("Failed to initialize Secret Manager client: %v", err)
	}
	defer smClient.Close()

	store := storage.NewPresetStore(gcsClient, "weitzer-sound-builder")

	orchMaker := func(ic context.Context, key string) (agents.OrchestratorService, error) {
		return agents.NewOrchestrator(ic, key)
	}

	// Initialize Server
	server := api.NewServer(store, gcsClient, smClient, orchMaker)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8081"
	}

	// Start server
	if err := server.Start(":" + port); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
