#!/usr/bin/env bash
set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
cd "$repo_root"

if ! command -v swag >/dev/null 2>&1; then
  echo "swag not found in PATH. Install with: go install github.com/swaggo/swag/cmd/swag@latest" >&2
  exit 1
fi

swag init -g cmd/monitor/main.go -o docs
