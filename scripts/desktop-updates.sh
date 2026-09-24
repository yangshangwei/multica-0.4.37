#!/usr/bin/env bash
# Operate the isolated desktop download service; never delete stored releases.
set -euo pipefail
CALLER_DIR="$PWD"
ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

usage() {
  cat <<'USAGE'
Usage: bash scripts/desktop-updates.sh [--env-file PATH] COMMAND [arguments]
  start | stop | status | logs   Manage Nginx; stop retains stored files
  collect SOURCE DESTINATION [--allow-prerelease]
                                Collect validated offline release files
  publish SOURCE [--allow-prerelease]
                                Publish files before channel metadata
  verify [METADATA] [--expected-version VERSION]
                                Verify HTTP downloads (default: latest.yml)
  configure [--config PATH]     Back up and merge the URL into desktop.json
  export-image OUTPUT PLATFORM  Export pinned Nginx for linux/amd64 or linux/arm64

Configuration: automatically reads updates.env beside the Compose file, or an
explicit --env-file. Docker Compose resolves dotenv without executing it;
exported environment variables take precedence. Node helpers require the
repository and its dependencies. export-image needs Docker with platform-aware
image inspect/save support (API 1.49+ and a matching CLI).

Variables: DESKTOP_UPDATES_STORAGE, DESKTOP_UPDATES_BIND, DESKTOP_UPDATES_PORT,
DESKTOP_UPDATES_IMAGE, DESKTOP_UPDATES_URL (required for configure/verify with
a custom env file). Positional paths are relative to the caller; a relative
storage path is resolved beside the Compose file.
USAGE
}

die() { printf 'Desktop updates: %s\n' "$*" >&2; exit 1; }
caller_path() {
  case "$1" in /*) printf '%s\n' "$1" ;; *) printf '%s/%s\n' "$CALLER_DIR" "$1" ;; esac
}

env_file=""
if [ "${1:-}" = "--env-file" ]; then
  [ "$#" -ge 2 ] && [ -n "$2" ] || die "--env-file requires a path"
  env_file="$(caller_path "$2")"
  [ -f "$env_file" ] || die "Configuration file does not exist: $env_file"
  shift 2
elif [ -f "$ROOT_DIR/updates.env" ]; then
  env_file="$ROOT_DIR/updates.env"
fi
command_name="${1:-help}"
if [ "$#" -gt 0 ]; then shift; fi

# Validate all arguments before resolving configuration or writing files.
case "$command_name" in
  help|--help|-h) usage; exit 0 ;;
  start|stop|status|logs) [ "$#" -eq 0 ] || die "$command_name takes no arguments" ;;
  collect)
    [ "$#" -eq 2 ] || { [ "$#" -eq 3 ] && [ "$3" = "--allow-prerelease" ]; } || die "collect requires SOURCE DESTINATION [--allow-prerelease]"
    [ -n "$1" ] && [ -n "$2" ] || die "Source and destination must not be empty"
    ;;
  publish)
    [ "$#" -eq 1 ] || { [ "$#" -eq 2 ] && [ "$2" = "--allow-prerelease" ]; } || die "publish requires SOURCE [--allow-prerelease]"
    [ -n "$1" ] || die "Source must not be empty"
    ;;
  configure)
    [ "$#" -eq 0 ] || { [ "$#" -eq 2 ] && [ "$1" = "--config" ] && [ -n "$2" ]; } || die "configure accepts only --config PATH"
    ;;
  verify)
    metadata="latest.yml"
    if [ "$#" -gt 0 ] && [[ "$1" != --* ]]; then metadata="$1"; shift; fi
    [[ "$metadata" =~ ^latest(-[A-Za-z0-9]+)*\.yml$ ]] || die "Invalid metadata filename: $metadata"
    [ "$#" -eq 0 ] || { [ "$#" -eq 2 ] && [ "$1" = "--expected-version" ] && [ -n "$2" ] && [[ "$2" != --* ]]; } || die "verify accepts METADATA [--expected-version VERSION]"
    ;;
  export-image)
    [ "$#" -eq 2 ] && [ -n "$1" ] || die "export-image requires OUTPUT PLATFORM"
    case "$2" in linux/amd64|linux/arm64) ;; *) die "Platform must be linux/amd64 or linux/arm64" ;; esac
    output="$(caller_path "$1")"
    [ ! -e "$output" ] && [ ! -L "$output" ] || die "Refusing to overwrite archive: $output"
    [ -d "$(dirname "$output")" ] || die "Archive parent directory does not exist"
    ;;
  *) usage >&2; die "Unknown command: $command_name" ;;
esac

if [ "$command_name" = "export-image" ]; then
  platform="$2"
  pinned_image="$(sed -n 's/.*DESKTOP_UPDATES_IMAGE:-\([^}]*\)}.*/\1/p' "$ROOT_DIR/docker-compose.desktop-updates.yml")"
  [[ "$pinned_image" =~ ^nginx:[A-Za-z0-9._-]+@sha256:[a-f0-9]{64}$ ]] || die "Compose must specify a pinned Nginx image"
  digest="${pinned_image##*sha256:}"
  image_tag="multica-desktop-nginx:${digest:0:12}-${platform#linux/}"
  docker pull --platform "$platform" "$pinned_image"
  actual_platform="$(docker image inspect --platform "$platform" --format '{{.Os}}/{{.Architecture}}' "$pinned_image")"
  [ "$actual_platform" = "$platform" ] || die "Image platform mismatch: expected $platform, got $actual_platform"
  docker tag "$pinned_image" "$image_tag"
  temporary="$(mktemp -d "$(dirname "$output")/.desktop-updates-image.XXXXXX")"
  trap 'rm -rf "$temporary"' EXIT
  docker save --platform "$platform" --output "$temporary/image.tar" "$image_tag"
  # Expose complete bytes without replacing a concurrent export.
  ln "$temporary/image.tar" "$output"
  printf 'Archive: %s\nDESKTOP_UPDATES_IMAGE=%s\n' "$output" "$image_tag"
  exit 0
fi

compose=(docker compose)
if [ -n "$env_file" ]; then compose+=(--env-file "$env_file"); fi
compose+=(-f "$ROOT_DIR/docker-compose.desktop-updates.yml")
cd "$ROOT_DIR"
if [ -n "$env_file" ] && [ "$command_name" != "collect" ]; then
  resolved="$("${compose[@]}" config --environment)"
  while IFS= read -r entry; do
    case "${entry%%=*}" in
      DESKTOP_UPDATES_STORAGE|DESKTOP_UPDATES_PORT|DESKTOP_UPDATES_BIND|DESKTOP_UPDATES_IMAGE|DESKTOP_UPDATES_URL)
        export "$entry"
        ;;
    esac
  done <<< "$resolved"
fi
export DESKTOP_UPDATES_STORAGE="${DESKTOP_UPDATES_STORAGE:-$ROOT_DIR/data/desktop-updates}"
case "$DESKTOP_UPDATES_STORAGE" in /*) ;; *) export DESKTOP_UPDATES_STORAGE="$ROOT_DIR/$DESKTOP_UPDATES_STORAGE" ;; esac
export DESKTOP_UPDATES_PORT="${DESKTOP_UPDATES_PORT:-18080}"
export DESKTOP_UPDATES_BIND="${DESKTOP_UPDATES_BIND:-127.0.0.1}"
if [ -n "$env_file" ] && { [ "$command_name" = "configure" ] || [ "$command_name" = "verify" ]; }; then
  [ -n "${DESKTOP_UPDATES_URL:-}" ] || die "Set DESKTOP_UPDATES_URL to the client-visible download directory in $env_file"
fi
update_url="${DESKTOP_UPDATES_URL:-http://127.0.0.1:$DESKTOP_UPDATES_PORT/desktop}"

show_endpoint() {
  local container_id binding public_storage
  container_id="$("${compose[@]}" ps -q desktop-updates)"
  if [ -n "$container_id" ]; then
    binding="$("${compose[@]}" port desktop-updates 8080)"
    public_storage="$(docker inspect --format '{{range .Mounts}}{{if eq .Destination "/srv/updates/desktop"}}{{.Source}}{{end}}{{end}}' "$container_id")"
    printf 'Published address: %s\nPublic storage: %s\n' "$binding" "$public_storage"
    printf 'Desktop update URL: %s\n' "${DESKTOP_UPDATES_URL:-http://${binding/#0.0.0.0:/127.0.0.1:}/desktop}"
  fi
}

case "$command_name" in
  start)
    mkdir -p "$DESKTOP_UPDATES_STORAGE/public"
    "${compose[@]}" up -d --pull never --wait
    show_endpoint
    ;;
  stop) "${compose[@]}" down ;;
  status)
    "${compose[@]}" ps
    show_endpoint
    ;;
  logs) "${compose[@]}" logs --tail 100 ;;
  collect)
    node "$ROOT_DIR/apps/desktop/scripts/update-artifacts.mjs" collect --source "$(caller_path "$1")" --destination "$(caller_path "$2")" "${@:3}"
    ;;
  publish)
    node "$ROOT_DIR/apps/desktop/scripts/update-artifacts.mjs" publish --source "$(caller_path "$1")" --destination "$DESKTOP_UPDATES_STORAGE" "${@:2}"
    ;;
  configure)
    config_args=()
    if [ "$#" -eq 2 ]; then config_args=(--config "$(caller_path "$2")"); fi
    node "$ROOT_DIR/apps/desktop/scripts/configure-updates.mjs" --url "$update_url" "${config_args[@]}"
    ;;
  verify)
    node "$ROOT_DIR/apps/desktop/scripts/verify-updates.mjs" --url "$update_url" --metadata "$metadata" "$@"
    ;;
esac
