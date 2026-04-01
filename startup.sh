#!/bin/bash
# startup.sh - Local development startup script

echo "===================================================="
echo "          Running Pre-flight Unit Tests             "
echo "===================================================="

# Run tests in verbose mode and capture logs into startup_test_logs.txt
echo "Spinning up test suite (this will take a moment)..."
go test -v ./... > startup_test_logs.txt 2>&1
TEST_EXIT_CODE=$?

if [ $TEST_EXIT_CODE -ne 0 ]; then
    echo ""
    echo "⚠️ WARNING: Pre-flight tests executed with failures or errors."
    echo "----------------------------------------------------"
    echo "Failing Tests / Build Errors Found:"
    
    # 1. Catch individual unit test failures
    if grep -q "^--- FAIL" startup_test_logs.txt; then
        grep "^--- FAIL" startup_test_logs.txt | sed 's/^--- FAIL: //' | while read -r line; do
            echo "  ❌ Test: $line"
        done
    fi
    
    # 2. Catch package compilation/build failures
    if grep -q "\[build failed\]" startup_test_logs.txt; then
        grep "\[build failed\]" startup_test_logs.txt | while read -r line; do
            pkg=$(echo "$line" | awk '{print $2}')
            echo "  ❌ Build Error in: $pkg"
        done
    fi

    # 3. Catch generic test run failures (timeouts, panics)
    if grep -q "^FAIL\s" startup_test_logs.txt; then
        grep "^FAIL\s" startup_test_logs.txt | grep -v "\[build failed\]" | while read -r line; do
            pkg=$(echo "$line" | awk '{print $2}')
            echo "  ❌ Package Failure: $pkg"
        done
    fi
    
    echo "----------------------------------------------------"
    echo "Review startup_test_logs.txt for the full stack trace."
    echo "Proceeding with server startup..."
    echo ""
else
    echo ""
    echo "✅ All pre-flight tests completed successfully!"
    echo ""
fi

# Set default development port if unmapped
export PORT=${PORT:-8082}

if [ -f .local_config ]; then
    source .local_config
fi

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
