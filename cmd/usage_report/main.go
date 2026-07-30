// Command usage_report summarizes the LLM call records
// internal/agents.recordUsage writes to R2/GCS/MinIO under usage/<date>/ —
// one small object per completed Orchestrator.RunAgentSplit attempt (see
// internal/agents/usage_recorder.go and usage_analytics_reference.md for
// the full schema/query recipes).
//
// This is a manual, on-demand tool, not a scheduled job. Run it whenever you
// want a spend/latency/error-rate snapshot:
//
//	go run ./cmd/usage_report -date 2026-07-29
//	go run ./cmd/usage_report -from 2026-07-23 -to 2026-07-29
//	go run ./cmd/usage_report                        # defaults to today (UTC)
//	go run ./cmd/usage_report -date 2026-07-29 -write-rollup   # also persist
//
// -write-rollup additionally persists each date's computed summary to
// usage/rollups/<date>.json (internal/agents.WriteUsageRollup). Report-only
// (writes nothing) by default.
//
// Storage backend selection mirrors cmd/server/main.go exactly (STORAGE_BACKEND,
// config.json's bucket_name, S3_BUCKET/GCS_BUCKET overrides) so this reads
// back from whichever backend the server actually wrote to.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"sort"
	"time"

	"github.com/weitzer-org/sound-builder/internal/agents"
	"github.com/weitzer-org/sound-builder/internal/config"
	"github.com/weitzer-org/sound-builder/internal/storage"
)

const dateLayout = "2006-01-02"

func main() {
	bucketFlag := flag.String("bucket", "", "bucket containing usage/ records (defaults to config.json's bucket_name / S3_BUCKET / GCS_BUCKET, same resolution as cmd/server)")
	configPath := flag.String("config", "config.json", "path to config.json (for the default bucket name)")
	date := flag.String("date", "", "single date to report on, YYYY-MM-DD (defaults to today, UTC; mutually exclusive with -from/-to)")
	from := flag.String("from", "", "start date of a range, YYYY-MM-DD (use with -to instead of -date)")
	to := flag.String("to", "", "end date of a range, YYYY-MM-DD, inclusive (use with -from)")
	writeRollup := flag.Bool("write-rollup", false, "persist each date's computed rollup to usage/rollups/<date>.json (default: report only, writes nothing)")
	flag.Parse()

	ctx := context.Background()

	backend := os.Getenv("STORAGE_BACKEND")
	if backend == "" {
		backend = "gcs"
	}

	bucket := *bucketFlag
	if bucket == "" {
		cfg, err := config.LoadConfig(*configPath)
		if err != nil {
			log.Printf("usage_report: reading %s: %v (continuing with an empty default bucket name)", *configPath, err)
			cfg = &config.AppConfig{}
		}
		bucket = cfg.BucketName
		if backend == "s3" {
			if b := os.Getenv("S3_BUCKET"); b != "" {
				bucket = b
			}
		}
	}
	if bucket == "" {
		log.Fatal("usage_report: -bucket (or config.json's bucket_name / S3_BUCKET / GCS_BUCKET) is required")
	}

	var storeClient storage.Client
	var err error
	switch backend {
	case "s3":
		storeClient, err = storage.NewS3Client(ctx)
	case "gcs":
		storeClient, err = storage.NewGCSClient(ctx)
	default:
		log.Fatalf("usage_report: unknown STORAGE_BACKEND %q (expected \"gcs\" or \"s3\")", backend)
	}
	if err != nil {
		log.Fatalf("usage_report: connecting to storage (%s): %v", backend, err)
	}
	defer storeClient.Close()

	dates, err := resolveDates(*date, *from, *to)
	if err != nil {
		log.Fatalf("usage_report: %v", err)
	}

	var combined []agents.UsageRecord
	for _, d := range dates {
		records, err := agents.ListUsageRecords(ctx, storeClient, bucket, d)
		if err != nil {
			log.Fatalf("usage_report: listing %s: %v", d, err)
		}
		rollup := agents.AggregateUsage(d, records)
		printRollup(rollup)
		combined = append(combined, records...)

		if *writeRollup {
			if err := agents.WriteUsageRollup(ctx, storeClient, bucket, rollup); err != nil {
				log.Fatalf("usage_report: writing rollup for %s: %v", d, err)
			}
			fmt.Printf("(rollup written to usage/rollups/%s.json)\n\n", d)
		}
	}

	if len(dates) > 1 {
		fmt.Println("=== Combined ===")
		printRollup(agents.AggregateUsage(dates[0]+".."+dates[len(dates)-1], combined))
	}
}

// resolveDates turns the CLI's mutually-exclusive -date / -from+-to flags
// into an explicit list of "YYYY-MM-DD" strings to look up, one per day in
// the requested range.
func resolveDates(date, from, to string) ([]string, error) {
	switch {
	case date != "" && (from != "" || to != ""):
		return nil, fmt.Errorf("-date is mutually exclusive with -from/-to")
	case date != "":
		if _, err := time.Parse(dateLayout, date); err != nil {
			return nil, fmt.Errorf("invalid -date %q: %w", date, err)
		}
		return []string{date}, nil
	case from != "" && to != "":
		start, err := time.Parse(dateLayout, from)
		if err != nil {
			return nil, fmt.Errorf("invalid -from %q: %w", from, err)
		}
		end, err := time.Parse(dateLayout, to)
		if err != nil {
			return nil, fmt.Errorf("invalid -to %q: %w", to, err)
		}
		if end.Before(start) {
			return nil, fmt.Errorf("-to (%s) is before -from (%s)", to, from)
		}
		var dates []string
		for d := start; !d.After(end); d = d.AddDate(0, 0, 1) {
			dates = append(dates, d.Format(dateLayout))
		}
		return dates, nil
	case from != "" || to != "":
		return nil, fmt.Errorf("-from and -to must be used together")
	default:
		return []string{time.Now().UTC().Format(dateLayout)}, nil
	}
}

func printRollup(r agents.UsageRollup) {
	fmt.Printf("=== %s ===\n", r.Date)
	fmt.Printf("Calls: %d (success=%d, failure=%d)\n", r.TotalCalls, r.SuccessCount, r.FailureCount)
	fmt.Printf("Tokens: input=%d output=%d thinking=%d\n", r.TotalInputTokens, r.TotalOutputTokens, r.TotalThinkingTokens)
	fmt.Printf("Cost: $%.4f\n", r.TotalCostUSD)
	fmt.Printf("Avg latency: %.0fms\n", r.AvgLatencyMS)

	if len(r.ByCallType) > 0 {
		fmt.Println("By call type (agent):")
		printBuckets(r.ByCallType)
	}
	if len(r.ByModel) > 0 {
		fmt.Println("By model:")
		printBuckets(r.ByModel)
	}
	if len(r.ByProvider) > 0 {
		fmt.Println("By provider:")
		printBuckets(r.ByProvider)
	}
	if len(r.ByErrorKind) > 0 {
		fmt.Println("By error kind:")
		for _, k := range sortedKeys(r.ByErrorKind) {
			fmt.Printf("  %-20s %d\n", k, r.ByErrorKind[k])
		}
	}
	fmt.Println()
}

func printBuckets(m map[string]*agents.UsageBucketStats) {
	for _, k := range sortedKeys(m) {
		b := m[k]
		fmt.Printf("  %-20s calls=%-5d success=%-5d failure=%-5d input=%-8d output=%-8d cost=$%.4f\n",
			k, b.Calls, b.SuccessCount, b.FailureCount, b.InputTokens, b.OutputTokens, b.CostUSD)
	}
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
