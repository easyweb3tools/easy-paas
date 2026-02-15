# ============================================================
# easyweb3 v2: PicoClaw + easyweb3 CLI (single container)
#
# Build context: repo root (v2/)
# Uses PicoClaw submodule at ./picoclaw
# ============================================================
# syntax=docker/dockerfile:1

FROM golang:1.23-alpine AS picoclaw_build

ENV GOTOOLCHAIN=auto

RUN apk add --no-cache git make ca-certificates tzdata

WORKDIR /src/picoclaw

COPY picoclaw/go.mod picoclaw/go.sum ./
RUN set -e; \
    for i in 1 2 3 4 5; do \
      if go mod download; then exit 0; fi; \
      echo "go mod download failed, retry ${i}/5..." >&2; \
      sleep $((i * 2)); \
    done; \
    exit 1

COPY picoclaw/ ./
RUN make build
# Export builtin skills to a stable path for the final image.
# Some CI checkouts / submodule states may not include the skills directory.
RUN set -e; \
    mkdir -p /out/picoclaw_skills; \
    if [ -d /src/picoclaw/skills ]; then \
      cp -R /src/picoclaw/skills/* /out/picoclaw_skills/ 2>/dev/null || true; \
    fi

FROM golang:1.22-alpine AS easyweb3_cli_build

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /src/easyweb3-cli

COPY easyweb3-cli/go.mod easyweb3-cli/go.sum ./
RUN go mod download

COPY easyweb3-cli/ ./
RUN CGO_ENABLED=0 go build -o /out/easyweb3 .

FROM alpine:3.21

RUN apk add --no-cache ca-certificates tzdata curl python3

COPY --from=picoclaw_build /src/picoclaw/build/picoclaw /usr/local/bin/picoclaw
COPY --from=picoclaw_build /out/picoclaw_skills /opt/picoclaw/skills
COPY --from=easyweb3_cli_build /out/easyweb3 /usr/local/bin/easyweb3

COPY picoclaw-entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

ENV HOME=/root

ENTRYPOINT ["/entrypoint.sh"]
CMD ["gateway"]
