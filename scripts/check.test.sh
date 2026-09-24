#!/usr/bin/env bash
# Exercise orchestration and failure exits without building or starting services.
set -euo pipefail
root_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
test_dir="$(mktemp -d)"
trap 'rm -rf "$test_dir"' EXIT
cat > "$test_dir/source.env" <<'ENV'
DATABASE_URL=postgres://test:test@localhost:5432/original
CORS_ALLOWED_ORIGINS=http://localhost:13000
ALLOWED_ORIGINS=http://localhost:13001
ENV

cat > "$test_dir/fixture.sh" <<'FIXTURE'
#!/usr/bin/env bash
set -euo pipefail
source "$1/scripts/check.sh"
fixture_dir=$2
phase=$3
export ENV_FILE="$fixture_dir/source.env"
CHECK_CALLS="$fixture_dir/calls"
record() { printf '%s\n' "$*" >> "$CHECK_CALLS"; }
prepare_check_environment() {
  NAME=check-fixture
  STATE_DIR="$fixture_dir"
  LOG_DIR="$fixture_dir"
  DATABASE_URL=postgres://test:test@localhost:5432/api_fixture
  CHECK_GO_DATABASE_URL=postgres://test:test@localhost:5432/go_fixture
  CHECK_GO_DB_NAME=go_fixture
  DB_NAME=api_fixture
}
pnpm() { record "pnpm $*"; [ "$phase" != ts-failure ] || return 17; }
psql() { record "psql $*"; }
lsof() { :; }
go() { record "go $*"; }
bash() {
  record "bash $* database=$DATABASE_URL"
  case "$*" in *test-go.sh*) [ "$DATABASE_URL" = "$CHECK_GO_DATABASE_URL" ] || return 18 ;; esac
}
ensure_database() { record database; [ "$phase" != database-failure ] || return 19; }
migrate_database() { record "migrate $DATABASE_URL"; }
start_api() { record api; [ "$phase" != api-failure ] || return 23; }
start_web() { record web; [ "$phase" != web-failure ] || return 29; }
stop_component() { record "stop $1"; [ "$phase" != cleanup-failure ] || return 31; }
print_status_json() { printf '{}\n'; }
check_main
FIXTURE

run_case() {
  local phase=$1 expected=$2 status=0
  : > "$test_dir/calls"
  bash "$test_dir/fixture.sh" "$root_dir" "$test_dir" "$phase" > "$test_dir/output" 2>&1 || status=$?
  [ "$status" -eq "$expected" ] || { cat "$test_dir/output"; echo "$phase: exit $status, expected $expected" >&2; exit 1; }
  if [ "$expected" -eq 0 ]; then
    grep -Fq 'All checks passed' "$test_dir/output"
  else
    grep -Fq 'Checks FAILED' "$test_dir/output"
    if grep -Fq 'All checks passed' "$test_dir/output"; then echo 'False success after failure' >&2; exit 1; fi
  fi
}

run_case ts-failure 17
! grep -Eq '^api$|^web$|^stop ' "$test_dir/calls" || { echo "Unexpected match in rejection assertion" >&2; exit 1; }
run_case database-failure 19
! grep -Eq '^api$|^web$|^stop ' "$test_dir/calls" || { echo "Unexpected match in rejection assertion" >&2; exit 1; }
run_case api-failure 23
! grep -Eq '^web$|^pnpm exec playwright' "$test_dir/calls" || { echo "Unexpected match in rejection assertion" >&2; exit 1; }
grep -Fxq 'stop api' "$test_dir/calls"
run_case web-failure 29
! grep -Eq '^pnpm exec playwright' "$test_dir/calls" || { echo "Unexpected match in rejection assertion" >&2; exit 1; }
grep -Fxq 'stop web' "$test_dir/calls"
run_case cleanup-failure 1
run_case success 0
grep -Fxq 'pnpm exec playwright test --workers=1 --retries=0' "$test_dir/calls"
grep -Fq -- '--concurrency=1 --force -- --maxWorkers=2' "$test_dir/calls"
grep -Fxq 'migrate postgres://test:test@localhost:5432/go_fixture' "$test_dir/calls"
grep -Fxq 'migrate postgres://test:test@localhost:5432/api_fixture' "$test_dir/calls"

# The task copy changes auth and database settings without touching the source.
bash -s -- "$root_dir" "$test_dir" <<'FIXTURE'
set -euo pipefail
source "$1/scripts/check.sh"
fixture_dir=$2
REPO_ROOT="$fixture_dir"
DEV_HOME="$fixture_dir/registry"
ENVS_DIR="$DEV_HOME/envs"
LOCK_DIR="$DEV_HOME/lock.d"
load_env_file() { set -a; source "$1"; set +a; }
allocate_offset() { printf 907; }
load_env_file "$fixture_dir/source.env"
prepare_check_environment "$fixture_dir/source.env"
[ "$MULTICA_DEVICE_AUTH_ENABLED" = false ]
[ "$DATABASE_URL" != "$CHECK_GO_DATABASE_URL" ]
[ "$WEB_MODE" = production ]
[ "$CORS_ALLOWED_ORIGINS" = "http://localhost:$FRONTEND_PORT" ]
[ "$ALLOWED_ORIGINS" = "http://localhost:$FRONTEND_PORT" ]
grep -Fxq 'CORS_ALLOWED_ORIGINS=http://localhost:13000' "$fixture_dir/source.env"
grep -Fxq 'ALLOWED_ORIGINS=http://localhost:13001' "$fixture_dir/source.env"
! grep -q 'MULTICA_DEVICE_AUTH_ENABLED' "$fixture_dir/source.env" || { echo "Unexpected match in rejection assertion" >&2; exit 1; }
grep -Fq 'WEB_MODE=production' "$STATE_DIR/manifest.env"
FIXTURE


# A real Make parent must not reparse the Bash-escaped task env on API launch.
# A query string, escaped password and ampersand all survive unchanged.
mkdir -p "$test_dir/make-case/bin" "$test_dir/make-case/server"
cat > "$test_dir/make-case/Makefile" <<'MAKE'
.PHONY: verify
verify:
	@bash "$(FIXTURE)" "$(REPO)" "$(CASE_DIR)"
MAKE
cat > "$test_dir/make-case/bin/go" <<'GO'
#!/usr/bin/env bash
set -euo pipefail
printf '%s\n' "$DATABASE_URL" > "$API_CAPTURE"
printf '%s\n' "$*" > "$API_ARGS_CAPTURE"
GO
chmod +x "$test_dir/make-case/bin/go"
cat > "$test_dir/make-case/fixture.sh" <<'FIXTURE'
set -euo pipefail
source "$1/scripts/dev-env.sh"
fixture_dir=$2
REPO_ROOT="$fixture_dir"
DIR="$REPO_ROOT"
STATE_DIR="$fixture_dir/state"
LOG_DIR="$STATE_DIR/logs"
ENV_FILE="$fixture_dir/task.env"
BACKEND_PORT=18999
WEB_MODE=development
mkdir -p "$LOG_DIR"
expected='postgres://test:p%40ss@localhost:5432/task?sslmode=disable&application_name=qa'
write_manifest_value DATABASE_URL "$expected" > "$ENV_FILE"
set -a
source "$ENV_FILE"
set +a
export PATH="$fixture_dir/bin:$PATH"
export API_CAPTURE="$fixture_dir/captured" API_ARGS_CAPTURE="$fixture_dir/args"
fixture_launched=false
health_json() { [ "$fixture_launched" = true ] && printf '{"pid":456,"commit":"testcommit","started_at":"2026-09-24T00:00:00Z"}'; }
health_belongs_to_api() { [ "$fixture_launched" = true ]; }
api_started_after() { return 0; }
port_free() { return 0; }
checkout_commit() { printf testcommit; }
checkout_source_id() { printf source-one; }
api_configuration_id() { printf config-one; }
component_pid() { [ "$fixture_launched" = true ] && printf '%s' "$$"; }
launch_detached() {
  local component=$1
  shift
  # Run the actual launcher argv, including its real bash -> go child.
  "${CLEAN_ENV[@]}" "$@"
  fixture_launched=true
  printf '%s\n' "$$" > "$(pid_file "$component")"
}
start_api
[ "$(cat "$API_CAPTURE")" = "$expected" ]
[ "$(cat "$API_ARGS_CAPTURE")" = 'run -ldflags -X main.commit=testcommit ./cmd/server' ]
FIXTURE
make --no-print-directory -f "$test_dir/make-case/Makefile" verify \
  FIXTURE="$test_dir/make-case/fixture.sh" REPO="$root_dir" CASE_DIR="$test_dir/make-case" > "$test_dir/make-output" 2>&1 \
  || { cat "$test_dir/make-output"; exit 1; }

echo 'check.test.sh: PASS'
