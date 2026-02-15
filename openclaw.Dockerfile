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

FROM node:22-alpine AS openclaw_build

ARG OPENCLAW_VERSION=latest

RUN apk add --no-cache ca-certificates tzdata curl python3

# Install OpenClaw CLI from npm.
RUN npm install -g "openclaw@${OPENCLAW_VERSION}"

FROM node:22-alpine

RUN apk add --no-cache ca-certificates tzdata curl python3

COPY --from=openclaw_build /usr/local/bin/openclaw /usr/local/bin/openclaw
COPY --from=openclaw_build /usr/local/lib/node_modules /usr/local/lib/node_modules

COPY --from=easyweb3_cli_build /out/easyweb3 /usr/local/bin/easyweb3

COPY openclaw-entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

ENV HOME=/root

ENTRYPOINT ["/entrypoint.sh"]

# Default: run OpenClaw gateway in foreground.
# You likely want to pass:
#   openclaw gateway --dev --allow-unconfigured --bind loopback --port 18789
CMD ["gateway", "--dev", "--allow-unconfigured", "--bind", "loopback", "--port", "18789"]

