# Account invalidation fanout verification — 2026-10-14

This implements the durable account-invalidation consumer from design D05 and
execution contract E02/E07. It is a bounded backend slice; it does not complete
the shared-execution protocol or any full Web/Desktop acceptance scenario.

## Changes

- `server/internal/service/collaboration_account_reconciliation.go`: a restartable
  `CollaborationAccountReconciler`, bounded `Reconcile(ctx, limit)` and immediate
  startup/polling `Run(ctx)` with context cancellation.
- `server/pkg/db/queries/collaboration_reconciliation.sql`: ordered intent claim,
  source locking, epoch-scoped cancellation/revocation and durable progress.
- `server/migrations/617_collaboration_account_invalidation_progress.{up,down}.sql`:
  nullable `last_issue_id` cursor. No index or foreign key is introduced. Down
  locks the intent table with `ACCESS EXCLUSIVE NOWAIT` and refuses retained
  invalidations, including already reconciled rows.
- `server/internal/service/collaboration_account_reconciliation_test.go`: real
  database regressions using private schemas and the existing testutil fixtures.

Generated SQL and Handler/main startup wiring belong to the integration owner.
That owner regenerated SQL and applied migration 617 to the task-owned databases.
The worker runs independently of creation, execution and platform-admin gates.

## Durable ordering and authority

The existing credential transaction remains unchanged: it advances the account
fence and writes an invalidation intent without scanning or locking source issues.
The consumer claims one intent with `FOR UPDATE SKIP LOCKED`. A pending earlier
epoch always excludes later epochs for that account, even when another worker
holds its row lock.

Each transaction handles one source issue in UUID order. It takes workspace,
issue, collaboration and session locks in that order, using NOWAIT at these
source boundaries, then reuses the existing grant/run/command and `stopRun`
primitives. Command cancellation, grant revocation, pause/stop, system event and
cursor advancement commit together. The separate completion step only marks the
intent reconciled after its candidate range is exhausted. A failed source never
advances the cursor.

Each bounded pass attempts at most the requested number of steps, with at most
three transactions per step. Failed accounts are excluded only for the remainder
of that pass; other accounts can converge before joined errors are returned.
This prevents a locked or corrupt source from indefinitely delaying independent
stop intents. A later pass retries the original persisted cursor. An uncertain
commit is not immediately retried; its account is likewise excluded for that
pass, and the next pass reads the stored progress.

All mutations compare against the **intent's** account epoch. Current-epoch
commands/grants and unrelated authors survive an older intent. Account disable
and restore never change creator, handler or participation. Already-stopped runs
keep their original forced disposition/reason, fence, nonce, claim generation and
unknown-proof barrier. The only immediate settlement remains the existing
server-proven never-launched case; possible execution requests a stop. A closed
session remains closed. The system event has no fabricated human actor. Worker
logs contain bounded outcome/SQLSTATE codes, not raw errors or private row values.

The keyset cursor depends on the existing authority writer contract: command
submission holds the user row before source locks, re-reads the account fence and
rejects a stale observed epoch. The credential writer holds that same user lock.
Once invalidation commits, no compatible writer can insert an old-authority
command behind its cursor. The future grant/promote/permit writers must obey the
same contract before execution is enabled. Re-invitation advances participant
authority and never revives old command/grant rows; this worker does not restore
either. Historical grants remain candidates even when already revoked, so their
possibly live runs are still found.

## Verification results

All Go commands used `scripts/go-test-with-agent-cli-guard.sh`. Before DB tests,
`DATABASE_URL` was loaded from the private task env file and `current_database()`
was asserted to equal `multica_collab_20261014_fanout_1791943635066`. No default
database or installed provider CLI was used.

| Check | Result | Evidence |
| --- | --- | --- |
| Initial new reconciliation suite | Passed, 9.031 s | `/tmp/multica-collab-20261014-yOusnI/fanout-db.log` |
| Final reconciliation, credential writer and queue suites with race detector | Passed, 84.145 s | `/tmp/multica-collab-20261014-yOusnI/fanout-race.log` |
| `go vet ./internal/service` | Passed, exit 0 | `/tmp/multica-collab-20261014-yOusnI/fanout-vet.log` |
| gofmt and whitespace checks of all owned code/SQL files | Passed | Tool output; untracked files checked explicitly against `/dev/null` |

Final DB command, run from the repository root after the database assertion:

```bash
scripts/go-test-with-agent-cli-guard.sh go -C server test -race ./internal/service \
  -run '^TestCollaboration(Account(Reconciliation|Invalidation)|Queue)' \
  -count=1 -timeout=180s
```

The new suite proves:

- Actual administrative disable/restore writers retain both invalidation intents;
  interrupted single-issue progress resumes in UUID order with fresh-epoch work
  preserved on the same issues.
- Obsolete authors' waiting commands cancel; obsolete resource owners' grants
  revoke and their active runs stop even when authored by somebody else. Other
  waiting work, responsibility and participation remain intact.
- Never-launched author work becomes cancelled; never-launched resource withdrawal
  becomes stopped. Success/started evidence without a permit does not fabricate
  proof. Existing forced roots and closed sessions survive subsequent invalidation.
- Held workspace/issue/session locks roll back all effects and exhaust exactly
  three bounded retries without skipping the source or advancing a later epoch.
- Two consumers converge without duplicated events; a locked earlier account
  intent does not block a different account or permit its own later epoch to pass.
- Held and missing sources retain their cursors and report errors while an
  independent account's live run and waiting work converge within the same pass.
- Injected event, cursor and commit failures roll back cancellation, revocation,
  stop intent and progress. A committed operation with a lost reply is recovered
  without another generation increment or duplicate event.
- `Run` starts immediately and stops on cancellation after a committed issue;
  a fresh worker completes the retained intent.
- Actual migration 617 down SQL refuses both an uncommitted writer and retained
  intent, and succeeds only in an empty private-schema maintenance fixture.

The first attempted test build met a temporary, independently owned handoff DTO
compile mismatch. The integration owner updated those tests; the final full
service-package build and the checks above include that correction.

## Independent review correction: fairness across batches

The first implementation excluded failures only within one call. Independent
review reproduced a failing prefix as large as the batch: with two earlier
locked accounts and `limit=2`, three polls processed zero steps and never stopped
a later account's run. The main session repaired this after the implementation
agent was rate-limited.

Migrations 618–619 add `last_attempted_at` and a concurrent ordered-pending index.
Successful effects stamp the attempt in their cursor transaction. After a failed
or uncertain source commit has been rolled back, one independent `SKIP LOCKED`
statement stamps only the same intent's scheduling timestamp. It never advances
the source cursor or repeats cancellation/stop effects; the original error and
any scheduling failure are both retained. Selection remains ordered by account
epoch, then by the persisted attempt position across independent accounts.

The original checker regression now passes independently. The parent also added
worker-restart and scheduling-write-failure tests, extended real rollback guards
to 617–619, and ran the final fanout race suite: 11 top-level tests plus 18
subtests pass with zero failure/skip (`fanout-final-fairness.jsonl`, 64.083s).
The original tests above remain earlier evidence; use the latest
[integration review](verification-integration-review-2026-10-14.md) and
[parent verification](../verification-2026-10-14.md) for the final result.

## Remaining boundaries

Run and grant rows in these tests are explicit DB fixtures. They do not prove
provider containment, actual launch admission, runtime enrollment, native sealed
termination proofs, transport shutdown or public grant creation. This slice does
not add positive Resume, new execution admission, or a fabricated termination
receipt. Those P4/P5 prerequisites and full integration/dual-client validation
remain necessary before enabling collaboration execution or creation.

The consumer intentionally retains a corrupt source's pending intent and reports
its failure; repairing or deleting that source is outside this slice. An unknown
run remains a barrier even after account invalidation has fully reconciled.
