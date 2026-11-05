package agents

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
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
	cost := computeCostUSD("gemini-3.1-pro-preview", 1_000_000, 0, 1_000_000, 0)
	want := 2.00 + 12.00
	if cost != want {
		t.Errorf("computeCostUSD = %v, want %v", cost, want)
	}
}

func TestComputeCostUSD_ThinkingTokensBilledAtOutputRate(t *testing.T) {
	withThinking := computeCostUSD("gemini-3.1-pro-preview", 0, 0, 0, 500_000)
	withOutput := computeCostUSD("gemini-3.1-pro-preview", 0, 0, 500_000, 0)
	if withThinking != withOutput {
		t.Errorf("thinking tokens should bill at the output rate: got %v vs %v", withThinking, withOutput)
	}
}

func TestComputeCostUSD_CachedTokens(t *testing.T) {
	const m = "gemini-3.6-flash"
	std := day(2027, time.February, 1) // standard period: input 1.50, cachedInput 0.15, output 7.50
	const eps = 1e-9
	cases := []struct {
		name                          string
		input, cached, output, thinks int32
		want                          float64
	}{
		{"cached=0 bills all at input rate", 1_000_000, 0, 0, 0, 1.50},
		{"partial cache", 1_000_000, 600_000, 0, 0, 0.4*1.50 + 0.6*0.15},
		{"fully cached", 1_000_000, 1_000_000, 0, 0, 0.15},
		{"cached > input is clamped to input", 1_000_000, 5_000_000, 0, 0, 0.15},
		{"negative cached is clamped to 0", 1_000_000, -5, 0, 0, 1.50},
		{"output and thinking unaffected by cache", 1_000_000, 500_000, 100_000, 100_000, 0.5*1.50 + 0.5*0.15 + 0.2*7.50},
	}
	for _, c := range cases {
		got := computeCostUSDAt(m, std, c.input, c.cached, c.output, c.thinks)
		if d := got - c.want; d > eps || d < -eps {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
	// Introductory period: input 0.75, cachedInput 0.075.
	if got := computeCostUSDAt(m, day(2026, time.November, 5), 1_000_000, 1_000_000, 0, 0); got-0.075 > eps || 0.075-got > eps {
		t.Errorf("intro-period fully cached: got %v, want 0.075", got)
	}
}

// sourcedCachedRates are the models whose cached-input rate comes from the Google Cloud pricing
// page (see the CACHED-INPUT RATES comment on geminiPriceSchedule). Every other model must keep
// cachedInput == input so caching can never understate its cost.
var sourcedCachedRates = map[string]bool{
	"gemini-4-argon": true, "gemini-3.8-flash": true, "gemini-3.6-flash": true, "gemini-3.1-pro-preview": true,
}

func TestComputeCostUSD_UnverifiedCachedRateNeverDiscounts(t *testing.T) {
	for model, periods := range geminiPriceSchedule {
		if sourcedCachedRates[model] {
			continue
		}
		for _, p := range periods {
			if p.cachedInput != p.input {
				t.Errorf("%s: cachedInput %v != input %v; only set a discount once the real rate is sourced", model, p.cachedInput, p.input)
			}
		}
	}
}

// Pins the cited cached-input figures (Google Cloud pricing page, fetched 2026-11-05, global endpoint).
func TestCachedInputRatesMatchSource(t *testing.T) {
	cases := []struct {
		model  string
		at     time.Time
		cached float64
	}{
		{"gemini-4-argon", day(2026, time.November, 30), 0.10},
		{"gemini-4-argon", day(2026, time.December, 1), 0.20},
		{"gemini-3.8-flash", day(2026, time.December, 31), 0.075},
		{"gemini-3.8-flash", day(2027, time.January, 1), 0.15},
		{"gemini-3.6-flash", day(2026, time.December, 31), 0.075},
		{"gemini-3.6-flash", day(2027, time.January, 1), 0.15},
		{"gemini-3.1-pro-preview", day(2026, time.November, 5), 0.20},
	}
	for _, c := range cases {
		p, ok := priceAt(c.model, c.at)
		if !ok || p.cachedInput != c.cached {
			t.Errorf("%s at %s: cachedInput = %v (ok=%v), want %v", c.model, c.at.Format("2006-01-02"), p.cachedInput, ok, c.cached)
		}
	}
	// A fully cached 1M-token Argon prompt during the promo costs $0.10, not $2.00.
	if got := computeCostUSDAt("gemini-4-argon", day(2026, time.November, 5), 1_000_000, 1_000_000, 0, 0); got < 0.0999 || got > 0.1001 {
		t.Errorf("fully cached Argon prompt = %v, want 0.10", got)
	}
}

func TestComputeCostUSD_UnknownModel_ReturnsZero(t *testing.T) {
	if got := computeCostUSD("some-future-model", 1000, 500, 1000, 0); got != 0 {
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
	recordUsage(context.Background(), nil, "bucket", nil, UsageRecord{CallType: "Test"})
}

func TestRecordUsage_EmptyBucket_NoOp(t *testing.T) {
	fake := newFakeStorageClient()
	recordUsage(context.Background(), fake, "", nil, UsageRecord{CallType: "Test"})
	if fake.count() != 0 {
		t.Errorf("expected no write when bucket is empty, got %d objects", fake.count())
	}
}

func TestRecordUsage_WritesRecord(t *testing.T) {
	fake := newFakeStorageClient()
	recordUsage(context.Background(), fake, "test-bucket", nil, UsageRecord{
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
	recordUsage(context.Background(), fake, "test-bucket", nil, UsageRecord{CallType: "Test", Success: true})
	if fake.count() != 0 {
		t.Errorf("expected no object written on failure, got %d", fake.count())
	}
}

func TestRecordAttemptUsage_ReturnsImmediatelyEvenWithASlowReporterAttached(t *testing.T) {
	block := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-block // never unblocks during this test
	}))
	defer func() {
		close(block)
		srv.Close()
	}()

	fake := newFakeStorageClient()
	o := &Orchestrator{gcs: fake, UsageBucket: "test-bucket", Reporter: NewUsageReporter(srv.URL, "test-shared-secret", "weitzer-org/sound-profile-builder")}

	done := make(chan struct{})
	go func() {
		recordAttemptUsage(context.Background(), o, "Tone Historian", "gemini-3.1-pro-preview", 500, nil, errors.New("429 rate limited"))
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(1 * time.Second):
		t.Fatal("recordAttemptUsage blocked on a slow attached Reporter instead of returning immediately")
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

func TestRecordAttemptUsage_PopulatesCachedInputTokens(t *testing.T) {
	fake := newFakeStorageClient()
	o := &Orchestrator{gcs: fake, UsageBucket: "test-bucket"}
	resp := &genai.GenerateContentResponse{
		UsageMetadata: &genai.GenerateContentResponseUsageMetadata{
			PromptTokenCount: 1_000_000, CachedContentTokenCount: 600_000, CandidatesTokenCount: 0,
		},
	}
	defer func(f func() time.Time) { nowFunc = f }(nowFunc)
	nowFunc = func() time.Time { return day(2027, time.February, 1) } // standard-period prices
	recordAttemptUsage(context.Background(), o, "CorOS Librarian", "gemini-3.6-flash", 100, resp, nil)

	rec := fake.firstRecord(t)
	if rec.InputTokens != 1_000_000 {
		t.Errorf("InputTokens must stay the full PromptTokenCount, got %d", rec.InputTokens)
	}
	if rec.CachedInputTokens != 600_000 {
		t.Errorf("CachedInputTokens = %d, want 600000", rec.CachedInputTokens)
	}
	want := 0.4*1.50 + 0.6*0.15
	if d := rec.CostUSD - want; d > 1e-9 || d < -1e-9 {
		t.Errorf("CostUSD = %v, want %v", rec.CostUSD, want)
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

// Prices from Google Cloud's pricing page (2026-11-05): introductory windows end on fixed dates.
func TestPriceSchedule_IntroductoryWindows(t *testing.T) {
	cases := []struct {
		model   string
		at      time.Time
		in, out float64
	}{
		{"gemini-4-argon", day(2026, time.November, 5), 2.00, 10.00},
		{"gemini-4-argon", day(2026, time.November, 30).Add(23 * time.Hour), 2.00, 10.00},
		{"gemini-4-argon", day(2026, time.December, 1), 4.00, 20.00},
		{"gemini-3.8-flash", day(2026, time.December, 31), 0.75, 3.75},
		{"gemini-3.8-flash", day(2027, time.January, 1), 1.50, 7.50},
		{"gemini-3.6-flash", day(2026, time.November, 5), 0.75, 3.75},
		{"gemini-3.5-flash-lite", day(2026, time.November, 5), 0.30, 2.50},
	}
	for _, c := range cases {
		p, ok := priceAt(c.model, c.at)
		if !ok || p.input != c.in || p.output != c.out {
			t.Errorf("priceAt(%s, %s) = %+v %v, want %v/%v", c.model, c.at.Format("2006-01-02T15"), p, ok, c.in, c.out)
		}
	}
	// 1M in + 1M out + 1M thinking on Argon after Dec 1: 4 + 20 + 20.
	if got := computeCostUSDAt("gemini-4-argon", day(2026, time.December, 2), 1_000_000, 0, 1_000_000, 1_000_000); got != 44 {
		t.Errorf("Argon standard cost = %v, want 44", got)
	}
}

func TestComputeCostUSD_UsesNow(t *testing.T) {
	old := nowFunc
	defer func() { nowFunc = old }()
	nowFunc = func() time.Time { return day(2027, time.February, 1) }
	if got := computeCostUSD("gemini-3.8-flash", 1_000_000, 0, 0, 0); got != 1.50 {
		t.Errorf("3.8 Flash input after the intro window = %v, want 1.50", got)
	}
}
