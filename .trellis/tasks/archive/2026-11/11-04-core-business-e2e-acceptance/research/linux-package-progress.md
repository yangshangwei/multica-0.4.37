# Linux arm64 package preparation paused

Coordinator requested a quiet serial RK1 runtime acceptance window and release of this agent slot. No heavy packaging step has started.

- Fresh detached worktree: `/Volumes/artisan/code/2026/mc-core-e2e-linux-package-20261104`, clean candidate `1a32ca36c2c4f4455a01bdd39d0e0ab4db20cccc`.
- Private phase runner: `.omx/private/core-e2e-20261104/linux-package/build-phase.py`; syntax checked only. Phases are `install`, `inputs`, `cli`, `dir`, `deb`. Each writes fresh numbered raw logs/JSON and refuses overwriting prior runs.
- `install`: existing locked dependencies only, pnpm offline frozen.
- `inputs`: verifies source commit and relevant clean product paths, copies `apps/desktop/out` from main isolated checkout, compares every copied hash before/after, extracts cached Electron 39.8.7 Linux arm64 ZIP for explicit electronDist.
- `cli`: static Linux arm64 CLI with version `0.6.7-476-g1a32ca36c`, GOMAXPROCS=2 and Go -p2; writes only packaging worktree resources/bin.
- `dir`: electron-builder --linux dir --arm64 --publish never with extraMetadata.version, explicit cached electronDist and separate private output.
- `deb`: prepackaged directory to fresh DEB output; verifies entire original directory payload unchanged.

After build, still required: inspect packaged metadata/ASAR and verify ELF/CLI/deb payload hashes, record package version/architecture, produce directory and DEB environment snippets for installed-main E2E. No app/daemon/installer/DB execution is authorized for this packaging lane.

Evidence already saved in `evidence/linux-package/01-worktree.*`, `preflight.json`, `images.json`. Both cached acceptance images are linux/arm64; no container was launched.
