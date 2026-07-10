#!/usr/bin/env bash
# Run the full QC-2 stack (app + MinIO) in MOCK mode on port 8082, for
# mobile/PWA testing. Uses the same Docker+MinIO flow as run-mock.sh, just
# remapped to 8082 via docker-compose.mobile.yml so it can run alongside a
# normal run-mock.sh instance on 8080.
set -euo pipefail
cd "$(dirname "$0")"

[ -f .env ] || cp .env.example .env

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
    docker compose -f docker-compose.yml -f docker-compose.mobile.yml down 2>/dev/null || true
fi

echo "===================================================="
echo "    Starting QC-2 Backend Server (Mobile Mode)     "
echo "    Running at: http://localhost:8082/?mode=standalone "
echo "===================================================="

export MOCK_MODE=true

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

exec docker compose -f docker-compose.yml -f docker-compose.mobile.yml up --build
