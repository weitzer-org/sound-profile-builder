#!/bin/bash
# api_e2e_tester.sh - Test running server endpoints via cURL without a browser in jetski

PORT=8081
URL="http://localhost:$PORT"

echo "===================================================="
echo "    Headless cURL API End-to-End Test Suite        "
echo "===================================================="

# 0. Start temporary mock server in background
function cleanup {
    echo ""
    echo "🧹 Cleaning up background server on port $PORT..."
    TEST_PID=$(lsof -ti:$PORT)
    if [ ! -z "$TEST_PID" ]; then
        kill -9 $TEST_PID
        echo "✅ Port $PORT freed."
    fi
    rm -f cookies.txt
}

# Trap exit to ensure cleanup runs even if script fails or is killed
trap cleanup EXIT

# 0. Pre-clean port 8081 in case of previous run leftovers
cleanup

echo "🚀 Starting temporary mock server on port $PORT..."

# Wait for server to bind
echo "⏳ Waiting for server to bind..."
for i in {1..20}; do
    if curl -s -f "$URL/login" > /dev/null; then
        echo "✅ Server bound and responding!"
        break
    fi
    if [ $i -eq 20 ]; then
        echo "❌ Server failed to bind in time."
        exit 1
    fi
    sleep 2
done

PASSWORD=${MOCK_PASSWORD:-"bluesmusic"}

# 1. Test Login to get Session Cookie
echo ""
echo "🔐 1. Testing Login..."
rm -f cookies.txt

# Store cookies in cookies.txt
curl -s -i -X POST -H "Content-Type: application/x-www-form-urlencoded" \
     -d "password=$PASSWORD" \
     "$URL/login" -c cookies.txt > /dev/null

if [ ! -f cookies.txt ] || [ -z "$(cat cookies.txt | grep session)" ]; then
    echo "❌ Login Failed (No session cookie saved or invalid)."
    exit 1
fi

echo "✅ Login Successful! Session cookie saved."

# 2. Test Get Presets (Authenticated)
echo ""
echo "📂 2. Testing /api/presets (Preset Library list)..."
RESPONSE=$(curl -s -b cookies.txt "$URL/api/presets")

if [[ "$RESPONSE" == *"No presets saved yet"* ]] || [[ "$RESPONSE" == *"Saved"* ]]; then
    echo "✅ /api/presets reachable and responsive!"
else
    echo "❌ /api/presets failed to load expected content context: $(echo "$RESPONSE" | head -n 5)"
fi

# 3. Test Generate Prompt
echo ""
echo "⚡ 3. Testing /api/preset/generate (Create Draft)..."
PROMPT_FORM="prompt=Fender Twin Reverb authentic tone"
GEN_RESPONSE=$(curl -s -b cookies.txt -H "Accept: text/html" -X POST -H "Content-Type: application/x-www-form-urlencoded" \
    -d "$PROMPT_FORM" "$URL/api/preset/generate")

if [[ "$GEN_RESPONSE" == *"Finalize Save"* ]]; then
    echo "✅ /api/preset/generate returned draft workspace view!"
else
    echo "❌ /api/preset/generate failed to return draft content."
    exit 1
fi

# Parse Draft ID from output hidden form
DRAFT_ID=$(echo "$GEN_RESPONSE" | grep -o 'name="id" value="[^"]*' | head -n 1 | cut -d'"' -f4)

if [ -z "$DRAFT_ID" ]; then
    echo "❌ Could not parse Draft ID from response!"
    exit 1
fi

echo "Parsed Draft ID: $DRAFT_ID"

# 4. Test Finalize Save (Rename Draft Preset)
echo ""
echo "💾 4. Testing /api/preset/rename (Finalize Save)..."
RENAME_FORM="id=$DRAFT_ID&preset_name=E2E Test Tone"
RENAME_RESPONSE=$(curl -s -b cookies.txt -H "Accept: text/html" -X POST -H "Content-Type: application/x-www-form-urlencoded" \
    -d "$RENAME_FORM" "$URL/api/preset/rename")

if [[ "$RENAME_RESPONSE" == *"E2E Test Tone"* ]]; then
    echo "✅ /api/preset/rename successfully converted draft to saved preset!"
else
    echo "❌ /api/preset/rename failed to convert draft. Response: $(echo "$RENAME_RESPONSE" | head -n 5)"
fi

# 5. Test Chat Refinement
echo ""
echo "💬 5. Testing /api/preset/chat (Refinement)..."
CHAT_FORM="id=$DRAFT_ID&message=make it warmer"
CHAT_RESPONSE=$(curl -s -b cookies.txt -H "Accept: text/html" -X POST -H "Content-Type: application/x-www-form-urlencoded" \
    -d "$CHAT_FORM" "$URL/api/preset/chat")

if [[ "$CHAT_RESPONSE" == *"Live DSP Matrix"* ]]; then
    echo "✅ /api/preset/chat successfully refined tone!"
else
    echo "❌ /api/preset/chat failed to load refinement workspace."
fi

# 7. Test Copy / Duplicate (CUJ 4)
echo ""
echo "📋 7. Testing /api/preset/copy (Duplication)..."
DUPLICATE_FORM="id=$DRAFT_ID&new_name=Duplicate E2E Tone"
DUPLICATE_RESPONSE=$(curl -s -b cookies.txt -H "Accept: text/html" -X POST -H "Content-Type: application/x-www-form-urlencoded" \
    -d "$DUPLICATE_FORM" "$URL/api/preset/copy")

if [[ "$DUPLICATE_RESPONSE" == *"Duplicate E2E Tone"* ]]; then
    echo "✅ /api/preset/copy successfully duplicated preset!"
else
    echo "❌ /api/preset/copy failed to duplicate."
fi

# 6. Test Two-Stage Delete
echo ""
echo "🗑️ 6. Testing /api/preset/delete (Two-Stage)..."
# We simulate the final stage click passing the specific ID
DELETE_FORM="id=$DRAFT_ID"
DELETE_RESPONSE=$(curl -s -b cookies.txt -H "Accept: text/html" -X POST -H "Content-Type: application/x-www-form-urlencoded" \
    -d "$DELETE_FORM" "$URL/api/preset/delete")

if [[ "$DELETE_RESPONSE" != *"$DRAFT_ID"* ]]; then
    echo "✅ /api/preset/delete successfully removed preset from view!"
else
    echo "❌ /api/preset/delete failed to remove preset."
fi

echo ""
echo "===================================================="
echo "    Comprehensive Headless Test Suite Completed     "
echo "===================================================="
rm -f cookies.txt
