#!/usr/bin/env bash
# Build and serve the web frontend. Vite preview serves the optimized bundle
# and proxies /api -> the SaaS server (see vite.config.ts preview.proxy).
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT/web-frontend"

[[ -d node_modules ]] || npm install --no-fund --no-audit
npm run build
exec npm run preview -- --host --port 5173
