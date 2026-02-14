# Polymarket -> easyweb3 SaaS/PaaS Integration

This document explains how the polymarket service (copied into `v2/services/polymarket`) is adapted to work behind the easyweb3 PaaS gateway.

## 1. Inbound Auth Model

- PaaS Gateway is the system entry point. It enforces JWT (Bearer token).
- The polymarket backend additionally requires `Authorization: Bearer ...` for:
  - `/api/*`
  - `/swagger/*`
  - `/docs`
- The following remain public:
  - `/healthz`
  - `/readyz`

Env toggles:
- `PM_AUTH_DISABLED=true` to disable inbound checks (dev only).
- `PM_REQUIRE_GATEWAY=true` to require `X-Easyweb3-Project` header, preventing direct access bypassing gateway.

## 2. Service Docs

- polymarket exposes `GET /docs` (markdown).
- PaaS can fetch it via `GET /api/v1/service/docs?name=polymarket` when configured with `docs_path=/docs`.

## 3. Outbound PaaS Usage (optional)

If `EASYWEB3_API_BASE` and `EASYWEB3_API_KEY` are set on the polymarket backend:
- it will login to PaaS and emit logs to `/api/v1/logs` for:
  - write HTTP requests (audit)
  - cron catalog sync ok/failed

This keeps polymarket operational data visible in the SaaS layer without changing the original repo.

## 4. PaaS Registration

Add to PaaS services config:

```json
{
  "polymarket": {
    "base_url": "http://polymarket-backend:8080",
    "health_path": "/healthz",
    "docs_path": "/docs"
  }
}
```

Then access through gateway:
- `/api/v1/services/polymarket/healthz`
- `/api/v1/services/polymarket/api/catalog/events`
- `/api/v1/services/polymarket/api/v2/opportunities`
