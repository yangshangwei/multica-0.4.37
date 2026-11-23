# Research: Target holds, lane controls, invalidation and strict Resume

- Query: Map the current M09 paths and smallest changes for `initial` / `grant_required` / `config_invalid` / `switch_hold`, grant invalidation without lane pause, partial Resume All and atomic Resume target.
- Scope: internal; service, SQL, handlers, retained execution projection and meaningful regression coverage. No implementation or git operations.
- Date: 2026-10-18
- Baseline: parent reports HEAD `5a0817b8a`; evidence below is from the workspace source read in this research turn.

## Findings

### Authoritative requirements

Read the **complete** current `research/execution-contract.md` and `research/data-contract.md`, `acceptance.md`, `.trellis/spec/server/collaboration-targets.md`, root `CLAUDE.md` and Trellis workflow. Current contracts override older progress notes and existing test expectations.

- `execution-contract.md:137`: Resume target validates the selected target and its lane first, then atomically releases its hold and, if needed, resumes the lane. A healthy already-active lane may stay busy; absent hold means `already_active`. A forced-stopped group lacking proof, or contradictory run/group pointers, refuses with `stop_unconfirmed`. Invalid selected configuration/grant refuses without any control/hold/event changes. Session paused/closed refuses first.
- `execution-contract.md:139`: Resume All passes global guards first, sets session active and independently resolves paused lanes by termination proof and held targets by configuration/grants/forced-stop proof. Invalid grants do **not** keep an otherwise settled lane paused. Removed-target barriers remain physical reservations and do not prevent global active.
- `execution-contract.md:129` and `:132`: explicit Stop / departed input author pauses the affected lane; grant revoke / resource-owner departure only holds affected targets and stops dependent groups, leaving lane state unchanged.
- `execution-contract.md:49`, `:163`; `data-contract.md:15`: creation and consent/configuration changes establish or preserve hold; only explicit Resume releases it. Switch writes `switch_hold`, keeps typed identity and old proof identities, and moves a squad's waiting inputs to the new leader's lane in original relative order.
- Relevant acceptance: AC23–24 (refused key stays refused), AC34 (same-lane control), AC37–40 (revoke/renew/switch), AC49–50 (owner departure), AC95 (partial issue results), AC96 (complete roster consent).

### Current code and precise change points

| Path | Existing behavior / consequence |
| --- | --- |
| `server/internal/service/collaboration_targets.go:79` | `RegisterTarget` validates organizer/owners/resources and inserts at `:120`; `ensureTargetChannel` binds at `:126`. No hold field exists. |
| `server/internal/service/collaboration_batches.go:112` | `ensureTargetChannel` creates/reuses the actual root-agent channel, rejects a target whose root channel changes at `:122`, then binds. `BindCollaborationTargetChannel` copies channel active/pause state into a new target (`server/pkg/db/queries/collaboration_batches.sql:13`): a new alias of an active channel currently inherits active. |
| `server/migrations/620_collaboration_targets.up.sql:8`; `server/migrations/681_collaboration_channels_batches.up.sql:3` | Targets and channels both store queue/control/group mirrors; target defaults paused/new_target and channel defaults paused/new_channel. Neither has hold. Do not rewrite these migrations. |
| `server/internal/service/collaboration_authority.go:495` | `ConsentTargetResource` independently validates actual owner, participant/account epochs, resource configuration and managed/enrolled binding; inserts consent at `:573`. It does not release pause, but has no independent hold to preserve. Existing matching consent returns at `:565`. |
| `server/internal/service/collaboration_authority.go:352` | `authorizeCollaborationTargetInput` compares fingerprint, complete roster grants, owner account/participant epochs and registration. `:430` adds NOWAIT resource locks. This helper is reused by read/catalogue and Resume; it must remain free of hidden hold/stop writes. |
| `server/internal/service/collaboration_targets.go:35` | `RevokeGrant` locks the actual owner's grants, revokes one at `:60`, then calls `stopInvalidatedWork` with the **pre-revocation** row at `:63`. Keep that retained identity. |
| `server/internal/service/collaboration_invalidation.go:15` | Collects current invalid target grants, group frozen grants, root/batch authors and run grants. At `:117` routes all causes into `stopTargets`, conflating owner/resource invalidation with author removal. |
| `server/internal/service/collaboration_control.go:493` | `stopTargets` expands a selected target to its entire channel, pauses at `:548`, mirrors aliases at `:553`, and stops the channel group at `:570`. Correct for explicit Stop, incorrect for revoking an unused same-lane alias or a grant affecting only one target. |
| `server/internal/service/collaboration_groups.go:106` | `stopGroup` already seals expansion, cancels pending delegations, stops each unproven run and settles only if evidence allows. It does **not** need to pause a channel; reuse it for resource invalidation. |
| `server/internal/service/collaboration_workflow.go:233` | `depart` atomically revokes participant and owner grants, cancels that author's waiting inputs, then uses the same invalidation helper at `:259`. Split author-caused lane pause from owner-caused target hold here through the helper, not separate transactions. |
| `server/internal/service/collaboration_lifecycle.go:346` | Reopen pauses session/targets; close cancels waiting and calls whole-issue `stopTargets`. Preserve these explicit control semantics and any existing holds; proof must not clear them. |
| `server/internal/service/collaboration_authority.go:591` | `ReconfigureTarget` currently requires mirrored target paused and no active pointer (`:618`), scans historical target groups/runs (`:626`), validates new resources, revokes all old consent and replaces configuration (`:667`). It neither rebinds leader lanes nor validates staged new-version consent. |
| `server/pkg/db/queries/collaboration_authority.sql:13` | Reconfigure increments target/config/control versions and sets target paused/configuration_changed, without updating channel control. This independent target control increment conflicts with channel-authoritative checks. Replace the pause side effect with target `switch_hold` and explicit lane-move rules. |
| `server/internal/service/collaboration_control.go:175` | Resume has usable global guards, gate epoch, receipt and retry framework. Its `channelResumeReason` at `:356` validates the head waiting batch and its required targets, so one invalid target can refuse every alias. It is not the final selected-target/hold predicate. |
| `server/internal/service/collaboration_control.go:255` | Resume treats **all** unsettled group/run/pointer rows as lane barriers, regardless of healthy active versus forced-stopped work. Target Resume of healthy active execution currently returns `stop_unconfirmed`; latest E05 requires `already_active` or hold release. |
| `server/internal/service/collaboration_control.go:273` | Issue Resume already activates session and returns `partially_resumed`, but at `:304` pauses a channel for a target authority failure. At `:315` it repeats one channel result per target. Removed channels have no target entry, so their refusal is not represented explicitly. |
| `server/internal/service/collaboration_runtime.go:438` | Promote validates target/inputs, but failures at `:465`/`:537` return an error, and `collaborationRuntimeMutation:388` rolls back. Merely inserting a hold before returning the same error would silently lose it. Current candidate SQL also filters away missing grants before discovery (`server/pkg/db/queries/collaboration_runtime.sql:39`). |

### Account epochs and missing invalidation discovery

- Entry point: `server/internal/auth/password_revoke.go:21` calls `InvalidateCollaborationAccount` inside the existing credential transaction (`:27`). Writers include `server/internal/handler/auth_password.go:328`, `:419`, `server/internal/service/platform_account.go:220`, and `platform_admin.go:314`, `:577`.
- `server/pkg/db/queries/collaboration_invalidation.sql:7` atomically raises the neutral epoch and stores the original revocation key. It deliberately does not scan source issues under user locks.
- `server/internal/service/collaboration_account_reconciliation.go:159` selects one issue, acquires source NOWAIT fences, locks target/channel set and applies `invalidateAccount:218`; obsolete grants/inputs and cursor commit together. It calls `stopInvalidatedWork:237`. Preserve account/epoch ordering, fair retry and unknown-commit behavior.
- **Confirmed discovery gap:** `server/pkg/db/queries/collaboration_reconciliation.sql:40` finds waiting authors, grant owners and the **root** command author of unsettled runs/groups. It does not find a non-root `in_flight` batch author with no grant and no waiting input. `collaborationBatchContainsAuthor` (`collaboration_invalidation.go:126`) can stop such a group **only after the issue is discovered**. Add discovery through the batch's `in_flight` commands and their author epoch, with no grant-ownership assumption. Later M11 needs the analogous read-input membership.
- Retain the old-epoch/new-consent distinction: invalidation `:19` filters `OwnerAuthEpoch >= event epoch`, and current-target matching checks configuration at `:31`; obsolete retained grants may stop an old frozen group but must not hold or revoke a freshly valid current target accidentally.

### Configuration and resource teardown are not yet durable stop/hold paths

- `server/migrations/660_collaboration_resource_consent.up.sql:9` raises only row-local agent/runtime/squad configuration epochs. Presence and names are excluded. Fingerprint mismatch dynamically refuses authorization (`collaboration_authority.go:368`), but no durable `config_invalid` hold or affected-group stop is written by these triggers.
- `server/internal/service/runtime_teardown.go:59` locks runtime then agents, cancels/drains ordinary tasks, unbinds agents/tasks and removes local infrastructure. It has **no collaboration invalidation hook** and ordinary task count is not proof about a collaboration run/reservation.
- Callers already holding resources: `server/internal/handler/runtime.go:902` → teardown `:924` → delete `:949`; confirmed unbind/delete uses teardown `:1144`; profile deletion uses `server/internal/handler/runtime_profile.go:435`; GC uses `server/cmd/server/runtime_sweeper.go:512` → teardown `:562` → delete `:574`. Daemon namespace reconciliation directly deletes an old runtime at `server/internal/handler/daemon.go:1011`.
- Managed binding replacement: `server/internal/service/managed_installation.go:473` takes user/session, namespace, workspace and installation fences, then revokes the old binding at `:550` and its tokens at `:553`; no collaboration grant/hold/stop transaction is present. Ordinary renewal (`:533`) must preserve valid authority; actual rebind/epoch change must invalidate only affected grant generations.
- Smallest shared entry is a borrowed-transaction resource invalidator: discover relevant current grants plus historical group/run dependencies, acquire all affected issue/session/channel/target sets in UUID order, revalidate, revoke current grants, hold targets and stop dependent groups **before engine cleanup commits**. For existing resource-first callers use source **NOWAIT throughout** and whole-transaction retry; never resource-lock → blocking source wait. Prefer moving discovery/fencing before runtime locks where feasible. Automatic GC must keep its existing refuse-undrained semantics for possibly-live collaboration reservations.
- Existing proof-survival test `server/internal/service/collaboration_runtime_test.go:247` explicitly revokes first, then manually deletes runtime/grant rows. It proves retained proof authority, **not** that real teardown initiates invalidation. Add actual service/handler teardown coverage.

### Smallest safe implementation sequence

1. **Persist independent target hold.** Add a forward migration with nullable `hold_reason`, restricted to the four contract values; null means no hold. New targets default `initial`. Explicitly classify existing paused/new/reconfigured rows during backfill without clearing any live barrier. Avoid changing the `collaboration_channel` name solely to match the contract term “lane”. Channel control remains authoritative; existing mirrored target control fields must never store/overwrite hold. Update sqlc only through its source queries and a single generator owner.
2. **Separate cause and effect.** Keep `stopTargets` for explicit Stop/lifecycle/author lane pause. In invalidation collect three distinct sets: current target holds, exact dependent group IDs, and author-affected lane IDs. Resource-only invalidation calls `stopGroup` without channel pause; same-lane unrelated current groups remain untouched. Combined author+owner loss pauses only author-affected lanes and still holds every invalid current target. Keep immutable history, first forced disposition, per-run proof and reservations.
3. **Consent/configuration.** Registration writes `initial`; new/replacement consent preserves an existing hold and must not turn eligibility into execution permission. Revoke writes `grant_required`; genuine configuration/registration drift writes `config_invalid`; reconfiguration writes `switch_hold`. Repeated current consent must not accidentally clear any cause. Do not add writes to the common read/authorization helper; callers owning invalidation/promote transactions persist holds deliberately.
4. **Strict target Resume.** Resolve global authority/session/gate/scope and selected target configuration first. Compute the full lane barrier/result in memory, validate every condition, then release only the selected hold and optionally resume its paused lane once. Active healthy lane stays intact, even if an unrelated alias is busy/invalid. Active forced-stopped group without proof still refuses. Invalid selection + proof-ready paused lane must leave the lane paused; valid selection + missing proof must retain its hold. A refusal may persist its required operation receipt, but no control/hold/event state: E02 durable refusal and E05 “zero writes” are consistent at that business-state boundary.
5. **Partial issue Resume.** After global checks, set session active. Evaluate lane proof separately from target authority. A lane with no unresolved stop resumes even when one target remains `grant_required`; another target on that lane can proceed. Independently inspect held targets even on already-active preserved lanes. No new groups, claims or reservations. Use `partially_resumed` whenever a lane/hold remains blocked; do not claim all restored.
6. **Promote/teardown convergence.** Candidate discovery must still find invalid waiting targets to materialize holds. Return a committed refusal/result when promote repairs target eligibility; do not return an error that rolls back the repair. Hold-skipping within a pre-coalesced mixed batch cannot be achieved by a single SQL filter: append currently chooses batches before promotion (`collaboration_batches.go:175`) and batch identity is immutable. Coordinate with the FIFO/batch owner before allowing cross-target execution.

### Candidate wire shape and read ownership

Retain the established `channel_id` wire vocabulary. Add `hold_reason: null | "initial" | "grant_required" | "config_invalid" | "switch_hold"` to target projections and mention/waiting feedback. Its meaning is independent of `control.queue_state`, which describes the channel. Do not put grants, roster or private input into receipts.

For control results, separate each channel transition from each checked held target. A concrete compatible-to-current-naming shape is:

```json
{
  "code": "partially_resumed",
  "scope": "issue",
  "queue_state": "active",
  "control_version": "7",
  "target_set_version": "4",
  "channels": [
    {"channel_id": "<lane UUID>", "code": "resumed", "queue_state": "active", "control_version": "3", "group_generation": "2", "group_id": null},
    {"channel_id": "<blocked lane UUID>", "code": "stop_unconfirmed", "queue_state": "paused", "control_version": "8", "group_generation": "5", "group_id": "<group UUID>"}
  ],
  "target_holds": [
    {"target_id": "<A UUID>", "channel_id": "<lane UUID>", "code": "hold_released", "hold_reason": null},
    {"target_id": "<S UUID>", "channel_id": "<lane UUID>", "code": "grant_required", "hold_reason": "grant_required"}
  ]
}
```

Keep per-target display/affected-alias entries only if useful to existing callers; they cannot replace a channel receipt, because a retained channel can have no current target. Target Resume returns one selected hold result plus one channel result; `already_active` requires both active lane and no hold. Enum names in the example are a proposed contract, not existing implemented API.

- Request parsing: `server/internal/handler/collaboration_control.go:10` carries scope, exact decimal versions, selected target/configuration and observed group/generation; `:106` calls the service. `server/internal/handler/collaboration_authority.go:48` already returns the original structured receipt even on a service refusal. Routes are `server/cmd/server/router.go:1637`–`:1645`.
- Receipt storage: `server/internal/service/collaboration_queue.go:53` owns `CollaborationQueueReceipt`; target result currently lives in `collaboration_control.go:39`. Extend those once, rather than recomputing results in the handler.
- Replay: `collaboration_queue.go:175` hashes normalized payload, `:290` reauthorizes before operation lookup, `:299` replays the original success/refusal, `:318`–`:335` stores it. Commit failure at `:188` is never blindly retried. Preserve these rules and exact string versions.
- Projection: `server/internal/service/collaboration_execution_view.go:20` / `:154` currently expose `PauseReason` and mirrored control but no hold; `ExecutionView:100` is read-only and does not load explicit channels. Add channel rows if exposing removed-channel controls/barriers. Core parser is `packages/core/collaboration/execution-schema.ts:8`; mention model/parser also needs the field. Installed-client API drift must fail safely, and execution availability stays unavailable until full acceptance.

### Integrity and locking traps

- **Current source order differs from E02:** queue `collaboration_queue.go:271`, runtime `collaboration_runtime.go:165`, comments `collaboration_comments.go:217`, workflow `collaboration_workflow.go:470` and reconciliation `collaboration_account_reconciliation.go:198` all lock **targets then channels**. The latest contract says channel/lane before target. Change every borrowed wrapper together; do not reverse one path in isolation. Target/leader changes may require sorted old/new lane locks plus bounded unique-conflict retry.
- `lockCollaborationResources` (`collaboration_authority.go:285`) already uses NOWAIT for reverse resource/namespace intersections; preserve this. The current queue wrapper holds gate FOR SHARE before source locks at `collaboration_queue.go:240`, through commit.
- Never weaken `validateGroup` (`collaboration_groups.go:16`), including retained cancelled delegations (`:94`), or release reservations before it passes (`:158`). Stop/proof must validate exact batch/root/run ancestry; a parent proof never settles children.
- **Switch is more than a hold assignment:** `ReconfigureTarget` accepts only `Control` and `Roster` (`collaboration_authority.go:53`), with no staged new-version consent. Current API therefore cannot by itself implement a completed Switch requiring prior full consent. Preserve explicit pending configuration semantics or implement staged consent before claiming AC39/40. Leader movement also conflicts with `collaboration_preserve_batch_input` (`681...up.sql:132`) and the command identity trigger retaining `seq`; both need forward migration, safe waiting-only movement and preserved executed history. The execution projection currently requires historical commands/groups to match the target's **current** channel (`collaboration_execution_view.go:174`, `:199`); it must distinguish historical channel identity before leader switching.
- **Hold freshness fence:** lane control must not advance just because another target's grant is revoked. A delayed first Resume request observed before revoke→reconsent should not release a newer hold accidentally. Bind hold transitions to an explicit target-scoped revision/epoch contract; do not bump target epoch casually, because current consent includes that epoch and doing so would invalidate otherwise valid roster grants. Decide this with the DTO/migration owner and add the race regression.

### Tests to strengthen or replace

| Existing test / location | Required change or assertion |
| --- | --- |
| `collaboration_authority_test.go:67`, `:185`, `:219` | Assert initial hold survives consent; changed config becomes switch/config hold and fresh consent never releases it; transient names/presence remain non-invalidating. Add new alias on an already-active channel. |
| `collaboration_targets_test.go:281` (`ResourceRevocationPreservesAllWaitingAuthors`) | Assert channel queue/control remain unchanged, target gets `grant_required`, all waiting survives and real stop intent/proof barrier remains. Add unused revoked S while healthy A runs in same channel: A must stay running. |
| `collaboration_targets_test.go:248`; `collaboration_workflow_test.go:205` | Separate departed **author** lane pause from departed **owner only** hold. Assert independent same-lane aliases and other lanes preserve state. |
| `collaboration_control_managed_test.go:76` | Replace invalid-grant lane-paused expectation at `:107` with lane-active + target-held, retain explicit proof-blocked lane. Assert channel and hold receipts independently and exact original-key replay after proof/consent changes. |
| `collaboration_control_managed_test.go:153` | Existing head-batch grant refusal for both aliases is obsolete. Test independent target holds on the same lane, selected-target validation and hold release while lane already active. Keep unsupported squad execution fail-closed pending its implementation. |
| `collaboration_control_managed_test.go:208` | Existing healthy active target expecting `stop_unconfirmed` contradicts latest E05. Replace with `already_active`; add genuinely forced-stopped active lane + unknown proof refusal and full unchanged snapshot. |
| `collaboration_control_test.go:107`, `:124`, `:186`, `:248` | Snapshot already captures complete target/channel rows, so hold fields are included automatically; add grant/delegation/termination/capability checks where modified. Preserve global refusal/freeze-through-commit races and removed-lane reservations; assert explicit removed-channel receipt. |
| `collaboration_control_managed_test.go:219`, `:242` | Inject failures at hold writes/channel writes/event/receipt commit and require total rollback; preserve >2^53 versions and uncertain-commit original-key recovery. Fault tools must cover Query, not only Exec/QueryRow. |
| `collaboration_account_reconciliation_test.go:224`, `:537` | Keep old epoch versus fresh grant/run protection; add current grant hold without channel pause. Add non-root in-flight author, no resource grants and no waiting input: invalidation must discover source, stop group and pause author lane. |
| `collaboration_runtime_test.go:247` | Add actual `TeardownRuntime`/confirmed-delete/rebind callers; assert stop+hold commit before deletion, rollback on reverse source lock conflict, and original minimal proof still works after resource rows disappear. |
| `collaboration_execution_view_test.go:38`; handler/core schema tests | Hold and channel state can differ; malformed hold values refuse safely, no private grant/roster leakage, explicit channel IDs/results, exact versions, and no accidental feature-enable change. |

New canonical target-Resume matrix: valid selected target + invalid alias; invalid selected + otherwise resumeable lane; valid selected + forced group missing proof; healthy busy active lane + held selected alias; no hold + active lane; paused/closed global session; orphan run or active pointer; stale hold/config/group/gate; same rejected key after repair. Every refused case compares before/after business state and verifies the stored refusal cannot act later.

### Suggested ownership boundaries

1. **One schema/generator owner:** new forward migrations; `server/pkg/db/queries/collaboration_targets.sql`, `collaboration_authority.sql`, `collaboration_batches.sql`, `collaboration_reconciliation.sql`, resource reverse-lookup SQL; generated models/query files and migration tests/cleanup registry if an index is added. Serialize sqlc with other implementation lanes.
2. **Control/hold owner:** `collaboration_control.go`, `collaboration_targets.go`, `collaboration_authority.go`, `collaboration_invalidation.go`, associated control/authority tests. Coordinate shared `collaboration_queue.go` receipt fields and all wrapper lock order changes explicitly.
3. **Resource/account invalidation owner:** `runtime_teardown.go`, actual runtime/managed-binding callers, `collaboration_account_reconciliation.go` and targeted regression files. Consume the control owner's borrowed invalidation helper; do not duplicate stop/proof logic. SQL discovery requests go through the schema owner.
4. **Projection owner:** `collaboration_execution_view.go`, handler DTO/tests, core execution/mention schema/tests. Freeze wire shape with control owner first.
5. **FIFO/runtime owner (sequential dependency):** `collaboration_batches.go`, runtime candidate/promote admission in `collaboration_runtime.go`, command identity migrations and ordering queries. This lane owns hold skipping, rebatching and future leader movement; no parallel edits to shared SQL/generated files.

## External References

No external API research was needed. Repository contracts and existing pgx/sqlc transaction patterns are the evidence. New dependencies are unnecessary.

## Related Specs

- `.trellis/spec/server/collaboration-targets.md:11` explicitly marks holds/multi-target/M11 pending and requires rereading current contracts.
- `.trellis/spec/server/collaboration-targets.md:104`–`:126`: retained proof, strict refusal and known current channel model limitations.
- Root `CLAUDE.md`: no FK/cascade, separate concurrent index migration, no migration rewrites, exact API versions, fixture-based DB tests, ordinary agent CLIs forbidden in default tests.

## Caveats / Not Found

- This is source research, not a passing test report. No tests, live mutations, production edits or git commands were run.
- No existing durable target hold, resource teardown collaboration invalidator, or complete Switch/leader-migration path was found. Adding only the hold column cannot claim complete cross-target FIFO or M11 acceptance.
- Two independent correctness gaps are concrete and should be included in the continuation: non-root author account-invalidation discovery, and actual resource teardown/rebind initiation of retained stop/hold state. Their regression plans are above.
