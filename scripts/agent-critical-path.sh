#!/usr/bin/env bash
# Internal orchestration, invoked in a clean environment by the JSON launcher.
# Tests source this file and replace process boundaries with explicit fakes.
set -euo pipefail
SMOKE_SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=dev-env.sh
. "$SMOKE_SCRIPT_DIR/dev-env.sh"

SMOKE_STARTED_API=false
SMOKE_STARTED_WEB=false
SMOKE_COMPLETE=false
SMOKE_TEST_EXIT=not_run
SMOKE_INVENTORY_EXIT=not_run

smoke_cleanup() {
  local result=$? cleanup=0
  trap - EXIT INT TERM
  if [ "$SMOKE_STARTED_WEB" = true ]; then stop_component web || cleanup=1; fi
  if [ "$SMOKE_STARTED_API" = true ]; then stop_component api || cleanup=1; fi
  if [ "$result" -eq 0 ] && { [ "$SMOKE_COMPLETE" != true ] || [ "$cleanup" -ne 0 ]; }; then result=1; fi
  node - "$MULTICA_SMOKE_RUN_DIR/result.json" "$result" "$cleanup" "$SMOKE_TEST_EXIT" "$SMOKE_INVENTORY_EXIT" <<'JS'
const fs = require('node:fs');
const [file, exit, cleanup, playwright, inventory] = process.argv.slice(2);
fs.writeFileSync(file, JSON.stringify({exit:Number(exit), cleanup:Number(cleanup), playwright, inventory},null,2)+'\n', {flag:'wx',mode:0o600});
JS
  exit "$result"
}

smoke_verify_database() {
  local actual
  # Do not print psql's raw diagnostics: connection strings may contain secrets.
  actual="$(PGOPTIONS='-c statement_timeout=5000' psql "$DATABASE_URL" -X -A -t -v ON_ERROR_STOP=1 \
    -c 'SELECT current_database()' 2> "$MULTICA_SMOKE_RUN_DIR/database-probe.log")" || die "Owned smoke database is unavailable."
  [ "$actual" = "$MULTICA_SMOKE_DATABASE_NAME" ] || die "Smoke database identity mismatch."
}

smoke_prepare_environment() {
  local key
  acquire_lock
  trap release_lock EXIT
  OFFSET="$(allocate_offset "$REPO_ROOT")" || die "No free smoke environment ports."
  NAME=smoke
  DIR="$REPO_ROOT"
  OWNER=agent
  CREATED_AT="$(now_iso)"
  TTL_HOURS=24
  EXPIRES_AT="$(expires_at_after_hours "$TTL_HOURS")"
  BACKEND_PORT=$((18080 + OFFSET))
  FRONTEND_PORT=$((13000 + OFFSET))
  DB_NAME="$MULTICA_SMOKE_DATABASE_NAME"
  PROFILE=smoke
  WEB_MODE=production
  bind_paths
  ENV_FILE="$STATE_DIR/smoke.env"
  export PORT="$BACKEND_PORT" POSTGRES_DB="$DB_NAME" FRONTEND_PORT
  export FRONTEND_ORIGIN="http://localhost:$FRONTEND_PORT"
  export CORS_ALLOWED_ORIGINS="$FRONTEND_ORIGIN" ALLOWED_ORIGINS="$FRONTEND_ORIGIN"
  export PLAYWRIGHT_BASE_URL="$FRONTEND_ORIGIN" MULTICA_APP_URL="$FRONTEND_ORIGIN"
  export MULTICA_PUBLIC_URL="http://localhost:$BACKEND_PORT" LOCAL_UPLOAD_BASE_URL="http://localhost:$BACKEND_PORT"
  export NEXT_PUBLIC_API_URL="http://localhost:$BACKEND_PORT" REMOTE_API_URL="http://localhost:$BACKEND_PORT"
  export NEXT_PUBLIC_WS_URL="ws://localhost:$BACKEND_PORT/ws"
  # This canonical file is generated from already validated environment values;
  # neither caller dotenv nor Make include/eval participates in startup.
  for key in DATABASE_URL PORT POSTGRES_DB FRONTEND_PORT FRONTEND_ORIGIN CORS_ALLOWED_ORIGINS ALLOWED_ORIGINS \
    PLAYWRIGHT_BASE_URL MULTICA_APP_URL MULTICA_PUBLIC_URL LOCAL_UPLOAD_BASE_URL NEXT_PUBLIC_API_URL REMOTE_API_URL \
    NEXT_PUBLIC_WS_URL MULTICA_AUTH_MODE MULTICA_DEV_VERIFICATION_CODE LOCAL_UPLOAD_DIR JWT_SECRET; do
    write_manifest_value "$key" "${!key}" >> "$ENV_FILE"
  done
  chmod 600 "$ENV_FILE"
  save_manifest
  release_lock
  trap - EXIT
}

smoke_verify_rendered_config() {
  node --input-type=module - "$FRONTEND_ORIGIN" "$NEXT_PUBLIC_API_URL" "$NEXT_PUBLIC_WS_URL" "$MULTICA_SMOKE_RUN_DIR/rendered.json" <<'JS'
import { writeFileSync } from 'node:fs';
const [origin, api, ws, file] = process.argv.slice(2);
const response = await fetch(origin, {signal:AbortSignal.timeout(15000)});
if (!response.ok) throw new Error('Production Web did not respond');
const html = (await response.text()).replaceAll('\\"','"');
// The streamed RSC payload supplies these props to WebProviders. Verify the
// rendered values, not only the server environment or proxy health.
const values = key => [...html.matchAll(new RegExp('"'+key+'":"([^"\\\\]*)"','g'))].map(m => m[1]);
const apis=values('apiBaseUrl'), sockets=values('wsUrl');
if (!apis.length || !sockets.length || apis.some(v=>v!==api) || sockets.some(v=>v!==ws)) throw new Error('Rendered API/WS configuration mismatch');
writeFileSync(file, JSON.stringify({origin,apiBaseUrl:api,wsUrl:ws},null,2)+'\n',{flag:'wx',mode:0o600});
JS
}

smoke_identity() {
  local source api web
  source="$(checkout_source_id)" || return $?
  api="$(api_configuration_id)" || return $?
  web="$(web_configuration_id)" || return $?
  [ -n "$source" ] && [ -n "$api" ] && [ -n "$web" ] || return 1
  printf '%s\n%s\n%s\n' "$source" "$api" "$web"
}

smoke_main() {
  local tool before after
  : "${MULTICA_SMOKE_RUN_DIR:?run directory required}"
  : "${MULTICA_SMOKE_DATABASE_NAME:?owned database name required}"
  : "${DATABASE_URL:?explicit database required}"
  : "${PLAYWRIGHT_BROWSERS_PATH:?explicit browser installation required}"
  for tool in node go pnpm psql lsof curl git; do
    command -v "$tool" >/dev/null 2>&1 || die "Missing smoke prerequisite: $tool."
  done
  cd "$REPO_ROOT"
  smoke_verify_database
  smoke_prepare_environment
  trap smoke_cleanup EXIT
  trap 'exit 130' INT
  trap 'exit 143' TERM
  before="$(smoke_identity)"
  migrate_database
  SMOKE_STARTED_API=true
  start_api
  SMOKE_STARTED_WEB=true
  start_web
  smoke_verify_rendered_config
  smoke_verify_database
  SMOKE_TEST_EXIT=0
  pnpm exec playwright test --config playwright.agent-critical-path.config.mjs --workers=1 --retries=0 || SMOKE_TEST_EXIT=$?
  SMOKE_INVENTORY_EXIT=0
  node "$REPO_ROOT/scripts/check-agent-critical-path.mjs" "$MULTICA_SMOKE_RUN_DIR/playwright.json" \
    > "$MULTICA_SMOKE_RUN_DIR/inventory.json" || SMOKE_INVENTORY_EXIT=$?
  after="$(smoke_identity)"
  node - "$MULTICA_SMOKE_RUN_DIR/identity.json" "$before" "$after" "$(checkout_commit)" <<'JS'
const fs = require('node:fs');
const [file,before,after,commit] = process.argv.slice(2);
fs.writeFileSync(file,JSON.stringify({commit,before:before.split('\n'),after:after.split('\n'),matches:before===after},null,2)+'\n',{flag:'wx',mode:0o600});
JS
  [ "$before" = "$after" ] || die "Source or configuration changed during smoke; retained run cannot be accepted."
  [ "$SMOKE_TEST_EXIT" -eq 0 ] && [ "$SMOKE_INVENTORY_EXIT" -eq 0 ] || return 1
  SMOKE_COMPLETE=true
}

if [ "${BASH_SOURCE[0]}" = "$0" ]; then smoke_main "$@"; fi
