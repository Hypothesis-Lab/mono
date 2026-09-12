#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")"

PKG="./cmd/app"

export PORT="8000"
export DATABASE_URL="postgres://postgres:postgres@localhost:5433/app?sslmode=disable"
export JWT_SECRET="dev-secret-change-in-production"
export S3_ENDPOINT="http://localhost:8333"
export S3_REGION="us-east-1"
export S3_BUCKET="my-bucket"
export S3_ACCESS_KEY_ID="admin"
export S3_SECRET_ACCESS_KEY="secret"
export DEV_LOGGER="true"

exec watchexec -r \
  --debounce 200ms \
  -- go run "$PKG"
