# ============================================================
# easyweb3 v2: build PicoClaw from upstream source (submodule)
#
# Notes:
# - We do NOT rely on upstream Dockerfile because it may pin to
#   non-existing Go versions. This Dockerfile is owned by easyweb3.
# - Build context should be the picoclaw submodule directory.
# ============================================================
# Use a reasonably recent Go toolchain, and allow auto toolchain download to follow upstream.
FROM golang:1.23-alpine AS builder

ENV GOTOOLCHAIN=auto

RUN apk add --no-cache git make

WORKDIR /src

COPY go.mod go.sum ./
# Download modules (with retries to tolerate flaky proxy/network).
RUN set -e; \
    for i in 1 2 3 4 5; do \
      if go mod download; then \
        exit 0; \
      fi; \
      echo "go mod download failed, retry ${i}/5..." >&2; \
      sleep $((i * 2)); \
    done; \
    exit 1

COPY . .
# Makefile runs `go build`; keep toolchain=auto so it can follow go.mod's required version.
RUN GOTOOLCHAIN=auto make build

FROM alpine:3.21

RUN apk add --no-cache ca-certificates tzdata

COPY --from=builder /src/build/picoclaw /usr/local/bin/picoclaw

# Keep the same convention as upstream: ship builtin skills.
COPY --from=builder /src/skills /opt/picoclaw/skills

RUN mkdir -p /root/.picoclaw/workspace/skills && \
    cp -r /opt/picoclaw/skills/* /root/.picoclaw/workspace/skills/ 2>/dev/null || true

ENTRYPOINT ["picoclaw"]
CMD ["gateway"]
