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
- CLI installer settings are public complete commands: preserve
  `MULTICA_CLI_INSTALL_COMMAND` and forward `_WINDOWS`, `_MACOS` and `_LINUX`
  variants through Compose, Helm and offline templates. New clients use the
  legacy value only for Unix and never fall back to GitHub for an unconfigured
  system when any internal source exists. Keep the source and embedded docs in
  sync; the full contract lives in
  [Add a computer](../views/frontend/connect-remote-computer.md).
- Compose defaults match backend semantics: `FF_PROJECTS_P1=true`, platform
  administration and managed installations off. Iterations have no deployment
  gate; they require an explicit workspace owner/admin action in settings.
  Existing enabled rows are preserved, including ones hidden by an old flag.
  Operator overlays may retain obsolete `FF_ITERATIONS_I1` values; preserve
  operator files while documenting that newer backends ignore that key.

### First-time platform administration

The operator procedure lives in
`docs/offline-upgrade.zh-CN.md#platform-administration`. Keep it in both upgrade
guide copies (`README.md` and `操作文档.md`) produced by
`scripts/build-offline-upgrade.sh`, and link it from the desktop delivery checklist.
Ordinary upgrades do not enable administration or change authentication mode.
After migration health succeeds, an eligible password/admin deployment invokes
the bundled `initialize-platform-admin.sh`; an existing platform role or retained
initialization history suppresses creation. Device-account deployments must plan
the existing password migration first. Signup need not be enabled.

The full console needs password mode, the administration/managed-installation
switches, and a stable deployment UUID. A successful administrator login alone
does not prove the console works: overview and installation reads return
`installation_unavailable` without `MULTICA_DEPLOYMENT_ID`. Generate it only when
absent; never reuse a development deployment's UUID or credentials in examples.
Recreate the backend with the original ordered Compose files after env changes.

Automatic initialization runs `/app/server platform-admin initialize
--credentials-file ABS_PATH --reason ...` in a one-off container from the selected
backend image. Preserve all overlay order, runtime database/network settings and
operator UID/GID. A temporary structured override adds the private deployment
bind; no permanent credential mount, host Node/Go dependency, port publishing or
image pull is needed. Never source `.env` or log full resolved Compose JSON.

The CLI must durably publish and sync `admin-initial-password.txt` (0600) and its
parent before committing account/credential/role/marker/audit. Reused valid files
must be synced again. Keep this file and `.admin-initial-password.*` staging
artifacts out of Git, build contexts and bundles. stdout carries only versioned,
nonsecret status JSON. A pending committed account without matching local
credentials returns `credentials_unavailable`, including a second maintenance
host; never claim delivered credentials or regenerate a password in that case.
Initialization failure prevents a whole-upgrade success report but does not undo
already upgraded services or migrations. The standalone helper is the retry path.

The explicit `platform-admin bootstrap --user UUID --reason ...` still grants an
existing ready account without setting its password and requires no effective
super administrator. Preserve its existing password-mode-only preconditions;
manual preservation markers may have no deployment UUID. Both paths retain a
permanent marker across role removal, account recovery and future upgrades.
Verify first-change restriction, rejection of an unchanged temporary password,
authenticated role and overview/account pages as well as migration health.

### Credential publication interrupted by process exit

**Scope and signatures.** `platformAdminCredentialFile.durable` and
`publishInitializationFile(directory, stagingName, finalName)` in `cmd/server`
must tolerate process termination immediately after the publication syscall.

**Contract.** Publish by an exclusive atomic rename against the pinned directory:
Linux `renameat2(RENAME_NOREPLACE)`, macOS `renameatx_np(RENAME_EXCL)`. Successful
publication consumes the staging name; do not unlink that name afterward.
Retain 0600, owner, no-follow and single-link validation, and sync both the final
file and directory before the database can commit. Unsupported operations fail
closed; there is no hardlink fallback.

**Validation and errors.** An existing final path must never be overwritten.
A published valid file is reusable with unchanged bytes after an interrupted
process; unsafe files still return `credentials_file_unsafe`. Existing unsafe
hardlinked files from older versions require operator inspection, not automatic
link deletion or weaker validation.

**Good/base/bad.** A crash after rename leaves one private final file that retry
can sync. Ordinary creation and retry preserve the same credential identity.
Linking then unlinking leaves a two-link inode when interrupted and is invalid.

**Required tests.** `TestPlatformAdminCredentialFileInterruptedPublication`
exits a child immediately after the real syscall, bypassing `TestMain` so the
child cannot seed or delete database fixtures. Assert unchanged retry bytes and
no staging name; retain no-clobber, unsafe-file and fsync-failure tests.

**Wrong versus correct.** Wrong: `Linkat(staging, final)` followed by
`Unlinkat(staging)`. Correct: exclusive rename, reopen/validate, then file and
directory fsync. A recoverable error test with deferred cleanup alone does not
prove recovery after process termination.

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
