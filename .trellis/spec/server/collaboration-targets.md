# Collaboration target and execution-group foundation

## 1. Scope / Trigger

Applies to the version 2 collaboration foundation, migrations 620–642 and
650. Read this before changing collaboration queue/control, grant invalidation,
ordinary execution admission, or the shared execution read projection.
It supersedes the single-session execution contracts in
[collaboration-foundation.md](./collaboration-foundation.md); that document's
human authority, creation, editing and account-fence rules still apply.

This foundation stores typed targets and controls seeded execution ledgers.
Public creation, target consent, comment-triggered execution, positive Resume,
promotion, runtime permits and process-proof ingestion remain unavailable.
Fixture-created groups/runs do not prove scheduling, capacity or native process
termination. Keep the creation/execution gates closed until P1–P8 acceptance.

## 2. Signatures

`CollaborationService` owns these internal transactions:

- `RegisterTarget(ctx, CollaborationRegisterTargetParams)` records a pending
  agent/squad configuration; it neither grants resource consent nor enrolls users.
- `SubmitCommand(ctx, CollaborationSubmitCommandParams)` requires `TargetID`,
  `InputEventID`, expected target/config versions and immutable content, in
  addition to the existing actor/participant/lifecycle/request identity.
- `CancelWaitingCommand(ctx, CollaborationCancelCommandParams)` additionally
  requires the exact target and command plus collaboration revision.
- `StopQueue(ctx, CollaborationStopQueueParams)` accepts `Scope=target|issue`
  and observed target controls/group generations. Issue scope also carries the
  session control version and target-set version.
- `ResumeQueue(ctx, CollaborationResumeQueueParams)` adds the expected execution
  gate epoch. It persists refusals only; no active-state writer exists yet.
- `RevokeGrant(ctx, CollaborationRevokeGrantParams)` checks the actual grant
  owner and expected grant epoch before revoking and stopping affected work.
- `ExecutionView(ctx, CollaborationIssueAccess)` borrows the caller's authorized
  repeatable-read snapshot for `GET /api/collaboration/issues/{id}`.

`stopInvalidatedWork`, `stopTargets`, and group settlement share the caller's
transaction. Participant departure, account reconciliation and lifecycle close
must use these primitives rather than reading one run or pausing every target.

Ordinary execution uses `checkOrdinaryExecutionSource` / `checkIssueExecution`,
the SQL `lock_issue_execution` function, and persisted-source checks in
`FinalizeTaskClaim`. These are independent of the public HTTP guards.

## 3. Contracts

### Identity and migration

Each issue retains one permanent session for issue-wide pause/close and event
order. Each `(session_id, kind, resource_id)` has one target with its own input
sequence, control version, group generation and active group. An agent and a
squad with that agent as leader are distinct targets. They do not acquire extra
physical capacity merely by having distinct target IDs.

Commands uniquely bind `(target_id, seq)` and `(input_event_id, target_id)`.
One root command has at most one execution group; each target has at most one
unsettled group. Runs bind their target, group, root command, parent and stable
step key. Group rosters freeze resource/grant references; a member's actual
resource owner is independent of the root human author.

Migration 620 preserves old command/run IDs, sequences, nonces and proof state.
Original indexed session/command keys become `legacy_*` identities; new typed
rows leave those old index keys null. Historical commands/runs/grants are marked
`legacy_unresolved`, with no guessed target. New unbound or fabricated legacy
inserts fail. Unresolved inputs cannot be promoted or acquire new permits.
Session pause and the old unknown execution barriers survive the upgrade.

All new indexes have one concurrent build per migration. No foreign keys or
cross-table relationship triggers are added: transactional service primitives
validate same-issue/target/group relationships. Row-local checks/identity guards
protect immutable snapshots and prevent revival. Down migrations use ordered
`ACCESS EXCLUSIVE NOWAIT` locks and refuse retained state; maintenance rollback
must stop all writers throughout the complete file sequence.

### Input, control and invalidation

Pending target registration requires creator/handler authority and already
participating resource owners. It always starts paused. Input authorization
requires the complete current roster and valid grants; busy/offline/maintenance
states may retain waiting input, but this check is never a launch permit.

Target Stop seals and stops the selected group's descendants while preserving
other targets and issue state. Issue Stop pauses the permanent session, covers
every observed target and fences later additions. Waiting root inputs survive
Stop. Scope versions reject a changed target set or a replacement group.
The first forced disposition remains the historical cause.

Settlement belongs to a sealed group after all descendant runs and delegation
intents are resolved. Never clear the root command or target position after only
the leader's proof. A never-permitted run can settle only with the existing
immutable zero-permit/no-start/no-contradictory-outcome checks. An issued permit,
unknown proof, missing/corrupt ledger or unresolved legacy identity keeps a
barrier. Future run/delegation insertion must validate relationships under the
same aggregate locks; these insertion APIs are not part of this foundation.
Whole-scope Stop/close rejects a retained group/run whose target row is missing,
and an unresolved legacy active pointer without its matching run. The entire
transaction rolls back; neither a partial stop nor a successful close is reported.

Target Resume cannot cross issue-wide pause. Issue Resume validates its whole
scope before any state change; currently all positive paths end in
`collaboration_execution_unavailable`. Refusal receipts are durable and never
become automatic resume intentions. The gate is locked `FOR SHARE` and compared
against the original expected epoch even on this refusal-only path.

Removal/exit cancels the author's waiting inputs across targets and stops that
author's active groups. Owner departure revokes that owner's grants and stops
affected current/frozen resource scopes. Other authors' waiting inputs and
unrelated targets survive. Account invalidation applies only to obsolete
author/grant epochs; a fresh grant and group on the same typed target must
survive an older intent. Close cancels all remaining inputs and stops all groups;
reopen keeps issue and targets paused without reviving past work.

User locks are discovered and sorted before source locks, then rechecked under
the aggregate fence. Current owners serialize through issue → session → sorted
targets. The stop path then locks unsettled groups/runs before validating their
commands; there is no independent runtime writer yet. Before adding claim,
delegation or proof writers, align the nested command/group/run lock order with
the execution contract and test those cross-writer races. Every helper borrows
the same transaction; only rolled-back conflicts receive bounded retries. An
uncertain commit is recovered with the original request key.

### Ordinary execution and wire format

Ordinary enqueue, existing-task reuse, claim, start, retry and credential
finalization reject a collaboration issue or captured/shared-comment content.
Persisted source/runtime/context must match the finalization snapshot.
An ordinary child or a pure parent reference retains ordinary behavior; the
reference does not grant access to collaboration content.

The issue response has an `execution` object with `protocol_version: 2`,
`availability: "unavailable"`, `unavailable_reason: "shared_runtime_unavailable"`,
explicit issue/target controls, typed commands/groups/runs, and unresolved legacy
IDs. No roster, grant, installation credential, working directory or input body
is exposed by this projection. Versions are exact signed-64-bit decimal strings;
group generation may be zero before the first group. Ordinary Issue revision
remains numeric.

Core validates complete identity/control relationships, duplicates, parent
cycles, terminal-group run evidence and issue/session scope. Missing old-server
execution data preserves human reads with `execution=null`; a malformed or
unknown present protocol makes the response unavailable. HTTP/core detail
projection disables execution affordances while endpoints are absent, without
changing internal participant authority. Future Stop must depend on current
human authority and supported control protocol, never on a still-valid grant.
All parsing and transport errors keep private-value redaction.

## 4. Validation & Error Matrix

| Condition | Result |
|---|---|
| New command lacks target/input identity | `400 invalid_command_target`, no input |
| Same input-event/target with different author or content | `409 input_event_conflict` |
| Changed target/config or scope snapshot | `stale_target`, `stale_scope`, `stale_execution` or `stale_control_version` |
| Incomplete/revoked resource consent | `409 grant_required`; no guessed target |
| Target Resume during issue pause | `409 issue_paused`, no gate changes |
| Applicable run/group/legacy barrier remains | `409 stop_unconfirmed` |
| Otherwise admissible Resume in this foundation | `503 collaboration_execution_unavailable` |
| Grant revocation by someone other than its owner | No matching owner grant; no mutation |
| Ordinary execution tries a collaboration source | `unsupported_collaboration_action` or SQL false; no launch credentials |
| Missing collaboration authority storage | Failure/closed admission, never ordinary fallback |
| Unknown/malformed present execution protocol | Private parse returns unavailable |
| Populated or busy migration rollback | Refuse without losing retained rows/indexes |

## 5. Good / Base / Bad Cases

- Good: one input addresses agent A and squad S led by A; each keeps its own
  target/sequence. Physical resource scheduling remains a later prerequisite.
- Good: stopping S records stop for its leader and members, keeps an unrelated
  target active, and waits for every required proof before settling S's root.
- Good: account epoch 2 invalidation observes an old grant but preserves a new
  epoch 3 grant/group on the same target.
- Base: ordinary child execution, ordinary chat/quick-create, human handoffs,
  metadata editing and collaboration version/idempotency rules remain intact.
- Bad: taking a legacy current grant as evidence of the intended target,
  clearing a group after one member, or treating a v2 read projection as runtime
  support.

## 6. Tests Required

- `collaboration_targets_test.go` and the multi-target service tests: explicit
  targets, distinct typed roles, FIFO/input deduplication, scoped Stop, later
  target creation, every-descendant settlement, owner/author invalidation,
  same-target fresh epochs, close/reopen and durable Resume refusals.
- `collaboration_targets_review_test.go`: missing-target/legacy-pointer
  Stop/close rollback, orphan runs, target-scope isolation, every-member consent
  and corrupt command/parent lineage.
- `internal/migrations/collaboration_targets_test.go`: populated upgrade keeps
  historical identity/proof; new legacy/unbound writes fail; target/group
  uniqueness replaces the old global limits; all populated/busy downs refuse
  and the empty maintenance rollback restores original indexes.
- `legacy_execution_boundary_test.go` in service and migrations: direct and
  persisted queue paths, stale finalization provenance, ordinary-child/chat/
  quick-create controls, rollback fencing and atomic creation visibility.
- `collaboration_execution_view_test.go` and handler execution tests: authorized
  snapshot, private-field omission, revoked/machine denial and legacy projection.
- Core `execution-schema.test.ts` and collaboration API tests: exact versions,
  unknown/malformed protocols, missing terminal evidence, complete lineage,
  private-log redaction, account/server epochs and old human-read compatibility.

Use task-owned PostgreSQL databases and guarded fake-agent tests. Native
containment, proof ingestion, resource-capacity scheduling, full @ submission
and Web/Desktop acceptance still require separate evidence.

## 7. Wrong vs Correct

Wrong: read `session.active_grant_id` when a waiting command reaches the front,
then clear `session.active_run_id` after its first run ends.

Correct: the command already names its typed target; the target owns its root
group, and that group retains the position until every descendant is settled.
Legacy session pointers are quarantine barriers only.

Wrong: ordinary enqueue is safe because the HTTP comment route rejects shared
issues, or because the caller supplied an ordinary source snapshot.

Correct: revalidate the persisted content source at service/SQL/finalization
boundaries, while preserving an ordinary child's independent identity.
