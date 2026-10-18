# Collaboration channels and supervised single-agent execution

## 1. Scope / Trigger

Apply this contract to collaboration input, control, authorization, execution
projection, and the managed daemon path. The current M09 slice supports one
direct agent with a configured static Linux executable. Dedicated comment,
consent, control and runtime APIs exist; public creation and client execution
affordances remain disabled pending complete product acceptance.

This is the implemented slice, not the complete multi-target design. Target
holds, cross-target FIFO, same-leader squad execution, M11 continuation and full
Web/Desktop delivery remain pending. Before starting those areas, reread the
latest [execution contract](../../tasks/10-10-cross-workspace-collaboration/research/execution-contract.md)
and current task artifacts. In particular, do not extend current channel-wide
resource refusal into the final target-hold policy.

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
  target-set version and execution gate epoch for Resume.
- `ExecutionView`: the authorized repeatable-read issue snapshot includes the
  v3 channel/batch projection consumed by `packages/core/collaboration`.
- Managed runtime list/claim/permit, `RecordRuntimeOutcome`, stop-intent and proof:
  `/api/collaboration/runtime/runs` and its run-scoped subresources.

`validateGroup`, `stopGroup` and `settleProvenGroup` share retained-ledger checks.
Departure, invalidation and lifecycle writers borrow the same transaction.
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

Target Resume cannot bypass a paused issue. Issue Resume rejects global
lifecycle/gate/epoch/authority/scope failures atomically; otherwise it returns
per-channel results and preserves unresolved barriers. Old rejected keys replay
the original refusal, never a future resume intention. A healthy explicit
Resume continues the next waiting batch; it does not rerun stopped inputs.

Current resource/consent failures still operate within the M09 channel model.
The later hold model separates target eligibility from lane pause, including
partial hold-release receipts; it is not implemented by this document's current
runtime or v3 reader. Before that extension reread the latest task contract.

### Read projection and account cache

The issue's `execution.protocol_version` is **3**. Targets carry nullable
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
