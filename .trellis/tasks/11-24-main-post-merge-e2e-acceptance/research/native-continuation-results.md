# Current native package continuation

Candidate: `103e2bc329a9fb845d9f45c20b2a9bb3ebb54625`. Run: `main-post-merge-e2e-20261124-080504`.

Status: **package preparation PASS; production Main journey FAIL in `installed-main-02`; cleanup and exact protocol restoration PASS**. The preparation sections below retain their original scope. The subsequent execution and its failure are recorded at the end of this report.

## Failure retained and narrow correction

The original `current05-assemble.receipt.json` remains exit1. Its first signing command failed with `bundle format is ambiguous (could be app or framework)` for Electron Framework. The installed Electron39.8.7 distribution has expanded framework links: root entries and `Versions/Current` are ordinary duplicate files/directories rather than links into `Versions/A`. All corresponding resolved file hashes and modes were equal before correction.

`mirror_electron.py` copied the installed distribution to a new task-owned external directory, then replaced only 14 duplicate-identical entries across the four frameworks with standard internal relative links. Its before/after content inventory proves all743 resolved files retain the same SHA256/mode. The original installed distribution was checked unchanged. No dependency installation, download, cache mutation or product code change occurred.

The existing private assembler gained one optional explicit task-local Electron distribution record. It checks the candidate, original distribution manifest, mirror manifest and task path before using that mirror. The original assembler is retained as `prepare_native.before-framework-mirror.py`. The frozen product/dependency tree was copied to a new `current06` label, preserving all previous attempts.

## Actual results

Evidence prefix: `artifacts/qa/main-post-merge-e2e-20261124-080504/native-acceptance/`.

| Evidence | Actual result |
| --- | --- |
| `continuation-plan.json` | Bounded correction and verification plan recorded before changes |
| `continuation01-electron-dist.json` | 14 link replacements; 743 resolved files identical; original cache unchanged |
| `continuation01-checks.json` | All private Python sources parse; existing offline safety suites20+21 PASS |
| `current06-stage.json` | Fresh frozen product/dependency copy; current-source audit PASS |
| `current06-assemble.receipt.json` | Canonical electron-builder26.8.1/installed Electron39.8.7, arm64 directory build, exit0 |
| `current06-package.json` | Complete package manifest, source audit, executable/ASAR/bundled CLI hashes |
| `current06-asar-verification.json` | All47,888 archive members checked;47,855 nonempty packed files,2 unpacked files; exact495 Desktop output files and resource bytes |
| `current06-signature.json` | `/usr/bin/codesign --verify --deep --strict --verbose=2` exit0 |
| `current06-cli-version.json` | Bundled CLI actual version command exit0; `v0.6.7-890-g103e2bc32`, commit103e2bc32, darwin/arm64, Go1.27.0 |
| `continuation01-protocol-before.json`, `current06-protocol-after-assembly.json` | Exact original default URL and registered application sets unchanged by assembly |
| `current06-launch-preflight.json` | 26 provider overrides, exact source/env/package/discovery hashes, fresh private HOME/scratch, no app launch |

The assembled application is:

```text
/Users/artisan/.multica/qa/main-post-merge-e2e-20261124-080504/native-acceptance/current06-package/mac-arm64/Multica.app
```

Signing is local ad-hoc only; it uses no developer signing identity, notarization or publication. Bundled CLI/resource files were excluded from re-signing and remain byte-identical to the pinned current build. This is a current-source test package, not a release installer or upgrade result.

## Concrete launch handoff

Use the unchanged canonical spec `e2e/collaboration-host-installed.spec.ts`, one test named **Packaged full Main controls a real native host run and explicitly resumes the shared Agent**. The spec already supports macOS through `COLLABORATION_INSTALLED_MAIN_EXECUTABLE`; no test adaptation is needed. It drives the packaged production Main/preload/renderer, real password form, real IPC/private managed daemon, cross-client Stop, application restart while paused, and explicit Resume through the existing controller.

The runtime owner must supply a fresh owned password+managed+host API/Web/native-renderer lane with the existing pinned `COLLABORATION_HOST_DAEMON`, `COLLABORATION_HOST_PROVIDER`, `COLLABORATION_HOST_CLI`, real database/deployment identity and unchanged original thresholds. Retain workers1/retries0, exact one-name collection and terminal accounting. The generic original/supplement runners do not currently admit installed Main automatically; adding its reviewed opt-in must not silently broaden other lane profiles.

Native environment JSON is `.omx/private/qa-work/main-post-merge-e2e-20261124-080504/native-acceptance/current06-native-env.json`. It includes:

- The absolute packaged executable, ASAR/CLI/EXE hashes and current full source SHA.
- All26 source-derived Provider path overrides.25 point to task-owned nonexistent paths; Claude points to the explicitly hashed discovery-only fake. OMP and DSH are included.
- Fresh external0700 `current06-acceptance-home` and `current06-acceptance-scratch`; no account configuration exists there. The only actually executed fake command in this continuation was `--version`, exit0.
- A discovery rejection marker that must remain absent. The fake only accepts version and structured initialize/list_models/get_supported_commands discovery; it refuses user/business execution.

Construct the native child environment from this JSON plus the reviewed lane's local inputs. Do not merge ambient credentials/model/account configuration. Keep the repository `scripts/go-test-with-agent-cli-guard.sh` around the actual command, and preserve all absolute provider overrides because Main's `fixPath()` can prepend ambient directories. The host fixture's own controlled Provider path remains separate from Main's discovery fake.

## Protocol preflight and restoration

The observed original default is exactly `file:///Applications/Multica.app/`, and nine preexisting URLs are registered for `ai.multica.desktop`. The preparation did not register its new `.app`.

Before actual launch, take a fresh snapshot with the existing private helper and inspect the exact original app URL/registered set:

```text
python3 PRIVATE/native-acceptance/protocol_state.py snapshot --output NEW_ABSOLUTE_QA_SNAPSHOT.json
python3 PRIVATE/native-acceptance/protocol_state.py plan --original NEW_ABSOLUTE_QA_SNAPSHOT.json --owned-app ABSOLUTE_CURRENT06_APP --output NEW_ABSOLUTE_QA_PLAN.json
```

The helper validates a canonical stable app path, refuses missing original defaults, rejects disappearance of existing registrations, and permits unregistering only a newly added exact current06 path with the expected bundle ID. It restores the original default by **exact URL**, not bundle ID, and requires exact snapshot equality afterward. A platform consent error is a failed restoration, never a synthetic pass.

`.omx/private/qa-work/main-post-merge-e2e-20261124-080504/native-acceptance/current06-protocol-review-template.json` deliberately says `REVIEW_REQUIRED`. The parent must independently review the helper, current package, fresh snapshot and hashes, then issue a mode0600 review with `status: pass`, `blocking_findings_open: 0`, `allow_protocol_restore: true` and matching candidate/python/swift/snapshot/app/ASAR fields. No self-issued execution clearance exists.

After the installed journey closes all of its own application/daemon processes, execute in a `finally` path even if the test fails:

```text
python3 PRIVATE/native-acceptance/protocol_state.py restore --original NEW_ABSOLUTE_QA_SNAPSHOT.json --owned-app ABSOLUTE_CURRENT06_APP --review ABSOLUTE_REVIEW.json --execute --output NEW_ABSOLUTE_QA_RESTORATION.json
```

Then require exact original default/registration-set equality, no owned live processes, an absent discovery rejection marker and unchanged source/package bytes. Keep the stable package and failed database/evidence rather than deleting them during system protocol recovery.

## Preparation boundary

The package and preparation checks now pass. A completed production Main journey still requires actual coordinated execution and successful cleanup. This preparation does not establish native Windows/iOS, commercial model behavior, signed distribution, installation, upgrade/downgrade or updater acceptance. Shared product lint/typecheck/tests remain the campaign's existing same-source results; this private Python-only correction did not rerun them or change their inputs.

## 2026-11-25 actual production Main execution

`installed-main-02` executed the unchanged canonical test with one worker and zero retries. It collected one identity and observed one failed terminal, zero skips, zero missing/unexpected identities and zero global Playwright errors. The browser command and native adapter both exited 1. The strict reconciler reports `inventory_complete=false` because the named test contains an error, rather than because a test identity is missing. The 480-second whole-test budget and 45-second login heading assertion were unchanged.

The actual failure is at `e2e/fixtures/collaboration-installed-main.mjs:528`: after the real password form submitted, HTTP 200 and the expected user ID passed, and the username field disappeared, the controller timed out waiting for the exact heading **Collaboration**. The complete test took 57.162 seconds.

Source inspection establishes a stale expectation: `DesktopAccountWorkbenchRoute` passes `navigationInSidebar` to `AccountWorkbenchPage`; that branch renders `page.current` or `page.participating`, whose English headings are **Current handling** and **Participating**. The controller still expects `page.title` (**Collaboration**). This is a canonical fixture mismatch with current product markup. The failing run did not retain the native DOM or a screenshot at this point, so this analysis does not claim the stale heading is the only possible runtime factor. No product or test change, weaker assertion, timeout extension or green-seeking replay was made.

Reached milestones are separately evidenced:

- Current-source production Main, preload and packaged `file:` renderer actually booted as macOS arm64 / Electron 39.8.7 / app `0.6.7-890-g103e2bc32`.
- Real native IPC returned the task-owned API, Web and WS configuration; Main HOME and userData were inside the fresh `installed-main-02` acceptance home.
- The bundled CLI version command ran successfully and matched the pinned current build.
- Real password-form submission returned HTTP 200 and the intended synthetic actor before the heading assertion failed.
- The separate controlled native host fixture enrolled and became ready, then exited 0 with zero claims, zero provider observations and an empty ledger.

The test did **not** reach account-detail navigation, native Stop, restart while paused, explicit Resume, or the final three-run lifecycle/history/proof assertions. This result cannot establish those flows or a green L7 gate.

### Protocol recovery and final resource proof

At continuation intake, the unrun `installed-main-01` preparation snapshot contained ten registrations: the original nine plus the exact task-owned `current06` package. The default was still `file:///Applications/Multica.app/`; no task-owned Main process was live. The two earlier snapshots agreed on the original nine registrations. The registration's caller/timing is not established by the retained evidence; it was not silently accepted as part of the original state.

After independent parent review, `continuation02-protocol-recovery.json` recorded unregistering only the new task-owned current06 URL, exit 0, and exact restoration of the original nine URLs/default. The old preparation was retained. `installed-main-02` then took a fresh snapshot, obtained distinct input-bound launch/protocol reviews and verified the same exact original state immediately before execution.

After the failed test, all cleanup paths ran. API/Web/renderer listener ports closed; the gate's original booleans (`enabled=false`, `creation_enabled=true`, `host_execution_enabled=false`) were restored with epoch increasing from 2 to 4. The native process monitor found no remaining owned processes and required no fallback signals. The only newly registered current06 URL was removed again; the exact campaign-original nine registrations and `/Applications/Multica.app/` default were restored without invoking the default setter. Full package/source/tool bindings remained unchanged, and the discovery-only provider rejection marker was absent. The runtime owner received the exclusive service slot back after cleanup. The failed database and all evidence were retained.

### Evidence and changes

Under this campaign's QA root:

- `native-acceptance/continuation02-result.json`: bounded milestone/failure/cleanup ledger with evidence and source hashes.
- `native-acceptance/continuation02-protocol-recovery-plan.json` and `continuation02-protocol-recovery.json`: exact recovery proposal and actual result. The independently issued review is in the campaign's private native directory as `continuation02-protocol-recovery-review.json`.
- `native-acceptance/installed-main-02/{adapter-inputs,run-start,completion,cleanup,protocol-restoration}.json`: actual reviewed launch and final native outcomes.
- `runtime/installed-main-02/{collection,playwright,named-inventory,completion,cleanup,source-end}.json`: exact canonical test, original failure, runtime ownership and source receipts.
- `runtime/installed-main-02/results/collaboration-host-install-7dc38-ly-resumes-the-shared-Agent-chromium/installed-host-evidence.json`: real boot, fixture lifecycle and controller timeline.

Fresh offline private checks passed 50/50 cases with `python3 -m unittest installed_main_selftest native_selftest preparation_selftest`. This continuation changed only this report and added private preparation/review inputs and QA evidence; existing launchers, product code, tests and dependencies were unchanged. Remaining risks are the failed canonical Main journey and the previously declared installer, updater, signing and native-platform boundaries.
