#!/usr/bin/env bash
# Run the bundled publisher using Node in an image already loaded offline.
set -euo pipefail

PACKAGE_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DEPLOYMENT_DIR=""
WEB_IMAGE=""
BACKEND_IMAGE=""
IMAGE_TAG=""
CONFIGURE_IMAGES=0
die() { echo "ERROR: $*" >&2; exit 1; }

# Docker parses each --mount argument as CSV after shell argv parsing. Quote
# the whole src field and double embedded quotes; shell quoting alone is not CSV.
mount_source() {
  local mount_path="$1"
  mount_path="${mount_path//\"/\"\"}"
  printf '"src=%s"' "$mount_path"
}

while [ $# -gt 0 ]; do
  case "$1" in
    --deployment-dir) [ $# -ge 2 ] || die "--deployment-dir needs a value"; DEPLOYMENT_DIR="$2"; shift 2 ;;
    --web-image) [ $# -ge 2 ] || die "--web-image needs a value"; WEB_IMAGE="$2"; shift 2 ;;
    --backend-image) [ $# -ge 2 ] || die "--backend-image needs a value"; BACKEND_IMAGE="$2"; CONFIGURE_IMAGES=1; shift 2 ;;
    --image-tag) [ $# -ge 2 ] || die "--image-tag needs a value"; IMAGE_TAG="$2"; CONFIGURE_IMAGES=1; shift 2 ;;
    -h|--help)
      echo "Usage: install-changelog.sh --deployment-dir DIR [--web-image loaded-image:tag] [--backend-image repository --image-tag tag]"
      echo "Validate/install the feed and persist its directory in the deployment .env. Docker and Compose are required; host Node is not."
      echo "Providing backend-image and image-tag also saves the selected backend/frontend images for subsequent Compose runs."
      exit 0 ;;
    *) die "unknown argument: $1" ;;
  esac
done

[ -n "$DEPLOYMENT_DIR" ] || die "--deployment-dir is required"
DEPLOYMENT_DIR="$(cd "$DEPLOYMENT_DIR" && pwd)"
[ -f "$DEPLOYMENT_DIR/.env" ] || die "deployment .env is missing"
for file in changelog/changelog.json scripts/changelog-lib.mjs scripts/publish-changelog.mjs scripts/install-changelog.mjs docker-compose.selfhost.yml; do
  [ -f "$PACKAGE_DIR/$file" ] || die "package is missing $file"
done
command -v docker >/dev/null 2>&1 || die "Docker is required"
docker compose version >/dev/null 2>&1 || die "Docker Compose is required"

if [ -z "$WEB_IMAGE" ]; then
  [ -f "$PACKAGE_DIR/MANIFEST.txt" ] || die "pass --web-image when no package manifest is present"
  WEB_IMAGE="$(sed -n '/^images:/,/^$/{s/^  //p;}' "$PACKAGE_DIR/MANIFEST.txt" | sed -n '2p')"
fi
[ -n "$WEB_IMAGE" ] || die "could not resolve the loaded frontend image"
if [ "$CONFIGURE_IMAGES" = "1" ]; then
  [ -n "$BACKEND_IMAGE" ] && [ -n "$IMAGE_TAG" ] || die "image selections require both --backend-image and --image-tag"
  [ "${WEB_IMAGE##*:}" = "$IMAGE_TAG" ] && [ "${WEB_IMAGE%:*}" != "$WEB_IMAGE" ] || die "loaded frontend image must use the selected image tag"
fi

# Compose resolves dotenv escaping and precedence; never execute an operator's
# .env as shell code. Keep its project directory independent of package location.
compose_json="$(docker compose --project-directory "$DEPLOYMENT_DIR" --env-file "$DEPLOYMENT_DIR/.env" -f "$PACKAGE_DIR/docker-compose.selfhost.yml" config --format json)"
# Validate structured values before any mkdir or bind mount. Line-oriented
# `config --environment` output loses embedded/trailing newlines in paths.
# This first container reads only stdin and needs neither host Node nor mounts.
resolved_paths="$(printf '%s' "$compose_json" | docker run --rm -i --pull never --network none --entrypoint node --user "$(id -u):$(id -g)" "$WEB_IMAGE" -e '
let input = "";
process.stdin.setEncoding("utf8");
process.stdin.on("data", (chunk) => {
  input += chunk;
  if (input.length > 2 * 1024 * 1024) {
    console.error("ERROR: Compose configuration exceeds the size limit");
    process.exit(1);
  }
});
process.stdin.on("end", () => {
  try {
    const backend = JSON.parse(input)?.services?.backend;
    const volumes = backend?.volumes?.filter((volume) => volume.target === "/app/data/changelog");
    const file = backend?.environment?.CHANGELOG_FILE ?? "";
    if (!Array.isArray(volumes) || volumes.length !== 1 || volumes[0].type !== "bind") throw new Error("missing unique changelog directory bind mount");
    const source = volumes[0].source;
    if (typeof source !== "string" || !source.startsWith("/") || /[\r\n]/.test(source) || source.includes("\0")) throw new Error("CHANGELOG_DIRECTORY must be an absolute single-line path");
    // Compose serializes literal dollars as $$ so its JSON can be reused as
    // input. Decode that one escaping layer before using the host path.
    const directory = source.replace(/\$\$/g, "$");
    if (file !== "" && file !== "/app/data/changelog/changelog.json") throw new Error("existing CHANGELOG_FILE is incompatible with the bundled mount");
    process.stdout.write(directory + "\n" + file);
  } catch (error) {
    console.error("ERROR: " + (error instanceof SyntaxError ? "invalid Compose JSON" : error.message));
    process.exitCode = 1;
  }
});
')"
# The producer above has rejected CR/LF, so this two-line handoff is unambiguous.
directory="$(printf '%s\n' "$resolved_paths" | sed -n '1p')"
configured_file="$(printf '%s\n' "$resolved_paths" | sed -n '2p')"
mkdir -p "$directory"
directory="$(cd "$directory" && pwd)"
args=(--input /changelog-input/changelog.json --destination /changelog-output/changelog.json --deployment-env /deployment/.env --directory "$directory")
[ -z "$configured_file" ] || args+=(--current-file "$configured_file")
if [ "$CONFIGURE_IMAGES" = "1" ]; then
  args+=(--backend-image "$BACKEND_IMAGE" --web-image "${WEB_IMAGE%:*}" --image-tag "$IMAGE_TAG")
fi

docker run --rm --pull never --network none --entrypoint node --user "$(id -u):$(id -g)" \
  --mount "type=bind,$(mount_source "$PACKAGE_DIR/scripts"),dst=/changelog-tools,readonly" \
  --mount "type=bind,$(mount_source "$PACKAGE_DIR/changelog"),dst=/changelog-input,readonly" \
  --mount "type=bind,$(mount_source "$directory"),dst=/changelog-output" \
  --mount "type=bind,$(mount_source "$DEPLOYMENT_DIR"),dst=/deployment" \
  "$WEB_IMAGE" /changelog-tools/install-changelog.mjs "${args[@]}"
