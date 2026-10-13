# v0.6.6 release design

## Frozen inputs and ownership

Application source is main commit `4797e6d971296e56f3edc4f6cb6d3f218d9f8d85`. Build in isolated snapshots/worktrees; never package the shared dirty checkout. Use separate Linux and Windows source paths under `.artifacts/release-v0.6.6/` so build outputs and documentation overlays cannot race. The final directory contains deliverables only.

Linux lane reuses `scripts/build-offline-upgrade.sh` and `scripts/offline-bundle.sh` with explicit VERSION, COMMIT, platform and changelog. Windows lane reuses `apps/desktop/scripts/package.mjs` with `MULTICA_DESKTOP_VERSION=0.6.6`, Windows x64 target and publication disabled. Version overrides avoid changing tracked package versions or creating a release tag. The main session owns operator documents and final assembly; a separate checker audits artifacts.

## Compatibility and migration

Parent inspection found no migration-file changes from local v0.6.5 source `7ab8d038963dd920a611047997ac1904ee69a1eb` to selected main; the numeric migration ceiling remains 570. The v0.6.5 tag-to-main `.env.example`, main Compose and release packaging scripts have no diff. Compare the actually delivered v0.6.5 template too before documenting no configuration-key change.

Preserve deployment secrets, database, uploads, desktop identities and authentication mode. Upgrade server/Web first, verify health/login/data, then distribute Desktop. Include the new administrator credential publication interruption recovery behavior and exclusive-rename host requirement already described by the offline spec. Never declare automatic database rollback.

## Changelog and guide contracts

Carry prior published changelog records, add a truthful local unreleased preview only from committed Git objects, and embed identical validated feed bytes in backend and bundle. Do not claim local v0.6.6 artifacts are a published GitHub release. Review both server and desktop runbooks. Update the delivered generic guide and version-specific notes without silently changing the application source. Record document overlay hashes. Both `README.md` and `操作文档.md` in the server bundle must match the delivered generic guide.

## Verification and failure boundaries

Verify artifact bytes after final assembly. Rehearsal uses disposable Docker projects, isolated database and synthetic accounts; no production deployment or user-installed AI CLI access. Docker Desktop amd64 emulation is recorded honestly. A macOS cross-build can prove Windows artifact structure and feed transport, but native GUI installation/upgrade remains a distinct check. Record signature/trust status rather than infer it from checksums.

If a build needs extra V8 memory, confine it to a build-only Dockerfile overlay, hash the overlay and verify the runtime has no added NODE_OPTIONS. Preserve failed-build evidence outside delivery. Final metadata must refer to final immutable executable bytes.
