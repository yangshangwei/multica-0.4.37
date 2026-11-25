# Linux Main preparation, 2026-11-25

Candidate remains `103e2bc329a9fb845d9f45c20b2a9bb3ebb54625`. Package preparation and offline checks pass; the canonical Main journey has not run. Runtime owns the service slot after the stage/native handback recorded in `stage-acceptance/slot-handback-runtime-20261125-0657.json`.

Paths below use QA=`artifacts/qa/main-post-merge-e2e-20261124-080504` and PRIVATE=`.omx/private/qa-work/main-post-merge-e2e-20261124-080504`.

## Review inputs

- Final scripts: `PRIVATE/linux-main-acceptance/{prepare_linux,run_linux_main,linux_docker,linux_selftest,share_linux_package,exporter}.py` and the exact private copy `installed-pnpm-receipt.private.json`.
- `QA/linux-main-acceptance/linux-final-offline03.json` gives all seven exact file hashes and the command/log digest. Seven offline checks pass, including a real Node child resolving the Docker guard, unchanged stdin and exit-code forwarding, selected-lane environment propagation, altered-command refusal, reused-container refusal, and synthetic identity byte/mode checks. All six Python sources parse.
- Final adapter/preparer/test diffs: `QA/linux-main-acceptance/{run_linux_main.py,prepare_linux.py,linux_selftest.py}.final03.diff`. Original sources remain in `PRIVATE/linux-main-acceptance/{docker-interface-v01,preparation-v01,preparation-v02}/`.
- Fresh launch preparation: `QA/linux-main-acceptance/linux-main-02/inputs.json`; corresponding review template is `PRIVATE/linux-main-acceptance/linux-main-02-review-template.json`. It requires independent acceptance with matching inputs digest and `allow_linux_main=true`. No self-issued clearance exists.
- After independent review and a new exclusive slot only: `python3 PRIVATE/linux-main-acceptance/run_linux_main.py run --label linux-main-02`.

## Package outcome and preserved failures

Attempt01's interrupted source/electron copies remain. Attempt02 stopped before compilation because the nonsecret public inventory receipt was0644, while the private JSON reader requires0600. Its refusal is retained in `preparation02-refusal.json`; attempt03 uses an exact0600 copy, pins both original/copy SHA256, and does not alter the original receipt or private-reader contract.

`linux-cli03.json` and `linux-package03.json` record exit0. `package-inputs.json`, `asar-verification.json`, and `package.json` bind the frozen current06 product/dependency tree, current Linux arm64 CLI, cached Electron39.8.7 and all packaged members. Original current06 and cached Electron manifests are unchanged. No install, dependency download, signature, publication or Main launch occurred.

The first actual read-only Docker mount rejected the external `/Users/artisan/.multica/qa/.../package03` path, exit125 before container startup. `package-preflight03.json` preserves that failure. Its case-sensitive absence detector initially reported false; `package-preflight03-container-audit.json` retains the real lowercase `no such object` response proving there is no residual container. No speculative cleanup occurred.

`share_linux_package.py` copied the exact verified package into `PRIVATE/linux-main-acceptance/shared-package03`, an existing Docker-shared checkout location. `shared-package03-intent.json` and `shared-package03.json` retain complete before/after manifest equality and original package linkage. Execution homes remain external; only read-only package assets moved. Original `linux-main-01` preparation remains unrun and is superseded by fresh `linux-main-02` inputs.

`shared-preflight03.json` records the real network-none/read-only container check: Node22.23.3, Playwright1.58.2, Linuxarm64, `/root` empty and selected as HOME, all25 non-Claude provider override paths nonexistent, and all three mounted hashes matching. The container exited0 and is absent. The launcher now binds this preflight and refuses failed/mismatched evidence.

| Artifact | SHA256 |
| --- | --- |
| app.asar | `0ef7ca752979950c14942279182628e201d7a65191a2d37ce7a33b58db0992da` |
| multica-desktop | `379334fb6707615062d32fcc481a8a8d3140938531378ccba14f3fa169665043` |
| Linux CLI | `45ffab6462ce1411a49f782586a26521e10685ea65e680c803eae6945602d521` |

## Runtime boundary and remaining risk

The former Python Popen hook could not intercept Node's own Docker spawn. It was removed in favor of the exact child PATH wrapper. Only the canonical argv, image, source/package mounts and synthetic identity paths are admitted. Added flags disable pulls, bind an owner label and CID file, provide an empty `/root` tmpfs, and override all25 non-Claude engines to missing absolute paths. Claude remains the canonical version-only discovery fixture; no ambient authenticated agent is selected. Credentials remain stdin-only and are never read or logged by the guard. Inspect/stop and fallback cleanup require the recorded container ID, label and image. The canonical full-source mount is read-only and the same source used by the unchanged test; no host HOME/profile is mounted.

The selected test remains the original `e2e/collaboration-installed-main.spec.ts`, exactly one case, one worker and zero retries. Its shared controller still has the known exact `Collaboration` heading assertion already failed by the macOS journey. No heading, product assertion, timeout or fixture source was changed. A prepared valid package and offline wrapper checks do not establish the Linux Main business journey or installer/updater/model acceptance.

No CLI ownership remains here and no CLI files were edited. Shared native probe/helpers stayed stable. Only private Linux preparation/QA and these dedicated research notes changed; tracked product source remains unchanged.

## Independent review corrections: linux-main-03

The independent review of linux-main-02 blocked the whole-checkout read-only mount and case-sensitive Docker absence matching. Its receipt remains `QA/linux-main-acceptance/independent-review-linux-main02-20261125.json`; all reviewed adapter/test bytes are preserved under `PRIVATE/linux-main-acceptance/review02-original/`. Both prior labels remain unrun. The earlier description of full-source isolation above is superseded by this correction, not retroactively accepted.

The new `controller_view.py` builds `PRIVATE/linux-main-acceptance/controller-view03` from exactly three unchanged canonical files: `collaboration-installed-main.mjs`, `collaboration-installed-layout.mjs`, and `collaboration-recipient-picker.mjs`. It copies only the already installed `@playwright/test`, `playwright`, and `playwright-core` packages at1.58.2. Package-manager nested `node_modules/.bin` wrappers are excluded; the three dependencies resolve directly within the frozen view. No install or download occurs. Full manifests bind directory/file modes and every file SHA256; hidden paths, symlinks, foreign ownership, writable ancestry entries in the view, added files and changed bytes are refused.

The Docker wrapper still admits only the exact canonical argv, but replaces its checkout mount with this frozen view at `/workspace:ro`. Its actual-child regression proves the original checkout is absent from forwarded mounts. Before every Docker operation, the wrapper verifies the frozen manifest and verifier hash. The ordinary source checkout remains the origin for unchanged outer Playwright tests, not a container mount. The exact package and synthetic machine/discovery files retain separate narrow mounts; empty `/root` and all provider guards remain.

`probe_controller_view.py` ran a fresh real network-none/read-only cached-image container with only that view mounted. `controller-view-preflight03.json` proves `/workspace` has exactly `e2e` and `node_modules`, `.env`, `.env.worktree`, `.omx`, `.git`, `.codex`, `artifacts`, and `CLAUDE.md` are absent, HOME is empty `/root`, both passive canonical helpers load, all three Playwright packages resolve under `/workspace/node_modules` at1.58.2, and the three controller file hashes match their source. It did not import the active Main controller or launch Electron/Main/CLI. The probe container is absent. The launcher binds this receipt and the complete frozen view before execution.

All three Docker absence sites now use one exact case-insensitive diagnostic matcher. It accepts only a nonzero inspect result with empty/`[]` stdout and an exact `no such object/container: <queried identity>` diagnostic, including the real lowercase `error:` form. Wrong identities, connection/permission failures, successful results and additional diagnostic lines remain failures. The actual Node-child test exercises lowercase absence at run admission and after container closure.

Outer completion now hashes its cleanup, captured Docker commands, final private registry, and nested runtime completion/cleanup/named inventory/source-end receipts. It checks captured/final registry equality and nested evidence hashes. Missing, malformed, unsuccessful or changed nested evidence prevents an outer PASS.

Final review artifacts:

- `QA/linux-main-acceptance/linux-review03-offline.json` and `.log`:10/10 offline checks, exact file hashes and command; includes view drift/private-path/symlink refusals and final evidence mutation controls.
- `QA/linux-main-acceptance/{run_linux_main.py,linux_docker.py,linux_selftest.py}.review03.diff`: exact changes from blocked reviewed bytes.
- `QA/linux-main-acceptance/controller-view03.json` and `controller-view-preflight03{,-intent}.json`: frozen source/dependency and actual container isolation evidence.
- `QA/linux-main-acceptance/linux-main-03/inputs.json` and `PRIVATE/linux-main-acceptance/linux-main-03-review-template.json`: fresh independent-review input and deliberately unaccepted template.

After independent acceptance and a new exclusive service slot only: `python3 PRIVATE/linux-main-acceptance/run_linux_main.py run --label linux-main-03`. The canonical heading mismatch remains unchanged. No execution clearance was self-issued, no runtime services launched, and no tracked product/test source modified by this correction.

## Actual Linux03 failure and minimal Linux04 preparation

Linux03 ran after the explicit CLI handback. It collected and observed exactly one canonical case, which failed with child exit2 and47 private stderr bytes before any Main container launched. Runtime and outer adapter exited1. Missing/unexpected identities and global errors were zero. The frozen controller view and packaged product remained unchanged; no product assertion or timeout changed.

The fresh connectivity container reached the exact103e2bc32 API and production Web200, then closed. The Main wrapper registry remained empty. The retained canonical synthetic identity directory is `.omx/private/installed-main-identities/container-XoXlF1`: parent0700, machine-id0400, discovery0500, correct owner and canonical paths. Discovery bytes match the reviewed fixture hash. Reapplying the reviewed pure guard to these exact files reproduces `Full Main synthetic identity ownership differs`, exactly47 bytes including newline. This identifies the failed private guard; the evidence does not claim Main boot or business behavior.

The canonical Node fixture requests0444/0555 file modes, but the private runner's077 umask yields0400/0500. `linux04-mode-red.{json,log}` retains a real Node file-creation regression failing against the original guard. The minimal correction changes the guard's two exact expected modes to0400/0500; it does not chmod the originals, broaden accepted modes, change process umask, alter canonical fixture bytes, or relax file owner/path/hash checks. The real Node-spawn recorder uses those actual modes. Expanded negative tests reject broader/writable modes and changed discovery bytes. `linux04-mode-green.{json,log}` records11/11 passing offline checks and direct admission of the retained Linux03 identities without executing them.

Original reviewed guard/test bytes remain in `PRIVATE/linux-main-acceptance/review03-original/`. Only `linux_docker.py` and `linux_selftest.py` changed for Linux04; exact diffs are `QA/linux-main-acceptance/{linux_docker.py,linux_selftest.py}.review04.diff`. The frozen controller view, package, outer adapter and canonical test remain unchanged.

`QA/linux-main-acceptance/linux-main-03/post-run-ownership.json` proves zero current owned processes, containers and volumes, API/Web/renderer ports closed, runtime locks absent, PG other clients0, restored gate booleans with epoch4, and unchanged final source/package bindings. The failed database, raw results, registry and synthetic identity files are preserved. PostgreSQL stays owned by runtime on49908 pending the next admitted attempt; no retry was started.

Fresh review inputs are `QA/linux-main-acceptance/linux-main-04/inputs.json` and `PRIVATE/linux-main-acceptance/linux-main-04-review-template.json`. After independent accepted review and explicit slot authorization: `python3 PRIVATE/linux-main-acceptance/run_linux_main.py run --label linux-main-04`. The Linux03 failure remains separate from this unrun preparation.

## Linux04 actual packaged Main result and final handback

Linux04 executed after independent accepted review and exclusive slot authorization. Exactly one canonical test ran and failed, zero pass/skip/retry/missing/unexpected/global errors; duration125487ms, runtime and outer adapter exit1. This is an actual packaged Main attempt, unlike Linux03's guard refusal.

The canonical failure attachment proves packaged Linuxarm64 Main, Electron39.8.7, app0.6.7-890-g103e2bc32, `file:///opt/multica/resources/app.asar/out/renderer/index.html`, and the actual bundled Linux CLI version command. The controller reached line528 only after the real password form returned HTTP200, the actor ID matched and the username field disappeared. It then failed the unchanged45000ms exact `Collaboration` heading assertion. This is the same canonical expectation already identified in the macOS run. No locator, assertion or timeout was changed and no retry followed. Account-detail navigation, subsequent information/acceptance handoffs, Stop/restart/Resume and later history/authority checks remain NOT_RUN.

Final audit is `QA/linux-main-acceptance/linux-main-04/slot-handback-global-final.json`, SHA256 `377fce79f5af3480e52401d5852cd03dd98c6206468e0a4e6c8d2c16fd0b0466`. It binds the extracted canonical failure attachment, milestones, raw Playwright/named results, nested and outer completion/cleanup, exact Docker registry and both protocol snapshots. The frozen source, package/controller view and all reviewed input bindings are unchanged. The Main container `multica-collaboration-main-97314619-034` (ID `0b17eec1704b3540d16e87ceed1af6a4e8ceb533bf78de2f7e075fa6b7541126`) and connectivity container are absent; fresh label inventories contain zero containers/volumes. No owned process remains and no fallback termination was needed. API/Web/renderer ports are closed, both runtime locks absent, PG other clients0, and gate booleans restored. Exact macOS protocol default and nine registrations are unchanged.

The exclusive service slot is returned to parent/runtime for final global cleanup. PostgreSQL remains runtime-owned on49908; the failed database and all historical evidence are retained. No additional test lane is planned or running.
