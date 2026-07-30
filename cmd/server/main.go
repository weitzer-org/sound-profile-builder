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

	// Storage backend selection. Defaults to GCS so the existing Google Cloud
	// deployment path is unchanged; set STORAGE_BACKEND=s3 for an S3-compatible
	// store (Cloudflare R2 in prod, MinIO for local Docker development).
	backend := os.Getenv("STORAGE_BACKEND")
	if backend == "" {
		backend = "gcs"
	}

	var storeClient storage.Client
	switch backend {
	case "s3":
		if os.Getenv("S3_BUCKET") == "" {
			log.Fatalf("STORAGE_BACKEND=s3 requires S3_BUCKET to be set")
		}
		s3Client, err := storage.NewS3Client(ctx)
		if err != nil {
			log.Fatalf("Failed to initialize S3 storage client: %v", err)
		}
		storeClient = s3Client
		log.Println("Storage backend: S3-compatible")
	case "gcs":
		gcsClient, err := storage.NewGCSClient(ctx)
		if err != nil {
			log.Fatalf("Failed to initialize GCS client: %v", err)
		}
		storeClient = gcsClient
		log.Println("Storage backend: Google Cloud Storage")
	default:
		log.Fatalf("Unknown STORAGE_BACKEND %q (expected \"gcs\" or \"s3\")", backend)
	}
	defer storeClient.Close()

	var smFetcher storage.SecretFetcher
	// UI login password: prefer UI_PASSWORD (production), fall back to
	// MOCK_PASSWORD (local dev). Either one selects the local secret fetcher and
	// bypasses GCP Secret Manager.
	uiPassword := os.Getenv("UI_PASSWORD")
	if uiPassword == "" {
		uiPassword = os.Getenv("MOCK_PASSWORD")
	}
	if uiPassword != "" {
		log.Println("Using Local Secret Fetcher for UI Authentication")
		smFetcher = &localSecretFetcher{uiPassword: uiPassword}
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

	// Resolve the bucket name. For S3, S3_BUCKET wins; otherwise fall back to
	// the config value (which GCS_BUCKET may already have overridden).
	bucket := cfg.BucketName
	if backend == "s3" {
		if b := os.Getenv("S3_BUCKET"); b != "" {
			bucket = b
		}
	}

	store := storage.NewPresetStore(storeClient, bucket)
	memoryStore := storage.NewMemoryStore(storeClient, bucket)
	orchMaker := func(ic context.Context, key string) (agents.OrchestratorService, error) {
		orch, err := agents.NewOrchestrator(ic, key, storeClient)
		if err != nil {
			return nil, err
		}
		// Enables durable per-call token/latency/cost/error analytics (see
		// internal/agents/usage_recorder.go) -- left unset for every other
		// caller of NewOrchestrator (cmd/eval_*, cmd/enrich_captures, etc.),
		// which construct an Orchestrator with gcs=nil anyway and have no
		// durability requirement for this data.
		orch.UsageBucket = bucket
		return orch, nil
	}

	// Initialize Server
	server := api.NewServer(store, memoryStore, storeClient, smFetcher, orchMaker, cfg)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("QC-2 Backend listening at: http://localhost:%s\n", port)
	if err := server.Start(":" + port); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
