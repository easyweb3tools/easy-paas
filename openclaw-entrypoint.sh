#!/bin/sh
set -eu

# OpenClaw state + workspace live under ~/.openclaw by default.
# Keep easyweb3 state under the workspace so exec sandboxes can access it reliably.
export OPENCLAW_STATE_DIR="${OPENCLAW_STATE_DIR:-/root/.openclaw}"
export EASYWEB3_DIR="${EASYWEB3_DIR:-${OPENCLAW_STATE_DIR}/workspace/.easyweb3}"

mkdir -p "${OPENCLAW_STATE_DIR}/workspace/skills"
mkdir -p "${EASYWEB3_DIR}"

# Install workspace skills from a mounted directory (recommended in compose).
if [ -d "${OPENCLAW_SKILLS_SRC:-/skills-src}" ]; then
  cp -R "${OPENCLAW_SKILLS_SRC:-/skills-src}"/* "${OPENCLAW_STATE_DIR}/workspace/skills/" 2>/dev/null || true
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

