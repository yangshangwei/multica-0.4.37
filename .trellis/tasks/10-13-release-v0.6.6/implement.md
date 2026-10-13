# Execution plan

- [x] Obtain task creation consent and release-source decision.
- [x] Freeze latest committed main as `4797e6d971296e56f3edc4f6cb6d3f218d9f8d85`.
- [x] Complete bounded server and desktop release research; review this plan before activation.
- [x] Create isolated build sources and save provenance, tool versions and clean status.
- [x] Prepare v0.6.6 operator guide, ENV comparison and cumulative local-preview changelog; validate feed.
- [x] Build Linux amd64 server bundle using `COPYFILE_DISABLE=1 VERSION=v0.6.6 COMMIT=<full SHA> bash scripts/build-offline-upgrade.sh --platform linux/amd64 --output <staging> --changelog <prepared JSON>`.
- [x] In independent clean Windows source, install locked dependencies then run `CSC_IDENTITY_AUTO_DISCOVERY=false MULTICA_DESKTOP_VERSION=0.6.6 pnpm --filter @multica/desktop package -- --win nsis --x64 --publish never` with only documented build-tool overrides if needed.
- [x] Run release Node regressions, selfhost configuration checks, desktop packaging/connection regressions, lint, typecheck and focused Go tests/vet against frozen inputs.
- [x] Extract final server bundle, compare source files/feed/guides, inspect saved image configs and binaries, and perform disposable v0.6.5 upgrade with retained synthetic data/auth/volumes.
- [x] Extract Windows installer, verify PE architecture, CLI build metadata and ASAR integrity; collect original artifacts via update-artifacts helper and verify a temporary loopback HTTP feed including Range 206.
- [x] Assemble deliverables and docs ZIP at `dist/release/v0.6.6/`; generate checksums only after bytes are final.
- [x] Dispatch independent Trellis artifact check; fix release-owned findings and regenerate affected evidence/checksums.
- [x] Record verification and explicit acceptance limits; verify all delivery checksums, update task records, and hand off exact paths.

## Boundaries and rollback

No feature edits or shared-branch reset/checkout. Restrict release-owned tracked changes to task artifacts and `docs/releases/v0.6.6-upgrade.zh-CN.md` plus necessary generic guide corrections. Build snapshots, fixture scripts and private rehearsal files stay in `.artifacts/release-v0.6.6/`; no secrets in reports. Do not overwrite v0.6.5 or publish remote refs/assets. Remove only task-owned disposable runtime resources after recording evidence.

## Expected validation commands

`node --test scripts/offline-changelog.test.mjs scripts/offline-build-changelog.test.mjs scripts/admin-init-compose.test.mjs scripts/desktop-updates.test.mjs scripts/windows-candidate-version.test.mjs scripts/release-changelog.test.mjs scripts/install-changelog.test.mjs`; `bash scripts/selfhost-config.test.sh`; desktop Vitest packaging and source-delta suites; `pnpm lint --force`; `pnpm typecheck --force`; guarded focused Go auth/admin/changelog tests; `go -C server vet` for affected packages. Persist outputs and exit codes; no broad repeated tests after checks pass without new evidence.
