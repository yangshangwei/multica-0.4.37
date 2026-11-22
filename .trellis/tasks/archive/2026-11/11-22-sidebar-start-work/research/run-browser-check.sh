#!/usr/bin/env bash
set -euo pipefail

# Run from the repository root. Reuse the existing environment allocator and
# provenance checks, while limiting validation to this sidebar increment.
source scripts/check.sh

sidebar_evidence_dir="${SIDEBAR_EVIDENCE_DIR:-$REPO_ROOT/.trellis/tasks/11-22-sidebar-start-work/research}"
sidebar_artifact_dir="${SIDEBAR_ARTIFACT_DIR:-$REPO_ROOT/artifacts/qa/sidebar-start-work-20261122}"
mkdir -p "$sidebar_evidence_dir" "$sidebar_artifact_dir"
sidebar_source_env="$(env_file_path "${ENV_FILE:-$(detect_env_file)}")"
[ -f "$sidebar_source_env" ] || die "Missing sidebar verification source environment"
for sidebar_tool in node go pnpm psql lsof make curl cmp; do
  command -v "$sidebar_tool" >/dev/null 2>&1 || die "Missing prerequisite: $sidebar_tool"
done
load_env_file "$sidebar_source_env"
prepare_check_environment "$sidebar_source_env"
trap check_cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM
mkdir -p "$DEV_TMPDIR"
export TMPDIR="$DEV_TMPDIR" TMP="$DEV_TMPDIR" TEMP="$DEV_TMPDIR"

step "Sidebar verification API"
ensure_database
migrate_database
CHECK_STARTED_API=true
start_api
info "Sidebar verification environment: $NAME"
print_status_json > "$sidebar_evidence_dir/verification.running.json"

# The coordinator admits the final code after focused RED/GREEN checks. No
# build can accidentally capture a partially edited shared component.
sidebar_build_admission="${SIDEBAR_BUILD_ADMISSION:-}"
if [ "$sidebar_build_admission" != build ]; then
  info "API ready; waiting for final-build admission from the coordinator."
  IFS= read -r sidebar_build_admission
fi
[ "$sidebar_build_admission" = build ] || die "Final build was not admitted"

step "Sidebar production Web and Desktop"
info "Desktop build log: $LOG_DIR/desktop-build.log"
pnpm --filter @multica/desktop build > "$LOG_DIR/desktop-build.log" 2>&1 || {
  tail -n 60 "$LOG_DIR/desktop-build.log"
  die "Sidebar Desktop build failed"
}
CHECK_STARTED_RENDERER=true
start_check_desktop_renderer
CHECK_STARTED_WEB=true
pnpm --filter @multica/web mdx > "$LOG_DIR/web-mdx.log" 2>&1 || die "Sidebar MDX setup failed"
start_web
print_status_json > "$sidebar_evidence_dir/verification.running.json"

step "Sidebar focused browser acceptance"
pnpm exec playwright test e2e/global-scope-navigation.spec.ts e2e/collaboration-creation-default.spec.ts e2e/settings-preferences.spec.ts \
  --workers=1 --retries=0 --reporter=line \
  --output="$sidebar_artifact_dir/browser-results"
CHECK_COMPLETE=true
