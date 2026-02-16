#!/bin/sh
set -eu

# OpenClaw state + workspace live under ~/.openclaw by default.
# Keep easyweb3 state under the workspace so exec sandboxes can access it reliably.
export OPENCLAW_STATE_DIR="${OPENCLAW_STATE_DIR:-/root/.openclaw}"
export EASYWEB3_DIR="${EASYWEB3_DIR:-${OPENCLAW_STATE_DIR}/workspace/.easyweb3}"

WS_DIR="${OPENCLAW_STATE_DIR}/workspace"
CFG_PATH="${OPENCLAW_STATE_DIR}/openclaw.json"

mkdir -p "${WS_DIR}/skills"
mkdir -p "${EASYWEB3_DIR}"

# If no OpenClaw config exists yet, create a minimal schema-valid config.
# Note: OpenClaw validates config strictly; unknown keys will prevent startup.
if [ ! -f "${CFG_PATH}" ]; then
  cat >"${CFG_PATH}" <<'JSON5'
// ~/.openclaw/openclaw.json (JSON5)
{
  agents: {
    defaults: {
      workspace: "~/.openclaw/workspace",
      // We manage bootstrap files (AGENTS.md/TOOLS.md/BOOTSTRAP.md) below.
      skipBootstrap: true,
    },
    list: [
      {
        id: "main",
        identity: { name: "easy-paas", theme: "easyweb3 agent runtime" },
      },
    ],
  },
}
JSON5
fi

# Install workspace skills from a mounted directory (recommended in compose).
if [ -d "${OPENCLAW_SKILLS_SRC:-/skills-src}" ]; then
  cp -R "${OPENCLAW_SKILLS_SRC:-/skills-src}"/* "${WS_DIR}/skills/" 2>/dev/null || true
fi

# Seed OpenClaw "project memory" via workspace bootstrap files.
# OpenClaw automatically injects these into the system prompt (within size limits).
#
# We only create them if missing so operators can customize without getting overwritten.
if [ "${OPENCLAW_BOOTSTRAP_MEMORY:-1}" != "0" ]; then
  if [ ! -f "${WS_DIR}/AGENTS.md" ]; then
    cat >"${WS_DIR}/AGENTS.md" <<'MD'
# easy-paas (easyweb3) Agent Operating Rules

You are OpenClaw, the "brain" of easy-paas.

- Humans read (GET/HEAD are open); AI writes (POST/PUT/PATCH/DELETE require auth).
- All state-changing operations must be executed via `exec easyweb3 ...`.
- Prefer deterministic CLI flows; avoid raw HTTP unless explicitly required for debugging.
MD
  fi

  if [ ! -f "${WS_DIR}/TOOLS.md" ]; then
    cat >"${WS_DIR}/TOOLS.md" <<'MD'
# Tools

Primary tool: `easyweb3` (Go CLI).

Common:
- `easyweb3 auth login --api-key ...`
- `easyweb3 docs get architecture`
- `easyweb3 docs get openclaw`
- `easyweb3 api polymarket ...`

Hard rule: never leak API keys/JWTs in logs or replies.
MD
  fi

  if [ ! -f "${WS_DIR}/BOOTSTRAP.md" ]; then
    {
      echo "# Project Knowledge (auto-generated)"
      echo
      echo "This file is generated at container start to seed OpenClaw with the latest project docs."
      echo "To refresh manually, delete this file and restart the container."
      echo
      echo "## Architecture"
      echo
      easyweb3 --api-base "${EASYWEB3_API_BASE:-http://127.0.0.1:18080}" docs get architecture 2>/dev/null || true
      echo
      echo "## OpenClaw Role"
      echo
      easyweb3 --api-base "${EASYWEB3_API_BASE:-http://127.0.0.1:18080}" docs get openclaw 2>/dev/null || true
    } >"${WS_DIR}/BOOTSTRAP.md"
  fi
fi

# Optional: one-time easyweb3 login at container start (token persisted in EASYWEB3_DIR).
if [ -n "${EASYWEB3_BOOTSTRAP_API_KEY:-}" ]; then
  if [ ! -f "${EASYWEB3_DIR}/credentials.json" ]; then
    # Avoid leaking tokens to container logs.
    easyweb3 auth login --api-key "${EASYWEB3_BOOTSTRAP_API_KEY}" >/dev/null 2>&1 || true
  fi
fi

# Default: run the Gateway in dev mode (creates minimal config/workspace if missing).
# Note: binding beyond loopback without auth is intentionally blocked by OpenClaw.
exec openclaw "$@"
