# Collaboration trigger previews and ordinary parent wakeup

## 1. Scope / Trigger

Apply when changing collaboration comment preview, FIFO grouping, squad
continuation hints, or a collaboration child's terminal lifecycle transition.
The existing queue owns admission/execution; the ordinary child-done hook owns
parent stage barriers. These changes do not open the product rollout gates.

## 2. Signatures

- `POST /api/collaboration/issues/{id}/comments/trigger-preview` keeps protocol 2
  and adds optional `targets[].grouping` only to the preview response:
  `{kind: "pending_batch" | "squad_continuation", target_ids: UUID[],
  has_existing_inputs: boolean, follows_pending_batch: boolean}`.
- `CollaborationService.previewCommentGrouping` reads the authorized snapshot.
  `collaborationRunInputSize` and `collaborationBatchTargetCompatible` are shared
  pure decisions used by preview and FIFO selection.
- `CollaborationWorkflowReceipt.ParentWakeup` is an internal `json:"-"`
  before/after issue pair. `ChangeCollaborationLifecycle` consumes it by calling
  `notifyParentOfChildDone` after the service confirms commit.

## 3. Contracts

### Read-only grouping

Grouping describes the currently eligible FIFO input set, not a physical staging
batch or guaranteed future run. Preserve typed target identity and mention order.
Direct A and one squad led by A can share a group; a second distinct squad, the
input-count limit or exact serialized-byte limit creates a later batch. Skip
held/unauthorized targets and invalid earlier authors. A paused queue may still
have pending grouping facts; those facts never resume it.

`target_ids` names only selected, unsuppressed targets from this preview that
share the projected grouping. Every peer lists the same set and flags. Earlier
stored inputs are represented only by `has_existing_inputs`, never their body,
author, command, batch or group IDs. For a continuation, this flag means unread
contributions; already consumed inputs still count toward its context budget.
`follows_pending_batch` means an earlier eligible batch in the same agent's
queue, and must be false for a squad continuation.

Continuation hints require the current active squad, live eligible run, valid
frozen dependencies/resources, no earlier eligible waiting input and sufficient
budget. Use read-only projections and queries; do not call locking/mutating
admission helpers from the repeatable-read preview. Submission rechecks the
actual current state and remains the authority for accepted/blocked results.

The receipt schema remains separate and unchanged. Old servers omitting grouping
stay usable; present malformed/nonreciprocal groups are unavailable. Core rejects
unknown group kinds, duplicate/missing peers and held/blocked/suppressed members.
The composer shows each group once using localized product terms and only shows
a preview when its exact draft UUID and complete request input still match the
live draft. External form updates and replacement drafts with identical text
must invalidate that match just like local text/checkbox changes.

### Ordinary parent wakeup

A new committed complete/cancel operation with a parent carries a transient
before/after pair. Reopen, nonterminal edits, refusals and write/commit errors do
not. JSON persistence and response mapping exclude the pair, so original-key
replay cannot repeat the wake even after the original parent task has settled.

The existing hook evaluates canonical status categories, the lowest unfinished
stage and the implicit stage for unstaged siblings. It dispatches the ordinary
agent or squad leader only when that barrier closes. Later parked stages do not
block the just-closed stage; unfinished same/earlier stages do. Before posting,
reload the parent and refuse terminal/backlog/human-owned or collaboration
parents, including malformed shared rows with an old agent assignee. Ordinary
queue admission retains its independent shared-issue fence.

The parent receives only the shared child's sanitized identifier/title, without
an ordinary child link, private body, attachments or comments. This minimal
reference grants no shared-child access. Ordinary child notification wording
and existing agent/squad attribution remain with their original owner.

Wake delivery retains ordinary best-effort semantics: a crash after commit, a
notification/queue failure, or an uncertain commit can lose the wake. Replaying
the operation does not repair it. Durable delivery would require a separate
contract change; do not imply these transient effects are an outbox.

## 4. Validation & Error Matrix

| Condition | Result |
| --- | --- |
| Same root A and one squad, within budget | One eligible grouping |
| Second squad or count/byte boundary | Separate later grouping |
| Held, suppressed or unauthorized target | No grouping; retain its existing reason |
| Active eligible squad without earlier FIFO work | Continuation hint; no new command/run |
| Queue/gate paused, sealed group or stale frozen authority | Pending grouping instead of continuation |
| Old server omits grouping | Existing preview works |
| Malformed or conflicting grouping peers | Reject the new projection |
| Live form input differs from inspected request | Hide stale preview and require inspection before send |
| Committed child close closes an ordinary stage | One invocation of the existing parent hook |
| Replay, rollback or unknown commit | No transient wake effect |
| Shared/backlog/terminal/human parent | No ordinary parent comment/task |

## 5. Good / Base / Bad Cases

- Good: preview shows A+S1 together and S2 later; suppressing S1 recomputes A+S2.
- Base: an older preview response and an ordinary child's completion keep their
  existing meanings.
- Bad: preview enqueues work, hides a pause, returns private prior inputs, uses
  a stale form preview, or reconstructs parent wakeup from a persisted receipt.

## 6. Tests Required

`collaboration_mention_grouping_test.go` owns real-DB snapshot/side-effect,
typed ordering, preview-vs-submit, size, FIFO, authority and continuation cases.
Existing comment/batch/FIFO/squad tests protect the extracted pure decisions.
`content-schema.test.ts` owns malformed/legacy grouping; shared composer tests
own display, suppression and external-draft synchronization. The production
Web preview journey uses real target/consent/comment APIs with an explicitly
paused synthetic queue; it does not prove provider execution.

Service/handler `collaboration_parent_wakeup_test.go` cover committed effects,
replay after settlement, concurrent same-key completion, write/commit failures,
custom terminal categories, source-departed creator authority, stage/sibling
rules, agent/squad provenance and shared-parent isolation. A second connection
observes no parent effects before commit. Keep ordinary child/stage/batch tests.

## 7. Wrong vs Correct

Wrong: call `tryJoinSquad` from preview, or display a receipt's group ID as a
prediction. Correct: use authorized read-only facts and revalidate on submission.

Wrong: rebuild `ParentWakeup` on receipt recovery. Correct: return it only from
the successful new invocation and consume it after commit through the existing
ordinary hook.
