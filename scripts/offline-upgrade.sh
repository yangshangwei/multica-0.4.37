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
COMPOSE_FILES=()

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
  --compose-file FILE   Additional overlay, relative to DIR or absolute; repeat in order
  --yes                 Skip the confirmation prompt
  -h, --help            Show this help

The script never removes Docker volumes. It saves MULTICA_BACKEND_IMAGE,
MULTICA_WEB_IMAGE, MULTICA_IMAGE_TAG, CHANGELOG_FILE and CHANGELOG_DIRECTORY
in deployment .env while preserving other settings.
It backs up and replaces the deployment's main compose file with this package's.
Pass every existing overlay explicitly on every upgrade. The complete command
is recorded in DIR/compose-command.txt for later maintenance.
USAGE
}

die() { echo "ERROR: $*" >&2; exit 1; }
fail_upgrade() {
  die "upgrade is incomplete; selected image settings remain saved. Containers and database were not rolled back. Inspect logs and the backup at $BACKUP_DIR before recovery."
}

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
    --compose-file)
      [ $# -ge 2 ] || die "--compose-file needs a path"
      COMPOSE_FILES+=("$2")
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
[ -f "$PACKAGE_DIR/install-changelog.sh" ] || die "missing package changelog installer"
# Resolve overlays before Docker calls or backup creation. Never evaluate paths
# or source .env; spaces and shell metacharacters must remain literal argv.
overlay_args=()
publisher_args=()
for file in "${COMPOSE_FILES[@]+"${COMPOSE_FILES[@]}"}"; do
  case "$file" in *$'\n'*|*$'\r'*) die "compose overlay path must be single-line" ;; esac
  case "$file" in /*) ;; *) file="$DEPLOYMENT_DIR/$file" ;; esac
  [ -f "$file" ] && [ -r "$file" ] || die "compose overlay is missing or unreadable: $file"
  file="$(cd "$(dirname "$file")" && pwd)/$(basename "$file")"
  [ "$file" != "$DEPLOYMENT_DIR/docker-compose.selfhost.yml" ] && [ "$file" != "$PACKAGE_DIR/docker-compose.selfhost.yml" ] || die "--compose-file accepts overlays, not the main compose file"
  overlay_args+=(-f "$file")
  publisher_args+=(--compose-file "$file")
done
command -v docker >/dev/null 2>&1 || die "docker is required"
docker compose version >/dev/null 2>&1 || die "Docker Compose plugin is required"

manifest_platform="$(sed -n 's/^platform:[[:space:]]*//p' "$PACKAGE_DIR/MANIFEST.txt" | head -1)"
case "$(uname -m)" in
  x86_64|amd64) host_platform="linux/amd64" ;;
  arm64|aarch64) host_platform="linux/arm64" ;;
  *) host_platform="unknown" ;;
esac
[ "$manifest_platform" = "$host_platform" ] || die "package platform is $manifest_platform, server platform is $host_platform"

manifest_images="$(sed -n '/^images:/,/^$/{s/^  //p;}' "$PACKAGE_DIR/MANIFEST.txt")"
default_backend_image="$(printf '%s\n' "$manifest_images" | sed -n '1p')"
default_web_image="$(printf '%s\n' "$manifest_images" | sed -n '2p')"
[ -n "$default_backend_image" ] && [ -n "$default_web_image" ] || die "invalid image list in MANIFEST.txt"

BACKEND_IMAGE="${BACKEND_IMAGE:-${default_backend_image%:*}}"
WEB_IMAGE="${WEB_IMAGE:-${default_web_image%:*}}"
IMAGE_TAG="${IMAGE_TAG:-${default_backend_image##*:}}"

if [ -z "$BACKUP_DIR" ]; then
  BACKUP_DIR="$DEPLOYMENT_DIR/backups/$(date +%Y%m%d-%H%M%S)"
fi
[ ! -e "$BACKUP_DIR/database.sql" ] || die "backup already exists; choose a new --backup-dir"

echo "Package:   $(sed -n 's/^version:[[:space:]]*//p' "$PACKAGE_DIR/MANIFEST.txt" | head -1)"
echo "Platform:  $manifest_platform"
echo "Backend:   $BACKEND_IMAGE:$IMAGE_TAG"
echo "Web:       $WEB_IMAGE:$IMAGE_TAG"
echo "Deploy:    $DEPLOYMENT_DIR"
echo "Backup:    $BACKUP_DIR"
echo ""
echo "The script will import images, dump PostgreSQL, and recreate backend/frontend."
echo "Selected images and changelog paths will be saved; other settings and Docker volumes will be kept."
if [ "$ASSUME_YES" -ne 1 ]; then
  printf 'Continue? [y/N] '
  read -r answer
  case "$answer" in
    y|Y|yes|YES) ;;
    *) echo "Cancelled."; exit 0 ;;
  esac
fi

compose=(docker compose --project-directory "$DEPLOYMENT_DIR" --env-file "$DEPLOYMENT_DIR/.env" -f "$PACKAGE_DIR/docker-compose.selfhost.yml" "${overlay_args[@]+"${overlay_args[@]}"}")
target_compose_json="$(MULTICA_BACKEND_IMAGE="$BACKEND_IMAGE" MULTICA_WEB_IMAGE="$WEB_IMAGE" MULTICA_IMAGE_TAG="$IMAGE_TAG" "${compose[@]}" config --format json)"

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

# A literal overlay image defeats environment interpolation. Validate the exact
# service/image mapping before any backup or deployment write. Service-filtered
# `config --images` includes dependencies and cannot prove this mapping.
printf '%s' "$target_compose_json" | docker run --rm -i --pull never --network none \
  --entrypoint node --user "$(id -u):$(id -g)" "$WEB_IMAGE:$IMAGE_TAG" -e '
let input = "";
process.stdin.setEncoding("utf8");
process.stdin.on("data", (chunk) => {
  input += chunk;
  if (input.length > 2 * 1024 * 1024) process.exit(1);
});
process.stdin.on("end", () => {
  try {
    const services = JSON.parse(input).services;
    for (const [name, expected] of [["backend", process.argv[1]], ["frontend", process.argv[2]]]) {
      if (services?.[name]?.image !== expected) throw new Error(name + " image does not match selected upgrade; remove or adjust the overlay image setting");
    }
  } catch (error) {
    console.error("ERROR: " + (error instanceof SyntaxError ? "invalid Compose JSON" : error.message));
    process.exitCode = 1;
  }
});
' "$BACKEND_IMAGE:$IMAGE_TAG" "$WEB_IMAGE:$IMAGE_TAG"

mkdir -p "$BACKUP_DIR"
echo "==> Backing up PostgreSQL to $BACKUP_DIR/database.sql"
"${compose[@]}" exec -T postgres \
  sh -c 'pg_dump -U "$POSTGRES_USER" -d "$POSTGRES_DB"' \
  >"$BACKUP_DIR/database.sql"

cp -p "$DEPLOYMENT_DIR/.env" "$BACKUP_DIR/.env"
for file in docker-compose.selfhost.yml compose-command.txt; do
  if [ -f "$DEPLOYMENT_DIR/$file" ]; then
    cp -p "$DEPLOYMENT_DIR/$file" "$BACKUP_DIR/$file"
  fi
done
for ((i = 0; i < ${#overlay_args[@]}; i += 2)); do
  cp -p "${overlay_args[i + 1]}" "$BACKUP_DIR/compose-overlay-$((i / 2 + 1)).yml"
done

# Stage deployment files before publication; rename only after the feed/env
# writer succeeds. These files and the database are not one transaction.
staged_compose="$(mktemp "$DEPLOYMENT_DIR/.compose-upgrade.XXXXXX")"
staged_command=""
trap 'rm -f "$staged_compose" "$staged_command"' EXIT
cp "$PACKAGE_DIR/docker-compose.selfhost.yml" "$staged_compose"
chmod 0644 "$staged_compose"
runtime_compose=(docker compose --project-directory "$DEPLOYMENT_DIR" --env-file "$DEPLOYMENT_DIR/.env" -f "$DEPLOYMENT_DIR/docker-compose.selfhost.yml" "${overlay_args[@]+"${overlay_args[@]}"}")
staged_command="$(mktemp "$DEPLOYMENT_DIR/.compose-command.XXXXXX")"
printf '%q ' "${runtime_compose[@]}" >"$staged_command"
printf '\n' >>"$staged_command"

echo "==> Installing the cumulative changelog and saving selected runtime images"
bash "$PACKAGE_DIR/install-changelog.sh" --deployment-dir "$DEPLOYMENT_DIR" \
  --web-image "$WEB_IMAGE:$IMAGE_TAG" --backend-image "$BACKEND_IMAGE" --image-tag "$IMAGE_TAG" "${publisher_args[@]+"${publisher_args[@]}"}"

mv -f "$staged_compose" "$DEPLOYMENT_DIR/docker-compose.selfhost.yml"
mv -f "$staged_command" "$DEPLOYMENT_DIR/compose-command.txt"
compose=("${runtime_compose[@]}")

echo "==> Starting the upgraded services"
if ! MULTICA_BACKEND_IMAGE="$BACKEND_IMAGE" \
MULTICA_WEB_IMAGE="$WEB_IMAGE" \
MULTICA_IMAGE_TAG="$IMAGE_TAG" \
  "${compose[@]}" up -d --pull never backend frontend; then
  fail_upgrade
fi

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
  fail_upgrade
fi

echo ""
echo "✓ Upgrade completed"
echo "  health:  http://127.0.0.1:$backend_port/healthz"
echo "  backup:  $BACKUP_DIR"
echo "  compose: $DEPLOYMENT_DIR/compose-command.txt (append a Compose command; preserve every -f overlay)"
