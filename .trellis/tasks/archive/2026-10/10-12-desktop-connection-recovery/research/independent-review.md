# Independent connection-recovery review

Status: independent code review complete. No unresolved task-code findings.

## Findings (fixed)

- File: `packages/core/api/recovery-requests.test.ts`
- Issue: The selected request lifecycle tests covered caller cancellation before headers and timeouts during body parsing, but did not cover endpoint invalidation during a pending body or caller cancellation before a legacy identity response is normalized to unauthorized.
- Fix: Added four endpoint/body regressions (identity, config, device login, workspace list) and a canceled legacy identity 404 regression. They verify immediate cancellation, deadline cleanup, and no late credential rotation or unauthorized callback. No production code was changed by the reviewer.

- Files: `packages/core/i18n/types.ts`, `packages/core/i18n/user-locale-sync.tsx`, `apps/desktop/src/renderer/src/platform/i18n-adapter.ts`, `apps/desktop/src/renderer/src/components/connection-recovery-boundary.tsx`, `apps/desktop/src/renderer/src/App.tsx`, and their locale/boundary tests.
- Issue: A4 failed when a recovered user's language differed from the boot language: global locale synchronization reloaded the whole renderer and erased the open editor/input. The coordinator reproduced this in real Electron after the reviewer reported it.
- Fix: The implementation owners added an optional `LocaleAdapter.reload()` override, preserving the browser default. Desktop owns a stable renderer-local gate. Button/IPC opening reports synchronously, locale preference persists while editing, and only explicit Back flushes one deferred refresh. No effect/unmount cleanup flushes, persisted draft, global store, or new context was added. Reviewed the actual `UserLocaleSync`/editor StrictMode regression, immediate refresh, coalescing, and disposal tests; no further findings.

## Findings (not fixed)

- No unresolved task-code findings. The accepted A4 change was implemented by its owning agents and passed the final review/tests.
- Existing desktop lint warning in `apps/desktop/src/renderer/src/components/tab-content.tsx:54`: missing `group` effect dependency. It is unrelated to this task and was left intact; desktop lint exits successfully with zero errors.

## Verification

- Final reviewer-run core affected auth/API/initializer/workspace plus full i18n suite: 44 files / 932 tests passed, exit 0.
- Final reviewer-run complete desktop suite: 93 files / 1034 tests passed, exit 0.
- Reviewer-run core lint and TypeCheck: pass, exit 0. After the locale change, the fresh `core-locale-lint.log` / `core-locale-typecheck.log` and `desktop-locale-lint.log` / `desktop-locale-typecheck.log` were inspected and reused: all pass; desktop retains only the unrelated warning listed above. Production files did not change between those checks and this final review.
- Reviewer-run task-scoped `git diff --check`: pass, exit 0.
- Additional focused implementer evidence inspected: desktop baseline 8 files / 38 tests passed; locale integration 5 files / 27 tests passed; core locale suite 4 files / 33 tests passed.
- Scope review: real renderer/preload and production native helpers use temporary profile/config and loopback fixtures; installed native agent services are stubbed and outbound requests are gated. Candidate probing precedes old-session cleanup/reset/save. Main editor requests are authorized to main or registered live issue windows and queue until the main listener is ready. Recovery details/copy are an allowlist; editor ownership is above changing auth/workspace branches.
- Native acceptance is coordinator-owned; its final re-run was still running when code review completed. The fixture exercises timeout recovery, retained credentials, copy feedback, retry deduplication, failed validation/save, language mismatch/editor preservation, locale/narrow views, child handoff, server switch and initial workspace recovery. Final native acceptance/visual artifacts are required before declaring the whole task complete.

Raw reviewer command output: `review-core-checks.log` and `review-final-checks.log` in this directory.
