# Trigger preview server increment

## Change boundary

The comment preview currently reports authorization and waiting/hold facts but
cannot explain same-channel batch merging, existing waiting input or an active
squad's next leader turn. The canonical owners are comment target parsing,
`selectWaitingInputs` and `tryJoinSquad`; preview must remain inside the existing
authorized repeatable-read, read-only transaction.

Owned files: `collaboration_mentions.go`, a focused adjacent grouping projection,
and service regressions. A narrow pure queue decision helper may be shared when
needed. No SQL/migration, queue mutation, receipt, runtime policy, lifecycle or
client changes are intended. Other lanes own those files; baseline dirty work is
preserved. This is an implementation slice, not completion of the parent task.

## Additive preview contract

Eligible unheld target rows optionally contain
`grouping: {kind: "pending_batch" | "squad_continuation", target_ids: UUID[],
has_existing_inputs: boolean, follows_pending_batch: boolean}`.

All four fields are required if the object is present. IDs include self and only
eligible unsuppressed targets emitted by the same preview, with reciprocal
membership. Distinct eligible squads on one channel split batches. The existing
input flag only reflects applicable eligible pending inputs or unread joins;
the preceding-batch flag is always false for continuation. Held input can become
eligible after an explicit Resume, but has no current grouping projection.
Receipt identity and command/join result codes retain their previous meaning.
The new object is an expectation from current facts, rechecked on submission;
it does not accept input, authorize execution, or reserve a future outcome.

## Verification plan

Regression-first service tests cover typed deduplication, direct/squad merging,
second-squad FIFO boundaries, independent channels, suppression, prior pending
eligibility, active squad joins, and unchanged persistence snapshots. Existing
author and resource authorization helpers and input wire budget are reused.
Compare preview with real comment/queue submission where appropriate. Use the
main session's isolated preview database and guarded Go entry point. Preserve
raw red/green logs under `.omx/reports/cwc-preview-wakeup-20261020/preview/`.

## Implemented

- `server/internal/service/collaboration_mentions.go`: preview enriches existing
  target decisions inside the same authorized read-only transaction.
- `server/internal/service/collaboration_mention_grouping.go`: preview-only target
  type and grouping projection. Existing eligible inputs follow channel sequence,
  independently of staging batches. Same-comment targets are evaluated in the
  exact parser/submission order. The retained execution projector, current
  authors, authorized target catalogue, frozen grant identities, group/control
  state and runtime support bound continuation candidates. No queue, hold,
  dependency, group, continuation, event or operation is created by preview.
- `server/internal/service/collaboration_batches.go`: extracted the existing pure
  same-squad compatibility and runtime-wire single-input size helpers, shared by
  preview and canonical FIFO selection. Selector ordering, mutation and refusal
  behavior remain unchanged.
- `server/internal/service/collaboration_mention_grouping_test.go`: real database
  regressions for merging/splitting, typed deduplication, independent channels,
  suppression, earlier eligible/held/revoked/stale-author input, private metadata,
  continuation with/without prior unread input, gate/issue/channel pause, holds,
  sealed groups, installation stop, stale frozen grants and input byte limits.
  Order-sensitive cases compare preview with actual comment submission and the
  canonical FIFO selector. Existing preview tests retain no-content-mutation and
  source-membership-without-execution-authority coverage.

Receipt structures and result codes were not changed. The final optional
`grouping` shape matches the client lane's agreed parser. No SQL, migrations,
dependencies, lifecycle files or task/spec metadata were changed by this lane.

## Verification evidence

Used the isolated `preview.env` provisioned by the main session, Go 1.26.6 and
the repository's ambient agent-CLI guard. No real provider or paid account ran.

| Check | Result | Log |
| --- | --- | --- |
| New initial preview regressions before implementation | Failed on missing grouping, as intended | `preview/red.log` |
| Full focused preview behavior and boundary tests | Passed | `preview/green-boundaries.log` |
| `go test ./internal/service -run '^TestCollaboration(Comment\|Mention\|FIFO\|Batch\|Squad)' -count=1 -race -v` | 45 top-level tests passed, 0 failed/skipped; 37.841 s | `preview/race.log` |
| `go test ./internal/service -run '^TestCollaborationCommentPreviewSplitsAtRuntimeInputBudget$' -count=1 -race -v` | Passed; 3.210 s | `preview/budget-race.log` |
| `go vet ./internal/service` | Passed, empty diagnostics | `preview/vet.log` |
| `gofmt` and scoped `git diff --check` | Passed | Command output |

Log paths are relative to `.omx/reports/cwc-preview-wakeup-20261020/`.
The pure helper extraction is covered by the 45-test race suite and the final
large-input regression: 30 legal 64 KiB prior inputs plus same-comment A/S input
split into the exact first 31-input batch selected by the scheduler.

## Remaining limits and handoff

This is a current snapshot, not a guarantee of admission, immediate launch or a
future group identity. Submission retains every authoritative check and receipt.
Held input remains valid waiting input and may group after explicit recovery;
it has no current grouping projection. Full provider/native capacity acceptance
and parent-wakeup behavior are outside this lane. Main owns integrated browser
verification, independent review, final evidence/spec updates and commits.
