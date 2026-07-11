# QC-2 Multi-Agent Modeler — Project Guide

Go service that orchestrates a 12-agent Google Gemini pipeline to generate
physics-accurate Quad Cortex (QC) guitar presets, served through an HTMX
dashboard.

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
- `internal/agents` — orchestrator + 12-agent Gemini/Open-LLM pipeline
  (`MOCK_MODE=true` returns canned data instead of calling the LLM).
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
  level explicitly. Bare `/code-review` (no args) defaults to `high`, which
  spawns 8 parallel finder agents plus verification passes; that's the
  expensive tier, not the routine one.
- Small/low-risk diffs (typos, config, doc tweaks): `/code-review low` is
  enough.
- Larger or risky changes (auth, storage backend, agent pipeline logic):
  `/code-review high`.
- Reserve `/code-review ultra` (multi-agent cloud review) for substantial
  features before merge — it's billed separately, so don't run it routinely.
- `/code-review --fix` applies the findings directly if you want them
  auto-fixed instead of just reported.

## Deployment
- **Fly.io** (chosen runtime): `fly.toml`; secrets via `fly secrets`; storage =
  Cloudflare R2 (S3 backend). Scales to zero.
- **GCP** (future / preserved): `cloudbuild.yaml` → Cloud Run + GCS.

## Conventions
- Keep the GCP/GCS path intact — the project may migrate back to GCP.
- Storage is accessed only through the `storage.Client` interface; add backends
  there, don't special-case callers.
