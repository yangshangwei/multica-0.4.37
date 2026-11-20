# Research: S3 field-specific concurrency and original-request recovery

- Query: identify the smallest missing S3 R04 / parent TC-16 verification for priority versus priority, priority versus handler transfer in both real commit orders, retained original keys, and an ordinary linked child's explicit numeric CAS.
- Scope: internal source, specs and retained acceptance artifacts. Only this research file was written; no code, task state, Git, database, API, browser or Go command was executed.
- Date: 2026-11-18

## Findings

### Existing coverage makes a narrow test increment sufficient

The current product already routes priority changes through the collaboration workflow and ordinary assignee/priority changes through the ordinary atomic issue writer. The remaining gap is field-specific composed evidence, not a demonstrated missing CAS implementation.

`editParams` in `server/internal/service/collaboration_edit_test.go:24` sets both title and priority (`urgent`). Consequently, `TestCollaborationEditAndTransferSerializeAtCommit` at line 304 already exercises priority as part of its edit, uses the real commit barrier, and checks both transfer-first and edit-first orderings. Its final checks at lines 353–360 assert title, handler and edit-history count, but omit priority and exact original-operation receipts. Do not describe this as an entirely untested lock path or copy its full generic matrix merely under a new name.

`TestCollaborationEditConcurrentIdentityAndRevision` at line 266 has same-key and different-key concurrency, but only changes the two competing titles. Priority remains the same `urgent` value, so the current test does not identify which priority input won or prove different-priority original-key conflicts.

The ordinary `TestConcurrentRevisionWritesHaveExactlyOneWinner` in `server/internal/handler/issue_revision_test.go:308` uses `title_base` and `content_base`, not a linked child and explicit numeric `expected_revision`. It cannot certify the missing ordinary-child field pair. Its legacy open-coded fixture/recorder pattern should not be copied into new tests.

### Production owners and refusal order

| Concern | Actual current owner | Required assertion |
| --- | --- | --- |
| Collaboration priority write | `CollaborationService.EditIssue`, `service/collaboration_edit.go:35`, then `prepareFieldEdit`, `UpdateCollaborationIssueFields`, `AdvanceCollaborationEditRevision` | Winning priority only; one ordinary and collaboration revision advance; exact `issue_edited` event/receipt |
| Collaboration handler change | `CollaborationService.Transfer`, `service/collaboration_workflow.go:128`, then `changeHandler` and `finish` | Winning handler only; one `transfer` handoff/event; no session execution changes |
| Collaboration authorization and CAS | `workflowInTx`, `service/collaboration_workflow.go:461`; current role at lines 563–589, epochs/revision at 590–609 | Current role refusal precedes stale-version refusal; never require every loser to return 409 |
| Ordinary child priority/assignee | `Handler.UpdateIssue`, `handler/issue.go:4232`; `updateIssueAtomicallyOnce` at 3936; `UpdateIssue`, `queries/issue.sql:159` | One 200 and one exact `revision_conflict` 409 for the same explicit numeric revision; only winning field changes |
| Original collaboration result | `GetWorkflowOperation`, `service/collaboration_workflow.go:693`; HTTP `GetCollaborationOperation`, `handler/collaboration_workflow.go:394` | Actor-owned content-free historical receipt; same status/code/version after later successful action |

Use a former noncreator handler to expose the authority-first branch. In transfer-first order, the old handler's subsequent priority edit returns **403 `forbidden`**, and priority remains unchanged. In priority-first order, the same handler's old-revision transfer returns **409 `stale_revision`** while handler identity remains unchanged. `workflowRoleAllowed` at `collaboration_workflow.go:375` uses `Capabilities.Edit` and `Capabilities.Transfer`, separately from creator control rights.

The existing workflow acquires the current account set, source workspace/catalog/I1 fences, issue, collaboration, session and target locks, then performs a fresh access read before deciding role/epoch/revision. Reuse those paths and the existing test wrappers; do not inject precomputed access or replace PostgreSQL responses.

### Original-key recovery has an important authority boundary

`workflowInTx` checks current role even when it finds a previous operation (`collaboration_workflow.go:565–580`). An old success or refusal POST cannot restore a former handler's new-write authority. After a real handler transfer, do not expect an old priority/transfer POST from that former handler to replay as 200/409 merely because its key exists.

Instead, use `GetWorkflowOperation` to recover that actor's own original result, which explicitly requires current human account validity but no current participation/source membership (`collaboration_workflow.go:690–742`). Snapshot the stored operation's ID, payload hash, status/code/version and result JSON before later actions, then compare after GET recovery. The minimal receipt must not disclose title/body/priority/handler/queue/history.

Before responsibility changes, same-key/same-priority concurrency can return the same success; same-key/different-priority must return `idempotency_conflict`. After a loser explicitly rereads and submits a fresh authorized request, the old rejected request remains historical. A creator remains authorized through transfer and can separately exercise POST-replay controls if needed; do not confuse that with a former-handler recovery claim.

### Reuse available fixture and barrier machinery

| Existing file/helper | Use |
| --- | --- |
| `service/collaboration_test.go:30`, `newCollaborationFixture` | Independent creator and external handler, actual source membership separation and scoped `testutil.Fixture` cleanup |
| `service/collaboration_queue_test.go:34`, `newCollaborationQueueFixture` | Created issue/session/target and original request context; `request` at 48 keeps explicit lifecycle/account/participant epochs |
| `service/collaboration_workflow_test.go:22`, `workflowRequest` | Same observed collaboration revision with separate request IDs |
| `service/collaboration_lifecycle_test.go:592`, `lifecycleCommitBarrierStarter` | Holds actual transaction immediately before commit and supplies real PostgreSQL holder PID |
| `service/collaboration_workflow_test.go:454`, `waitWorkflowBlocked` | Observe a real follower through `pg_blocking_pids` before releasing the winner |
| `handler/collaboration_boundary_test.go:19`, `collaborationBoundaryIssue` | Actual collaboration parent for an ordinary linked child |
| `handler/collaboration_stage_test.go:34`, `stageHTTPFixture` | Alternative fully isolated workspace/parent plus `fx`, current source actor and recipient; keep stage policy off for ordinary-child CAS |
| `handler/collaboration_stage_exit_continuation_test.go:414`, `stageExitHTTPBarrierStarter` | Generic `beforeCommit=true` branch supplies real PID via `atomic.Uint32`; reuse rather than add another transaction wrapper |
| `handler/collaboration_stage_exit_continuation_test.go:479`, `stageExitHTTPWaiter` | Database-scoped actual follower-wait observation using the real holder PID |
| `internal/testutil/db.go:174`, `Fixture.Issue` and `Fixture.Member` | Ordinary child, distinct valid source member assignee and fixture-owned cleanup |
| `internal/testutil/http.go:93`, `Call(...).Want(...).JSON(...)` | Actual handler responses; `newRequest`/`WithURLParams` retain the established auth/workspace context |

For a cloned first ordinary handler, set its `TxStarter` to the existing barrier. `UpdateIssue`'s atomic transaction calls that field (`issue.go:3940`). Use the second unchanged handler for the losing request. Every blocked-writer test must release the barrier during cleanup even on failed assertions, and collect both completed requests. Do not alter default pools, lock budgets or timeouts to force success.

### Recommended bounded implementation ownership

Suggested two **new** files, leaving the existing canonical tests and their WIP bytes untouched:

1. `server/internal/service/collaboration_recovery_cas_test.go`:
   - `TestCollaborationRecoveryPriorityConcurrentIdentity`: priority-only same-key/same-payload convergence, same-key/different-priority conflict, and distinct-key/same-revision priority competition. Assert the winning value, one event and exact saved original requests; reuse the current fixture and barrier.
   - `TestCollaborationRecoveryPriorityTransferCommitOrders`: priority-only edit versus actual transfer, both commit orders, exact authority403 versus stale409, unchanged losing field, then fresh authorized continuation and actor-owned GET recovery of the original success/refusal. Reuse `lifecycleCommitBarrierStarter` / `waitWorkflowBlocked`.
2. `server/internal/handler/collaboration_child_recovery_cas_test.go`:
   - `TestCollaborationOrdinaryChildPriorityAssigneeCASCommitOrders`: an actual ordinary child under a collaboration parent; priority-only versus assignee-only PUT from the same numeric `expected_revision`, both commit orders using existing handler barrier/PID helpers. One 200, one409 with `code=revision_conflict`, `resource_type=issue`, exact child ID, expected revision and actual winning revision. An explicitly fresh request can apply the remaining field once. Parent collaboration revision/history/session remain unchanged; child refresh hints are separately allowed committed projections.

A smaller alternative is to strengthen the existing two collaboration tests' assertions and add only the ordinary-child file. That changes fewer lines but touches existing canonical WIP. Main session should choose ownership after its recorded dirty/source boundary; neither alternative requires product code, SQL, migration, dependency or new retry infrastructure unless a regression actually fails.

### Effects to compare, without erasing legitimate delivery markers

- Collaboration `issue.priority`, `assignee_type/id`, ordinary revision; `issue_collaboration.revision`, lifecycle/authority epochs and active information pointer; session queue/control/event sequence.
- Exact winning edit event ID, actor and request ID; before/after priority in its payload (`collaboration_edit.go:145–168`); exact transfer handoff and corresponding event identity. Priority-only metadata changes do not manufacture I1 events (`TestCollaborationEditRecordsIterationTitleWithoutDateFacts`, edit test line 439).
- Per-key `collaboration_operation` IDs/payload hashes/result JSON. Refusals have their own saved result without a winning business event.
- Existing command/group/run/grant/target identity and state, plus `agent_task_queue` count. Seeded run rows are test fixtures and must not be described as actual native execution.
- Ordinary child winning fields and revision, unchanged parent collaboration/session/participants, and winner-only committed refresh hints. Asynchronous outbox delivery state is not a losing business write and must not be included in an indiscriminate whole-row zero-change assertion.

### Source boundary checked in this research

Retained final S2 acceptance: `../11-13-pay-241-s2-stage-approvals/research/r33-full-s2-a01-a12-independent-check-20261118.json`, status `ACCEPTED_FULL_S2_CODE_AND_SYNTHETIC_A01_A12_READY_ROOT_BOOKKEEPING_PENDING_COMMIT`.

I compared the current bytes of six relevant files against the retained R33 backend manifest `/Volumes/artisan/.multica-acceptance/pay241-r33-policy-wait-20261118-hc2gedj3/build-final-r01/source-after.json` (5,056 entries; manifest SHA256 `dfb1afa63f96cf419527cb77fa85bdc0a632607c8a612ce5fa1c6fdc5280bf84`). All six matched:

- `server/internal/service/collaboration_edit.go`
- `server/internal/service/collaboration_workflow.go`
- `server/internal/service/collaboration_edit_test.go`
- `server/internal/service/collaboration_queue_test.go`
- `server/internal/handler/issue.go`
- `server/pkg/db/queries/issue.sql`

This is six-file byte equivalence, not a current whole-ROOT/HEAD, mode, dependency-graph or build certification. The coordinator must bind its actual isolated verification source/configuration separately before test execution.

### Verification and review checklist

1. Bind the accepted S2 source and the narrow added tests; prove other product/SQL inputs retain their bytes/modes. Use four fresh coordinator-owned service-normal, service-race, handler-normal and handler-race databases with matching migrations and recorded identity. No reuse of prior failed or runtime databases.
2. Format the changed Go tests; run compile-only checks and appropriate package vet in the private bound environment. Run only the approved exact new test inventory in normal and `-race` modes, preserving default pool and timeout behavior. Test-name filters do not isolate handler `TestMain`.
3. Parse actual Go JSON starts/terminals: each expected top and child identity must terminate once; normal/race sets equal, no skip, failed, missing, duplicate or unfinished tests. Record complete failures and issue a new run after any necessary correction.
4. Independently review the writer barriers, numeric-versus-decimal wire, authority-first branch, immutable receipts, winning-field assertions and fixture cleanup. Prove fixture rows, connections and singleton creation flags were restored in their own databases.
5. Treat the result as an S3 R04 service/HTTP increment. No browser/installed Desktop, full S3 or parent TC-16 PASS claim until the required current API/Web/Desktop combined evidence exists. Other S3 requirements and real S4/S5 remain open.

## Related Specs

- `.trellis/spec/server/collaboration-foundation.md`: creator/handler roles, current account fences, original operations and metadata semantics.
- `.trellis/spec/server/collaboration-source-editing.md`: source versus object authority, actual writer barriers, original receipts and current source projections.
- `.trellis/spec/server/collaboration-stage-approvals.md`: source-member fences and separation between stage authority and ordinary children.
- `.trellis/spec/server/database-test-isolation.md`: package-specific databases, explicit identity, `TestMain`, source/state isolation and connection budgets.
- `.trellis/tasks/11-13-pay-241-s3-recovery/{prd,design,implement}.md` and `research/recovery-gaps.md`: reviewed requirement boundary and ordinary optional-CAS compatibility.

## External References

None. All findings use repository source, specs and retained local S2 evidence.

## Caveats / Not Found

- No product defect was reproduced; no verification command, database or runtime was executed.
- `ExpectedRevision` remains optional in ordinary input (`handler/issue.go:3426`) and SQL (`queries/issue.sql:204,253`). This increment proves explicit numeric CAS, not mandatory blind-write rejection for old clients.
- Current task creation and metadata history, native recovery, mixed notifications, uploaded Agent outputs and structured blocker information are separate requirements; this two-file proposal does not implement them.
- The existing transfer/priority lock path already has partial composed coverage; use the missing specific assertions rather than add duplicate broad matrices.
