# Trigger preview and parent wakeup increment

The user authorized continuing these two items on 2026-10-20 in the existing
in-progress task. This is an implementation slice of the approved R24/M06/M08/M11
and R58/AC89 contracts, not a new task or release authorization.

## Behavior gap and boundary

- Trigger preview currently reports per-target admission/hold facts but does not
  explain same-lane merging, existing pending batches or active squad continuation.
  Extend the authorized read-only server projection and shared schema/composer so
  users see the applicable grouping/continuation facts before sending. The queue
  remains authoritative at submission; a preview must not enqueue, authorize,
  resume or promise a future runtime outcome.
- Completing a collaboration child must enter the existing ordinary-agent parent
  wakeup path with the ordinary stage/sibling rules. Preserve terminal transitions,
  commit/rollback and replay semantics, and the prohibition on running an ordinary
  executor for a shared human-owned parent. Inspect the actual lifecycle owner and
  reuse its effect mechanism rather than introducing a parallel scheduler.

## Ownership and implementation sequence

1. Preview server lane owns collaboration mention-preview projection/service tests
   and narrowly necessary queue read helpers. Agree the additive preview-only API
   with the client lane before changing it; preserve old receipt parsing and
   authority/privacy boundaries.
2. Preview client lane owns core preview schema/tests and shared composer/tests,
   bilingual copy and focused browser coverage. Existing unknown-request recovery,
   current-access fencing, suppression and draft identity must remain intact.
3. Wakeup lane owns lifecycle-to-ordinary-parent integration and focused Go tests.
   Confirm exact affected files after tracing both ordinary and collaboration
   paths. Do not overwrite existing uncommitted changes in overlapping files.
4. Main session coordinates shared SQL generation if required, test environments,
   independent review, integrated checks, spec/progress updates and scoped commits.

All agents must read the current PRD, design, implementation plan and relevant
execution/data contracts. They are not alone in the workspace. Report ownership
collisions before changing another lane's files; do not revert baseline work.
The initial 38 changed paths and full binary diff are backed up outside the repo
at `/var/folders/3n/gbt3p39s5pdc4l55js62gxnm0000gn/T/multica-preview-wakeup-20261020-vwujh9xt`.

## Verification

- Failing behavioral regressions first: same-lane typed target merging,
  suppression, pending-batch/continuation distinctions, held/denied targets and
  no preview persistence or unauthorized metadata; malformed/older API responses
  and a shared UI happy path with actionable grouping facts.
- Real PostgreSQL lifecycle regressions: collaboration child completion wakes an
  eligible ordinary agent parent, unfinished same-stage siblings block it, replay
  does not duplicate it, rollback cannot leak effects, and shared parents remain
  protected. Include permission and stage cases at the canonical service layer.
- Run focused tests while iterating, affected package lint/typecheck, Go vet/race,
  SQL generation consistency when SQL changes, and appropriate shared-client
  verification. Record exact passed, skipped and untested scopes.
- Dispatch independent Trellis checking, resolve verified findings, then retain
  evidence and update current task progress. Keep the parent task in progress and
  default creation/execution rollout gates closed.

## Excluded from this increment

Runtime attachment/Git preparation, general provider adapters, other operating
system containment, P7 administration and exhaustive P8/97-criterion acceptance
remain the parent task's later work. No new dependencies or broad refactors.
