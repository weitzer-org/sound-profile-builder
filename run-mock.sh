#!/usr/bin/env bash
# Run the full QC-2 stack (app + MinIO) in MOCK mode.
# The 12-agent pipeline returns canned data, so NO Gemini API key is needed.
set -euo pipefail
cd "$(dirname "$0")"

[ -f .env ] || cp .env.example .env

export MOCK_MODE=true

echo "=================================================="
echo " QC-2 — MOCK mode (no Gemini key required)"
echo " App:          http://localhost:8080"
echo " MinIO console: http://localhost:9001 (minioadmin/minioadmin)"
echo " Login with the MOCK_PASSWORD from your .env"
echo "=================================================="

exec docker compose up --build "$@"
