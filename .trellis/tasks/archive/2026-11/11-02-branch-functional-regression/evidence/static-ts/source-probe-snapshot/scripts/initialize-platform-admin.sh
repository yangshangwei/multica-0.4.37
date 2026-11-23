#!/usr/bin/env bash
# Initialize through deployment authority using the installed Compose settings.
set -euo pipefail

PACKAGE_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DEPLOYMENT_DIR=""
WEB_IMAGE=""
BACKEND_IMAGE=""
COMPOSE_FILES=()
die() { echo "ERROR: $*" >&2; exit 1; }
single_line() { case "$1" in *$'\n'*|*$'\r'*) die "paths must be single-line" ;; esac; }

while [ $# -gt 0 ]; do
  case "$1" in
    --deployment-dir) [ $# -ge 2 ] || die "--deployment-dir needs a directory"; DEPLOYMENT_DIR="$2"; shift 2 ;;
    --compose-file) [ $# -ge 2 ] || die "--compose-file needs a path"; COMPOSE_FILES+=("$2"); shift 2 ;;
    --web-image) [ $# -ge 2 ] || die "--web-image needs a loaded image"; WEB_IMAGE="$2"; shift 2 ;;
    --backend-image) [ $# -ge 2 ] || die "--backend-image needs an expected image"; BACKEND_IMAGE="$2"; shift 2 ;;
    -h|--help)
      echo "Usage: initialize-platform-admin.sh --deployment-dir DIR [--compose-file OVERLAY ...]"
      echo "Use every deployment overlay in its original order. Requires Docker/Compose and loaded images, not host Node or Go."
      echo "Optional: --web-image loaded:tag selects the Node runner (default: package manifest); --backend-image expected:tag verifies the backend selection."
      exit 0 ;;
    *) die "unknown argument: $1" ;;
  esac
done

[ -n "$DEPLOYMENT_DIR" ] || die "--deployment-dir is required"
single_line "$DEPLOYMENT_DIR"
DEPLOYMENT_DIR="$(cd "$DEPLOYMENT_DIR" && pwd)" || die "deployment directory does not exist"
single_line "$DEPLOYMENT_DIR"
[ -f "$DEPLOYMENT_DIR/.env" ] && [ -r "$DEPLOYMENT_DIR/.env" ] || die "deployment .env is missing or unreadable"
MAIN_COMPOSE="$DEPLOYMENT_DIR/docker-compose.selfhost.yml"
[ -f "$MAIN_COMPOSE" ] && [ -r "$MAIN_COMPOSE" ] || die "installed deployment Compose file is missing or unreadable"
TOOL="$PACKAGE_DIR/scripts/admin-init-compose.mjs"
[ -f "$TOOL" ] || TOOL="$PACKAGE_DIR/admin-init-compose.mjs"
[ -f "$TOOL" ] || die "package is missing scripts/admin-init-compose.mjs"

compose=(docker compose --project-directory "$DEPLOYMENT_DIR" --env-file "$DEPLOYMENT_DIR/.env" -f "$MAIN_COMPOSE")
for file in "${COMPOSE_FILES[@]+"${COMPOSE_FILES[@]}"}"; do
  single_line "$file"
  case "$file" in /*) ;; *) file="$DEPLOYMENT_DIR/$file" ;; esac
  [ -f "$file" ] && [ -r "$file" ] || die "compose overlay is missing or unreadable: $file"
  file="$(cd "$(dirname "$file")" && pwd)/$(basename "$file")"
  [ "$file" != "$MAIN_COMPOSE" ] || die "--compose-file accepts overlays, not the main compose file"
  compose+=(-f "$file")
done
command -v docker >/dev/null 2>&1 || die "Docker is required"
docker compose version >/dev/null 2>&1 || die "Docker Compose is required"
if [ -z "$WEB_IMAGE" ]; then
  [ -f "$PACKAGE_DIR/MANIFEST.txt" ] || die "pass --web-image when no package manifest is present"
  WEB_IMAGE="$(sed -n '/^images:/,/^$/{s/^  //p;}' "$PACKAGE_DIR/MANIFEST.txt" | sed -n '2p')"
fi
[ -n "$WEB_IMAGE" ] || die "could not resolve the loaded frontend image"

operator="$(id -u):$(id -g)"
tool_source="$(cat "$TOOL")"
node_tool() {
  docker run --rm -i --pull never --network none --entrypoint node --user "$operator" \
    "$WEB_IMAGE" --input-type=module -e "$tool_source" admin-init-compose "$@"
}

# Full resolved configuration may contain secrets. Pipe it directly to the
# parser; only the nonsecret status and small override ever enter shell state.
plan="$("${compose[@]}" config --format json | node_tool plan "$DEPLOYMENT_DIR" "$operator" "$BACKEND_IMAGE")"
status="${plan%%$'\n'*}"
case "$status" in
  skipped_mode_disabled|skipped_admin_disabled)
    printf '{"schema_version":1,"status":"%s"}\n' "$status"
    exit 0 ;;
  initialize) ;;
  *) die "invalid administrator initialization plan" ;;
esac

umask 077
overlay="$(mktemp "$DEPLOYMENT_DIR/.admin-initialization-compose.XXXXXX")"
trap 'rm -f "$overlay"' EXIT
trap 'exit 130' INT
trap 'exit 143' TERM
printf '%s\n' "${plan#*$'\n'}" >"$overlay"

# The API service never retains this credential mount. Its database, network,
# and operator overlays are inherited only for this one-off maintenance run.
"${compose[@]}" -f "$overlay" run --no-deps --pull never --rm -T \
  --entrypoint /app/server backend platform-admin initialize \
  --credentials-file /deployment-admin-init/admin-initial-password.txt \
  --reason "Offline deployment administrator initialization" \
  | node_tool receipt "$DEPLOYMENT_DIR"
