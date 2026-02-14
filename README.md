# v2

This directory contains the easyweb3 PaaS platform, CLI, and onboarded business services.

## Components

- `v2/easyweb3-platform`: PaaS (auth/logging/notify/integration/cache + gateway)
- `v2/easyweb3-cli`: CLI client for agents (exec)
- `v2/services`: business services proxied behind PaaS
  - `v2/services/polymarket`: Polymarket service migrated and adapted for PaaS
- `v2/picoclaw`: PicoClaw upstream vendored as a subproject (for containerized deployment)
- `v2/skills`: PicoClaw skills (one project/service one skill)

## Local Compose

A minimal local stack (PaaS + polymarket backend + postgres):

- `v2/docker-compose.v2.local.yml`

Ports:
- PaaS: `http://127.0.0.1:18080`
- Polymarket backend (direct): `http://127.0.0.1:18088` (should be accessed via PaaS gateway in normal usage)

## Register Services

The PaaS service registry is loaded from:
- `v2/services/services.local.json`

## PicoClaw Skills

In this repo, PicoClaw skills live under:

- `v2/skills/<skill-name>/SKILL.md`

The runtime convention described in `v2/ARCHITECTURE.md` is:

- `<picoclaw-workspace>/skills/<skill-name>/SKILL.md`

So in deployment (including docker compose), you typically:

1. Mount `v2/skills` into PicoClaw workspace `skills/`
2. Ensure `easyweb3` CLI is on `PATH` inside the PicoClaw container/VM (recommended install path: `~/.local/bin/easyweb3`)
3. Provide `EASYWEB3_API_BASE` and a way to authenticate (usually `easyweb3 auth login --api-key ...` at startup)

Skill authoring guidelines (kept intentionally simple, SKILL = Markdown):

- One skill per project/service.
- Keep tool usage stable: call `easyweb3 ...` via `exec`, avoid raw HTTP calls.
- Use progressive disclosure: only open/expand details when needed; prefer deterministic CLI flows.
- Prefer idempotent commands (safe retries) and record every meaningful action in PaaS logs.

See: `v2/skills/README.md`.

### PicoClaw Image Build (GitHub Actions)

This repo vendors upstream PicoClaw in `v2/picoclaw` and provides a build workflow:

- `v2/.github/workflows/v2-picoclaw-image.yml`

PicoClaw is included as a git submodule, so the build can pull latest upstream at build time.

On `release/v*` branch updates (e.g. `release/v1.0.0`), it builds and pushes:

- `ghcr.io/<owner>/easyweb3-picoclaw:<version>` (e.g. `v1.0.0`)
- `ghcr.io/<owner>/easyweb3-picoclaw:sha-<shortsha>`

### Compose Deployment Pattern (Skill Mount + Startup Login)

There is no hard dependency that PicoClaw must be deployed via compose, but if you do, the pattern is:

1. Start the PaaS + services (e.g. `v2/docker-compose.v2.local.yml`)
2. Start a PicoClaw container with:
   - `v2/skills` mounted into the container workspace skills directory
   - `easyweb3` available on `PATH`
   - startup command that logs in once (API key) then runs PicoClaw

Example snippet (you need to adjust `image`, `command`, and the exact workspace path to match your PicoClaw image):

```yaml
services:
  picoclaw-polymarket:
    image: <your-picoclaw-image>
    restart: unless-stopped
    environment:
      EASYWEB3_API_BASE: http://easyweb3-platform:8080
      EASYWEB3_PROJECT: polymarket
      EASYWEB3_BOOTSTRAP_API_KEY: ew3_admin_dev
    volumes:
      # Mount skill sources read-only, then copy into the actual workspace at container start.
      - ./v2/skills:/skills-src:ro
      - picoclaw_workspace:/workspace
    command:
      - sh
      - -lc
      - |
        mkdir -p /workspace/skills
        cp -R /skills-src/* /workspace/skills/
        easyweb3 auth login --api-key "${EASYWEB3_BOOTSTRAP_API_KEY}"
        exec picoclaw --workspace /workspace

volumes:
  picoclaw_workspace:
```

## Quick Manual Test

1. Start stack.
2. Login to PaaS using bootstrap key (dev only).
3. Call polymarket through gateway.

Example (after building CLI):
- `easyweb3 --api-base http://127.0.0.1:18080 auth login --api-key ew3_admin_dev`
- `easyweb3 --api-base http://127.0.0.1:18080 api raw --service polymarket --method GET --path /healthz`
