# QC-2 Multi-Agent Modeler — Project Guide

Go service that orchestrates a 13-agent Google Gemini pipeline (12 generation
agents plus an advisory Preset Critic) to generate physics-accurate Quad
Cortex (QC) guitar presets, served through an HTMX dashboard.

## Run locally (Docker)
Local dev runs the app in a container with a **MinIO** (S3-compatible) store
standing in for Cloudflare R2 — no cloud accounts required.

```bash
cp .env.example .env        # first time
./run-mock.sh               # MOCK mode: canned pipeline, no Gemini key needed
./run-live.sh               # LIVE mode: requires a real GEMINI_API_KEY in .env
```
- App: http://localhost:8080 (log in with the `MOCK_PASSWORD` from `.env`)
- MinIO console: http://localhost:9001 (`minioadmin` / `minioadmin`)
- Stop: `docker compose down` (add `-v` to also wipe the MinIO volume)

## Architecture
- `cmd/server/main.go` — entrypoint; selects the storage backend, builds the server.
- `internal/api` — HTTP handlers, auth middleware (HMAC session cookie), HTMX server.
- `internal/agents` — orchestrator + 13-agent Gemini/Open-LLM pipeline
  (12 generation agents plus a 13th advisory Preset Critic that re-reads the
  Architect's output for prose-vs-data contradictions; `MOCK_MODE=true`
  returns canned data instead of calling the LLM).
- `internal/storage` — `Client` interface
  (`ReadFile`/`WriteFile`/`ListFiles`/`DeleteFile`/`Close`) with two backends:
  - `gcs.go` — Google Cloud Storage (original; **kept for a future GCP move**).
  - `s3.go` — S3-compatible (Cloudflare R2 in prod, MinIO locally) via AWS SDK Go v2.
  - Presets and memories are JSON blobs keyed by UUID under `presets/` and `memories/`.
- `internal/config` — loads `config.json` with env overrides.

## Capture metadata (`coros_map.json` / `user_captures.json`)
Both are embedded (`//go:embed`, `internal/agents/orchestrator.go`) — changes require a
rebuild/redeploy, not just a file edit.

**Both enrichment tools only ever research new/not-yet-covered entries, never the whole
file again** — important since new factory captures ship with firmware updates and new
user captures get downloaded regularly, and re-researching everything each time would
waste API budget on records that already have a good answer. For `coros_map.json`, an
entry with a non-empty `tonal_archetype` already IS the "done" signal (`cmd/enrich_captures`
only targets entries where it's empty) — no extra field needed. For `user_captures.json`,
every entry has *some* description by construction, so there's no natural empty-field
signal; `description_verified: true` on an entry is what marks it as already covered by a
citation-backed pass. Set it explicitly in `user_captures.json` when merging an approved
`verified_description` from `cmd/verify_user_captures`'s draft file — a merge that doesn't
set it will cause the next run to needlessly re-research that entry.

- **`coros_map.json`** — the factory capture map: real-world gear name -> QC block name
  (`coros_equivalent`), whether it's a pre-trained Neural Capture (`is_capture`), and an
  optional `tonal_archetype` (a real descriptive tonal color, consumed by
  `SelectedCaptureContext` in `internal/agents/fuzzy_matcher.go` to help the Architect pick
  the right relative-dB direction/magnitude on confirmed-capture blocks per Rule 9). An
  entry for a device that ships inside a paid Neural DSP Archetype plugin (Plini, Gojira,
  Cory Wong, John Mayer, ...) also carries `required_plugin` (the short name as it appears
  in `config.json`'s `available_plugins`) — `Orchestrator.RunPipeline` and
  `GetCategorizedAmplifiers` (`internal/agents/orchestrator.go`,
  `internal/agents/fuzzy_matcher.go`) use it to strip that entry out of the Librarian's
  Dictionary/amp menu entirely when the user doesn't own the plugin, rather than relying
  on the "AVAILABLE PLUGINS: ..." prompt line alone to discourage it.
  `tonal_archetype` coverage is sparse today (~51%) — this is a known, tracked gap (see
  TODO.md's Pipeline Quality Work section for the full history), not an oversight to fix
  ad hoc. **Whenever new entries are added to `coros_map.json`** (e.g. after a CorOS/NanOS
  firmware update adds new factory captures), re-run `cmd/enrich_captures` afterward to
  research `tonal_archetype` for any of the new entries that are missing one — don't
  hand-write it inline in the same edit. It's a two-step process by design: the tool writes
  proposed labels (each with a citation) to a sibling draft file, never `coros_map.json`
  directly, and a human reviews/merges them via a normal PR — see `cmd/enrich_captures`'s
  package doc comment and TODO.md for the full rationale (constrained label vocabulary,
  why it's offline/PR-gated rather than live).
  - Do NOT commit a new capture entry with a guessed or hand-invented `tonal_archetype` —
    either leave it unset until `cmd/enrich_captures` researches it, or add it yourself only
    with a real citeable source, same bar the tool holds itself to.
- **`user_captures.json`** — the personal/downloaded 3rd-party (Cortex Cloud) capture
  library. Structurally different from `coros_map.json`: every entry already carries a
  `description` field (coverage is 100%, confirmed by inspecting the file directly), so
  there's no *coverage* gap the way there is for `coros_map.json`. But coverage isn't the
  same as verified accuracy — the original 87 descriptions (commit `62ff925`) were written
  by an earlier Claude Code session inferring gear identity from the (often cryptic,
  Cortex-Cloud-exported) capture name alone, with no search grounding and no citation, the
  same unverified-guess risk `coros_map.json`'s gap represented. `cmd/verify_user_captures`
  (parallel tool to `cmd/enrich_captures`, same offline/draft-file/PR-gated process)
  independently re-derives a citation-backed description from the name alone for every
  entry (deliberately not shown the existing description, to avoid anchoring the new
  answer on a possibly-wrong old one) and writes both side by side in the draft for
  comparison — the new answer isn't assumed better, a human still judges each one.
  **When adding a new user capture, write a real, specific `description` in the same
  edit** (not a placeholder) and run it through `cmd/verify_user_captures` for a
  citation-backed second opinion before treating it as settled, same bar as
  `coros_map.json`.

## Storage backend selection
`STORAGE_BACKEND=s3` (default `gcs`). For `s3`, set `S3_ENDPOINT`, `S3_BUCKET`,
`S3_ACCESS_KEY_ID`, `S3_SECRET_ACCESS_KEY`, `S3_REGION` (`auto` for R2).
See `.env.example`.

**Usage analytics.** Every real agent call funnels through `Orchestrator.
RunAgentSplit` (`internal/agents/orchestrator.go`), which is wrapped by
`recordAttemptUsage`/`recordUsage` (`internal/agents/usage_recorder.go`) to
persist a per-call token/latency/cost/success record to the same bucket
under `usage/<date>/`. See `usage_analytics_reference.md` for the schema and
query recipes (`cmd/usage_report`, or raw `jq`-over-S3/GCS) — read that
before answering any "what's our token spend/error rate" question instead of
re-deriving the storage layout. `usage_recorder.go`'s `geminiPriceTable`
mirrors `job_tracker`'s `internal/scoring/pricing.go` — keep both in sync
when Gemini prices change. `internal/agents/usage_reporter.go`'s
`UsageReporter` also optionally pushes this same data to the GSR
code-review project's hosted usage dashboard (`GSR_USAGE_INGEST_URL`/
`GSR_USAGE_INGEST_KEY`, see `.env.example`) — opt-in, non-blocking, filtered
to `provider=="gemini"` only.

## Secrets & auth
Setting `MOCK_PASSWORD` uses a local secret fetcher (skips GCP Secret Manager)
and becomes the dashboard login password; `GEMINI_API_KEY` supplies the LLM key.
In prod, secrets are injected as env vars (Fly: `fly secrets`), never committed.
`.env` is git-ignored.

## Tests
- **Unit:** `go test ./...`
- **E2E (Playwright, HTMX UI):** `tests/e2e/` — drives a running app and uses
  `?mock=true`. Node isn't installed locally; run the suite via the
  `mcr.microsoft.com/playwright:v1.58.2` container against the app.
- Always run and show the **unit + e2e results** as proof before claiming the
  application works.

## Code review
Merges to `main` auto-deploy to prod (see Deployment) and there's no CI test
gate, so review before merging is the main safety net.

**Cost policy (Claude quota is a real constraint on this project).** The
bundled `/code-review` spawns 8 finder agents plus up to 8 verifiers — ~17
model calls per run, the single largest discretionary expense in the workflow.
So it is **not** the default. As of 2026-07-15, **`/quick-review` is no
longer run automatically either** — the free GitHub-integrated bots
(gemini-code-assist, CodeRabbit) and the **GSR GitHub Action**
(`.github/workflows/gsr-review.yml`, github.com/weitzer-org/gsr, `basic`
mode) review every PR at zero Claude quota — GSR runs on its own Gemini API
key, not Claude's — and are now the sole default gate for routine PRs.

- **Default — every PR:** open the PR and let GSR + gemini-code-assist +
  CodeRabbit review it. No automatic Claude review step. Read and address
  `security-high`/`security-critical` findings before merging regardless of
  what else ran.
- **Run `/quick-review` on demand, not automatically** — when you explicitly
  want a local pass before opening a PR, or a bot finding is worth a second
  look. Still the project's own **`/quick-review`** (`.claude/skills/quick-review/`):
  one inline pass, no sub-agents, ~1 call.
- **Escalate to the multi-agent `/code-review high`** only for large or
  architecturally risky changes (auth, storage backend, agent pipeline logic)
  where the fan-out's extra recall is worth ~17 calls. Always pass the effort
  level explicitly. `medium` and `high` both run the same 8 parallel finder
  agents; the level changes candidate volume and verify aggressiveness
  (precision-biased at `medium`, recall-biased at `high`), not agent count.
- For the same class of large/risky change, also consider **GSR's agent-swarm
  mode** (`.github/workflows/gsr-review-deep.yml`) as a zero-Claude-quota
  complement to `/code-review high` — apply the `deep-review` label to the PR
  to trigger it. It runs GSR's full swarm (Architecture, Logic, Security,
  TechDebt, Testing agents + a dedup pass) instead of the `basic` single-pass
  mode that runs automatically on every PR. Label-only, deliberately: GSR's
  entrypoint hard-requires a real `pull_request` event payload, and a
  `workflow_dispatch` manual-trigger path was tried and dropped after two
  different workarounds both failed on live testing for platform-level
  reasons (see the workflow file's comments).
- Reserve `/code-review ultra` (multi-agent cloud review) for substantial
  features before merge — it's billed separately, so don't run it routinely.
- `/code-review --fix` applies findings directly if you want them auto-fixed.
- If a diff feels too big or risky for a single `/quick-review` pass, say so
  and let the user decide whether to budget for the full fan-out — don't
  quietly spawn sub-agents to compensate.

### Responding to GSR's PR comments
`gsr-review.yml`/`gsr-review-deep.yml` run GSR's PR-comment feedback loop
(`feedback-loop: respond`, `feedback-post: true`) — GSR reads replies to its
own findings and can post a rebuttal back if it disagrees. That loop only has
something to classify if a reply exists in the first place, so when GSR
leaves findings on a PR you're working on, reply to at least the
`security-high`/`security-critical`/`Architecture`/`Logic` ones inline (fix
and say so, or push back with a concrete reason) rather than leaving them
unaddressed — don't just silently fix-and-merge. If GSR's feedback loop then
posts a rebuttal to your reply, engage with that too instead of treating it
as the last word or as noise: at least one more round (concede, or restate
why the original reply stands) before moving on. Unlike gemini-code-assist/
CodeRabbit comments (background automation this repo treats as a second
opinion, not something to chase proactively), GSR is this project's own
dogfooded tool — its findings and rebuttals are worth a real reply.

### Security review
The standard review lenses (correctness, cleanup, altitude, conventions) are
not a substitute for an explicit security pass — they check whether a change
does what it intends, not whether an adversary can bend it. A hand-rolled regex
"sanitizer" for agent-authored HTML passed an internal `/code-review high` pass
in this repo and still shipped with several XSS bypasses (HTML-entity-encoded
schemes, attribute selectors without leading whitespace, dangerous attributes
beyond href/src) that only surfaced once GitHub's automated reviewers
(gemini-code-assist, CodeRabbit) looked at the PR with that specific lens.

- Run **`/security-review`** (project skill, `.claude/skills/security-review/`)
  as an optional, additive pass — not a replacement for `/quick-review` —
  whenever a diff touches auth, secrets, storage-backend credentials, or how
  externally-influenced content (user input, Gemini/LLM output, anything
  from the agent pipeline) gets rendered, parsed, or escaped. It runs
  adversarial angles (injection, auth/authz, secrets handling, supply chain)
  the standard lenses don't cover. It was rewritten to a **single inline pass
  (no sub-agents, ~1 call)** to fit the cost policy above — same quota profile
  as `/quick-review`, not the old multi-agent fan-out.
- Don't hand-roll HTML sanitization, escaping, or URL-scheme filtering with
  regex — regex can't safely parse HTML. Use a real parser-based allowlist
  library (this repo uses `github.com/microcosm-cc/bluemonday`).
- Treat the GitHub-integrated bots' comments on open PRs (gemini-code-assist,
  CodeRabbit) as a real second opinion, not noise — they've caught real
  security bugs internal review missed. Read and address `security-high`/
  `security-critical` findings before merging regardless of what level of
  `/code-review` already ran.

## Deployment
- **Fly.io** (chosen runtime): `fly.toml`; secrets via `fly secrets`; storage =
  Cloudflare R2 (S3 backend). Scales to zero.
- **GCP** (future / preserved): `cloudbuild.yaml` → Cloud Run + GCS.

## Conventions
- Keep the GCP/GCS path intact — the project may migrate back to GCP.
- Storage is accessed only through the `storage.Client` interface; add backends
  there, don't special-case callers.
