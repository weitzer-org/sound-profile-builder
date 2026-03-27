#!/bin/bash
# startup.sh - Local development startup script

echo "===================================================="
echo "          Running Pre-flight Unit Tests             "
echo "===================================================="

# Run test coverage and capture logs into startup_test_logs.txt
go test -v ./... 2>&1 | tee startup_test_logs.txt
TEST_EXIT_CODE=${PIPESTATUS[0]}

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

echo "===================================================="
echo "    Starting QC-2 Backend Server on Port $PORT      "
echo "===================================================="
go run cmd/server/main.go
