# ============================================================
# easyweb3 v2: OpenClaw + easyweb3 CLI (single container)
#
# Build context: repo root
# ============================================================
# syntax=docker/dockerfile:1

FROM golang:1.22-alpine AS easyweb3_cli_build

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /src/easyweb3-cli

COPY easyweb3-cli/go.mod easyweb3-cli/go.sum ./
RUN go mod download

COPY easyweb3-cli/ ./
RUN CGO_ENABLED=0 go build -o /out/easyweb3 .

# NOTE: OpenClaw's npm dependencies include native modules (e.g. node-llama-cpp).
# Alpine (musl) often forces source builds and can fail without heavy toolchains.
# Use Debian-based Node to prefer prebuilt binaries and improve reliability.
FROM node:22-bookworm-slim AS openclaw_build

ARG OPENCLAW_VERSION=latest

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates tzdata curl python3 git \
  && rm -rf /var/lib/apt/lists/*

# Install OpenClaw CLI from npm.
RUN npm install -g "openclaw@${OPENCLAW_VERSION}"

FROM node:22-bookworm-slim

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates tzdata curl python3 \
  && rm -rf /var/lib/apt/lists/*

COPY --from=openclaw_build /usr/local/bin/openclaw /usr/local/bin/openclaw
COPY --from=openclaw_build /usr/local/lib/node_modules /usr/local/lib/node_modules

COPY --from=easyweb3_cli_build /out/easyweb3 /usr/local/bin/easyweb3

COPY openclaw/entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

# npm's generated bin shim uses relative imports from its own location, which breaks when
# copied across stages. Provide a stable wrapper that runs the actual package entry.
RUN rm -f /usr/local/bin/openclaw && \
  printf '%s\n' '#!/bin/sh' 'exec node /usr/local/lib/node_modules/openclaw/openclaw.mjs "$@"' > /usr/local/bin/openclaw && \
  chmod +x /usr/local/bin/openclaw

ENV HOME=/root

ENTRYPOINT ["/entrypoint.sh"]

# Default: run OpenClaw gateway in foreground.
# You likely want to pass:
#   openclaw gateway --dev --allow-unconfigured --bind loopback --port 18789
CMD ["gateway", "--dev", "--allow-unconfigured", "--bind", "loopback", "--port", "18789"]
