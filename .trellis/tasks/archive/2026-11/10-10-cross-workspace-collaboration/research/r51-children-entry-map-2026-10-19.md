# Research: R51 ordinary children and legacy collaboration entries

- Query: Identify the remaining implementation and meaningful regression coverage for R51 / AC82, AC84, AC87, relevant AC85 and AC89, including ordinary child authority, shared projections, source/account detail mounts, copy/share, selection and list actions.
- Scope: internal; research only. The parallel run-history/usage/provenance/AI-notification increment is outside this report.
- Date: 2026-10-19
- Active task: `.trellis/tasks/10-10-cross-workspace-collaboration`

## Findings

### Recommended implementation boundary

The ordinary issue write and execution owners already preserve the essential R51 distinction. A normal child is not a collaboration issue merely because its parent is one. Keep those owners and add the missing minimal read projection, a shared children section, and narrowly scoped legacy-entry guards. Do not create a new child-membership workflow or grant the parent's participants access to an ordinary child.

Recommended shared response addition to the existing `GET /api/collaboration/issues/{uuid}`:

```ts
ordinary_children?: {
  items: Array<{
    id: string;             // child UUID
    identifier: string;     // source identifier, e.g. MUL-123
    title: string;
    can_open: boolean;
  }>;
  source_workspace_slug: string | null;
};
```

The block is optional only for compatibility with older servers. A server implementing it returns the complete set, including an explicit empty `items` array. Missing support is unavailable, not proof of no children. Malformed present data must not become an empty list or openable ordinary Issue.

- Select direct children from the same source workspace, using the ordinary formal-admission predicate (`not_required` / `accepted`) and excluding children that have their own `issue_collaboration` relationship. Each independently shared child would need its own object authorization; R51 does not authorize recursively exposing it.
- Order by `number ASC` with an ID tie-breaker if needed. The ordinary `ListChildIssues` query already returns the complete set in number order, without pagination (`server/pkg/db/queries/issue.sql:498`). Do not accidentally reuse the 200-row **parent-picker** cap from `collaboration_source_fields.go:59`.
- For an authorized source member, return `can_open: true` and the current source slug. For a participant without source membership, return `can_open: false` and `source_workspace_slug: null`. Derive this from the current read snapshot. `edit_source_structure` is insufficient: it deliberately excludes ordinary source members.
- Prefer adding an internal `SourceMember` field to `CollaborationIssueAccess` populated from `GetCollaborationAccess`, rather than making child authorization depend on the unrelated `reorder` capability. `row.SourceMember` is already loaded in `server/internal/service/collaboration.go:96` but is not retained explicitly in the access struct.
- Title belongs in the established minimal-reference vocabulary: `collaborationIssueReference` already includes `id`, `identifier`, and `title` (`server/internal/handler/collaboration_boundary.go:26`); source parent references include `name` and `identifier` (`server/internal/service/collaboration_source_fields.go:107`). R51 should not add child description, status, assignee, comments, attachment metadata, properties, execution details, or source catalogs.
- Produce the block inside `collaborationIssueResponseInSnapshot`. The existing wrapper opens a repeatable-read read-only transaction and resolves access again before projecting fields (`server/internal/handler/collaboration.go:107`). Children and source-open authority must use those transaction-bound queries, not a separate pool query after the snapshot.
- A small SQL query selecting only `id`, `number`, and `title` is preferable to assembling a generic `IssueResponse` and deleting private fields afterward.

For this increment, a complete unpaginated **minimal** list matches the ordinary child-list contract and avoids adding another endpoint. If an implementation instead imposes any limit, it must introduce a real continuation contract (`next_cursor` plus a Load more UI) and test the last page; silently returning the first N entries is not acceptable. A server test with more than 200 children distinguishes these alternatives.

### Stable navigation and shared UI

Use `paths.workspace(ordinary_children.source_workspace_slug).issueDetail(child.id)` through `AppLink` only when `can_open === true` and the slug is present. This is independent of the viewer's current workspace mirror or whether the workspace list has loaded. The existing detail route may then canonicalize the UUID to the identifier; browser/native assertions should wait for the canonical result.

For outside participants, render plain text with identifier/title and a source-access explanation. Do not render an anchor, disabled link with a hidden href, tooltip preview, source issue hook, or prefetch. `AppLink` performs prefetch on hover/focus, so avoiding the anchor matters (`packages/views/navigation/app-link.tsx`).

Add one `CollaborationChildren` section to `packages/views/collaboration/collaboration-detail.tsx`, near the source field summary. That component currently renders fields, reactions, attachments, execution, activity and comments but no children (`:72–84`). Its input should be the already-projected block; do not call `useWorkspaceId()` or generic child-list APIs from this component.

No new shared child editor is required for the smallest complete R51 path. Source members can create an ordinary issue, then use its existing Set parent action to select the collaboration issue, and Remove parent to detach. Those operations target the child. The shared list can explain this path. If a direct Attach action is added later, it must keep the same ordinary write owner and source authority; it cannot mount the source picker for outside participants.

All existing source mounts reach the same shared detail:

| File | Existing role / implication |
| --- | --- |
| `packages/views/issues/components/issue-detail.tsx:1164` | Classifies the generic issue before mounting `OrdinaryIssueDetail`; preserves known collaboration identity from cache. |
| `packages/views/collaboration/source-bridge.tsx:35` | `SourceCollaborationDetail` mounts shared detail and collaboration realtime, and keeps comment anchors. |
| `packages/views/collaboration/account-workbench-page.tsx` | Account list/detail owner; outside participants enter here. |
| `packages/views/issues/components/issue-detail-route.tsx` | Source UUID/identifier route resolution and canonicalization. |
| `apps/web/app/[workspaceSlug]/(dashboard)/issues/[id]/page.tsx` | Web source-route wiring. |
| `apps/desktop/src/renderer/src/pages/issue-detail-page.tsx` | Desktop source-route page. |
| `apps/desktop/src/renderer/src/components/issue-window.tsx` | Existing account/workspace native window branching. |
| `apps/desktop/src/renderer/src/platform/issue-window-navigation.tsx` | Popout routing back to main workspace/account destinations. |
| `apps/desktop/src/shared/issue-window.ts` | Validated tagged account/workspace window context. |

The child section should therefore require no duplicated page implementation. Account-to-source child navigation must still pass through the platform adapter so Electron changes workspace groups correctly.

### Ordinary child attach/detach authority already present

1. `UpdateIssue` first checks whether the **updated issue itself** is a collaboration issue (`server/internal/handler/issue.go:3892`). An ordinary child is not rejected merely because its parent is shared.
2. Parent updates validate the resolved child's workspace, reject self-parent/cycles, and accept explicit null for detach (`issue.go:4019`). The client Remove parent action also clears stage in the same ordinary write (`packages/views/issues/actions/use-issue-actions.ts:210`).
3. The atomic update transaction takes workspace/subscriber, live member, status-catalog and I1 workspace fences (`issue.go:3644`), locks the actual child, checks current authority, and prepares the existing iteration recorder (`issue.go:3720`, `:3826`). The member row is held with `FOR SHARE`; missing current membership returns `workspace membership required` (`:3668`). It does not use the collaboration session/participant/command workflow.
4. Ordinary creation uses `IssueService.CreateInTx`, current workspace/catalog/I1 fences and same-workspace parent validation (`server/internal/service/issue.go:382`, `:438`). The parent's project may be inherited under the existing ordinary rule; its collaborator identities or grants are not inherited.
5. Ordinary deletion locks its selected/direct-child set, authorizes the actor, records iteration facts and performs dependent cleanup atomically (`server/internal/handler/issue.go:4505`). Deleting the ordinary child is not deleting the collaboration parent or its history. Generic delete/batch operations on a collaboration issue are already refused.
6. `SetParentIssueModal` writes `{id: ordinaryChild.id, parent_issue_id: selected.id}`. `AddChildIssueModal` writes the selected child, not the parent. Both currently use ordinary source queries (`packages/views/modals/set-parent-issue.tsx`, `add-child-issue.tsx`). Set parent remains available from an ordinary issue even though collaboration row menus are restricted.

These are source contracts read from code, not a claim that the new R51 acceptance matrix has already executed. Existing general parent validation should be reused; do not introduce collaboration manager requirements for ordinary children. Concurrent handoff must not make an otherwise-authorized source member lose ordinary child editing, and concurrent source departure must respect the existing member fence.

### Execution, machine and source-copy boundaries already present

- `checkOrdinaryExecutionSource` checks the actual source issue's collaboration relationship, never its parent (`server/internal/service/issue_admission.go:40`). Ordinary children retain their own normal assignment/execution contract.
- `TestLegacyExecutionSQLBoundaryAndOrdinaryChild` covers collaboration/ordinary/ordinary-child insert, deferred, claim, runtime, batch, start, retry, promotion, reclaim and linking (`server/internal/service/legacy_execution_boundary_test.go:139`). The same file tests a quick-create parent reference independently from captured collaboration content (`:357`). Reuse these tests; do not duplicate their entire matrix in UI tests.
- `ResolveIssueAccess` admits only the trusted human credential whitelist (`server/internal/service/collaboration.go:64`; `server/internal/collaboration/access.go`). Ordinary task/agent credentials cannot read a parent's full collaboration details just because a child points to it.
- Generic machine references already have `can_open: false` and no body. `collaboration_boundary_test.go` checks private search/filter/count and malformed identity boundaries. `collaboration_source_test.go:105` checks machine projection in child/table reads. That test is about **shared children under source parents**, not the new external R51 ordinary-child projection.
- Source-context capture rejects collaboration sources before attachment reads/copies and before agent lookup or queue writes. `server/internal/handler/source_context_collaboration_test.go` tests preview, manual, agent and lookup failure, including no source-context/object-intent/attachment/task artifacts. `TestSourceContextOrdinaryChildHTTPStillCreatesOwnSubtask` (`:113`) confirms an ordinary child can still capture its own content without inheriting the parent's protected body.
- Shared detail uses `RichContent resourcePolicy="restricted"` (`collaboration-detail.tsx:70`) and does not mount the ordinary detail's comment-create-subissue callback. Add an actual shared body/comment editor selection regression; do not reopen R56 while adding an Attach explanation.

### Child updates currently do not refresh an external open detail

Ordinary create/update/delete emit ordinary workspace events (`server/internal/service/issue.go:722`; `server/internal/handler/issue.go:4174`, `:4483`, `:5035`, `:5171`). Outside participants intentionally have no source workspace subscription. The collaboration outbox only sees collaboration events. Current `collaboration:workbench_changed` processing refreshes account collections but explicitly excludes issue queries (`packages/core/realtime/collaboration.ts:42`, `:100`). The ordinary change therefore cannot update a newly added child section in an external open detail until a later read/poll.

Recommended small integration:

1. Capture affected old/new parent IDs from the ordinary child's committed write owner. Include create-with-parent, attach, move between shared parents, detach, title change, single/batch deletion, and ordinary batch parent updates. The current update bus payload does not contain the old parent ID; delete publishes only the deleted issue ID, so do not rely on a post-delete lookup or only the new parent.
2. After successful commit, resolve which affected parents are collaboration issues and emit one content-free invalidation per affected parent, through the existing broadcaster/relay abstraction. A dedicated `collaboration:children_changed` frame with only `{issue_id: parentUuid}` delivered to **that issue scope** is a precise fit. `registerListeners` already demonstrates explicit scoped handling and exclusion from generic workspace fanout (`server/cmd/server/listeners.go:79`). Never relay the ordinary child payload/body to a collaboration room.
3. Add a strict core event parser/branch that invalidates `collaborationKeys.issue(scope, parentUuid)` and any child-query subkeys. This hint has no collaboration event sequence and must not pass through the sequence freshness gate. Keep normal protected reads; do not advance the access epoch, cancel a pending POST, clear drafts, or infer membership.
4. Preserve final-send issue authorization. Account/source denial cleanup continues to use the existing access/source-access events. A source membership change must downgrade openable references even when participation remains valid.
5. Keep the ordinary write transaction free of collaboration session/participant/command locks and do not advance `collaboration_revision`, lifecycle/control versions, or create participant notifications. Do not reuse a lifecycle/edit operation just to refresh the child section.

This proposed hint follows the ordinary event bus's transient delivery semantics. Existing polling/focus/reconnect reads remain recovery for loss. It is not a claim of durable child-event delivery. If durable delivery is made an explicit requirement, define an outbox entry within the ordinary write owner instead of silently introducing parent collaboration workflow locks. Existing `recordCollaborationExecutionChange` advances the parent's collaboration event sequence (`service/collaboration_realtime.go:165`) and should not be copied blindly into an ordinary child writer.

### Concrete remaining legacy-entry gaps

| Surface | Current behavior and smallest correction |
| --- | --- |
| Shared context/dropdown menus | `IssueActionsMenuItems` returns only Open in collaboration for a shared issue (`packages/views/issues/actions/issue-actions-menu-items.tsx:177`); its generic task query is correctly disabled (`:137`). Add safe Copy link / Open in new tab actions using the account destination. Keep ordinary mutation/plugin/source-copy actions excluded. |
| Common copy/open helper | `useIssueActions.copyLink` (`use-issue-actions.ts:158`) and `openInNewTab` (`:129`) always use the source workspace path. Branch shared identities to `accountPath({view:"participating", issueId: issue.id})`, using the existing `getShareableUrl` and native tab adapter. Preserve ordinary identifier-form links. A shared link must be openable by an outside participant with no source membership. |
| Search command copy | `packages/views/search/search-command.tsx:439` adds Copy issue link for the current source issue and copies `pathname` at `:448`. Use the same shared account-path decision. The palette's account-page behavior is not a substitute for a discoverable Copy link action on shared detail. |
| Mixed batch selection | The surface controller refuses batches containing a known shared issue before generic transport (`packages/views/issues/surface/use-issue-surface-actions.ts:164`). However, `BatchActionToolbar.handleBatchAssignee` can open ordinary `issue-run-confirm` before calling that guard (`packages/views/issues/components/batch-action-toolbar.tsx:129`). Check `selectedIssues` for shared identities before the preview/confirmation path. |
| Batch fallback without a surface | The toolbar directly calls `useBatchUpdateIssues` / `useBatchDeleteIssues` when no surface context exists (`batch-action-toolbar.tsx:98`, `:152`). This occurs in embedded ordinary detail child lists. Guard there too; a shared+ordinary selection should show the existing `source.batchUnsupported` explanation, retain selection, and not trigger a write or ordinary AI preview. |
| Shared child inside ordinary parent detail | `SubIssueRow` owns direct `useUpdateIssue().mutate` calls (`packages/views/issues/components/issue-detail.tsx:675`, `:702`) and ordinary inline pickers, independent of the surface guard. Its context menu is restricted, but that does not protect its other controls or supply external account display. Render a shared-specific row branch using the existing AccountSummary component and shared destination/actions; do not expose ordinary inline writes. This is distinct from the new ordinary children under a shared parent. |
| Main source cards/lists/tables | Already use AccountSummary for external actors: `board-card.tsx:143`, `list-row.tsx:177`, `gantt-view.tsx:419`, `table-view.tsx:1129`. `board-view.tsx:541`/`:588` prevents optimistic cross-column shared moves; surface actions route transfer/status/metadata to shared workflows. Add targeted integration coverage instead of rewriting these paths. |
| Legacy source-copy menus | Shared detail/body/comment selection must omit manual and agent create-subtask entries; backend denial is already authoritative. Ordinary child source-copy remains available. Test the actual selection/menu path, not only absence of a static button before a popup opens. |

Do not represent an old source-workspace URL as an account link. Source members can still use their existing source route and panels; freshly copied collaboration links should use the account UUID destination. This report does not recommend weakening a workspace route guard so an outside account can browse it.

### Suggested ownership paths

One server owner should own all server/SQL edits in this increment, as coordinated by the parent, including adjacent R52 work. The following are the R51-specific responsibilities:

| Owner | Files / responsibility |
| --- | --- |
| Server | `server/internal/handler/collaboration.go`; `server/internal/service/collaboration.go`; a focused `server/internal/service/collaboration_children.go`; a minimal child query in `server/pkg/db/queries/collaboration_source.sql` or a dedicated `collaboration_children.sql`, with regenerated sqlc output. Projection and source-open authority only. |
| Server | `server/internal/handler/issue.go`, `server/internal/service/issue.go`, `server/cmd/server/listeners.go` or a focused child-listener companion, and shared event constants: old/new-parent capture and postcommit scoped child invalidation. Preserve ordinary I1/execution ownership. |
| Server tests | `server/internal/handler/collaboration_children_test.go` for the new projection and ordinary attach/detach authority matrix; existing source-context/execution boundary suites remain their canonical owners. Scoped broadcaster/real WS tests under `server/cmd/server/` own privacy and final-send behavior. |
| Core | `packages/core/collaboration/schema.ts` (or pure `children-schema.ts` imported there), `packages/core/collaboration/index.ts`, `packages/core/realtime/collaboration.ts`, `packages/core/types/events.ts`, and relevant tests. Optional block compatibility, path construction, malformed redaction and normal-event invalidation. |
| Core / shared navigation | A pure shared-destination helper may live beside `packages/core/issues/source-collaboration.ts` or use the existing `accountPath` directly. Keep API schemas free of request-bearing imports. |
| Views | New `packages/views/collaboration/children.tsx`; `collaboration-detail.tsx`; source menu/common action helper; `packages/views/search/search-command.tsx`; `batch-action-toolbar.tsx`; the shared branch of ordinary `SubIssueRow`; `packages/views/locales/en/collaboration.json` and `zh-Hans/collaboration.json` for new copy/child copy. |
| Integration | Extend `e2e/fixtures/collaboration-editing.ts`, `e2e/collaboration-editing-web.spec.ts`, and the named editing/entry scenario in `e2e/collaboration-account-desktop.spec.ts`. Reuse `TestApiClient` and the isolated native profile harness. |

No migration or new dependency is needed for the unpaginated read projection and transient hint design. Do not add files under Mobile or duplicate shared components under platform apps.

### Meaningful regression and acceptance checks

**Canonical server tests**

1. Ordinary source member who is not a participant/creator/handler can attach and detach an ordinary child. Snapshot parent collaboration/session/participants/commands/grants/history before and after; collaboration revision, handler, participant set and queue remain unchanged. Child revision and ordinary parent field change normally.
2. Reject foreign-workspace parent, nonmember actor, stale child revision and invalid parent/cycle without partial child changes. Use a transaction barrier for source departure versus write and for a simultaneous collaboration handoff; avoid sleeps. Handoff is not child-write authority.
3. Projection for source member versus outside creator/handler/participant: exact child-field allowlist, source slug/can-open pairing, no body/comment/file/resource sentinels, no source catalogs. Explicitly include a child with private body/attachments and an independently shared sibling to check exclusion.
4. Zero children is a supported empty block. More than 200 ordinary children all appear in stable order, or every cursor page is traversable if pagination is chosen. A child lookup error is unavailable, not an empty success.
5. Detach, change parent, rename and delete update the projected set; they retain all parent collaboration history. No parent participant notifications are generated by R51 operations.
6. Ordinary child execution remains normal and has no parent collaboration run/group/grant binding. Reuse `legacy_execution_boundary_test.go` for the broad runtime-selector matrix; add only new lifecycle assertions not already owned there.
7. Machine/unknown identities still cannot obtain shared detail through child routes or the new block. Keep the existing source-copy preview/manual/agent refusal tests and the ordinary-child control.
8. Postcommit child hints reach only authorized parent issue subscribers and have the exact minimal payload. Attach/detach both parents and batch deletion are covered; rollback emits no hint. Removal before final send drops delivery. No collaboration edit revision or pending human operation is changed by delivery.

**Canonical core tests**

- Extend `packages/core/collaboration/schema.test.ts` or a focused children schema suite: old-server omission, empty supported set, malformed-present block, duplicate child IDs, invalid UUID/slug, contradictory `can_open`/slug, private unknown fields/redacted failures. Do not silently promote malformed shared data to ordinary Issue.
- Pure path test: source-open uses the returned source slug rather than current workspace; shared Copy/Open uses account UUID and keeps comment anchors where supplied; ordinary links retain identifier paths.
- Extend `packages/core/realtime/collaboration.test.ts`: strict child hint invalidates parent reads without changing access epoch/drafts; malformed, obsolete-server/viewer and unrelated parent hints do not corrupt state. Source-membership loss downgrades open authority while retaining valid participation. Reconnect remains GET-only.

**Shared view tests**

- A focused children component test: source member gets an accessible AppLink; external participant gets text with no anchor/prefetch/generic API; missing support differs from empty. Shared detail has one composed-section wiring test.
- `packages/views/issues/actions/__tests__/use-issue-actions.test.tsx` and `issue-actions-menu.test.tsx`: shared Copy/Open emits account URL, ordinary stays identifier URL; shared menu never queries tasks or shows source-copy/plugin/ordinary mutation actions.
- `packages/views/search/search-command.test.tsx`: source shared Copy issue link uses the account route.
- `packages/views/issues/components/batch-action-toolbar.test.tsx` / `.confirm.test.tsx`: mixed selection refuses before agent preview, generic transport or selection clearing, including no-surface usage. Ordinary selection still works.
- `packages/views/issues/components/issue-detail.test.tsx`: ordinary parent containing a collaboration child displays external AccountSummary and safe actions, without generic inline mutation. Existing `:733` classification test already guards shared detail before ordinary hooks; retain it.
- Shared body/comment selection tests exercise mounted selection controls and confirm no manual/agent subissue capture is available. The normal ordinary detail control remains available.

**Web end-to-end**

Extend the real-API source-editing fixture with a normal source member, an outside recipient, and ordinary children with private sentinel content. Through UI create an ordinary issue, attach it with Set parent, open the shared source detail and the account detail, and detach it again. Assert source link canonicalization and outside text-only presentation. Observe the actual WebSocket frame and immediate refresh after attach/detach/rename/delete without reload; a polling-only success is not realtime evidence.

Exercise a mixed ordinary/shared table/list selection and agent-assignee entry, confirming no ordinary run-preview or generic write happens. Copy the collaboration link, open it in a separately authenticated outside browser context, and verify the same UUID without any source member/project/issue catalog requests. Include comment-anchor sharing if adding that action.

The current `e2e/collaboration-editing-web.spec.ts:26` comment mentions a linked child, but its fixture makes the **collaboration issue a child of an ordinary parent**. It does not prove R51 ordinary children under a shared parent. Its outside-catalog sentinel and source-workspace empty-frame assertions are useful reusable patterns.

**Native Electron**

Extend the actual native editing/entry flow in `e2e/collaboration-account-desktop.spec.ts:346` using the existing isolated profile, real API/WS, production renderer and `openDesktop` harness. Verify shared source route, panel/popout and account detail show the same child projection. Click an authorized ordinary-child link from the account tab/popout; verify correct source workspace group and final identifier route. Copy shared link through native clipboard/share handling and reopen as an outside account destination. Check restart retains route/layout without persisting server child data, and verify 200% zoom/no horizontal overflow.

Avoid adding a complete duplicated matrix to both app suites. Core owns wire/privacy matrices; shared views own behavior; Web/Electron own the actual routing/clipboard/WS integration. Existing native tests already cover account links, popouts, zero-workspace/list-error boot and restart; extend the closest named cases.

### Related specifications and task artifacts

- `CLAUDE.md`: Query/store split, package boundaries, API parsing/redaction, source UUID handling, tests beside their owners, no real-agent execution in default tests.
- `.trellis/workflow.md`: research persistence and role isolation; this report does not activate, mutate or complete the task.
- `.trellis/spec/server/collaboration-source-editing.md`: source markers, classification before ordinary hooks, source-only structure, minimal external references, source tick redaction.
- `.trellis/spec/core/frontend/collaboration-workbench.md`: scope/access-epoch fencing, optional routes, original intent and no generic protected-file fallback.
- `.trellis/spec/server/collaboration-realtime.md`: independently authorized issue scopes, final-send authorization, content-free hints and polling/reconnect recovery.
- `.trellis/spec/desktop/frontend/account-navigation.md`: account/workspace window context, persisted identity, native routing and evidence limits.
- `.trellis/spec/web/frontend/e2e-run-environment.md`: production build/start for Playwright and canonical UUID-to-identifier navigation assertions.
- Task `prd.md:73` (R51), `:78` (R56); `acceptance.md:91` (AC82), `:93–98` (AC84–89); `design.md` D03/D08/D09; `implement.md` P1/P6; `research/execution-contract.md` E02/E08.
- `research/remaining-product-work-2026-10-18.md` was updated 2026-10-19 and explicitly retains R51 ordinary-child presentation plus exhaustive old-entry compatibility as incomplete.

### External references / versions

No external documentation was needed for this bounded internal contract map, and no package upgrade is proposed. Recommendations reuse the checkout's existing React Query, zod, NavigationAdapter/AppLink, Go/sqlc and realtime primitives. The accepted collaboration execution read protocol is currently v5; R51 does not revise it.

## Caveats / Not Found

- Research only: no implementation, application tests, browser run or Electron run was executed. Existing tests cited above are discovered coverage, not freshly passing evidence. No git operations or task-state writes were performed.
- Only this research file is owned by this agent. Code, manifests, specs, acceptance checkboxes and other task artifacts remain for the parent and implementation owners.
- R51 requires source membership for opening an ordinary child, not just participation or manager authority on its parent. Do not let broad account workspace-list cache state become the permission source.
- The proposed child hint is transient like the ordinary bus. A stronger durable-delivery requirement would need an explicit additional design; do not claim the collaboration outbox already covers ordinary child updates.
- Parent-direction stage wakeup remains a separate parent-task caveat, excluded from this increment by the parent's latest scope direction. `ChangeCollaborationLifecycle` calls the dedicated service and writes a response but does not call ordinary `notifyParentOfChildDone`; existing stage-edit tests do not establish a collaboration child's completion waking an ordinary agent parent. An ordinary child finishing under a human-assigned collaboration parent appropriately creates no ordinary AI wake (`issue_child_done.go:112`). Do not mark all of AC89 complete from R51/reorder tests alone.
- Public creation/execution rollout, P7 administration and full AC01–97 acceptance remain separate gates. This research does not justify enabling them.
