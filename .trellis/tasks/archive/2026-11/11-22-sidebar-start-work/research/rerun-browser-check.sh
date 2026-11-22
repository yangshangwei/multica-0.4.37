#!/usr/bin/env bash
set -euo pipefail

# Reuse the exact production candidate after a test-only correction. The
# coordinator supplies the named environment and has verified source hashes.
source scripts/check.sh

sidebar_evidence_dir="${SIDEBAR_EVIDENCE_DIR:?}"
sidebar_artifact_dir="${SIDEBAR_ARTIFACT_DIR:?}"
sidebar_environment="${SIDEBAR_VERIFICATION_ENV:?}"
resolve_env_for_read "$sidebar_environment"
load_env_file "$ENV_FILE" "$DIR"
export PORT="$BACKEND_PORT" FRONTEND_PORT DATABASE_URL POSTGRES_DB="$DB_NAME"
mkdir -p "$DEV_TMPDIR" "$sidebar_artifact_dir"
export TMPDIR="$DEV_TMPDIR" TMP="$DEV_TMPDIR" TEMP="$DEV_TMPDIR"
trap check_cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

ensure_database
CHECK_STARTED_API=true
start_api
if [ "${SIDEBAR_REBUILD:-0}" = 1 ]; then
  info "Rebuilding the final copied Desktop source: $LOG_DIR/desktop-build-final.log"
  pnpm --filter @multica/desktop build > "$LOG_DIR/desktop-build-final.log" 2>&1 || {
    tail -n 60 "$LOG_DIR/desktop-build-final.log"
    die "Final Sidebar Desktop build failed"
  }
fi
CHECK_STARTED_RENDERER=true
start_check_desktop_renderer
CHECK_STARTED_WEB=true
if [ "${SIDEBAR_REBUILD:-0}" = 1 ]; then
  pnpm --filter @multica/web mdx > "$LOG_DIR/web-mdx-final.log" 2>&1 || die "Final Sidebar MDX setup failed"
  start_web
else
  sidebar_build_id="$(cat "$REPO_ROOT/apps/web/.next/BUILD_ID")"
  sidebar_source_id="$(checkout_source_id)"
  sidebar_configuration_id="$(web_configuration_id)"
  launch_detached web pnpm --dir "$REPO_ROOT" --filter @multica/web exec next start --port "$FRONTEND_PORT"
  write_component_provenance web production "$sidebar_build_id" \
    "pnpm --filter @multica/web exec next start --port $FRONTEND_PORT (verified unchanged runtime source; test-only update)" \
    "$sidebar_source_id" "$sidebar_configuration_id"
  sidebar_waited=0
  while [ "$sidebar_waited" -lt 120 ]; do
    if curl -sf --max-time 3 "http://localhost:$FRONTEND_PORT" >/dev/null 2>&1; then
      listener_belongs_to_component web "$FRONTEND_PORT" || die "Sidebar Web listener ownership differs"
      record_component_listener web "$(port_listener_pid "$FRONTEND_PORT")"
      break
    fi
    component_pid web >/dev/null || die "Sidebar Web exited"
    sleep 1
    sidebar_waited=$((sidebar_waited + 1))
  done
  [ "$sidebar_waited" -lt 120 ] || die "Sidebar Web did not become ready"
fi
print_status_json > "$sidebar_evidence_dir/verification.running.json"
export PLAYWRIGHT_JSON_OUTPUT_NAME="$sidebar_artifact_dir/report.json"
pnpm exec playwright test "$@" --workers=1 --retries=0 --reporter=line,json --output="$sidebar_artifact_dir/browser-results"
CHECK_COMPLETE=true
