# Research: Lane FIFO, squad batches, Switch, M08 and M11 after M09

- Query: Map the verified single-agent foundation to the next lane/squad slices, with concrete missing primitives and ownership independent of the hold/control work.
- Scope: internal; source inspection only, no production changes or git operations.
- Date: 2026-10-18
- Fully reread: `research/execution-contract.md` (207 lines), `research/data-contract.md` (58 lines), `research/squad-delegation-2026-10-17.md` (34 lines). Also read the 2026-10-18 follow-up and current server specs. These contracts take precedence over old implementation assumptions.

## Findings

### Existing functionality and its actual boundary

| Area | Existing implementation / evidence | Boundary still missing |
| --- | --- | --- |
| Actual-root lane identity | `server/internal/service/collaboration_batches.go:96` resolves A or squad leader; `:112` ensures `collaboration_channel(session_id, root_agent_id)`. This is the current storage name for the contract's lane. | Command order is still per target, not per channel. |
| Immutable human inputs | `collaboration_batches.go:23` projects each command's own author/participant/account/lifecycle epochs; `:227` creates a separate command. `collaboration_runtime.go:270` rechecks every active input author. | A group/run needs all contributing target/config/grant snapshots, not only current target authorization and one root resource. |
| Waiting coalescing | `collaboration_batches.go:169` scans waiting batches and coalesces direct A with one S. `collaboration_batches_test.go:129` verifies storage coalescing and a second same-leader squad getting another batch. | Selection happens at enqueue, before promotion-time eligibility. It does not implement M06/M07 FIFO. |
| Direct-agent promotion | `collaboration_runtime.go:438` locks, verifies all inputs, freezes a one-resource roster, creates group/run/reservation and starts the entire batch atomically. | `:27` explicitly rejects squads; `:479` requires channel root to equal direct `target.ResourceID`; SQL hardcodes `step_kind='agent'` (`collaboration_runtime.sql:64`). Keep these gates until squad execution is proved. |
| Group proof barriers | `collaboration_groups.go:16` validates retained inputs, roster, ancestry and every delegation tombstone; `:106` stops every run; `:153` releases only proven reservations and settles the group. | Members/continuations are represented in schema and validation, but there is no production creator/wakeup path. Existing child-run control fixtures do not prove dispatch. |
| Managed machine slot | `daemon/daemon.go:4855` takes the real semaphore before collaboration list/claim. `daemon/collaboration.go:219` retains unknown runs' slots; otherwise it releases after Runner returns. | Poller supports one configured runtime/executable and one active collaboration run (`collaboration.go:18`, `:185`). Server namespace reservation is intentionally exclusive (`collaboration_runtime.sql:48`). This is M09, not full multi-target capacity. |

Paths abbreviated in this document: service files are under `server/internal/service/`; SQL query files under `server/pkg/db/queries/`; daemon files under `server/internal/daemon/`.

### Next independent slice: lane sequencing and eligible batch selection

Hold/M04 is a prerequisite. The FIFO owner should consume the new hold semantics, not independently change Resume/revoke behavior.

1. **Add a channel-owned command sequence.** `collaboration.sql:162` increments `collaboration_target.next_command_seq`; migration `628_collaboration_command_target_sequence.up.sql:1` makes `(target_id, seq)` unique. `collaboration_batches.sql:49` sorts direct batches by target seq but mixed batches by transaction-start `created_at`. The M09 timestamp regression is fixed only for the direct case. New input order must be allocated under the channel lock and compared numerically, including A/S aliases.
2. **Select at promotion from ordered individual waiting commands.** Current `appendInput` chooses the first compatible historical waiting batch (`collaboration_batches.go:175`), so `S1, S2, A` places the last A into S1's batch. `S1, S2, S1` also appends the last S1 backwards. Contract selection must stop at the second *eligible distinct squad*; later commands cannot jump over it. Existing partial unique index `686_collaboration_batch_waiting_target.up.sql:1` permits only one waiting batch per channel/effective-target, reinforcing the old coalescing model.
3. **Skip rather than reject the entire lane.** Promotion currently requires `batches[0].EffectiveTargetID == target.ID` (`collaboration_runtime.go:500`) and returns on any invalid batch author or target (`:512`, `:537`). Contract behavior: skip held, invalid, or already-joined waiting commands; mark invalid configuration/grant targets held in the same transaction; remove invalid unstarted author inputs through their prescribed cancellation path. Neither a skipped target nor an already-joined command truncates the scan. A normal domain refusal must not roll back a hold that is meant to persist.
4. **Freeze only the selected input set and complete grants.** `StartCollaborationBatchInputs` currently starts every waiting command in a prebuilt batch (`collaboration_runtime.sql:82`). Change selected commands/group association atomically with occupying the channel and creating the root intent. A/S coalescing uses S's leader environment; every command keeps its own typed target and author.

Selection examples to pin in tests (numbers are authoritative lane sequence):

| Input order / eligibility | First promoted set |
| --- | --- |
| `S1(1), S2(2), A(3), S1(4)`; all eligible | Only `S1(1)`; S2 is the cutoff, A and later S1 stay behind it. |
| `S1(1), S2-held(2), A(3), S1(4), S3(5), A(6)` | `1,3,4`; skip 2 without cutoff, stop at eligible S3, retain 6. |
| `S1-invalid(1), A(2), S2(3)` | Hold S1 atomically; select A+S2 using S2's leader role and full grants. |
| A command already joined to an active group appears among waiting rows | Skip it; it must not be read again or block/cut off a later eligible batch. |

**Persistence blockers shared with Switch/M11:** migration `680_collaboration_remove_legacy_execution.up.sql:47` freezes command seq and prohibits `in_flight -> waiting`; `681_collaboration_channels_batches.up.sql:132` freezes command channel/batch; `680:70` freezes group roster. Implement the reviewed command/group relationships with forward migrations, preserving immutable target/content/author/epoch identity and historical run/proof identity. Do not disable all identity guards just to allow resequencing or unread detachment. Main session owns these migrations, index-recovery registration and sqlc generation.

### Switch is currently configuration replacement, without lane movement

`collaboration_authority.go:591` requires a paused target, no active pointer and no unsettled target group/run; validates actual owners/configuration; revokes old consent; updates roster/epochs. `collaboration_authority.sql:13` leaves target paused. This preserves the typed target and old commands, but:

- It neither resolves/binds the new root channel nor migrates waiting commands/batches. A later append fails `configured target changed its root channel` at `collaboration_batches.go:121`.
- The contract requires UUID-ordered old/new lane locks, whole-transaction retry on a new-lane unique conflict, `switch_hold`, stable relative waiting order appended at the destination tail, and unchanged old-lane state. New S inputs must immediately use the destination lane.
- Resume S must validate S's own new complete grants and release its hold, without substituting the destination's current head-batch authority. The hold/control owner should supply this policy; the Switch owner supplies movement and preservation tests.

### Grant sets and authors are the bridge to squad execution

Current full-roster authorization is useful: `collaboration_authority.go:352` checks every resource's consent, actual owner participation/account epoch, configuration fingerprint and managed/enrollment identity. `:495` implements idempotent explicit owner consent. `collaboration_authority_test.go:100` covers complete squad consent; `:185` covers reconfiguration needing new consent.

However, `collaboration_runtime.go:549` chooses only the effective direct target's grant, `:580` freezes a single `GroupResource`, and `internal/collaboration/execution.go:32` has no per-target configuration identity or input dependency. `currentAdmission` (`collaboration_runtime.go:319`) checks the actual run grant plus *current* authorization for other input targets. It does not freeze and validate an A+S grant set. `collaboration_invalidation.go:71` discovers dependencies from the existing roster; its final `stopTargets` call (`:117`) uses the old channel-pause behavior.

Add persistent group target/config/grant dependencies before launching A+S; reuse the current owner authorization checks. M11 needs provenance for grants added only by individual unread inputs, so removing one such input cannot remove a grant still required by the original batch, another joined input, or a running member. Never replace all authors with the first command, resource owner, leader, or latest commenter. Current non-root author invalidation regression is `collaboration_runtime_test.go:282`.

### M08 comment delegation is not yet implemented for collaboration runs

- Ordinary behavior is documented by `handler/squad_briefing.go:39`, `:67`, `:74`: comment mentions, end the turn, wake on updates/completion, optionally no_action. Preserve this text/CLI. `handler/comment.go:2932` uses the issue's squad assignee for fallback; collaboration's human handler makes that path unsuitable. Ordinary attribution retains the parent human at `internal/attribution/attribution.go:223`.
- Collaboration comments currently use a human-only transaction (`collaboration_comments_create.go:13`, `collaboration_comments.go:229`). The `mcb_` capability is accepted only by the outcome endpoint (`handler/collaboration_runtime.go:166`; service capability uses are confined to outcome/permit). There is no narrow business-token context/comment/squad-activity path yet. Search found no implementation of `collaboration_subtask_unsupported`.
- Retained delegation schema is from the withdrawn structured phase model: `620_collaboration_targets.up.sql:45` stores request_key/phase_key/members/continuation_key, and `638_collaboration_delegation_request.up.sql:1` deduplicates parent/request. No source-comment/member identity or queued-continuation uniqueness exists. Production code only locks/cancels existing rows; tests insert synthetic intents (`collaboration_targets_test.go:221`). Adapt the internal ledger to comment/member dispatch; do not add a new delegate command or system comment protocol.
- Needed entry semantics: current live group run + frozen member roster + grant checks; `comment_id + member` dedup; leader mention coalesces continuation; editing does not dispatch; run comments mentioning current human participants follow R52; other machine mentions stay inert. Members and continuations use independent runs/permits/nonces and actual resources, outside external lane FIFO.
- The ordinary tests `handler/squad_worker_comment_wakes_leader_test.go:31`, `:109` cover wake/coalescing on ordinary tasks, not collaboration. Reuse the intended behavior, not the old assignee-based authorization.

### M11 needs a persisted unread/consumed boundary, not another enqueue branch

- No current command-to-consuming-run relation or persisted per-run prompt snapshot exists. Claim (`collaboration_runtime_capabilities.go:189`) recomputes all batch inputs; promotion already changed them all to in_flight. `validateGroup` (`collaboration_groups.go:42`) rejects waiting inputs in an active batch. Simply adding waiting commands to its current batch is incompatible with the validator and claim replay.
- Human joining must retain gate `FOR SHARE` **before** lane/group locks. Current runtime source locking already takes gate first (`collaboration_runtime.go:121`), but takes targets before channels (`:165`). Content locking takes targets then channels with **no gate** (`collaboration_comments.go:217`). Do not acquire the gate late inside `appendInput`; update the applicable entry's lock acquisition and revalidate lane/group after lock. R45 cancellation must remain gate/grant independent, but lock the joined command's group (`collaboration_queue.go:111` currently locks only command plus aggregate).
- Admission requires an unsealed/unsettled squad group with live/queued work, target S or its current leader A, no hold, valid complete authority, and no earlier unjoined eligible waiting input. Another same-leader squad stays queued.
- Claim must atomically persist the continuation's prompt input set and mark just those joined inputs in_flight; replay must return that same snapshot. Only a valid `never_launched_proven` for the consuming run restores unread status. Failed exit, missing started report, transport failure, or later run success does not.
- Stop/normal settlement/permanent continuation prepare failure must detach unread inputs back to waiting at their existing sequence, or cancel them for removed authors/closed issues. Only-unread added-grant invalidation returns that input to waiting and holds its target; unread author removal/cancellation removes only that input. Already-read author/grant invalidation stops the group. `collaborationBatchContainsAuthor` (`collaboration_invalidation.go:126`) is useful for read inputs but does not implement these detach/dependency rules.
- Settlement is still direct-only: `AcceptRuntimeProof` seals completed agent groups (`collaboration_runtime_capabilities.go:459`); `SettleCollaborationProvenGroup` requires an agent success and every run success (`collaboration_runtime.sql:150`). M08/M11 require locking lane→group, proving no pending/live work before normal seal, preserving first forced disposition, and using the final leader outcome despite an earlier member failure.

### Real capacity evidence to preserve and extend

`TestCollaborationBatchPollerSlotsAndRunnerLinux` (`daemon/collaboration_test.go:187`) uses a real semaphore of 1 and a test executable; it proves pre-claim occupancy and checks no release while its proof request awaits ACK (`:350`). Runner observes containment exit before outcome/seal (`collaborationrun/runner.go:90`, `:114`). Unknown state retains capacity. Server reservation release is proof-bound (`collaboration_runtime.sql:163`). Ordinary agent capacity also counts retained reservations (`agent.sql:1892`); `task_collaboration_capacity_test.go:61` covers this interaction.

This does **not** prove leader→member→continuation at capacity 1. Queue members independently of parent proof; their actual start depends on their own resources. A leader must exit and finish local cleanup before its real slot is reused. Do not implement handoff by setting DB `waiting`, ignoring a still-live leader, or allocating capacity once per runtime. Test a delayed remote parent proof, local slot release, retained unknown barriers, shared members and exclusive directories separately. A provider that cannot perform the allowed business-token comment and end its turn must not advertise shared-squad support.

## Recommended bounded sequence and ownership

| Order | Slice / owner | File boundary and acceptance |
| --- | --- | --- |
| 1 | Hold/M04 owner (already planned by main) | `collaboration_control.go`, `collaboration_targets.go`, `collaboration_invalidation.go`, relevant authority/receipt/view paths. Complete target hold versus lane pause and partial Resume semantics first. |
| 2 | Main: schema/index/sqlc integration; FIFO owner: enqueue/promotion | Main owns `server/migrations/`, `server/cmd/migrate/main.go`, generated DB files and shared SQL interfaces. FIFO implementation owns `collaboration_batches.go`, its tests, and the promotion part of `collaboration_runtime.go`. Prove cross-target sequence, eligibility skip, second squad cutoff, selected-input atomicity, cancellation/promotion races. Keep runtime direct-agent gate. |
| 3 | Switch owner, after the same schema and hold model | `collaboration_authority.go` reconfigure path and focused Switch tests; coordinate `ensureTargetChannel` with FIFO owner rather than concurrent edits. Prove old/new lane lock races, destination order, immutable input authors, old lane unchanged and new consent/hold behavior. |
| 4 | Group-authority/M08 owner, sequential with promotion changes | `collaboration_groups.go`, runtime/group grant checks, dedicated collaboration business-comment/delegation service + handler/SQL; parent owns shared route/protocol integration. Prove comment/member dedup, at most one queued continuation, complete grants, activity/context capability scope, stop/seal race and final-leader settlement. |
| 5 | Daemon verifier/implementer, once M08 APIs are stable | `daemon/collaboration.go`, `daemon/collaborationrun/`, their tests and real-router Linux fixture. Prove capacity=1 handoff/member/continuation with test executables before relaxing squad capability gate. |
| 6 | M11 owner, after queued continuation and dependency model | `collaboration_comments_create.go`, claim snapshot/consumption, cancellation/invalidation/settlement integrations. Prove unread versus read matrix and same-key claim replay; serialize changes to shared group/runtime/control files. |

The useful independent work alongside hold/M04 is to prepare lane-selection cases and a bounded FIFO patch against agreed SQL/hold interfaces. Do not let that owner independently alter Resume, grant revocation, shared migrations or regenerate sqlc. Full UI/provider redesign is not part of this map.

## Tests and related specs

- Preserve `collaboration_batches_test.go:12` overlapping-transaction direct FIFO, `:103` separate immutable authors/cancellation, `:153` next-batch behavior. Replace the deliberately deferred mixed-order expectation at `:78` with authoritative lane order when that slice lands.
- Extend `collaboration_authority_test.go:185` to a real leader change and waiting migration; the existing test only changes configuration/consent.
- Preserve `collaboration_control_test.go:76` orphan cancelled delegation rejection and `collaboration_targets_test.go:213` all-descendant stop proof. Add real creation/claim paths to supplement their synthetic fixtures.
- Run targeted DB tests/race for queue, authority, control and runtime, plus actual Linux capacity tests. Use deterministic lock/channel barriers; no installed provider CLI.
- Related specs: `.trellis/spec/server/collaboration-targets.md` (current M09 limits), `collaboration-foundation.md` (human/access invariants), `project-execution-squad.md` (ordinary squad boundaries), root `CLAUDE.md` (migrations, test fixtures, generated code), `.trellis/spec/guides/cross-layer-thinking-guide.md` (future DTO changes).
- External references: none required; this is an internal contract/code comparison, not a new SDK decision.

## Caveats / Not Found

- No tests were executed in this research pass. Prior M09 passing counts are the user's recorded evidence, not new validation of the pending work.
- No production M08 dispatch/continuation creator, M11 consumption snapshot, target hold field, lane-wide command sequence or Switch waiting migration was found in the inspected current collaboration paths. Existing storage types, ordinary squad tests and v3 alias projection do not imply those behaviors exist.
- Historical mixed-target order was never authoritative: forward migration cannot reconstruct the exact lock-acquisition sequence from transaction timestamps. The main migration owner must document retained-row handling without deleting unresolved group/run proof barriers or changing historical target/author identity.
- Source paths and line numbers describe the read baseline. Other agents may subsequently change them. Only this research artifact was written.
