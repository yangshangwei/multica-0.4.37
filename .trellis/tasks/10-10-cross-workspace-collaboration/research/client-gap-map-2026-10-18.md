# Research: Client target-hold controls and P6 implementation gaps

- Query: Map the existing core v3 reader/control boundary and Web/Desktop workbench integration; identify the smallest hold and per-lane/per-held-target receipt slice before full P6.
- Scope: Internal source and current task contracts; no implementation, dependency changes, git operations, or runtime tests.
- Date: 2026-10-18
- Ownership: This research file only. Other agents own backend implementation and shared task artifacts.

## Findings

### 1. Contract baseline and immediate conclusion

Read `research/execution-contract.md` in full (207 lines), `design.md` D09–D11, `implement.md` P6, the 2026-10-18 follow-up, and the core/views/web/desktop spec indexes. The authoritative distinction is:

- **Resume target is atomic.** All E05 checks happen before mutation. A paused/closed issue, invalid target grant/configuration, or an applicable unresolved stop rejects the entire target operation. No hold release is retained when that operation fails (`research/execution-contract.md:137`; `design.md:191`).
- **Resume All permits partial results.** Global authority/lifecycle/gate/scope failures reject everything. Otherwise the same transaction activates the session and independently decides each paused lane and each held target. Invalid target authority blocks that target, not another target sharing its lane (`research/execution-contract.md:139`).
- **A hold and a lane pause are different facts.** Initial targets, revocation, configuration drift and Switch set/retain the target hold; consent never clears it. Stop target pauses the lane. Removed-target historical barriers do not prevent activating the session, while real reservations remain until proof (`research/execution-contract.md:127`, `research/execution-contract.md:143`, `research/data-contract.md:15`).

The paragraph in `review-followup-2026-10-18.md` under the earlier incremental document update that describes releasing a target hold while its paused lane remains unconfirmed is stale for **scope=target**. The latest E05 and D09 require rejection with zero state changes; only Resume All has the independent partial decision behavior.

**Current source:** core has a substantive read/transport/cache/human-workflow foundation. It has **no execution-control client API or receipt parser**, and its v3 projection still expresses channel control through target fields without target holds. There is **no `packages/views/collaboration/` workbench and no `/account` Web/Desktop route**. Proceeding with a bounded core hold/receipt slice in parallel with backend is useful; enabling a product execution affordance or describing P6 as integrated is premature.

### 2. Existing source map: what can be retained

| Files / evidence | Existing behavior |
| --- | --- |
| `packages/core/collaboration/execution-schema.ts:8`, `:70`, `:95` | Strict v3 targets/commands/groups/runs, exact decimal versions, typed identities, channel/batch lineage, active-group and run-proof validation. |
| `packages/core/collaboration/execution-schema.test.ts:183`, `:191`, `:204`, `:226` | Rejects missing channel/batch identities, preserves multiple immutable inputs in one batch and cancelled tombstones, accepts same-channel active-group aliases. |
| `packages/core/collaboration/schema.ts:134`, `:188`, `:319` | Separate private issue DTO; exact authority/write context; issue/session/control consistency; redacted unavailable fallback. Missing old-server `execution` preserves human reading as `null`; malformed present execution rejects the private issue. |
| `packages/core/collaboration/schema.ts:202`, `:217`, `:253`, `:262` | Workbench, human operation, source status options, participant/lifecycle/account versions and human write types already exist. |
| `packages/core/api/client.ts:883`, `:904`, `:1002`, `:1378` | Account-scoped transport, request/session fencing, resource identity verification, explicit write key/version serialization, stripping ambient workspace headers and redacted private HTTP logs. |
| `packages/core/api/client.ts:910`, `:917`, `:937` | Creation-result recovery, exact recipient lookup, paginated current/participating workbench reads. There is no create-collaboration POST method. |
| `packages/core/api/client.ts:945`, `:962`, `:969`, `:990` | Comment reads/preview/create/edit/delete and notification list/read/archive transport exist. |
| `packages/core/collaboration/content-schema.ts:8`, `:30`, `:39`, `:46`, `:66` | Content protocol v2, typed targets, suppression/blocked/waiting receipts, optional batch/channel references, private comments and minimal denied notifications. |
| `packages/core/collaboration/access.ts:11`, `:19`, `:56`, `:77` | Server/API-instance/session/viewer query partitioning; issue access epoch; deny-before-cancel with `revert:false`; reject stale success **and error**; authorized detail recovery. |
| `packages/core/collaboration/queries.ts:9`, `:23`, `:30`; `content-queries.ts:5` | Fresh detail/workbench/comment/mention/notification query options; infinite cursors; mount/focus/network reconnect revalidation and 15-second polling. |
| `packages/core/collaboration/draft.ts:6`, `:29`, `:43`, `:63`; `action.ts:13`, `:43`, `:65` | StorageAdapter-backed server/viewer/issue-scoped immutable request drafts; original human-operation key/payload replay. Existing action union is human workflow/edit only. |
| `packages/core/paths/account.ts:12`, `:28`, `:36`; `paths/index.ts:7`; `paths/reserved-slugs.ts:69` | `/account` parser/builder, current/participating view, issue UUID, comment/description anchors, view/issue-specific memento key; `account` is reserved. |

Protocol versions are separate namespaces: execution **read** projection is v3 (`execution-schema.ts:71`); content DTOs are v2 (`content-schema.ts:31`); current control/queue receipt uses `server/internal/collaboration/execution.go:3` **v2**. Runtime protocol is independently v2 (`.trellis/spec/server/collaboration-targets.md:83`). Do not bulk-change all literal `2` values when adding hold reading.

### 3. Current capability gates and limits

1. Server issue response forces `control_ai=false` and `submit_command=false`, despite internal participant authority (`server/internal/handler/collaboration.go:151`). Core independently forces both false (`packages/core/collaboration/schema.ts:192`). This double gate is intentional, covered by `execution-schema.test.ts:18`, `:29`, `:49`, `:168`.
2. V3 requires `availability="unavailable"` and `unavailable_reason="shared_runtime_unavailable"` (`execution-schema.ts:74`); server emits those values (`server/internal/service/collaboration_execution_view.go:133`). A response claiming availability currently fails parsing (`execution-schema.test.ts:36`).
3. Mention catalogue carries internal `can_submit_commands`, but every target requires `execution_available=false` (`content-schema.ts:30`). It is not a substitute for the supported-client control capability. A fully authorized paused candidate is still a valid waiting candidate (`content-schema.test.ts:22`).
4. Stop must eventually depend on supported control protocol plus valid human authority, not a grant, online runtime, or execution gate (`schema.ts:195`; execution contract E01). Adding a hold cannot make Stop disappear when authorization is revoked.
5. Creation remains a separate deployment gate, not an implication of authenticated workbench reading (`.trellis/spec/server/collaboration-targets.md:5`; `design.md:235`). No existing client creation affordance was found.

### 4. Minimal core/read/control slice to align with the backend

#### 4.1 Read DTO: separate hold from physical channel control

Current target fields are `channel_id`, `control.queue_state/control_version`, `group_generation`, `active_group_id`, `next_command_seq`, `pause_reason`, and `revoked_at` (`execution-schema.ts:8`). The execution envelope has only targets/commands/groups/runs, without a channel collection or gate epoch (`execution-schema.ts:70`).

Minimum required facts:

- A **required target hold value**, independently validated: released versus `initial`, `grant_required`, `config_invalid`, `switch_hold` (data contract line 15). Do not infer it from `revoked_at` or `pause_reason`, and do not default a missing or unknown hold to released in a protocol that advertises hold support.
- One authoritative **channel control snapshot** per physical lane, including ID, queue state, control version, current group and generation. This permits deduplicated partial-Resume results, same-leader aliases, and retained lanes that currently have no active target. Inferring every lane from targets cannot express the removed-target case in E05.
- A current, exact **execution gate epoch** in an authorized read before a client can construct Resume; the current read exposes no `expected_execution_epoch` source. A schema-only hold change does not solve that precondition.
- Preserve historical group/run identities, exact signed-64-bit string validation, and current active-group/proof checks. A lane pause is not proof of termination; hold release is not permission to ignore group/run lineage.

Bounded candidate wire extension, for backend/core owners to finalize together (the names below are a proposed integration shape, not a new product design):

```ts
// Continue using the implementation's `channel` name for the contract's lane.
execution: {
  protocol_version: 3,
  // Existing issue/session/control/target_set_version/collections remain.
  channels: [{
    id, queue_state, control_version, group_generation,
    active_group_id, pause_reason,
  }],
  targets: [{
    // Existing typed identity/configuration fields remain.
    id, channel_id, hold_reason,
  }],
  execution_gate: { enabled, epoch },
}
```

Use one canonical released-hold representation across backend/core (for example `""` to match existing reason fields, with the four held enum values). Neither `null` nor a missing property should silently become `""`. If existing target control fields are retained during this bounded extension, they must match the referenced channel; they are not a second mutable control authority. Backend owns any protocol-version decision. Additive gated reading does not require changing content/runtime versions; incompatible removals or a new executable protocol require explicit negotiation and fixtures, not permissive parser fallback.

The candidate channel snapshot need not expose root resource directories, grants, rosters, paths, credentials, or nonces. Only the state required for display and exact control requests belongs in the private read DTO.

#### 4.2 Dedicated control request and receipt types

Backend already has `POST .../control`; its current request is at `server/internal/handler/collaboration_control.go:10`:

- Base resource authority: request key, lifecycle/participant/account epochs.
- `action=stop|resume`, `scope=target|issue`.
- Issue-scope global `expected_control_version` and `expected_target_set_version`.
- Resume-only `expected_execution_epoch` (Stop explicitly rejects it).
- Observed targets with target/config/control versions, nullable observed group ID, and exact nonnegative group generation. Target scope currently requires exactly one observation and no issue-control versions (`server/internal/service/collaboration_control.go:52`).

Mirror this through a discriminated core input schema; do not reuse `CollaborationWriteInput` blindly, because its human-workflow revision shape is a different boundary. Keep the server's field vocabulary and omit fields disallowed for that scope/action.

The current queue receipt (`server/internal/service/collaboration_queue.go:53`) contains status/code/protocol/scope, queue/control/target-set state, and a `targets` array of `CollaborationTargetControlResult` (`collaboration_control.go:39`). Its per-target row combines channel state and run counts; it cannot distinguish lane refusal from target hold refusal. Current `CollaborationOperationSchema` only parses `{code, collaboration_revision, status_options}` (`schema.ts:253`) and cannot parse that queue receipt. `getCollaborationOperation` always uses that human parser (`api/client.ts:1074`).

Bounded candidate receipt extension:

```ts
{
  protocol_version, scope, status, code,
  queue_state, control_version, target_set_version,
  channels: [{
    channel_id, code, queue_state, control_version,
    group_generation, group_id, group_settled,
    stopped_runs, unsettled_runs,
  }],
  target_holds: [{
    target_id, channel_id, code, hold_reason,
  }],
}
```

- Channel results are unique by channel ID, not repeated per alias. Hold results are unique by held target ID; do not require a hold result for each target that was already released.
- Scope=target returns the target operation's atomic result. A `stop_unconfirmed` refusal cannot contain a successful hold-release subresult.
- Scope=issue successful/partial receipts describe session activation plus both decision sets. Top-level `partially_resumed` alone is insufficient to render accurate counts or reasons. A channel may resume while one target in it retains `grant_required`; this is a valid combination.
- Global refusals, unavailable/unknown receipt formats, and missing required decision collections are not successful empty results. Required null/zero/version rules must follow the finalized server DTO; arbitrary action-result strings may be shown as unavailable, never guessed as success.
- Parser checks bind the returned scope/target/channel observations to the original request where the receipt contains those identities. Preserve the original receipt on operation recovery. Do not refresh its versions or convert a former refusal into a future Resume intent.
- Parse private success **and refusal bodies** with `redact:true`. `ApiError.body` carries the raw HTTP error payload (`api/client.ts:1423`); downstream UI must not cast it or display its private text. Transport already logs only a generic collaboration error.

Suggested source ownership: core agent owns `collaboration/execution-schema.ts`, a small adjacent control schema/module and canonical tests, `collaboration/index.ts`, corresponding client methods in the collaboration section of `api/client.ts`, and fixtures. Backend owner owns projection/control DTOs and service/handler tests. Coordinate that shared wire shape before either publishes fixtures. No dependencies or unrelated ApiClient refactor are needed.

#### 4.3 Recovery and mutations

Reuse the existing `fetchCollaboration`, `protectCollaborationRead`, scoped query keys, StorageAdapter draft ownership and original-request replay patterns. Add an execution-control action/receipt branch rather than pretending it is a human handoff. `action.ts:65` currently understands only human operations and assumes their receipt schema; the shared operation endpoint needs an explicit expected-kind parser or a typed dedicated wrapper.

Do not optimistically release holds/pause lanes. Await the server, consume the validated original receipt, then invalidate/refetch the private issue/control snapshot. Scope or access changes must invalidate the in-flight result, including error paths. An unknown outcome retains its original key and captured payload; a later proof/grant update alone does not schedule a replay.

### 5. Boundary tests for this immediate slice

Keep each behavior at its canonical layer rather than repeating every schema matrix through a DOM component.

| Boundary | Meaningful additions / retained evidence |
| --- | --- |
| Read schema | Same lane with target A released and S held; two distinct lanes; held unused target; revoked target history; missing/unknown hold; duplicate/missing channel; contradictory alias control; historical group proof remains mandatory. Extend `execution-schema.test.ts:95`, `:106`, `:131`, `:183`, `:226`. |
| Exact versions | Gate/control/target-set/config epochs beyond JS safe integers; signed-64-bit maximum; zero allowed only for unstarted generation. Preserve `execution-schema.test.ts:62`, `:72`, `:88`. |
| Receipt schema | Resume All with one resumed lane, one `stop_unconfirmed` lane, one invalid-grant hold on the resumed lane; empty target set; removed-target retained lane; duplicate decisions, malformed state/proof counts, wrong scope and missing required arrays reject. |
| Target atomicity | Server DB test proves byte-for-byte unchanged hold/lane/session after each target refusal. Core API/recovery test proves the refusal is not reinterpreted or partially applied. Do not simulate backend transaction atomicity only in a parser test. |
| Transport / replay | Stop omits gate epoch; Resume includes exact epoch; issue scope includes global versions; target scope excludes them; key and payload remain unchanged across lost response, refusal and recovery. New control endpoint coverage can follow `api/collaboration-workflow.test.ts:91`, `:133`, `:158`, `:200`. |
| Privacy / isolation | Wrong issue/target receipt and delayed old-session/old-access denial are rejected; private response sentinels absent from logs; no workspace header; stale HTTP error cannot revoke recovered access. Preserve `api/collaboration.test.ts:40`, `:88`, `:103`, `:158`; `access.test.ts:55`. |
| Gates | Hold-aware reading must still yield execution-unavailable/control=false until the separate supported-client rollout is authorized. Do not replace the existing capability-negative tests with available fixtures. |

### 6. Concrete P6 remainder, separate from the immediate control slice

Source search used `rg --files` plus identifier/import search across `packages/views`, `apps/web`, `apps/desktop` and `e2e`. No collaboration workbench component, app route, collaboration module consumer, or workbench end-to-end test was found. Names containing unrelated marketing “collaboration” and settings `AccountTab` are not this feature.

| P6 area | Already present | Remaining implementation and bounded landing points |
| --- | --- | --- |
| Shared account workbench | Core current/participating reads, summaries, paths, query isolation | New `packages/views/collaboration/` page, independent selected detail, list pagination/filter/sort UI, notification list/count, exact-recipient forms, participant and human handoff/lifecycle/edit interfaces. Wire existing human APIs before rebuilding them. Component tests belong beside these new views. |
| Shared issue detail | Existing presentation pieces | `IssueDetail` accepts an ID/presentation/slots but owns workspace queries (`issue-detail.tsx:976`, `:1171`). It cannot be mounted as an external-account detail by providing a source slug: that would reintroduce workspace-member/agent catalog assumptions. Extract/inject only reusable body/comment/history pieces needed by the collaboration view. |
| Restricted mention editor | `ContentEditor` can disable mentions or inject context items (`content-editor.tsx:185`); existing popup and mention nodes | Context mode is **not restrictive**: non-empty search merges ordinary workspace members/agents/squads (`mention-suggestion.tsx:706`, `:810`) and enables project search (`:827`). Add an explicit issue-authorized suggestion source; disable ordinary search/capture/selected-text subtask creation in collaboration mode. Preserve ordinary editor behavior and its tests. |
| Comments / R43/R44/R57 | Private comment schema/transport and preview exist; `CommentCard` has callback hooks (`comment-card.tsx:90`) | No collaboration composer, preview/suppression/receipt UI, comment-history/tombstone integration, or reaction/resolve client methods. Do not pass workspace moderation authority into R43. R56 must remain closed before capture. Attachment history and delivery need P3 APIs, not ordinary attachment fallbacks. |
| Execution / consent / R45 | Read-only v3 and comment candidate/preview DTOs | Queue/group/member/proof display, two-level Stop/Resume, per-lane and per-hold results, waiting-command cancellation, owner consent/target organization/switch UI; corresponding control/consent/revoke/cancel client methods are absent. These are subsequent consumers of the immediate typed core control slice. |
| Creation / controlled files | Creation-result GET exists (`api/client.ts:910`) | No `POST /api/collaboration/issues` client method, creation action/draft type or shared form. Existing draft requires an issue UUID and only `operation|comment` (`draft.ts:6`), so pre-issue create drafts need explicit source/request ownership. Private upload/preview/range/download/short-capability schemas and consumers were not found. P1/P3 contracts must land before the end-to-end creation-with-file claim. |
| Web `/account` | Central path builder/export; login honors sanitized `next` before workspace destination (`apps/web/app/(auth)/login/page.tsx:127`, `:148`) | Add the page and auth-only guard outside `[workspaceSlug]`; new route must clear workspace mirror. `useDashboardGuard` waits for workspace list and requires a current workspace (`packages/views/layout/use-dashboard-guard.ts:41`); `[workspaceSlug]/layout.tsx:52`/`:105` also gates children on that list. Neither is the account guard. Test zero workspaces, list 500 with valid auth, required password setup, anchors and login return. |
| Desktop persistent scope | Single Coordinator, persisted workspace tab/history/memento machinery | `routes.tsx:93` declares all tabs workspace-scoped; `tab-store.ts:131` has only `activeWorkspaceSlug/byWorkspace`; `sanitizeTabPath` drops reserved `/account` (`:343`); `resourceKeyForUrl` uses pathname (`:318`). Add account/workspace/none active scope plus one account tab, recognized before reserved-slug rejection, retaining separate issue/view mementos and updating `?issue=` when focusing an existing tab. Extend migrations without losing workspace tabs/pins/MRU/history. |
| Desktop boot / shell | Authentication/endpoint recovery and per-workspace pruning are tested | `App.tsx:226` reads workspace list, `:302` opens new-workspace overlay for zero spaces, `:319` auto-selects a first workspace, `:353` wraps the shell in workspace recovery. Account scope must remain navigable when that list is unavailable and must not be overwritten by these effects. Authentication failure/required password change remain blocking. `Cmd+W` also assumes no active slug means no tab (`App.tsx:77`). |
| Desktop routing / pop-out | Coordinator is sole router writer (`platform/tab-coordinator.ts:17`); adapter routes workspace/overlay paths (`platform/navigation.tsx:49`, `:90`, `:189`) | Recognize account destinations in every navigation path, active host, reload scope and workspace mirror ownership. Current pop-out parser accepts only `/{slug}/issues/{id}` (`src/shared/issue-window.ts:56`); wire account navigation/deep links deliberately. Extend `tab-store.test.ts`, `tab-coordinator.test.ts`, navigation/issue-window tests and native isolated Electron acceptance. |
| Account realtime / revocation | Private reads poll/refetch, access epoch + draft cleanup exist | `WSProvider` refuses to connect without `wsSlug` (`packages/core/realtime/provider.tsx:80`); `WSClient.setAuth` requires a workspace slug (`api/ws-client.ts:68`). No collaboration/account subscription handlers found in core realtime. Add an authenticated account/issue-scope connection and explicit reconnect/reauth permission refresh after P3 server contract; network `refetchOnReconnect` is not WS reauthentication. Wire source-membership invalidation separately from participant revocation. |
| Source workspace mixed views / R54/R58 | Ordinary lists, detail, board and assignee infrastructure | `IssueSchema`/`Issue` have no typed collaboration marker or AccountSummary (`api/schemas.ts:1274`; `types/issue.ts:161`); no views branch consumes one. Add server-backed mode/summary parsing and shared detail routing, external collaborator display/filter, capability-based non-optimistic handoff/state-column drag while retaining ordinary intra-column order semantics. |
| Native/file/i18n/accessibility acceptance | Generic editor, navigation, recovery and shell tests exist | New collaboration screens need shared keyboard/focus/narrow/200%/i18n checks and real Electron/production Web journeys. Current workbench path tests are pure helpers, not platform or end-to-end coverage. P7 administrator successor form is also not integrated into this account shell. |

The existing attachment editor explicitly describes CloudFront signature or raw-URL fallback (`packages/views/editor/content-editor.tsx:211`). That is unsuitable for D09 collaboration delivery; reuse display components only after injecting the controlled transport. The general HTML sandbox/renderer-file defense remains required.

### 7. Recommended ownership and order

1. **Immediate backend/core interface pair:** agree hold/channel/gate snapshot and separate decision receipt wire names; backend writes service/handler projection and transactional tests, core writes parsers/API/recovery and boundary tests. Keep capability gates closed. This can be implemented without editing app routers or shared editors.
2. **Shared views owner:** after that contract stabilizes, build the account page and restricted detail/composer/forms using core APIs. Own `packages/views/collaboration/`, narrowly shared presentation/editor injection, exports and locale files. No app-specific routing imports.
3. **Web wiring owner:** own `/account`, auth-only routing/mirror handoff and platform tests. Reuse the existing centralized path parser and login `next` support.
4. **Desktop shell owner:** own tab scope/persistence/coordinator/adapter/boot/pop-out together. These mutate one shared routing system and should not be split into competing tab-store writers. Preserve existing workspace route ownership tests and native isolation.
5. **Realtime/file integration owner:** core account events/access invalidation and controlled attachment clients require the server P3 contract. Integrate with the views after that source is available. The backend execution/hold slice does not prove this P3/P6 work.

No new dependency is needed for any identified immediate gap. Reuse existing Zod, TanStack Query, StorageAdapter and platform adapters. Workbench UI design beyond the approved D10 is outside this research.

## Related Specs and References

- `.trellis/spec/core/frontend/index.md`: points to current execution and API auth-epoch contracts; many general core guide entries remain placeholders.
- `.trellis/spec/server/collaboration-targets.md:11`, `:128`: M09/v3 implementation limits and retained cache/read invariants.
- `.trellis/spec/views/frontend/index.md` and `quality-guidelines.md`: shared component ownership; callable-store mocks, canonical test layering.
- `.trellis/spec/web/frontend/index.md` and `e2e-run-environment.md`: use a production Next build/start target for E2E evidence.
- `.trellis/spec/desktop/frontend/index.md` and `connection-recovery.md`: preserve verified auth on workspace failures, endpoint/credential fencing, persistent server editor and native acceptance.
- `CLAUDE.md`: core/views/ui boundaries, redacted schema parsing, Query server ownership, exact API action verification and platform-only routing.
- Task contracts: `research/execution-contract.md` E01–E08, `research/data-contract.md:14`, `design.md:174` D09–D11, `implement.md:171` P6.
- External references: none fetched; this is repository-owned protocol work. Observed wire versions are listed in section 2. No unfamiliar SDK or API was introduced or evaluated.

## Caveats / Not Found

- This is source research, **not newly executed verification**. Test locations establish existing coverage and recommended boundaries, not fresh passing results. The earlier M09 run totals belong to the follow-up verification record.
- Candidate `channels`, `target_holds`, `execution_gate` field names are a bounded integration proposal; backend/core must finalize them together. Latest E05 target atomicity and issue partial-result semantics are fixed requirements, not optional field-design choices.
- Current code still has historical target-oriented comments and gate wording that predates dedicated backend control routes. Read implementation gates and schemas rather than interpreting those comments as evidence that the routes do not exist.
- M11 cannot be represented merely by relaxing `waiting` validation: `execution-schema.ts:115` currently requires every non-cancelled input of an active batch to be `in_flight`. M11 needs the explicit group/read association so waiting-but-merged input remains cancellable without becoming executed. That is a later schema slice, after backend behavior exists.
- Leader Switch likewise needs a deliberate historical/current-lane distinction: `execution-schema.ts:111` currently requires every historical command channel to equal its target's current channel. Do not weaken lineage globally while implementing hold-only changes.
- No complete workbench UI, account route, account-scope WS integration, private attachment client path, or collaboration Web/Electron end-to-end suite was found in the searched source trees. Ordinary settings AccountTab and ordinary collaboration-marketing references are unrelated.
