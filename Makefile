SHELL := /bin/bash

REPO := $(CURDIR)
PLATFORM_DIR := $(REPO)/easyweb3-platform

.PHONY: help dev-stack dev-platform dev-dummy-polymarket test-docs smoke compose-web-up compose-web-down

help:
	@echo "Targets:"
	@echo "  make dev-stack            Run dummy polymarket (:18081) + easyweb3-platform (:18080) with docs enabled"
	@echo "  make dev-platform         Run easyweb3-platform only (expects upstream polymarket at :18081)"
	@echo "  make dev-dummy-polymarket Run dummy polymarket upstream only (:18081)"
	@echo "  make test-docs            Curl-check /docs endpoints on http://127.0.0.1:18080"
	@echo "  make smoke                Run e2e/smoke.sh (full local smoke)"
	@echo "  make compose-web-up       docker compose --profile web up -d --build"
	@echo "  make compose-web-down     docker compose --profile web down"

# Local dev environment (override as needed):
#   EASYWEB3_LISTEN=:18080
#   EASYWEB3_DOCS_DIR=/abs/path/to/docs
#   EASYWEB3_SERVICES_JSON='{"polymarket":{"base_url":"http://127.0.0.1:18081","health_path":"/healthz","docs_path":"/docs"}}'
dev-platform:
	@set -euo pipefail; \
	export EASYWEB3_BOOTSTRAP_ADMIN_API_KEY="$${EASYWEB3_BOOTSTRAP_ADMIN_API_KEY:-ew3_admin_dev}"; \
	export EASYWEB3_JWT_SECRET="$${EASYWEB3_JWT_SECRET:-dev-secret-change-me-please}"; \
	export EASYWEB3_LISTEN="$${EASYWEB3_LISTEN:-:18080}"; \
	export EASYWEB3_NOTIFY_FILE="$${EASYWEB3_NOTIFY_FILE:-/tmp/ew3-notify-config.json}"; \
	export EASYWEB3_CACHE_BACKEND="$${EASYWEB3_CACHE_BACKEND:-memory}"; \
	export EASYWEB3_CACHE_DEFAULT_TTL="$${EASYWEB3_CACHE_DEFAULT_TTL:-30s}"; \
	export EASYWEB3_DOCS_DIR="$${EASYWEB3_DOCS_DIR:-$(REPO)/docs}"; \
	export EASYWEB3_SERVICES_JSON="$${EASYWEB3_SERVICES_JSON:-{\"polymarket\":{\"base_url\":\"http://127.0.0.1:18081\",\"health_path\":\"/healthz\",\"docs_path\":\"/docs\"}}}"; \
	cd "$(PLATFORM_DIR)"; \
	exec go run ./cmd/platform

dev-dummy-polymarket:
	@set -euo pipefail; \
	PYTHONDONTWRITEBYTECODE=1 exec python3 "$(REPO)/e2e/dummy_polymarket.py"

dev-stack:
	@set -euo pipefail; \
	export EASYWEB3_BOOTSTRAP_ADMIN_API_KEY="$${EASYWEB3_BOOTSTRAP_ADMIN_API_KEY:-ew3_admin_dev}"; \
	export EASYWEB3_JWT_SECRET="$${EASYWEB3_JWT_SECRET:-dev-secret-change-me-please}"; \
	export EASYWEB3_LISTEN="$${EASYWEB3_LISTEN:-:18080}"; \
	export EASYWEB3_NOTIFY_FILE="$${EASYWEB3_NOTIFY_FILE:-/tmp/ew3-notify-config.json}"; \
	export EASYWEB3_CACHE_BACKEND="$${EASYWEB3_CACHE_BACKEND:-memory}"; \
	export EASYWEB3_CACHE_DEFAULT_TTL="$${EASYWEB3_CACHE_DEFAULT_TTL:-30s}"; \
	export EASYWEB3_DOCS_DIR="$${EASYWEB3_DOCS_DIR:-$(REPO)/docs}"; \
	export EASYWEB3_SERVICES_JSON="$${EASYWEB3_SERVICES_JSON:-{\"polymarket\":{\"base_url\":\"http://127.0.0.1:18081\",\"health_path\":\"/healthz\",\"docs_path\":\"/docs\"}}}"; \
	\
	PYTHONDONTWRITEBYTECODE=1 python3 "$(REPO)/e2e/dummy_polymarket.py" >/tmp/ew3-dummy-polymarket.log 2>&1 & \
	PM_PID=$$!; \
	cleanup() { kill $$PM_PID >/dev/null 2>&1 || true; }; \
	trap cleanup EXIT; \
	\
	cd "$(PLATFORM_DIR)"; \
	exec go run ./cmd/platform

	test-docs:
		@set -euo pipefail; \
		for p in /docs /docs/architecture /docs/architecture/ /docs/openclaw /docs/openclaw/; do \
		  code=$$(curl -sS -o /tmp/ew3-doc-body.txt -w '%{http_code}' "http://127.0.0.1:18080$${p}"); \
		  first=$$(head -n 1 /tmp/ew3-doc-body.txt | tr -d '\r'); \
		  echo "$${p} -> $${code} :: $${first}"; \
		done

smoke:
	@set -euo pipefail; \
	exec sh "$(REPO)/e2e/smoke.sh"

compose-web-up:
	@set -euo pipefail; \
	docker compose --profile web up -d --build

compose-web-down:
	@set -euo pipefail; \
	docker compose --profile web down
