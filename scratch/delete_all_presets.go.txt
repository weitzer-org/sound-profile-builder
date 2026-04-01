package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/weitzer-org/sound-builder/internal/storage"
)

func main() {
	ctx := context.Background()

	// 1. Initialize storage client (using local mock or real GCS based on environment)
	client, err := storage.NewGCSClient(ctx)
	if err != nil {
		log.Fatalf("Failed to create storage client: %v", err)
	}
	defer client.Close()

	bucket := os.Getenv("PRESET_BUCKET_NAME")
	if bucket == "" {
		bucket = "weitzer-sound-builder" // Default fallback from config.json
	}

	store := storage.NewPresetStore(client, bucket)

	// 2. List all presets
	presets, err := store.List(ctx)
	if err != nil {
		log.Fatalf("Failed to list presets: %v", err)
	}

	if len(presets) == 0 {
		fmt.Println("No presets found to delete.")
		return
	}

	fmt.Printf("Found %d presets. Deleting all...\n", len(presets))

	// 3. Delete each preset
	for _, p := range presets {
		fmt.Printf("Deleting preset [%s] (%s)...\n", p.Name, p.ID)
		if err := store.Delete(ctx, p.ID); err != nil {
			log.Printf("Failed to delete preset %s: %v", p.ID, err)
		} else {
			fmt.Printf("Deleted preset %s successfully.\n", p.ID)
		}
	}

	fmt.Println("Cleanup complete.")
}
