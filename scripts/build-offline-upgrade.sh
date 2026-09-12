#!/usr/bin/env bash
# Build a versioned server upgrade archive from this checkout.
#
# The archive contains the three runtime images, the exact compose file used to
# run them, and the operator script/documentation needed on the offline host.
# Run this on a machine with network access; the target host only needs Docker.
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

OUT_DIR="dist/offline-upgrade"
PLATFORM="linux/amd64"
CHANGELOG_ARGS=()

usage() {
  cat <<'USAGE'
Usage: scripts/build-offline-upgrade.sh [--output DIR] [--platform PLAT] [--changelog JSON]

  --output DIR     Output directory (default: dist/offline-upgrade)
  --platform PLAT  Image target (default: linux/amd64; use linux/arm64 for ARM)
  --changelog JSON Cumulative feed to embed and install (defaults to seed)
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
      PLATFORM="$2"
      shift 2
      ;;
    --changelog)
      [ $# -ge 2 ] || { echo "--changelog needs a JSON file" >&2; exit 1; }
      CHANGELOG_ARGS=(--changelog "$2")
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

VERSION="${VERSION:-$(git describe --tags --match 'v[0-9]*' --dirty 2>/dev/null || echo "v0.0.0-0-g$(git rev-parse --short HEAD 2>/dev/null || echo 0000000)")}"
SAFE_VERSION="$(printf '%s' "$VERSION" | tr '/ ' '__' | tr -cd '[:alnum:]._-')"
SAFE_PLATFORM="$(printf '%s' "$PLATFORM" | tr '/' '-')"
PACKAGE_NAME="multica-server-upgrade-${SAFE_VERSION}-${SAFE_PLATFORM}"
PACKAGE_DIR="$OUT_DIR/$PACKAGE_NAME"
ARCHIVE="$OUT_DIR/$PACKAGE_NAME.tar.gz"

mkdir -p "$OUT_DIR"
rm -rf "$PACKAGE_DIR"
mkdir -p "$PACKAGE_DIR"

echo "==> Building runtime bundle from commit $(git rev-parse --short HEAD 2>/dev/null || echo unknown)"
bash scripts/offline-bundle.sh --output "$PACKAGE_DIR" --platform "$PLATFORM" "${CHANGELOG_ARGS[@]+"${CHANGELOG_ARGS[@]}"}"

# The generic offline bundle README describes first installation. An upgrade
# archive must be self-contained for the operator on the existing server, so
# replace it with the server-side upgrade guide.
cp docs/offline-upgrade.zh-CN.md "$PACKAGE_DIR/README.md"
cp docs/offline-upgrade.zh-CN.md "$PACKAGE_DIR/操作文档.md"
cp scripts/offline-upgrade.sh "$PACKAGE_DIR/offline-upgrade.sh"
chmod 0755 "$PACKAGE_DIR/offline-upgrade.sh"

cat >>"$PACKAGE_DIR/MANIFEST.txt" <<MANIFEST

package:
  name:     $PACKAGE_NAME
  archive:  $(basename "$ARCHIVE")
  operator: offline-upgrade.sh
  guide:    操作文档.md
MANIFEST

rm -f "$ARCHIVE"
tar -czf "$ARCHIVE" -C "$OUT_DIR" "$PACKAGE_NAME"

if command -v shasum >/dev/null 2>&1; then
  sha256="$(shasum -a 256 "$ARCHIVE" | awk '{print $1}')"
elif command -v sha256sum >/dev/null 2>&1; then
  sha256="$(sha256sum "$ARCHIVE" | awk '{print $1}')"
else
  sha256="unavailable"
fi
printf '%s  %s\n' "$sha256" "$(basename "$ARCHIVE")" >"$ARCHIVE.sha256"

echo ""
echo "✓ Upgrade package ready"
echo "  directory: $PACKAGE_DIR"
echo "  archive:   $ARCHIVE"
echo "  sha256:    $sha256"
echo "  checksum:  $ARCHIVE.sha256"
