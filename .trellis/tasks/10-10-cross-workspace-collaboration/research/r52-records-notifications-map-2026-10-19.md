# Research: Recorded runs and R52 AI-result notifications

- Query: Map source-owned run content, immutable input/resource provenance, result ingestion, group settlement, account notifications and shared Web/Electron readers; recommend the smallest complete recording/read/notification increment.
- Scope: internal; read-only investigation outside this file. No application code, task state or git operations changed.
- Date: 2026-10-19
- Governing continuation: `../continuation-r51-r52-2026-10-19.md`. The server implementation owner freezes the exact DTO in `r51-r52-api-contract-2026-10-19.md`; proposed names below are implementation advice, not an independently approved interface.

## Findings

### 1. Required boundary and present gap

R05 shares the process and results the system actually records. R08/R23 require independent input authors, target identity and run lineage. R28 distinguishes actual resource/provider account provenance from human command authorship. R52 requires durable participant account notifications; AI completion is not issue completion. D01 expressly forbids making retained history depend on engine task/message rows (`../design.md:20`); D03 distinguishes author, owner, creator and handler (`../design.md:67`).

The current foundation already stores source-owned inputs, run identities, outcome text, agent comments and squad activity. It has no collaboration transcript/tool/usage ingestion or human run-record read endpoint. A finished process or existing ordinary task telemetry must not be presented as already-recorded collaboration usage. The authorized continuation explicitly adds real typed recording to the supported static runtime path, so merely showing “not recorded” everywhere would leave this increment incomplete.

Keep execution projection v5 as its current control/lineage read. Add a separate object-authorized record reader and a bounded optional outcome recording extension. Do not add general provider discovery, real accounts, new containment, file preparation, dependencies or rollout changes.

### 2. Files found and responsibilities

| File | Existing responsibility / reuse |
| --- | --- |
| `server/pkg/protocol/collaboration.go:90` | Shared narrow outcome identity, event key, attempt and `{outcome, body}`; no record/usage fields yet. |
| `server/internal/daemon/collaborationrun/runner.go:95` | Decodes one explicit stdout business result after reap, uploads with business capability, then seals/proves independently. |
| `server/internal/daemon/collaborationrun/controller.go:75` | Bounded stdout buffer; stderr is discarded at line 175. |
| `server/internal/daemon/collaborationrun/client.go:134` | Outcome input validation and runtime HTTP transport. |
| `server/internal/handler/collaboration_runtime.go:46` | Strict, duplicate-aware runtime JSON decoder and bounded body; outcome handler at line 166. |
| `server/internal/service/collaboration_runtime_capabilities.go:327` | Current-admission outcome ingestion, original event replay comparison, source run-event insertion and ordered invalidation. |
| `server/internal/service/collaboration_runtime_capabilities.go:435` | Historical narrow proof acceptance, independent run settlement, squad wake/seal and group settlement. |
| `server/internal/service/collaboration_squad.go:205` | Runtime comment, immutable comment input/receipt event, delegation and human mention notifications. |
| `server/internal/service/collaboration_squad.go:370` | Squad activity business outcome, separate from OS proof. |
| `server/internal/service/collaboration_groups.go:243` | Single successful whole-group terminal transition; common to normal proof and Stop/invalidation paths. |
| `server/internal/service/collaboration_control.go:682` | `stopRun`, including safe server-proven nonlaunch settlement. |
| `server/internal/service/collaboration_runtime_capabilities.go:210` | Claim-time immutable per-run prompt snapshot. |
| `server/internal/service/collaboration_input_readers.go:13` | Exact prompt reader discovery; conservative unknown historical readers are barrier evidence, not an exact reconstructed prompt. |
| `server/pkg/db/queries/collaboration_runtime.sql:132` | Run event lookup/outcome insert, proof write and final group SQL. |
| `server/pkg/db/queries/collaboration_squad.sql:36` | Generic internal business-event insertion for known kinds. |
| `server/internal/service/collaboration_comments.go:101` | Reusable authorized repeatable-read transaction for content. |
| `server/internal/handler/collaboration_activity.go:16` | Existing issue-bound strict cursor/limit handler pattern. |
| `server/internal/service/collaboration_notifications.go:29` | Effective human participants, self exclusion for human comment events, retained notification reads and read/archive. |
| `server/internal/service/collaboration_realtime.go:165` | Content-free runtime invalidation event helper; currently creates no notification intents. |
| `packages/core/api/client.ts:928` | Optional-history transport distinguishes absent routes from structured object denial. |
| `packages/core/collaboration/access.ts:19` | Server/viewer/issue Query keys and stale-account/access-epoch fencing. |
| `packages/core/collaboration/content-queries.ts:8` | Protected infinite-query patterns and notification state mutation. |
| `packages/core/collaboration/content-schema.ts:100` | Notification source-kind parser; currently only issue/comment/body. |
| `packages/core/paths/account.ts:2` | Account destination parser, URL builder and tab identity. |
| `packages/views/collaboration/execution-panel.tsx:83` | Current group/run status rows; no run-record drilldown. |
| `packages/views/collaboration/collaboration-detail.tsx:20` | Shared account/source detail, exact comment loading, focus/scroll anchors and access-loss rendering. |
| `packages/views/collaboration/notifications.tsx:25` | Notification links and state actions; currently does not render kind or mention labels. |
| `e2e/collaboration-account-desktop.spec.ts:107` | Actual Electron, isolated profile/API, cold links, popouts, account identity and restart coverage. |

### 3. What is actually recorded now

There are exactly three production `collaboration_run_event` write shapes in the inspected code:

1. `outcome`: the complete `protocol.CollaborationRunOutcome`, including private execution/attempt identity, success/failure and body (`collaboration_runtime_capabilities.go:361–388`). These identity fields are not a public history DTO.
2. `comment`: `{input: {comment_id,parent_id,content}, receipt: {comment_id,run_ids,continuation_run_id}}` (`collaboration_squad.go:276–285`). The comment itself is also retained on the source issue. This event preserves the original authoring input without needing live agent/task rows.
3. `squad_activity`: `{outcome: action|no_action|failed, reason}`; action/no_action map to run business success, failed to failure (`collaboration_squad.go:370–410`).

Runtime preparation, claim, permit, outcome, proof and group-settlement metadata live in `collaboration_event`; they are invalidation/audit facts, not provider transcript. `RuntimeContext` reads live issue title/description and the newest 200 comments (`collaboration_squad.go:147`); that read is not persisted as an exact historical context snapshot. Do not reconstruct an old prompt from current issue text or claim that the bounded context list is a complete transcript.

Current size limits are inconsistent but all enforced at different boundaries:

| Boundary | Current limit |
| --- | --- |
| Controller stdout capture | 128 KiB (`controller.go:85`) |
| Client outcome body | 64 KiB (`client.go:135`) |
| Entire runtime HTTP JSON | 96 KiB (`collaboration_runtime.go:55`) |
| Service outcome body | 1 MiB (`collaboration_runtime_capabilities.go:329`) |
| Human/agent collaboration comment text | 64 KiB (`collaboration_mentions.go:90`) |
| Frozen batch inputs | 1024 inputs, just under 2 MiB encoded (`collaboration_batches.go:19`) |

The Runner accepts only strict stdout `{outcome,body}` when output was not truncated. It never invents success from exit code zero; stderr is not uploaded. Missing/revoked outcome upload does not block `finish`/sealed proof. Restart of a reaped ledger goes to proof recovery, not a provider rerun or arbitrary late log upload (`runner.go:31–40`, `113–132`).

### 4. Minimal useful typed recording extension

Extend the existing explicit result envelope with optional `record`, both in provider stdout and `CollaborationRunOutcome`. Preserve legacy `{outcome,body}`. Suggested record fields:

```text
record?: {
  version: 1,
  completeness: complete | partial,
  entries: [
    {kind: message, role: assistant | user | system, text},
    {kind: tool_call, call_id, tool_name, text},
    {kind: tool_result, call_id, text, is_error}
  ],
  usage?: {input_tokens?, output_tokens?, cache_read_tokens?, cache_write_tokens?, cost_usd?},
  provider?: {name, model?, account_source: runtime_configuration}
}
```

The exact field names/roles should be frozen by the server owner. Use discriminated typed structs, not `map[string]any`, provider-native arbitrary JSON, log blobs or opaque credential/config snapshots. Tool arguments/results are explicit recorded text; do not expose executable configuration, environment, access tokens, install/binding secrets or account directories as provenance. Normal content is rendered using the existing restricted resource policy.

Use exact nonnegative counters, with null/absent distinguished from recorded zero. Decimal-string counters avoid JavaScript precision loss; an optional cost should be a bounded decimal amount, not inferred from the model or silently converted to zero. Do not invent provider totals from process duration. Provider/model facts are runtime-reported observations, not verified billing identity; resource owner/account attribution is server-derived.

A practical bounded contract is at most 256 entries, 64 KiB text per entry, 512 KiB encoded optional record, 64 KiB final body and a 1 MiB encoded outcome envelope. Put shared constants/validation in the protocol leaf and make controller, client and outcome HTTP limits agree. Do not enlarge the narrow proof's accepted fields or let optional recording widen its purposes. Existing proof/claim request size policy can remain separately bounded.

Persist the accepted typed record in the source outcome event in the same transaction as `business_outcome`. No second task-message/usage store is needed. Read projection can flatten its bounded entry array with stable event/ordinal IDs. Retain body as the canonical final result; do not count it twice when presenting transcript and summary.

Important implementation hazards:

- Adding arrays/pointers means the current `saved != p` replay test cannot express value equality. Normalize/validate once, then use typed deep equality or a canonical request digest, including every optional record field. Same key/same content is inert; changed content conflicts.
- `RuntimeSquadActivity` can already set business outcome before stdout upload. Do not overwrite a settled/previous activity outcome merely to attach telemetry. A compatible same-outcome recording attach, if implemented, needs a deliberate immutable identity contract; otherwise show that this path did not report extra telemetry. The supported direct static-runtime path must demonstrate actual message/tool/usage recording in this increment.
- `RecordCollaborationRuntimeProof` currently overwrites `recording_incomplete` with `business_outcome IS NULL` (`collaboration_runtime.sql:144`). If the flag is extended to include explicit partial recording, preserve that fact instead of erasing it on proof. Also expose separate transcript/usage availability; legacy recorded outcomes do not imply recorded telemetry.
- Invalid/oversized record output must fail the business recording path and still permit historical sealed proof. Never relax business revocation to recover a missing tail. The user can see partial/not-recorded/incomplete, not fabricated empty success.

### 5. Exact input and resource provenance

`claimPromptSnapshot` freezes each run's input array once and returns it unchanged on replay (`collaboration_runtime_capabilities.go:210–239`). It includes command ID, true author ID, captured author/participant/lifecycle epochs, body and attachment IDs. Later members and continuations can read overlapping input sets. M11 input joins remain waiting until consumed by a claim.

For display, use that exact snapshot. Enrich command/target identity only through immutable command fields and `collaboration_group_input` / `collaboration_group_dependency` history. A current command's batch/channel/group may change on nonlaunch return or target Switch. Its current placement cannot identify an earlier run's consumption. Preserve snapshot order; do not sort authors or duplicate an input because the same comment selected both A and A-led squad S. If multiple commands intentionally exist for one comment, keep both command/target identities while grouping their display if desired.

`collaborationInputReaders` treats some historic launches without snapshots as possible readers for safety. That conservative barrier rule is not permission to fabricate their exact input body. Missing unclaimed snapshot should show not-yet-claimed; an old/malformed missing snapshot should show unavailable/not-recorded. Do not infer it from current batches.

Root creation already retains actual agent, execution workspace, runtime, owner, grant/version, installation/binding and group/target identity (`collaboration_runtime.go:642–646`); derived runs do the same (`collaboration_squad.go:321–335`). Group roster freezes role/resource/grant IDs, but no immutable display names or actual provider/model/account telemetry. No need to expose all those internal IDs.

For useful resource display, freeze a small allowlisted source-owned snapshot when creating/claiming root and derived runs: agent id/name, target type/id/display label, execution workspace id/name, runtime id/display label, resource-owner user id/name/avatar, and explicit snapshot version. Derive IDs from the retained run and validate correspondence; do not trust owner IDs from stdout. Store names only if observed at that time. Existing runs can retain known IDs with missing labels; do not relabel history with a changed/deleted runtime's current metadata. A nullable JSONB `resource_snapshot` on run is sufficient if display snapshots are added; no foreign keys or live-resource joins are required to read retained records.

Provider/model/account-source belongs to the optional accepted recording and is immutable with it. The static runtime is explicitly configured by host executable/args (`server/internal/daemon/collaboration.go:18`); it does not automatically use the ordinary agent provider configuration. Do not claim an ordinary configured provider/account was actually used merely because an agent row names it. The provider account-source label can identify the approved runtime resource, without including an email, credential identifier, raw profile or path.

### 6. Authorized and bounded read recommendation

Use a separate `GET /api/collaboration/issues/{issueID}/runs/{runID}/records`, with a frozen versioned DTO. The existing execution v5 projection can supply run IDs and group selection; it intentionally excludes transcript, roster and working paths (`collaboration_execution_view.go:98–138`).

Suggested single handler supports `section=inputs|events` (default events), `cursor` and `limit`. Each response contains:

- Required `protocol_version`, `issue_id`, `run_id`, selected section.
- Minimal retained run/group/parent/step identity and separate run business outcome, forced disposition, proof state and group terminal state.
- Allowlisted resource/provider summary and nullable typed usage with explicit recording states.
- Typed page `items`, `next_cursor` and a captured upper event sequence. Inputs paginate by frozen ordinal; event records paginate by `(run_event.seq, nested_entry_ordinal)` if optional recordings are flattened.

An alternative metadata/detail split is fine if the server owner finds it simpler, but never place all potentially 1024/~2 MiB input bodies in every event page. Suggested page limit 50, max 100 and an encoded content budget around 256 KiB; define how a single legacy large entry advances without truncation/livelock. The existing 64 KiB comment bound makes normal entries tractable. A bounded payload or not-recorded marker is different from silently omitting remaining pages.

Cursor identity should include issue, run, section and fixed high-water sequence, with strict finite-size/decimal validation. Refresh can capture later events; paging an earlier snapshot should not duplicate or omit existing entries. Return stable IDs and explicit completeness. Unknown/malformed stored kinds expose only a safe unavailable marker, never raw `payload` or SQL/decoder details. Do not silently count a skipped unsupported entry as a fully recorded transcript.

Reuse `readCollaborationContent` for current human/account and independent participant/source-member authorization plus same-snapshot data. Check the run's retained session issue matches the requested issue before reading events. Read permission does not require current grant, online runtime or current participation if valid source membership remains. Machines, unrelated execution-space members and revoked viewers without another read basis cannot read it. Source-member access remains broader than AI control.

Missing optional history routes are not lost issue access. Reuse `fetchCollaborationOptionalHistory` (`client.ts:928`), which converts an unstructured/other 404 to unavailable but preserves the object's structured `code=not_found`. A missing/wrong-issue run can use a distinct `run_not_found` after issue authorization; it must not destroy an otherwise valid issue cache. `protectCollaborationRead` alone treats every 401/403/404/410 as denial, so this transport distinction matters.

Keys belong below `collaborationKeys.issue(scope, issueID)` and include run/section/cursor. Keep pure schemas outside request modules, parse with `redact:true`, correlate response issue/run/section, and pass abort signals. Existing source/issue realtime invalidation will then refetch these subqueries; access epoch changes cancel and remove all private record pages. No persisted server content in Zustand or platform-specific duplicate reader.

### 7. All current settlement paths and the R52 hook

| Path | Current evidence / correct notification point |
| --- | --- |
| `RecordRuntimeOutcome` | Saves business outcome/event while run may be live. Do not announce terminal completion here. |
| `RuntimeSquadActivity` | Saves one turn's business outcome. It is not whole-squad completion. |
| `AcceptRuntimeProof` | Settles the individual run from a sealed historical identity, releases its own reservation, settles its delegations; member completion schedules a leader wakeup. |
| Normal direct completion | Proof seals direct group, then calls `settleProvenGroup`. |
| Normal squad completion | Final leader/continuation proof can seal only when no wake remains and all group runs are settled; group transition still checks full retained dependencies. |
| `stopGroup` | Seals expansion, detaches unread joins, retains first forced cause, requests every descendant stop and calls `settleProvenGroup`. It is reused by explicit Stop, lifecycle closure, departures and resource/account invalidation. |
| `stopRun` without any possible permit/start | `SettleNeverLaunchedCollaborationRun` can supply server nonlaunch evidence immediately. Group notification still waits for the group transition; other live descendants block it. |
| Delayed proof after Stop/revoke/resource deletion | Historical proof still works; group transition preserves forced stopped/cancelled state instead of late success. |

The single suitable hook is the successful branch after `SettleCollaborationProvenGroup` in `collaboration_groups.go:264–289`. The SQL updates only an un-settled, expansion-sealed group after all run proof and delegation conditions pass (`collaboration_runtime.sql:162`). A no-row/replayed transition creates no notification. Keep notification/event writes in this existing transaction after its successful transition; no secondary asynchronous inference from task status.

Final success/failure follows the highest immutable `leader_turn` among agent/leader/continuation runs, not timestamp, UUID, last proof sender or a member result. Forced disposition wins. Proof without business outcome yields failed/incomplete; exit zero is not success. A stopped request with a live/unknown run must not emit confirmed-stop completion.

Recommended notification design: **one logical account result event per terminal group, no separate completion notifications for each member or interim leader turn**. Retain their outcomes/proof in run records. Save the representative final agent/leader/continuation run ID with the terminal event and use it for the exact record link; do not use whichever member's proof happened to arrive last. The linked record should show its group result and sibling-run links so a stopped/failed group is not described as an unrelated individual success. If an old settled group lacks a defensible representative run, open the group/issue summary instead of inventing one.

### 8. Recipient, actor, replay and delivery semantics

Human workflow events pass the real `t.actor` and exclude it (`collaboration_workflow.go:311`); comment recipients exclude their verified human actor (`collaboration_notifications.go:45`). Effective participants exclude revoked memberships, database-disabled users and temporary disabled-account policy (`collaboration_comments.sql:66`, `collaboration_notifications.go:29`). R36 source-only users are not automatically collaboration-notification recipients.

Runtime/proof transactions intentionally have no human actor: `lockCollaborationRuntimeSource` initializes `collaborationQueueTx` without `actor` (`collaboration_runtime.go:97–100`), and `recordCollaborationExecutionChange` writes no `ActorUserID`. Agent mention notifications also do not exclude a resource owner (`collaboration_squad.go:299–311`). **System AI-result notifications go to all current effective participants; neither the earliest input author nor resource owner is a proxy human operator.** The original human Stop/change audit keeps its actual actor. This matches the main continuation's decision.

Reuse current participant filtering and the existing notification insert uniqueness `(recipient_user_id, issue_id, event_seq)` (migration 601). Fix recipients in the terminal business transaction. The source issue lock serializes participant changes; do not introduce late user/resource authorization locks or `currentAdmission` into historical proof just to emit a notification. Owner/resource deletion must not strand legal proof. Snapshot participant eligibility safely; account delivery/read rechecks remain independent.

The existing `collaboration_event` is the durable outbox. `CollaborationRealtime.deliverNext` publishes only invalidation metadata and reuses stable event IDs (`collaboration_realtime.go:97`). Delivery retry may notify an account to refetch, but must not insert notifications or recompute the terminal recipient set. Notification row-write failure rolls back its required atomic transaction; network delivery failure happens later and leaves the committed business facts/intents intact.

The current notification list retains rows after loss of access but removes title and source anchor and forces `source_kind=issue, source_id=null, can_open=false` (`collaboration_notifications.go:179–185`). `removed` remains content-free even if source membership survives. Read/archive remains recipient-only and does not change assignment, participation or execution.

### 9. Deep links and shared UI gaps

`source_kind` is currently constrained to issue/comment/body in migration 651 and the core schema. The path parser permits only `#comment-UUID` and `#description`. The notification view renders title/actions but currently ignores both `kind` and `is_mention`; add bilingual result and “mentioned you” text while updating this surface.

Freeze one exact run anchor, preferably `/account?view=participating&issue=UUID#run-RUN_UUID`. Extend the path constructor/parser together, reject anchors without an issue and preserve the account route key's issue/list identity. `CollaborationDetail` should fetch/open the linked run independently of which history page is visible, then scroll/focus once the exact record is mounted. Current comment anchoring provides the pattern (`collaboration-detail.tsx:26–48`). Use `AppLink`/NavigationAdapter so Electron shares the account tab, popout, cold-link and restart behavior.

Compatibility point for the owner to resolve in the frozen DTO: blindly returning a new `source_kind=run` makes older installed core parsers reject the whole notification page. An additive optional run-anchor field on an existing issue notification projection can preserve old clients opening the issue while new clients open the exact run; alternatively use explicit response-version negotiation for the new source kind. Do not quietly accept arbitrary unknown run anchors. New clients should tolerate unknown notification kinds with a factual generic label, while known run anchors require current `can_open` and a valid issue/run association.

Show a drilldown from every run, including failed, stopped, cancelled and incomplete ones. Keep group completion separate from run business/proof status. Show true authors and resource provenance, recorded-zero versus not-recorded usage, exact input targets and restricted-rendered text/tool output. Pending/missing/partial history needs a retry or explicit explanation; do not replace missing telemetry with empty “complete” history. Loading another run or changing account/issue must retire the old scoped content and focus intent.

### 10. Retention and migrations

Run events have no foreign-key dependency on engine tasks/resources. The workspace deletion manifest explicitly retains collaboration runs, events, grants, input memberships, attempts, reservations and termination rows (`server/internal/handler/workspace_delete_manifest_test.go:31–58`). Ordinary `task_message` and `task_usage` are deleted by workspace task cleanup (`server/pkg/db/queries/workspace_delete.sql:198–210`). This is a direct reason not to source the new view from engine history.

The existing concurrent indexes already support `(run_id, producer_event_id)` idempotency and `(run_id, seq)` paging (migrations 596–597); do not add redundant indexes. If adding a nullable snapshot column or notification constraint, use a forward migration. Any actual new index needs its own single-statement `CREATE ... INDEX CONCURRENTLY` migration and cleanup-registry entry. No foreign keys, cascades or rewriting prior migration files. Test cleanup after recording while retaining the source issue; source workspace deletion must still refuse under R37.

### 11. Meaningful verification plan

Server/typed recording:

- Reuse managed fixtures in `collaboration_runtime_test.go:155` and add legacy versus typed outcome tests. Assert exact message/tool text, nullable versus recorded-zero usage, provider/resource ownership and duplicate-key replay; changed usage/transcript must conflict and not create a second event.
- Real client/router strict JSON tests: wrong issue/run/attempt, revoked business token, unknown/duplicate fields, negative/overflow counters, excessive entries/bytes, malformed provider fields, and no private values in errors/logs. Keep proof acceptance independent of failed recording.
- A test-created static executable emits the new record with synthetic assistant/tool messages, known token counts and provider metadata. Extend `TestCollaborationRuntimeOutcomeClientMatchesService`, `TestCollaborationSingleAgentRuntimeRoutesLinux` and Runner tests to prove stdout → client → real handler → source storage → authorized records. Do not use installed providers or paid accounts.
- Frozen inputs: two authors in one batch; A+S alias inputs; M11 later input and member/continuation prompt readers; edit/delete original comments; nonlaunch return, switch and later re-promotion. Assert the earlier run retains the exact original snapshot/target/sequence.
- Records ACL/paging: participant, source-only member, no-workspace participant, execution-space outsider and machine; wrong issue/run; viewer removal with/without surviving source membership; >1 page of inputs/events; stable cursor/high-water; unknown stored kind; no live-engine/resource dependency after cleanup.

R52:

- Before proof, a business result creates no terminal notification. Individual member and early leader proof create no whole-group completion notification. Highest leader-turn result decides the terminal group, including the existing delayed-BEGIN order test.
- Group success, failure, proof-only incomplete, sealed stopped/cancelled and immediate server-proven nonlaunch each produce exactly one logical group event and one row per eligible recipient. A stop request with unconfirmed proof produces none.
- Exact repeated proof/outcome, duplicate/concurrent settlement and outbox retry produce no duplicate rows. Removed/disabled users are filtered; source-only users are not added; input authors/owners still receive system results. A later participant is not added by retry.
- Inject required notification/event write failure and assert full transaction rollback; inject relay failure and assert committed records/intents remain with pending outbox. Read/archive and revoked-content redaction reuse `collaboration_notifications_test.go`.

Core/views:

- New pure schema tests cover unknown/malformed record variants, identity mismatch, exact large counters, null/missing telemetry and redacted errors. Guard against API/request import cycles.
- API/query tests cover missing optional route, distinct missing run, structured issue denial, paging, abort, stale account/server responses and old denied responses after restored access. All pages lie under issue-owned keys.
- Path/notification tests validate exact run anchors, generic legacy/unknown notification display, mention labels, whole-group wording and no open action after denial. Shared views verify long text, multiple pages, sibling run identity, incomplete output, real input authors and keyboard focus of a deep-linked record.

Web/Electron:

- Extend `e2e/collaboration-workbench-web.spec.ts` and `e2e/collaboration-account-desktop.spec.ts` using task-owned synthetic records/API actors. Exercise the production shared detail, record pagination, result notification click, reader removal without reload, refresh, narrow viewport and 200% zoom in both locales. The runtime route test supplies ingestion evidence; seeded browser history alone does not prove ingestion.
- Observe a live account WebSocket and the invalidation that reveals the committed notification/record; a poll-only pass is not realtime proof. Add notification read/archive assertions without changing current responsibility.
- Electron must launch the actual built renderer/preload with its isolated profile, use the production account navigation/tab coordinator, open the exact run from another workspace/popout/cold link, and reopen it after restart. A Chromium tab is not Electron evidence. Native macOS UI evidence is not native Linux runtime containment evidence.
- Parent owns the already-created isolated DB/Redis and production Web/Electron environments. Follow the current continuation's explicit migration command; do not rerun shared bootstrap. Save screenshots/visual verdicts in the parent's approved evidence scope, not from this research agent.

Useful existing suites: `collaboration_runtime_test.go`, `collaboration_execution_history_test.go`, `collaboration_squad_test.go:348`, `collaboration_squad_order_test.go`, `collaboration_squad_boundary_test.go`, `collaboration_realtime_runtime_test.go`, notification service/handler tests, core `execution-squad.test.ts`, `access.test.ts`, `api/collaboration-content.test.ts`, shared execution/detail tests and account-path/platform navigation tests. Run focused Go race and affected core/views/desktop suites first, then the parent-required quality gate; these were not run by this research-only pass.

## Related specs and external references

- Root `AGENTS.md`, `CLAUDE.md`, `.trellis/workflow.md` and the task PRD/design/implementation plan were read. No implement/check JSONL was loaded.
- `.trellis/spec/server/collaboration-targets.md`: current v5 projection, hold/proof, retention and narrow runtime contract.
- `.trellis/spec/server/collaboration-squads.md`: immutable prompt readers, M11 joins and leader turn ordering.
- `.trellis/spec/server/collaboration-runtime-cli.md`: explicit static executable, narrow CLI and capacity-one test requirements.
- `.trellis/spec/server/collaboration-content.md`, `collaboration-realtime.md`: current content ACL, outbox, safe projection and final-send denial.
- `.trellis/spec/core/frontend/collaboration-workbench.md`, views component/quality guidelines: Query ownership, optional-history distinction, parser redaction and shared UI constraints.
- `.trellis/spec/web/frontend/e2e-run-environment.md`: production Next, isolated API/DB, provenance and one-build-per-checkout requirements; desktop account navigation is referenced by the core/shared contract.
- Task `research/data-contract.md` and `research/execution-contract.md` E03–E05: source history, original input provenance, outcome/proof ordering and no late private logs after revocation.
- No external dependency/API research was needed. Inspected repository versions: Go 1.26.6 and pgx v5.9.2 (`server/go.mod`); catalog TanStack Query `^5.96.2`, Zod `^4.1.5`, Vitest `^4.1.0` (`pnpm-workspace.yaml`). No new dependency is recommended.

## Caveats / Not Found

- Findings describe the code inspected before the concurrently assigned server implementation. Re-read that owner's frozen API contract before dependent edits; it supersedes suggested DTO names/limits here.
- This is research evidence, not test evidence. No tests, provider CLI, paid account, browser, migration or runtime was executed.
- The older foundation/task text contains intentionally dated pre-v2 execution statements. Current targets/squads specs and actual source were used for current behavior; old “one run per issue” or old gate notes must not override them.
- Existing history cannot supply previously unrecorded transcript, provider usage, deleted resource display names or an exact historical `RuntimeContext`. Preserve known retained IDs/body/comments and explicitly state missing observations. The new static test path must nevertheless record real synthetic telemetry end to end.
- General provider adapters, arbitrary live transcript streaming, runtime Git/file preparation, extra OS containment and entire-parent completion remain outside this increment. R51 is owned separately by the server implementation lane.
