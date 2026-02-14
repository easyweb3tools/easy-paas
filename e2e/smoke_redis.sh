#!/bin/sh
set -e

# Optional: validates Redis cache backend.
# If docker isn't available, this script exits 0 (skip).

if ! command -v docker >/dev/null 2>&1; then
  echo "skip (docker not found)"
  exit 0
fi

ROOT="$(cd "$(dirname "$0")/.." && pwd)"

# Start redis
REDIS_PORT=18084
CID=$(docker run -d --rm -p ${REDIS_PORT}:6379 redis:7-alpine)

cleanup() {
  docker stop "$CID" >/dev/null 2>&1 || true
  kill $PL_PID >/dev/null 2>&1 || true
}
trap cleanup EXIT

export EASYWEB3_BOOTSTRAP_ADMIN_API_KEY="ew3_admin_dev"
export EASYWEB3_JWT_SECRET="dev-secret-change-me-please"
export EASYWEB3_LISTEN=":18085"
export EASYWEB3_CACHE_BACKEND="redis"
export EASYWEB3_CACHE_DEFAULT_TTL="30s"
export EASYWEB3_REDIS_ADDR="127.0.0.1:${REDIS_PORT}"

cd "$ROOT/easyweb3-platform"

go run ./cmd/platform >/tmp/ew3-platform-redis.log 2>&1 &
PL_PID=$!

sleep 0.7

cd "$ROOT/easyweb3-cli"

go build -o /tmp/easyweb3 .

/tmp/easyweb3 --api-base http://127.0.0.1:18085 auth login --api-key ew3_admin_dev >/dev/null

/tmp/easyweb3 --api-base http://127.0.0.1:18085 cache put --key foo --value '{"bar":2}' --ttl-seconds 60 >/dev/null
/tmp/easyweb3 --api-base http://127.0.0.1:18085 cache get foo >/dev/null

echo "smoke redis ok"
