# Collaboration child completion and ordinary parent wakeup

Implemented the authorized R58/AC89 wakeup slice on 2026-10-20. The parent task
remains in progress; this report does not establish full acceptance or authorize
creation/execution rollout.

## Boundary and implementation

The collaboration lifecycle transaction already committed issue status, queue
closure, lifecycle versions, history and the idempotency receipt. Its HTTP owner
did not call the ordinary child-completion hook, so an eligible ordinary parent
never received its stage-completion comment or task.

`CollaborationWorkflowReceipt.ParentWakeup` now carries the locked before/after
child rows only for a new complete/cancel transition with a parent. It has
`json:"-"`: neither operation/history JSON nor HTTP responses contain the
effect. The existing workflow owner discards the receipt on write or commit
failure; successful receipt replay has no effect because decoding cannot restore
that field. Reopen and ordinary nonterminal status changes carry no wakeup.

`ChangeCollaborationLifecycle` consumes the effect after the service confirms
commit and calls the existing `notifyParentOfChildDone`. The existing code owns
terminal/custom-status resolution, staged frontier and unstaged barriers,
agent readiness, pending-task deduplication and squad-leader-only dispatch.
There is no new scheduler or ordinary executor for the collaboration child.

Immediately before posting the ordinary parent comment, the helper reloads the
parent and checks terminal/backlog/human ownership plus the authoritative
collaboration relation. Even a malformed shared parent with an agent assignee
stays inert. Ordinary task admission retains its independent collaboration fence.
The shared child's identifier/title remain the permitted minimal source
reference; its comment reference has no ordinary issue link. No description,
comments, attachments or other private child context are copied into the parent
comment or queued task. Ordinary child comment text remains unchanged.

## Files changed

- `server/internal/service/collaboration_workflow.go`: transient receipt effect.
- `server/internal/service/collaboration_lifecycle.go`: capture the terminal
  transition after required lifecycle/history writes.
- `server/internal/handler/collaboration_lifecycle.go`: consume the committed
  effect through the existing ordinary hook.
- `server/internal/handler/issue_child_done.go`: current-parent/shared-parent
  guard and minimal shared-child reference.
- `server/internal/service/collaboration_parent_wakeup_test.go`: committed and
  transient effects, permission/refusal/write/commit failures, replay and
  concurrent same-key replay.
- `server/internal/handler/collaboration_parent_wakeup_test.go`: real comment and
  task dispatch, stage and custom-status cases, parent privacy/guards, and
  explicit precommit/rollback/unknown-commit observations.

No SQL, generated files, dependencies, task metadata or existing unrelated work
were changed. No commit was made by this implementation lane.

## Verification

Tests ran against the main session's isolated wakeup PostgreSQL database using
the supplied `wakeup.env`, Go 1.26.6 and
`scripts/go-test-with-agent-cli-guard.sh`. Handler and service executions were
sequential against that database. No ambient AI CLI ran.

Raw logs are in `.omx/reports/cwc-preview-wakeup-20261020/wakeup/`.

| Log | Check and result |
| --- | --- |
| `handler-red.log` | Before production edits: three top-level regression groups failed because eligible parent comments/tasks stayed absent; custom terminal cases reproduced the same gap. |
| `handler-green.log` | Initial implementation: all three groups passed (20 tests/subtests). |
| `handler-regression.log` | Ordinary child/stage/batch/R51 regressions plus new tests: 32 top-level tests passed (68 including subtests). |
| `service-green.log` | Initial service effect/rollback tests: two top-level tests passed (8 including subtests). |
| `service-race.log` | `go test -race ./internal/service -run 'TestCollaboration(Lifecycle\|Status)' -count=1 -v`: 16 top-level tests passed, 43 including subtests; 11.331 s. |
| `handler-race.log` | `go test -race ./internal/handler -run 'Test(CollaborationChild\|.*ChildDone\|ChildReopen\|Stage\|CollaborationOrdinaryChild)' -count=1 -v`: 35 top-level tests passed, 74 including subtests; 4.091 s. |
| `vet.log` | `go vet ./internal/service ./internal/handler`: exit 0, empty output. |

All green/race runs had zero skips and no race reports. `gofmt -l` for the six Go
files and scoped `git diff --check` also passed with empty output.

Behavior covered includes agent and squad parent dispatch with correct triggering
comment/leader provenance; replay after the original task has settled; same and
earlier unfinished stages; later parked stages; unstaged rules; cancelled and
custom terminal siblings/children; backlog, terminal, human and shared parents;
stale parent snapshots; departed-source creator lifecycle authority; forbidden
handler and stale revision; event/receipt/commit failures; and concurrent same-key
completion yielding exactly one transient effect.

The handler commit observer reads through a second connection before commit and
asserts that child closure, parent comment and task are not visible. Confirmed
commit produces the wake; rollback and uncertain commit produce no wake. Unknown
commit recovery returns the retained result without reconstructing the effect.

## Limits and main-session follow-ups

- The ordinary hook remains best effort. A process crash after lifecycle commit
  but before hook delivery, or comment/task creation failure, can lose the wake.
  An uncertain commit also intentionally supplies no effect. Original-key replay
  does not repair those gaps; adding durable scheduling/recovery is outside this
  increment and would change the ordinary contract.
- Stage and parent checks retain ordinary post-commit snapshot semantics. This
  slice does not introduce a transactional cross-issue scheduler or change
  concurrent distinct-child notification coalescing.
- Future non-HTTP lifecycle consumers must explicitly consume the committed
  effect if they need the ordinary wakeup behavior. The current production
  `ChangeLifecycle` caller is the HTTP handler connected here.
- Main owns independent Trellis checking, full integration checks and spec/task
  progress updates. Relevant built-in documentation is
  `multica-working-on-issues/SKILL.md` (sub-issue stages) and
  `references/working-on-issues-source-map.md` (stage barrier table). These were
  surfaced to main for its documentation pass.
- No full repository suite, browser/desktop run or native runtime/provider
  execution was performed by this backend lane. The tests prove database
  transitions and ordinary queue insertion, not an actual model execution.
