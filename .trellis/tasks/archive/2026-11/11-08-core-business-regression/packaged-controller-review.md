# Packaged Main controller review

Reviewed the frozen incoming controller `f45a10888f22f0483782bd6eac5ed8f02aa3e8327bf0eccfd1f403605ced1681`, its manifest, mac3/Linux2 proofs, production Main/daemon discovery and the selected installed-Main fixtures. Root authorized fixes to the private controller after review findings. No application, API, browser, daemon, Docker container, database or real provider was executed by this lane. The runtime acceptance remains root-owned.

## Findings repaired

1. **Native discovery could reach ambient agent CLIs.** Private HOME alone does not restrict executable discovery: production Main adds normal install directories to PATH, and the real daemon probes provider paths and later versions/models. The controller now derives all 26 real `MULTICA_*_PATH` names from `server/internal/daemon/agents_probe.go` and descriptor `EnvPrefix` entries in `server/pkg/agent/builtin_runtimes.go`. Each native lane pins every provider to an absolute absent path under its private home, except a task-created Claude discovery stub. Absolute missing overrides disable the production login-shell and Codex-app fallback branches. The stub permits only `--version`, returns the same synthetic version used by the Linux fixture, records any other invocation and refuses it with exit97. A marker makes even an otherwise passing packaged test fail. Native child SHELL is `/bin/sh`, and HOME/USERPROFILE/XDG/Claude config stay private. Production Main and daemon source are unchanged.
2. **Full package/helper evidence was recorded but not enforced.** Entry now compares the full regular-file inventory, every SHA-256, size and mode against the pinned manifest. Missing/extra files, symlinks and changed manifest bytes fail admission. The application must resolve under its expected task build directory; the three named primary artifacts must belong to it. Source snapshot verification now also checks file modes. LaunchServices source and executable are checked against their reviewed fixed hashes before use.
3. **An unregister error skipped restoration.** The exact task-path unregister and original-default restoration now have separate guarded cleanup steps. Restoring the previous default is attempted even when unregister fails; both errors remain recorded, the handler/default snapshot must still match, and cleanup failure cannot yield an accepted run.

Original controller is preserved as `packaged/run.before-review.py`; the patch and before/after identities are in `packaged/review-repair.patch` and private `packaged-review-result.json`.

## Retained execution boundaries

- Only fixed task registry `check-20261108144845-80493` API is switched after root releases it. Web/renderer are checked but not rebuilt or stopped. Each package spec creates its own fresh local database and synthetic password identities, private HOME/TMP/catalog/uploads and max6/min1 pool.
- Runtime selected by the manifest is native macOS full Main (`pw-10`) and cached Linux full Main (`pw-15`). Host AI execution uses the explicitly compiled local provider fixture; native bundled-daemon discovery is separate and now constrained as above.
- API/WS/app/update origins are explicit local task endpoints. The fixture writes automaticUpdates=false before launch. Lane env does not inherit model credentials, SMTP or Cloud configuration. Linux uses the existing pinned arm64 harness, local Docker socket and no-pull guard; it mounts only candidate source/package and task-created machine/discovery data, not user homes.
- The selected fixture launches packaged Main/preload/renderer, not alternate IPC stubs. Its cleanup verifies/stops the managed daemon profile and closes the application. The outer controller records subprocess interruption, terminates its exact process group and preserves original execution failure separately from API restoration/source-audit errors.
- Native LaunchServices checks concern the `multica` scheme's default bundle identifier and returned handler-ID set. The task app path alone is unregistered; no global reset or unrelated application removal occurs. Runtime restoration is still a required observation; equal preparation snapshots do not prove that it succeeded after launch.
- Package proofs are directory-package evidence. They do not establish signing/notarization, installer lifecycle, updater behavior, Windows execution or commercial-model quality.

## Validation

Five credential-free helper methods PASS, covering full-manifest missing/extra/content/mode/symlink/manifest drift, helper-byte drift, the actual native child environment returned by `package_env`, discovery-only stub behavior, unexpected invocation marker rejection, and restoring the original default after unregister failure. The original cleanup path failed the unregister/restoration regression; new helper checks were absent before repair.

Read-only hash verification of the actual mac3 and Linux2 full payloads and both LaunchServices helper files passed. Python compilation passed. Neither the LaunchServices snapshot/restore executable nor any packaged app was run during this review. Root must still execute each packaged test and inspect live daemon/container/process and protocol restoration evidence before acceptance.

The repaired controller hash and all input hashes are in `/Users/artisan/.multica/qa/core-20261108/packaged-review-result.json`; raw tests/logs are `packaged-review-{tests.py,red.log,green.log}` in the same parent directory.

## Exact post-build test source binding (v3)

Root subsequently repaired only `e2e/collaboration-live-revoke-web.spec.ts` to open the real Mention disclosure before testing authority. The controller now pins `source-binding-v3.json` at SHA256 `945586671aaf6d81d343699dd791b5d2502c4b4fa30b87e42c6d64552e3ef434`. It retains the original v2 build-source and package proof identities, verifies v3 test-source identity, compares equal inventories and the complete exact before/after hash+mode delta, then checks all current files against v3. No generic E2E-directory exemption was added. Any later change requires a new exact binding and explicitly reviewed binding hash; changing only current files or a manifest is rejected.

The one allowed path changed from `25edb2607a927196ba3d21315261eb3775e7b0446c9fa203d0e86186780d3594` to `49db85d0cdb4c3f2e87643cf2776dc7463b2a89900a497ffb45dfc7262938718`, mode0644 unchanged. All other 27,606 recorded files/modes are identical. The original mac3/Linux2 proof files are additionally checked against their exact hashes in this binding; their build identity remains v2. Full payload, provider, helper and protocol-restoration guards are unchanged.

Credential-free tests passed the exact binding and ten rejection variants: binding or manifest drift, omitted delta, wrong before/after hash, duplicate delta, declared product change, changed mode, undeclared current test or product bytes. The prior five package guard tests also passed. Actual read-only audit verified current v3 source plus both complete original payloads; no application/API/database/browser was run. Evidence: private `packaged/source-binding-v3-reviewed-audit.json`, `source-binding-v3-runner.patch`, and parent `packaged-source-binding-{tests.py,red.log,green.log,prior-guards.log}`.

Final controller SHA256: `196535ba01e3219fc3ce5d58ca3ece36112a7ae92919e629b1a03b0c9b1d9fb8`. This establishes runtime-entry readiness for the exact bound files after root releases the API; actual packaged acceptance remains NOT_RUN by this reviewer.

## Current620 candidate binding (v5)

Root's current product candidate is `6201a58102668ce19e5f2959ea7dfb807d932a43`; the previous725 packages remain historical. The controller now selects **mac4/Linux3**, compiled from source-files-v4 (`5464885455b7dc51ad45d6df3eba94b950b5fda5590efc13090cadcc51ee64c6`), and pins exact `source-binding-v5.json` SHA256 `f6a2ae8454a8f140a40d4d9152e5635d735e60b91a2d659f705178187e965bc7`. Execution source-files-v5 permits only the declared hash/mode changes to `e2e/fixtures/collaboration-cross-client-execution.ts` and `e2e/fixtures/collaboration-host-runtime.ts`; all27,605 other recorded files remain unchanged. No generic test directory exclusion exists. Current QA HEAD and each package proof HEAD must equal the binding's620 identity. Backend tests edited only in the root checkout remain separate evidence and are not silently folded into this QA snapshot.

Original runner saved as `run.before-source-v5.py`; original source/proof files remain untouched. The guard consumes the v5 record's exact field names, pins original mac4/Linux3 proof hashes, verifies both full payload inventories and unchanged LaunchServices helpers, and retains provider/protocol/cleanup guards.

Validation: exactbinding acceptance plus10 rejection variants PASS; prior5 package guards PASS; Python compilation PASS. Actual read-only current-source/full mac4/Linux3 manifest checks PASS. Additionally, the real `prepare_packages` mac path completed against actual source/package files with only git/snapshot subprocess returns mocked, proving the current record field mapping without invoking LaunchServices or runtime. No API/application/browser/database/Docker execution occurred.

Final620 runner SHA256: `69da49a83b06f6dd6319536969b21d7607281dc974612a8cfa74796f46010932`. Exact audit: private `packaged/source-binding-v5-reviewed-audit.json`; patch: `source-binding-v5-runner.patch`; test logs: parent `packaged-source-v5-tests.log` and `packaged-source-v5-prior-guards.log`. Root may enter runtime only after the active controlled lane releases the shared API.

## Final controlled fixture binding (v6)

After root completed controlled execution, the final QA620 overlay contains exactly seven test-only changes from the original v4 package build: `collaboration-memory-runtime.spec.ts` and fixture files `collaboration-cross-client-execution.ts`, `collaboration-host-runtime.ts`, `collaboration-host-squad.ts`, `collaboration-installed-main.mjs`, `collaboration-lane-close-controls.ts`, `collaboration-lifecycle-controls.ts`. These are five additional changes beyond v5. Every exact before/after hash and mode is declared; all27,600 other captured files remain identical. Root-only backend test repairs are separate evidence, absent from this QA execution manifest; they do not alter the package's product identity.

The controller pins `source-binding-v6.json` SHA256 `8647bed358090d4456e9b7a954697574ed925b6dd09e9836252ca43c10a67b9d`, retaining the same mac4/Linux3 v4 build and full-payload proof hashes. Prior runner saved as `run.before-source-v6.py`; old manifests/proofs remain untouched. Final runner SHA256 `4af3cf346d61e846b298777bf97b7c1c5bbe8c2cdeb6b52b8e3047202cfb153d`.

Exact-binding acceptance and ten negative variants PASS; prior five package guards PASS; Python compilation PASS. Both real `prepare_packages` branches completed against actual current source, mac4/Linux3 full payloads and helper files, with OS LaunchServices/Docker inspections replaced by credential-free recorded return values. No app/API/browser/database/Docker operation ran. Evidence: `packaged/source-binding-v6-reviewed-audit.json`, `source-binding-v6-runner.patch`, and parent `packaged-source-v6-{tests,prior-guards}.log`. Runtime entry is ready for the exact frozen source; actual Main results remain root-owned.

## Linux Docker-shared copy binding

The first actual Linux entry failed with container125 because Docker could not bind the private `/Users` package path. Root also retained a denied `/Volumes/artisan/e2e-private` probe, then confirmed a successful bind under the existing QA checkout. No global Docker sharing configuration changed.

The controller now pins `linux-shared-copy-2.json` SHA256 `fe513a81743d914dd1a2a06fdbe04f8ada0600a35025b65638469a628cd8652d` and exact destination `/Volumes/artisan/code/2026/mc-core-qa-20261108/.omx/private/core-linux-main/app`. It verifies the original Linux3 package and copied80-file payload against the same original complete content/size/mode manifest, rejects redirected paths or record source/destination drift, and repeats that verification when selecting the actual `pw-15` child environment. The original package proof remains unchanged. Native selection and all source/provider/protocol guards remain unchanged; the author's API-log preservation addition is retained.

Original runner saved as `run.before-linux-shared-copy.py`. Final runner SHA256 `e99509cfbb0b740e14675d8f34cf94cfee93788240d67f62a61f4d53e1137722`. Eight exact-copy/negative variants PASS, prior exact-source acceptance/ten negative variants PASS, prior five package guards PASS. The first positive test fixture used a macOS symlinked temp root and was corrected to its canonical path without changing the guard; that failed log is retained. Actual read-only original/copy full-manifest and returned child-path checks PASS. No app/API/DB/browser/Docker command ran in this reviewer lane. Evidence: `packaged/linux-shared-copy-reviewed-audit.json`, `linux-shared-copy-runner.patch`, parent `packaged-shared-copy-{tests.py,red.log,green.log,green-2.log,source-guards.log,prior-guards.log}`.


## Final auth/onboarding/performance product freeze (v10)

The original mac4/Linux3 and intermediate mac5/Linux4 builds no longer represent final product source. The controller now selects mac6/Linux5 built from exact27612-file source-v10, HEAD6201a58102668ce19e5f2959ea7dfb807d932a43 plus declared auth/onboarding/performance product repairs. Build and execution manifest are identical, SHA29e2d9c6af8954a6104b0ae0abf58d061b266c00834a069dc1c5fde97d00a11a; deltas=[] (no exclusions). BindingSHA916fd4d0c0fc7529ab1fb5ae693f465d4c7aa36c757eb419e5a171d3c1bc3ca9 pins mac6proofbff6e09c… and Linux5proof249fa104…. Linuxsharedcopyrecord5f86794f… selects QA/.omx/private/core-linux-main-final/app with the complete original80-file hash/size/mode manifest.

Final runnerSHAf0d391fdd91bd517845a4a43499b73a084a2d2c6db63b99cc915dcf654adb8e3; previous runner preserved as run.before-source-v10.py. Existing API-log retention, controlled provider pins, exact LaunchServices helper bytes, restore behavior and native alias wrapper remain unchanged.

All source-binding exact/tennegative tests, five package-guard methods and eight shared-copy variants PASS; Python compilePASS. Actual currentQA source and full mac6/Linux5/original-copy manifests passed through both real prepare_packages branches with only OS snapshot/Docker inspection subprocesses mocked. No app/API/DB/browser or Docker execution by reviewer. Evidence: packaged/source-binding-v10-reviewed-audit.json, v10-guard-tests.json, review-v10-audit.log. Runtime-entry ready; root must collect new actualMain outcomes for this product freeze.


## Final native-focus execution binding (v11)

After final2 whole-suite exposed native clipboard focus readiness, the entire11case native account spec passed under the unchanged5second all-three-focus assertion and single real clipboard click. The runner now pins source-binding-v11 SHA68d09662d4cf1b151e267fc1ef7cc5e37000b3affa3d318e5d1beb96550b2090. It declares only e2e/collaboration-account-desktop.spec.ts hash5fb058ef…→7119535b… with mode0644 unchanged. All27,611 other captured source files match v10. mac6/Linux5 original build/proof/payload identities remain v10; no product rebuild is required for this single test change.

RunnerSHA62c5ba9f5e72d2b8379119ae233550ebb4130efe7c759ed0809149d3c3c2920c; old runner saved run.before-source-v11.py. Exact-source negative tests, package guards and shared-copy variants allPASS; actual source and both full package preparation pathsPASS with only OS inspection calls mocked. Existing native alias, Linux final shared copy, provider isolation and protocol/API cleanup unchanged. Evidence source-binding-v11-reviewed-audit.json and v11-guard-tests.json. Root may now perform final actualMain runs then the complete441suite; this reviewer performed no API/app execution.
