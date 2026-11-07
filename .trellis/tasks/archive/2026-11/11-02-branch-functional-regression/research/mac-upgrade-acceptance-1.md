# macOS ARM64 upgrade subgates — 2026-11-02

**Distribution inspection and manual file-install upgrade PASS. Actual old-to-new Main restart and runtime-retained-state acceptance remain open.**

Candidate product remains 7af9ab9905c98c77bb091223ebe687c639475bb4. The task copied immutable ZIP distributions into a new private application installation and never modified /Applications, a user installation, either source ZIP or either ASAR. No application was launched.

## Exact inputs

| Input | Actual bundle version | ZIP SHA-256 |
| --- | --- | --- |
| Historical local ARM64 ZIP | 0.6.5-185-g61ff581d2-dirty | 807cf1adb72639dc696070de8c0e0a684386b5092daac1da48b8c2b56cc473a1 |
| Current candidate ARM64 ZIP | 0.6.8-rc.20261102 | b50a00fa6fc44ca612a834aa8fd1c638f91431b87d5b6bb47abbd479433183d9 |

Both distributions identify ai.multica.desktop and contain an ARM64 main executable. Candidate version is numerically greater. Both pass ZIP integrity and local deep/strict code-signature structure verification; both signatures are ad-hoc with no TeamIdentifier. Signature-structure success is not Developer ID, Gatekeeper or notarization acceptance.

The historical artifact is explicitly dirty. Its actual version and bytes are verified, but its full source diff and reproducible build provenance were not reconstructed. The result applies to this exact baseline ZIP, not every 0.6.5 installation.

Baseline ASAR SHA-256: da566a4d55d35948fdf8150251e5980bbfcc6ffd0845f659f8f02737fe4201c6.

Candidate ASAR SHA-256: 128ff6f87dc18b78355ee5161171a86ec7828b18d5b87751d0f8142dd81b8485. Candidate main executable, ASAR and bundled CLI matched research/reusable-package-pins.json. Candidate ZIP provenance was already tied to packaging source 639bf6ef8 with product Git trees equal to the fixed candidate; current extraction matched those pins.

## Actual manual installation

Commands, times, exit codes and stdout/stderr hashes are in evidence/next-mac-upgrade-1/run.json.

1. Validate each ZIP and extract with ditto into separate new task staging directories.
2. Copy the historical app to the stable task-owned installed/Multica.app path and compare its full file/symlink/mode/hash manifest with the extracted distribution.
3. Retain that installed baseline as retained-baseline.app, then copy the candidate to the same installed/Multica.app path.
4. Compare the candidate's entire manifest and actual Info.plist version; verify the installed candidate signature structure.
5. Verify byte/permission/ownership retention for five synthetic profile files and retention of the baseline application copy. Recheck both source ZIP hashes.

All **24 assertions passed** and all recorded commands exited 0. Raw source manifests, profile manifests and command logs are preserved.

Installed candidate:

/Volumes/artisan/code/2026/multica-0.4.37/.omx/private/branch-functional-regression-20261102/next-mac-upgrade-1/installed/Multica.app

The profile contains synthetic runtime server/update addresses, daemon autoStart=false, automaticUpdates=false, a clearly synthetic device identity and state/authentication-mode marker. It contains no real credentials. Because neither application ran, this proves filesystem retention only; it does not prove either release read those files, preserved a real login, migrated a device identity or rendered a retained draft.

## Runtime blocker found before launch

Both actual bundled Main programs support MULTICA_PACKAGED_ACCEPTANCE_HOME and redirect Electron home/userData before their single-instance lock. That addresses profile filesystem isolation.

Both bundled Main programs then call setAsDefaultProtocolClient unconditionally, outside an acceptance-mode guard, immediately before requestSingleInstanceLock:

- Historical bundled Main: protocol call at line 5444.
- Candidate bundled Main: protocol call at line 5482.

The exact snippets and bundled Main SHA-256 values are in evidence/next-mac-upgrade-1/static-main-inspection.json. The current source equivalent is apps/desktop/src/main/index.ts at the Protocol registration block.

Redirecting acceptance HOME does not establish a separate macOS LaunchServices registration boundary. This lane was expressly constrained to avoid native host protocol registration. No independent disposable macOS user/VM or verified system sandbox boundary was available to this lane. Both immutable apps were therefore left unlaunched; no Main/preload/ASAR patch or injected API override was used.

This is an acceptance-environment blocker, not a claim that application upgrade fails. Continue actual restart/retained runtime acceptance on a disposable macOS session with its own LaunchServices database, using these same pinned distribution bytes and a stable install/profile path. The existing historical Mac full-Main pass is not reused to close the new old-to-new upgrade gate.

## Subgate results and recovery

| Subgate | Result |
| --- | --- |
| Actual distribution integrity, bundle metadata and payload pins | PASS |
| Manual file-install old-to-new app replacement | PASS |
| Synthetic filesystem profile retention | PASS |
| Actual historical Main → upgraded Main restart | BLOCKED: native protocol/session isolation |
| Actual runtime profile/device/login retention | NOT_RUN |
| Business authentication/IPC and automatic updater upgrade | NOT_RUN |
| Developer ID/notarized delivery | BLOCKED: inspected artifacts are ad-hoc |

No app/daemon/provider process, external request, disk-image mount, database or remote CI run was started. Only task-owned staging, installed and profile directories were written and retained for replay. Authentication-mode configuration was not changed; the reviewed runbooks require retaining the deployment's selected device/password mode and separate server/account migration and login verification when applicable.

Changed files are this note, its JSON summary, new evidence/next-mac-upgrade-1 files, and a private task driver/staging tree. Product code and dependencies were unchanged, so product lint/typecheck was not repeated. Verification was the actual ZIP, installed-file, signature-structure and evidence/hash checks described above.
