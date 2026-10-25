# Verification ledger

This task is in progress. The records below distinguish code/adapter tests from Provider-account and packaged-platform acceptance. Nothing here enables production admission.

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
| Complete repository gate | Third `make check`, Go 1.26.9, fresh API/Go databases, serialized Go packages | Running in environment `check-20261025192655-82819`; first two runs failed and their findings were fixed |

## Platform/Provider admission matrix

“Fixture” identifies a test-created executable exercising the real ordinary backend/OS path. It is not a claim that an authenticated vendor CLI version or an installer was accepted.

| OS | Arch | Native lifecycle/ledger | Host backend/file fixture | Authenticated Provider/version | Packaged acceptance / admission |
| --- | --- | --- | --- | --- | --- |
| macOS | arm64 | Passed; 46 top-level tests + 62 subtests, zero failures/skips in both final toolchains | Claude fixture + Git passed; actual Web/Electron host composition passed | Not run | Full Main package pending; not released |
| Linux | arm64 | Host suite passed in a native arm64 Linux container | Claude fixture + Git passed | Not run | Not released; gate closed |
| Linux | amd64 | Cross-compilation only | Pending native CI | Not run | Unavailable |
| macOS | amd64 | Pending | Pending | Not run | Unavailable |
| Windows | amd64 | Cross-compilation only, including ACL/Job fault tests | Pending native CI | Not run | Unavailable |
| Windows | arm64 | Cross-compilation only | Pending native runner | Not run | Unavailable |

The supported local backend adapters are Claude, Codex and OpenCode. Remote/gateway modes and unmapped configurations remain rejected. Opening any actual Provider/version/OS/arch combination requires its own evidence; a provider name or compilation success does not establish support.

## Remaining gates

- Third complete `make check`; full-scope Trellis review, repository TypeScript suites and production Web/Electron host composition have already passed.
- Native Windows failure matrix and current installed artifacts; the final Linux arm64 host lifecycle/runner rerun has passed.
- Separately authorized real-Provider smoke for an initial delivery combination.
- Update final spec/plan status, commit only task-owned changes, and retain rollout disabled for unverified combinations.

Detailed local command output is retained in `/tmp/multica-host-runtime.HiUXkC/`, `/tmp/multica-host-review.meYfq8/`, and `/tmp/multica-host-frontend-{full-tests,typecheck,lint}.log`. These paths are local verification artifacts, not public runtime diagnostics.

## Final evidence locations

- `native-macos-final.jsonl` and `native-macos-go1.26.9.jsonl`: complete native matrix event streams; original evidence retained unchanged.
- `native-linux-arm64-complete.log`: final non-root native Linux host lifecycle/runner execution.
- `host-e2e-banner-final/collaboration-host-runtime-ae4a5-ly-resume-after-scoped-Stop-chromium/`: `host-runtime-evidence.json`, `build-provenance.json` and 19 screenshots; production Web build `H590KeAXOv0Kk5MKecxNY` and matching renderer hashes.
- `host-banner-red.log` / `host-banner-green.log`: the availability regression fails before the fix and passes afterwards.
- `admin-cross-package-parallel.jsonl` / `admin-cross-package-serial.jsonl`: identical 12-iteration tests fail with package parallelism 2 and pass with parallelism 1. This establishes the shared singleton interference; it does not conclusively explain every earlier empty-result/404 failure.
- `make-check-third.log`: complete final-gate output, still in progress. The check owns its process lifecycle; a tool observation timeout is not a failed run or a reason to restart it.

The composed host journey uses the real native Electron renderer harness, not packaged full Main. Its fake Claude executable does not authenticate a vendor account. Git preparation and squad composition have separate Go/DB evidence. These distinctions remain part of the acceptance record.
