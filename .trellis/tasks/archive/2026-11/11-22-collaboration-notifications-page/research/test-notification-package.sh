#!/usr/bin/env bash
set -euo pipefail

TASK_ROLE="${1:-}"
case "$TASK_ROLE" in handler|service) ;; *) exit 2 ;; esac
TASK_RESEARCH_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TASK_PROJECT_ROOT="$(git -C "$TASK_RESEARCH_DIR" rev-parse --show-toplevel)"
TASK_ENV_FILE="$(node -e 'const r=require(process.argv[1]); process.stdout.write(r[process.argv[2]+"_environment"])' \
  "$TASK_RESEARCH_DIR/qa-environment.json" "$TASK_ROLE")"
export PATH="/opt/homebrew/Cellar/libpq/18.6/bin:$PATH"
cd "$TASK_PROJECT_ROOT"
source "$TASK_PROJECT_ROOT/scripts/dev-env.sh"
load_env_file "$TASK_ENV_FILE"
test "$(psql "$DATABASE_URL" -X -Atqc 'SELECT current_database()')" = "$TASK_DATABASE_NAME"
unset DATABASE_MAX_CONNS DATABASE_MIN_CONNS MULTICA_RESOURCE_PUBLISH_DIR
mkdir -p "$DEV_TMPDIR"
export TMPDIR="$DEV_TMPDIR" TMP="$DEV_TMPDIR" TEMP="$DEV_TMPDIR"
exec "${CLEAN_ENV[@]}" bash "$TASK_PROJECT_ROOT/scripts/go-test-with-agent-cli-guard.sh" -- \
  go -C server test "./internal/$TASK_ROLE" -run '^TestCollaborationNotification' -count=1 -v
