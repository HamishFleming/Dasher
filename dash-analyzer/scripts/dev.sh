#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
run_dir=$(mktemp -d /tmp/dash-analyzer-dev.XXXXXX)
backend_pid=''
frontend_pid=''
cleanup() {
  trap - EXIT INT TERM
  if [[ -n "$frontend_pid" ]]; then kill "$frontend_pid" 2>/dev/null || true; fi
  if [[ -n "$backend_pid" ]]; then kill "$backend_pid" 2>/dev/null || true; fi
  wait 2>/dev/null || true
  rm -rf -- "$run_dir"
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM
go build -o "$run_dir/server" ./cmd/server
"$run_dir/server" &
backend_pid=$!
export GO_API_URL="${GO_API_URL:-http://127.0.0.1:${PORT:-8080}}"
export NEXT_TELEMETRY_DISABLED=1
echo "Next.js dashboard http://127.0.0.1:${FRONTEND_PORT:-3000}"
node frontend/node_modules/next/dist/bin/next dev frontend --hostname 127.0.0.1 --port "${FRONTEND_PORT:-3000}" &
frontend_pid=$!
wait -n "$backend_pid" "$frontend_pid"
