# ============================================================
# easyweb3 v2: build PicoClaw from upstream source (submodule)
#
# Notes:
# - We do NOT rely on upstream Dockerfile because it may pin to
#   non-existing Go versions. This Dockerfile is owned by easyweb3.
# - Build context should be the picoclaw submodule directory.
# ============================================================
FROM golang:1.22-alpine AS builder

RUN apk add --no-cache git make

WORKDIR /src

COPY go.mod go.sum ./
# Upstream may temporarily set an invalid or too-new Go version in go.mod.
# Patch it here to keep our build toolchain stable.
RUN set -e; \
    if grep -qE '^go 1\\.(2[3-9]|[3-9][0-9])\\.' go.mod || grep -qE '^go 1\\.25\\.' go.mod; then \
      sed -i -E 's/^go 1\\.[0-9]+\\.[0-9]+/go 1.22/' go.mod; \
    fi; \
    # If a toolchain line exists for a too-new version, drop it (Go will otherwise insist on it).
    sed -i -E '/^toolchain go1\\.(2[3-9]|[3-9][0-9])\\./d' go.mod || true; \
    go mod download

COPY . .
RUN make build

FROM alpine:3.21

RUN apk add --no-cache ca-certificates tzdata

COPY --from=builder /src/build/picoclaw /usr/local/bin/picoclaw

# Keep the same convention as upstream: ship builtin skills.
COPY --from=builder /src/skills /opt/picoclaw/skills

RUN mkdir -p /root/.picoclaw/workspace/skills && \
    cp -r /opt/picoclaw/skills/* /root/.picoclaw/workspace/skills/ 2>/dev/null || true

ENTRYPOINT ["picoclaw"]
CMD ["gateway"]
