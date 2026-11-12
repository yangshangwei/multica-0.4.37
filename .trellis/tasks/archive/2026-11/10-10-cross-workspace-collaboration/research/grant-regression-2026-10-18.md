# Ordinary grant concurrency under configuration invalidation locks

The reported failures were reproduced serially against the task-owned
`collaboration_squad` database through migration 726. They were outdated test
assumptions, not permission bypasses. Only the three test files below changed;
production handlers, API contracts, SQL, applied migrations and retry budgets
remain unchanged.

## Actual lock contract

Migration 704's `collaboration_advance_child_resource_config` takes the parent
agent `FOR UPDATE NOWAIT` on every invocation-target insert/delete, then advances
its configuration epoch in the same transaction. `UpdateAgent` now also keeps
its agent update and invocation-target replacement in one borrowed resource
transaction. The former gap between committing the agent and replacing grants
does not exist.

Ordinary assignment, triage acceptance and project evidence take the agent's
shared lock before locking its grant rows. A later insert therefore receives
55P03 while authorization holds that agent. A prestarted revocation conflicts
at the agent locking read, before the grant read the old tests awaited.

## Every affected test identity

| Test | Cause and retained evidence |
| --- | --- |
| `TestIssueAssignmentIgnoresGrantInsertedAfterLockRead/create` | The late insert now correctly fails with 55P03. The original request stays 403 with an identical issue/counter/audit/dispatch snapshot and no inserted grant. A grant committed after that transaction permits a fresh create (201). |
| `TestIssueAssignmentIgnoresGrantInsertedAfterLockRead/update` | Same parent lock exclusion and unchanged denied snapshot; only the fresh request after grant commit succeeds (200). |
| `TestIssueAssignmentIgnoresGrantInsertedAfterLockRead/lifecycle` | Same exclusion and 403/no-side-effect guarantee; only a fresh authorized handoff succeeds (201). |
| `TestProjectUpdateEvidenceGrantRevocationRestartsRRAuthorization` | The previous hook waited for a grant read that could not be reached while revocation owned the agent. The hook now observes actual 55P03 after the agent row's Scan, commits revocation, and asserts at least two RR attempts, final 403, no remaining grant and no project update. |
| `TestTriageIterationAuthorizationRejectsPrestartedGrantRemoval/agent` | The old hook never released the parent lock, exhausting the bounded retries into a correct 409. The corrected hook commits after observing the agent lock's 55P03, then asserts a fresh retry, 403, pending admission, no iteration membership, action or event. |
| `TestTriageIterationAuthorizationRejectsPrestartedGrantRemoval/squad` | The same rule applies to the squad's actual leader agent. Shared squad/agent authorization and the final 403/no-partial-state assertions remain. |
| `TestTriageIterationAuthorizationIgnoresGrantInsertedAfterLockedRead` | Late insertion is explicitly checked for 55P03 and zero authorization residue. The old request stays 403 with no acceptance/action/iteration event; after grant commit a fresh request succeeds (200), records membership/action and does not dispatch an agent. |

The tests observe completed row scans, not query construction or timing sleeps.
The project hook gained an optional after-Scan observer; triage reuses that
existing transaction fixture and removes its obsolete grant-only observer.
All original denial and atomicity assertions remain, with explicit lock-error,
fresh-attempt and positive-control assertions added.

## Changed files

- `server/internal/handler/issue_create_authorization_test.go`
- `server/internal/handler/project_update_concurrency_test.go`
- `server/internal/handler/triage_iteration_authorization_test.go`

No canonical project fixture, configuration outbox consumer or workspace
retention manifest was modified by this lane.

## Verification

Go 1.26.6 and `scripts/go-test-with-agent-cli-guard.sh`, using
`postgres://postgres@127.0.0.1:51974/collaboration_squad?sslmode=disable`.
No installed/paid AI CLI was used.

Evidence directory:
`/var/folders/3n/gbt3p39s5pdc4l55js62gxnm0000gn/T/multica-cwc-resume-20261018-23vtl_pb`.

- `grant-regression-initial.log`: reproduces every reported case.
- `grant-regression-observed-race.log`: the four named test functions and all
  seven affected leaf cases pass with the completed-row lock observations.
- `grant-regression-neighbors-race.jsonl`: all 20 functions from the three
  changed files pass under `go test -race ./internal/handler -count=1`,
  68 test entries, zero failures/skips. Includes authorization held through
  commit, retry revalidation, notification concurrency and foreign-leader denial.
- `grant-regression-vet.log`: `go vet ./internal/handler` passes.
- `git diff --check`: clean for all changed files.

No unresolved defect remains in this slice. The root owns the ongoing broad
verification and other independently assigned failures.
