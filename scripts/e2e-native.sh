#!/usr/bin/env bash
# Run the mock-mode Playwright suite against a natively built server, for machines where
# Docker can't run (e.g. a cloud sandbox). Docker Compose (run-mock.sh) stays the usual
# local route; this does the same with `moto` standing in for MinIO.
#
#   scripts/e2e-native.sh                     # every mock-mode spec
#   scripts/e2e-native.sh search.spec.js      # extra args go to `playwright test`
#
# Needs Go, Python 3 (for moto, installed into a venv on first use) and Node/npm. Specs run
# from a copy under $E2E_WORK, so screenshots and videos don't land in the repo. The
# live_* / real_gemini / builder_statement_check specs call Gemini and are skipped.
#
# Environment: E2E_WORK (scratch dir, default a fresh temp dir), PORT (8080), S3_PORT
# (9000), PW_CHROMIUM_PATH (a preinstalled Chromium; found automatically under
# /opt/pw-browsers if unset), MOTO_VENV (reuse an existing venv with moto installed),
# E2E_SEED=1 (load the synthetic presets in testdata/e2e_presets first, for
# data_grid.spec.js; off by default because extra presets change what the library-list
# specs see), SCREENSHOT_DIR (where data_grid.spec.js writes its screenshots).
set -euo pipefail

ROOT=$(cd "$(dirname "$0")/.." && pwd)
WORK=${E2E_WORK:-$(mktemp -d)}
PORT=${PORT:-8080}
S3_PORT=${S3_PORT:-9000}
MOTO_VENV=${MOTO_VENV:-$WORK/venv}
BUCKET=sound-builder-local
mkdir -p "$WORK"

if [ -z "${PW_CHROMIUM_PATH:-}" ]; then
  PW_CHROMIUM_PATH=$(ls -d /opt/pw-browsers/chromium-*/chrome-linux64/chrome 2>/dev/null | head -1 || true)
fi
export PW_CHROMIUM_PATH

pids=()
cleanup() { for p in "${pids[@]}"; do kill "$p" 2>/dev/null || true; done; }
trap cleanup EXIT

# A server or moto left over from an earlier run would answer instead of this run's,
# with its own build and data. Refuse rather than test the wrong thing.
for port in "$PORT" "$S3_PORT"; do
  if curl -s -o /dev/null "http://127.0.0.1:$port"; then
    echo "port $port is already in use; stop whatever is listening there first" >&2
    exit 1
  fi
done

echo "== build server"
(cd "$ROOT" && go build -o "$WORK/server" ./cmd/server)

echo "== S3 stand-in (moto) on :$S3_PORT"
if [ ! -x "$MOTO_VENV/bin/moto_server" ]; then
  python3 -m venv "$MOTO_VENV"
  "$MOTO_VENV/bin/pip" install --quiet "moto[server]"
fi
"$MOTO_VENV/bin/moto_server" -p "$S3_PORT" >"$WORK/moto.log" 2>&1 &
pids+=($!)
for _ in $(seq 40); do curl -s -o /dev/null "http://127.0.0.1:$S3_PORT" && break; sleep 0.25; done
"$MOTO_VENV/bin/python" - "$S3_PORT" "$BUCKET" <<'PY'
import sys, boto3
boto3.client("s3", endpoint_url=f"http://127.0.0.1:{sys.argv[1]}", aws_access_key_id="test",
             aws_secret_access_key="test", region_name="us-east-1").create_bucket(Bucket=sys.argv[2])
PY

echo "== server (mock mode) on :$PORT"
# exec, so the recorded pid is the server itself and cleanup stops it.
(cd "$ROOT" && exec env -u GEMINI_API_KEY STORAGE_BACKEND=s3 S3_ENDPOINT="http://127.0.0.1:$S3_PORT" \
  S3_BUCKET="$BUCKET" S3_ACCESS_KEY_ID=test S3_SECRET_ACCESS_KEY=test S3_REGION=us-east-1 \
  GCS_BUCKET_NAME="$BUCKET" MOCK_MODE=true MOCK_PASSWORD=bluesmusic PORT="$PORT" \
  "$WORK/server" >"$WORK/server.log" 2>&1) &
pids+=($!)
for _ in $(seq 60); do curl -s -o /dev/null "http://127.0.0.1:$PORT/login" && break; sleep 0.25; done

if [ "${E2E_SEED:-}" = "1" ]; then
  echo "== seed testdata/e2e_presets"
  (cd "$ROOT" && STORAGE_BACKEND=s3 S3_ENDPOINT="http://127.0.0.1:$S3_PORT" S3_BUCKET="$BUCKET" \
    S3_ACCESS_KEY_ID=test S3_SECRET_ACCESS_KEY=test S3_REGION=us-east-1 go run ./cmd/e2e_seed)
fi

echo "== playwright (specs copied to $WORK/specs)"
rm -rf "${WORK:?}/specs"
mkdir -p "$WORK/specs"
(cd "$ROOT/tests/e2e" && tar --exclude=node_modules --exclude=test-results -cf - .) | (cd "$WORK/specs" && tar -xf -)
cd "$WORK/specs"
npm ci --silent
specs=("$@")
if [ ${#specs[@]} -eq 0 ]; then
  mapfile -t specs < <(ls *.spec.js | grep -v -E '^(live_|real_gemini|builder_statement_check)')
fi
set +e
BASE_URL="http://localhost:$PORT" npx playwright test --reporter=list "${specs[@]}"
status=$?
set -e
echo "== server log: $WORK/server.log"
exit $status
