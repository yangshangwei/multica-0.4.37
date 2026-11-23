# Independent review: human workflow and account invalidation tranche

Date: 2026-10-13. Reviewer: dispatched `trellis-check` (`workflow_final_check`).

Verdict: **FAIL pending the legacy agent principal boundary below.** This is a
bounded review of invitation/ordinary transfer/removal/leave, their HTTP and core
contracts, account invalidation migrations 613–616 and the common credential
revocation callers. It is not acceptance of full P2/P4/P5. Concurrent lifecycle
implementation, invalidation fanout, real promotion/launch and native termination
remain outside this review.

## Findings (fixed)

No production code was changed by this reviewer. The following earlier review
finding was fixed by the main agent and independently checked here:

- File: `server/internal/service/collaboration_queue.go`,
  `server/pkg/db/queries/collaboration.sql`,
  `server/pkg/db/queries/collaboration_workflow.sql`.
- Issue: author departure must not classify a possibly launched execution as
  an unstarted cancellation or overwrite a previous forced termination root.
- Fix checked: `stopRun` evaluates the original locked run facts before writing
  `ending`. Cancellation is retained only for the server-proven never-launched
  case; otherwise it requests `stopped`. Both stop/cancellation SQL writers keep
  an existing forced disposition and reason. No permit/started/progress/outcome
  contradiction can settle via the server shortcut. The run fence and nonce
  remain intact, while claim generation is revoked. This matches D06/E03/E05.

## Findings (not fixed)

### High: legacy agent requests are classified as human workflow callers

- Evidence: `server/internal/middleware/auth.go:66`, `:203`, `:239` and `:279`
  stamp password and legacy JWT/PAT requests solely by credential kind.
  `server/internal/collaboration/access.go:22` treats every unrestricted JWT/PAT
  principal as human. `server/internal/handler/collaboration_workflow.go:161`
  forwards that context to the action service without checking legacy actor
  identity; `collaboration_boundary.go` uses the same classification for private
  content projection.
- The existing application recognizes valid paired `X-Agent-ID` and `X-Task-ID`
  as an agent caller even when authenticated with a member JWT/PAT
  (`server/internal/handler/handler.go:901–963`). The new context omits that
  distinction. A legacy agent call using an authorized participant's credential
  and a valid agent/task pair therefore receives that participant's human
  invitation/transfer/removal authority, and can receive the full collaboration
  read projection. No body spoofing or unknown credential kind is required.
- Contract: design D03 explicitly rejects legacy agent actors impersonating
  human confirmation. `research/cross-space-authority-design.md:67` identifies
  this exact PAT/JWT plus legacy actor case. R53 requires ordinary machines to
  receive only minimum references. This is distinct from the passing tests for
  actual task/cloud tokens and unknown principals.
- Recommendation: preserve a deny-only legacy-agent signal in the trusted
  authentication context, or apply an equivalent shared collaboration guard,
  covering account reads, workflow actions, operation recovery and source-side
  projections. Keep ordinary human JWT/PAT behavior and other API contracts.
  Add a focused real-router regression with a valid JWT/PAT and legacy
  agent/task pair, alongside the ordinary human control. Never grant authority
  based on a client actor header.
- Not fixed here because the dispatch restricts this reviewer to this report;
  the authentication integration and regression belong to the main agent.
  The finding was sent to the main agent before closing this review.

## Verified behavior

- The actor's current user, participant, account and lifecycle versions, role
  and collaboration revision are checked under the shared transaction locks
  before the workflow callback. User lock sets are sorted and rechecked after
  the source lock; a newly discovered identity causes rollback/retry.
- Invitation directly enrolls the target without changing responsibility.
  Ordinary transfer changes the persisted handler, iteration audit and handoff
  facts without changing session/run state or granting a personal runtime.
  Removal/leave retain other authors' waiting commands; removal of a handler
  returns responsibility to the creator atomically.
- After leaving, the real actor can recover only their own stored operation's
  status/code/revision. HTTP deliberately exposes just code/revision. Recovery
  does not call the issue response builder or expose title, content, note,
  handler, target, queue or run fields. An unrelated actor cannot recover it.
  Current source membership remains an independent basic read/comment basis.
- Security writers call the same borrowed-transaction revocation helper with
  stable password-version or admin-operation keys. Fence advancement and a
  durable invalidation insert precede the no-runtime early return; failure
  rolls back credential/account changes. The LEFT JOIN preserves legacy users
  with no password row. Ordinary login/logout/single-token operations do not
  enter this global invalidation path.
- Migration indexes are single-statement CONCURRENTLY builds; no foreign keys
  are introduced. All four down migrations hold an exclusive NOWAIT table lock
  and refuse retained invalidations, including reconciled rows. Admin fixture
  schemas clone both new security tables, and the issue helper clones the
  relevant command/write tables rather than writing through `public` fallback.
- Core preserves the original key and exact decimal versions, drops ambient
  workspace headers, applies strict redacted response parsing, and discards
  responses superseded by account/server changes. Missing write context does
  not manufacture epochs. No new dependency or execution fallback is present.
- The main agent updated `collaboration-foundation.md` during this review to
  describe the workflow and actual invalidation writer integration, while
  retaining the explicit unfinished fanout/native-protocol limitations.

## Verification

- Lint: **pass**, independently ran
  `pnpm --filter @multica/core exec eslint api/client.ts api/collaboration-workflow.test.ts collaboration`
  (exit 0).
- TypeCheck: **pass**, independently ran
  `pnpm --filter @multica/core typecheck` (exit 0).
- Tests: **existing tranche suites pass**, inspected main-agent evidence;
  database execution was intentionally not duplicated while the main agent
  owned the task database lane:
  - `workflow-service-final.json`: 110 passing test/subtest events, zero failed
    or skipped events, package pass in 48.111s after the stop fix.
  - `workflow-handler-final.json`: 108 passing test/subtest events, zero failed
    or skipped events, package pass in 33.335s.
  - `workflow-router.log`: both account/workflow real-router tests pass.
  - `workflow-migrations.log`: four migration/schema checks pass.
  - `workflow-core-final.log`: 36 files and 904 tests pass.
  These logs are in `/tmp/multica-collaboration.Qp1TA6/`; the main agent confirms
  the final service run used `-race`. The legacy actor regression above is
  missing, so passing existing tests does not close that finding.
- Changed files in this review: this report only. No code simplification,
  migration execution, database mutation, commit or new test matrix was made.
