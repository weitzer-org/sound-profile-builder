package agents

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/weitzer-org/sound-builder/internal/storage"
)

// UsageBucketStats is one slice of a UsageRollup's breakdown (by call type,
// model, or provider) — the same shape repeated three ways rather than
// three bespoke structs, since the aggregation is identical. Mirrors
// internal/usage.Bucket in the sibling job_tracker project.
type UsageBucketStats struct {
	Calls        int     `json:"calls"`
	SuccessCount int     `json:"successCount"`
	FailureCount int     `json:"failureCount"`
	InputTokens  int32   `json:"inputTokens"`
	OutputTokens int32   `json:"outputTokens"`
	CostUSD      float64 `json:"costUsd"`
}

// UsageRollup is a day's aggregated usage — the persisted counterpart of
// cmd/usage_report's on-screen summary. Mirrors internal/usage.Rollup in the
// sibling job_tracker project.
type UsageRollup struct {
	Date         string `json:"date"`
	TotalCalls   int    `json:"totalCalls"`
	SuccessCount int    `json:"successCount"`
	FailureCount int    `json:"failureCount"`

	TotalInputTokens    int32 `json:"totalInputTokens"`
	TotalOutputTokens   int32 `json:"totalOutputTokens"`
	TotalThinkingTokens int32 `json:"totalThinkingTokens"`

	TotalCostUSD float64 `json:"totalCostUsd"`
	AvgLatencyMS float64 `json:"avgLatencyMs"`

	ByCallType  map[string]*UsageBucketStats `json:"byCallType,omitempty"`
	ByModel     map[string]*UsageBucketStats `json:"byModel,omitempty"`
	ByProvider  map[string]*UsageBucketStats `json:"byProvider,omitempty"`
	ByErrorKind map[string]int               `json:"byErrorKind,omitempty"`
}

// ListUsageRecords reads every UsageRecord written under usage/<date>/
// (date is "YYYY-MM-DD", matching usageObjectKey's directory format). A
// record that fails to fetch or decode is skipped with a logged warning
// rather than failing the whole listing — one malformed object shouldn't
// block aggregating everything else that day.
func ListUsageRecords(ctx context.Context, gcs storage.Client, bucket, date string) ([]UsageRecord, error) {
	prefix := "usage/" + date + "/"
	keys, err := gcs.ListFiles(ctx, bucket, prefix)
	if err != nil {
		return nil, fmt.Errorf("usage: listing %s: %w", prefix, err)
	}

	records := make([]UsageRecord, 0, len(keys))
	for _, key := range keys {
		data, err := gcs.ReadFile(ctx, bucket, key)
		if err != nil {
			log.Printf("usage: reading %s: %v (skipped)", key, err)
			continue
		}
		var rec UsageRecord
		if err := json.Unmarshal(data, &rec); err != nil {
			log.Printf("usage: decoding %s: %v (skipped)", key, err)
			continue
		}
		records = append(records, rec)
	}
	return records, nil
}

func addToUsageBucket(m map[string]*UsageBucketStats, key string, rec UsageRecord) {
	if key == "" {
		return
	}
	b, ok := m[key]
	if !ok {
		b = &UsageBucketStats{}
		m[key] = b
	}
	b.Calls++
	if rec.Success {
		b.SuccessCount++
	} else {
		b.FailureCount++
	}
	b.InputTokens += rec.InputTokens
	b.OutputTokens += rec.OutputTokens
	b.CostUSD += rec.CostUSD
}

// AggregateUsage summarizes records into a UsageRollup for date. Pure/
// deterministic — no I/O — so it's independently testable from
// ListUsageRecords/WriteUsageRollup.
func AggregateUsage(date string, records []UsageRecord) UsageRollup {
	r := UsageRollup{
		Date:        date,
		ByCallType:  map[string]*UsageBucketStats{},
		ByModel:     map[string]*UsageBucketStats{},
		ByProvider:  map[string]*UsageBucketStats{},
		ByErrorKind: map[string]int{},
	}

	var totalLatencyMs int64
	for _, rec := range records {
		r.TotalCalls++
		if rec.Success {
			r.SuccessCount++
		} else {
			r.FailureCount++
			if rec.ErrorKind != "" {
				r.ByErrorKind[rec.ErrorKind]++
			}
		}
		r.TotalInputTokens += rec.InputTokens
		r.TotalOutputTokens += rec.OutputTokens
		r.TotalThinkingTokens += rec.ThinkingTokens
		r.TotalCostUSD += rec.CostUSD
		totalLatencyMs += rec.LatencyMS

		addToUsageBucket(r.ByCallType, rec.CallType, rec)
		addToUsageBucket(r.ByModel, rec.Model, rec)
		addToUsageBucket(r.ByProvider, rec.Provider, rec)
	}

	if r.TotalCalls > 0 {
		r.AvgLatencyMS = float64(totalLatencyMs) / float64(r.TotalCalls)
	}
	return r
}

// rollupObjectKey is the fixed (non-random) key a given date's rollup is
// written to — unlike per-call records, a rollup is meant to be recomputed
// and overwritten idempotently, not accumulated.
func rollupObjectKey(date string) string {
	return "usage/rollups/" + date + ".json"
}

// WriteUsageRollup persists rollup, overwriting any existing rollup for the
// same date.
func WriteUsageRollup(ctx context.Context, gcs storage.Client, bucket string, rollup UsageRollup) error {
	data, err := json.Marshal(rollup)
	if err != nil {
		return fmt.Errorf("usage: marshaling rollup for %s: %w", rollup.Date, err)
	}
	if err := gcs.WriteFile(ctx, bucket, rollupObjectKey(rollup.Date), data); err != nil {
		return fmt.Errorf("usage: writing rollup for %s: %w", rollup.Date, err)
	}
	return nil
}

// ReadUsageRollup reads back a previously written rollup for date.
func ReadUsageRollup(ctx context.Context, gcs storage.Client, bucket, date string) (UsageRollup, error) {
	data, err := gcs.ReadFile(ctx, bucket, rollupObjectKey(date))
	if err != nil {
		return UsageRollup{}, fmt.Errorf("usage: reading rollup for %s: %w", date, err)
	}
	var rollup UsageRollup
	if err := json.Unmarshal(data, &rollup); err != nil {
		return UsageRollup{}, fmt.Errorf("usage: decoding rollup for %s: %w", date, err)
	}
	return rollup, nil
}
