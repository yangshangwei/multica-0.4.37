#!/usr/bin/env bash
set -euo pipefail

TASK_RESEARCH_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TASK_PROJECT_ROOT="$(git -C "$TASK_RESEARCH_DIR" rev-parse --show-toplevel)"
TASK_RECORD="${1:-$TASK_RESEARCH_DIR/qa-isolated-environment.json}"
TASK_ENV_NAME="$(node -e 'process.stdout.write(require(process.argv[1]).name)' "$TASK_RECORD")"
TASK_SOURCE_ROOT="$(node -e 'process.stdout.write(require(process.argv[1]).source_root)' "$TASK_RECORD")"
export PATH="/opt/homebrew/Cellar/libpq/18.6/bin:$PATH"
cd "$TASK_PROJECT_ROOT"
source "$TASK_PROJECT_ROOT/scripts/check.sh"
load_manifest "$TASK_ENV_NAME"
test "$DIR" = "$TASK_SOURCE_ROOT"
bind_paths
load_env_file "$ENV_FILE" "$DIR"

# The check renderer has its own launcher; ordinary make down stops desktop.
for TASK_COMPONENT in desktop-renderer web api; do
  case "$TASK_COMPONENT" in
    desktop-renderer) TASK_PORT="$DESKTOP_RENDERER_PORT" ;;
    web) TASK_PORT="$FRONTEND_PORT" ;;
    api) TASK_PORT="$BACKEND_PORT" ;;
  esac
  if ! port_free "$TASK_PORT"; then
    listener_belongs_to_component "$TASK_COMPONENT" "$TASK_PORT" \
      || die "Refusing to stop a listener outside this task's recorded ownership."
  fi
  stop_component "$TASK_COMPONENT"
  port_free "$TASK_PORT" || die "Task listener remains after cleanup."
done

node - "$TASK_RECORD" <<'JS'
const fs = require('node:fs');
const file = process.argv[2];
const record = JSON.parse(fs.readFileSync(file, 'utf8'));
record.components_stopped = ['desktop-renderer', 'web', 'api'];
record.stopped_at = new Date().toISOString();
record.ports_free = true;
fs.writeFileSync(file, JSON.stringify(record, null, 2) + '\n');
JS
