# Offline deployment configuration

## Scope / trigger

Read when changing server environment settings, Compose delivery, offline
upgrades, or release verification. A new backend variable does not reach Docker
merely because it appears in `.env.example`.

## Signatures

`offline-upgrade.sh --deployment-dir DIR [--compose-file OVERLAY ...] --yes`
and `install-changelog.sh --deployment-dir DIR [--compose-file OVERLAY ...]`
accept ordered, repeated overlays. Relative paths resolve from `DIR`.

## Contracts

- The upgrade backs up `.env`, the old main Compose and selected overlays, then
  installs the bundled main Compose after changelog/config publication succeeds.
- Every Compose operation, including publisher mount discovery, uses the same
  overlay order. `compose-command.txt` records a quoted command prefix; it is
  not an automatic overlay loader. Operators must explicitly preserve the list.
- Existing secrets, authentication choice and data volumes remain intact.
- Compose defaults match backend semantics: `FF_PROJECTS_P1=true`,
  `FF_ITERATIONS_I1=false`, platform administration and managed installations off.
  Enabling I1 still requires an explicit workspace owner/admin action.

## Validation and errors

Missing/unreadable overlays and CR/LF paths fail before image loading or backup.
After image loading, the loaded frontend's Node validates the full target Compose
JSON without network or host mounts. Literal overlay images that defeat the
selected backend/frontend versions fail before backup or deployment writes.
Reusing a database backup path fails rather than overwriting it. Publication
failure does not replace the main Compose or recreate services. Container,
database, feed and configuration changes are not an atomic transaction: a later
failure retains evidence and requires inspected recovery, not automatic downgrade.

### Cross-platform artifact checks

On macOS, invoke packaging with `COPYFILE_DISABLE=1` (for example,
`COPYFILE_DISABLE=1 VERSION=v0.6.0 bash scripts/build-offline-upgrade.sh`).
Otherwise BSD tar may add AppleDouble `._*` metadata, including a sibling of the
package root. Reject unexpected archive roots, traversal, links/devices and
private deployment files. Re-archiving unchanged files to remove host metadata
requires a new outer SHA-256; retain the superseded hash only in build evidence.

Docker's containerd image store may expose a multi-platform index as `.Id`.
`docker image inspect pgvector/pgvector:pg17` can select the ARM build host's
variant even when the running database container is amd64. Use
`docker image inspect --platform linux/amd64 <image>` for target image checks;
validate the saved image config's `os`, `architecture`, `rootfs.diff_ids` and
runtime config. For running containers, inspect `ImageManifestDescriptor` and
confirm `docker exec <container> uname -m`; do not equate an OCI index digest
with a config digest or treat host-default inspection as runtime proof.

If an emulated frontend build exhausts V8's heap, keep the adjustment confined
to the build RUN, for example
`RUN NODE_OPTIONS=--max-old-space-size=4096 pnpm --filter @multica/web build`.
Record any copied Dockerfile/build-overlay hashes; do not imply the recipe was
byte-identical. Verify the final runtime image has no added `NODE_OPTIONS`.
After any rebuild, refresh binary hashes and module inspection against the final
saved/loaded image, then compare them with the running container. An earlier
successful inspection can reference obsolete binaries when a retry changes DATE.

Before creating an immutable release tag, wait for the frozen commit's current
main CI, including the live vulnerability gate. An older clean scan does not
cover subsequently published advisories. A dependency fix changes both server
and bundled CLI artifacts: rebuild them, repeat native acceptance, and inspect
the actual binaries with `go version -m` for the fixed module version. Distinguish
scanner symbol reachability from a demonstrated exploitable application path;
neither ambiguity nor earlier acceptance justifies bypassing the release gate.

## Good / base / bad cases

- Base: plain deployment installs the new main Compose and retains its secrets.
- Good: resource-publishing and local overlays are passed in order during upgrade
  and every later maintenance command.
- Bad: restarting with an old main Compose drops new mounts/configuration; adding
  a variable only to `.env` silently leaves it outside the container.

## Required tests

Run `scripts/offline-changelog.test.mjs`, `offline-build-changelog.test.mjs`
and `selfhost-config.test.sh`. Assert actual Compose environment values, overlay
mount resolution, special-character paths, backups, publication failure and
packaged guide bytes. Opt-in Docker smoke uses
`MULTICA_RUN_DOCKER_CHANGELOG_SMOKE=1 MULTICA_CHANGELOG_SMOKE_IMAGE=<loaded-web>`.
Release acceptance additionally upgrades a disposable older deployment and proves
retained data, credentials, mounts, migration health and image architecture.
Check the final distributed archive against the actual rehearsal directory,
including inner image and outer archive checksums and both source-guide copies.
For a multi-platform local database tag, assert the saved and running target
variants rather than weakening architecture checks after a host-default mismatch.

## Wrong versus correct

Wrong: edit `.env` and run `docker compose restart backend`.
Correct: verify Compose passes the variable, then run the recorded full Compose
prefix with `up -d --pull never --no-deps --force-recreate backend` and inspect
the appropriate authenticated capability/login flow as well as `/healthz`.
