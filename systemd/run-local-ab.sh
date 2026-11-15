#!/usr/bin/env bash
set -euo pipefail

root=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)
database_url='postgres://postgres:postgres@127.0.0.1:5433/hypothesis_ab?sslmode=disable'
go_bin=$(command -v go)
npm_bin=$(command -v npm)
env_bin=$(command -v env)
runtime="${XDG_DATA_HOME:-$HOME/.local/share}/hypothesislab-ab/runtime.conf"

test -f "$root/.env"
test -f "$runtime"
test -x "$root/generator/.venv/bin/python"
test -f "$root/frontend/dist/index.html"

systemd-run --user --collect --unit=startup-ab-backend \
  --property="WorkingDirectory=$root/backend" \
  --property="EnvironmentFile=$root/.env" \
  --property="EnvironmentFile=$runtime" \
  "$env_bin" PORT=8000 DATABASE_URL="$database_url" \
  JWT_SECRET=local-ab-development-only \
  S3_ENDPOINT=http://127.0.0.1:8333 S3_REGION=us-east-1 S3_BUCKET=my-bucket \
  S3_ACCESS_KEY_ID=admin S3_SECRET_ACCESS_KEY=secret \
  GENERATOR_URL=http://127.0.0.1:8001 TEMPORAL_ADDRESS=127.0.0.1:7233 \
  DEV_LOGGER=true WRITE_TIMEOUT=30m \
  "$go_bin" run ./cmd/app

systemd-run --user --collect --unit=startup-ab-generator \
  --property="WorkingDirectory=$root/generator" \
  --property="EnvironmentFile=$root/.env" \
  --property="EnvironmentFile=$runtime" \
  "$env_bin" DATABASE_URL="$database_url" HOST=127.0.0.1 PORT=8001 \
  WORKER_CONCURRENCY=2 GRAPH_MAX_CONCURRENCY=2 \
  CODEX_MODEL=gpt-5.6-luna CODEX_REASONING_EFFORT=low \
  CODEX_TIMEOUT_SECONDS=300 CODEX_MAX_RETRIES=2 \
  "$root/generator/.venv/bin/python" -m app.cmd.server

systemd-run --user --collect --unit=startup-ab-frontend \
  --property="WorkingDirectory=$root/frontend" \
  "$env_bin" VITE_API_PROXY_TARGET=http://127.0.0.1:8000 \
  "$npm_bin" run preview -- --host 127.0.0.1 --port 5175
