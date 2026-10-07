# Independent review: R1–R6

Date: 2026-10-07. Reviewer: dispatched `trellis-check` (`audit_check`).
Reviewed the PRD, design, implementation plan, check-context references, all task-owned production/test changes and new/updated specs. No child agents, commits, application databases, real agent accounts or external plugin endpoints were used. Unrelated working-tree changes were excluded.

## Findings (fixed)

- Files: `packages/views/iterations/iteration-page.tsx` and `iteration-navigation.test.tsx`.
- Issue: the new cached-read guards treated every HTTP status below 500 as definitive. A background HTTP 408 or 429 therefore unmounted a dirty iteration editor and lost its unsent values, despite being a retryable timeout/rate-limit response. This occurred both for detail refresh and workspace-wide invalidation through the capability/settings ancestors.
- Fix: a file-local `isDefinitiveReadError` predicate retains the previous behavior except for 408/429, and is reused by the three existing guards. Cached input remains mounted with the existing refresh affordance; authentication, authorization and resource-deletion responses still hide it. The form's imperative-refresh branch already retained these failures and needed no change. No command replay, permission-policy or public interface changes.
- Regression: extend the existing mounted navigation test with detail/workspace 408 and 429 cases. Before the fix, all four new cases failed because the Name input was absent; both existing network-error controls passed. Afterward, the canonical form/navigation/project-detail suites passed 51/51, including existing deletion/access-denial controls.
- Spec/report follow-up: the integration owner explicitly recorded HTTP 408/429 under the transient-read contract in `iteration-operations.md` and the client implementation record.

## Findings (not fixed)

No unresolved functional finding in R1–R6. The review leaves the following existing or excluded boundaries unchanged:

- Changed-file ESLint reports two existing hook-dependency warnings in `project-detail.tsx` (lines 150 and 255). They predate this task's identity-key edit; broad hook changes are outside this review's scope. Root lint has 30 existing warnings and zero errors.
- `webSecurity` remains disabled, as the reviewed design explicitly requires. The native checks establish the reported local-file boundary, not general origin/CORS isolation. A renderer-protocol/CORS migration remains separate work.
- Existing duplicate resource rows are not repaired, and explicitly supplied old full execution refs retain their existing compatibility semantics. The implemented contract protects newly serialized writes and omitted fields.

## Acceptance review

| Requirement | Evidence and assessment |
|---|---|
| R1 / AC1 | Production `loadRenderer` installs the native file policy before main/issue loads, alongside the existing navigation guard. One WeakMap entry owns each session listener; destroyed WebContents registrations are removed; every request compares its live attached frame with the current main frame. No other production `onBeforeRequest` owner is displaced. Independently ran the native script: Electron 39.8.7, 16/16 groups passed. Actual inline, attachment and full-page components deny local fetch/XHR/script/image/style/nested-frame/worker probes, cached-file reuse and navigation escapes; trusted renderer/module/worker/preload, interactive HTTP, image/PDF and independent header-handler controls passed. |
| R2 / AC2 | Bridge Auth precedes `RequireHumanActor`; `pluginSessionCaller` rejects machine provenance before installation/member lookup. Dedicated plugin bearer dispatch remains separate. Router tests use real task/cloud/PAT/JWT/cookie authentication, valid scoped installations and requests, both auth modes, same/cross-workspace ownership, forged header controls, unchanged issue/storage/comment/invocation state and zero denied hook transport. Permitted human and installation/callback attribution controls pass; removed alias remains absent. Direct absent-service backstop tests prove early denial. |
| R3 / AC3 | Baseline values and revision share state. Dirty fields retain their original revision and submit only changed mutable fields. Explicit comparison/rebase preserves untouched remote fields and requires a later Save. Successful writes wait for protected authoritative resource reads; failed refreshes lock further writes and offer read retry. Exact uncertain-command identity stays in `useIterationCommand`; stale-session/access fences remain in protected queries. Form and parent recovery tests cover these paths, including the review's 408/429 correction. |
| R4 / AC4 | Stable workspace/project key is at the actual overview call site. The canonical detail test uses real Query cache, preloads A and B, navigates while A's editor emission is still pending, verifies A-bound unmount storage, B-only draft/preview payload and return-to-A restoration. Editor double models its existing mount/debounce/unmount contract; unrelated tab/store behavior is unchanged. |
| R5 / AC5 | Both standalone writers use the existing READ COMMITTED transaction runner and exclusive project lock before separate fresh resource-set/row reads. Conflict checks use transaction-bound queries. Tests use distinct members/connections and observed direct/transitive PostgreSQL lock dependencies before releasing the fixture lock. Create/create, update/update and create/update produce one winner and one conflict; different-daemon controls preserve distinct append positions. |
| R6 / AC6 | Partial ref/label/position merge is recomputed inside each locked transaction attempt, retaining unknown stored JSON, explicit/null label semantics and old-client rename exemptions. Concurrent worktree ref plus label/position tests preserve both intents. Existing rename, capability, membership/association and execution-snapshot suites pass. Commit tests verify parent-lock lifetime, no pre-commit publication, exactly one success event and rollback/no events on write or commit failure. |
| AC7 | Inspected lane records and available native/backend red logs: original file leaks, elevated machine operations, stale-resource races and backstop lookup failures were substantive failures. New green backend regressions contain real PASS events with no DB-suite skips. Frontend lane records retain original red/green results. Reviewer recorded and reran the additional transient-response red/green regression. Verification limits below are explicit. |

Specs for renderer access, plugin principal boundaries, resource transactions, iteration drafts/recovery and project overview lifetime match the implementation. No schema, generated SQL, success DTO, dependency or template update is required by these changes.

## Verification

### Reviewer-executed checks

| Command | Result |
|---|---|
| `node apps/desktop/scripts/verify-renderer-file-access.mjs` | PASS, exit 0; Electron 39.8.7, 16/16 groups. Log: `/tmp/multica-independent-native-review.log`. |
| `pnpm --filter @multica/views exec vitest run iterations/iteration-navigation.test.tsx -t transient --maxWorkers 2` before guard fix | Expected FAIL, exit 1; four 408/429 failures, two network-error controls pass. Log: `/tmp/multica-independent-transient-red.log`. |
| `pnpm --filter @multica/views exec vitest run iterations/iteration-form.test.tsx iterations/iteration-navigation.test.tsx projects/components/project-detail.test.tsx --maxWorkers 2` after guard fix | PASS, exit 0; 3 files / 51 tests. Log: `/tmp/multica-independent-clients-green.log`. |
| `pnpm --filter @multica/views typecheck` after guard fix | PASS, exit 0. Log: `/tmp/multica-independent-views-typecheck.log`. |
| `pnpm --filter @multica/views exec eslint iterations/iteration-form.tsx iterations/iteration-form.test.tsx iterations/iteration-page.tsx iterations/iteration-navigation.test.tsx projects/components/project-detail.tsx projects/components/project-detail.test.tsx` | PASS, exit 0; zero errors, two existing warnings. Log: `/tmp/multica-independent-views-lint.log`. |
| `git diff --check` | PASS, exit 0 after review edits. |

### Inspected integration/lane evidence

- Root typecheck/lint: 15/15 successful tasks, 10 cached, 30 existing warnings, zero errors (`/tmp/multica-audit-final-static.log`).
- Root TypeScript tests: 876 files / 10,245 tests, 5/5 successful tasks; only docs cached (`/tmp/multica-audit-final-tests.log`). These broad checks preceded the review's 408/429 change; the final affected tests/typecheck/lint above cover that change.
- Backend guarded race runs: 117 new-regression PASS events, 147 neighboring handler PASS events, 116 router PASS events, 18 service PASS events and 99 final real-router PASS events, zero failures or DB skips in those runs. Reviewed the corresponding `/tmp/multica-audit-{backend-final,handler-neighbors,router-final,service-final,plugin-verified}.log` files. Exact commands and disposable-DB preflight are in `backend-implementation.md`.
- Backend middleware run: 25 PASS events and two optional Redis skips (`TestAuth_PATCacheHit`, `TestPluginRateLimitIsPerCredentialAndUsesStableProblem`); no Redis executable/test URL was available. No shared Redis database was used.
- `go -C server vet -p 2 ./...`: backend owner recorded exit 0; inspected empty diagnostic log `/tmp/multica-audit-vet.log`.
- Backend/database checks were not redundantly rerun by the reviewer. The private database was `audit_backend_a096cd642bf243da`; the integration owner reported dropping it after confirming zero connections and verifying its absence. Aggregate integration outcomes are recorded in `remediation-verification.json`.

### Limits

Native acceptance ran on macOS with installed Electron, temporary profile/files and loopback fixtures. Windows/Linux, installer/ASAR packaging, full production preload/auth/daemon startup and a new full browser UI run were not exercised. The full repository Go suite was not run; focused affected and neighboring race suites plus repository Go vet were used. Session-local log paths are supporting evidence; durable outcomes are recorded here and in the lane reports.
