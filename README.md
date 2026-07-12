# QC-2 Multi-Agent Modeler

This repository holds the Go-based Multi-Agent system that orchestrates Google Gemini Phase-Agents (Tone Historian, CorOS Librarian, FOH Optimizer, etc.) to evaluate and build physics-accurate Quad Cortex DSP matrices.

## 🖥️ Local Development (Docker)

Run the full stack locally in Docker — the app plus a **MinIO** (S3-compatible) store that stands in for Cloudflare R2. No cloud accounts required.

**Prerequisites:** Docker + Docker Compose (on Windows: Docker Desktop with WSL2).

```bash
cp .env.example .env      # first run only (the scripts also create it if missing)
```

Start in **mock mode** — canned pipeline, no Gemini key needed:
```bash
./run-mock.sh             # streams logs; add -d to run detached
```

Start in **live mode** — real Gemini pipeline (needs a valid GEMINI_API_KEY in .env):
```bash
./run-live.sh
```

- App: **http://localhost:8080** — log in with the `MOCK_PASSWORD` from your `.env`
- MinIO console: **http://localhost:9001** (`minioadmin` / `minioadmin`)
- Stop: `docker compose down` (add `-v` to also wipe stored presets)

See `CLAUDE.md` for architecture, storage backends, and testing.

## 🚀 Deployment (Fly.io — production)

**Fly.io + Cloudflare R2** is the chosen production runtime (`fly.toml`). The Google Cloud path below is preserved for a possible future migration, not currently used for production traffic.

### Auto-deploy on merge

`.github/workflows/ci.yml` runs on every push and PR:
- The **Unit** job (`go build`, `go vet`, `go test ./...`) runs on PRs and pushes alike — this is the PR check gate.
- The **Deploy** job only fires on an actual push to `main` (i.e. after a PR merges), and only if Unit passed first. It runs `flyctl deploy --remote-only` using the `FLY_API_TOKEN` repo secret. PRs never deploy — the Deploy job shows as *skipped*, not run, in a PR's checks.

So the normal flow is: open a PR → CI runs unit tests → merge → CI deploys to Fly.io automatically. No manual deploy step required for normal changes.

### Manual deploy / first-time setup

```bash
fly launch --no-deploy   # first time only -- creates the app, may rename it
fly deploy               # build + deploy from your local machine
```

Secrets are set on Fly, not committed (`fly.toml` documents the full list):
```bash
fly secrets set \
  GEMINI_API_KEY=... \
  UI_PASSWORD=<your-ui-login-password> \
  S3_ACCESS_KEY_ID=... \
  S3_SECRET_ACCESS_KEY=... \
  S3_ENDPOINT=https://<ACCOUNT_ID>.r2.cloudflarestorage.com \
  S3_BUCKET=<your-r2-bucket>
```

Storage is Cloudflare R2 via the S3-compatible backend (`STORAGE_BACKEND=s3`, set in `fly.toml`'s `[env]`); the app scales to zero when idle and wakes in ~1-2s on the next request (`fly.toml`'s `[http_service]`).

To set up the auto-deploy CI itself in a new environment, add a `FLY_API_TOKEN` repo secret:
```bash
fly tokens create deploy -x 999999h
# then add it under GitHub -> Settings -> Secrets and variables -> Actions
```

## 🗄️ Legacy / Future Deployment (Google Cloud Run)

The original runtime, kept working intentionally in case of a future migration back to GCP -- not used for current production traffic. Because of internal MacOS constraints (`Santa` restricting `go build` binaries in randomized temp `/tmp` folders), this path deploys dynamically via **Google Cloud Build** to **Cloud Run** for isolated, secure execution and testing.

### 1. Provisioning the Cloud Storage Backend
The CorOS mapping dictionary and agent cache live inside a secure Google Cloud Storage bucket (`gs://weitzer-sound-builder`). 
Because we mocked a baseline `coros_map.json` dictionary for testing, you must upload it to the Bucket before your agents can query it. 

Copy the local mock file into your active Bucket:
```bash
gcloud storage cp ./coros_map.json gs://weitzer-sound-builder/
```

### 2. Deploying via Cloud Build
We enforce building and deploying strictly from the supplied `cloudbuild.yaml` file. The file utilizes a Private Worker Pool (`faster-machine`) to bypass cold start constraints and builds a multi-stage distroless Linux runtime.

Submit a build natively from your local terminal using our wrapper script (which forces the URL output natively on your Mac):
```bash
# Deploys code, pushes to Artifact Registry, and surfaces the HTTP container URL natively
chmod +x deploy.sh
./deploy.sh
```
*(Note: Because of Google VPC-SC log streaming constraints, the `logging: CLOUD_LOGGING_ONLY` flag is intrinsically bound to this YAML structure).*

### 3. Application Access
When the `gcloud beta builds submit` command successfully resolves, look at the bottom of the streamed terminal output! It runs a bash echo dynamically surfacing your active URL string:
`🚀 YOUR LIVE DASHBOARD URL IS: https://sound-builder-dev-[hash].a.run.app`

Click the URL to access your live HTMX Dashboard protected by the native Service Account.
