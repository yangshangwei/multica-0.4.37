#!/usr/bin/env bash
# Build one air-gapped package containing server images and a Desktop installer.
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

OUT_DIR="dist/offline-installer"
SERVER_PLATFORM="linux/amd64"
DESKTOP_TARGET=""

usage() {
  cat <<'USAGE'
Usage: scripts/offline-installer.sh [options]

  --output DIR             Output directory (default: dist/offline-installer)
  --platform PLAT          Server image platform (default: linux/amd64)
  --desktop-target TARGET  mac-arm64, mac-x64, linux-x64, linux-arm64,
                           win-x64, or win-arm64
  -h, --help               Show this help.
USAGE
}

while [ $# -gt 0 ]; do
  case "$1" in
    --output)
      [ $# -ge 2 ] || { echo "--output needs a directory" >&2; exit 1; }
      OUT_DIR="$2"
      shift 2
      ;;
    --platform)
      [ $# -ge 2 ] || { echo "--platform needs a value" >&2; exit 1; }
      SERVER_PLATFORM="$2"
      shift 2
      ;;
    --desktop-target)
      [ $# -ge 2 ] || { echo "--desktop-target needs a value" >&2; exit 1; }
      DESKTOP_TARGET="$2"
      shift 2
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "Unknown argument: $1" >&2
      usage >&2
      exit 1
      ;;
  esac
done

if [ -z "$DESKTOP_TARGET" ]; then
  case "$(uname -s):$(uname -m)" in
    Darwin:arm64) DESKTOP_TARGET="mac-arm64" ;;
    Darwin:x86_64) DESKTOP_TARGET="mac-x64" ;;
    Linux:x86_64) DESKTOP_TARGET="linux-x64" ;;
    Linux:arm64|Linux:aarch64) DESKTOP_TARGET="linux-arm64" ;;
    *) echo "Pass --desktop-target on this host." >&2; exit 1 ;;
  esac
fi

case "$DESKTOP_TARGET" in
  mac-arm64) DESKTOP_FLAGS=(--mac --arm64); DESKTOP_LABEL="macOS arm64" ;;
  mac-x64) DESKTOP_FLAGS=(--mac --x64); DESKTOP_LABEL="macOS x64" ;;
  linux-x64) DESKTOP_FLAGS=(--linux --x64); DESKTOP_LABEL="Linux x64" ;;
  linux-arm64) DESKTOP_FLAGS=(--linux --arm64); DESKTOP_LABEL="Linux arm64" ;;
  win-x64) DESKTOP_FLAGS=(--win --x64); DESKTOP_LABEL="Windows x64" ;;
  win-arm64) DESKTOP_FLAGS=(--win --arm64); DESKTOP_LABEL="Windows arm64" ;;
  *) echo "Unsupported Desktop target: $DESKTOP_TARGET" >&2; exit 1 ;;
esac

VERSION="${VERSION:-v$(node -p "require('./apps/web/package.json').version")-selective}"
SAFE_VERSION="$(printf '%s' "$VERSION" | tr '/ ' '__' | tr -cd '[:alnum:]._-')"
SAFE_SERVER_PLATFORM="$(printf '%s' "$SERVER_PLATFORM" | tr '/' '-')"
PACKAGE_NAME="multica-offline-${SAFE_VERSION}-${SAFE_SERVER_PLATFORM}-${DESKTOP_TARGET}"
PACKAGE_DIR="$OUT_DIR/$PACKAGE_NAME"
ARCHIVE="$OUT_DIR/$PACKAGE_NAME.tar.gz"

mkdir -p "$OUT_DIR"
rm -rf "$PACKAGE_DIR"
mkdir -p "$PACKAGE_DIR/desktop"

echo "==> Building server bundle for $SERVER_PLATFORM"
bash scripts/offline-bundle.sh --output "$PACKAGE_DIR/server" --platform "$SERVER_PLATFORM"

echo "==> Building Desktop installer for $DESKTOP_LABEL"
MULTICA_DESKTOP_VERSION="${VERSION#v}" \
CSC_IDENTITY_AUTO_DISCOVERY="${CSC_IDENTITY_AUTO_DISCOVERY:-false}" \
  pnpm --filter @multica/desktop package -- "${DESKTOP_FLAGS[@]}" --publish never

shopt -s nullglob
desktop_artifacts=(apps/desktop/dist/multica-desktop-*)
if [ "${#desktop_artifacts[@]}" -eq 0 ]; then
  echo "Desktop packaging completed without an artifact in apps/desktop/dist" >&2
  exit 1
fi
cp "${desktop_artifacts[@]}" "$PACKAGE_DIR/desktop/"
shopt -u nullglob

cat >"$PACKAGE_DIR/README.md" <<README
# Multica offline installer

This package contains the server images and the $DESKTOP_LABEL Desktop
installer built from the same checkout and git revision.

## Server

```bash
cd server
docker load -i multica-images.tar.gz
cp .env.example .env
```

Set a strong `JWT_SECRET`, the PostgreSQL password, and reachable
`MULTICA_PUBLIC_URL` / `MULTICA_APP_URL`. Keep the image overrides from
`server/README.md`, then start the stack:

```bash
docker compose -f docker-compose.selfhost.yml up -d
curl -sf http://localhost:8080/health
```

## Desktop

Install the file under `desktop/` on a matching client machine and configure
the server endpoint on first launch. The installer and server images were built
from the same revision. The macOS package is ad-hoc signed and not notarized
without Apple credentials, so Gatekeeper may require right-click → Open.
README

if command -v shasum >/dev/null 2>&1; then
  (cd "$PACKAGE_DIR" && find . -type f -not -name SHA256SUMS -print0 | sort -z | xargs -0 shasum -a 256) >"$PACKAGE_DIR/SHA256SUMS"
elif command -v sha256sum >/dev/null 2>&1; then
  (cd "$PACKAGE_DIR" && find . -type f -not -name SHA256SUMS -print0 | sort -z | xargs -0 sha256sum) >"$PACKAGE_DIR/SHA256SUMS"
fi

rm -f "$ARCHIVE"
tar -czf "$ARCHIVE" -C "$OUT_DIR" "$PACKAGE_NAME"

if command -v shasum >/dev/null 2>&1; then
  ARCHIVE_SHA="$(shasum -a 256 "$ARCHIVE" | awk '{print $1}')"
elif command -v sha256sum >/dev/null 2>&1; then
  ARCHIVE_SHA="$(sha256sum "$ARCHIVE" | awk '{print $1}')"
else
  ARCHIVE_SHA="unavailable"
fi
printf '%s  %s\n' "$ARCHIVE_SHA" "$(basename "$ARCHIVE")" >"$ARCHIVE.sha256"

echo "✓ Offline installer ready"
echo "  directory: $PACKAGE_DIR"
echo "  archive:   $ARCHIVE"
echo "  sha256:    $ARCHIVE_SHA"
