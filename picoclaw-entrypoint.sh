#!/bin/sh
set -eu

mkdir -p /root/.picoclaw/workspace/skills
mkdir -p /root/.easyweb3

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
  if [ ! -f /root/.easyweb3/credentials.json ]; then
    easyweb3 auth login --api-key "${EASYWEB3_BOOTSTRAP_API_KEY}" || true
  fi
fi

exec picoclaw "$@"

