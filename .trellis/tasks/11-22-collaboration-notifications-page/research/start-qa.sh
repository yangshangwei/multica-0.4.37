#!/usr/bin/env bash
set -euo pipefail

TASK_MODE="${1:-}"
case "$TASK_MODE" in api|clients) ;; *) exit 2 ;; esac
TASK_RESEARCH_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TASK_PROJECT_ROOT="$(git -C "$TASK_RESEARCH_DIR" rev-parse --show-toplevel)"
TASK_ENV_NAME="$(node -e 'process.stdout.write(require(process.argv[1]).name)' "$TASK_RESEARCH_DIR/qa-environment.json")"
export PATH="/opt/homebrew/Cellar/libpq/18.6/bin:$PATH"
cd "$TASK_PROJECT_ROOT"
source "$TASK_PROJECT_ROOT/scripts/check.sh"
load_manifest "$TASK_ENV_NAME"
bind_paths
load_env_file "$ENV_FILE"
export PORT="$BACKEND_PORT" FRONTEND_PORT DATABASE_URL POSTGRES_DB="$DB_NAME"
mkdir -p "$DEV_TMPDIR"
export TMPDIR="$DEV_TMPDIR" TMP="$DEV_TMPDIR" TEMP="$DEV_TMPDIR"
test "$(psql "$DATABASE_URL" -X -Atqc 'SELECT current_database()')" = "$DB_NAME"

if [ "$TASK_MODE" = api ]; then
  start_api
else
  # Build only once the implementation/test source is stable. The registry
  # rejects replacing any other owned listener's shared build output.
  start_api
  pnpm --filter @multica/desktop exec electron-vite build > "$LOG_DIR/desktop-build.log" 2>&1
  start_check_desktop_renderer
  start_web
fi
print_status_json > "$TASK_RESEARCH_DIR/qa-running.json"
node - "$TASK_RESEARCH_DIR/qa-environment.json" "$TASK_MODE" "$BACKEND_PORT" "$FRONTEND_PORT" "$DESKTOP_RENDERER_PORT" <<'JS'
const fs = require('node:fs');
const [file, mode, api, web, desktop] = process.argv.slice(2);
const record = JSON.parse(fs.readFileSync(file, 'utf8'));
record.api_url = `http://localhost:${api}`;
record.web_url = `http://localhost:${web}`;
record.renderer_url = `http://127.0.0.1:${desktop}`;
record.components_started = [...new Set([...record.components_started, ...(mode === 'api' ? ['api'] : ['desktop-renderer', 'web'])])];
fs.writeFileSync(file, JSON.stringify(record, null, 2) + '\n');
console.log(JSON.stringify(record, null, 2));
JS
