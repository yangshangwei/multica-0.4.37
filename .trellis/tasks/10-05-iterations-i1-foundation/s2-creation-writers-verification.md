# S2 creation and T1 writer evidence

Date: 2026-10-06. Branch: `codex/projects-p1`. This slice covers W06–W11
under the existing FG contract; it does not enable I1 or implement LG membership.
The isolated database was `multica_i1_create_20261006b` on local PostgreSQL,
fully migrated by the integration owner before these runs.

## Implemented boundaries

| Writer | Production entry and evidence |
|---|---|
| W06 | Owning `createOnce` takes workspace KEY SHARE → subscriber → active member before `CreateInTx` takes catalog/I1 and any source issue/comment, project, labels, counter, or insert. Member creation uses its explicit CreatorID; agent creation requires transport-supplied `IssueCreateOpts.ActorUserID`, never inferred agent ownership. Ordinary `Create` retains its existing four-attempt 55P03 owner. Borrowed T1/lifecycle owners take their member/catalog/I1 fences before owned rows; reacquisition in `CreateInTx` is reentrant. |
| W07 | `dispatchCreateIssue` resolves the manual actor or the existing immutable trigger creator, then takes workspace → subscriber → active member → catalog/I1 before project/autopilot locks. It uses the refreshed assignment, SHARE NOWAIT squad/agent locks and any required public-to invocation grant locks, then rechecks the existing permission policy through qtx. Missing/revoked principals are skipped with `InvocationNotAllowed`; no autopilot-owner fallback is introduced. A real concurrent `DeleteSquad` no longer creates an archived squad assignment. |
| W08 | Both legacy onboarding transactions use workspace → subscriber → active member → catalog → I1 before agent creation or issue reuse/creation. Existing endpoint responses, idempotent guide reuse and enqueue behavior remain covered. |
| W09–W11 | `beginTriageWrite` preserves workspace → T1 settings → subscriber/member, then takes catalog/I1 before import rows, attachments, issues or dispatch references. This also covers the second T1 execution transaction. The redundant late action-level catalog acquisition was removed. |
| T1 facts exemption | Pending/rejected/duplicate rows cannot legitimately have a current iteration. Review now refuses a persisted nonnull pointer with 409 before any mutation. Normal intake, import, acceptance, rejection and reopening create no participation/event or scope change. Unknown `fields.current_iteration_id` remains explicitly rejected by the existing acceptance allowlist; LG will add confirmed membership atomically later. |

Creation omitting iteration fields keeps the database defaults: no current
iteration and no participation/event. This is an evidenced no-event case, not a
claim that explicit join creation or accept-and-join has been implemented.

No new transaction owner, retry loop, isolation override, dependency, migration
or SQL/generated change belongs to this slice. No recorder is called for a
new unassociated issue or for T1 intake that cannot have formal membership.
The lifecycle outer owner also rechecks and locks the current request member
before catalog/I1. Source-context errors map a revoked creator to HTTP 403.
The lifecycle source's metadata changes remain outside iteration statistical
facts; its new follow-up does not inherit source membership.

## Test-first evidence

Before production changes, the focused real-database run failed as expected:

- All ten owner cases completed HTTP 200/201 despite another transaction holding
  the I1 fence: ordinary create, autopilot trigger, lifecycle follow-up, both
  onboarding endpoints, T1 intake/import/accept/reject/reopen.
- Source-context creation likewise completed while the I1 fence was held.
- Five malformed T1 membership cases returned 200 rather than 409:
  accept/reject/duplicate/reopen/snooze.
- `DispatchAutopilotManual` paused before its issue transaction, followed by a
  real `DeleteSquad`, created and enqueued the stale squad assignment.
- Review then identified missing current-member checks: ordinary and lifecycle
  creation still returned 201 after revocation won; manual and scheduled
  autopilot dispatch each created an issue after their principal was removed.
  All four cases now refuse the write after a real PostgreSQL lock wait.
- Before reference locking, another transaction could NO KEY UPDATE both the
  squad and leader while autopilot creation paused before commit. Both probes
  now receive 55P03; the original reference values remain stable through commit.
- The unsupported explicit T1 iteration field already returned 400 and retained
  pending/unassociated state; that assertion is a compatibility regression,
  not a newly failing behavior.

After the changes, the owner tests hold I1 on a second connection, observe the
writer's fence attempt, and successfully take issue, attachment, agent and
project NOWAIT locks before releasing the writer. T1 cases additionally probe
the intake issue/import rows. Committed writes preserve the one existing
participant and unchanged iteration event count/scope revision. The source
capture test probes both source issue and comment locks. The dispatch/delete
race now creates an agent-assigned issue and one normal enqueue.

The first broad run exposed an obsolete **test barrier**, not a production
deadlock: `TestLifecycleAtomicSourceContextCreateLockOrder` paused a creator
holding source/I1 and waited for the competing lifecycle writer to reach the
source NOWAIT lock. The new correct order waits at I1 first. With integration
owner approval, this existing test's synchronization changed: PostgreSQL
`pg_blocking_pids` now proves the actual current-actor fence wait (before I1)
before the creator is released.
The original successful response and exactly-one-child assertions remain.
The existing workspace-deletion test also used a goroutine-start signal that
allowed deletion to win during preflight admission, sometimes returning 409.
Its approved synchronization fix now observes the actual workspace lock wait,
retaining its original 404/400 and no-orphan assertions.

## Verification results

The commands below define the focused checks used for this slice; the final
results and the one superseded local failure are recorded immediately below.
The final service, root integrated handler, build, vet and diff checks all
completed with exit 0. Agent CLI guard was enabled for every substantive test
run; no real agent CLI ran.

```sh
DATABASE_URL='<isolated database URL>' bash scripts/go-test-with-agent-cli-guard.sh -- \
  go -C server test -race -p 2 -parallel 2 ./internal/handler \
  -run 'Test(CreationIteration|TriageIteration|SourceContextCreateIteration|AutopilotCreation|Triage|LifecycleAtomic|BootstrapOnboarding|CreateIssue|ProjectAssociation|CommentSourceContextLifecycle|IssueCreate|WriteSourceContextError|AgentCreateOriginator)' \
  -count=1 -json

DATABASE_URL='<isolated database URL>' bash scripts/go-test-with-agent-cli-guard.sh -- \
  go -C server test -race -p 2 -parallel 2 ./internal/service \
  -run 'Test(ProjectDeletionStopsStaleAutopilotDispatch|ProjectAutopilot|IssueCount|IssueCreate|CreateIssue|CreateMediaGatedIssue|AutopilotDispatch|AutopilotQuota|TriageAdmission|ResolveAutopilot)' \
  -count=1 -json

git diff --check
go -C server vet ./internal/service ./internal/handler
go -C server build ./internal/service ./internal/handler
```

- Initial focused handler pass: **249 passed, 0 failed, 2 skipped**, including
  subtests; 17.354 s, before the subsequent current-authorizer correction.
- **Final integrated handler run by the root owner: 395 passed, 0 failed,
  2 skipped**, including subtests; 18.938 s. The final JSON was inspected here:
  all new authority/reference cases, both updated lifecycle barriers and
  `TestCommentSourceContextLifecycle` passed. This final run supersedes the
  earlier focused evidence for the final product code.
  Skips are existing opt-in `TestTriageQueueScaleBaseline`
  (`MULTICA_TRIAGE_PERF=1`) and `TestTriageWireFixtures` (fixture export).
- Final focused service: **133 passed, 0 failed, 0 skipped**, including subtests;
  6.528 s, after all current-principal/reference changes.
- Existing rollback/replay/concurrency coverage includes T1 audit failure,
  every required lifecycle write failure, source-context capture, concurrent
  review/import dedup, P1 project dispatch/deletion and create issue limits.
- A preceding local handler run observed a source-context cleanup count of 1
  where the existing test expects at least 3. No production change followed;
  the root's final isolated integrated run passed that exact full test. This
  observation is retained rather than silently reporting every local run green.
- Local raw logs: `/tmp/i1-creation-red.log`,
  `/tmp/i1-creation-authorization-red.log`,
  `/tmp/i1-creation-reference-red.log`,
  `/tmp/i1-creation-autopilot-auth-red.log`,
  `/tmp/i1-creation-handler-final.json`,
  `/tmp/i1-creation-service-final.json`. Authoritative final integrated handler
  evidence: `/tmp/i1-s2-final-handler.jsonl`.

## Remaining integration work

- Root owns generic HTTP explicit iteration-field confirmation/response
  compatibility in `handler/issue.go`, and integrated ActorUserID plus 403
  mapping there. This slice does not edit that file.
- Current workspace-member/principal protection is completed here. Ordinary
  HTTP agent creator autonomy and originating-task identity validation still
  follow their existing preflight checks; comprehensive transaction-bound
  revalidation of those grants remains a FG authorization audit item. Passing
  originator regressions does not prove every possible revoke race.
- Confirmed create-and-join and T1 accept-and-join remain LG work and must not be
  advertised by capability responses before their full transaction is ready.
- Full FG lock/throughput benchmarks, all-package checks, frontend checks,
  browser/Electron E2E and remote CI are not certified by this focused slice.
- Existing autopilot dispatch persists issue creation before task enqueue;
  these edits preserve that behavior and do not claim creation+enqueue atomicity.
  The independent `run_only` lifecycle is not a W07 issue-creation writer.
