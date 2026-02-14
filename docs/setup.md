# Setup (Docker Compose)

This doc explains what `docker compose` does for easy-paas, what you still need to configure, and how to debug PicoClaw runtime in production.

## Layout

- `docker-compose.yml`: local compose (profiles: `web`, `picoclaw`)
- `easyweb3-platform/`: PaaS API + gateway (Go)
- `services/`: business services (e.g. `services/polymarket`)
- `skills/`: PicoClaw skills (mounted into gateway container)
- `picoclaw/`: upstream PicoClaw (git submodule)
- `nginx/`: local nginx entrypoint config (only used by `web` profile)

## Compose Profiles

- Default (no profile): PaaS + polymarket backend + postgres
- `--profile web`: adds `nginx` and `polymarket-frontend`
- `--profile picoclaw`: adds `picoclaw-gateway`

Example:

```bash
docker compose up -d
docker compose --profile web up -d
docker compose --profile picoclaw up -d
```

## What Compose Does Automatically

### PaaS (`easyweb3-platform`)

- Starts the HTTP server on `EASYWEB3_LISTEN` (default `:8080`).
- Loads service registry from `EASYWEB3_SERVICES_FILE`.
  - Local default: `services/services.local.json`
- Stores state in files (inside the mounted volume):
  - api keys: `EASYWEB3_API_KEYS_FILE`
  - logs: `EASYWEB3_LOGS_FILE`
  - notify config: `EASYWEB3_NOTIFY_FILE`
  - users/grants: `EASYWEB3_USERS_FILE` (if enabled)

### Polymarket backend (`polymarket-backend`)

- Runs DB auto-migrate on startup (GORM AutoMigrate).
- Runs internal background loops (signals, strategy scan, settlement ingest).
- May run cron jobs depending on config (`cron.enabled`, `cron.catalog_sync` etc).

### PicoClaw gateway (`picoclaw-gateway`)

Container entrypoint: `picoclaw-entrypoint.sh`

On startup it:
1. Ensures workspace directories exist under `/root/.picoclaw/workspace`.
2. Copies builtin skills from the image (best-effort).
3. Copies project skills from a mounted directory (`/skills-src`) into the PicoClaw workspace skills directory.
4. If `EASYWEB3_BOOTSTRAP_API_KEY` is set and no credentials exist yet, runs:

   - `easyweb3 auth login --api-key ...`

5. Executes `picoclaw ...` (default `gateway`).

## What You Still Need To Configure

### 1) Secrets

At minimum:
- `EASYWEB3_JWT_SECRET`: JWT signing secret (production must be strong)
- `EASYWEB3_BOOTSTRAP_ADMIN_API_KEY`: bootstrap admin api-key (production must be random)

### 2) PicoClaw Provider (LLM)

If you see:
- `Error creating provider: no API key configured for model: glm-4.7`

It means PicoClaw default model is `glm-4.7` but you didn't configure `providers.zhipu` or `providers.openrouter`.

In production we recommend mounting a PicoClaw config file:
- `/root/.picoclaw/config.json`

See templates in the deploy repo:
- `deploy/paas/picoclaw-config/config.example.json`

### 3) Skills Mount (for PicoClaw)

If you deploy via compose, mount:
- `./skills:/skills-src:ro`

At runtime PicoClaw loads:
- `/root/.picoclaw/workspace/skills/<skill>/SKILL.md`

## How To Check PicoClaw Status

### Containers

```bash
docker compose ps
docker compose --profile picoclaw ps
```

### Logs

```bash
docker compose logs --tail=200 -f picoclaw-gateway
docker compose logs --tail=200 -f easyweb3-platform
docker compose logs --tail=200 -f polymarket-backend
```

If you use `deploy/paas` on the server:

```bash
cd /path/to/deploy/paas
docker compose logs --tail=200 -f picoclaw-gateway
```

### Enter The Container

```bash
docker compose exec picoclaw-gateway sh
```

Inside the container:

```sh
picoclaw --help
easyweb3 --help
ls -la /root/.picoclaw/workspace/skills
cat /root/.easyweb3/credentials.json || true
```

### Talk To PicoClaw Interactively (No External Channel)

If a provider is configured:

```bash
docker compose exec picoclaw-gateway picoclaw agent -m "What is 2+2?"
```

### Check “Scheduled Jobs” (Polymarket Backend)

Polymarket backend has internal background loops + cron.

Ways to check:
1. Look for cron / collector log lines:

```bash
docker compose logs --tail=500 polymarket-backend | rg -n "cron|catalog|collector|scan|settlement" || true
```

2. Query signal source health (if enabled by the service):
- `GET /api/v2/signals/sources`

When deployed with nginx + platform proxy, you can hit:
- `https://<domain>/api/v2/signals/sources`

## Common Production Debugging

### `/api/catalog/*` returns 404

Most often nginx isn't routing `/api/catalog/*` to the platform gateway, so the request hits the frontend and returns Next.js 404.

Fix:
1. Update deploy repo on the server.
2. Recreate nginx container.

```bash
cd /path/to/deploy/paas
git pull
docker compose up -d --force-recreate nginx
```

### `/api/v2/analytics/*` returns 502

This typically means the backend returned an error (DB schema mismatch is a common cause).

Check:
```bash
docker compose logs --tail=200 polymarket-backend
```

If the error mentions a missing column, either:
- upgrade to a newer backend image that includes migrations/renames, or
- recreate the DB volume in a non-critical environment.

