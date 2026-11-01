# Local candidate installer artifacts

Candidate `639bf6ef8`, product source_id `9f72fc21…d80ee`; tracked product files are unchanged. Local version is `0.6.8-rc.20261102`.

- macOS arm64 app, ZIP and DMG built successfully in `dist/local-candidate-639bf6ef8/mac-arm64-r3/`. The app has a verified ad-hoc signature and its actual bundled CLI reports this candidate/version and darwin/arm64. ZIP integrity and DMG verification passed. The canonical offline collector verified five metadata/artifact files and exact source bytes in `mac-collected-r2/`; nothing was published.
- Linux arm64 directory built successfully in `dist/local-candidate-639bf6ef8/linux-arm64-dir/linux-arm64-unpacked/`. Its Electron and bundled CLI are ARM aarch64 ELF; CLI build metadata reports this candidate/version. No Linux executable or container was run.
- Both ASARs contain only runtime roots and no `.env` or private agent/task/tool directories. Runtime JS/HTML and license hashes match across platforms. Archive, ASAR, app/directory-tree and CLI hashes are in the referenced JSON records.

All build/package/collection commands deny network operations. Frozen dependency install and both Electron distributions used existing local caches; no new dependency/tool, account, Provider or database was used. Both earlier Mac failures and the collector path setup failure remain recorded alongside their logs and output. Mac signing succeeded after using the canonical cached archive with real framework symlinks; production config was not changed.

This is artifact preparation, not release or installation acceptance. Mac installed/upgrade/account journeys remain root-owned. Linux `.deb`/RPM tools are unavailable and were not downloaded; directory output does not close those gates. Linux/Windows native installation, offline server image bundle, real models/accounts and final global acceptance remain open.

Inputs and evidence: `source-binding.json`, `electron-distributions.json`, `mac-framework-layout-correction.json`, `mac-artifacts.json`, `mac-runtime-audit.json`, `mac-asar-audit.json`, `mac-offline-collection-r2.json`, `linux-directory-artifacts.json`, `linux-asar-audit.json`, and `summary.json`. Raw logs and complete directory inventories stay under `.omx/private/local-packaging/`.
