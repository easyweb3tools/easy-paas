#!/bin/sh
set -eu

mkdir -p /root/.picoclaw/workspace/skills

# PicoClaw sandboxes may restrict FS access outside workspace. Keep easyweb3 state inside workspace by default.
export EASYWEB3_DIR="${EASYWEB3_DIR:-/root/.picoclaw/workspace/.easyweb3}"
mkdir -p "${EASYWEB3_DIR}"

# Install builtin skills bundled in the image (best effort).
if [ -d /opt/picoclaw/skills ]; then
  cp -R /opt/picoclaw/skills/* /root/.picoclaw/workspace/skills/ 2>/dev/null || true
fi

# Install easyweb3 project skills from a mounted directory (recommended in compose).
if [ -d "${PICOCLAW_SKILLS_SRC:-/skills-src}" ]; then
  cp -R "${PICOCLAW_SKILLS_SRC:-/skills-src}"/* /root/.picoclaw/workspace/skills/ 2>/dev/null || true
fi

# Optional: one-time easyweb3 login at container start (token persisted in /root/.easyweb3).
if [ -n "${EASYWEB3_BOOTSTRAP_API_KEY:-}" ]; then
  if [ ! -f "${EASYWEB3_DIR}/credentials.json" ]; then
    # Avoid leaking tokens to container logs.
    easyweb3 auth login --api-key "${EASYWEB3_BOOTSTRAP_API_KEY}" >/dev/null 2>&1 || true
  fi
fi

exec picoclaw "$@"
