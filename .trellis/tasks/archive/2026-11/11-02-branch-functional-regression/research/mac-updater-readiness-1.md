# macOS updater offline readiness

**Offline preparation passed; MAC-AUTOMATIC-UPDATER remains NOT_RUN. MAC-TRUST is BLOCKED and real authenticated migration remains NOT_RUN.** This is the native implement child's source/artifact/template self-check, separate from coordinator verification and independent review. No application, updater, HTTP feed, account, signing/notarization, service, container, database, mount or native protocol action occurred.

## Concrete prepared inputs

Candidate7af9ab9905c98c77bb091223ebe687c639475bb4 /0.6.8-rc.20261102 is unchanged. The feed template is a byte-identical copy of the known generated latest-mac.yml, not hand-edited metadata. Its two files retain their original names, SHA-512 and sizes:

| File | Bytes | SHA-256 |
| --- | ---: | --- |
| multica-desktop-0.6.8-rc.20261102-mac-arm64.zip | 223524090 | b50a00fa6fc44ca612a834aa8fd1c638f91431b87d5b6bb47abbd479433183d9 |
| multica-desktop-0.6.8-rc.20261102-mac-arm64.dmg | 233408426 | bd82fe6ddc6b65fce891f078d64326d91742d951f310b67589eb654bdaceabf6 |

The existing electron-updater6.8.3 parseUpdateInfo and repository updateReferences validated the copied metadata and exact current file fingerprints. Default stable-only validation rejects this RC; explicit allowPrerelease=true succeeds. No collection/publication call was executed. The template preserves the original builder releaseDate and is not evidence of a published release. Payload files remain at their original pinned source paths; they have not been copied into or served from a new feed. Associated existing blockmap identities are listed separately when present; their bytes are not referenced by this metadata, and no HTTP/differential-update proof is claimed.

Both exact ASARs contain updater6.8.3 and original real updater IPC. The fixed dirty baseline is0.6.5-185-g61ff581d2-dirty, ZIP807cf1adb72639dc696070de8c0e0a684386b5092daac1da48b8c2b56cc473a1. The candidate is numerically newer by the installed semver library. This establishes version ordering only, not an actual availability/download decision or universal0.6.5 compatibility. Baseline dirty source was not reconstructed.

Machine-readable preparation is evidence/next-updater-readiness-1/readiness-manifest.json. The adjacent runtime-config.template.json has explicit unresolved owned-endpoint placeholders, updater-preferences.template.json records the manual-update retained preference, and future-run.recipe.json describes the required protocol steps. None is a runnable Mac update harness or allocated environment.

## Actual product path and a new isolation boundary

- apps/desktop/src/main/updater.ts:17-18 sets autoDownload and autoInstallOnAppQuit true. Mac ARM64 retains latest-mac.yml; latest-arm64 is Windows ARM64 and latest-x64-mac is Mac x64. Installed GenericProvider.channel resolves the same Mac metadata filename.
- src/main/index.ts:1107 passes the current successful runtimeConfigResult.config.updateUrl to setupAutoUpdater. src/main/runtime-config-loader.ts:84 reads .multica/desktop.json under Electron home; current explicit updateUrl replaces the generated GitHub provider. Missing configuration blocks checks. The source parser derives a default update address, but this recipe requires an explicit owned loopback feed and does not invent business endpoints.
- updater.ts:269-301 persists and schedules the automaticUpdates preference. The separate updater:check handler at303 bypasses that scheduling preference, so an explicit manual check can auto-download with automaticUpdates=false. onUpdateDownloaded and onDownloadProgress in src/preload/index.ts:345-370 are the real public observer/check/install APIs.
- **Cache isolation needs more than MULTICA_PACKAGED_ACCEPTANCE_HOME.** src/main/packaged-acceptance-home.ts redirects Electron home and userData; electron-updater/out/ElectronAppAdapter.js:29 delegates baseCachePath to AppAdapter.getAppCacheDir, which uses Node os.homedir()/Library/Caches on Mac. AppUpdater.js:545 adds the actual app-update.yml updaterCacheDirName, @multicadesktop-updater. A future child must also have task-owned HOME and verify Node/Electron/cache paths before checking. No current user cache was read or changed. Previous runtime evidence had not triggered updater downloads, so this finding does not reclassify those results.
- MacUpdater.js:77 selects a ZIP, checks/downloads it and manages cached update.zip. With no prior cached ZIP the current product uses a full download. At118-224 it starts a separate loopback Squirrel.Mac proxy and emits the wrapper downloaded event before native preparation completes. Its native readiness/error and proxy listener must be tracked; a renderer event does not prove payload replacement or restart. updater:install calls quitAndInstall(false,true), with the MacUpdater native readiness branch owning the install.

## Required actual-run conditions

The future recipe must have an owned stable baseline install/profile, actual owned API/app/WS/feed endpoints, same authentication mode, complete source/artifact/signing compatibility identity, an audited Node cache HOME, and a Mac-specific native updater driver/manual journey. apps/desktop/scripts/windows-update-probe.mjs:64 expressly requires disposable GitHub-hosted Windows x64; it is not a Mac driver and must not be enabled by environment spoofing.

A future run should attach real preload/request/process observers before the check, require ok/available/expected-version, capture actual metadata/ZIP transport and checksum, then observe Squirrel native readiness/error, explicit install and an actual restarted candidate with exact ASAR/executable/CLI hashes. Retained config/preferences/synthetic markers, updater cache/native proxy/process cleanup and exact original multica protocol default/registration set are separate assertions. Keep framework retries0 and original product behavior. Do not substitute manual file copying, a changed file mtime or only an update-downloaded callback for actual installation.

Known artifacts are ad-hoc and no actual Squirrel.Mac signing compatibility has been exercised. Developer ID/notarization trust is still unproved; this report does not claim all ad-hoc updates necessarily fail. Real credentials/login migration requires its own owned approved account journey. Source instructions in docs/offline-upgrade.zh-CN.md and desktop-intranet-update-runbook.zh-CN.md preserve authentication mode and server/account data; client updateUrl cannot choose the backend auth mode.

## Verification and limits

The first recorder run completed successfully. Exact commands/times/exits/streams, copied validator and recorder, source/dependency hashes, ASAR versions/hashes, artifact SHA-256/SHA-512/size, original generated metadata and parser results are saved in evidence/next-updater-readiness-1. Private recorder AST, JSON parsing, template byte equivalence, actual offline parser/reference/channel/ZIP-selection checks, strict source/frozen identity and git diff --check passed. No completed Mac installation/runtime/trust test or P95/cache/statistics/E2E suite was repeated.

Full product lint/typecheck/tests were NOT_RERUN: only private preparation and task evidence changed; no product/dependency change. This child created no lasting process, listener, app/update, mount, protocol registration, container, DB or external account action. Coordinator owns the fresh batch-wide resource audit. Original QA-PERF-001 remains2171ms>2000msFAIL. No product refactor or simplification occurred.
