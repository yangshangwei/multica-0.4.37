# Debian arm64 installation fixture implementation

The existing directory-package journey now accepts one explicit, SHA-256-pinned
`.deb` source as an alternative. The default directory/volume transport keeps
`/opt/multica`; a real package-manager install uses `/opt/Multica`. Credentials
still travel through stdin, and the real Main, preload, renderer and password
form remain in use. The original case title and control assertions are retained.

## Source boundary

- `e2e/fixtures/collaboration-installed-main.ts`: mutually exclusive directory
  and `.deb` inputs, read-only `.deb` bind and its explicit hash, and typed
  installation evidence.
- `e2e/fixtures/collaboration-installed-main.mjs`: verifies the input archive
  before `dpkg --install`, checks the configured package's architecture/version,
  exact files and three original payload hashes, desktop/protocol/icons/launcher,
  and preservation of a test-owned standalone CLI sentinel. Both client modes
  assert the exact file protocol and complete renderer pathname.
- `e2e/collaboration-host-installed.spec.ts`: retains its existing journey and
  validates installed-package provenance when opted in. FPM's documented Linux
  Debian rule replaces SemVer `-` with `~`; the dpkg version is checked against
  that actual Main version. The bundled CLI must equal the Main version after
  removing the CLI build's single leading `v`; raw versions remain in evidence.

No application behavior, repository dependencies, authentication configuration,
admission defaults, timeouts or release metadata changed.

## Build and installation evidence

Task-owned artifact directory:
`/Volumes/artisan/code/2026/multica-0.4.37/.omx/reports/collaboration-deb-install-20261026/`.

`build-deb.py` used electron-builder `--prepackaged --linux deb --arm64
--publish never` on the accepted `1bfd169dc` directory. Build exit 0; all three
input payload sizes and hashes stayed unchanged. The `.deb` is 166,477,828 bytes,
SHA-256 `7e23d62da6189d128bb08dd2caa011a8ca61ea6ad5256d346c331ec794b1c346`.
ASAR version is `0.6.5-179-g1bfd169dc`; the CLI's raw version is
`v0.6.5-179-g1bfd169dc`, and Debian version is `0.6.5~179~g1bfd169dc`.
Commands, source/payload hashes and full build output are
in `build.json` and `build.log`.

The first actual, network-disabled `dpkg` attempt refused configuration because
the old browser image lacked libnotify4, xdg-utils and libsecret-1-0. Its original
`install-preflight.json`, `.stdout` and `.stderr` are preserved; the preflight
wrapper exited 1 even though the controller's error protocol exited 0.

With explicit parent authorization, a fresh fixture image was built from the
unchanged cached base using `docker build --pull=false`. Only that new image
downloaded system prerequisites during build; no repository dependency changed:

- Base image: `multica-gap-full-main:20261022`,
  `sha256:db164dd5195d37ded1e6732eec4cc6b87decac66825be15fae9c7b87b2bac6c4`.
- Acceptance image: `multica-collaboration-deb-acceptance:20261026`,
  `sha256:0d433ecf8ede66777494051568fd760468be288fd99d36d80bc68d022769471c`.
- Packages: libnotify4 `0.8.1-1` arm64, libsecret-1-0 `0.20.5-3` arm64 and
  xdg-utils `1.1.3-4.1` all. Exact Dockerfile, command, versions and full output
  are retained in `Dockerfile`, `image-build.json` and `image-build.log`.

The explicit second preflight passed once with `--network none`: wrapper exit
0, dpkg exit 0 and `install ok installed`. It verified the desktop entry,
`multica://` association, hicolor icons, `/usr/bin/multica-desktop` launcher and
byte-preservation of `/usr/bin/multica` sentinel. Both package fields and payload
hashes match. See `install-preflight-dependencies-ready.json` and its raw output.

The installer emitted a nonfatal `unshare: Operation not permitted` probe
message. Its extracted `postinst.sh` shows the existing fallback to a SUID
chrome-sandbox when user namespaces are unavailable. The established GUI
fixture uses `--no-sandbox`; this acceptance does not establish Chromium sandbox
support or Ubuntu AppArmor integration. No dependency checks were forced or
skipped.

## Handoff and remaining checks

`deb-overrides.json` contains only public fixture paths, hashes and the selected
local image. It selects `COLLABORATION_INSTALLED_DEB` plus its SHA-256,
executable/CLI hashes and existing ASAR pin, and clears the two directory-source
variables. Missing configured inputs fail instead of skipping or pulling images.

`node --check e2e/fixtures/collaboration-installed-main.mjs` and `git diff --check`
passed locally. The independent checker owns scoped lint and strict E2E TypeScript
verification. The main session owns actual installed Main and retained-directory
journeys against the isolated API/production Web and their final evidence,
cleanup and recording; installation preflight alone is not a browser-flow pass.

Remaining scope remains explicit: Debian 12 arm64 package/client with a separate
native macOS fixture Provider runtime, not other distributions/architectures,
macOS or Windows installers, vendor accounts, automatic updates or production
release. Prior original inventory passes retain their source provenance.

## First installed-Main run and bounded corrections

The first `.deb` Main browser attempt installed and booted the actual package,
then failed before runtime setup on the new CLI/Main version assertion. The raw
CLI embeds `git describe` with its leading `v` (`bundle-cli.mjs`), while package
and UI version normalization remove only one leading `v`. The fixture now uses
that exact normalization for comparison; all raw versions and payloads remain
unchanged. The original failure is retained under `runs/deb-installed/`.

Boot metadata is assigned to evidence immediately after `init`, before its
provenance assertions, so subsequent failures retain the actual install and raw
version observations. Nested `finally` now guarantees native container closure
even when daemon cleanup or ledger decoding rejects, and still writes evidence
if native closure itself fails. No timeout or business assertion was relaxed.
The parent session owns final `.deb` and directory snapshot browser reruns.
