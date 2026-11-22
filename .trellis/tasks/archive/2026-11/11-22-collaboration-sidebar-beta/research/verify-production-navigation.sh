#!/usr/bin/env bash
set -euo pipefail

sidebar_research_dir="/Volumes/artisan/code/2026/multica-0.4.37/.trellis/tasks/11-22-collaboration-sidebar-beta/research"
cd "${SIDEBAR_VERIFY_SOURCE_ROOT:-/Volumes/artisan/code/2026/multica-0.4.37}"
export PATH="/opt/homebrew/opt/libpq/bin:$PATH"
source scripts/check.sh
sidebar_source_env="$(env_file_path "${SIDEBAR_VERIFY_SOURCE_ENV:-${ENV_FILE:-$(detect_env_file)}}")"
load_env_file "$sidebar_source_env"
prepare_check_environment "$sidebar_source_env"
trap check_cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM
mkdir -p "$DEV_TMPDIR"
export TMPDIR="$DEV_TMPDIR" TMP="$DEV_TMPDIR" TEMP="$DEV_TMPDIR"
export NO_PROXY="localhost,127.0.0.1,::1"
export no_proxy="$NO_PROXY"
export VITE_API_URL="http://localhost:$BACKEND_PORT"
export VITE_WS_URL="ws://localhost:$BACKEND_PORT/ws"
export VITE_APP_URL="http://localhost:$FRONTEND_PORT"
ensure_database
migrate_database
CHECK_STARTED_API=true
start_api
pnpm -C apps/desktop exec electron-vite build
CHECK_STARTED_RENDERER=true
start_check_desktop_renderer
CHECK_STARTED_WEB=true
start_web
print_status_json > "$sidebar_research_dir/production-preview-status.json"
pnpm exec playwright test e2e/global-scope-navigation.spec.ts --workers=1 --retries=0 --reporter=line --output="$sidebar_research_dir/production-journey"
CHECK_COMPLETE=true
