# easyweb3 v2 Development Plan (from v2/ARCHITECTURE.md)

This repo currently contains `v2/easyweb3-platform` (Go) implementing a subset of Phase 1.

## Current Status

Implemented (in `v2/easyweb3-platform`):
- Auth Service
  - POST `/api/v1/auth/login` (API Key -> JWT)
  - POST `/api/v1/auth/refresh`
  - GET `/api/v1/auth/status` (public)
  - POST `/api/v1/auth/keys` (admin-only)
- Logging Service
  - POST `/api/v1/logs`
  - GET `/api/v1/logs`
  - GET `/api/v1/logs/:id`
  - GET `/api/v1/logs/stats`
- Gateway
  - Reverse proxy: ANY `/api/v1/services/{name}/*` -> configured upstream `base_url`
- Minimal service endpoints for CLI
  - GET `/api/v1/service/list`
  - GET `/api/v1/service/health?name=...`

Not implemented yet (relative to `v2/ARCHITECTURE.md`):
- `easyweb3-cli` (Go)
  - auth: login/refresh/status with local token persistence (`~/.easyweb3/credentials.json`)
  - log: create/list/get
  - api: `api raw` (generic HTTP) + `api meme` and `api story` convenience commands (Phase 1/2)
  - notify: send/broadcast (Phase 2)
  - service: health/list/docs (docs not implemented on platform yet)
- PaaS platform services (Phase 2)
  - Notification Service (`/api/v1/notify/*` + project channel config)
  - Integration Service (`/api/v1/integrations/{provider}/query`)
  - Cache Service (`/api/v1/cache/*`)
- Persistence
  - Logging store should move from JSONL to PostgreSQL (doc target) OR keep JSONL but add migration path
  - API key store should move from file to DB (optional)
- Gateway ops
  - Nginx API Gateway layer + deployment compose updates (`deploy/docker-compose.*`)
- Business services adaptation (out of scope for v2 platform code, but required for E2E)
  - easymeme: switch auth to PaaS JWT, call PaaS logging/integration/notify
  - story-fork: switch auth to PaaS JWT, call PaaS logging/notify
- Admin panel (Phase 3)

## Development Phases

### Phase 1 (finish MVP: platform + CLI + proxy)

Goal: PicoClaw can `exec easyweb3 ...` to authenticate, write logs, and call business service endpoints via the PaaS gateway.

Tasks:
1. Platform: add Service Docs endpoint
   - Add `GET /api/v1/service/docs?name=...`.
   - Behavior: if service has `docs_path`, fetch it from upstream and return as `text/markdown`.
   - This keeps gateway stack-agnostic and matches CLI `easyweb3 service docs --name ...`.
2. CLI: create `v2/easyweb3-cli` (Go)
   - Commands (Phase 1):
     - `easyweb3 auth login --api-key <key>`
     - `easyweb3 auth refresh`
     - `easyweb3 auth status`
     - `easyweb3 log create --action ... --details ... --level ...`
     - `easyweb3 log list --action ... --limit ...`
     - `easyweb3 log get <id>`
     - `easyweb3 service list`
     - `easyweb3 service health --name <name>`
     - `easyweb3 service docs --name <name>`
     - `easyweb3 api raw --service <name> --method <M> --path <P> --body <JSON>`
   - Config + credential persistence:
     - `~/.easyweb3/config.json`: `api_base`, `project`, `log_level`
     - `~/.easyweb3/credentials.json`: `token`, `expires_at`, `api_key`
   - Output formats: `--output json|text|markdown` (default json).
3. E2E smoke script (local)
   - Script under `v2/easyweb3-cli/scripts/` or `v2/e2e/` to:
     - login -> create log -> list logs -> proxy raw call (to a dummy upstream).

Exit criteria:
- `v2/easyweb3-platform` + `v2/easyweb3-cli` can run locally, and CLI can:
  - login, store token, refresh token
  - create/list/get logs
  - proxy a request to an upstream service through `/api/v1/services/{name}/*`

### Phase 2 (notify + integration + cache + storyfork CLI)

Goal: converge third-party APIs and notifications into PaaS and expose via CLI.

Tasks:
1. Platform Notification Service
   - Endpoints per doc: send, broadcast, get/put config.
   - Start with: telegram webhook + generic webhook (email/slack optional).
2. Platform Integration Service
   - Implement minimal providers: dexscreener + goplus (pass-through with shared HTTP client)
   - Add basic caching hooks (in-memory or Redis when Cache Service lands).
3. Platform Cache Service
   - Redis-backed GET/PUT/DELETE.
4. CLI additions
   - `easyweb3 notify send/broadcast`
   - `easyweb3 api story ...` convenience commands

### Phase 3 (productization)

- DB-backed auth/logging, migrations
- Admin panel
- Multi-agent deployment workflows

## Implementation Order (what we will do next)

We will execute Phase 1 tasks in order:
1. Add platform `service docs` endpoint.
2. Implement `v2/easyweb3-cli` and wire it to platform.
3. Add a small local smoke script and run it.

## New Service Onboarding (example: polymarket)

Goal: migrate an existing repo into `v2/services/<name>` without modifying the original repo, and adapt it to run behind the PaaS gateway.

Checklist:
1. Copy service code into `v2/services/<name>/...` and ensure it builds/tests in-place.
2. Inbound auth: require `Authorization: Bearer ...` on business endpoints; keep health endpoints public.
3. Add `GET /docs` markdown for `easyweb3 service docs`.
4. Optional outbound: if service needs to write logs/notify/cache/integration, add a small PaaS client and use `EASYWEB3_API_BASE` + `EASYWEB3_API_KEY`.
5. Register upstream in PaaS config with `base_url`, `health_path`, `docs_path`.
