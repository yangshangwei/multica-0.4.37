# Intranet desktop release storage

The desktop main process uses `desktop.json.updateUrl` as a generic
electron-updater feed. Installed configuration defaults to
`http://<api-host>:18080/desktop`, independent of the business URL's protocol,
port, or path. Preserve the full hostname (including `api.`) and IPv6 brackets.
The packaged loader atomically backfills missing update URLs without changing
other stored fields. Explicit custom sources remain untouched; saving a new
server moves a source equal to the old server's default to the new host.
If backfilling fails, log the write failure and keep the valid configuration
and derived source available in memory. An explicit save still reports failures.

Missing business configuration requires setup and prevents update checks.
The updater reads the current configuration before every check, so a saved
server takes effect without restarting the main process. Manual file edits
still require a full restart. Development mode does not load or migrate this
file or enable the packaged update flow.

Coalesce checks only for the same update source. A different source must wait
for the previous check and its download to settle before switching providers.
Queued automatic checks must recheck the preference and current URL when they
resume; disabling automatic updates or changing servers discards stale work.

## Release contracts

- Use `apps/desktop/scripts/update-artifacts.mjs` for offline collection and
  publication. It uses the installed electron-updater YAML parser; when upgrading
  that dependency, run its artifact tests to verify the parser import contract.
- Preserve generated channel filenames and artifact bytes. Verify SHA-512,
  sizes and safe local references before publishing. Stable versions are the
  default; prereleases require explicit opt-in for local testing.
- Keep publication staging, metadata snapshots and locks outside `public/`.
  Publish complete immutable artifacts first, atomically rename channel YAML,
  then switch the generated Windows download projection last when present.
  Each mutable file switches independently; this is not a multi-channel or
  power-loss transaction. Do not overwrite a versioned filename with new bytes.
- `offline-installer.sh` must include update metadata, the generated download
  projection when present, ZIPs and blockmaps, not
  just a glob of installer filenames. Reject unapproved prerelease VERSION
  before expensive builds.
- Setting `autoUpdater.channel` enables downgrades as a side effect. Explicitly
  restore `allowDowngrade = false` for every architecture-specific channel.

## Local service

`docker-compose.desktop-updates.yml` runs the separate Nginx project. Mount only
the host `data/desktop-updates/public` directory read-only. The packaging `dist`
directory is transient and must never be the persistent download store.
The lifecycle wrapper starts with `--pull never`; prepare or load images first.
Retag the verified image digest before saving an offline image archive, because
pulling by digest need not assign the mutable source tag.

Nginx must serve direct GET/HEAD downloads, Range requests and revalidated
metadata, with no directory listing or upload API. Default binding is loopback;
LAN binding is explicit. Status reads actual container bindings/mounts.

The lifecycle wrapper auto-loads `updates.env` beside the Compose file or accepts
`--env-file PATH`. Resolve dotenv through `docker compose config --environment`;
never source or eval configuration. Only consume the known desktop variables.
Exported environment variables retain Compose precedence. Relative positional
paths refer to the caller; relative storage refers to the Compose directory.
Custom env files must supply `DESKTOP_UPDATES_URL` for configure/verify so a LAN
deployment cannot accidentally distribute a loopback update URL. Keep runtime
configuration out of git; `deploy/desktop-updates/updates.env.example` is tracked.

`export-image OUTPUT PLATFORM` reads the pinned image from Compose, explicitly
selects the target architecture for pull, inspect and save, and uses an
architecture-specific local tag. It requires platform-aware Docker APIs and
must not overwrite an archive or restart services. The minimal lifecycle kit
needs Bash/Docker/Compose; publication and HTTP verification additionally need
the real repository's Node helpers and dependencies. Keep this distinction
explicit in deployment instructions.

## Configuration and verification

`configure-updates.mjs` validates with the existing runtime parser, retains
business and unknown fields, backs up original bytes and atomically changes
only `updateUrl`. It requires Node >=22.18 for the TypeScript parser import.
Do not invent business endpoints when configuration is absent.

Use artifact/configuration/updater regressions and an actual HTTP full-file
checksum plus `206` check. Container restart must retain host artifacts.
Download checks do not prove signed installation, elevation, offline certificate
trust or user-state retention; those need an installed older client on each
target OS/architecture.

`verify-updates.mjs` shares the publisher's reference validation and checks one
channel, optionally asserting the exact expected version. Reject redirects,
credentialed/external/path-traversing references and transformed content;
bound response sizes and duration. Stream the full SHA-512 and compare both
HEAD length and actual Range bytes to the full download. Optional blockmaps
have no expected source hash unless metadata references them: report their
availability and transport checks without claiming source integrity. Success
prints JSON evidence; any failed check exits nonzero.

Shell regressions run with `node --test scripts/desktop-updates.test.mjs` and
are wired into the frontend CI job. Real HTTP fixture tests belong beside the
desktop verifier. Actual Docker rehearsals should use independent projects,
temporary storage and fixed host ports; port 0 can change on container restart.

## Candidate version provenance

Git-derived versions depend on tags present in the checkout; an unpublished local
release tag is not available to hosted CI. Candidate CI must provide one validated
`MULTICA_DESKTOP_VERSION` to both the desktop package and bundled CLI and use that
same value for installed-version checks. Keep the candidate numeric version above
the selected stable upgrade baseline. Do not publish a release tag merely to make
an acceptance build newer. Preserve prerelease channel metadata with its artifacts.

## Windows protocol ownership on uninstall

Electron registers `multica://` when a packaged app launches. The NSIS target does
not consume the top-level `protocols` configuration (unlike macOS/Linux/Appx), so
installer-only smoke cannot reveal the stale protocol left after actual GUI use.
`build/installer.nsh` removes the current user's protocol only when its command
exactly matches the uninstalling executable, and preserves it during upgrade.
Never delete a handler reassigned to another installation. Native business
acceptance checks real GUI registration cleanup plus a foreign-handler fixture.

## Static first-install download page

### 1. Scope / Trigger

Use this contract when changing the Nginx download page, its deployment kit,
or the collect/publish output consumed by first-time installers. This page is
the only desktop download page: Web has no `/download` route and links here
through `MULTICA_DESKTOP_DOWNLOAD_URL` (see `server/offline-delivery.md`).

### 2. Signatures

- `collectArtifacts({ source, destination, allowPrerelease })` returns every
  exposed filename, including `download.json` when `latest.yml` is present.
- `publishArtifacts(...)` exposes the same projection under `destination/public`.
- `desktop-updates.sh start` atomically installs `download.html` and creates
  `download-config.json` only if absent; its Node-free lifecycle still requires
  only Bash, Docker and Compose.
- `GET /desktop` returns a relative `308 /desktop/`; `/desktop/` serves the HTML.
  Existing concrete file URLs, GET/HEAD/Range and `/health` remain supported.

### 3. Contracts

`download.json` is `{ schemaVersion: 1, platform: "windows", arch: "x64",
version: string, file: string, size: positive safe integer }`. Generate it from
staged, checksum-validated `latest.yml`; its unique EXE must be exactly
`multica-desktop-${version}-windows-x64.exe`. Never trust source JSON. Other
platform channels must neither replace this projection nor enable pending UI.

`download-config.json` is `{ schemaVersion: 1, serverUrl: string }`. The tracked
example has an empty URL; the deployment administrator supplies the business
HTTP(S) endpoint without embedded credentials. Do not infer it from the download
host/port. Publishing and page upgrades must preserve these administrator bytes.

Expose immutable artifacts, then channel YAML, then the projection. Validate and
stage before exposure. Snapshot old JSON with metadata for a Windows publication.
If the final JSON rename fails, return failure; the prior JSON still references
a retained installer and retry can finish. Do not claim a cross-file transaction.

The HTML is self-contained and uses only same-origin data and installer requests.
JSON requests have a 10-second timeout and 16-KiB cap. Validate shape and filename,
then require an untransformed 200 HEAD with matching Content-Length before
enabling downloads. Address loading is independent of release loading. Provide
truthful clipboard feedback plus selectable text on HTTP or denied clipboard
access. Only Windows x64 is available; Mac Apple Silicon/Intel and Linux
x86/ARM64 stay visibly pending.

### 4. Validation & Error Matrix

| Condition | Required result |
| --- | --- |
| Wrong EXE architecture/version or bad checksum | Reject before public exposure |
| Forged input download.json | Ignore it; regenerate from verified metadata |
| Non-Windows publication | Preserve current Windows JSON |
| Final projection rename fails | Nonzero/error, safe old pointer, retry succeeds |
| Missing/malformed JSON, timeout, wrong installer HEAD | No active download link; static guide and retry remain |
| Invalid/missing server URL | No invented address; downloading remains independent |
| Existing administrator config | Preserve byte-for-byte on start and publication |
| Symlink/non-file page/config target | Refuse lifecycle write; keep existing files |
| Clipboard API unavailable/denied | Try supported copy capability, otherwise keep selected address and report manual-copy instructions |

### 5. Good / Base / Bad Cases

- Good: collect a verified Windows release, install its files with JSON last,
  then refresh the browser to see that same version without editing HTML.
- Base: a new store has no release/config yet; the page explains unavailable
  data and still presents the installation guide.
- Bad: copy the marketing page with its GitHub fallback, infer a filename,
  overwrite config from the template, or expose JSON before the installer.

### 6. Tests Required

- `update-artifacts.test.mjs`: derived fields, hostile inputs, non-Windows
  preservation, backups, immutable files, failed final rename and retry.
- `download-page.test.mjs`: execute the real HTML and assert enabled links,
  pending platforms, independent failures, bounded responses and copy feedback.
- `scripts/desktop-updates.test.mjs`: atomic owned-file installation, config
  preservation, argument/path handling and failed target preflight.
- Real isolated Nginx/browser: HTML/JSON MIME, relative redirect, refresh after
  publication, full download/206, no external requests, HTTP clipboard,
  keyboard access, narrow layout and persistence after container restart.

### 7. Wrong vs Correct

Wrong: `cp -r release/* public/` and assume transfer order makes a release safe.

Correct: use `publishArtifacts`; on a Node-free download server, copy verified
installers first, replace channel YAML atomically, and replace `download.json`
last. Restore both YAML and JSON from the same backup point when withdrawing a
version. Existing installed clients do not downgrade automatically.

Updating a bind-mounted Nginx config needs `nginx -t` and a reload; Compose up
alone may reuse the running process. Atomic replacement of the host config file
can also leave an existing single-file bind mount on the old inode, requiring
container recreation. Page/version file changes use the directory mount and do
not require a routine Nginx restart.
