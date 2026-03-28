#!/bin/bash
# startup.sh - Local development startup script

echo "===================================================="
echo "          Running Pre-flight Unit Tests             "
echo "===================================================="

# Run tests silently and capture logs into startup_test_logs.txt
echo "Spinning up test suite (this will take a moment)..."
go test ./... > startup_test_logs.txt 2>&1
TEST_EXIT_CODE=$?

if [ $TEST_EXIT_CODE -ne 0 ]; then
    echo ""
    echo "⚠️ WARNING: Unit tests executed with failures or errors."
    echo "Review startup_test_logs.txt for details."
    echo "Proceeding with server startup..."
    echo ""
else
    echo ""
    echo "✅ Unit tests completed successfully!"
    echo ""
fi

# Set default development port if unmapped
export PORT=${PORT:-8082}

echo "Checking if port $PORT is in use..."
PID=$(lsof -ti:$PORT)
if [ ! -z "$PID" ]; then
    echo "Port $PORT is currently in use by PID $PID. Shutting it down..."
    kill -9 $PID
    sleep 1
    echo "Port $PORT successfully freed."
fi

echo "===================================================="
echo "    Starting QC-2 Backend Server on Port $PORT      "
echo "    Running at: http://localhost:$PORT              "
echo "===================================================="
go run cmd/server/main.go
