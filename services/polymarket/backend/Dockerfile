# syntax=docker/dockerfile:1

FROM golang:1.25.6-alpine AS build

WORKDIR /app

RUN apk add --no-cache ca-certificates

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /app/bin/monitor ./cmd/monitor

FROM alpine:3.20

RUN apk add --no-cache ca-certificates

WORKDIR /app

COPY --from=build /app/bin/monitor /app/monitor
COPY config/config.yaml /app/config/config.yaml

ENV PM_CONFIG=/app/config/config.yaml

EXPOSE 8080

ENTRYPOINT ["/app/monitor"]
