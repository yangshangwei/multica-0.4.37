#!/usr/bin/env bash
set -euo pipefail

TASK_RESEARCH_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TASK_PROJECT_ROOT="$(git -C "$TASK_RESEARCH_DIR" rev-parse --show-toplevel)"
TASK_ENV_NAME="$(node -e 'process.stdout.write(require(process.argv[1]).name)' "$TASK_RESEARCH_DIR/qa-environment.json")"
export PATH="/opt/homebrew/Cellar/libpq/18.6/bin:$PATH"
cd "$TASK_PROJECT_ROOT"
source "$TASK_PROJECT_ROOT/scripts/check.sh"
load_manifest "$TASK_ENV_NAME"
bind_paths
load_env_file "$ENV_FILE"
export PORT="$BACKEND_PORT" FRONTEND_PORT DATABASE_URL POSTGRES_DB="$DB_NAME" TASK_DATABASE_NAME="$DB_NAME"
export COLLABORATION_E2E_API_URL="http://localhost:$BACKEND_PORT"
export COLLABORATION_ELECTRON_RENDERER_URL="http://127.0.0.1:$DESKTOP_RENDERER_PORT"
export COLLABORATION_ELECTRON_WS_URL="ws://localhost:$BACKEND_PORT/ws"
export PLAYWRIGHT_BASE_URL="http://localhost:$FRONTEND_PORT"
mkdir -p "$DEV_TMPDIR"
export TMPDIR="$DEV_TMPDIR" TMP="$DEV_TMPDIR" TEMP="$DEV_TMPDIR"
test "$(psql "$DATABASE_URL" -X -Atqc 'SELECT current_database()')" = "$DB_NAME"
exec "${CLEAN_ENV[@]}" "$@"
