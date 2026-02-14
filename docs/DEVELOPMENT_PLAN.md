# easy-paas Development Plan

This doc tracks what is implemented vs missing for the easy-paas MVP and the next hardening steps.

Reference design:
- `docs/ARCHITECTURE.md`

## Current Status (as of 2026-02-14)

Implemented:
- PaaS API (`easyweb3-platform`, Go)
  - Auth
    - API key login: `POST /api/v1/auth/login` (api_key -> JWT)
    - User register: `POST /api/v1/auth/register`
    - User login: `POST /api/v1/auth/login` (username/password/project_id -> JWT, requires grants)
    - Grants: `POST /api/v1/auth/grants` (agent/admin)
    - Users list: `GET /api/v1/auth/users` (agent/admin)
    - Status: `GET /api/v1/auth/status` (public)
  - Logging: `/api/v1/logs/*`
  - Notify: `/api/v1/notify/*`
  - Integrations: `/api/v1/integrations/{provider}/query`
  - Cache: `/api/v1/cache/*`
  - Gateway proxy: `/api/v1/services/{name}/*`
  - Service registry ops: `/api/v1/service/*`
- Agent SDK (`easyweb3-cli`, Go)
  - `auth login|register|grant|refresh|status`
  - `log create|list|get`
  - `notify send|broadcast|config`
  - `integrations query`
  - `cache get|put|delete`
  - `service list|health|docs`
  - `api raw` + `api polymarket ...`
- Polymarket service (migrated into `services/polymarket`)
  - Exposes `/api/catalog/*` and `/api/v2/*` behind the PaaS gateway
  - Writes best-effort audit logs to PaaS for write-ish calls
- Local deployment (`docker-compose.yml`)
  - `web` profile adds nginx as a single entrypoint
  - `picoclaw` profile runs `picoclaw-gateway` (skills + exec easyweb3)
- Production deployment (`deploy/paas`)
  - nginx on `:80` and pinned GHCR images

Known issues / active fixes:
- Production `GET /api/v2/analytics/*` can 502 if DB schema is inconsistent (migration/column naming).
- Production `/api/catalog/*` 404 usually means nginx config wasn’t updated/restarted (request is hitting frontend instead of platform).
- Public read access for polymarket queries is currently implemented as a temporary exception (see “Hardening” below).

## Roadmap

### 1) Production Hardening

Tasks:
1. Make “public read” an explicit config flag (default: off)
   - Platform: `EASYWEB3_PUBLIC_READ_POLYMARKET=true` (or a generic ruleset)
   - Keep write endpoints always protected.
2. Analytics stability
   - Fix polymarket DB schema mismatch (automatic rename/migration for PnL columns).
   - Add a `/readyz` check that fails when critical migrations are missing (optional).
3. Observability
   - Add request IDs across nginx -> platform -> service.
   - Add structured logs for gateway proxy errors (include upstream url + path).

Exit criteria:
- Fresh install + existing DB both work.
- `/api/v2/analytics/*` stable (no schema-related 502).

### 2) PicoClaw Ops + Skills

Tasks:
1. Add a “bootstrap skill” for ops (done in deploy repo)
   - `deploy/paas/skills/paas-bootstrap/SKILL.md`
2. Provider configuration templates (done in deploy repo)
   - `deploy/paas/picoclaw-config/config.example.json`
3. Add a health/reporting skill
   - Pull `easyweb3 service health/list`
   - Pull “cron last run” style data (needs new endpoint or parse logs)

### 3) Multi-Service Onboarding

Tasks:
- Standardize service contracts:
  - `/healthz` (public), `/docs` (public), `/api/*` (protected by gateway)
- Provide per-service “service.json” registry + “deploy overlay”
- Add more services (easymeme/storyfork) behind the gateway.
