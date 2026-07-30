package agents

import (
	"context"
	"testing"
)

func TestAggregateUsage_Empty(t *testing.T) {
	r := AggregateUsage("2026-07-29", nil)
	if r.TotalCalls != 0 || r.AvgLatencyMS != 0 {
		t.Errorf("expected a zero-value rollup, got %+v", r)
	}
}

func TestAggregateUsage_MixedSuccessAndFailure(t *testing.T) {
	records := []UsageRecord{
		{Provider: "gemini", CallType: "Tone Historian", Model: "gemini-3.1-pro-preview", InputTokens: 100, OutputTokens: 20, CostUSD: 0.01, LatencyMS: 1000, Success: true},
		{Provider: "gemini", CallType: "Tone Historian", Model: "gemini-3.1-pro-preview", InputTokens: 200, OutputTokens: 40, CostUSD: 0.02, LatencyMS: 3000, Success: true},
		{Provider: "gemini", CallType: "Tone Historian", Model: "gemini-3.1-pro-preview", LatencyMS: 500, Success: false, ErrorKind: "rate_limit"},
		{Provider: "openllm", CallType: "Sonic Profiler", Model: "active-model", InputTokens: 50, OutputTokens: 10, CostUSD: 0, LatencyMS: 500, Success: true},
	}

	r := AggregateUsage("2026-07-29", records)

	if r.TotalCalls != 4 {
		t.Errorf("TotalCalls = %d, want 4", r.TotalCalls)
	}
	if r.SuccessCount != 3 || r.FailureCount != 1 {
		t.Errorf("SuccessCount/FailureCount = %d/%d, want 3/1", r.SuccessCount, r.FailureCount)
	}
	if r.TotalInputTokens != 350 || r.TotalOutputTokens != 70 {
		t.Errorf("token totals = %d/%d, want 350/70", r.TotalInputTokens, r.TotalOutputTokens)
	}
	if r.ByErrorKind["rate_limit"] != 1 {
		t.Errorf("ByErrorKind[rate_limit] = %d, want 1", r.ByErrorKind["rate_limit"])
	}
	toneHistorian, ok := r.ByCallType["Tone Historian"]
	if !ok || toneHistorian.Calls != 3 || toneHistorian.SuccessCount != 2 || toneHistorian.FailureCount != 1 {
		t.Errorf("Tone Historian bucket = %+v (ok=%v), want Calls=3 SuccessCount=2 FailureCount=1", toneHistorian, ok)
	}
	openllmBucket, ok := r.ByProvider["openllm"]
	if !ok || openllmBucket.Calls != 1 {
		t.Errorf("expected an openllm provider bucket with 1 call, got %+v (ok=%v)", openllmBucket, ok)
	}
	wantAvg := float64(1000+3000+500+500) / 4
	if r.AvgLatencyMS != wantAvg {
		t.Errorf("AvgLatencyMS = %v, want %v", r.AvgLatencyMS, wantAvg)
	}
}

func TestListUsageRecords_AggregateRoundTrip(t *testing.T) {
	fake := newFakeStorageClient()
	ctx := context.Background()

	recordUsage(ctx, fake, "test-bucket", UsageRecord{CallType: "Tone Historian", Provider: "gemini", Model: "x", Success: true})
	recordUsage(ctx, fake, "test-bucket", UsageRecord{CallType: "Tone Historian", Provider: "gemini", Model: "x", Success: false, ErrorKind: "timeout"})

	// Both records land under today's date, whatever that is when the test
	// runs, so discover it via ListFiles under the usage/ root instead of
	// hardcoding a date.
	keys, err := fake.ListFiles(ctx, "test-bucket", "usage/")
	if err != nil {
		t.Fatalf("ListFiles: %v", err)
	}
	if len(keys) != 2 {
		t.Fatalf("expected 2 objects, got %d: %v", len(keys), keys)
	}
	date := keys[0][len("usage/") : len("usage/")+10]

	records, err := ListUsageRecords(ctx, fake, "test-bucket", date)
	if err != nil {
		t.Fatalf("ListUsageRecords: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("expected 2 records, got %d", len(records))
	}

	rollup := AggregateUsage(date, records)
	if rollup.TotalCalls != 2 || rollup.SuccessCount != 1 || rollup.FailureCount != 1 {
		t.Errorf("rollup = %+v, want TotalCalls=2 SuccessCount=1 FailureCount=1", rollup)
	}
}

func TestListUsageRecords_NoObjects_ReturnsEmptyNotError(t *testing.T) {
	fake := newFakeStorageClient()
	records, err := ListUsageRecords(context.Background(), fake, "test-bucket", "2026-01-01")
	if err != nil {
		t.Fatalf("ListUsageRecords: %v", err)
	}
	if len(records) != 0 {
		t.Errorf("expected no records, got %d", len(records))
	}
}

func TestWriteUsageRollupThenReadUsageRollup_RoundTrips(t *testing.T) {
	fake := newFakeStorageClient()
	ctx := context.Background()
	rollup := AggregateUsage("2026-07-29", []UsageRecord{
		{CallType: "Tone Historian", Provider: "gemini", Model: "x", InputTokens: 10, Success: true},
	})

	if err := WriteUsageRollup(ctx, fake, "test-bucket", rollup); err != nil {
		t.Fatalf("WriteUsageRollup: %v", err)
	}
	got, err := ReadUsageRollup(ctx, fake, "test-bucket", "2026-07-29")
	if err != nil {
		t.Fatalf("ReadUsageRollup: %v", err)
	}
	if got.TotalCalls != 1 || got.TotalInputTokens != 10 {
		t.Errorf("got %+v, want TotalCalls=1 TotalInputTokens=10", got)
	}
}

func TestWriteUsageRollup_Overwrites(t *testing.T) {
	fake := newFakeStorageClient()
	ctx := context.Background()
	first := AggregateUsage("2026-07-29", []UsageRecord{{CallType: "x", Success: true}})
	if err := WriteUsageRollup(ctx, fake, "test-bucket", first); err != nil {
		t.Fatalf("WriteUsageRollup (first): %v", err)
	}

	second := AggregateUsage("2026-07-29", []UsageRecord{
		{CallType: "x", Success: true},
		{CallType: "x", Success: true},
	})
	if err := WriteUsageRollup(ctx, fake, "test-bucket", second); err != nil {
		t.Fatalf("WriteUsageRollup (second): %v", err)
	}

	got, err := ReadUsageRollup(ctx, fake, "test-bucket", "2026-07-29")
	if err != nil {
		t.Fatalf("ReadUsageRollup: %v", err)
	}
	if got.TotalCalls != 2 {
		t.Errorf("TotalCalls = %d, want 2 (rollup should be overwritten, not accumulated)", got.TotalCalls)
	}
}

func TestReadUsageRollup_NotFound(t *testing.T) {
	fake := newFakeStorageClient()
	_, err := ReadUsageRollup(context.Background(), fake, "test-bucket", "2026-01-01")
	if err == nil {
		t.Fatal("expected an error for a missing rollup")
	}
}
