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

## Wrong versus correct

Wrong: edit `.env` and run `docker compose restart backend`.
Correct: verify Compose passes the variable, then run the recorded full Compose
prefix with `up -d --pull never --no-deps --force-recreate backend` and inspect
the appropriate authenticated capability/login flow as well as `/healthz`.
