#!/usr/bin/env bash
# Run the full QC-2 stack (app + MinIO) in LIVE mode.
# Calls the real Gemini pipeline, so a valid GEMINI_API_KEY must be set in .env.
set -euo pipefail
cd "$(dirname "$0")"

[ -f .env ] || cp .env.example .env

# Guard: refuse to start live without a real key. Read the effective (last)
# GEMINI_API_KEY line and tolerate surrounding whitespace.
KEY=$(grep -E '^[[:space:]]*GEMINI_API_KEY[[:space:]]*=' .env | tail -1 | sed -E 's/^[^=]*=[[:space:]]*//; s/[[:space:]]*$//')
if [ -z "$KEY" ] || printf '%s' "$KEY" | grep -q 'replace-with'; then
  echo "ERROR: set a real GEMINI_API_KEY in .env before running LIVE mode." >&2
  echo "       (Get one from Google AI Studio — no GCP project needed.)" >&2
  exit 1
fi

export MOCK_MODE=false

echo "=================================================="
echo " QC-2 — LIVE mode (real Gemini pipeline)"
echo " App:          http://localhost:8080"
echo " MinIO console: http://localhost:9001 (minioadmin/minioadmin)"
echo " Login with the MOCK_PASSWORD from your .env"
echo "=================================================="

exec docker compose up --build "$@"
