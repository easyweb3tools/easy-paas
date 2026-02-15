#!/bin/sh
set -e

ROOT="$(cd "$(dirname "$0")/.." && pwd)"

python3 "$ROOT/e2e/dummy_polymarket.py" >/tmp/ew3-polymarket.log 2>&1 &
UP_PID=$!
python3 "$ROOT/e2e/dummy_webhook.py" >/tmp/ew3-webhook.log 2>&1 &
WH_PID=$!
python3 "$ROOT/e2e/dummy_dexscreener.py" >/tmp/ew3-dex.log 2>&1 &
DX_PID=$!
python3 "$ROOT/e2e/dummy_goplus.py" >/tmp/ew3-goplus.log 2>&1 &
GP_PID=$!

cleanup() {
  kill $UP_PID >/dev/null 2>&1 || true
  kill $WH_PID >/dev/null 2>&1 || true
  kill $DX_PID >/dev/null 2>&1 || true
  kill $GP_PID >/dev/null 2>&1 || true
  kill $PL_PID >/dev/null 2>&1 || true
}
trap cleanup EXIT

export EASYWEB3_BOOTSTRAP_ADMIN_API_KEY="ew3_admin_dev"
export EASYWEB3_JWT_SECRET="dev-secret-change-me-please"
export EASYWEB3_LISTEN=":18080"
export EASYWEB3_SERVICES_JSON='{"polymarket":{"base_url":"http://127.0.0.1:18081","health_path":"/healthz","docs_path":"/docs"}}'
export EASYWEB3_NOTIFY_FILE="/tmp/ew3-notify-config.json"
export EASYWEB3_DEXSCREENER_BASE_URL="http://127.0.0.1:18083"
export EASYWEB3_GOPLUS_BASE_URL="http://127.0.0.1:18087"
export EASYWEB3_CACHE_BACKEND="memory"
export EASYWEB3_CACHE_DEFAULT_TTL="30s"
export EASYWEB3_DOCS_DIR="$ROOT/docs"

cd "$ROOT/easyweb3-platform"

go run ./cmd/platform >/tmp/ew3-platform.log 2>&1 &
PL_PID=$!

sleep 0.7

cd "$ROOT/easyweb3-cli"

go build -o /tmp/easyweb3 .

# Login (bootstrap admin key => token)
/tmp/easyweb3 --api-base http://127.0.0.1:18080 auth login --api-key ew3_admin_dev >/dev/null

# Cache basic
/tmp/easyweb3 --api-base http://127.0.0.1:18080 cache put --key foo --value '{"bar":1}' --ttl-seconds 60 >/dev/null
/tmp/easyweb3 --api-base http://127.0.0.1:18080 cache get foo >/dev/null
/tmp/easyweb3 --api-base http://127.0.0.1:18080 cache delete foo >/dev/null

# Create a log
/tmp/easyweb3 --api-base http://127.0.0.1:18080 log create --action trade_executed --details '{"token":"PEPE2","type":"BUY"}' >/dev/null

# List logs
/tmp/easyweb3 --api-base http://127.0.0.1:18080 log list --limit 5 >/dev/null

# Service health/docs
/tmp/easyweb3 --api-base http://127.0.0.1:18080 service health --name polymarket >/dev/null
/tmp/easyweb3 --api-base http://127.0.0.1:18080 service docs --name polymarket >/dev/null

# Raw API proxy call
/tmp/easyweb3 --api-base http://127.0.0.1:18080 api raw --service polymarket --method GET --path /healthz >/dev/null

# Public docs
/tmp/easyweb3 --api-base http://127.0.0.1:18080 docs get architecture >/dev/null
/tmp/easyweb3 --api-base http://127.0.0.1:18080 docs get picoclaw >/dev/null

# Configure notifications for current project (bootstrap admin project = platform)
/tmp/easyweb3 --api-base http://127.0.0.1:18080 notify config put \
  --body '{"channels":[{"type":"webhook","url":"http://127.0.0.1:18082/hook","events":["*"]}]}' \
  >/dev/null

# Broadcast notification
/tmp/easyweb3 --api-base http://127.0.0.1:18080 notify broadcast --message "trade ok" --event trade_executed >/dev/null

# Integration query (dexscreener)
/tmp/easyweb3 --api-base http://127.0.0.1:18080 integrations query \
  --provider dexscreener \
  --method search \
  --params '{"q":"pepe"}' \
  >/dev/null

# Integration query (goplus)
/tmp/easyweb3 --api-base http://127.0.0.1:18080 integrations query \
  --provider goplus \
  --method token_security \
  --params '{"chain_id":"56","contract_addresses":"0xdeadbeef"}' \
  >/dev/null

# Integration query (polymarket)
/tmp/easyweb3 --api-base http://127.0.0.1:18080 integrations polymarket healthz >/dev/null
/tmp/easyweb3 --api-base http://127.0.0.1:18080 integrations polymarket opportunities --limit 5 >/dev/null
/tmp/easyweb3 --api-base http://127.0.0.1:18080 integrations polymarket catalog-events --limit 5 >/dev/null
/tmp/easyweb3 --api-base http://127.0.0.1:18080 integrations polymarket catalog-markets --limit 5 >/dev/null
/tmp/easyweb3 --api-base http://127.0.0.1:18080 integrations polymarket catalog-sync --scope all >/dev/null

echo "smoke ok"
