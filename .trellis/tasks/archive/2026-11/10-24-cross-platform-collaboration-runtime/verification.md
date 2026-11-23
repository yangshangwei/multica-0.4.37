# Verification ledger

This task is in progress. The records below distinguish code/adapter tests from Provider-account and packaged-platform acceptance. Nothing here enables production admission.

## Resumed control and installed-client acceptance — 2026-10-26

The original identity inventory is **31/31 passed**. The original collaboration
inventory is **58/58 passed**: 30 retained qualified-source records plus the 28
previously pending originals executed at `1bfd169dc`. The final selected new
results contain no retries, skips or extra originals. Main-session verification
matched each original file/project/title to an actual expected-pass result.

Current Linux arm64 full Main/preload/renderer, native macOS host composition,
Linux namespace control/recovery, cached v0.6.7 CLI boundaries, offline flows and
both-client administration passed in their explicitly configured environments.
Package hashes stayed identical across the shared-path recovery. The first
Docker-125 prerequisite failure remains retained alongside the successful rerun.

The isolated gates are closed and owned API/Web/renderer/test processes stopped;
data, worktrees and raw evidence remain. See
[the exact acceptance handoff](acceptance-continuation-2026-10-26.md) and its
sanitized machine-readable evidence. Native Windows/remaining installer and
authenticated Provider-account gates are still pending; this inventory result
does not open production admission or complete the broader platform matrix.

## Debian installation continuation — 2026-10-26

Independent recovery audit matched all 89 retained identity/collaboration
originals to one actual pass each and verified 51 evidence/artifact hashes.
The inventory remains 31/31 and 58/58 with its original source qualifications.
See [the audit](research/progress-reaudit-2026-10-26.json).

The pinned accepted Linux payload was wrapped in a real arm64 `.deb` using
`--prepackaged`; its original three file hashes stayed identical. Fresh Debian
12 arm64 container installation verified actual `dpkg` ownership/status,
architecture, Debian/application/CLI versions, desktop/protocol/icon metadata
and preservation of the standalone CLI command. Offline install preflight
passed after the separate test image received three declared runtime
dependencies. The original dependency failure remains intact.

Final full Main runs passed with no retries/skips: installed Debian client
30.418s; original directory client 26.853s. Both run the existing real password
login, scoped Stop, restart while paused, retained history and explicit mixed
author Resume assertions against the separate native macOS fixture Provider.
An initial new assertion missed the CLI's existing leading `v`; its failure is
retained, and only the comparison was normalized. Review also repaired cleanup
so daemon teardown rejection cannot bypass Main close or failure attachment.

Final scoped ESLint, strict E2E TypeScript, syntax and diff checks passed;
[independent review](research/installer-review-2026-10-26.md) has no unresolved
finding. Gates are closed at epoch 7; owned API/Web stopped, ports
19180/14100/6274 are closed and no owned test containers remain. Data and all
raw attempts are retained. Exact hashes, commands, image dependency versions,
results and cleanup are in
[the installer evidence](research/deb-install-acceptance-2026-10-26.json).

This root/Xvfb container uses the existing `--no-sandbox` test path and does not
prove OS sandbox configuration, other distributions/architectures, in-place
upgrade, automatic updates, macOS/Windows installers, vendor accounts or
publication. No fresh full repository gate is claimed for this fixture-only
continuation.

## macOS packaged full Main acceptance — 2026-10-27

The packaged macOS arm64 control journey passed after the fixture explicitly
opened the task-owned creation/host gates, waited for the enrolled runtime to be
online, initialized the fresh acceptance home in English, and waited through the
short post-login daemon `starting` state before asserting it was stopped. The
same Electron 39.8.7 ad-hoc signed `.app`, ZIP and DMG were used for the run;
the acceptance home and `userData` were task-owned and isolated.

The run used the real password form and a separately running test-owned native
host fixture. It passed with 1 expected test, 0 unexpected, 0 skipped and 0
retries in 29.9 seconds. It covered shared participant input, target Stop,
restart while paused, mixed-author input, explicit Resume, three real host
launches, v3 lifecycle receipts, capacity release, sealed ledger records and
`host_exit_observed`. Gate cleanup restored all three booleans to `false` and
advanced the database epoch to 8. Machine-readable hashes and scope limits are
in [the acceptance evidence](research/macos-packaged-acceptance-2026-10-27.json).

This proves packaged macOS arm64 control against the test-owned host fixture.
It does not prove a real Provider account through the installed delivery, other
macOS architectures, Windows, notarization or release readiness.

## Verified during implementation

| Area | Evidence | Result |
| --- | --- | --- |
| Protocol and service | Host v3, strict legacy v2, independent launch receipts, unauthorized tombstones, sealed final count, mode-aware SQL/Go settlement, concurrent permits | Focused DB tests and race checks passed |
| Authorization and control | Current input authors, resource/Agent owners, permission-change consent, explicit Resume, Stop/revoke cleanup, legacy Linux draining and host cutover | Focused DB tests passed |
| Queues and squads | Three independent leader/member/continuation runs with capacity one, preserved old-mode group continuation | DB composition tests passed |
| Read compatibility | v6 mode/cleanup facts; consistent unavailable v5 host graph; mixed comments retain people and block incompatible AI; historical proof identity after Stop | Service and HTTP tests passed |
| Provider lifecycle | Common real Start/Wait, Codex internal retry, rejected/repeated calls, Stop races, scoped environment refresh and escaped descendants | Guarded native macOS package/race tests passed; final CI-equivalent matrix passed with Go 1.26.9 and 1.27.0 |
| Daemon ledger/runner | Corrupt retained claims, interrupted responses, exact permit/proof replay, unknown/session barriers, pending outcome ACK after proof, preparation facts | Guarded native macOS package race tests passed |
| Git and attachments | Fixed local Git bundle/commit, no dirty/untracked copy, every helper permitted and waited, explicit unsupported input rejection | macOS arm64 fake-Provider integration passed |
| Linux host adapter | Native Linux arm64 container, unprivileged UID 1000, no namespace setup, no network/account access; actual test-owned Provider and Git processes | Host lifecycle/runner rerun after review fixes passed |
| Shared frontend | Final third check: Core 308 files / 4,147 tests; Views 584 files / 7,030 tests | Passed |
| Repository typecheck/lint | Final third check: 15 lint/typecheck tasks | Passed; existing views lint warnings outside collaboration |
| Repository TypeScript suites | Final third check: all 5 suites; Web 285, Desktop 1,235, docs 62 in addition to Core/Views | Passed |
| Production Web/native Electron host journey | Four real test-owned native starts; shared participant use, scoped Stop, independent issue, mixed-author explicit Resume, current CLI hash, attachment bytes and v3 receipts | Final rerun passed: 1 test, 25.7s; both clients had zero page errors |
| Availability copy | v6 enabled/disabled and legacy v5 behavior, preserving retained facts and Stop; actual Web/native captures | 11 component tests passed; 19 final screenshots; visual review passed (96/100) |
| Native CI evidence gate | Missing/unsupported OS, missing/failed required tests and skipped required descendants | 7 script regressions passed; corrected gate accepted all 11 required macOS cases |
| Packaged full Main host journey | Linux arm64 installed/directory clients and macOS arm64 packaged `.app` against the native host fixture; real password login, target Stop, app restart while paused, mixed-author explicit Resume, history, CLI/attachments and capacity | Linux `.deb` 30.418s + directory 26.853s; macOS `.app` 29.9s; all final runs passed with zero retries/skips |
| Complete repository gate | Third `make check`, Go 1.26.9, fresh API/Go databases, serialized Go packages | Static/TS/all Go race/vet passed; browser result 260 passed, 128 skipped, 1 invitation-navigation timeout. Run ended and cleaned its services/Go DB; investigation remains open |
| Final traced repository gate | Canonical `scripts/check.sh --trace=retain-on-failure`, identical checks with failure-trace retention and zero retries | Passed, exit 0: all static/TS/Go race/vet checks; browser 261 passed, 128 environment-gated skips (13.9m). Services stopped and isolated Go DB dropped successfully |

## Platform/Provider admission matrix

“Fixture” identifies a test-created executable exercising the real ordinary backend/OS path. It is not a claim that an authenticated vendor CLI version or an installer was accepted.

| OS | Arch | Native lifecycle/ledger | Host backend/file fixture | Authenticated Provider/version | Packaged acceptance / admission |
| --- | --- | --- | --- | --- | --- |
| macOS | arm64 | Passed; 46 top-level tests + 62 subtests, zero failures/skips in both final toolchains | Claude fixture + Git passed; actual Web/Electron host composition passed | Claude Code 2.1.310 smoke passed; full installed-delivery Provider/account acceptance pending | Packaged `.app` full Main control against test-owned host fixture passed; ad-hoc local artifacts, not released |
| Linux | arm64 | Host suite passed in a native arm64 Linux container | Claude fixture + Git passed | Not run | Directory full Main and real Debian 12 `.deb` installation/control passed against macOS fixture runtime; no release/admission |
| Linux | amd64 | Cross-compilation only | Pending native CI | Not run | Unavailable |
| macOS | amd64 | Pending | Pending | Not run | Unavailable |
| Windows | amd64 | Cross-compilation only, including ACL/Job fault tests | Pending native CI | Not run | Unavailable |
| Windows | arm64 | Cross-compilation only | Pending native runner | Not run | Unavailable |

The supported local backend adapters are Claude, Codex and OpenCode. Remote/gateway modes and unmapped configurations remain rejected. Opening any actual Provider/version/OS/arch combination requires its own evidence; a provider name or compilation success does not establish support.

## Remaining gates

- Native Windows CI publication approval remains pending. A verification-only branch is ready; no push or workflow run has occurred.
- Native Windows failure matrix and remaining packaged platform combinations. Linux arm64 lifecycle/runner and current directory-packaged full Main client acceptance have passed, with their separate execution scopes recorded.
- Installed-delivery Provider/account acceptance for the selected initial combination.
- Remaining platform/package rollout stays disabled; keep this task in progress until the required native/account acceptance is satisfied.

## S6 continuation — 2026-10-26

The first delivery matrix is recorded in
[research/s6-delivery-matrix-2026-10-26.md](research/s6-delivery-matrix-2026-10-26.md).
The local machine completed a guarded real Provider smoke using Claude Code
2.1.310 on macOS arm64. The smoke passed in 43.129s and used a test-owned
collaboration server/issue; it returned the fixed `HOST_SMOKE_OK` marker and
did not modify files, comments or accounts. Raw output and its hash are in
[the S6 evidence](research/s6-local-acceptance-2026-10-26.json).

The focused lifecycle evidence also passed for session replay, fresh-session
fallback rules, scoped Stop, transient disconnect, receipt replay and logical
session ownership after supervisor restart. This is separate from the real
Provider smoke because those fault cases intentionally use the repository's
test-owned provider fixture.

The current source produced an unsigned/ad-hoc macOS arm64 `.app` and ZIP
with Electron 39.8.7. A packaged acceptance-home opt-in now maps Electron
`home`, `userData`, runtime config and daemon preferences into a task-owned
temporary directory. Its focused tests, Desktop Node typecheck, lint and
strict E2E fixture checks passed. A full packaged macOS control attempt reached
the packaged startup/isolation path but stopped before collaboration control:
the seed fixture received HTTP 503 while creating the issue, with an associated
host claim/register timing failure. This attempt is retained as a blocked
fixture run, not a packaged-control pass. The temporary API/Web/DB were
stopped or dropped; production admission and release remain closed.

## Local implementation commits

These commits are on `codex/cross-workspace-collaboration` and have not been pushed. The separate native CI snapshot remains a verification-only branch.

| Commit | Scope |
| --- | --- |
| `75d4d341c037` | Serialize shared-database test packages |
| `e4df692885c7` | Ordinary Provider launch/wait observation and native lifecycle |
| `8bfc84ad17fb` | Host protocol, migration, service, CLI and daemon integration |
| `88d9949b1f06` | Shared-client negotiation, consent and truthful execution facts |
| `a76605caca6c` | Native CI gates and Web/packaged host acceptance |

Parallel work and generated `apps/web/next-env.d.ts` are excluded from these commits.

Detailed local command output is retained in `/tmp/multica-host-runtime.HiUXkC/`, `/tmp/multica-host-review.meYfq8/`, and `/tmp/multica-host-frontend-{full-tests,typecheck,lint}.log`. These paths are local verification artifacts, not public runtime diagnostics.

## Final evidence locations

- `native-macos-final.jsonl` and `native-macos-go1.26.9.jsonl`: complete native matrix event streams; original evidence retained unchanged.
- `native-linux-arm64-complete.log`: final non-root native Linux host lifecycle/runner execution.
- `host-e2e-banner-final/collaboration-host-runtime-ae4a5-ly-resume-after-scoped-Stop-chromium/`: `host-runtime-evidence.json`, `build-provenance.json` and 19 screenshots; production Web build `H590KeAXOv0Kk5MKecxNY` and matching renderer hashes.
- `host-banner-red.log` / `host-banner-green.log`: the availability regression fails before the fix and passes afterwards.
- `admin-cross-package-parallel.jsonl` / `admin-cross-package-serial.jsonl`: identical 12-iteration tests fail with package parallelism 2 and pass with parallelism 1. This establishes the shared singleton interference; it does not conclusively explain every earlier empty-result/404 failure.
- `make-check-third.log`: complete third-gate output. The single failing browser case is `workspace-access-branches.spec.ts:87`; the original accepted/redirecting screenshot and API/Web provenance are preserved under `invite-failure-original/`. Its API calls returned 200 quickly; a slow backend and simple query invalidation have not been established as causes. Eleven unchanged canonical traced runs and eight passive diagnostic cases passed the original five-second assertion, so no production fix or timeout increase is claimed.
- `invite-investigation-final.json`: the bounded investigation and negative findings. Passive navigation completed in 1.859–1.867 seconds with no page errors. The final full run also passed this unchanged case in 2.8s. The original single timeout remains unexplained; no fix or increased timeout is claimed.
- `full-check-traced-final.log`: passed full canonical pipeline with traces retained on failure. `make check` does not forward `ARGS`, so this uses its underlying `scripts/check.sh` entry point directly; checks are not skipped or weakened. Final environment `check-20261025210127-31684` retains its launch/build provenance.
- `host-installed-final/collaboration-host-install-24c58-ly-resumes-the-shared-Agent-chromium/`: actual full Main host-control evidence, two native screenshots and `installed-host-provenance.json`.
- `package-source/.package-evidence/package-handoff.json`: Linux arm64/Electron 39.8.7/Go 1.26.9 directory-package provenance. ASAR `b35629813de01b3efcb8d0e2af29163b6f4a54f2b08a03911b75cffef39950b6`; original root Desktop outputs and Oct 22 volume were preserved.
- `packaged-host-boot-preflight.json`: real packaged Main/preload/renderer/CLI startup. The v6 fixture uses current `getCliPath()` IPC; the legacy fixture ABI remains unchanged.
- `native-ci-snapshot-review/`: local verification snapshot `92bb6d86` on `codex/verify-collaboration-host-runtime-20261026`, exact source manifest and branch-only native workflow. Upload to the user's public repository was requested but has not been authorized or performed.

The first composed host journey uses the native Electron renderer harness; the second additionally exercises actual directory-packaged full Main on Linux. Both use a separate native macOS fixture daemon and fake Claude executable, without authenticating a vendor account. Linux daemon lifecycle/Git preparation and squad composition have separate Go/DB evidence. These distinctions remain part of the acceptance record.
