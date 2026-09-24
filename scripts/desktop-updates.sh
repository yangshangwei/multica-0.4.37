#!/usr/bin/env bash
# Manage the isolated local desktop download service; never delete stored releases.
set -euo pipefail
ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

usage() {
  cat <<'USAGE'
Usage: bash scripts/desktop-updates.sh COMMAND [arguments]
  start                         Start Nginx (image must already be loaded)
  stop                          Stop Nginx and retain all stored files
  status                        Show container and download endpoint
  logs                          Show recent Nginx access/error logs
  publish SOURCE [--allow-prerelease]
                                Validate and publish an existing Desktop build
  configure [--config PATH]     Merge the update URL into existing desktop.json

Environment: DESKTOP_UPDATES_STORAGE (default: data/desktop-updates),
DESKTOP_UPDATES_BIND (127.0.0.1), DESKTOP_UPDATES_PORT (18080),
DESKTOP_UPDATES_URL (client-visible URL for configure), DESKTOP_UPDATES_IMAGE.
USAGE
}

command_name="${1:-help}"
if [ "$#" -gt 0 ]; then shift; fi
export DESKTOP_UPDATES_STORAGE="${DESKTOP_UPDATES_STORAGE:-$ROOT_DIR/data/desktop-updates}"
export DESKTOP_UPDATES_PORT="${DESKTOP_UPDATES_PORT:-18080}"
export DESKTOP_UPDATES_BIND="${DESKTOP_UPDATES_BIND:-127.0.0.1}"
compose=(docker compose -f "$ROOT_DIR/docker-compose.desktop-updates.yml")
update_url="${DESKTOP_UPDATES_URL:-http://127.0.0.1:$DESKTOP_UPDATES_PORT/desktop}"

show_endpoint() {
  local container_id binding public_storage
  container_id="$("${compose[@]}" ps -q desktop-updates)"
  if [ -n "$container_id" ]; then
    binding="$("${compose[@]}" port desktop-updates 8080)"
    public_storage="$(docker inspect --format '{{range .Mounts}}{{if eq .Destination "/srv/updates/desktop"}}{{.Source}}{{end}}{{end}}' "$container_id")"
    printf 'Published address: %s\nPublic storage: %s\n' "$binding" "$public_storage"
    printf 'Desktop update URL: http://%s/desktop\n' "${binding/#0.0.0.0:/127.0.0.1:}"
  fi
}

case "$command_name" in
  start)
    [ "$#" -eq 0 ] || { usage >&2; exit 1; }
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
  publish)
    [ "$#" -ge 1 ] || { usage >&2; exit 1; }
    node apps/desktop/scripts/update-artifacts.mjs publish --destination "$DESKTOP_UPDATES_STORAGE" --source "$@"
    ;;
  configure)
    node apps/desktop/scripts/configure-updates.mjs --url "$update_url" "$@"
    ;;
  help|--help|-h) usage ;;
  *) usage >&2; exit 1 ;;
esac
