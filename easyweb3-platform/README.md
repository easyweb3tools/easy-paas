# easyweb3-platform (v2)

Go implementation of the easyweb3 PaaS MVP described in `docs/ARCHITECTURE.md`.

## MVP Scope Implemented

- Auth Service
  - `POST /api/v1/auth/login` (API Key -> JWT)
  - `POST /api/v1/auth/refresh`
  - `GET /api/v1/auth/status`
  - `POST /api/v1/auth/keys` (admin only)
- Logging Service
  - `POST /api/v1/logs`
  - `GET /api/v1/logs`
  - `GET /api/v1/logs/:id`
  - `GET /api/v1/logs/stats`
- Notification Service
  - `POST /api/v1/notify/send`
  - `POST /api/v1/notify/broadcast`
  - `GET /api/v1/notify/config`
  - `PUT /api/v1/notify/config`
- Integration Service
  - `POST /api/v1/integrations/{provider}/query` (MVP providers: `dexscreener`, `goplus`)
- Cache Service
  - `GET /api/v1/cache/:key`
  - `PUT /api/v1/cache/:key`
  - `DELETE /api/v1/cache/:key`
- Gateway (reverse proxy)
  - `ANY /api/v1/services/{name}/*` -> upstream `base_url` (stack-agnostic: Go/Next.js/etc.)
- Service management endpoints for CLI (minimal)
  - `GET /api/v1/service/list`
  - `GET /api/v1/service/health?name=...`
  - `GET /api/v1/service/docs?name=...` (fetches upstream markdown when `docs_path` is configured)

Storage (MVP):
- API keys: JSON file (`./data/api_keys.json`) storing SHA-256 hashes.
- Logs: JSONL file (`./data/logs.jsonl`).
- Notify config: JSON file (`./data/notify_config.json`).

Cache backend:
- Default: in-memory.
- Optional: Redis (`EASYWEB3_CACHE_BACKEND=redis` + `EASYWEB3_REDIS_ADDR=host:port`).

## Run

From `v2/easyweb3-platform`:

```bash
# Bootstrap admin key for first run (required to create real per-project keys)
export EASYWEB3_BOOTSTRAP_ADMIN_API_KEY="ew3_admin_dev"

# JWT secret (must be >= 16 bytes)
export EASYWEB3_JWT_SECRET="dev-secret-change-me-please"

# Configure upstream business services (examples)
export EASYWEB3_SERVICES_JSON='{
  "meme": {"base_url": "http://localhost:8081", "health_path": "/health"},
  "story": {"base_url": "http://localhost:3000", "health_path": "/api/health"}
}'

go run ./cmd/platform
```

Health:

```bash
curl -sS http://localhost:8080/healthz
```

## Auth Flow (cURL)

Login with bootstrap admin key:

```bash
curl -sS -X POST http://localhost:8080/api/v1/auth/login \
  -H 'content-type: application/json' \
  -d '{"api_key":"ew3_admin_dev"}'
```

Create a project key (admin-only):

```bash
# TOKEN=... from login
curl -sS -X POST http://localhost:8080/api/v1/auth/keys \
  -H 'content-type: application/json' \
  -H "authorization: Bearer $TOKEN" \
  -d '{"project_id":"easymeme","role":"agent","name":"easymeme-trader"}'
```

## Logs (cURL)

```bash
curl -sS -X POST http://localhost:8080/api/v1/logs \
  -H 'content-type: application/json' \
  -H "authorization: Bearer $TOKEN" \
  -d '{
    "agent":"trader-agent",
    "action":"trade_executed",
    "level":"info",
    "session_key":"tg:group1",
    "details":{"token":"PEPE2","type":"BUY","amount":"0.1 BNB"}
  }'

curl -sS "http://localhost:8080/api/v1/logs?limit=20" \
  -H "authorization: Bearer $TOKEN"
```

## Proxy Business Service

Example: forward to the configured `meme` upstream.

```bash
curl -sS http://localhost:8080/api/v1/services/meme/api/v1/some-endpoint \
  -H "authorization: Bearer $TOKEN"
```
