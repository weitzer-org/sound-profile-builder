#!/bin/bash
set -e

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

# Make sure port 8082 is cleared
echo "Checking if port 8082 is in use..."
if lsof -t -i:8082 >/dev/null 2>&1; then
    echo "Port 8082 is busy. Ousting current process..."
    kill -9 $(lsof -t -i:8082) || true
    sleep 1
fi

echo "===================================================="
echo "    Starting QC-2 Backend Server (Mobile Mode)     "
echo "    Running at: http://localhost:8082/?mode=standalone "
echo "===================================================="

# Launch the Go server process in the background (Live API mode, zero MOCK_MODE environment variables!)
MOCK_PASSWORD="bluesmusic" PORT=8082 go run cmd/server/main.go &
SERVER_PID=$!

# Trap exits to kill background Go process cleanly upon ctrl+c interrupts
trap "echo -e '\nStopping server process...'; kill -9 $SERVER_PID 2>/dev/null || true" EXIT INT TERM

# Wait-poll until the Go port binding completes successfully
while ! (echo > /dev/tcp/127.0.0.1/8082) >/dev/null 2>&1; do
    sleep 0.5
done

# Launch system default browser to mobile persistent PWA URL
if command -v xdg-open > /dev/null; then
    xdg-open "http://localhost:8082/?mode=standalone" >/dev/null 2>&1
elif command -v open > /dev/null; then
    open "http://localhost:8082/?mode=standalone" >/dev/null 2>&1
else
    echo "🚀 Server is live! Please open your browser and navigate to:"
    echo "👉 http://localhost:8082/?mode=standalone"
fi

# Stream Go server log pipes directly to terminal console interactive block
wait $SERVER_PID
