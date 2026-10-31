# Research: Runtime handoff envelope and squad provenance

- Query: How can a newly admitted collaboration Agent receive an auditable handoff containing current handler/intent, memory references, prior results, immutable input readers and all coalesced wakeup causes?
- Scope: internal; read-only code inspection and implementation recommendations, no product edits or runtime tests
- Date: 2026-10-31
- Baseline: parent session reported HEAD `4c833e737`; this research does not certify an implementation or an experiment result.

## Findings

### 1. Current context does not reach the provider automatically

`CollaborationService.RuntimeContext` in `server/internal/service/collaboration_squad.go:146` runs under `collaborationRuntimeMutation` and `lockRuntimeBusiness`. It returns identity, issue title/description, agent/squad, step kind, frozen roster, the immutable `prompt_snapshot` inputs, the latest 200 comments, and a delegation string. It does not return the human handler/handoff, prior results, memory version, input readers or wakeup causes.

The HTTP entry point is `server/internal/handler/collaboration_runtime_business.go:38`. The Go CLI DTO and identity check live at `server/internal/cli/collaboration.go:166` and `:196`. The check verifies run/issue identity, but there is no strict handoff parser because that field does not yet exist.

`server/internal/daemon/collaborationrun/host_runner.go:131` fetches shared memory before Provider start. `hostPrompt` at `:491` serializes claim inputs, role, step, parent, attachments, Git and memory. It merely tells the model to read `multica issue get`; the daemon does not itself load RuntimeContext. The member-specific line at `:519` asks the model to discover `dispatch_comment`. Consequently an API-only enhancement cannot guarantee that delegation/results reach the actual provider prompt.

Required consumer change: a supported daemon must fetch and validate the handoff launch snapshot after permit/session wait and before Provider start, then inject it directly. A refusal/identity mismatch must prevent that launch; retryable transport errors need the same bounded retry treatment as memory. Old-server absence needs an explicit unsupported state, not fabricated empty history.

### 2. Sources already exist for most fields

| Required fact | Current source | Projection rule |
| --- | --- | --- |
| Current human handler and issue status | `issue.assignee_type/assignee_id/status`, through the currently locked `CollaborationIssueAccess`; access type at `server/internal/service/collaboration.go:50` | Current authority is the issue row. Do not infer from the latest comment or resource owner. |
| Handoff author/from/to/purpose/note | `issue_handoff`, created at `server/internal/service/collaboration_workflow.go:204`; SQL at `server/pkg/db/queries/collaboration_workflow.sql:36` | Preserve exact original actor and note; add a bounded query joined to authoritative `collaboration_event` handoff references. |
| Handoff ordering | `collaboration_event.event_seq` and `payload.handoff_id`; workflow finish writes that reference | Use event sequence for causal order. `issue_handoff.created_at` uses transaction time and should not become a synthetic monotonic handoff sequence. Initial assignment may have no handoff row: expose that fact. |
| Active information request/requester | `issue_collaboration.active_information_request_id`; `GetCollaborationInformationRequest` at `collaboration_workflow.sql:53` | Keep request origin distinct from the current handler and resource owner. |
| Original admitted human commands | `collaboration_command` plus immutable `collaboration_run.prompt_snapshot`; `protocol.CollaborationRunInput` at `server/pkg/protocol/collaboration.go:35` | Body, author and original epochs come from immutable claim input, not live comment text. |
| Delegation identity/role/step/parent | `collaboration_delegation`, `collaboration_run.parent_run_id/step_kind/step_key`; creation at `collaboration_squad.go:260` and `:316` | Match exact member run and frozen roster; it is only this step's admitted assignment. |
| Delegation original body | Parent run `collaboration_run_event` with `kind=comment`, `producer_event_id=comment:<comment_id>`; immutable input body stored at `collaboration_squad.go:278` | Prefer this immutable payload and verify `collaboration_delegation.payload_hash`. The current RuntimeContext instead reads live `comment.content` at `:195`, which can diverge from the original assignment. |
| Memory version and author | `collaboration_memory_version`, existing `RuntimeMemoryBase`/`RuntimeMemory` | Refer to existing versioned memory; do not add another document or copy purged content into a durable handoff. |
| Run result/proof/termination | `collaboration_run` and typed `outcome` / `squad_activity` events | Separate business outcome, result text, activity reason, forced disposition and proof. An absent result is not a success or proof of no work. |
| Input reader IDs | `collaborationInputReaders` in `server/internal/service/collaboration_input_readers.go:13` | Reuse it for complete group readers. Existing execution view does so at `collaboration_execution_view.go:582`. |
| Leader order | Immutable `leader_turn` allocated under the group lock, described by `collaboration-squads.md` | Never choose the last leader by UUID or transaction-start `created_at`. |

The existing activity endpoint demonstrates safe, explicit field projection: `CollaborationActivityHandoff` at `server/internal/service/collaboration_activity.go:29`; `ListCollaborationActivityHandoffs` at `server/pkg/db/queries/collaboration_activity.sql:13`. Raw event JSON is deliberately not an HTTP DTO.

### 3. Immutable launch snapshot and live refresh are different products

Recommended contract:

1. **Claim inputs remain frozen at claim.** Do not add later joined commands to an already claimed run. `consumeJoinedInputs` at `server/internal/service/collaboration_squad_join.go:170` consumes waiting joins together with that continuation's first prompt freeze; replay cannot consume new joins.
2. **Launch handoff snapshot is frozen before each actual provider launch**, after permit and memory-base retrieval. It has a protocol version, admitted run/attempt identity, snapshot ID, issue event watermark, source references and exact hashes for text supplied to that launch. Repeated retrieval for that launch returns the same admitted source set. A fresh-session fallback has its own launch/attempt identity and its own recorded consumption; it must not silently reuse an undocumented new set of facts.
3. **Live context refresh is a current projection**, tagged as live and carrying its own watermark. It may return newer comments, results, current handler or latest memory. It must not overwrite the launch snapshot or imply those later facts were present in the earlier prompt.
4. **Historical retention cannot bypass purge.** Snapshot records should retain reference IDs, source version, hashes and missing/purged markers; do not retain a duplicate unpurgeable copy of memory or other purgeable source text. A current read rechecks source availability and returns missing explicitly. If exact repeated prompt material can no longer be supplied, surface the loss or refuse supported replay rather than silently substitute a different text under the same snapshot identity.
5. **Consumption facts have precise meaning.** Existing `reader_run_ids` means the input was contained in a frozen claimed prompt (conservatively including certain legacy readers); it is not proof that the model read or acted on it. For launch/context consumption, record source IDs/hash plus launch identity; an HTTP context read only establishes delivery to a caller, while the daemon's provider-input fixture proves actual prompt injection.

A compact additive envelope can have `protocol_version`, `snapshot` (mode/id/watermark/launch identity), `authority`, `intent`, `memory` references, `inputs` with readers/state, `assignment`, `runs` with typed results, `wakeup_causes`, and `limits`. Strings in memory/comments/handoff notes remain sourced data even though their server-generated provenance is authoritative.

For bounds, set explicit maximum run/result counts and UTF-8 byte limits; include `total`, `returned`, omitted reasons and retrieval references. Do not use an unbounded session-wide `ListCollaborationRuns` plus all raw records as the prompt. The existing SQL at `server/pkg/db/queries/collaboration_targets.sql:11` is unbounded and sorted by `created_at,id`; a dedicated bounded handoff query is preferable.

### 4. Results are available, but a historical result is not always a final answer

`RecordRuntimeOutcome` at `server/internal/service/collaboration_runtime_capabilities.go:421` validates the typed result and stores an immutable `outcome` event. An already accepted identical outcome can be ACKed after revoke/settlement, but no new late content is accepted (`:438`). `RuntimeSquadActivity` at `collaboration_squad.go:374` can record activity/business completion before stdout supplies its result.

`server/internal/daemon/collaborationrun/host_recording.go:203` maps Provider status to business outcome. At `:217`, body comes from `result.Output`; if the failed result has no output, it instead uses `result.Error`. Therefore historical `outcome.body` must be labeled `provider_result` (with business outcome and event provenance), not blindly exposed as a confirmed `final_answer`. An activity `reason` also is not a final answer.

Recommended first version: a discriminated result object with `status=present|absent|unavailable`, `kind=provider_result|squad_activity`, source event ID, role/step/parent, outcome, body/reference, truncation and completeness. An explicit `final_answer` should be present only when a new producer contract proves that provenance. Historical data remains `unknown/absent` for that narrower fact. A no-output provider must produce an explicit missing marker, not an inferred answer from the last assistant transcript item.

Decisions/reasons/next steps currently live in memory or sourced text; there is no existing normalized decision table. Without a new user/Agent authoring contract, the envelope should reference these sources and describe their verification status, not claim the server extracted or verified decisions. A general model summarizer is outside scope.

### 5. Wakeup coalescing currently loses source multiplicity

Three paths reach `queueContinuation`:

- Runtime comment/member reply: `collaboration_squad.go:256` and `:271`.
- A newly joined human input: `collaboration_squad_join.go:112`.
- Member completion proof: `completeProvenRuntimeRun` at `collaboration_runtime_capabilities.go:715`, which schedules a wake at `:738`; polling drains it at `collaboration_squad.go:421`.

`queueContinuation` at `collaboration_squad.go:345` immediately returns the existing prepared continuation ID. `ScheduleCollaborationSquadWakeup` at `server/pkg/db/queries/collaboration_squad.sql:51` stores only `wake_pending=true,wake_parent_run_id=<latest>`. Multiple proofs can overwrite the parent while only one continuation is queued. ParentRunID alone cannot reconstruct the complete reasons.

Recommended change: retain immutable causes with source kind, source event/command/comment/run ID, group ID, ordering/watermark and the continuation to which they were assigned. Stable producer keys deduplicate retries. Preserve the existing group-locked rule of at most one prepared continuation. Causes arriving after its claim are assigned to the next continuation, not retroactively to its launch snapshot.

No new domain table is inherently necessary: `collaboration_run_event.kind` is unrestricted text (`server/migrations/571_collaboration_schema.up.sql:112`), and `(run_id,producer_event_id)` already has a unique concurrent index (`596_collaboration_run_event_producer.up.sql:1`). Server-only pending-wakeup events can be written on the source run; a later immutable assignment event on the continuation can link all pending source IDs while draining under the existing group locks. Immediate comment/join paths can write the assignment once the queued continuation is known. The query must distinguish pending vs already assigned causes and bound returned data. Use a narrow table only if this event query cannot remain clear and efficient; any new index needs its own concurrent migration.

Proof-triggered cause recording is metadata-only and must remain possible without current business authority, like existing proof settlement. It must not accept arbitrary text supplied through a termination token or synthesize a business result. Legacy runs with no retained cause records must expose incomplete provenance rather than fabricate a complete historical cause list.

### 6. New event kinds can break existing record readers

`ListCollaborationRunRecordEvents` at `server/pkg/db/queries/collaboration_records.sql:23` currently excludes only `resource_snapshot` and `preparation`. `collaborationRecordEventItems` at `server/internal/service/collaboration_records.go:391` only accepts `outcome`, `comment`, and `squad_activity`; the default at `:426` rejects the page.

Adding `handoff_snapshot`, `wakeup_pending`, or `wakeup_assigned` events without updating the query/projection makes human RunRecords fail. These internal provenance events should be filtered by an explicit supported-record-kind query or separately handled as non-record metadata; avoid adding unrecognized public item kinds to protocol 1. Include a regression that puts every new internal event before/between/after existing events and still paginates retained record text correctly. Changes to SQL require `make sqlc` and generated files.

### 7. Authorization, trust and compatibility

- Preserve `lockRuntimeBusiness` (`collaboration_squad.go:84`) for context/refresh: narrow capability, exact admitted attempt, current input/resource authority, and retained reservation. A daemon identity, participant display role or textual claim never replaces this predicate.
- Project only issue-authorized retained facts, using explicit DTOs. Never expose raw capability, attempt tokens, provider session IDs, private workspace paths, complete raw prompt events or resource secrets from event JSON.
- Frozen command/delegation are the only admitted executable request sources. Comments, memory, human handoff notes, result text and claimed verification are evidence/data; they cannot extend tool or business permissions.
- Preserve existing RuntimeContext fields for installed clients. Add an optional versioned envelope with strict validation of present fields. Unknown versions/malformed identity must be unavailable, not coerced into authority. A missing envelope from an older server is an explicit unsupported baseline, separate from an intentionally empty valid envelope.
- Do not put the handoff support version into resource capability/configuration digests merely to upgrade readers; that would revoke existing consent. The existing `MemoryVersion` comment at `server/pkg/protocol/collaboration.go:57` documents this distinction.
- Reuse the current shared-memory base semantics (`server/internal/service/collaboration_memory_runtime.go:80`): the first base read freezes a version, refresh reads latest, and a purged base requires explicit treatment. Avoid presenting the latest live memory version as the version actually injected earlier.

## Minimal implementation surfaces

| Surface | Needed work |
| --- | --- |
| `server/pkg/protocol/` | Shared handoff DTOs, discriminants, validation and bounded fields; avoid duplicate CLI/service definitions. |
| `server/internal/service/collaboration_squad.go` and focused new handoff helper | Add current projection and immutable launch snapshot flow; retain immutable delegation source and complete input readers. |
| `collaboration_squad_join.go`, `collaboration_runtime_capabilities.go` | Persist every join/comment/proof cause under existing ordering locks; preserve proof's metadata-only authority. |
| `server/pkg/db/queries/collaboration_squad.sql`, focused handoff queries, `collaboration_records.sql` and generated counterparts | Bounded source/result reads, pending-cause assignment, snapshot retrieval and record compatibility. |
| `server/internal/handler/collaboration_runtime_business.go` | Parse/version the narrow handoff read if a separate base endpoint is chosen; existing context can remain additive/live. |
| `server/internal/cli/collaboration.go` | Parse optional envelope strictly and expose it in existing issue get output. |
| `server/internal/daemon/collaborationrun/host_runner.go` and focused client/host helper | Retrieve validated launch handoff automatically; inject delegation/results/trust labels; record launch consumption; repeat on actual fresh launch. |
| `server/internal/service/builtin_skills/multica-working-on-issues/` | Update affected collaboration read/continuation guidance and source map together with the changed behavior. |

Frontend shared human handoff evidence should consume a human-authorized projection, not the runtime business capability endpoint; that ownership is outside this research slice. Checkpoint integration should supply only explicit recording completeness/missing ranges to the handoff.

## Canonical verification plan

1. Protocol/CLI pure tests: valid envelope, absent older-server envelope, unknown protocol, malformed UUID/version/discriminant, duplicate readers/causes, wrong issue/run/attempt identity, source scope mismatch, byte overflow and incomplete markers. Keep malformed response coverage at the boundary.
2. DB service `collaboration_squad_test.go` fixture (`newManagedSquadFixture`, `permitSquadRun`, `finishSquadRun`) extended with a focused handoff suite: exact member delegation, source comment later edited, previous leader result, failed-member activity, no provider result, multiple completion causes coalesced into one continuation, comment+proof+joined-input causes, stable replay and causes after claim assigned only to a later turn.
3. Reuse `collaboration_squad_boundary_test.go` for reader semantics and `collaboration_squad_order_test.go` for delayed-BEGIN leader ordering; do not build a second reader heuristic. Check removed authors preserve original IDs and cannot regain authority via retained text.
4. Snapshot/live test: freeze launch handoff, append authorized results/comments and transfer handler, fetch live context, prove earlier snapshot references/hashes remain unchanged; purge a referenced memory version and prove no durable copied content escapes via snapshot replay.
5. `collaboration_records_test.go`: internal handoff/wakeup events do not cause record parse failures, duplicate displayed text or stuck pagination.
6. Daemon fake executable tests following `host_memory_test.go` and `host_prompt_role_test.go`: actual provider stdin contains the served assignment/result/snapshot; spoofed comments remain data; supported snapshot refusal prevents start; transient failure retries; fresh-session launch records its actual snapshot. Keep full text out of ordinary brief, ledger capability payloads and termination messages.
7. Run Go packages against an explicit test DB serially. `collaboration-host-execution.md` documents that handler/service packages share an organization singleton; they cannot safely run concurrently merely using different workspace fixtures. Confirm named DB tests ran rather than skipped.
8. Real account/model behavior and Web/Desktop integration remain separate evidence. This research ran no tests and no authenticated provider calls.

## Related specs

- `CLAUDE.md`: source authority, no foreign keys, concurrent indexes, explicit API parsing, built-in skill/source-map sync, fixture helpers and default fake executables.
- `.trellis/spec/server/collaboration-squads.md`: immutable input readers, claim/continuation boundaries, leader order and coalescing.
- `.trellis/spec/server/collaboration-host-execution.md`: launch identities, memory base, terminal proof and original-outcome ACK semantics.
- `.trellis/spec/server/collaboration-command-history.md`: immutable attribution, absence vs unknown provenance, exact verification boundaries.
- `.trellis/workflow.md`: research-only planning; this document does not authorize product implementation.

## External references

None required: the researched contracts are repository-owned Go/SQL protocols. No unfamiliar external API is proposed and no dependency change is needed.

## Caveats / Not Found

- No existing normalized decisions/verification table or guaranteed provider-final-answer field was found. Do not implement server-inferred decisions or relabel error text as an answer.
- No complete durable wakeup-cause set exists in current squad state. Historical completeness cannot be backfilled truthfully from only `wake_parent_run_id`.
- A first-read freeze is not automatically an actual prompt-consumption proof; test the daemon integration and retain precise launch/source references.
- The former planning parent still gates real-model evidence on formal E0. This research neither re-ran E0 nor treats an upstream environment block as a product pass.
- Existing `fix-closure/research/2026-10-31` artifacts belong to another workflow and were not edited.
