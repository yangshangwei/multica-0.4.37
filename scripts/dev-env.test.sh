#!/usr/bin/env bash
# Registry-level behaviour of scripts/dev-env.sh, with no services started.
#
# Everything here runs against a throwaway MULTICA_DEV_HOME holding hand-written
# manifests, so the verbs are exercised end to end without a database, a
# backend, or a port.
set -euo pipefail

root_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
tmp_dir="$(mktemp -d)"
trap 'rm -rf "$tmp_dir"' EXIT

export MULTICA_DEV_HOME="$tmp_dir/dev"
export MULTICA_DEV_WORKSPACES_PARENT="$tmp_dir/workspaces-parent"
export MULTICA_DEV_DESKTOP_APP_DATA="$tmp_dir/app-data"
export MULTICA_DEV_PROFILES_HOME="$tmp_dir/profiles"

fake_bin="$tmp_dir/bin"
mkdir -p "$fake_bin"
cat > "$fake_bin/psql" <<'EOF'
#!/usr/bin/env bash
case " $* " in
  *" DROP DATABASE "*) [ "${FAIL_DROP:-0}" != 1 ] ;;
  *) printf '1\n' ;;
esac
EOF
chmod +x "$fake_bin/psql"
export PATH="$fake_bin:$PATH"

fail() {
  echo "FAIL: $*" >&2
  exit 1
}

require_contains() {
  local file=$1 expected=$2
  if ! grep -Fq "$expected" "$file"; then
    echo "Expected output to contain: $expected" >&2
    echo "Observed:" >&2
    sed 's/^/  /' "$file" >&2
    exit 1
  fi
}

dev_env() {
  bash "$root_dir/scripts/dev-env.sh" "$@"
}

write_manifest() {
  local name=$1 dir=$2 offset=$3
  local profile="dev-dev-env-test-$offset"
  mkdir -p "$MULTICA_DEV_HOME/envs/$name/logs"
  cat > "$MULTICA_DEV_HOME/envs/$name/manifest.env" <<EOF
NAME=$name
DIR=$(printf '%q' "$dir")
CREATED_AT=2026-01-01T00:00:00Z
OWNER=agent
TTL_HOURS=0
ENV_FILE=.env.example
OFFSET=$offset
BACKEND_PORT=$((18080 + offset))
FRONTEND_PORT=$((13000 + offset))
DB_NAME=multica_dev_env_test_$offset
DATABASE_URL=postgres://multica:multica@localhost:5432/multica_dev_env_test_$offset?sslmode=disable
PROFILE=$profile
WORKSPACES_ROOT=$(printf '%q' "$MULTICA_DEV_WORKSPACES_PARENT/multica_workspaces_$profile")
DESKTOP_RENDERER_PORT=$((5174 + offset))
DESKTOP_APP_SUFFIX=$name
EOF
}

out="$tmp_dir/out"

# ---------------------------------------------------------------------------
# An empty registry is a normal state, not an error.
# ---------------------------------------------------------------------------
dev_env list > "$out" 2>&1 || fail "list on an empty registry must succeed"
require_contains "$out" "No environments registered"

dev_env list --json > "$out" 2>&1 || fail "list --json on an empty registry must succeed"
if [ "$(cat "$out")" != "[]" ]; then
  fail "list --json on an empty registry = $(cat "$out"), want []"
fi

# ---------------------------------------------------------------------------
# Manifest serialization and user-provided names are safe. A manifest is
# sourced by Bash, so values must be shell-escaped and a name must never be
# able to walk outside envs/ before destroy eventually runs rm -rf.
# ---------------------------------------------------------------------------
quoted="$tmp_dir/quoted.env"
dangerous='a path with spaces;$(touch should-not-exist)'
bash -c 'source "$1"; write_manifest_value DIR "$2"' _ "$root_dir/scripts/dev-env.sh" "$dangerous" > "$quoted"
loaded="$(bash -c 'source "$1"; printf %s "$DIR"' _ "$quoted")"
[ "$loaded" = "$dangerous" ] || fail "manifest value did not round-trip safely"
[ ! -e "$root_dir/should-not-exist" ] || fail "loading a manifest executed its value"

status=0
dev_env up --name ../../escape > "$out" 2>&1 || status=$?
[ "$status" -ne 0 ] || fail "up accepted a path-traversing environment name"
require_contains "$out" "Invalid environment name"

status=0
dev_env up --ttl nope > "$out" 2>&1 || status=$?
[ "$status" -ne 0 ] || fail "up accepted a non-numeric TTL"
require_contains "$out" "TTL must be a positive integer"

# Rewriting an allocated database name must preserve the existing connection
# endpoint, credentials and query parameters.
rewritten="$(bash -c 'source "$1"; database_url_with_name "$2" "$3"' _ \
  "$root_dir/scripts/dev-env.sh" \
  'postgres://dev:p%40ss@127.0.0.1:55432/old_db?sslmode=require&application_name=dev' \
  'new_db')"
[ "$rewritten" = 'postgres://dev:p%40ss@127.0.0.1:55432/new_db?sslmode=require&application_name=dev' ] \
  || fail "database URL rewrite changed more than the database name: $rewritten"

# ---------------------------------------------------------------------------
# A registered environment is visible to both renderings, and the JSON one
# parses — agents read it, so a stray log line in it is a broken contract.
# ---------------------------------------------------------------------------
write_manifest "probe-901" "$tmp_dir/checkout" 901
mkdir -p "$tmp_dir/checkout"

dev_env list > "$out" 2>&1 || fail "list must succeed with one environment"
require_contains "$out" "probe-901"
require_contains "$out" "18981"

dev_env status probe-901 --json > "$out" 2>&1 || fail "status --json must succeed"
node -e '
  const fs = require("fs");
  const payload = JSON.parse(fs.readFileSync(process.argv[1], "utf8"));
  if (payload.name !== "probe-901") throw new Error("name = " + payload.name);
  if (payload.backend_port !== 18981) throw new Error("backend_port = " + payload.backend_port);
  for (const key of ["api", "web", "daemon", "desktop"]) {
    if (!payload.components[key]) throw new Error("missing component " + key);
    if (payload.components[key].state !== "stopped") {
      throw new Error(key + " state = " + payload.components[key].state);
    }
  }
' "$out" || fail "status --json is not machine-readable"

# ---------------------------------------------------------------------------
# Stopping an environment that is not running is a no-op that SUCCEEDS.
#
# This is the regression that made `make down` exit 1 after reporting success:
# on bash 3.2 a command substitution whose function ends in a failing command
# aborts the whole script under `set -e`, and "no process is listening on this
# port" is that function's normal answer.
# ---------------------------------------------------------------------------
status=0
dev_env down probe-901 --components api,web > "$out" 2>&1 || status=$?
if [ "$status" -ne 0 ]; then
  echo "Observed:" >&2
  sed 's/^/  /' "$out" >&2
  fail "down on a stopped environment exited $status, want 0"
fi
require_contains "$out" "stopped"

# Commands launched through env-exec must not inherit the daemon-task identity
# hints that make human/profile CLI commands reject --profile.
write_manifest "clean-env-903" "$root_dir" 903
MULTICA_TASK_CONFIG_ROOT=/task/config \
MULTICA_TASK_WORKSPACES_ROOT=/task/workspaces \
MULTICA_WORKSPACES_ROOT=/owner/workspaces \
  dev_env exec clean-env-903 -- sh -c '
    test -z "${MULTICA_TASK_CONFIG_ROOT:-}" &&
    test -z "${MULTICA_TASK_WORKSPACES_ROOT:-}" &&
    test "$MULTICA_WORKSPACES_ROOT" = "$1"
  ' _ "$MULTICA_DEV_WORKSPACES_PARENT/multica_workspaces_dev-dev-env-test-903" \
  > "$out" 2>&1 || fail "env-exec leaked daemon task identity or owner workspaces root"

# A health response without process identity is never proof that the process is
# this checkout's freshly launched API.
if bash -c 'source "$1"; api_started_after '\''{"status":"ok"}'\'' 1' _ "$root_dir/scripts/dev-env.sh"; then
  fail "legacy /health without started_at was accepted as current"
fi

# ---------------------------------------------------------------------------
# A listener that was moved into its own process group still belongs to the
# launcher it descends from.
#
# This is the regression that made `make up` fail on its default components:
# turbo starts each task in a fresh process group, so the Next.js dev server
# that `web-dev` had just started reported a pgid that was not the launcher's
# pid, pgid equality read that as a stranger on the port, and up killed the
# server it had launched a second earlier and exited 1.
# ---------------------------------------------------------------------------
detached_fixture="$tmp_dir/detached-child.mjs"
cat > "$detached_fixture" <<'EOF'
import { spawn } from "node:child_process";
import { writeFileSync } from "node:fs";

// detached: true calls setsid(), which is what a task runner does to the task it
// starts: the child lands outside this process's group while staying its child.
const child = spawn("sleep", ["30"], { detached: true, stdio: "ignore" });
writeFileSync(process.argv[2], String(child.pid));
setTimeout(() => {}, 30_000);
EOF

detached_pid_file="$tmp_dir/detached-child.pid"
node "$detached_fixture" "$detached_pid_file" &
fixture_launcher=$!
for _ in $(seq 1 50); do
  if [ -s "$detached_pid_file" ]; then break; fi
  sleep 0.1
done
[ -s "$detached_pid_file" ] || fail "process-group fixture never reported its child"
detached_child="$(cat "$detached_pid_file")"

fixture_pgid="$(bash -c 'source "$1"; process_group_id "$2"' _ \
  "$root_dir/scripts/dev-env.sh" "$detached_child")"
[ "$fixture_pgid" != "$fixture_launcher" ] \
  || fail "fixture did not reproduce a child outside its launcher's process group"

bash -c 'source "$1"; process_ancestry_includes "$2" "$3"' _ \
  "$root_dir/scripts/dev-env.sh" "$detached_child" "$fixture_launcher" \
  || fail "a process whose parent chain reaches the launcher was not recognised as ours"

if bash -c 'source "$1"; process_ancestry_includes "$2" "$3"' _ \
  "$root_dir/scripts/dev-env.sh" "$$" "$detached_child"; then
  fail "an unrelated process was claimed as a descendant"
fi

kill "$detached_child" "$fixture_launcher" 2>/dev/null || true
wait "$fixture_launcher" 2>/dev/null || true

# ---------------------------------------------------------------------------
# Unknown names and components fail loudly instead of doing something else.
# ---------------------------------------------------------------------------
status=0
dev_env status no-such-env > "$out" 2>&1 || status=$?
[ "$status" -ne 0 ] || fail "status on an unknown environment must fail"
require_contains "$out" "Unknown environment"

status=0
dev_env up --components nope > "$out" 2>&1 || status=$?
[ "$status" -ne 0 ] || fail "up with an unknown component must fail"
require_contains "$out" "Unknown component"

# ---------------------------------------------------------------------------
# gc reports what it would collect and touches nothing in --dry-run. An
# environment whose checkout is gone has no owner left to stop it, which is how
# 152 databases accumulated with nothing on the machine able to list them.
# ---------------------------------------------------------------------------
write_manifest "orphan-902" "$tmp_dir/deleted-checkout" 902

dev_env gc --dry-run > "$out" 2>&1 || fail "gc --dry-run must succeed"
require_contains "$out" "orphan-902 would be collected"
if grep -Fq "probe-901 would be collected" "$out"; then
  fail "gc must not collect an environment whose directory still exists"
fi
[ -f "$MULTICA_DEV_HOME/envs/orphan-902/manifest.env" ] || fail "gc --dry-run deleted a manifest"

# A failed database drop keeps the manifest and slot so cleanup can be retried;
# destroy must never print success and forget the only deletion recipe.
write_manifest "drop-fails-904" "$root_dir" 904
status=0
FAIL_DROP=1 dev_env destroy drop-fails-904 --yes > "$out" 2>&1 || status=$?
[ "$status" -ne 0 ] || fail "destroy succeeded after DROP DATABASE failed"
[ -f "$MULTICA_DEV_HOME/envs/drop-fails-904/manifest.env" ] \
  || fail "destroy discarded the manifest after DROP DATABASE failed"
require_contains "$out" "manifest and slot were kept"
dev_env destroy drop-fails-904 --yes > "$out" 2>&1 || fail "retrying destroy after database recovery failed"

# ---------------------------------------------------------------------------
# destroy consumes the manifest: the slot is free afterwards, which is what
# makes the registry an allocator rather than a second place to leak.
# ---------------------------------------------------------------------------
dev_env destroy probe-901 --yes > "$out" 2>&1 || fail "destroy must succeed"
[ ! -d "$MULTICA_DEV_HOME/envs/probe-901" ] || fail "destroy left the environment directory behind"

dev_env list > "$out" 2>&1 || fail "list must succeed after destroy"
if grep -Fq "probe-901" "$out"; then
  fail "destroyed environment is still listed"
fi

# Declining the confirmation is a successful no-op, not a failure.
printf 'n\n' | dev_env destroy orphan-902 > "$out" 2>&1 || fail "declining destroy must exit 0"
require_contains "$out" "Cancelled."
[ -d "$MULTICA_DEV_HOME/envs/orphan-902" ] || fail "declined destroy removed the environment anyway"


# Production Web is built before launch; reuse requires mode, source, build and
# configuration identity in addition to listener ownership. Unknown listeners
# are never stopped, including when a previous build has a different mode.
web_fixture="$tmp_dir/web-fixture.sh"
cat > "$web_fixture" <<'FIXTURE'
set -euo pipefail
source "$1/scripts/dev-env.sh"
fixture_dir=$2
scenario=$3
REPO_ROOT="$fixture_dir/checkout"
DIR="$REPO_ROOT"
STATE_DIR="$fixture_dir/state"
LOG_DIR="$fixture_dir/logs"
FRONTEND_PORT=13999
BACKEND_PORT=18999
WEB_MODE=production
ENV_FILE=.env
ENVS_DIR="$fixture_dir/registry/envs"
mkdir -p "$REPO_ROOT/apps/web/.next" "$STATE_DIR" "$LOG_DIR"
launched=false
fixture_source_id=source-one
eval "$(declare -f checkout_source_id | sed '1s/checkout_source_id/actual_checkout_source_id/')"
if [ "$scenario" = next-mode ] || [ "$scenario" = real-source-change ]; then
  git -C "$REPO_ROOT" init -q
  printf '.next/\n.multica/\n' > "$REPO_ROOT/.gitignore"
  printf 'import "./.next/dev/types/routes.d.ts";\n' > "$REPO_ROOT/apps/web/next-env.d.ts"
  printf 'export const value = 1;\n' > "$REPO_ROOT/apps/web/source.ts"
  git -C "$REPO_ROOT" add .gitignore apps/web/next-env.d.ts apps/web/source.ts
  git -C "$REPO_ROOT" -c user.name=Fixture -c user.email=fixture@example.invalid commit -qm 'Record fixture input'
fi
curl() { [ -f "$fixture_dir/launched" ]; }
port_free() { [ "$scenario" != stranger ]; }
port_listener_pid() { printf 456; }
listener_belongs_to_component() { [ -f "$fixture_dir/launched" ]; }
checkout_commit() { printf testcommit; }
checkout_source_id() {
  if [ "$scenario" = next-mode ] || [ "$scenario" = real-source-change ]; then
    actual_checkout_source_id
  else
    printf '%s' "$fixture_source_id"
  fi
}
component_pid() { printf '%s' "$$"; }
stop_component() { echo stop >> "$fixture_dir/calls"; }
describe_port_owner() { printf stranger; }
pnpm() {
  echo "pnpm $*" >> "$fixture_dir/calls"
  [ "$scenario" != build-failure ] || return 17
  printf 'test-build\n' > "$REPO_ROOT/apps/web/.next/BUILD_ID"
  if [ "$scenario" = next-mode ] || [ "$scenario" = real-source-change ]; then
    printf 'import "./.next/types/routes.d.ts";\n' > "$REPO_ROOT/apps/web/next-env.d.ts"
  fi
  if [ "$scenario" = real-source-change ]; then printf 'export const value = 2;\n' > "$REPO_ROOT/apps/web/source.ts"; fi
  if [ "$scenario" = config-change ]; then printf 'DOCS_URL=http://changed\n' > "$REPO_ROOT/.env"; fi
}
launch_detached() {
  echo "launch $*" >> "$fixture_dir/calls"
  [ -s "$REPO_ROOT/apps/web/.next/BUILD_ID" ]
  printf '%s\n' "$$" > "$(pid_file web)"
  launched=true
  touch "$fixture_dir/launched"
}
if [ "$scenario" = active-other ]; then
  mkdir -p "$ENVS_DIR/other"
  {
    write_manifest_value NAME other
    write_manifest_value DIR "$REPO_ROOT"
    write_manifest_value OFFSET 999
    write_manifest_value PROFILE other
    write_manifest_value FRONTEND_PORT 13998
  } > "$ENVS_DIR/other/manifest.env"
fi
start_web
if [ "$scenario" = next-mode ]; then
  web_identity_matches
  exit 0
fi
[ "$scenario" = success ] || exit 1
[ "$(json_field "$(cat "$STATE_DIR/web.running.json")" mode)" = production ]
[ "$(json_field "$(cat "$STATE_DIR/web.running.json")" build_id)" = test-build ]
start_web
[ "$(grep -c '^launch ' "$fixture_dir/calls")" -eq 1 ]
fixture_source_id=source-two
! web_identity_matches || { echo "Unexpected match in rejection assertion" >&2; exit 1; }
fixture_source_id=source-one
REMOTE_API_URL=http://localhost:18998
! web_identity_matches || { echo "Unexpected match in rejection assertion" >&2; exit 1; }
REMOTE_API_URL=http://localhost:18999
WEB_MODE=development
! web_identity_matches || { echo "Unexpected match in rejection assertion" >&2; exit 1; }
WEB_MODE=production
printf 'changed-build\n' > "$REPO_ROOT/apps/web/.next/BUILD_ID"
! web_identity_matches || { echo "Unexpected match in rejection assertion" >&2; exit 1; }
FIXTURE
for scenario in success next-mode real-source-change stranger build-failure config-change active-other; do
  mkdir -p "$tmp_dir/web-$scenario"
  status=0
  bash "$web_fixture" "$root_dir" "$tmp_dir/web-$scenario" "$scenario" > "$out" 2>&1 || status=$?
  if [ "$scenario" = success ] || [ "$scenario" = next-mode ]; then
    [ "$status" = 0 ] || { cat "$out"; fail "production Web fixture failed"; }
    require_contains "$tmp_dir/web-$scenario/calls" 'launch web pnpm --dir'
    require_contains "$tmp_dir/web-$scenario/calls" 'exec next start --port 13999'
  else
    [ "$status" -ne 0 ] || fail "$scenario unexpectedly succeeded"
    if [ -f "$tmp_dir/web-$scenario/calls" ]; then
      ! grep -Eq '^launch |^stop' "$tmp_dir/web-$scenario/calls" || fail "$scenario launched/stopped a service"
    fi
  fi
done


# Focused production runs must rebuild an owned API after dirty source/config
# changes even if HEAD is unchanged. Development keeps its existing reuse rule.
api_fixture="$tmp_dir/api-fixture.sh"
cat > "$api_fixture" <<'FIXTURE'
set -euo pipefail
source "$1/scripts/dev-env.sh"
fixture_dir=$2
scenario=$3
STATE_DIR="$fixture_dir/state"
LOG_DIR="$fixture_dir/logs"
WEB_MODE=production
ENV_FILE=.env
BACKEND_PORT=18999
mkdir -p "$STATE_DIR" "$LOG_DIR"
launched=false
fixture_source_id=source-one
fixture_config_id=config-one
fixture_health='{"pid":456,"commit":"testcommit","started_at":"2026-09-24T00:00:00Z"}'
health_json() { [ "$launched" = true ] && printf '%s' "$fixture_health"; }
health_belongs_to_api() { [ "$launched" = true ]; }
api_started_after() { return 0; }
sleep() { echo "Unexpected API readiness wait in fixture" >&2; return 1; }
port_free() { [ "$launched" = false ]; }
checkout_commit() { printf testcommit; }
checkout_source_id() { printf '%s' "$fixture_source_id"; }
api_configuration_id() { printf '%s' "$fixture_config_id"; }
component_pid() { [ "$launched" = true ] && printf '%s' "$$"; }
stop_component() { echo stop >> "$fixture_dir/calls"; launched=false; }
launch_detached() {
  echo launch >> "$fixture_dir/calls"
  launched=true
  printf '%s\n' "$$" > "$(pid_file api)"
  if [ "$scenario" = changing-source ]; then fixture_source_id=source-two; fi
  if [ "$scenario" = changing-config ]; then fixture_config_id=config-two; fi
}
start_api
[ "$scenario" = success ] || exit 1
[ "$(json_field "$(cat "$STATE_DIR/api.running.json")" listener_pid)" = 456 ]
start_api
[ "$(grep -c '^launch$' "$fixture_dir/calls")" -eq 1 ]
fixture_source_id=source-two
! api_identity_matches "$fixture_health" testcommit || { echo "Unexpected match in rejection assertion" >&2; exit 1; }
WEB_MODE=development
api_identity_matches "$fixture_health" testcommit
WEB_MODE=production
start_api
[ "$(grep -c '^launch$' "$fixture_dir/calls")" -eq 2 ]
fixture_config_id=config-two
start_api
[ "$(grep -c '^launch$' "$fixture_dir/calls")" -eq 3 ]
FIXTURE
for scenario in success changing-source changing-config; do
  mkdir -p "$tmp_dir/api-$scenario"
  status=0
  bash "$api_fixture" "$root_dir" "$tmp_dir/api-$scenario" "$scenario" > "$out" 2>&1 || status=$?
  if [ "$scenario" = success ]; then
    [ "$status" = 0 ] || { cat "$out"; fail "production API fixture failed"; }
  else
    [ "$status" -ne 0 ] || fail "$scenario unexpectedly reused an API"
    require_contains "$tmp_dir/api-$scenario/calls" stop
  fi
done


# Two check runs can allocate distinct ports but must serialize the shared
# checkout build output. Hold barriers before build completion and PID capture.
lock_fixture="$tmp_dir/lock-fixture.sh"
cat > "$lock_fixture" <<'FIXTURE'
set -euo pipefail
source "$1/scripts/dev-env.sh"
REPO_ROOT=$2
WEB_MODE=production
scenario=$3
start_web_locked() {
  printf '%s\n' "$scenario" >> "$REPO_ROOT/builders"
  [ "$scenario" != failure ] || return 41
  touch "$REPO_ROOT/building"
  for _ in $(seq 1 100); do
    [ -f "$REPO_ROOT/release-build" ] && break
    sleep 0.05
  done
  [ -f "$REPO_ROOT/release-build" ]
  touch "$REPO_ROOT/launching"
  for _ in $(seq 1 100); do
    [ -f "$REPO_ROOT/release-launch" ] && break
    sleep 0.05
  done
  [ -f "$REPO_ROOT/release-launch" ]
  touch "$REPO_ROOT/listener-registered"
}
start_web
FIXTURE
lock_checkout="$tmp_dir/lock-checkout"
mkdir -p "$lock_checkout"
bash "$lock_fixture" "$root_dir" "$lock_checkout" first > "$tmp_dir/first-builder.log" 2>&1 &
first_builder=$!
for _ in $(seq 1 100); do
  [ -f "$lock_checkout/building" ] && break
  sleep 0.05
done
[ -f "$lock_checkout/building" ] || fail "first builder missed barrier"
if bash "$lock_fixture" "$root_dir" "$lock_checkout" second > "$out" 2>&1; then
  fail "second builder entered during build"
fi
require_contains "$out" 'Refusing to modify the shared build output'
touch "$lock_checkout/release-build"
for _ in $(seq 1 100); do
  [ -f "$lock_checkout/launching" ] && break
  sleep 0.05
done
[ -f "$lock_checkout/launching" ] || fail "first builder missed launch barrier"
if bash "$lock_fixture" "$root_dir" "$lock_checkout" second > "$out" 2>&1; then
  fail "second builder entered between build and PID registration"
fi
[ "$(cat "$lock_checkout/builders")" = first ] || fail "concurrent builder touched output"
touch "$lock_checkout/release-launch"
wait "$first_builder" || { cat "$tmp_dir/first-builder.log"; fail "first builder failed"; }
[ -f "$lock_checkout/listener-registered" ] || fail "lock released before PID registration"
[ ! -d "$lock_checkout/.multica/web-build.lock.d" ] || fail "successful build leaked lock"
status=0
bash "$lock_fixture" "$root_dir" "$lock_checkout" failure > "$out" 2>&1 || status=$?
[ "$status" = 41 ] || fail "build failure lost its exit status"
[ ! -d "$lock_checkout/.multica/web-build.lock.d" ] || fail "failed build leaked lock"


# Only the generated Next dev/prod route-types import is normalized. Other
# changes in that same declaration, file identity, and app source still matter.
source_checkout="$tmp_dir/source-checkout"
mkdir -p "$source_checkout/apps/web" "$source_checkout/packages/core"
git -C "$source_checkout" init -q
printf 'import "./.next/dev/types/routes.d.ts";\n' > "$source_checkout/apps/web/next-env.d.ts"
printf 'export const original = true;\n' > "$source_checkout/packages/core/example.ts"
git -C "$source_checkout" add .
git -C "$source_checkout" -c user.name=Fixture -c user.email=fixture@example.invalid commit -qm 'Record fingerprint fixture'
source_fingerprint() {
  bash -c 'source "$1"; DIR=$2; checkout_source_id' _ "$root_dir/scripts/dev-env.sh" "$source_checkout"
}
original_fingerprint="$(source_fingerprint)"
printf 'import "./.next/types/routes.d.ts";\n' > "$source_checkout/apps/web/next-env.d.ts"
[ "$(source_fingerprint)" = "$original_fingerprint" ] || fail "generated Next mode switch changed identity"
printf 'declare const modified: string;\n' >> "$source_checkout/apps/web/next-env.d.ts"
[ "$(source_fingerprint)" != "$original_fingerprint" ] || fail "other next-env declaration edit was ignored"
printf 'import "./.next/types/routes.d.ts";\n' > "$source_checkout/apps/web/next-env.d.ts"
chmod +x "$source_checkout/apps/web/next-env.d.ts"
[ "$(source_fingerprint)" != "$original_fingerprint" ] || fail "next-env file-mode edit was ignored"
chmod -x "$source_checkout/apps/web/next-env.d.ts"
rm "$source_checkout/apps/web/next-env.d.ts"
[ "$(source_fingerprint)" != "$original_fingerprint" ] || fail "next-env deletion was ignored"
printf 'import "./.next/types/routes.d.ts";\n' > "$source_checkout/apps/web/next-env.d.ts"
printf 'export const original = false;\n' > "$source_checkout/packages/core/example.ts"
[ "$(source_fingerprint)" != "$original_fingerprint" ] || fail "tracked source edit was ignored"
printf 'export const original = true;\n' > "$source_checkout/packages/core/example.ts"
printf 'export const added = true;\n' > "$source_checkout/packages/core/new.ts"
[ "$(source_fingerprint)" != "$original_fingerprint" ] || fail "untracked source edit was ignored"

echo "✓ dev-env.sh registry behaviour verified"
