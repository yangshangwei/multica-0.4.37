# Current-source directory packages for full Main acceptance

## Final runtime candidate v10

After the original performance gates passed with the batched iteration reads, the complete frozen source (performance, auth/logout, onboarding and prior fixes) was copied exactly into the separate packaging checkout. **mac-6 / linux-5** builds both exited 0 under the offline network-denial sandbox. `source-files-v10.json` contains 27,612 files, SHA-256 `29e2d9c6af8954a6104b0ae0abf58d061b266c00834a069dc1c5fde97d00a11a`. HEAD remains `6201a58102668ce19e5f2959ea7dfb807d932a43`; the exact dirty overlay is retained separately. Build and execution source manifests are identical, with no allowed deltas.

`source-binding-v10.json` SHA-256 is `916fd4d0c0fc7529ab1fb5ae693f465d4c7aa36c757eb419e5a171d3c1bc3ca9`. The actual proof files are `mac-6-proof.json` (SHA `bff6e09cd6828ca740583071358eb61965511e19d85adaaed9d3ae58f26b4a6a`) and `linux-5-proof.json` (SHA `249fa104e5f38b27188feca5ed15ccde915f3c48bc325a1dbad3b0246d6e8dc2`). Full payload and ASAR audits passed; common compiled outputs are identical. `final-source-v10-audit.json` confirms zero source-byte/mode drift in QA and packaging checkouts.

Linux's new Docker-shared copy is `/Volumes/artisan/code/2026/mc-core-qa-20261108/.omx/private/core-linux-main-final/app`; all 80 files, hashes and modes equal the original Linux-5 payload. An actual cached-image, network-disabled, read-only mount probe exited 0 without launching the app. `linux-final-shared-copy.json` SHA is `5f86794f007e8dc3559cf96f8981a4c475d50096221121c02676afecdf768e07`. The old shared copy was preserved.

The independent reviewer owns the exact v10 controller rebind (mac-6/Linux-5 labels and new shared-copy path). Actual app/browser/API execution remains root-owned; this packaging lane has not launched the final apps. Earlier v9/v4 packages and their runtime evidence remain historical and are not relabeled as v10 results.

## Auth/onboarding candidate v9

After the final browser run exposed cookie-logout ordering and questionnaire write ordering, the reviewed shared runtime fixes were copied into the isolated packaging checkout. `source-files-v9.json` freezes 27,610 current QA source files at SHA-256 `b7811b89eb5c10ca2bcc0fd67cc271135bf1fba43e8a8a6d40afb04006edf3d3`, still HEAD 620 with the recorded dirty overlay. **mac-5 / linux-4** directory builds completed offline with exit 0. Their proof, full-file and ASAR records use those labels; old candidates remain intact.

These packages are an intermediate auth/onboarding candidate. The parent subsequently authorized a backend performance repair, which will change production API/bundled CLI inputs. Do not treat v9 as the final runtime candidate or silently reuse its bundled CLI after that repair. Final package/guard binding waits for the backend source freeze; application runtime was not launched for v9 by this packaging lane.

## Integrated 620 candidate

The current final target advanced to `6201a58102668ce19e5f2959ea7dfb807d932a43` while testing continued. It includes the iteration durable-write callback fix and the committed collaboration Leave readiness helper. Packaging preparation preserves all earlier 725 packages/snapshots and copies the exact current QA overlay into the separate packaging checkout. New `source-files-v4.json` binds 27,607 files with SHA-256 `5464885455b7dc51ad45d6df3eba94b950b5fda5590efc13090cadcc51ee64c6`; `source-v4.json` records HEAD and overlay identity. The project conflict fix retains its reviewed `59cd62f7…` hash.

The new outputs **mac-4 / linux-3** both completed with exit 0. Full payload/ASAR audits passed; common compiled outputs are byte-identical, version `0.6.7-582-g6201a5810-dirty`. The packaging source exactly matches v4. During the build, root repaired two E2E Mention-disclosure helpers (`collaboration-cross-client-execution.ts`, `collaboration-host-runtime.ts`); v5 records those exact test-only deltas, with all production bytes/modes and all other 27,605 source files unchanged.

`source-binding-v5.json` SHA-256 `f6a2ae8454a8f140a40d4d9152e5635d735e60b91a2d659f705178187e965bc7` binds v4 compiled inputs, v5 execution inputs and both exact proof files. Mac proof SHA-256: `a913d8848f5f65f7bfcb34426acc9f1c8b421321167d35d5cbdcc4c3fc83033a`; Linux proof SHA-256: `d03bc3d9060d6d417ec8c975dddc0885a5b3eb4ab972ac5eac304427864a3172`. `final-source-v4-audit.json` retains the full comparison. Both bundled CLIs have mode 0755; the separate controlled-fixture compiler umask defect did not affect their package modes.

Earlier v2→v3 test-only rebinding cannot cover the new iteration product changes. The independent controller reviewer owns the final exact v4/v5 rebind and label selection; runtime stays root-owned and has not been started by this packaging lane. Earlier source/proof/package records remain unchanged.

Both unsigned directory packages were built offline in the new task-owned checkout `/Volumes/artisan/code/2026/mc-core-package-20261108`. The latest **mac-3 / linux-2** variants contain the reviewed P1-P10 product fix and current test fixes; the earlier **mac-2 / linux-1** packages remain preserved as pre-fix evidence. No application, daemon, API, browser or full Main test was launched by this packaging task.

## Post-fix rebuild and prepared runner

`source-files-v2.json`, `source-v2.json` and `source-v2.patch` bind the exact reviewed product file SHA-256 `59cd62f77b7e2ecccfe33301dae80cec897cad7144c6e80d8f14e8f9392d07fb`, canonical component tests and current E2E repairs. Both `mac-3` and `linux-2` builds exited 0 under the same offline network sandbox. Exact current artifacts are in `mac-3-proof.json` / `linux-2-proof.json`; full-file and ASAR inventories have matching names. The pre-fix tables below are retained for provenance and must not be selected for final acceptance.

The separate `/Users/artisan/.multica/qa/core-20261108/packaged/run.py` controller prepares fresh password/managed databases and selects exactly two original specs:

```sh
python3 /Users/artisan/.multica/qa/core-20261108/packaged/run.py \
  --execute --run-id packaged-native-1 --ids pw-10
python3 /Users/artisan/.multica/qa/core-20261108/packaged/run.py \
  --execute --run-id packaged-linux-1 --ids pw-15
```

Root must release all other browser/API work first. The controller validates current source bytes against `source-files-v2.json`, all three package hashes, original service ownership, and fixed production client origins. Native Host compiles current explicit controlled provider/daemon/CLI binaries; Linux uses the exact cached image, current package read-only mount and a local Docker no-pull guard. The reviewed original controller's interrupted-process settlement, separate original/cleanup errors and final API restoration are retained.

Native cleanup snapshots LaunchServices before app launch, unregisters only the exact task-owned `.app` path afterward, restores the original default handler and requires the entire handler set to match. The read-only preflight found `com.github.Electron` as default and both `ai.multica.desktop` / `com.github.Electron` in the handler set. The small CoreServices helper was compiled locally with clang and a snapshot invocation succeeded. Initial Swift compilation failed due the local Foundation/SDK module interface and remains recorded in `launchservices-build.log`; `launchservices-build-2.log` records the successful Objective-C compile. No registration or restoration mutation has run yet. An absent default refuses entry rather than introducing an un-restorable OS preference.

Controller syntax/preparation mode and the two-spec Playwright collection passed. Runtime, cleanup restoration and unsigned ARM launch remain NOT_RUN. The controller and native helper are submitted for independent review before root execution.

## Source and build identity

- Source checkout: `/Volumes/artisan/code/2026/mc-core-qa-20261108`.
- HEAD: `7253784066d51a53432687aeada8cc208a31a943`.
- Version: `0.6.7-580-g725378406-dirty` on both platforms.
- Exact source snapshot: `/Users/artisan/.multica/qa/core-20261108/packaged/source-files.json`, 27,607 tracked/untracked product source files; SHA-256 `8c9f95888297566a544be2538e57ed73ec47047e1d317dafd38b8e56b80b14d7`.
- Dirty overlay patch SHA-256: `fc259602c6d68f7d1fa7850c4cd0810381fadb8d77dc31a117fc55020286d513`.

The source copy includes existing Chinese docs/assets, the built-in docs bundle and both current test-only fixes. A final byte audit found no differences between the captured snapshot, packaging checkout and serving QA checkout at packaging completion. Workspace dependencies resolve to the packaging checkout's own core/ui/views packages. Neither the serving checkout's `out/` nor another user's artifacts were changed.

Dependencies installed with `pnpm install --offline --frozen-lockfile --ignore-scripts`. Existing Electron 39.8.7 macOS/arm64 and extracted Linux/arm64 distributions were copied into task-owned staging and passed explicitly through `electronDist`. Build commands ran under an OS sandbox denying network operations; Go used `GOTOOLCHAIN=local`, `GOPROXY=off`, `GOSUMDB=off`. No signing account, publishing or notarization credentials were provided. Authentication/upgrade guidance in both required offline documents was reviewed; this work does not change deployment authentication or migrate an existing database.

## Artifacts

Private artifact root: `/Users/artisan/.multica/qa/core-20261108/packaged/`.

| Platform | Directory package | Proof |
| --- | --- | --- |
| macOS arm64 | `mac-2/mac-arm64/Multica.app` | `mac-2-proof.json`, `mac-2-files.json` (745 files) |
| Linux arm64 | `linux-1/linux-arm64-unpacked` | `linux-1-proof.json`, `linux-1-files.json` (80 files) |

Each proof records source identity plus executable, app.asar and bundled CLI path/hash/size. Read-only ASAR audits confirm the package version, required Main/preload/renderer outputs, platform-specific CLI headers, and only allowed top-level inputs (`node_modules`, `out`, `package.json`, `resources`). Both archives contain identical common compiled outputs. `final-source-audit.json` records the full source and workspace-edge comparison. These are file/content checks, not application launch evidence.

macOS executable SHA-256: `cf74833a9b0c242e26c177005d7d61819e5bf5ed199acc794fed2eb0cc4f3149`.

macOS app.asar SHA-256: `fb40fa513d27586d0385f6dc9d2d543b9f0e2aa5560feb83dfd4e902c0317438`.

macOS bundled CLI SHA-256: `a127deabfbff234b3a87bf2758bede661a8a9197c077d1071cbaa0ce39ab1514`.

Linux hashes are recorded in `linux-1-proof.json`; use that exact record when preparing the test environment rather than copying a historical package hash.

## Preserved first failure and signing limitation

`mac-1` failed before packaging because the private launcher redundantly passed `-c.mac.notarize=false`, while the repository package wrapper already adds it. Electron-builder received duplicate values instead of one boolean. The failed log/exit 1 remain in `mac-1.log` and `mac-1.json`; removing only that redundant launcher argument produced `mac-2`, exit 0. No product code or assertion was changed.

The macOS package deliberately uses `identity=null`. Electron-builder reports signing skipped and warns that arm64 requires signing. Its actual launch viability is NOT_RUN. If the native runtime rejects it, preserve that failure and prepare a separate task-owned ad-hoc variant if authorized by root; do not silently use a Developer ID or claim signature/notarization acceptance.

## Later full Main execution

Prepared public-origin/package bindings:

- `native-mac-environment.json` → `e2e/collaboration-host-installed.spec.ts`.
- `linux-environment.json` → `e2e/collaboration-installed-main.spec.ts`.

These records contain no database passwords or model credentials. Their fixed API/Web/renderer origins come from the current read-only detail allocation. They are **not standalone runnable environments**: root must inject the fresh password/managed database and matching deployment UUID, and for native Host also the current controlled daemon/provider/CLI binaries. Current API ownership must be released by baseline before any switch.

Before selecting either package, run:

```sh
python3 /Users/artisan/.multica/qa/core-20261108/packaged/verify-source.py
```

This refuses current-source drift with exit 2. A product fix requires a newly numbered source snapshot/build rather than weakening that comparison or overwriting these pre-fix artifacts.

Native macOS bindings include the exact executable, source/executable/ASAR/CLI hashes, a private acceptance HOME and `MULTICA_PACKAGED_ACCEPTANCE_HOME`, and an intentionally absent task-owned DSH path. Preserve the current user's Multica protocol association: production Main calls `setAsDefaultProtocolClient`, so record the prior LaunchServices handler and restore only task-added registration after the actual run. Private HOME alone does not isolate that OS registration.

Linux bindings select the cached arm64 harness image `sha256:db164dd5195d37ded1e6732eec4cc6b87decac66825be15fae9c7b87b2bac6c4` and the new directory package as a read-only mount. Use the existing controlled Docker no-pull guard and local socket binding. This is actual packaged Linux Main in the cached container environment, not native Linux desktop or DEB installation evidence.

No installer was generated, distributed or installed. Automatic update, publisher signature, notarization and release acceptance remain separate from these directory packages and full Main business journeys.
