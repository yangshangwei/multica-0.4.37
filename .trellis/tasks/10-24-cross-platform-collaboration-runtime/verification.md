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
| Packaged full Main host journey | Current Linux arm64 Main/preload/renderer with real synthetic-user password login; native macOS host v3 runtime; target Stop, app restart while paused, mixed-author explicit Resume, history, current CLI/attachments and capacity | Passed: 1 test, 22.0s; three real host launches and two native screenshots |
| Complete repository gate | Third `make check`, Go 1.26.9, fresh API/Go databases, serialized Go packages | Static/TS/all Go race/vet passed; browser result 260 passed, 128 skipped, 1 invitation-navigation timeout. Run ended and cleaned its services/Go DB; investigation remains open |
| Final traced repository gate | Canonical `scripts/check.sh --trace=retain-on-failure`, identical checks with failure-trace retention and zero retries | Passed, exit 0: all static/TS/Go race/vet checks; browser 261 passed, 128 environment-gated skips (13.9m). Services stopped and isolated Go DB dropped successfully |

## Platform/Provider admission matrix

“Fixture” identifies a test-created executable exercising the real ordinary backend/OS path. It is not a claim that an authenticated vendor CLI version or an installer was accepted.

| OS | Arch | Native lifecycle/ledger | Host backend/file fixture | Authenticated Provider/version | Packaged acceptance / admission |
| --- | --- | --- | --- | --- | --- |
| macOS | arm64 | Passed; 46 top-level tests + 62 subtests, zero failures/skips in both final toolchains | Claude fixture + Git passed; actual Web/Electron host composition passed | Not run | Full Main package pending; not released |
| Linux | arm64 | Host suite passed in a native arm64 Linux container | Claude fixture + Git passed | Not run | Current directory-packaged full Main host-control journey passed against macOS runtime; no release/admission |
| Linux | amd64 | Cross-compilation only | Pending native CI | Not run | Unavailable |
| macOS | amd64 | Pending | Pending | Not run | Unavailable |
| Windows | amd64 | Cross-compilation only, including ACL/Job fault tests | Pending native CI | Not run | Unavailable |
| Windows | arm64 | Cross-compilation only | Pending native runner | Not run | Unavailable |

The supported local backend adapters are Claude, Codex and OpenCode. Remote/gateway modes and unmapped configurations remain rejected. Opening any actual Provider/version/OS/arch combination requires its own evidence; a provider name or compilation success does not establish support.

## Remaining gates

- Native Windows CI publication approval remains pending. A verification-only branch is ready; no push or workflow run has occurred.
- Native Windows failure matrix and remaining packaged platform combinations. Linux arm64 lifecycle/runner and current directory-packaged full Main client acceptance have passed, with their separate execution scopes recorded.
- Separately authorized real-Provider smoke for an initial delivery combination.
- Remaining platform/package rollout stays disabled; keep this task in progress until the required native/account acceptance is satisfied.

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
