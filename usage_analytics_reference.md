# Usage analytics reference

How LLM call data (tokens, latency, cost, success/failure) is captured for
this repo, and where to find it — written so a future session (human or AI)
can answer a cost/latency/error-rate question directly from R2/GCS/MinIO
without building a dashboard first. This is the sound-profile-builder
counterpart of the identical pattern in the sibling `job_tracker` and `gsr`
projects — all three deliberately share the same storage layout and record
schema, so query recipes are interchangeable across projects.

## What's captured, and by what

Every real agent call in this codebase — all 13 pipeline agents
(`RunPipeline`) plus `RefineChat` — funnels through one function:
`Orchestrator.RunAgentSplit` (`internal/agents/orchestrator.go`). That's the
single choke point `recordAttemptUsage`/`recordUsage`
(`internal/agents/usage_recorder.go`) are wired into — unlike `gsr`, which
had several scattered call sites, this repo already centralized every LLM
call in one place, so instrumenting it required no new call-site
duplication.

`callType` on every record is the **agent role** (e.g. `"Tone Historian"`,
`"Architect & Evaluator"`, `"Refinement Architect"`), matching the labels
already used in `AGENT_TOKEN_LOG` console output and `TokenUsage.PerAgent`.

Two backends are tracked, distinguished by `provider`:
- `"gemini"` — the standard Gemini branch, including every model tried in
  the fallback chain (`getFallbackChain`) if the primary model fails, and
  the same-model "retry without grounding" degraded attempt (see
  `RunAgentSplit`'s comment on Tools+JSON-mime-type rejection).
- `"openllm"` — the Open-LLM branch (`USE_OPENLLM=true` or an
  `"openllm:"`-prefixed model override).

A **failed** call is logged too (`success: false`, zero tokens, a coarse
`errorKind` — `rate_limit`, `auth`, `timeout`, `unavailable`, or
`api_error`, see `classifyError`) for **every** attempt in the fallback
chain, not just the first one — so a run that needed its 3rd fallback model
to succeed leaves two failure records and one success record, all with
their own real latency.

### What's NOT covered

- **`cmd/eval_*`, `cmd/enrich_captures`, `cmd/verify_user_captures`,
  `cmd/probe_thinking`, `cmd/save_presets`** construct an `Orchestrator`
  with `gcs=nil` (see their calls to `agents.NewOrchestrator`) and never set
  `UsageBucket` — `recordUsage` is nil-safe on both, so these tools simply
  don't record anything. This mirrors the same boundary `job_tracker` draws
  around its own `cmd/eval` (a separate, deliberately-excluded cost-tracked
  tool) and required no code changes to any of these 10 call sites.
- No `refId` (e.g. a preset ID) is threaded through yet — a record can be
  correlated to *which agent* and *when*, but not yet to *which preset
  generation run* it belonged to. A reasonable follow-up, not done here to
  keep this change a pure instrumentation add with no method signature
  changes to `RunPipeline`/`RunAgentSplit`/`RefineChat`.
- The `countTokens`-style pre-flight calls other sibling projects might have
  don't apply here; this repo has none in the agent path.

### Also reported to GSR's hosted usage dashboard

Beyond writing to this project's own bucket (below), every recorded
`UsageRecord` is also, optionally, pushed to the GSR code-review project's
own hosted usage dashboard (`internal/agents/usage_reporter.go`'s
`UsageReporter`, wired into `Orchestrator.Reporter` in `cmd/server/main.go`)
— so this project's native Gemini usage shows up there too, tagged
`repository: "weitzer-org/sound-profile-builder"` (the real GitHub slug,
**not** this repo's Go module path, `github.com/weitzer-org/sound-builder`)
and classified under GSR's `"product"` workload (distinct from the
`"review"` usage GSR's own GitHub Action already reports when it reviews a
PR here — see that project's `usage_analytics_reference.md`).

- **Opt-in, not automatic**: only active when both `GSR_USAGE_INGEST_URL`
  and `GSR_USAGE_INGEST_KEY` are set (see `.env.example`) — absent means
  silently disabled, the same convention GSR itself uses for its own
  optional secrets. Neither is set by default, even in production.
- **`"openllm"`-provider records are filtered out before ever reaching
  GSR** — GSR's ingest endpoint only accepts `provider == "gemini"`.
- **Best-effort and non-blocking**: reports go through a small bounded
  worker pool (a fixed number of goroutines draining a bounded channel),
  never the agent-pipeline request path itself. A report still queued when
  the process exits (deploy, restart) is lost — an accepted tradeoff for a
  side-channel that must never slow down or fail a real pipeline request.
- This project's own bucket (below) remains the authoritative source for
  its own usage — the GSR dashboard is a convenience aggregate view across
  GSR, `tools/eval`, job_tracker, and this project, not a replacement for
  `cmd/usage_report` against this project's own data.

## Where the data lives

```
usage/{YYYY-MM-DD}/{HHMMSSmmm}-{8 hex chars}.json   — one object per call attempt
usage/rollups/{YYYY-MM-DD}.json                      — optional persisted daily aggregate
```

Both live in the same bucket presets/memories already use (`cfg.BucketName`
/ `S3_BUCKET` / `GCS_BUCKET` — whichever `STORAGE_BACKEND` selects). Per-call
objects are written unconditionally (`storage.Client.WriteFile` has no
ETag/conditional-write parameter in this repo to begin with) — no
read-modify-write, so nothing here can contend. A rollup is a recomputed
summary, not an append-only log: writing one for a date overwrites whatever
was there before.

Rollups are **not created automatically** — nothing in the app writes them
on a schedule. They only exist for a date if `cmd/usage_report -write-rollup`
was run for it.

## Record schema (`agents.UsageRecord`)

```jsonc
{
  "timestamp": "2026-07-29T20:13:53.282Z",   // RFC3339Nano, UTC
  "provider": "gemini",                       // "gemini" | "openllm"
  "callType": "Tone Historian",               // agent role
  "refId": "",                                // not populated yet — see "What's NOT covered"
  "model": "gemini-3.1-pro-preview",
  "inputTokens": 1345,
  "outputTokens": 210,
  "thinkingTokens": 0,                        // omitted when zero
  "latencyMs": 2431,
  "costUsd": 0.0034,
  "success": true,                            // always present, even when false
  "errorKind": "rate_limit"                   // present only when success is false
}
```

## How to query it

**Preferred: `cmd/usage_report`** — handles backend selection (mirrors
`cmd/server`'s `STORAGE_BACKEND`/config resolution exactly), listing,
decoding, and aggregating for you:

```bash
go run ./cmd/usage_report                                  # today, UTC
go run ./cmd/usage_report -date 2026-07-29
go run ./cmd/usage_report -from 2026-07-23 -to 2026-07-29   # range + a combined total
go run ./cmd/usage_report -date 2026-07-29 -write-rollup    # also persist the rollup
go run ./cmd/usage_report -bucket my-bucket -date 2026-07-29
```

Needs the same environment `cmd/server` runs with (`STORAGE_BACKEND`,
`config.json`, and whichever of `S3_*`/`GCS_BUCKET`/ADC credentials that
backend requires). Prints total calls/tokens/cost/avg latency plus
breakdowns by agent (`callType`), model, provider, and error kind.
Report-only by default; `-write-rollup` is the only thing that writes
anything.

**Ad hoc queries the CLI doesn't cover** — read the raw objects directly and
pipe through `jq`:

```bash
# Via the aws CLI (S3 backend) + S3_* env vars:
aws s3api list-objects-v2 --endpoint-url "$S3_ENDPOINT" --bucket "$S3_BUCKET" \
  --prefix "usage/2026-07-29/" --query 'Contents[].Key' --output text | tr '\t' '\n' | \
while read -r key; do [ -n "$key" ] && [ "$key" != "None" ] && aws s3 cp --endpoint-url "$S3_ENDPOINT" "s3://$S3_BUCKET/$key" -; done | jq -s '
  group_by(.callType) | map({callType: .[0].callType, totalCost: (map(.costUsd) | add)})'

# Via gsutil (GCS backend):
gsutil ls "gs://$GCS_BUCKET/usage/2026-07-29/**" | \
while read -r uri; do gsutil cat "$uri"; done | jq -s '
  group_by(.provider) | map({provider: .[0].provider, calls: length})'
```

(`--output text` prints the literal string `None` when no S3 objects matched
the prefix — the `[ "$key" != "None" ]` guard avoids a spurious `aws s3 cp`
of a nonexistent `None` object in that case.)

Swap the `jq` filter for whatever question you're answering (`select(.success
== false)` for an error-rate check, `sort_by(.latencyMs) | .[-1]` for the
slowest call, etc.). If neither CLI is available, `internal/agents.
ListUsageRecords` is the programmatic fallback — write a one-off `go run`
snippet calling it directly rather than reinventing the listing/reading logic.

## Rollup schema (`agents.UsageRollup`, written by `-write-rollup`)

```jsonc
{
  "date": "2026-07-29",
  "totalCalls": 42, "successCount": 40, "failureCount": 2,
  "totalInputTokens": 58000, "totalOutputTokens": 9200, "totalThinkingTokens": 4100,
  "totalCostUsd": 0.87, "avgLatencyMs": 2103.4,
  "byCallType": { "Tone Historian": { "calls": 12, "successCount": 12, "failureCount": 0, "inputTokens": 15000, "outputTokens": 2200, "costUsd": 0.20 }, "...": "..." },
  "byModel":    { "gemini-3.1-pro-preview": { "...": "..." } },
  "byProvider": { "gemini": { "...": "..." }, "openllm": { "...": "..." } },
  "byErrorKind": { "rate_limit": 2 }
}
```

## Pricing table maintenance

`internal/agents/usage_recorder.go`'s `geminiPriceTable` mirrors the
sibling `job_tracker` project's `internal/scoring/pricing.go` for the models
both share (`gemini-3.1-pro-preview`, `gemini-3.5-flash`,
`gemini-3.6-flash`). This repo's own fallback chain (`getFallbackChain`)
additionally uses `gemini-2.5-pro` and `gemini-3-flash-preview`, which
`job_tracker` doesn't reference — those two figures are this repo's own and
should be verified against Google's published pricing before relying on
them for a real cost report (the `3-flash-preview` figure is currently a
placeholder matching `3.5-flash`'s rate).
