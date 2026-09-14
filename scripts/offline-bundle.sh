#!/usr/bin/env bash
# Build an air-gapped install bundle: the three container images the self-hosted
# stack runs, plus the compose file and env template needed to start them on a
# machine with no registry access.
#
# Run this on a machine WITH network. Nothing here can run inside the air gap:
# the backend image build runs `go mod download` and the web image build runs
# `pnpm install`, both of which need upstream package registries.
#
# Deliberately builds from the current checkout rather than pulling the
# published GHCR images: an offline site is usually offline because it runs a
# build the public images do not have (intranet device auth, for one), and
# pulling `:latest` would ship something else than the checkout under review.
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

COMPOSE_FILE="docker-compose.selfhost.yml"
BUILD_OVERLAY="docker-compose.selfhost.build.yml"

OUT_DIR="dist/offline"
DRY_RUN=0
CHANGELOG_INPUT="${CHANGELOG_ARTIFACT:-server/internal/changelog/content/changelog.json}"
# Defaults to the architecture the overwhelming majority of servers run, NOT to
# the build host. Getting this wrong is the worst failure this script can
# produce: an arm64 bundle built on a developer laptop loads fine and then every
# container dies with "exec format error" on the x86 server, after someone has
# already carried it across the air gap. A slow emulated build is visible here;
# a wrong-architecture bundle is only visible there.
PLATFORM="linux/amd64"

usage() {
  cat <<'USAGE'
Usage: scripts/offline-bundle.sh [--output DIR] [--platform PLAT] [--changelog JSON] [--dry-run]

  --output DIR     Where to write the bundle (default: dist/offline)
  --platform PLAT  Target platform for the images (default: linux/amd64).
                   Use linux/arm64 for an ARM server. Building for an
                   architecture other than the host's runs under emulation
                   and is slow.
  --dry-run        Print the plan and stage nothing. Touches no images.
  --changelog JSON Cumulative feed to embed and distribute (default: embedded
                   seed, or CHANGELOG_ARTIFACT). Local previews stay unreleased.
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
      [ $# -ge 2 ] || { echo "--platform needs a value, e.g. linux/amd64" >&2; exit 1; }
      PLATFORM="$2"
      shift 2
      ;;
    --dry-run)
      DRY_RUN=1
      shift
      ;;
    --changelog)
      [ $# -ge 2 ] || { echo "--changelog needs a JSON file" >&2; exit 1; }
      CHANGELOG_INPUT="$2"
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

# Env overrides first: a checkout that is not a git working tree (an exported
# tarball, a vendored copy) otherwise stamps "dev/unknown" into the manifest,
# and then nobody at the offline site can answer "which build is this?".
#
# Never `--always`: this VERSION is stamped into the bundled multica CLI too
# (Dockerfile builds ./cmd/multica with -X main.version), and its bare-hash
# fallback parses as neither semver nor the git-describe shape — so the CLI
# version gates fail closed and agent-create refuses the daemon at the offline
# site. Synthesize the describe shape from HEAD instead, which those gates
# exempt as a dev build. Keep in sync with the Makefile's VERSION.
VERSION="${VERSION:-$(git describe --tags --match 'v[0-9]*' --dirty 2>/dev/null || echo "v0.0.0-0-g$(git rev-parse --short HEAD 2>/dev/null || echo 0000000)")}"

# Resolve the same versioned tags that Compose builds. Reading the YAML text
# would save literal interpolation expressions or a different checkout's dev
# images. --images prints no runtime environment values or build-host secrets.
RESOLVED_IMAGES="$(VERSION="$VERSION" JWT_SECRET="${JWT_SECRET:-build-time-placeholder-not-shipped}" \
  docker compose -f "$COMPOSE_FILE" -f "$BUILD_OVERLAY" config --images)"
BACKEND_IMAGE="$(printf '%s\n' "$RESOLVED_IMAGES" | sed -n '/^multica-backend:/p' | head -1)"
WEB_IMAGE="$(printf '%s\n' "$RESOLVED_IMAGES" | sed -n '/^multica-web:/p' | head -1)"
# The database image is a literal in the compose file (no env override), so the
# bundle has to carry that exact tag or `docker compose up` reaches for a
# registry that is not there.
DB_IMAGE="$(sed -n '/^[[:space:]]*postgres:/,/^[[:space:]]*[a-z]/s/^[[:space:]]*image:[[:space:]]*\([^[:space:]]*\).*/\1/p' "$COMPOSE_FILE" | head -1)"

for var in BACKEND_IMAGE WEB_IMAGE DB_IMAGE; do
  if [ -z "${!var}" ]; then
    echo "Could not read $var out of the compose files." >&2
    echo "Their service/image shape changed; update scripts/offline-bundle.sh to match." >&2
    exit 1
  fi
done

IMAGES_ARCHIVE="$OUT_DIR/multica-images.tar.gz"

host_platform() {
  case "$(uname -m)" in
    x86_64|amd64) echo "linux/amd64" ;;
    arm64|aarch64) echo "linux/arm64" ;;
    *) echo "unknown" ;;
  esac
}

echo "==> Air-gapped bundle plan"
echo "    platform      : $PLATFORM"
echo "    backend image : $BACKEND_IMAGE (built from this checkout)"
echo "    web image     : $WEB_IMAGE (built from this checkout)"
echo "    database image: $DB_IMAGE (pulled)"
echo "    output        : $OUT_DIR"
echo "    changelog     : $CHANGELOG_INPUT"

if [ "$PLATFORM" != "$(host_platform)" ]; then
  echo ""
  echo "    NOTE: $PLATFORM is not this machine's architecture, so both image"
  echo "    builds run under emulation. Expect this to take a long time (the"
  echo "    Go build and the web build are the slow parts). Building on a"
  echo "    native $PLATFORM host instead is much faster."
fi

if [ "$DRY_RUN" = "1" ]; then
  echo "==> --dry-run: stopping before build."
  exit 0
fi

COMMIT="${COMMIT:-$(git rev-parse --short HEAD 2>/dev/null || echo unknown)}"
DATE="$(date -u +%Y-%m-%dT%H:%M:%SZ)"

mkdir -p "$OUT_DIR"

# Stage a validated artifact inside Docker's context without modifying the
# checked-in fallback. The exact bytes also travel in the offline package.
command -v node >/dev/null 2>&1 || { echo "Node 22 is required on the bundle build machine" >&2; exit 1; }
mkdir -p "$ROOT_DIR/.changelog-build"
CHANGELOG_BUILD_DIR="$(mktemp -d "$ROOT_DIR/.changelog-build/offline.XXXXXX")"
trap 'rm -rf "$CHANGELOG_BUILD_DIR"' EXIT
node scripts/publish-changelog.mjs --input "$CHANGELOG_INPUT" --destination "$CHANGELOG_BUILD_DIR/changelog.json"
CHANGELOG_ARTIFACT_PATH="${CHANGELOG_BUILD_DIR#"$ROOT_DIR"/}/changelog.json"
mkdir -p "$OUT_DIR/changelog" "$OUT_DIR/scripts"
cp "$CHANGELOG_BUILD_DIR/changelog.json" "$OUT_DIR/changelog/changelog.json"
for helper in changelog-lib.mjs publish-changelog.mjs install-changelog.mjs; do
  cp "scripts/$helper" "$OUT_DIR/scripts/$helper"
done
cp scripts/install-changelog.sh "$OUT_DIR/install-changelog.sh"
chmod 0755 "$OUT_DIR/install-changelog.sh"

# Compose interpolates the whole file before it builds anything, and the backend
# service declares JWT_SECRET with `:?` so an unset value aborts. The real secret
# belongs to the offline machine, not to this build host — it is generated there,
# in its own .env — so feed the interpolation a placeholder. It reaches no image:
# JWT_SECRET is a runtime environment variable of the backend service, not a
# build arg.
echo "==> Building images for $PLATFORM from the current checkout..."
VERSION="$VERSION" COMMIT="$COMMIT" DATE="$DATE" DOCKER_DEFAULT_PLATFORM="$PLATFORM" \
  CHANGELOG_ARTIFACT_PATH="$CHANGELOG_ARTIFACT_PATH" \
  JWT_SECRET="${JWT_SECRET:-build-time-placeholder-not-shipped}" \
  docker compose -f "$COMPOSE_FILE" -f "$BUILD_OVERLAY" build

# --platform matters here too: the database image is multi-arch, and `docker
# pull` without it would fetch the build host's architecture.
echo "==> Pulling $DB_IMAGE for $PLATFORM..."
docker pull --platform "$PLATFORM" "$DB_IMAGE"

# --platform is what keeps the archive to one architecture. Without it, `docker
# save` writes every platform the local store happens to hold for a tag — the
# database image is multi-arch, so a machine that pulled it natively at some
# point would ship both, silently inflating what someone has to carry across
# the air gap.
echo "==> Saving images to $IMAGES_ARCHIVE..."
docker save --platform "$PLATFORM" "$BACKEND_IMAGE" "$WEB_IMAGE" "$DB_IMAGE" | gzip >"$IMAGES_ARCHIVE"

echo "==> Staging compose file and env template..."
cp "$COMPOSE_FILE" "$OUT_DIR/"
cp .env.example "$OUT_DIR/"

archive_bytes="$(wc -c <"$IMAGES_ARCHIVE" | tr -d ' ')"
if command -v shasum >/dev/null 2>&1; then
  archive_sha="$(shasum -a 256 "$IMAGES_ARCHIVE" | awk '{print $1}')"
elif command -v sha256sum >/dev/null 2>&1; then
  archive_sha="$(sha256sum "$IMAGES_ARCHIVE" | awk '{print $1}')"
else
  archive_sha="unavailable (no shasum/sha256sum on the build machine)"
fi

cat >"$OUT_DIR/MANIFEST.txt" <<MANIFEST
Multica air-gapped bundle
built:    $DATE
version:  $VERSION
commit:   $COMMIT
platform: $PLATFORM
changelog: changelog/changelog.json

images:
  $BACKEND_IMAGE
  $WEB_IMAGE
  $DB_IMAGE

multica-images.tar.gz
  bytes:  $archive_bytes
  sha256: $archive_sha
MANIFEST

# The import side is three commands nobody remembers under pressure, and the
# image-name overrides are the step whose omission looks like a broken build
# rather than a missing setting. Ship them next to the tarball.
cat >"$OUT_DIR/README.md" <<README
# Multica air-gapped bundle

Built from commit \`$COMMIT\` ($VERSION) on $DATE for **$PLATFORM**. See
\`MANIFEST.txt\` for the image list and archive checksum.

If the server is not $PLATFORM, stop here — the containers will load and then
fail to execute. Rebuild the bundle with \`--platform\` set to that server's
architecture.

## Install on the offline machine

\`\`\`bash
docker load -i multica-images.tar.gz
cp .env.example .env
\`\`\`

Then edit \`.env\`. These are the settings this bundle needs:

\`\`\`
# Point Compose at the images you just loaded instead of the registry.
MULTICA_BACKEND_IMAGE=${BACKEND_IMAGE%%:*}
MULTICA_WEB_IMAGE=${WEB_IMAGE%%:*}
MULTICA_IMAGE_TAG=${BACKEND_IMAGE##*:}

JWT_SECRET=<openssl rand -hex 32>
POSTGRES_PASSWORD=<openssl rand -hex 24>
# Keep DATABASE_URL's password in sync with POSTGRES_PASSWORD.

# Reachable from the client machines, not localhost.
MULTICA_PUBLIC_URL=http://<server-host>:8080
MULTICA_APP_URL=http://<server-host>:3000

# Intranet mode: clients get a session from a device id, with no login step.
# This removes authentication from the deployment — read
# SELF_HOSTING.md "Intranet Mode - No Login" first.
MULTICA_DEVICE_AUTH_ENABLED=true
\`\`\`

Start it:

\`\`\`bash
bash install-changelog.sh --deployment-dir "\$PWD"
docker compose -f docker-compose.selfhost.yml up -d --pull never
curl -sf http://localhost:8080/health
curl -s http://localhost:8080/api/config    # expect "device_auth_available":true
\`\`\`

Compose publishes both ports on \`127.0.0.1\` by default, so other machines
cannot reach them yet. Bind them to the LAN or put a reverse proxy in front —
skipping this looks exactly like device auth failing to work.

Migrations are not a separate step: the backend container runs them before the
API starts.

The changelog installer validates the bundled cumulative feed, runs the publisher
using Node in the already-loaded frontend image, and saves only
\`CHANGELOG_FILE\` / \`CHANGELOG_DIRECTORY\` in \`.env\`. No host Node installation
or network pull is required. The containing directory is mounted read-only in
the backend, which sees atomic replacements without a restart. Keep
\`changelog/changelog.json\` as the next generator's \`--history\` input.

## Client machines

The desktop installers are NOT in this bundle — build them from the same commit
with \`cd apps/desktop && pnpm package\` and distribute them yourself. Device
auth needs the app and the server to come from the same build.

On each machine, write \`~/.multica/desktop.json\`:

\`\`\`json
{
  "schemaVersion": 1,
  "apiUrl": "http://<server-host>:8080",
  "wsUrl": "ws://<server-host>:8080/ws",
  "appUrl": "http://<server-host>:3000"
}
\`\`\`

## Agents

Agents run through an agent CLI on the runtime machine, which needs a model
endpoint. With no egress, assigning an issue to an agent will not run it until
an internal OpenAI-compatible gateway is reachable.
README

echo ""
echo "✓ Bundle ready in $OUT_DIR"
ls -1 "$OUT_DIR"
echo ""
echo "Next: copy that directory into the air-gapped network and follow its README.md."
echo "Desktop installers are separate — build them from this same commit with:"
echo "  (cd apps/desktop && pnpm package)"
