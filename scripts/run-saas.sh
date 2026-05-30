#!/usr/bin/env bash
# Launch the SaaS API server with secrets loaded from .env.
# The Go server reads DB_DSN / REDIS_PASSWORD / JWT_SECRET from the environment
# (it does NOT parse .env itself), so we export them here first.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

if [[ -f .env ]]; then
  set -a
  # shellcheck disable=SC1091
  source .env
  set +a
fi

: "${DB_DSN:?DB_DSN must be set (see .env)}"
: "${JWT_SECRET:?JWT_SECRET must be set (see .env)}"

go build -o bin/saas ./cmd/saas
exec ./bin/saas "$@"
