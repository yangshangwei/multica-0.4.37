# Integration review — 2026-10-14

## Scope and environment

Independent Trellis review of the current authorized continuation: R41/R48
metadata editing, shared workflow authorization, root PATCH status compatibility,
directed-handoff status confirmation, persisted account-invalidation consumption,
and the previously reported legacy-agent JWT/PAT boundary. The latest task
contracts, complete server security boundaries, and continuation plan were read.

All database tests used only `multica_collab_20261014_check_1791943635066`, sourced
from the private `check.env` in `/tmp/multica-collab-20261014-yOusnI`. The database
was independently identified with `SELECT current_database()` and was migrated
through 619 before the final fanout run. Every Go test used
`scripts/go-test-with-agent-cli-guard.sh`. No provider CLI, deployment, commit,
new dependency, application database or global migration rollback was used.

Reviewer-owned changes are the regression
`server/internal/service/collaboration_account_reconciliation_review_test.go`
and this report. Production repairs and migration generation were handled by
the parent/consumer owner; concurrent and preexisting changes were preserved.

## Findings (fixed)

### F1 — Repeated bounded passes could starve later invalidations

- Files: `server/internal/service/collaboration_account_reconciliation.go:41`,
  `server/pkg/db/queries/collaboration_reconciliation.sql:16`.
- The initial within-pass exclusion fix still consumed one batch slot per
  failed account and discarded exclusions at the next call. With at least
  `limit` earlier busy accounts, every call selected the same failing prefix.
  Later independent accounts never received waiting-command cancellation or
  run stop intent. `Run` uses a limit of 32, so this was also a production
  polling problem, not just an artificial unbounded scan.
- Added `TestCollaborationAccountReconciliationMakesProgressBeyondFailedPass`.
  Real account-disable operations create two earlier invalidations sharing a
  transaction-locked source and a third account with an independent run and
  waiting command. Three calls with `limit=2` initially processed zero steps.
  The test also requires the failed source cursors to remain unchanged and the
  independent run's proof to remain unknown/unsettled after its stop request.
- Parent repair: migration 618 persists `last_attempted_at`; migration 619
  adds the single-statement concurrent pending-poll index. Selection orders by
  `COALESCE(last_attempted_at, created_at)`. Successful effects stamp attempt
  time with their original cursor transaction. After rollback/unknown commit,
  `recordFailedAttempt` updates only the same intent's polling timestamp using
  `FOR UPDATE SKIP LOCKED`; it does not replay source effects or move the cursor.
  Original errors remain visible, and later account epochs still cannot
  overtake an earlier pending epoch.
- Independent result: red log reports `processed=0`; the repaired regression
  passes in 4.75 seconds. The complete then-current fanout suite also passes.

### Previous high legacy-agent finding — closed by current evidence

- Files: `server/internal/middleware/auth.go:34`,
  `server/internal/auth/principal.go`, `server/internal/collaboration/access.go`,
  `server/cmd/server/collaboration_routes_test.go:113`.
- Authentication now records either nonempty legacy agent/task header as a
  deny-only `AgentActor` hint. Verified JWT/PAT identity remains intact; this
  hint cannot grant authority. Collaboration's human whitelist rejects it.
- Independently ran middleware cases covering JWT, cookie, cached PAT, partial
  and malformed hints, and real-router JWT/PAT cases in both legacy and password
  modes. The same credentials retain ordinary human access without agent hints;
  with hints they cannot read private detail/workbench/receipts, invite, or
  search private body text. Permitted title references stay minimal. The prior
  high finding must not remain open based on the superseded implementation.

## Additional review results

- Metadata edits use the shared transaction owner and locked current
  creator/handler authority, including external handlers and creators who left
  the source workspace. Values, I1 facts, collaboration revision, history and
  receipt commit together. Null/omission/date compatibility and no-op behavior
  match the documented contract. Command snapshots, execution generations and
  responsibility remain unchanged.
- The root PATCH union rejects mixed status/metadata fields, including explicit
  null. Status actions retain their existing service contract. Unimplemented
  structural fields remain rejected.
- Directed status digests bind the source/action, relevant current catalog
  facts and saved return-request identity. Mutation rechecks occur under the
  catalog/source locks. Refused-key replay does not become a fresh action;
  supplemental options are projected from current authority. Ordinary transfers
  reject directed status fields rather than silently discarding them.
- Fanout compares authority with the persisted intent epoch, preserving fresh
  commands/grants after disable/restore. An issued permit, conflicting success
  or started report cannot become fabricated termination evidence. Closed and
  paused queues, first forced dispositions, run fence/nonce and unknown barriers
  survive. Only the existing server-proven never-launched path settles locally.
- Failure scheduling is separate from source effects. Missing/busy sources,
  required-write failures and uncertain commits retain the source checkpoint.
  The account consumer is connected to Handler/main and continues while
  creation/execution are disabled. Notification intent remains reference-only;
  metadata edits do not broadcast private before/after values as inbox content.

## Findings (not fixed)

No additional production defect remains open in this reviewed tranche after
F1's repair. No design/public-interface changes were made by this reviewer.

This is not full-feature approval: native containment/termination proof,
positive Resume/admission and launch, complete old-writer coverage, controlled
content/realtime delivery, R55 edit-mention delivery, full history reads and
Web/Desktop end-to-end acceptance remain governed by the task's unfinished
P1–P8 work. Seeded run rows test server barriers; they prove no actual process
exit. Creation/execution release gates must stay closed until those independent
requirements are satisfied. The parent owns the associated final spec/progress
sync, including the new fairness rule.

## Verification

Logs below are under `/tmp/multica-collab-20261014-yOusnI/`.

| Check | Result | Evidence |
| --- | --- | --- |
| Core lint | Pass | `check-core-lint.log` |
| Core TypeCheck | Pass | `check-core-typecheck.log` |
| Core collaboration tests | 250 tests / 7 files passed | `check-core-collaboration.log` |
| Focused Go race checks: middleware, real router, service, handler | 36 top-level tests / 139 test and subtest passes; zero failure/skip | `check-focused-race.jsonl` |
| Added fairness regression before repair | Expected failure, `processed=0` | `check-fanout-fairness-red.jsonl` |
| Final fanout race suite after 618/619 | 9 top-level tests / 24 test and subtest passes; zero failure/skip, 56.644s | `check-fanout-fairness-green.jsonl` |
| Go vet: collaboration/service/handler/middleware/server | Pass | `check-go-vet.log` |
| Go vet: service after fairness repair | Pass | `check-fanout-final-vet.log` |
| Changed Go formatting and diff whitespace | Pass | `check-gofmt.log`; `git diff --check` |
| Pending-poll index | Valid and ready on the checker DB | `pg_index` inspection of `idx_collaboration_account_invalidation_poll` |

The initial database preflight used an unavailable unqualified `psql` and never
started tests; it is not counted as verification. Subsequent checks used the
confirmed `/opt/homebrew/opt/libpq/bin/psql`. Broader parent-run results are not
counted as this reviewer's independent execution evidence.
