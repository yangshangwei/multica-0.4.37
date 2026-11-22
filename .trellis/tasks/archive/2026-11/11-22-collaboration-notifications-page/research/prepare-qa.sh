#!/usr/bin/env bash
set -euo pipefail

TASK_RESEARCH_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TASK_PROJECT_ROOT="$(git -C "$TASK_RESEARCH_DIR" rev-parse --show-toplevel)"
TASK_PG_BIN=/opt/homebrew/Cellar/libpq/18.6/bin
test -x "$TASK_PG_BIN/psql"
export PATH="$TASK_PG_BIN:$PATH"
cd "$TASK_PROJECT_ROOT"

# Reuse the registry's allocation and private configuration contract only.
# This script does not run check_main or start application listeners.
source "$TASK_PROJECT_ROOT/scripts/check.sh"
TASK_SOURCE_ENV="$(env_file_path "$(detect_env_file)")"
load_env_file "$TASK_SOURCE_ENV"
node <<'JS'
const url = new URL(process.env.DATABASE_URL ?? '');
if (!['postgres:', 'postgresql:'].includes(url.protocol) ||
    !['localhost', '127.0.0.1', '[::1]'].includes(url.hostname) ||
    ['host', 'hostaddr', 'port', 'dbname', 'service'].some(key => url.searchParams.has(key))) {
  console.error('Task QA requires the recorded loopback PostgreSQL endpoint.');
  process.exit(2);
}
JS
prepare_check_environment "$TASK_SOURCE_ENV"
mkdir -p "$DEV_TMPDIR"
export TMPDIR="$DEV_TMPDIR" TMP="$DEV_TMPDIR" TEMP="$DEV_TMPDIR"

TASK_ADMIN_URL="$(admin_database_url "$DATABASE_URL")"
PGCONNECT_TIMEOUT=3 psql "$TASK_ADMIN_URL" -X -Atqc \
  'SELECT count(*) AS current_connections FROM pg_stat_activity' \
  > "$TASK_RESEARCH_DIR/qa-connection-count.txt" 2> "$TASK_RESEARCH_DIR/qa-connection-probe.log"
ensure_database
TASK_API_ENV="$ENV_FILE"
TASK_API_DB="$DB_NAME"
TASK_STATE_DIR="$STATE_DIR"
TASK_ENV_NAME="$NAME"

# One new migrated database per DB-backed package; never borrow the API data.
for TASK_PACKAGE in handler service; do
  TASK_DATABASE_NAME="${CHECK_GO_DB_NAME}_${TASK_PACKAGE}"
  TASK_DATABASE_URL="$(database_url_with_name "$DATABASE_URL" "$TASK_DATABASE_NAME")"
  TASK_TEST_ENV="$TASK_STATE_DIR/notifications-${TASK_PACKAGE}.env"
  PGCONNECT_TIMEOUT=3 psql "$TASK_ADMIN_URL" -X -v ON_ERROR_STOP=1 \
    -c "CREATE DATABASE \"$TASK_DATABASE_NAME\"" \
    > "$TASK_RESEARCH_DIR/qa-${TASK_PACKAGE}-create.log" 2>&1
  cp "$TASK_API_ENV" "$TASK_TEST_ENV"
  {
    printf '\n'
    write_manifest_value DATABASE_URL "$TASK_DATABASE_URL"
    write_manifest_value POSTGRES_DB "$TASK_DATABASE_NAME"
    write_manifest_value TASK_DATABASE_NAME "$TASK_DATABASE_NAME"
  } >> "$TASK_TEST_ENV"
  chmod 600 "$TASK_TEST_ENV"
  (
    load_env_file "$TASK_TEST_ENV"
    test "$(psql "$DATABASE_URL" -X -Atqc 'SELECT current_database()')" = "$TASK_DATABASE_NAME"
    go -C server run ./cmd/migrate up \
      > "$TASK_RESEARCH_DIR/qa-${TASK_PACKAGE}-migrate.log" 2>&1
  )
  printf 'Prepared isolated %s database: %s\n' "$TASK_PACKAGE" "$TASK_DATABASE_NAME"
done

test "$(psql "$DATABASE_URL" -X -Atqc 'SELECT current_database()')" = "$TASK_API_DB"
go -C server run ./cmd/migrate up > "$TASK_RESEARCH_DIR/qa-api-migrate.log" 2>&1

TASK_ENV_NAME="$TASK_ENV_NAME" TASK_STATE_DIR="$TASK_STATE_DIR" \
TASK_API_ENV="$TASK_API_ENV" TASK_API_DB="$TASK_API_DB" \
TASK_RESEARCH_DIR="$TASK_RESEARCH_DIR" node <<'JS'
const fs = require('node:fs');
const path = require('node:path');
const state = process.env.TASK_STATE_DIR;
const record = {
  name: process.env.TASK_ENV_NAME,
  api_environment: process.env.TASK_API_ENV,
  api_database: process.env.TASK_API_DB,
  handler_environment: path.join(state, 'notifications-handler.env'),
  service_environment: path.join(state, 'notifications-service.env'),
  stop_command: `make down ARGS="${process.env.TASK_ENV_NAME}"`,
  components_started: [],
};
fs.writeFileSync(path.join(process.env.TASK_RESEARCH_DIR, 'qa-environment.json'), JSON.stringify(record, null, 2) + '\n');
console.log(JSON.stringify(record, null, 2));
JS
