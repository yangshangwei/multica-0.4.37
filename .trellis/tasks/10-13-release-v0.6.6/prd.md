# 制作 v0.6.6 内网升级包与 Windows x64 安装包

## Goal

Follow the previous v0.6.5 delivery procedure to provide a usable Linux amd64 offline server upgrade and Windows x64 desktop installer, both versioned 0.6.6, with Chinese operator instructions and verifiable provenance.

## Confirmed decisions

- The user approved creation of this Trellis task and a release plan.
- The user selected the latest committed main source. Freeze `4797e6d971296e56f3edc4f6cb6d3f218d9f8d85`, inspected on 2026-10-13; preserve concurrent uncommitted login and collaboration work.
- Previous local delivery lives at `dist/release/v0.6.5/`; deliver this version at `dist/release/v0.6.6/`.
- One task owns the coupled release: both platform artifacts must share one source decision, document set, version and final verification report. Independent build lanes do not need separate lifecycle tasks.

## Requirements

1. Deliver `multica-server-upgrade-v0.6.6-linux-amd64.tar.gz`, containing backend, Web and PostgreSQL images, offline upgrade helpers, Compose/environment templates, cumulative changelog, and Chinese upgrade instructions.
2. Deliver `multica-desktop-0.6.6-windows-x64.exe` with its original `.exe.blockmap` and `latest.yml`; the installed Electron payload and bundled CLI must both be Windows amd64 and report 0.6.6.
3. Include version-specific upgrade notes, an explicit environment change comparison against v0.6.5, authentication/account preservation and migration instructions, operational verification, rollback limits and SHA-256 checksums.
4. Keep release source and provenance frozen throughout parallel builds; record any documentation/build-only overlays and do not mix shared working-tree changes into the package.
5. Retain prior release artifacts intact. Keep source snapshots, logs, temporary deployment configuration and private rehearsal credentials outside the delivery directory.

## Acceptance criteria

- Both named artifacts exist and their hashes are listed in `SHA256SUMS-v0.6.6.txt` alongside all delivered documents and metadata.
- Server archive members are safe, contain only the expected root, exclude host metadata/private files, and contain images with Linux amd64 configs. Both packaged guide copies match the delivered source guide.
- A disposable v0.6.5-to-v0.6.6 upgrade passes `/healthz`, version, migration ledger, existing authentication/data retention, and administrator initialization/retry checks as applicable.
- Windows installer extraction proves x64 Electron/CLI, matching 0.6.6 version, expected source revision, intact ASAR, and no local environment/tool state. Feed metadata hashes/sizes match actual bytes; local HTTP checks verify full download and Range 206.
- Release-related tests, lint, TypeScript checks and Go static analysis are recorded against the frozen source. Any limits of native Windows or real intranet acceptance are explicit.
- Delivery is local; no GitHub release, tag publication or production deployment is implied by artifact completion.

## Out of scope

New features, incorporation of uncommitted source changes, production deployment, authentication-mode changes, unrelated dependency updates and public release publication.
