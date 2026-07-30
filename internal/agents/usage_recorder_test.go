package agents

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sync"
	"testing"
	"time"

	"google.golang.org/genai"
)

// fakeStorageClient is a minimal in-memory storage.Client for these tests —
// this repo has no existing in-memory test double (unlike the sibling
// job_tracker project's storage.NewMemoryClient()), so this is scoped
// narrowly to what recordUsage/recordAttemptUsage actually exercise.
type fakeStorageClient struct {
	mu       sync.Mutex
	objects  map[string][]byte
	writeErr error
}

func newFakeStorageClient() *fakeStorageClient {
	return &fakeStorageClient{objects: make(map[string][]byte)}
}

func (f *fakeStorageClient) ReadFile(_ context.Context, bucket, object string) ([]byte, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	data, ok := f.objects[bucket+"/"+object]
	if !ok {
		return nil, fmt.Errorf("not found: %s/%s", bucket, object)
	}
	return data, nil
}

func (f *fakeStorageClient) WriteFile(_ context.Context, bucket, object string, data []byte) error {
	if f.writeErr != nil {
		return f.writeErr
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.objects[bucket+"/"+object] = data
	return nil
}

func (f *fakeStorageClient) ListFiles(_ context.Context, bucket, prefix string) ([]string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var keys []string
	full := bucket + "/" + prefix
	for k := range f.objects {
		if len(k) >= len(full) && k[:len(full)] == full {
			keys = append(keys, k[len(bucket)+1:])
		}
	}
	return keys, nil
}

func (f *fakeStorageClient) DeleteFile(_ context.Context, bucket, object string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.objects, bucket+"/"+object)
	return nil
}

func (f *fakeStorageClient) Close() {}

func (f *fakeStorageClient) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.objects)
}

func (f *fakeStorageClient) firstRecord(t *testing.T) UsageRecord {
	t.Helper()
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, data := range f.objects {
		var rec UsageRecord
		if err := json.Unmarshal(data, &rec); err != nil {
			t.Fatalf("decoding record: %v", err)
		}
		return rec
	}
	t.Fatal("no records written")
	return UsageRecord{}
}

func TestComputeCostUSD_KnownModel(t *testing.T) {
	cost := computeCostUSD("gemini-3.1-pro-preview", 1_000_000, 1_000_000, 0)
	want := 2.00 + 12.00
	if cost != want {
		t.Errorf("computeCostUSD = %v, want %v", cost, want)
	}
}

func TestComputeCostUSD_ThinkingTokensBilledAtOutputRate(t *testing.T) {
	withThinking := computeCostUSD("gemini-3.1-pro-preview", 0, 0, 500_000)
	withOutput := computeCostUSD("gemini-3.1-pro-preview", 0, 500_000, 0)
	if withThinking != withOutput {
		t.Errorf("thinking tokens should bill at the output rate: got %v vs %v", withThinking, withOutput)
	}
}

func TestComputeCostUSD_UnknownModel_ReturnsZero(t *testing.T) {
	if got := computeCostUSD("some-future-model", 1000, 1000, 0); got != 0 {
		t.Errorf("computeCostUSD for an unknown model = %v, want 0", got)
	}
}

func TestClassifyError(t *testing.T) {
	cases := []struct {
		err  error
		want string
	}{
		{errors.New("429 Too Many Requests"), "rate_limit"},
		{errors.New("RESOURCE_EXHAUSTED: quota exceeded"), "rate_limit"},
		{errors.New("401 Unauthorized"), "auth"},
		{errors.New("PERMISSION_DENIED"), "auth"},
		{errors.New("context deadline exceeded"), "timeout"},
		{errors.New("504 Gateway Timeout"), "timeout"},
		{errors.New("503 Service Unavailable"), "unavailable"},
		{errors.New("something completely unrecognized"), "api_error"},
	}
	for _, c := range cases {
		if got := classifyError(c.err); got != c.want {
			t.Errorf("classifyError(%q) = %q, want %q", c.err, got, c.want)
		}
	}
}

func TestClassifyError_Nil(t *testing.T) {
	if got := classifyError(nil); got != "" {
		t.Errorf("classifyError(nil) = %q, want empty", got)
	}
}

func TestUsageObjectKey_Format(t *testing.T) {
	key := usageObjectKey(time.Now())
	matched, err := regexp.MatchString(`^usage/\d{4}-\d{2}-\d{2}/\d{9}-[0-9a-f]{8}\.json$`, key)
	if err != nil {
		t.Fatalf("regexp: %v", err)
	}
	if !matched {
		t.Errorf("usageObjectKey = %q, doesn't match expected format", key)
	}
}

func TestRecordUsage_NilStorage_NoPanic(t *testing.T) {
	recordUsage(context.Background(), nil, "bucket", UsageRecord{CallType: "Test"})
}

func TestRecordUsage_EmptyBucket_NoOp(t *testing.T) {
	fake := newFakeStorageClient()
	recordUsage(context.Background(), fake, "", UsageRecord{CallType: "Test"})
	if fake.count() != 0 {
		t.Errorf("expected no write when bucket is empty, got %d objects", fake.count())
	}
}

func TestRecordUsage_WritesRecord(t *testing.T) {
	fake := newFakeStorageClient()
	recordUsage(context.Background(), fake, "test-bucket", UsageRecord{
		Provider: "gemini", CallType: "Tone Historian", Model: "gemini-3.1-pro-preview",
		InputTokens: 100, OutputTokens: 20, LatencyMS: 500, Success: true,
	})
	if fake.count() != 1 {
		t.Fatalf("expected 1 object, got %d", fake.count())
	}
	rec := fake.firstRecord(t)
	if rec.CallType != "Tone Historian" || rec.Provider != "gemini" || rec.InputTokens != 100 {
		t.Errorf("unexpected record: %+v", rec)
	}
	if rec.Timestamp == "" {
		t.Error("expected Timestamp to be set by recordUsage")
	}
}

func TestRecordUsage_WriteFailure_DoesNotPanic(t *testing.T) {
	fake := newFakeStorageClient()
	fake.writeErr = errors.New("network down")
	recordUsage(context.Background(), fake, "test-bucket", UsageRecord{CallType: "Test", Success: true})
	if fake.count() != 0 {
		t.Errorf("expected no object written on failure, got %d", fake.count())
	}
}

func TestRecordAttemptUsage_Success(t *testing.T) {
	fake := newFakeStorageClient()
	o := &Orchestrator{gcs: fake, UsageBucket: "test-bucket"}
	resp := &genai.GenerateContentResponse{
		UsageMetadata: &genai.GenerateContentResponseUsageMetadata{
			PromptTokenCount: 100, CandidatesTokenCount: 20, ThoughtsTokenCount: 5,
		},
	}
	recordAttemptUsage(context.Background(), o, "Tone Historian", "gemini-3.1-pro-preview", 1234, resp, nil)

	rec := fake.firstRecord(t)
	if !rec.Success || rec.InputTokens != 100 || rec.OutputTokens != 20 || rec.ThinkingTokens != 5 {
		t.Errorf("unexpected record: %+v", rec)
	}
	if rec.CostUSD <= 0 {
		t.Errorf("expected a positive cost for a priced model, got %v", rec.CostUSD)
	}
}

func TestRecordAttemptUsage_Failure(t *testing.T) {
	fake := newFakeStorageClient()
	o := &Orchestrator{gcs: fake, UsageBucket: "test-bucket"}
	recordAttemptUsage(context.Background(), o, "Tone Historian", "gemini-3.1-pro-preview", 500, nil, errors.New("429 rate limited"))

	rec := fake.firstRecord(t)
	if rec.Success {
		t.Error("expected Success=false for a failed attempt")
	}
	if rec.ErrorKind != "rate_limit" {
		t.Errorf("ErrorKind = %q, want rate_limit", rec.ErrorKind)
	}
	if rec.InputTokens != 0 || rec.CostUSD != 0 {
		t.Errorf("expected zero tokens/cost on failure, got %+v", rec)
	}
}
