# Release runbook

## Normal release

Release from a reviewed commit on `main` by creating and pushing a new semantic
version tag such as `v0.18.4`. The Release workflow intentionally has no manual
trigger: a tag push is the only event that can publish binaries, Homebrew
formulae, and container images.

The verification job runs the Go tests and `govulncheck` before any publishing
job starts. The vulnerability scan is fail-closed by default.

### Fork tags and imported upstream tags

Every build stamp — `make build`, the Desktop package, the CLI bundled into it,
and the release images — comes from `git describe --tags --match 'v[0-9]*'`,
which only walks the ancestry of the commit being built. A `v*` tag that points
at an upstream commit outside `main`'s history is therefore invisible to every
build and only makes the tag list lie about what is current.

- Create fork tags only on commits reachable from `main`. The first fork tag is
  `v0.4.44`; continue from there.
- Keep imported upstream tags under the `upstream-v<version>` prefix. Fetch the
  upstream remote with tags disabled so bare `v*` tags never come back:

  ```bash
  git config remote.upstream.tagOpt --no-tags
  git fetch upstream 'refs/tags/v*:refs/tags/upstream-v*'
  ```

- Never reuse or move an imported tag. A version number upstream already
  published is skipped, not reclaimed.
- Desktop auto-update requires monotonic versions, so a fork tag must be higher
  than any Desktop build already installed on the target deployment.

## Changelog artifacts and fork releases

The same tag push also generates a cumulative, deployment-owned changelog.
This checkout belongs to `yangshangwei/multica-0.4.37`; the official Multica
entry in its seed is attributed reference material. Imported upstream tags,
package versions, local bundles, and an `Unreleased` preview are not evidence
that this fork has published a release. Use a new fork tag; never reuse or move
an imported upstream tag.

The workflow runs under one repository-wide concurrency group, with active-run
cancellation disabled. History discovery, builds, and publication therefore
cannot race across two tags. GitHub can supersede a pending run; such a run has
not published a release.

1. `verify` runs the changelog fixture tests, release dependency-graph checks,
   existing Go tests, and the vulnerability gate.
2. `changelog` retrieves this repository's releases with the Actions token,
   including prereleases and excluding drafts. It validates the JSON, Markdown,
   and immutable metadata and chooses a cumulative history containing every
   prior publication. It does not use `/releases/latest`, which omits prerelease
   history. Only a successfully retrieved list with zero published releases
   permits bootstrap from the checked-in seed. Authentication, network, invalid
   metadata, missing assets, and conflicting histories fail the run.
3. Generation resolves the tag and ancestral base to immutable commits, emits
   `changelog.json`, `changelog.md`, and `changelog-metadata.json` once, and saves
   them in the `release-changelog` workflow artifact. Metadata includes the
   repository, tag, commit range, publication timestamp, and SHA-256 digests.
4. The backend and web builds download that exact artifact. The generated JSON
   is installed as the backend's embedded baseline before compilation. No build
   independently regenerates notes or invents a new timestamp.
5. `publish-changelog` requires verification and both image manifests to succeed.
   It checks the requested repository/tag against the built metadata and the
   current GitHub tag, then revalidates current published cumulative history
   before any remote write. It creates a draft in **this repository**, uploads all
   three artifacts, then publishes the Release with the generated Markdown body.
   It has no dependency on the upstream-only GoReleaser, Homebrew, Helm, or
   desktop-binary jobs. Those guards remain in place; these notes do not claim
   that a desktop installer was built.

A failed upload leaves a new Release as a draft. Retry the workflow for the
same tag after fixing the failure. If another release has published since that
tag's artifacts were built, rerunning only the failed publication job is
rejected before any mutation when those artifacts omit or conflict with current
history. Rerun full preparation and all builds so the published bytes remain
exactly the bytes used in the images; publication never regenerates notes.
Exact retries do not mutate an already
identical release. An older-version retry retains later stable/prerelease
history and does not replace a higher stable version's latest marker, even if
the older version is prepared later. A changed commit,
status, or immutable release content under an existing identity is rejected.
Do not delete old artifacts to force seed fallback; restore the verified
cumulative artifact before retrying instead.

The checked-in first-release base is the local ancestral marker
`f1c059edee4ec39d464915e9c9a83cf6c0c37925`. Higher, unrelated imported tags are
never selected as this fork's base. A deployment of these scripts in a different
fork needs its own attributed seed and explicit first-release base; the
generator intentionally refuses mixed-fork histories.

When an older first tag is rebuilt after a later tag has published, there may
still be no published ancestor for that older commit. Preparation then uses the
already-configured `--first-base`, verifies that it is an ancestor, and retains
the full downloaded history. This base selection never permits resetting
history to the seed. Existing ancestral publications still take precedence;
incomparable ancestral bases require an explicit `--base`.

## Writing public notes

The generator includes merged non-merge commits, meaningful Conventional
Commits, and legacy intent-style subjects. It omits routine `docs`, `test`,
`chore`, `build`, and `ci` subjects, including scoped forms. `feat` becomes
Features, `fix` becomes Fixes, and `perf` becomes Improvements. Other meaningful
subjects remain visible under Other.

Use native Git trailers to supply public wording or an explicit category:

```text
feat(changelog): Keep deployed clients informed

Release-note: See newly published updates without reopening the app
Changelog: improvements
```

Categories are `features`, `improvements`, `fixes`, and `other`.
`Changelog: skip` suppresses a commit. Invalid, empty, duplicate, or conflicting
overrides fail with the relevant commit ID. A public note overrides the default
housekeeping filter. Notes are plain text; Markdown output escapes formatting
and HTML rather than interpreting commit text as commands or markup.

## Local generation and cumulative history

Node 22 and Git are needed on the generation/build machine. `--history` is
required, even for the first release. Keep the previous published cumulative
JSON as the next input; never reset a later release to the seed.

For an unpublished review of this checkout's frozen inspected range:

```bash
node scripts/generate-changelog.mjs \
  --ref 719cb82d02c1ff80b80e094ba39ffd0f040950a5 \
  --base f1c059edee4ec39d464915e9c9a83cf6c0c37925 \
  --history server/internal/changelog/content/changelog.json \
  --repository yangshangwei/multica-0.4.37 \
  --version Unreleased --status unreleased \
  --output /tmp/multica-changelog.json --markdown /tmp/multica-changelog.md
```

Preview reads committed Git objects and ignores dirty worktree bytes. Published
generation requires a clean tracked tree, an immutable full commit or tag, a
`vX.Y.Z[-suffix]` version, and an explicit stable `--published-at` RFC3339 value.
Use `--status prerelease` for suffixed versions. After the first release, omit
`--base` to select the unique nearest ancestral published/prerelease commit;
incomparable histories require an explicit ancestral base.

The JSON has a 2 MiB maximum, strict schema version 1, unique source/repository/
version identities, and one fork repository. Oversized or invalid histories
are rejected without dropping older entries. Release generation removes a
covered preview but preserves genuinely later preview work.

## Delivering notes to an intranet or offline deployment

The runtime API reads a configured local file; it does not call GitHub. A
disconnected server learns about a release when its artifact reaches that
deployment through the existing transfer process. Existing and newly installed
clients then read the same history from their configured authenticated API.

Carry the generated cumulative file with an existing package command:

```bash
bash scripts/offline-bundle.sh --changelog /path/to/changelog.json
bash scripts/build-offline-upgrade.sh --changelog /path/to/changelog.json
bash scripts/offline-installer.sh --desktop-target win-x64 \
  --changelog /path/to/changelog.json
```

`CHANGELOG_ARTIFACT` is the equivalent build environment setting. With neither
override, the current attributed embedded seed is carried unchanged; its local
preview stays unpublished. The builder validates/stages the file under its own
temporary `.changelog-build/` directory, passes it as `CHANGELOG_ARTIFACT_PATH`
to the Docker build, copies the exact bytes into the package, and removes its
temporary stage on exit. It never rewrites the source seed.

Each package contains `changelog/changelog.json`, `install-changelog.sh`, and
all required publisher modules under `scripts/`. On the offline server, load
the bundled images, prepare `.env` as described by the package README, then run:

```bash
bash install-changelog.sh --deployment-dir /srv/multica
docker compose --project-directory /srv/multica --env-file /srv/multica/.env \
  -f docker-compose.selfhost.yml up -d --pull never
```

For an existing installation, `offline-upgrade.sh --deployment-dir /srv/multica`
performs this handoff after image import and backups and before recreating
services. Its backup keeps the original `.env`.

The installer uses Node in the already-loaded frontend image with
`docker run --pull never --network none --entrypoint node --user UID:GID`.
The target needs Docker and Compose, **not a host Node installation**. It uses
structured Compose JSON to resolve paths; an initial container reads that JSON
through stdin without bind mounts and rejects complete CR/LF-containing paths
before creating host directories. It then uses directory mounts and checks
write access by staging flushed temporary files. Literal double quotes and
commas in package, deployment, or feed paths are preserved by CSV-encoding each
Docker mount source. The installer persists only these two settings while
preserving unrelated `.env` values:

```dotenv
CHANGELOG_FILE='/app/data/changelog/changelog.json'
CHANGELOG_DIRECTORY='/srv/multica/changelog'
```

An existing custom `CHANGELOG_DIRECTORY` is resolved relative to the deployment
and retained as an absolute path. An incompatible nonempty `CHANGELOG_FILE`
fails with an explanation. Never bind-mount only `changelog.json`: atomic
replacement requires mounting its containing directory so the server opens the
new inode.

For later feed-only updates on a Node-equipped operator machine, use the same
standalone publisher against the mounted directory:

```bash
node scripts/publish-changelog.mjs --input /path/to/changelog.json \
  --destination /srv/multica/changelog/changelog.json
```

For a target without Node, replace the package's incoming
`changelog/changelog.json` with the new cumulative artifact and rerun
`install-changelog.sh --deployment-dir /srv/multica --web-image multica-web:dev`
using its already-loaded frontend tag. No server restart is needed once the
mount and `CHANGELOG_FILE` are active. Reopen or refresh Help → Changelog;
an already-active reader also checks every 60 seconds.

Validation, staging, or image-execution failures leave the previous feed and
configuration untouched. If configuration finalization fails after feed rename,
the installer restores the old feed. If the filesystem also prevents rollback,
it reports that explicitly; restore the deployment backup/last cumulative file
before retrying. Keep the successful JSON as the next generation's `--history`.
The API preserves its last valid content with a visible stale warning when a
configured file later becomes missing or invalid.

Focused verification commands:

```bash
node --test scripts/*changelog*.test.mjs
# Optional real-container smoke: requires an already-loaded frontend image.
MULTICA_RUN_DOCKER_CHANGELOG_SMOKE=1 node --test scripts/offline-changelog.test.mjs
```

### Delivering Desktop updates to an intranet deployment

Installed Desktops check the publish feed compiled into the package. A
deployment that ships its own Desktop builds points them at a static directory
instead by setting `updateUrl` in each client's `~/.multica/desktop.json`
(see the Desktop documentation). The directory serves the packaging output
unchanged; nothing is renamed:

```text
<updateUrl>/
  latest.yml                                    # Windows x64 metadata
  latest-arm64.yml                              # Windows arm64 metadata
  multica-desktop-<version>-windows-<arch>.exe
  multica-desktop-<version>-windows-<arch>.exe.blockmap
  latest-mac.yml                                # macOS arm64 metadata
  latest-x64-mac.yml                            # macOS x64 metadata
  multica-desktop-<version>-mac-<arch>.zip
  multica-desktop-<version>-mac-<arch>.zip.blockmap
  latest-linux.yml                              # Linux x64 metadata
  latest-linux-arm64.yml                        # Linux arm64 metadata
  multica-desktop-<version>-linux-<arch>.AppImage
```

Copy the new version's files in, then replace the `latest*.yml` files last so a
client never reads metadata for an installer that is not there yet. Keep the
previous version's `.blockmap` alongside the new one; electron-updater uses it
for differential downloads. Clients poll hourly and on startup, download in the
background, and install on the next quit.

`updateUrl` only takes effect when `desktop.json` parses. A client whose file
is invalid shows the configuration error and, because no configuration was
loaded, falls back to the feed compiled into the package. On an intranet that
fallback fails harmlessly, but a machine with internet access would check the
upstream release feed, so fix the file rather than leaving it broken.

## Emergency vulnerability-scan bypass

Use the bypass only when `govulncheck` itself or its live vulnerability database
is unavailable, or when maintainers have documented a confirmed false positive
that blocks an urgent release. Never use it to publish a release with an
unresolved reachable vulnerability.

1. Record the reason and maintainer approval in the release issue or pull
   request, and confirm no other release is in progress.
2. In **Settings → Secrets and variables → Actions → Variables**, set the
   repository variable `ALLOW_VULN_BYPASS_FOR_TAG` to the exact release tag,
   for example `v0.18.4`.
3. Re-run the failed Release workflow for that tag. A different tag, an empty
   value, or any typo keeps the scan enabled.
4. Confirm the verification log contains the explicit bypass warning and retain
   the workflow URL in the incident record.
5. Delete `ALLOW_VULN_BYPASS_FOR_TAG` immediately after the release run
   completes. The tag-scoped value prevents a concurrent release with another
   tag from inheriting the bypass.

Every Go binary retains its compiler version in the standard Go build metadata;
use `go version -m <binary>` when auditing a downloaded release artifact.
