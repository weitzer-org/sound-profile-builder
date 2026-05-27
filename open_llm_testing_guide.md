# 🪐 Open-LLM Gateway Developer Integration & Testing Guide

This guide describes how to run and verify the new **Open-LLM Gateway** backend target in the `sound-profile-builder` multi-agent pipeline. 

The sound builder treats the Open-LLM Gateway as a completely external service (just like the Gemini API) with zero dependencies on local paths or background secure tunnels in production.

---

## ⚙️ Configuration Variables Reference

The sound builder loads the following optional environment parameters at startup when the Open-LLM model backend is enabled:

| Parameter | Type | Default Value | Purpose / Notes |
| :--- | :--- | :--- | :--- |
| `USE_OPENLLM` | `Boolean` | `false` | Set to `true` to shift all agent generations to the OpenAI-compatible REST endpoint. |
| `OPENLLM_API_URL` | `String` | `https://open-llm-gateway-710019748844.us-central1.run.app/v1` | Base REST endpoint targeting `/chat/completions`. |
| `OPENLLM_API_KEY` | `String` | *(Resolved dynamically from Secret Manager)* | Pre-shared Master Bearer token for gateway auth. |
| `OPENLLM_MODEL` | `String` | `active-model` | Default active open model (e.g., `qwen-2.5-7b`, `llama-3.1-8b`). |
| `OPENLLM_TIMEOUT` | `Duration` | `10m` | Connection wait limit to prevent timeouts during scale-to-zero cold boots! |

---

## 🛰️ Sandbox Option A: Direct Cloud Run Target (No Local Tunnel or Gateway Needed!)

Because the Cloud Run service allows direct access at the IAM boundary and is secured at the application layer via a pre-shared token, **you can query the live L4 GPU pipeline directly from your local system without running any local tunnels or loopback gateways!**

Additionally, the sound builder automatically resolves security credentials dynamically from Google Secret Manager if they aren't supplied in your environment:

```bash
cd sound-profile-builder

# Enable the Open-LLM backend (URL & Keys resolve dynamically from GCP by default!)
export USE_OPENLLM="true"
export MOCK_PASSWORD="bluesmusic"

# Launch the server
go run cmd/server/main.go
```

*That's it! Your local sound builder server will load the master token from Secret Manager under the name `open-llm-api-auth-secret` and route requests directly to the live Cloud Run endpoint at `https://open-llm-gateway-710019748844.us-central1.run.app/v1`.*

---

## 🔌 Sandbox Option B: 100% Offline Testing (Zero Cloud Cost)

If you are working offline or want to test the full 12-agent pipeline locally without incurring live Cloud GPU startup times or token costs:

### Step 1: Spin up the Offline Mock vLLM Server
This service simulates streaming a standard LLM completion at ~25 tokens per second on port `8000`:
```bash
python3 path/to/mock_vllm.py
```

### Step 2: Run the Go API Gateway
Run the lightweight Go API Gateway locally on port `8080`, configured to forward requests to the mock vLLM loopback:
```bash
cd open-llm/gateway
PORT=8080 API_AUTH_SECRET="open-llm-dev-token" DEV_MODE="true" VLLM_API_URL="http://localhost:8000" go run main.go
```

### Step 3: Run the Sound Builder Server
Boot the main Go dashboard backend pointed to your offline gateway target:
```bash
cd sound-profile-builder
export USE_OPENLLM="true"
export OPENLLM_API_URL="http://localhost:8080/v1"
export OPENLLM_API_KEY="open-llm-dev-token"
export OPENLLM_MODEL="mock-qwen-model"
export MOCK_PASSWORD="bluesmusic"
go run cmd/server/main.go
```

---

## 🎨 Dynamic Per-Agent Model Overrides

You can optimize performance and context limits across the 12-agent orchestration framework by targeting specific models for specific tasks. 

For example, you can assign heavy physics mapping logic to large model sizes (like a 72B parameter instruction set) while routing format and cleanup tasks to fast/lightweight models (like an 8B parameter set).

To set this up, update the `agent_models` mapping in the main server configuration file (`config.json`):

```json
{
  "single_amp_mode": true,
  "allow_cloud_captures": false,
  "allow_factory_captures": true,
  "favor_captures": false,
  "allow_paid_plugins": true,
  "available_plugins": [
    "Cory Wong"
  ],
  "guitars": [
    "Gibson ES-339",
    "Fender Telecaster American Pro II"
  ],
  "project_id": "710019748844",
  "bucket_name": "weitzer-sound-builder",
  "agent_prompts": {
    "2_sonic_profiler": "v2",
    "4_coros_librarian": "v2",
    "6_acoustician": "v2",
    "12_architect": "v2"
  },
  "agent_models": {
    "1_tone_historian": "qwen-2.5-72b-instruct",
    "2_sonic_profiler": "qwen-2.5-8b-instruct",
    "4_coros_librarian": "qwen-2.5-72b-instruct",
    "12_architect": "qwen-2.5-72b-instruct"
  }
}
```

When the sound builder pipeline executes, any task listed in `"agent_models"` will request that specific model parameter from the Open-LLM Gateway, falling back to standard values for other nodes.
