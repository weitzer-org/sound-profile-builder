package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/weitzer-org/sound-builder/internal/agents"
	"github.com/weitzer-org/sound-builder/internal/api"
	"github.com/weitzer-org/sound-builder/internal/config"
	"github.com/weitzer-org/sound-builder/internal/devicejobs"
	"github.com/weitzer-org/sound-builder/internal/mcpserver"
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

	// GSR_USAGE_INGEST_URL/GSR_USAGE_INGEST_KEY report this project's own
	// native Gemini usage to GSR's hosted usage dashboard -- opt-in exactly
	// like GSR's own EVALUATOR_SHARED_SECRET convention: absent means
	// silently disabled, not a startup error, since this is a supplementary
	// analytics side-channel, not a primary feature. One Reporter per
	// process, shared by every Orchestrator orchMaker builds, the same way
	// storeClient is shared -- constructing one per request would defeat its
	// bounded worker pool.
	var usageReporter *agents.UsageReporter
	if ingestURL := os.Getenv("GSR_USAGE_INGEST_URL"); ingestURL != "" {
		if ingestKey := os.Getenv("GSR_USAGE_INGEST_KEY"); ingestKey != "" {
			usageReporter = agents.NewUsageReporter(ingestURL, ingestKey, "weitzer-org/sound-profile-builder")
		}
	}

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
		// Agent output logs go to the run store's bucket too, so get_agent_output
		// reads them from where presetsvc's run store (and its prompt logs) live.
		orch.LogBucket = bucket
		orch.Reporter = usageReporter
		return orch, nil
	}

	// Initialize Server
	server := api.NewServer(store, memoryStore, storeClient, smFetcher, orchMaker, cfg)

	// MCP front end on the same mux. It shares the UI's per-IP rate limiter and
	// the same preset service (and so the same concurrency cap). Without a usable
	// MCP_TOKEN its routes reject everything; the UI is unaffected.
	//
	// Quad Cortex device tools (docs/design/device-job-contract.md) are added
	// only when DEVICE_JOBS_BUCKET and DEVICE_JOB_KEY are set. A bad device
	// config disables those tools and is logged; it never stops the app.
	//
	// The device export (plan P2) is pure and needs no bridge: get_preset
	// (format=device) and the UI's download link use it even when the device
	// tools are off. One exporter serves all three, so a dry run, a push and a
	// download describe the same operations.
	var mcpOpts []mcpserver.Option
	var exporter devicejobs.Exporter = devicejobs.UnavailableExporter{}
	if exp, err := server.DeviceExporter(); err != nil {
		log.Printf("Device export unavailable: %v", err)
	} else {
		exporter = exp
		mcpOpts = append(mcpOpts, mcpserver.WithExporter(exp))
	}
	switch dcfg, err := devicejobs.ConfigFromEnv(bucket); {
	case err != nil:
		log.Printf("Device tools disabled: %v", err)
	case dcfg != nil:
		bridge := devicejobs.NewBridge(storeClient, dcfg, exporter)
		mcpOpts = append(mcpOpts, mcpserver.WithDevice(bridge))
		log.Println("Device tools enabled (jobs bucket configured)")
	}
	mcpserver.Mount(server.Mux(), server.Service(), mcpserver.ConfigFromEnv(), server.RateLimit(), mcpOpts...)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("QC-2 Backend listening at: http://localhost:%s\n", port)
	// SIGTERM (Fly stop/deploy) or Ctrl-C triggers a graceful stop; see serve.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, os.Interrupt)
	defer stop()
	srv := &http.Server{Addr: ":" + port, Handler: server.Handler()}
	if err := serve(ctx, srv, server.Service()); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
