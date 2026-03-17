#!/usr/bin/env bash
set -euo pipefail

# Start Go API server in background with dev mode enabled.
# If it fails (e.g. auth error), we exit before launching Vite.
REDTEAM_DEV=1 go run ./cmd/develop redteam setup --experimental &
GO_PID=$!

# Give the server a moment to either start or fail.
trap 'kill $GO_PID 2>/dev/null' EXIT

# Wait for the server to be ready or exit.
for _ in $(seq 1 30); do
  if ! kill -0 "$GO_PID" 2>/dev/null; then
    wait "$GO_PID"
    exit $?
  fi
  if curl -sf http://127.0.0.1:8484/api/config > /dev/null 2>&1; then
    break
  fi
  sleep 0.1
done

cd web && npx vite dev --open
