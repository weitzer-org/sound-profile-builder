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
  S3_BUCKET=<your-r2-bucket> \
  MCP_TOKEN=$(openssl rand -hex 32)
```

Storage is Cloudflare R2 via the S3-compatible backend (`STORAGE_BACKEND=s3`, set in `fly.toml`'s `[env]`); the app scales to zero when idle and wakes in ~1-2s on the next request (`fly.toml`'s `[http_service]`).

To set up the auto-deploy CI itself in a new environment, add a `FLY_API_TOKEN` repo secret:
```bash
fly tokens create deploy -x 999999h
# then add it under GitHub -> Settings -> Secrets and variables -> Actions
```

## 🔌 MCP server (Claude connector)

The app exposes an MCP server at `/mcp`, so claude.ai or Claude Code can generate, refine, read and save presets, and inspect run logs and prompts. It runs in the same process as the web UI and shares its rate limit and concurrency cap.

**Enable it** by setting `MCP_TOKEN` (at least 32 characters; generate one with `openssl rand -hex 32`). If `MCP_TOKEN` is unset or too short, MCP is disabled and every request is rejected. Locally, `.env.example` and `docker-compose.yml` already set a placeholder token.

**Connect claude.ai (header form, preferred):**
1. Settings → Connectors → Add custom connector.
2. URL: `https://<app>.fly.dev/mcp`. The first probe gets a 401 with no OAuth metadata. Click **Continue anyway**.
3. Choose **No sign-in**. Under **Request headers**, set the name to `authorization` and the value to `Bearer <token>`. Type the `Bearer ` prefix yourself.
4. Keep Transport on Streamable HTTP.

Auth settings can't be edited after the connector is added. To rotate, set `MCP_TOKEN_NEXT` on the app, remove and re-add the connector with the new token, then promote it to `MCP_TOKEN`.

**Connect Claude Code:**
```bash
claude mcp add --transport http qc2 https://<app>.fly.dev/mcp --header "Authorization: Bearer $MCP_TOKEN"
```

**Fallback for clients that can't set headers:** use the URL `https://<app>.fly.dev/mcp/<token>` with no header. The URL itself is the credential, so treat it like a password.

**Local:** `http://localhost:8080/mcp` with the placeholder token from `.env.example`.

**Tools:**
- `list_presets`: list saved presets and drafts.
- `get_preset`: read one preset as Markdown or JSON.
- `save_preset`: rename a draft to save it.
- `generate_preset`: start a generation run and wait for the result. It accepts every option on the web UI's tone prompt form (`allow_factory_captures`, `allow_paid_plugins`, `allow_user_captures` = "Use My Capture Library", `favor_captures`, `favor_cloud_captures` = "Favor My Capture Library"), plus three **MCP-only** options with no UI checkbox: `guitars` (1-3 names; omit for the default Gibson ES-339 + Fender Telecaster pair), `single_amp_mode` and `allow_cloud_captures` (both: the request value wins, omit to use `config.json`). A test (`internal/mcpserver/parity_test.go`) fails if the UI form gains a field MCP lacks.
- `refine_preset`: apply a refinement to an existing preset.
- `get_run_status`: check or long-poll a run's progress.
- `get_run_log`: fetch a run's timeline, prompts and totals.
- `get_run_prompts`: read the prompts sent to each agent, in slices.
- `get_agent_output`: read one agent's output, in slices.

**Resources:**
- `qc://runs/{run_id}/log`: the run log.
- `qc://runs/{run_id}/prompts/{prompt_key}`: one agent's system and input prompts.
- `qc://presets/{preset_id}`: a preset.
- `qc://prompt-templates/{prompt_key}/{version}`: a prompt template version.
- `qc://style/preset-display`: the house style for showing a preset in chat (one table per guitar, separate Scene A / Scene B columns, estimate marker, rationales and warnings, then refine/save). The server `instructions` carry a short version of it.

**Prompt:** `design_tone` takes an optional song, artist, guitar and notes, asks Claude to clarify missing tone details, then calls `generate_preset`.

Full design: [docs/mcp/DESIGN.md](docs/mcp/DESIGN.md).

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
