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

## Storage backend selection
`STORAGE_BACKEND=s3` (default `gcs`). For `s3`, set `S3_ENDPOINT`, `S3_BUCKET`,
`S3_ACCESS_KEY_ID`, `S3_SECRET_ACCESS_KEY`, `S3_REGION` (`auto` for R2).
See `.env.example`.

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

- Before opening/merging a PR, run **`/code-review low`** or
  **`/code-review medium`** against the branch diff — always pass the effort
  level explicitly rather than relying on whatever `/code-review` defaults to
  bare. `medium` and `high` both run the same 8 parallel finder agents; the
  effort level changes candidate volume per agent and how aggressively
  findings get verified (precision-biased at `medium`, recall-biased at
  `high`), not the number of agents spawned.
- Small/low-risk diffs (typos, config, doc tweaks): `/code-review low` is
  enough.
- Larger or risky changes (auth, storage backend, agent pipeline logic):
  `/code-review high`.
- Reserve `/code-review ultra` (multi-agent cloud review) for substantial
  features before merge — it's billed separately, so don't run it routinely.
- `/code-review --fix` applies the findings directly if you want them
  auto-fixed instead of just reported.

### Security review
`/code-review`'s standard finder angles (correctness, cleanup, altitude,
conventions) are not a substitute for an explicit security pass — they check
whether a change does what it intends, not whether an adversary can bend it.
A hand-rolled regex "sanitizer" for agent-authored HTML passed an internal
`/code-review high` pass in this repo and still shipped with several XSS
bypasses (HTML-entity-encoded schemes, attribute selectors without leading
whitespace, dangerous attributes beyond href/src) that only surfaced once
GitHub's automated reviewers (gemini-code-assist, CodeRabbit) looked at the
PR with that specific lens.

- Run **`/security-review`** (project skill, `.claude/skills/security-review/`)
  as an optional, additive pass — not a replacement for `/code-review` —
  whenever a diff touches auth, secrets, storage-backend credentials, or how
  externally-influenced content (user input, Gemini/LLM output, anything
  from the agent pipeline) gets rendered, parsed, or escaped. It runs
  adversarial finder angles (injection, auth/authz, secrets handling, supply
  chain) that `/code-review`'s standard angles don't cover.
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
