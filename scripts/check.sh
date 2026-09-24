#!/usr/bin/env bash
set -euo pipefail

# Ordered local verification using the environment registry's allocation and
# process ownership rules. No existing service is a verification target.
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=dev-env.sh
. "$SCRIPT_DIR/dev-env.sh"

CHECK_STARTED_API=false
CHECK_STARTED_WEB=false
CHECK_GO_DB_CREATED=false
CHECK_COMPLETE=false

check_cleanup() {
  local result=$? cleanup_failed=0
  trap - EXIT INT TERM
  if [ "$CHECK_STARTED_WEB" = true ]; then stop_component web || cleanup_failed=1; fi
  if [ "$CHECK_STARTED_API" = true ]; then stop_component api || cleanup_failed=1; fi
  if [ "$CHECK_GO_DB_CREATED" = true ]; then
    # Never force-disconnect consumers; a surviving connection is a cleanup
    # failure worth reporting, rather than permission to kill another process.
    info "Dropping isolated Go database $CHECK_GO_DB_NAME"
    psql "$(admin_database_url "$DATABASE_URL")" -v ON_ERROR_STOP=1 \
      -c "DROP DATABASE \"$CHECK_GO_DB_NAME\"" >/dev/null || cleanup_failed=1
  fi
  if [ "$result" -eq 0 ] && { [ "$CHECK_COMPLETE" != true ] || [ "$cleanup_failed" -ne 0 ]; }; then result=1; fi
  if [ "$result" -eq 0 ]; then
    echo "✓ All checks passed."
  else
    echo "✗ Checks FAILED (exit $result)."
  fi
  exit "$result"
}

prepare_check_environment() {
  local source_env=$1 offset task_id
  task_id="$(date -u '+%Y%m%d%H%M%S')-$$"
  acquire_lock
  trap release_lock EXIT
  offset="$(allocate_offset "$REPO_ROOT")" || die "No free verification slot."
  NAME="check-$task_id"
  DIR="$REPO_ROOT"
  CREATED_AT="$(now_iso)"
  OWNER=agent
  TTL_HOURS=24
  EXPIRES_AT="$(expires_at_after_hours "$TTL_HOURS")"
  OFFSET="$offset"
  BACKEND_PORT=$((18080 + offset))
  FRONTEND_PORT=$((13000 + offset))
  DB_NAME="multica_check_${task_id//-/_}_api"
  CHECK_GO_DB_NAME="multica_check_${task_id//-/_}_go"
  PROFILE="dev-$NAME"
  WEB_MODE=production
  bind_paths
  ENV_FILE="$STATE_DIR/check.env"
  cp "$source_env" "$ENV_FILE"
  printf '\n' >> "$ENV_FILE"
  # Use task-only database/URLs and classic auth. Explicit trailing assignments
  # also cover older env files where those settings did not exist yet.
  DATABASE_URL="$(database_url_with_name "$DATABASE_URL" "$DB_NAME")"
  {
    write_manifest_value PORT "$BACKEND_PORT"
    write_manifest_value BACKEND_PORT "$BACKEND_PORT"
    write_manifest_value FRONTEND_PORT "$FRONTEND_PORT"
    write_manifest_value POSTGRES_DB "$DB_NAME"
    write_manifest_value GO_TEST_DB_NAME "$CHECK_GO_DB_NAME"
    write_manifest_value DATABASE_URL "$DATABASE_URL"
    write_manifest_value FRONTEND_ORIGIN "http://localhost:$FRONTEND_PORT"
    write_manifest_value CORS_ALLOWED_ORIGINS "http://localhost:$FRONTEND_PORT"
    write_manifest_value ALLOWED_ORIGINS "http://localhost:$FRONTEND_PORT"
    write_manifest_value PLAYWRIGHT_BASE_URL "http://localhost:$FRONTEND_PORT"
    write_manifest_value MULTICA_PUBLIC_URL "http://localhost:$BACKEND_PORT"
    write_manifest_value MULTICA_APP_URL "http://localhost:$FRONTEND_PORT"
    write_manifest_value MULTICA_SERVER_URL "ws://localhost:$BACKEND_PORT/ws"
    write_manifest_value LOCAL_UPLOAD_BASE_URL "http://localhost:$BACKEND_PORT"
    write_manifest_value NEXT_PUBLIC_API_URL "http://localhost:$BACKEND_PORT"
    write_manifest_value NEXT_PUBLIC_WS_URL "ws://localhost:$BACKEND_PORT/ws"
    write_manifest_value REMOTE_API_URL "http://localhost:$BACKEND_PORT"
    write_manifest_value MULTICA_DEVICE_AUTH_ENABLED false
    write_manifest_value MULTICA_DEV_VERIFICATION_CODE "$DEV_CODE_DEFAULT"
    write_manifest_value APP_ENV development
    # Test accounts share a loopback IP. These overrides only reach this API.
    write_manifest_value RATE_LIMIT_AUTH 1000
    write_manifest_value RATE_LIMIT_AUTH_VERIFY 1000
  } >> "$ENV_FILE"
  chmod 600 "$ENV_FILE"
  save_manifest
  release_lock
  trap - EXIT
  load_env_file "$ENV_FILE"
  export PORT="$BACKEND_PORT" FRONTEND_PORT DATABASE_URL POSTGRES_DB="$DB_NAME"
  CHECK_GO_DATABASE_URL="$(database_url_with_name "$DATABASE_URL" "$CHECK_GO_DB_NAME")"
}

check_main() {
  local source_env tool
  cd "$REPO_ROOT"
  source_env="$(env_file_path "${ENV_FILE:-$(detect_env_file)}")"
  [ -f "$source_env" ] || die "Missing env file: $source_env"
  for tool in node go pnpm psql lsof make; do
    command -v "$tool" >/dev/null 2>&1 || die "Missing prerequisite: $tool (add its installed bin directory to PATH)."
  done
  load_env_file "$source_env"
  prepare_check_environment "$source_env"
  trap check_cleanup EXIT
  trap 'exit 130' INT
  trap 'exit 143' TERM
  info "Verification environment: $NAME; logs and provenance: $STATE_DIR"
  mkdir -p "$DEV_TMPDIR"
  export TMPDIR="$DEV_TMPDIR" TMP="$DEV_TMPDIR" TEMP="$DEV_TMPDIR"

  step "[1/6] Static checks"
  pnpm exec turbo run lint typecheck --filter='!@multica/mobile' --concurrency=1 --force
  pnpm check:ui-exports

  step "[2/6] TypeScript tests (one package, two workers)"
  pnpm exec turbo run test --filter='!@multica/mobile' --concurrency=1 --force -- --maxWorkers=2

  step "[3/6] Script regressions and isolated Go tests"
  bash "$SCRIPT_DIR/test-go.test.sh"
  bash "$SCRIPT_DIR/dev-env.test.sh"
  bash "$SCRIPT_DIR/check.test.sh"
  ensure_database
  psql "$(admin_database_url "$DATABASE_URL")" -v ON_ERROR_STOP=1 \
    -c "CREATE DATABASE \"$CHECK_GO_DB_NAME\"" >/dev/null
  CHECK_GO_DB_CREATED=true
  (
    export DATABASE_URL="$CHECK_GO_DATABASE_URL" POSTGRES_DB="$CHECK_GO_DB_NAME"
    migrate_database
    bash "$SCRIPT_DIR/test-go.sh" --race
    cd "$REPO_ROOT/server"
    go vet -p 2 ./...
  )

  step "[4/6] Isolated API"
  migrate_database
  CHECK_STARTED_API=true
  start_api

  step "[5/6] Production Web build and start"
  CHECK_STARTED_WEB=true
  start_web
  print_status_json > "$STATE_DIR/verification.running.json"

  step "[6/6] Playwright (one worker, zero retries)"
  pnpm exec playwright test --workers=1 --retries=0 "$@"
  CHECK_COMPLETE=true
}

if [ "${BASH_SOURCE[0]}" = "$0" ]; then check_main "$@"; fi
