#!/usr/bin/env bash
# Upgrade an existing Multica Docker Compose deployment from an offline package.
set -euo pipefail

PACKAGE_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DEPLOYMENT_DIR=""
BACKUP_DIR=""
BACKEND_IMAGE=""
WEB_IMAGE=""
IMAGE_TAG=""
ASSUME_YES=0

usage() {
  cat <<'USAGE'
Usage: offline-upgrade.sh --deployment-dir DIR [options]

Required:
  --deployment-dir DIR  Existing directory containing .env

Options:
  --backup-dir DIR      Backup directory (default: DIR/backups/<timestamp>)
  --backend-image NAME  Override backend image repository (default: package image)
  --web-image NAME      Override web image repository (default: package image)
  --image-tag TAG       Override image tag (default: package image tag)
  --yes                 Skip the confirmation prompt
  -h, --help            Show this help

The script never removes Docker volumes and never changes the deployment .env.
It uses the compose file shipped in this package with the deployment's .env.
USAGE
}

die() { echo "ERROR: $*" >&2; exit 1; }

while [ $# -gt 0 ]; do
  case "$1" in
    --deployment-dir)
      [ $# -ge 2 ] || die "--deployment-dir needs a directory"
      DEPLOYMENT_DIR="$2"
      shift 2
      ;;
    --backup-dir)
      [ $# -ge 2 ] || die "--backup-dir needs a directory"
      BACKUP_DIR="$2"
      shift 2
      ;;
    --backend-image)
      [ $# -ge 2 ] || die "--backend-image needs a value"
      BACKEND_IMAGE="$2"
      shift 2
      ;;
    --web-image)
      [ $# -ge 2 ] || die "--web-image needs a value"
      WEB_IMAGE="$2"
      shift 2
      ;;
    --image-tag)
      [ $# -ge 2 ] || die "--image-tag needs a value"
      IMAGE_TAG="$2"
      shift 2
      ;;
    --yes)
      ASSUME_YES=1
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      die "unknown argument: $1"
      ;;
  esac
done

[ -n "$DEPLOYMENT_DIR" ] || { usage >&2; die "--deployment-dir is required"; }
DEPLOYMENT_DIR="$(cd "$DEPLOYMENT_DIR" 2>/dev/null && pwd)" || die "deployment directory does not exist: $DEPLOYMENT_DIR"
[ -f "$DEPLOYMENT_DIR/.env" ] || die "missing $DEPLOYMENT_DIR/.env"
[ -f "$PACKAGE_DIR/multica-images.tar.gz" ] || die "missing package image archive"
[ -f "$PACKAGE_DIR/docker-compose.selfhost.yml" ] || die "missing package compose file"
[ -f "$PACKAGE_DIR/MANIFEST.txt" ] || die "missing package manifest"
command -v docker >/dev/null 2>&1 || die "docker is required"
docker compose version >/dev/null 2>&1 || die "Docker Compose plugin is required"

manifest_platform="$(sed -n 's/^platform:[[:space:]]*//p' "$PACKAGE_DIR/MANIFEST.txt" | head -1)"
case "$(uname -m)" in
  x86_64|amd64) host_platform="linux/amd64" ;;
  arm64|aarch64) host_platform="linux/arm64" ;;
  *) host_platform="unknown" ;;
esac
[ "$manifest_platform" = "$host_platform" ] || die "package platform is $manifest_platform, server platform is $host_platform"

manifest_images="$(sed -n '/^images:/,/^$/{s/^  //p}' "$PACKAGE_DIR/MANIFEST.txt")"
default_backend_image="$(printf '%s\n' "$manifest_images" | sed -n '1p')"
default_web_image="$(printf '%s\n' "$manifest_images" | sed -n '2p')"
[ -n "$default_backend_image" ] && [ -n "$default_web_image" ] || die "invalid image list in MANIFEST.txt"

BACKEND_IMAGE="${BACKEND_IMAGE:-${default_backend_image%:*}}"
WEB_IMAGE="${WEB_IMAGE:-${default_web_image%:*}}"
IMAGE_TAG="${IMAGE_TAG:-${default_backend_image##*:}}"

if [ -z "$BACKUP_DIR" ]; then
  BACKUP_DIR="$DEPLOYMENT_DIR/backups/$(date +%Y%m%d-%H%M%S)"
fi
mkdir -p "$BACKUP_DIR"

echo "Package:   $(sed -n 's/^version:[[:space:]]*//p' "$PACKAGE_DIR/MANIFEST.txt" | head -1)"
echo "Platform:  $manifest_platform"
echo "Backend:   $BACKEND_IMAGE:$IMAGE_TAG"
echo "Web:       $WEB_IMAGE:$IMAGE_TAG"
echo "Deploy:    $DEPLOYMENT_DIR"
echo "Backup:    $BACKUP_DIR"
echo ""
echo "The script will import images, dump PostgreSQL, and recreate backend/frontend."
echo "Existing .env and Docker volumes will be kept."
if [ "$ASSUME_YES" -ne 1 ]; then
  printf 'Continue? [y/N] '
  read -r answer
  case "$answer" in
    y|Y|yes|YES) ;;
    *) echo "Cancelled."; exit 0 ;;
  esac
fi

echo "==> Checking image archive checksum"
expected_sha="$(sed -n 's/^  sha256:[[:space:]]*//p' "$PACKAGE_DIR/MANIFEST.txt" | head -1)"
if command -v sha256sum >/dev/null 2>&1; then
  actual_sha="$(sha256sum "$PACKAGE_DIR/multica-images.tar.gz" | awk '{print $1}')"
  [ "$actual_sha" = "$expected_sha" ] || die "image archive checksum mismatch"
elif command -v shasum >/dev/null 2>&1; then
  actual_sha="$(shasum -a 256 "$PACKAGE_DIR/multica-images.tar.gz" | awk '{print $1}')"
  [ "$actual_sha" = "$expected_sha" ] || die "image archive checksum mismatch"
else
  echo "WARN: neither sha256sum nor shasum is installed; skipping checksum verification" >&2
fi

echo "==> Loading images"
docker load -i "$PACKAGE_DIR/multica-images.tar.gz"

compose=(docker compose --env-file "$DEPLOYMENT_DIR/.env" -f "$PACKAGE_DIR/docker-compose.selfhost.yml")

echo "==> Backing up PostgreSQL to $BACKUP_DIR/database.sql"
"${compose[@]}" exec -T postgres \
  sh -c 'pg_dump -U "$POSTGRES_USER" -d "$POSTGRES_DB"' \
  >"$BACKUP_DIR/database.sql"

cp "$DEPLOYMENT_DIR/.env" "$BACKUP_DIR/.env"

echo "==> Starting the upgraded services"
MULTICA_BACKEND_IMAGE="$BACKEND_IMAGE" \
MULTICA_WEB_IMAGE="$WEB_IMAGE" \
MULTICA_IMAGE_TAG="$IMAGE_TAG" \
  "${compose[@]}" up -d --pull never backend frontend

backend_port="$("${compose[@]}" port backend 8080 | sed -n 's/.*:\([0-9][0-9]*\)$/\1/p' | head -1)"
[ -n "$backend_port" ] || backend_port=8080

echo "==> Waiting for backend health at http://127.0.0.1:$backend_port/healthz"
healthy=0
for _ in $(seq 1 60); do
  if curl -fsS "http://127.0.0.1:$backend_port/healthz" >/dev/null 2>&1; then
    healthy=1
    break
  fi
  sleep 2
done

if [ "$healthy" -ne 1 ]; then
  echo "Backend did not become healthy. Recent logs:" >&2
  "${compose[@]}" logs --tail=120 backend >&2 || true
  die "upgrade is incomplete; inspect logs before attempting rollback"
fi

echo ""
echo "✓ Upgrade completed"
echo "  health:  http://127.0.0.1:$backend_port/healthz"
echo "  backup:  $BACKUP_DIR"
