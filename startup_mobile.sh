#!/usr/bin/env bash
# Run the full QC-2 stack (app + MinIO) on port 8082, for mobile/PWA testing.
# Uses the same Docker+MinIO flow as run-mock.sh/run-live.sh, remapped to 8082
# via docker-compose.mobile.yml and run under a separate Compose project name
# (sound-profile-builder-mobile) so it's a genuinely independent stack that
# can run at the same time as a normal run-mock.sh/run-live.sh instance on
# 8080, not just a reconfiguration of it.
#
# Usage:
#   ./startup_mobile.sh          MOCK mode: canned pipeline, no Gemini key needed
#   ./startup_mobile.sh --live   LIVE mode: calls the real Gemini pipeline
set -euo pipefail
cd "$(dirname "$0")"

[ -f .env ] || cp .env.example .env

LIVE_MODE=false
if [ "${1:-}" = "--live" ]; then
    LIVE_MODE=true
    shift
fi

echo "===================================================="
echo "          Running Pre-flight Unit Tests             "
echo "===================================================="
echo "Spinning up test suite (this will take a moment)..."

if go test ./... >/dev/null 2>&1; then
    echo "✅ All pre-flight tests completed successfully!"
else
    echo "⚠️ WARNING: Pre-flight tests executed with failures or errors."
    echo "Proceeding with server startup anyway..."
fi

echo "Checking if port 8082 is in use..."
if lsof -t -i:8082 >/dev/null 2>&1; then
    echo "Port 8082 is busy. Bringing down any existing mobile compose stack..."
    docker compose -p sound-profile-builder-mobile -f docker-compose.yml -f docker-compose.mobile.yml down 2>/dev/null || true
fi

if [ "$LIVE_MODE" = true ]; then
    # Guard: refuse to start live without a real key (same check as run-live.sh).
    KEY=$(grep -E '^[[:space:]]*GEMINI_API_KEY[[:space:]]*=' .env | tail -1 | sed -E 's/^[^=]*=[[:space:]]*//; s/[[:space:]]*$//')
    if [ -z "$KEY" ] || printf '%s' "$KEY" | grep -q 'replace-with'; then
        echo "ERROR: set a real GEMINI_API_KEY in .env before running LIVE mode." >&2
        echo "       (Get one from Google AI Studio — no GCP project needed.)" >&2
        exit 1
    fi
    export MOCK_MODE=false
    echo "===================================================="
    echo "  Starting QC-2 Backend Server (Mobile Mode, LIVE)  "
    echo "    Running at: http://localhost:8082/?mode=standalone "
    echo "===================================================="
else
    export MOCK_MODE=true
    echo "===================================================="
    echo "    Starting QC-2 Backend Server (Mobile Mode)     "
    echo "    Running at: http://localhost:8082/?mode=standalone "
    echo "===================================================="
fi

# Open the browser once the port is accepting connections.
(
    while ! (echo > /dev/tcp/127.0.0.1/8082) >/dev/null 2>&1; do
        sleep 0.5
    done
    if command -v xdg-open > /dev/null; then
        xdg-open "http://localhost:8082/?mode=standalone" >/dev/null 2>&1
    elif command -v open > /dev/null; then
        open "http://localhost:8082/?mode=standalone" >/dev/null 2>&1
    else
        echo "🚀 Server is live! Please open your browser and navigate to:"
        echo "👉 http://localhost:8082/?mode=standalone"
    fi
) &

exec docker compose -p sound-profile-builder-mobile -f docker-compose.yml -f docker-compose.mobile.yml up --build
