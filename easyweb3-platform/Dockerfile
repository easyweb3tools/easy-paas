# syntax=docker/dockerfile:1

FROM golang:1.22-alpine AS build

WORKDIR /src

# Build deps
RUN apk add --no-cache ca-certificates tzdata

COPY go.mod go.sum ./
RUN go mod download

COPY cmd ./cmd
COPY internal ./internal
COPY public-docs ./public-docs

RUN CGO_ENABLED=0 go build -o /out/easyweb3-platform ./cmd/platform

FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app
COPY --from=build /out/easyweb3-platform /usr/local/bin/easyweb3-platform
COPY --from=build /src/public-docs /app/public-docs

ENV EASYWEB3_DOCS_DIR=/app/public-docs

EXPOSE 8080

CMD ["easyweb3-platform"]
