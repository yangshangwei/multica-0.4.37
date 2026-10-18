# Collaboration holds, channel controls and supervised execution

## 1. Scope / Trigger

Apply this contract to collaboration input, control, authorization, execution
projection, and the managed daemon path. The current M09 slice supports one
direct agent with a configured static Linux executable. Dedicated comment,
consent, control and runtime APIs exist; public creation and client execution
affordances remain disabled pending complete product acceptance.

Independent target holds, M04 channel/hold receipts and resource teardown
invalidation extend the M09 foundation. This is not the complete multi-target
design: mixed A/S waiting-batch eligibility and channel-wide FIFO, full frozen
multi-target grants, leader switching, external agent/squad configuration-writer
convergence, M11 and full Web/Desktop delivery remain pending. A held S can still
block a pre-coalesced A/S waiting batch; channel activation alone does not prove
that mixed waiting work can advance. Before starting those areas, reread the
latest [execution contract](../../tasks/10-10-cross-workspace-collaboration/research/execution-contract.md)
and current task artifacts. Keep the current direct-agent runtime capability
gate until the corresponding execution and capacity paths have evidence.

## 2. Signatures

`CollaborationService` owns the transaction boundaries:

- `RegisterTarget` / owner consent: creator or handler organizes a target;
  each actual resource owner must already be a valid issue participant and
  approve their own current resource/configuration.
- `AppendCollaborationInputInTx`: the comment writer appends immutable inputs
  in the same transaction as comment, per-target receipts and notifications.
- `SubmitCommand` / `CancelWaitingCommand`: explicit target/input identities,
  captured actor/participant/lifecycle versions and idempotency key.
- `StopQueue` / `ResumeQueue`: observed issue/target controls, group generation,
  target-set version and execution gate epoch for Resume. Each Resume target
  observation also carries `expected_hold_version`.
- `GetControlOperation`: current human/account/object authorization before
  returning the original full stop/resume receipt from
  `GET .../control/operations/{requestId}`. Human workflow operation lookup
  retains its separate response contract.
- `ExecutionView`: the authorized repeatable-read issue snapshot includes the
  v3 channel/batch projection consumed by `packages/core/collaboration`.
- Managed runtime list/claim/permit, `RecordRuntimeOutcome`, stop-intent and proof:
  `/api/collaboration/runtime/runs` and its run-scoped subresources.

`validateGroup`, `stopGroup` and `settleProvenGroup` share retained-ledger checks.
Departure, invalidation and lifecycle writers borrow the same transaction.
`InvalidateCollaborationRuntimeInTx(ctx, qtx, runtimeID, reason)` and the exact
binding invalidator save holds and stop intents before resource cleanup.
`WithRuntimeResourceTransaction(ctx, starter, queries, mutate)` retries only
confirmed rolled-back lock/serialization conflicts, at most three attempts;
commit errors never enter the retry branch.
Ordinary execution still uses `checkOrdinaryExecutionSource`,
`checkIssueExecution`, SQL `lock_issue_execution`, and finalization revalidation.
A parent reference never grants ordinary execution access to shared content.

## 3. Contracts

### Inputs, channels and migrations

A permanent issue session owns global pause/close and event order. Targets keep
typed identity; a channel is keyed by session and actual root agent. Each channel
holds at most one unsettled group. Each input retains its own command ID,
author/epochs, target, body and sequence; one batch can contain multiple inputs.
Starting a batch freezes all eligible inputs. Cancelling a waiting input removes
only that input; ending an already-started batch settles the batch together.

For current direct-agent batches, `ListCollaborationBatchInputs` orders by the
target's locked enqueue sequence. PostgreSQL `now()` is transaction-start time:
an earlier transaction acquiring the queue lock later must not move its input
or author to the front. Mixed/squad ordering is explicitly deferred; independent
target sequence values are not a global lane sequence.

Migration 680 removes unpublished legacy execution data and legacy session
pointers through a forward migration. Do not rewrite migrations 571–650 or
restore legacy quarantine/mapping branches. Current input/proof identities and
human history survive; the destructive cleanup cannot be undone with a down
migration. Migration 681 and following indexes add channels, batches, attempts
and retained reservations.

Every index build has a separate concurrent migration and an explicit entry in
`cmd/migrate/main.go`'s existing cleanup registry. A successful empty migration
does not prove recovery from an interrupted invalid index. No foreign keys or
cascades; transactional service code owns relationship checks and cleanup.

Migration 698 adds nullable `hold_reason` and positive BIGINT `hold_version`.
NULL means released; the only held values are `initial`, `grant_required`,
`config_invalid` and `switch_hold`. Registration starts held even when an alias
joins an active channel. New consent, reconsent, configuration changes and hold
transitions advance the target's hold version independently of channel control.
An exact idempotent replay never advances it. Reconsent does not release hold.
Retained active intent and all group/run/proof identities survive upgrade.

### Authorization and execution identity

Actual resource owners are full issue participants, not an authorization-only
role. Freeze their participant/account/configuration/binding epochs separately
from every input author's epochs. Recheck all batch authors before launch;
using the earliest author as the run root does not replace the other authors.

The current launch path requires a valid managed installation/MDT, an admitted
Linux runtime, a direct agent and actual resource capacity. Legacy enrollment
and squad storage are not permission to launch through this slice. Ordinary
agent/PAT/plugin paths cannot obtain collaboration business or proof authority.

The shared runtime DTO remains protocol **2**, independently of the UI execution
projection's protocol **3**. `CollaborationRunIdentity` binds deployment,
installation, issue/session, target/channel/batch/group, run/fence, claim
generation and launch nonce. Business outcomes use shared protocol constants
`success` / `failure`; command/group terminal states remain `succeeded` / `failed`.
Never substitute one vocabulary for the other.

`MULTICA_COLLABORATION_RUNNER_CONFIG` identifies the configured runtime,
executable/arguments, sandbox and ledger directories. The managed ledger path is
installation-scoped under `.multica/management/<deployment>/collaboration-runs/<installation>`.
Its cross-process lock, fsync, nonce burn and final seal survive restart. An
uncertain permit/fork cannot be retried as a fresh run or release capacity.

The Linux controller probes real user/PID/mount containment before advertising
support. Kernel namespace destruction supplies descendant termination evidence;
PGID, closed stdio, a missing PID or ordinary task completion do not. macOS and
Windows remain unsupported for this runtime path. Default tests use only
self-created executables; general provider CLIs and file/Git preparation are
separate work. Termination credentials stay with the supervisor and never enter
the provider process.

### Stop, proof and explicit Resume

Stop seals expansion and retains every possibly live run's barrier. A
never-permitted run may be settled by the server only with zero permits, no
started evidence and no contradictory success/running evidence. Otherwise only
an authenticated sealed proof can settle it. Business outcome and termination
proof are independent; exit code zero is not a business success.

Before releasing any reservation or settling a group, `validateGroup` checks
input/batch/run/roster identities and **every** retained delegation's parent,
including cancelled tombstones. Normal proof settlement uses the same check as
Stop. An orphan/corrupt ledger fails closed and rolls back the complete write.

Target Resume cannot bypass a paused issue. It validates the selected target's
current authority, captured hold version and applicable channel proof before
changing either hold or channel state. Refusal may save its operation receipt,
but changes no business state. A healthy active/busy channel needs no new proof
just to retain its intent; a forced-stopped current group with missing proof
still blocks hold release. Contradictory channel/target mirrors refuse rather
than being silently repaired. No hold plus active channel returns `already_active`.

Issue Resume rejects global lifecycle/gate/epoch/authority/scope or stale-hold
failures atomically. Otherwise it activates the session, resumes proof-ready
channels, and separately releases valid target holds. Invalid grants keep only
their target held; unresolved channel proof remains a barrier. Receipts have
unique `channels` and `target_holds` decisions plus `issue_id`, `request_id` and
`action`; unresolved run counts describe the retained ledger. Original-key
replay/GET recovery returns the original success or refusal, never a future
intention. Stop HTTP requests omit Resume-only gate/hold fields.

Resource-only invalidation holds affected current targets and calls `stopGroup`
for exact frozen dependencies, leaving channel pause/control untouched. Author
departure still pauses channels whose consumed batch contains that author.
Account invalidation discovery includes non-root in-flight batch authors; root
run attribution alone is insufficient. Current list/promote/claim/permit gates
reject held inputs even after grants become valid. A deliberate authoritative
promote refusal must commit its discovered hold and audit before returning the
refusal; arbitrary errors, capacity/offline conditions and failed writes do not
commit a hold. Keep the common read/authorization helper free of hidden writes.

Actual runtime deletion, profile deletion, legacy runtime merging, GC and
managed binding replacement use the borrowed resource invalidator. Resource-first
writers acquire every source child fence with NOWAIT and retry the whole
transaction only after rollback. Callbacks contain database work only; publish
after commit. Normal same-authority bind/renewal is unchanged. Retained grants,
group/run dependencies and reservations, not just engine task rows, determine
whether GC has unproven work. Historical termination proof remains usable after
runtime/grant deletion. None of this substitutes for the deferred mixed FIFO.

### Read projection and account cache

The issue's `execution.protocol_version` is **3**. It includes explicit
`channels`, same-snapshot `execution_gate {enabled, epoch}`, and each target's
required `hold_reason`/`hold_version`. Target queue/control/pause/group mirrors
must equal their channel. Missing/unknown hold facts never default to released.
Targets carry nullable
`channel_id` only while unused; commands/groups/runs carry matching channel and
batch IDs. Multiple inputs may share an active batch, and same-channel target
aliases may share the active group. A cancelled waiting input retains its batch
identity without becoming an executed input. Missing rows, impossible lineage,
unknown versions and terminal groups without evidence remain unavailable.

No roster, grant, nonce, installation credential, working path or private input
body is exposed by this projection. Versions remain exact signed-64-bit decimal
strings. Ordinary issue revisions remain numeric. A missing old-server execution
object preserves human reads with `execution=null`; an unknown present protocol
is not silently accepted. Client `availability` and execution affordances remain
unavailable until full acceptance, independently of internal participant rights.

Both success and error paths in `protectCollaborationRead` compare the captured
account session and object access epoch. A delayed old denial after recovery
returns `AbortError`; it must not clear the recovered cache or revoke its new
epoch. Parsing and transport errors redact private values.

Core owns distinct control input/receipt schemas and explicit send/recover
helpers. Control drafts retain the original key and captured observations under
`payload.domain=execution_control`; they cannot parse as human workflow drafts.
Recovery uses GET only, keeps a missing outcome pending and never sends a fresh
POST automatically. A receipt is historical: refetch the authorized live state
instead of optimistically projecting its old controls or holds into Query.

## 4. Validation & Error Matrix

| Condition | Result |
| --- | --- |
| Invalid target/input identity or conflicting input-event reuse | Reject; no guessed target or duplicate input |
| Target/configuration, scope, gate or actor snapshot changed | Stable stale/refusal receipt; no implicit retry with newer authority |
| Unknown/malformed v3 projection, missing batch/run evidence | Private read unavailable; no execution fallback |
| Business outcome uses `succeeded` instead of `success` | Client rejection or `invalid_runtime_outcome`; no recorded outcome |
| Outcome exists without proof | Reservation and execution barrier remain |
| Owner disabled after launch | Business/list access denied; original minimal termination proof still accepted |
| Resume with missing proof | Target remains paused with `stop_unconfirmed`; issue-scope receipt is explicit |
| Old hold snapshot after revoke/reconsent | `stale_hold`; no control/hold changes or future intention |
| Resume All with one invalid target grant | Valid lane can resume; that target retains hold with its own refusal |
| Resource-only revocation of an unused same-channel alias | Target held; unrelated healthy group and channel control unchanged |
| Source lock conflict during resource teardown | Entire resource and collaboration write rolls back before bounded retry |
| Unknown resource commit result | No replay of the mutation; preserve original proof/operation recovery |
| Orphan delegation, including cancelled state | Group validation fails before reservation release |
| Old access denial after object epoch recovery | `AbortError`; current data/epoch retained |
| Unsupported controller, uncertain permit/burn/fork | No new fork; retain unknown state and capacity |
| Interrupted concurrent index creation | Existing registered cleanup removes the invalid remnant before retry |

## 5. Good / Base / Bad Cases

- Good: while input 1 runs, inputs 2 and 3 from different people join one next
  batch; Stop, proof and a new Resume produce one run containing both authors.
- Good: ordinary human editing/handoffs preserve execution. Test fixtures must
  start the first batch **before** appending inputs meant to remain waiting.
- Good: stale account invalidation cancels obsolete waiting input while a fresh
  grant/current run remains untouched. Never seed a purported fresh run that
  has already consumed obsolete authors from the same waiting batch.
- Base: ordinary tasks, children with reference-only parents and source-member
  read rights keep their existing boundaries.
- Bad: accepting a successful exit as proof, using one member's proof for a
  whole group, or reopening the ledger because another daemon process starts.

## 6. Tests Required

- `collaboration_batches_test.go`: overlapping transaction order must not change
  claimed input order/root author; preserve the explicitly deferred mixed path.
- `collaboration_control_test.go`: cancelled orphan delegation refuses settlement
  with an identical before/after snapshot; Stop/proof and scope races retain
  their barriers.
- `collaboration_hold_test.go`: initial/reconsent holds, stale hold versions,
  selected-target atomicity, independent issue decisions, accurate proof counts,
  non-root author discovery, committed promote refusal and write-fault rollback.
- `collaboration_resource_invalidation_test.go`: actual deletion/rebind callers,
  exact retained dependencies, all NOWAIT source fences, ordinary renewals,
  rollback/unknown commit and historical proof after resource loss.
- Migration 698 tests: old active/paused intent, narrowly recognized old
  target-only reconfiguration mirrors, idempotent rerun and guarded down.
- Route fixtures bind targets through `BindCollaborationTargetChannel` instead
  of independently seeding target/channel pause defaults. Real registration,
  consent and detail GET must agree with the same strict projection.
- Workflow/lifecycle/account tests: realistic running versus waiting batches,
  actual external actor/resource owner separation, original request replay and
  unchanged proof identities. Fault injection must cover `Query` as well as
  `Exec` / `QueryRow`, including channel mirroring writes.
- Runtime outcome HTTP and v3 service/handler/core tests: shared wire values,
  exact versions, channel/batch linkage, malformed responses and private logs.
- Core `access.test.ts`: a recovered epoch survives an older denied request.
- `TestCollaborationSingleAgentRuntimeRoutesLinux` and owner-disable counterpart:
  real DB/router/Runner, actual kernel proof, merged next input and never-launch.
- `TestCollaborationBatchPollerSlotsAndRunnerLinux`: managed registration,
  pre-claim machine slot, real Runner and release only after sealed proof.
- Linux collaborationrun suite: detached/double-fork descendants, early stdio,
  supervisor crash, nonce replay, ledger corruption and proof ACK loss. Record
  actual kernel/controller; Docker Linux evidence is not native macOS evidence.
- `TestCollaborationPartialResumeRoutesLinux`: two actual managed namespaces and
  native test providers; withheld A proof keeps its pause/hold/reservation while
  B resumes and executes the next input. Old refusal/partial GET receipts remain
  unchanged after proof; only a fresh explicit Resume releases A.
- M10 retention and migration cleanup registry/retry tests. Rollback rehearsals
  only run in disposable databases/schemas, never the application database.

## 7. Wrong vs Correct

Wrong: sort a direct-agent batch by `created_at`, or seed two waiting inputs and
then assert that starting the batch leaves its second input waiting.

Correct: use locked enqueue sequence; append the next batch after the current
one starts. Preserve separate input authors and immutable snapshots.

Wrong: `outcome: "succeeded"`, legacy execution projection v2, or proof settlement
that only checks nonterminal delegation rows.

Correct: business outcome `success`, execution projection v3, and shared retained
ledger validation before any resource release. These version and state domains
are independent.

Wrong: reuse channel pause for grant loss, release hold during a rejected target
Resume, or save a hold then return an error that rolls its transaction back.

Correct: persist target eligibility independently, validate target Resume before
any business-state write, and commit only the intentional authoritative hold
refusal while preserving rollback for real failures.
