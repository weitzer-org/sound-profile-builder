# QC-2 Multi-Agent Modeler

This repository holds the Go-based Multi-Agent system that orchestrates Google Gemini Phase-Agents (Tone Historian, CorOS Librarian, FOH Optimizer, etc.) to evaluate and build physics-accurate Quad Cortex DSP matrices.

## 🚀 Deployment (Google Cloud)
Because of internal MacOS constraints (`Santa` restricting `go build` binaries in randomized temp `/tmp` folders), we deploy the application dynamically to **Google Cloud Build** and **Cloud Run** for isolated, secure execution and testing. 

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
