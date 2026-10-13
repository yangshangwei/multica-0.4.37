# Research: v0.6.6 offline server release

- Query: Reuse the previous local intranet release process to build a Linux amd64 upgrade bundle from frozen main, identify configuration/migration changes, and define isolated acceptance and minimum documentation updates.
- Scope: mixed — repository and retained v0.6.5 evidence, parent-supplied committed-source comparisons, official Docker documentation and Linux man-pages.
- Date: 2026-10-13
- Ownership: research only; this file is the sole write. No builds, tests, services, Git operations, or deployment-secret reads were performed.

## Findings

### Source and baseline

- Approved target, confirmed by parent: `4797e6d971296e56f3edc4f6cb6d3f218d9f8d85` on committed `main`. Shared working-tree content is dirty; build from an isolated frozen source tree.
- Previous delivered v0.6.5 binaries used `7ab8d038963dd920a611047997ac1904ee69a1eb`, not the later v0.6.5 tag commit `24771b0d803f2063e432a38cfe2bf72c9f107a92`. The tag added release documentation, an ENV comment, desktop packaging allowlist and test fixes. This distinction is explicit in `dist/release/v0.6.5/verification-v0.6.5.json` and `README-v0.6.5.zh-CN.md:3`.
- Parent's Git comparison reports **no changes** from v0.6.5 tag to frozen main in `.env.example`, `docker-compose.selfhost.yml`, `scripts/build-offline-upgrade.sh`, `scripts/offline-bundle.sh`, or `apps/desktop/electron-builder.yml`.
- Parent's comparison of `server/migrations/` from local v0.6.5 build source to frozen main is empty. **No new database migration; maximum numeric migration remains 570.** Previous verification has **594 ledger rows**, because numeric maximum is not the row count.
- Relative to the older local build source, ENV differences are comments, not keys/default values. Do not repeat v0.6.5's “three added variables, one removed” as the v0.6.6 delta.
- Parent identifies post-v0.6.5 work including administrator credential publication recovery, early editor-draft preservation, numeric workspace task references, device/password-binding connectivity, Desktop recovery, MCP agent-category filtering and CSV encoding. Product release notes should describe these from the frozen comparison, with detailed Desktop review owned by the other lane.

### Files found and purpose

| File | Purpose |
| --- | --- |
| `CLAUDE.md` | Package rules, mandatory offline authentication documentation and release checks. |
| `.trellis/workflow.md` | Research persistence and research-only role boundaries. |
| `.trellis/spec/server/offline-delivery.md` | Overlay preservation, initialization, archive/architecture proof and release acceptance contract. |
| `scripts/build-offline-upgrade.sh` | Outer versioned upgrade archive; invokes runtime bundler and copies both operator guides. |
| `scripts/offline-bundle.sh` | Builds backend/Web images, pulls target PostgreSQL image, saves image archive, stages feed/templates/helpers. |
| `docker-compose.selfhost.build.yml` | Build arguments and versioned image names. |
| `Dockerfile`, `Dockerfile.web`, `.dockerignore` | Container recipes, source exclusions and version injection. |
| `scripts/offline-upgrade.sh` | Validates package/overlays, backs up PostgreSQL/configuration, publishes feed, replaces Compose, recreates and verifies backend. |
| `scripts/install-changelog.sh`, `scripts/initialize-platform-admin.sh` | Target-side publication and optional first administrator initialization. |
| `scripts/admin-init-compose.mjs` | Structured private initialization-container override and sanitized receipt validation. |
| `docker/entrypoint.sh` | Runs migrations before starting the API. |
| `docs/offline-upgrade.zh-CN.md` | Source copied to both package guides; currently still labels release scope V0.6.5. |
| `docs/desktop-intranet-update-runbook.zh-CN.md` | Desktop release checklist, authentication guidance, download/installation verification boundary. |
| `dist/release/v0.6.5/verification-v0.6.5.json` | Previous artifact, build-source, rehearsal, migration matrix and test evidence. |
| `dist/release/v0.6.5/README-v0.6.5.zh-CN.md`, `ENV-CHANGES-v0.6.5.zh-CN.md` | Reusable delivery-document structure, but version and comparison baseline must change. |
| `server/cmd/server/platform_admin_credentials*.go` | Atomic publication, file validation and interrupted-process regression. |
| `scripts/generate-changelog.mjs` | Generates a cumulative local preview from a chosen commit and prior history. |

### Exact reusable build entry point

Run **inside the isolated source tree**, after preparing a cumulative v0.6.6 preview and delivery-guide edits:

```bash
COPYFILE_DISABLE=1 \
VERSION=v0.6.6 \
COMMIT=4797e6d971296e56f3edc4f6cb6d3f218d9f8d85 \
bash scripts/build-offline-upgrade.sh \
  --platform linux/amd64 \
  --output /absolute/path/dist/release/v0.6.6/_build/server \
  --changelog /absolute/path/dist/release/v0.6.6/changelog.json
```

Expected result: `multica-server-upgrade-v0.6.6-linux-amd64/`, its `.tar.gz`, and `.tar.gz.sha256`. Copy only final delivery artifacts to the release root.

- Wrapper options are only `--output`, `--platform`, `--changelog` (`scripts/build-offline-upgrade.sh:18`). It has no `--dry-run` or `--skip-build` option.
- Nonbuilding plan: same `VERSION`/`COMMIT`, `bash scripts/offline-bundle.sh --platform linux/amd64 --output ... --changelog ... --dry-run`. It still requires functional Compose configuration resolution but exits before staging/build/pull (`scripts/offline-bundle.sh:96`, `:139`).
- The direct wrapper avoids Makefile parsing of a checkout's local environment. Make's equivalent is `make offline-upgrade-bundle OUT=... PLATFORM=linux/amd64`, but its recipe does not forward a `--changelog` argument; use the direct script or explicitly supply `CHANGELOG_ARTIFACT` (`Makefile:164`).
- `VERSION` stamps both image tags and binaries; `COMMIT` preserves provenance for exported trees lacking `.git` (`scripts/offline-bundle.sh:81`, `:144`; `docker-compose.selfhost.build.yml:6`). The wrapper's informational commit log still uses Git and may say `unknown` in an archive tree; the manifest/build argument is the authoritative explicitly supplied commit.
- `DATE` is recomputed by the bundler, so a rebuilt CLI may have different bytes. Refresh all final hashes/module inspection after the last build (`scripts/offline-bundle.sh:145`).
- Reusing the same package output removes/replaces its directory after private-file checks (`scripts/build-offline-upgrade.sh:62`). Keep candidate/rehearsal directories separate and do not point output at an installed deployment.

### Build and target prerequisites

- Build machine: Docker daemon, Compose and buildx plugins, target-platform emulation or a native amd64 builder, Bash, Node 22, gzip/tar and SHA-256 tooling; registry/network access for Go modules, pnpm packages and base/PostgreSQL images. Host Go/pnpm are needed for their native checks, but image builds provide their own toolchains.
- Docker must support `docker save --platform`; Docker documents API **1.48+** for this flag. Removing the flag would weaken architecture correctness (`scripts/offline-bundle.sh:188`).
- Recipes use `golang:1.26-alpine`, `alpine:3.21`, `node:22-alpine`, and `pgvector/pgvector:pg17`. Source `server/go.mod` declares Go `1.26.6`; `package.json` selects `pnpm@10.28.2`; the exact actual pulled base IDs/toolchain must be recorded. These tags are mutable.
- Source currently retains `golang.org/x/text v0.41.0`; previous delivered binaries were inspected for that fix. Inspect the actual final server/CLI with `go version -m` again rather than copying old evidence.
- Build interpolation uses a dummy JWT value; it is a runtime configuration value, not a build argument (`scripts/offline-bundle.sh:174`). Use a clean source tree with no real `.env`; never copy production credentials into it.
- Previous amd64 Web build under macOS ARM emulation exhausted V8 heap. If reproduced, confine the adjustment to the isolated recipe's build RUN: `RUN NODE_OPTIONS=--max-old-space-size=4096 pnpm --filter @multica/web build`. Record the recipe's hash/diff and verify final runtime environment has no added `NODE_OPTIONS`; the repository recipe currently has plain `RUN pnpm --filter @multica/web build` at `Dockerfile.web:50`.
- Target: Linux amd64, Docker/Compose, Bash, curl and SHA-256 tooling plus sufficient space for archives, imported images and database backup. No target Go/Node/pnpm or registry access is needed. PostgreSQL backup uses `pg_dump` inside the existing Compose `postgres` service; external database deployments must verify that this really backs up the backend's database (`scripts/offline-upgrade.sh:199`).
- New target requirement for first administrator publication: Linux `renameat2(RENAME_NOREPLACE)` **and underlying filesystem support**. Kernel 3.15 alone is insufficient for every filesystem: ext4 support begins at 3.15; btrfs/tmpfs/cifs 3.17; xfs 4.0. Unsupported publication fails closed with `credentials_file_publish_failed`. This matters to the deployment-directory bind mount, not merely the image filesystem.

### Safe local Docker configuration

Use an isolated client configuration with no copied login credentials/credential-helper settings. This isolates Docker client state, **not the daemon's images/containers/volumes**, so the rehearsal additionally needs its own project and data.

On this machine, these paths were checked for existence without reading credentials: `/usr/local/bin/docker`, `/Applications/Docker.app/Contents/Resources/cli-plugins`, `/Users/artisan/.docker/run/docker.sock`, `/var/run/docker.sock`. Recheck them at execution time.

Execution setup example for the implementer:

```bash
RELEASE_DOCKER_CONFIG=/absolute/path/dist/release/v0.6.6/_build/docker-config
mkdir -p "$RELEASE_DOCKER_CONFIG/cli-plugins"
printf '{"auths":{}}\n' > "$RELEASE_DOCKER_CONFIG/config.json"
ln -s /Applications/Docker.app/Contents/Resources/cli-plugins/docker-compose \
  "$RELEASE_DOCKER_CONFIG/cli-plugins/docker-compose"
ln -s /Applications/Docker.app/Contents/Resources/cli-plugins/docker-buildx \
  "$RELEASE_DOCKER_CONFIG/cli-plugins/docker-buildx"
```

Prefix build, image-inspection and rehearsal calls with:

```bash
env -u DOCKER_CONTEXT -u DOCKER_TLS -u DOCKER_TLS_VERIFY -u DOCKER_CERT_PATH \
  DOCKER_CONFIG="$RELEASE_DOCKER_CONFIG" \
  DOCKER_HOST=unix:///Users/artisan/.docker/run/docker.sock \
  docker compose version
```

Replace the final command with the build `bash ...` command so its child Docker calls inherit the same settings. `DOCKER_CONTEXT` overrides `DOCKER_HOST`, hence explicitly remove it. On another machine resolve and confirm its intended local endpoint first; do not transplant this macOS socket path. Do not inspect/copy the real `~/.docker/config.json`, perform logins, or change the user's default context. Avoid global prune/cleanup.

### Configuration and migration implications

1. **v0.6.5 → v0.6.6:** no new/removed ENV keys, default changes, Compose forwarding changes or migration files in the parent-confirmed frozen comparison. Preserve existing `.env`, authentication mode, UUID, keys, addresses, overlays and volumes.
2. **Earlier deployments:** v0.6.0/v0.6.1 still need migrations 567–570 and the CLI-install/iteration changes already documented in v0.6.5. Mark these historical catch-up requirements, not v0.6.6 additions.
3. The upgrade writes only selected image/feed settings: `MULTICA_BACKEND_IMAGE`, `MULTICA_WEB_IMAGE`, `MULTICA_IMAGE_TAG`, `CHANGELOG_FILE`, `CHANGELOG_DIRECTORY`; it replaces main Compose and preserves explicitly listed overlays (`scripts/offline-upgrade.sh:30`, `:203`, `:225`).
4. Repeated `--compose-file` order must be identical for upgrade, initialization and later maintenance. A recorded `compose-command.txt` does not automatically reload overlays. Literal stale `image:` values are rejected before backup/deployment changes, but after importing images (`scripts/offline-upgrade.sh:155`, `:170`).
5. Ordinary upgrades do not enable administration or change device/password mode. Existing administrators/initialization records suppress creating another account. Preserve existing credentials rather than “resetting” them.
6. Credential publication now consumes the staging path in one exclusive rename, revalidates the final private single-link inode and syncs file/directory before DB commit (`platform_admin_credentials.go:119`; `platform_admin_credentials_publish_linux.go:10`). A crash after rename can reuse unchanged bytes; preexisting unsafe hardlinks need operator inspection, not automatic weakening/deletion.
7. Current operator guide includes the device-account password-binding recovery route: Settings → daemon → reconnect this device, refreshing runtime credentials after binding (`docs/offline-upgrade.zh-CN.md:254`). Preserve this improvement in delivery copies.
8. `/healthz` checks database and migration readiness. `/health` only proves process/version. A conditional migration can be recorded even if its SQL was skipped. Old upstream/fork migration-number collisions are not detected by readiness alone; previous evidence explicitly excludes genuine upstream >=v0.4.38 databases from blanket safe-upgrade claims.

### Retained tests and reusable verification

Run these in the isolated frozen tree and retain their logs; they were researched, not executed here:

```bash
node --test \
  scripts/offline-changelog.test.mjs \
  scripts/offline-build-changelog.test.mjs \
  scripts/admin-init-compose.test.mjs \
  scripts/release-changelog.test.mjs \
  scripts/install-changelog.test.mjs
bash scripts/selfhost-config.test.sh
bash scripts/entrypoint.test.sh
bash scripts/go-test-with-agent-cli-guard.sh \
  go -C server test -count=1 -v ./internal/changelog
bash scripts/go-test-with-agent-cli-guard.sh \
  go -C server test -count=1 -v ./internal/auth \
  -run '^TestPassword(Config|LimiterAndProxy)$'
go -C server vet ./cmd/server ./internal/changelog ./internal/auth
```

- `offline-changelog.test.mjs:152`, `:214`, `:249`, `:262`, `:295`, `:454` cover complete packaged guides/tools, ordered overlays/backups, missing paths, pinned images, non-atomic failure boundaries and initialization after health.
- `offline-build-changelog.test.mjs:172`, `:228`, `:254` cover selected image/version identity, wrapper version freezing and private-file exclusion. Compose-dependent cases may skip if plugin unavailable; check actual test/skipped counts.
- Docker-backed publication smoke after the final Web image is loaded:

```bash
MULTICA_RUN_DOCKER_CHANGELOG_SMOKE=1 \
MULTICA_CHANGELOG_SMOKE_IMAGE=multica-web:v0.6.6 \
node --test scripts/offline-changelog.test.mjs
```

It tests publication/UID/escaping with the real frontend image, not a complete database upgrade. The fixture lives under source `.changelog-build` because macOS per-user temporary directories may not be shared with Docker (`offline-changelog.test.mjs:556`).

- New regression gate: run `TestPlatformAdminCredentialFile*`, especially `TestPlatformAdminCredentialFileInterruptedPublication`; retain no-clobber, unsafe files and fsync tests. `server/cmd/server/integration_test.go:43` executes `TestMain` even with a narrow `-run`, defaults to `localhost:5432/multica`, mutates fixtures if connected, and returns exit 0 without running tests if unreachable. Supply an explicitly dedicated **migrated test database**, use the agent-CLI guard, and assert named tests actually ran. Never aim this at an existing developer/deployment database.
- A filesystem-only alternative can pass exactly `platform_admin_credentials.go`, `platform_admin_credentials_unix.go`, the matching platform publication file, `platform_admin_credentials_test.go` and `platform_admin_credentials_process_test.go` to `go test`; this excludes `integration_test.go`. This invocation is an unexecuted recommendation; it should be checked by the implementer. It does not replace Linux image initialization acceptance.
- Previous release's clean-source checks also included `pnpm lint --force`, `pnpm typecheck --force` and Desktop packaging tests. Apply the root's chosen current release gate; previous evidence did not include all Go DB-backed tests, full `pnpm test`, or Playwright.
- Previous `_build/verification` scripts, source snapshots, logs and harness were **removed after delivery**, explicitly recorded in `verification-v0.6.5.json.build_workspace`; historical paths cannot be run now.
- Retained `dist/release/new-install-image/.../verify.sh` hardcodes `multica-web:v0.6.5` and its `check-endpoints.mjs` assumes a password/admin-enabled **new installation** and download endpoints. Reuse readiness assertion ideas only; do not run it unchanged against an ordinary v0.6.6 upgrade.
- `dist/release/v0.5.3/_build/verify-candidate.mjs` is Desktop-specific and hardcodes old source/output paths; it is not a server-upgrade harness.

### Disposable v0.6.5 → v0.6.6 rehearsal

Use the actual distributed v0.6.5 archive as baseline, and the actual **final** v0.6.6 archive as upgrade input. Rehearsal logic needs to be recreated; do not infer a pass from old evidence.

1. Validate archive hashes and members before extraction. Require exactly the expected root; reject traversal, unexpected roots, links/devices, AppleDouble `._*` and private deployment files. Extract into new distinct baseline-package, target-package and deployment directories outside both source/output trees.
2. Choose a unique `COMPOSE_PROJECT_NAME` (e.g. `multica-v066-rehearsal-<run>`), set it for **every** Compose call and the upgrade script. Main Compose has `name: multica` (`docker-compose.selfhost.yml:33`); merely selecting a new directory is insufficient isolation. Use fresh project-owned volumes and loopback-only free backend/frontend ports through `BACKEND_PORT`/`FRONTEND_PORT`; explicitly set `platform: linux/amd64` for all three services in a rehearsal overlay.
3. Use synthetic local-only credentials, UUIDs and data. Keep optional integrations/provider keys empty; do not load a real `.env`. Keep deployment/credential directories owned by the invoking UID, without group/world write access. Include local Skill/MCP files and a custom ordered overlay (and resource-publishing overlay only if exercising that optional case) to prove retention.
4. Load baseline images and start baseline Compose with `--pull never`. Establish `/healthz`, Web login, baseline version, complete migration ledger/index validity, volume IDs and mounts. Create a test account/workspace/issue and attachment sentinel using current API/fixture patterns; save sanitized IDs/counts and hashes, not passwords/tokens.
5. Primary case: password mode with an existing administrator/initialization marker and a saved workspace setting. Record data/session usability and protected-file hash before upgrade. A separate legacy/device-mode case must keep that mode and skip administrator initialization. To prove the new publication path, a third eligible password/admin-enabled fixture without existing admin roles/marker can exercise first initialization; this should not change the primary case's expected configuration.
6. Run the bundled script, keeping project/platform/config environment and overlay order:

```bash
COMPOSE_PROJECT_NAME="$REHEARSAL_PROJECT" \
DOCKER_DEFAULT_PLATFORM=linux/amd64 \
bash "$TARGET_PACKAGE/offline-upgrade.sh" \
  --deployment-dir "$REHEARSAL_DEPLOYMENT" \
  --backup-dir "$REHEARSAL_BACKUP" \
  --compose-file docker-compose.rehearsal.yml \
  --yes
```

Add every other selected overlay before/after the rehearsal overlay consistently. On macOS ARM the unmodified host script checks `uname -m` and rejects the amd64 package; previous release ran it via `/usr/bin/arch -x86_64 /bin/bash` under Rosetta while Docker ran amd64 containers. Reuse that approach only if Rosetta is present and record emulation as a limitation; do not remove the architecture guard. A native amd64 Linux host is better proof.

7. Verify `/healthz` database/migrations OK, `/health` version `v0.6.6`, Web login page, original-account identity/data/session behavior, volume IDs/attachment bytes, template mounts/bytes and overlay bytes. Compare the full ledger to baseline: v0.6.5→v0.6.6 should add zero migrations and retain the recorded set (previous baseline 594 rows; numeric maximum 570). Verify no invalid indexes.
8. Verify `.env` backup and main-Compose backup, unchanged auth/key/UUID/custom values using private in-process comparisons, correct new image/feed keys, byte-identical installed main Compose, exact ordered overlays in recorded command, and bundled feed bytes/API feed. Later ordinary Compose resolution must still select v0.6.6.
9. Existing administrator fixture: no credential reset/new account; repeated initializer preserves expected state. New initialization fixture: sanitized `initialized` receipt, 0600 single-link private file, first-change restriction, unchanged-password rejection, successful distinct password change, `admin/me=super_admin`, overview/users success and invalid old temporary password. Keep all secret values inside the harness; write only assertions/nonsecret receipts. Retry must not change credential identity/bytes.
10. Match saved image configuration `os=linux`, `architecture=amd64`, `rootfs.diff_ids` and runtime config to target-platform inspection and running containers. Inspect PostgreSQL with `docker image inspect --platform linux/amd64`; default inspection on ARM may select the wrong variant. Container `ImageManifestDescriptor` plus `uname -m` proves runtime platform; an OCI index ID is not a config digest.
11. Extract/inspect the **final** server and CLI binaries without running their normal entrypoints; record hashes, `file`, `go version -m`, and CLI `version --output json` with expected version/commit/platform. Compare with running-container binaries. Revalidate guide/feed/source-copy hashes against the final archive after any repack.
12. Clean up only the explicitly identified disposable project and its project-owned volumes after evidence is saved. Never reuse the ordinary `multica` project for rehearsal; no global prune or unqualified `down -v`.

The package script's own success waits for backend readiness and initializer, but does not prove Web/business-state/Windows acceptance (`offline-upgrade.sh:241`). Upgrade/DB/feed/config actions are not atomic; scripts retain evidence and do not automatically roll back (`offline-upgrade.sh:39`, `:264`).

### Changelog and minimum documentation changes

- Update `docs/offline-upgrade.zh-CN.md:19` scope to v0.6.6/0.6.6, add the direct v0.6.5 baseline row with **no migration or ENV delta**, and retain explicit historical catch-up rows. Keep the entire authentication/migration/admin sections and the new reconnect guidance. Add the exclusive-rename filesystem prerequisite and recovery error to the first-admin section.
- Produce `README-v0.6.6.zh-CN.md`, `ENV-CHANGES-v0.6.6.zh-CN.md`, `CHANGELOG-v0.6.6.md`, cumulative `changelog.json`, verification JSON, final checksums and the docs ZIP matching the prior delivery layout. Record frozen source, build overlays, actual validation, unsigned/native-Windows limitations and local publication status accurately. Do not copy v0.6.5's published-GitHub wording into an unpublished local release.
- ENV document should lead with zero new keys/removals/default changes against v0.6.5. Reuse preservation/merge instructions, plus historical notes for upgrades from <=v0.6.1. Do not add administrator username/password ENV settings.
- Builder copies the source guide exactly to `README.md` and `操作文档.md` (`build-offline-upgrade.sh:75`). It does **not** automatically include version-specific README/ENV documents. If matching the previous archive layout, add those documents to staging deliberately, then repack and recompute the outer checksum. Verify both generic copies against the approved guide bytes after final packaging.
- Always pass a fresh cumulative feed: current repository seed predates this release (latest recorded stable v0.4.37), and `dist/release/v0.6.5/changelog.json` predates v0.6.5 publication (Unreleased at `7ab8d...`, latest stable v0.6.1). Retrieve/validate the actual published v0.6.5 cumulative history read-only before creating the new preview; do not silently erase published history.
- Local preview generation contract (`generate-changelog.mjs:70`, `:128`): `--ref 4797e6d971296e56f3edc4f6cb6d3f218d9f8d85 --base 24771b0d803f2063e432a38cfe2bf72c9f107a92 --history <validated-v065-history.json> --repository yangshangwei/multica-0.4.37 --version Unreleased --status unreleased --output <v066/changelog.json> --markdown <v066/CHANGELOG-v0.6.6.md>`. Generate where immutable Git objects are available before exporting build source. The actual app version is still v0.6.6; explain the local feed's Unreleased label.
- `release-changelog.mjs prepare/publish` targets an exact remote tag and uses GitHub APIs; it is not the local preview workflow and no external publication is implied by this task.
- Avoid unnecessary packaging-script changes: parent confirms current reusable scripts match v0.6.5. Any build-only recipe/doc overlay must be disclosed rather than represented as a byte-identical checkout.

### Related specs

- `.trellis/spec/server/offline-delivery.md`: full release contract, including credential publication recovery, macOS archive metadata, image-platform identity, Docker smoke and retained-data rehearsal.
- `.trellis/spec/server/index.md`: server spec entry point.
- `CLAUDE.md`: preserve authentication documentation in both packaged guides; use frozen source and current verification evidence.
- `.trellis/spec/views/frontend/connect-remote-computer.md` is referenced by the offline spec for CLI-install-command semantics; no UI implementation is proposed by this research.

### External references consulted

- Docker CLI configuration/environment, accessed 2026-10-13: <https://docs.docker.com/reference/cli/docker/>. Confirms `DOCKER_CONFIG`, `DOCKER_HOST`, and `DOCKER_CONTEXT` precedence.
- Docker image-save reference, accessed 2026-10-13: <https://docs.docker.com/reference/cli/docker/image/save/>. Confirms `--platform` requires API 1.48+ and exports selected platform content.
- Linux `rename(2)` man-page, accessed 2026-10-13: <https://man7.org/linux/man-pages/man2/rename.2.html>. Confirms exclusive no-replace behavior and filesystem-specific support versions.

## Caveats / Not Found

- Commit comparisons are supplied by the parent; this research role prohibits Git operations. Working-tree code references were read directly, so final execution must verify them against the approved frozen source and record any intentional overlays.
- No v0.6.6 artifact was built, no service started and no tests executed here. Commands above are a researched execution handoff, not completion evidence.
- The v0.6.5 rehearsal harness/log paths are historical and absent. Retained JSON is useful as an acceptance checklist, not a runnable harness.
- Linux ARM-host emulation cannot prove native Linux hardware/kernel compatibility or native Windows installer/profile behavior. Previous release declared these limits explicitly.
- Prior direct-jump migration evidence covers empty-schema equality for several older fork builds; it is not seeded-data acceptance for every historical migration or compatibility with genuine upstream images sharing migration numbers.
- Exact actual baseline/final OCI IDs, package hashes, pulled base versions, free port choices and target filesystem capability remain execution-time facts.
