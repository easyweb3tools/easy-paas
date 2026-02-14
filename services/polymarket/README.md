# polymarket (service)

Migrated from `/Users/bruce/git/polymarket` into `v2/services/polymarket` and adapted to work behind the easyweb3 PaaS gateway.

## Layout

- `v2/services/polymarket/backend`: Go (Gin) API + cron + DB
- `v2/services/polymarket/frontend`: Next.js UI (copied; not yet integrated with PaaS auth)

## PaaS Adaptation (backend)

- Inbound auth:
  - All `/api/*`, `/swagger/*`, `/docs` require `Authorization: Bearer ...`.
  - `/healthz` and `/readyz` stay public.
  - Env toggles:
    - `PM_AUTH_DISABLED=true` disables inbound auth checks (dev only)
    - `PM_REQUIRE_GATEWAY=true` also requires `X-Easyweb3-Project` header (to force access via gateway)

- Outbound PaaS usage (optional):
  - If `EASYWEB3_API_BASE` + `EASYWEB3_API_KEY` are set, the service logs some write actions and cron results to PaaS Logging Service.

- Docs:
  - `GET /docs` returns markdown for PaaS `service docs`.

## Run Backend (local)

```bash
cd v2/services/polymarket/backend

# Existing polymarket config
export PM_CONFIG=config/config.yaml

# Optional: enable outbound logging to PaaS
export EASYWEB3_API_BASE=http://127.0.0.1:8080
export EASYWEB3_API_KEY=ew3_xxx

go run ./cmd/monitor
```

## Register In PaaS

Set PaaS services config (example):

```bash
export EASYWEB3_SERVICES_FILE="./v2/services/services.local.json"
```

Then access through gateway:
- `GET /api/v1/services/polymarket/healthz`
- `GET /api/v1/services/polymarket/api/catalog/events`
