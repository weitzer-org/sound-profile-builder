// e2e_seed loads the synthetic presets in testdata/e2e_presets into the app's preset store,
// for the Playwright specs that need saved presets to look at (tests/e2e/data_grid.spec.js).
// Each fixture is raw Architect-style output; the seeder runs the same post-generation
// checks presetsvc's generate path runs, in the same order, so the saved preset carries the
// notes a real generation would. It uses the S3 settings the server uses (STORAGE_BACKEND=s3
// is assumed; scripts/e2e-native.sh points it at its local moto) and makes no Gemini calls.
//
//	go run ./cmd/e2e_seed [-config config.json] [testdata/e2e_presets]
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"

	"github.com/weitzer-org/sound-builder/internal/agents"
	"github.com/weitzer-org/sound-builder/internal/config"
	"github.com/weitzer-org/sound-builder/internal/storage"
)

type fixture struct {
	Name             string `json:"name"`
	BuilderStatement string `json:"builder_statement"`
	Payload          struct {
		Structured storage.StructuredPreset `json:"structured"`
		LegacyHTML map[string]string        `json:"legacy_html,omitempty"`
	} `json:"payload"`
}

func main() {
	configPath := flag.String("config", "config.json", "app config (capture and plugin settings for the checks)")
	flag.Parse()
	dir := "testdata/e2e_presets"
	if flag.NArg() > 0 {
		dir = flag.Arg(0)
	}
	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		log.Fatalf("load %s: %v", *configPath, err)
	}
	bucket := os.Getenv("S3_BUCKET")
	if bucket == "" {
		log.Fatal("S3_BUCKET must be set (the server's preset bucket)")
	}
	ctx := context.Background()
	client, err := storage.NewS3Client(ctx)
	if err != nil {
		log.Fatal(err)
	}
	store := storage.NewPresetStore(client, bucket)
	valid := agents.BuildEffectiveValidBlocksFor(agents.ValidBlockFilter{
		AllowFactoryCaptures: cfg.AllowFactoryCaptures,
		AllowUserCaptures:    cfg.AllowUserCaptures,
		Plugins:              agents.NewPluginGate(cfg.AllowPaidPlugins, cfg.AvailablePlugins),
	})

	files, err := filepath.Glob(filepath.Join(dir, "*.json"))
	if err != nil || len(files) == 0 {
		log.Fatalf("no fixtures in %s: %v", dir, err)
	}
	sort.Strings(files)
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			log.Fatal(err)
		}
		var fx fixture
		if err := json.Unmarshal(b, &fx); err != nil {
			log.Fatalf("%s: %v", f, err)
		}
		sp := &fx.Payload.Structured
		legacy := fx.Payload.LegacyHTML
		if len(legacy) == 0 {
			// Like a real generation: the stored table is rendered from the Architect's
			// output before the checks run (RunPipeline renders it; presetsvc checks after).
			legacy = agents.RenderAllGuitarsHTML(sp.Guitars)
		}
		// Same checks, same order, as presetsvc's generate path.
		agents.FlagCaptureFormattingMismatches(sp, valid)
		agents.FlagIncompleteCabinetBlocks(sp)
		agents.FlagLeftoverValueRanges(sp)
		agents.FlagUnverifiedStructuredBlocks(sp, valid)
		counts := agents.FlagCatalogParamMismatches(sp, valid)

		// The {structured, legacy_html} envelope generation writes; the app only reads it as
		// an envelope when legacy_html is present.
		payload, err := json.Marshal(map[string]any{"structured": sp, "legacy_html": legacy})
		if err != nil {
			log.Fatal(err)
		}
		p := &storage.Preset{Name: fx.Name, BuilderStatement: fx.BuilderStatement, Payload: string(payload)}
		if err := store.Save(ctx, p); err != nil {
			log.Fatalf("save %s: %v", fx.Name, err)
		}
		fmt.Printf("seeded %q (%s): %d device-check findings\n", fx.Name, p.ID, counts.Defects())
	}
}
