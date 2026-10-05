# P1 frontend source map

Date: 2026-10-05. Read-only source inspection at local HEAD `4920b0d1b`; this document does not claim P1 implementation or passing application tests. Scope: shared core/views, Web/Desktop entry points, and mobile compatibility. Source paths below are repository-relative, with line numbers from the inspected worktree.

## Required contracts

- `CLAUDE.md:25` establishes Query ownership of server data, client-only Zustand state, captured workspace query identities, conservative optimism, and server-first create/delete/confirm flows. `CLAUDE.md:137` requires drift-safe response schemas; shared UI must remain in views with navigation adapters.
- Read `.trellis/spec/core/frontend/index.md`, `.trellis/spec/views/frontend/index.md`, `.trellis/spec/web/frontend/index.md`, and `.trellis/spec/desktop/frontend/index.md`. Most generic sections are placeholders; active focused guides supplement the root rules. No deeper AGENTS.md or CLAUDE.md was returned under these layers in this inspection.
- `docs/plans/2026-10-04-work-management-prds/projects-prd.md:191` defines the full formal-issue set; filtering, paging, folding, actor tabs, and hidden columns cannot change it. Lines 199–207 separate formal total, completed, cancelled, unfinished, and scope-closure percentage. Lines 213–245 require one timed/timezone-aware health snapshot, exact risk sets, unknown/incomplete states, and independent project status.
- P1 also includes explicit acceptance records and description-version validity, not just the four short feature labels in the request: PRD lines 161–185 and 251–267. Project completion cannot synthesize acceptance or trigger execution.

## Project API and cache baseline

| Area | Existing source and behavior | P1 implication |
| --- | --- | --- |
| Types | `packages/core/types/project.ts:22` defines `Project`, including `issue_count`, `done_count`, dates, lead, and execution squads. `UpdateProjectRequest` at line 63 has no description version, completion reason, health, or progress fields. | Extend the existing identity. New progress/history and snapshot contracts need types; do not create a parallel “专项” entity. |
| Schema | `packages/core/api/schemas.ts:1371` defines `ProjectObjectSchema`; old date fields default null, counters default zero, additive squad failures remain recoverable. `ProjectSchema` is exported at line 1402. | Preserve old response parsing. Missing P1 statistics must remain distinguishable from an authoritative zero. Validate complete snapshots as a unit and do not synthesize healthy/accepted results from defaults. |
| API identity | `packages/core/api/client.ts:4468` `parseProjectResponse` rejects malformed/mismatched project or workspace identity. `listProjects` at 4477 verifies every returned workspace; detail/create/update at 4493–4509 carry captured workspace and signal. | Follow this boundary for overview, progress, evidence, and history. Mutation success must be based on a parsed authoritative response. |
| Query identities | `packages/core/projects/queries.ts:4` defines `["projects", wsId]`, list, and detail prefixes; read functions forward signal at lines 11–22. | Keep overview/progress/history/description-version keys under the same workspace/project ownership; overview identity must exclude personal view filters. |
| Update | `packages/core/projects/mutations.ts:84` `useUpdateProject` optimistically spreads every request field into list/detail, rolls back, then invalidates. | Description conflict tokens and completion audit commands cannot simply reuse unrestricted optimism. Separate versioned description/completion operations or branch the existing hook narrowly; retain simple predictable property patches. |
| Create | `packages/core/projects/mutations.ts:59` only inserts the parsed project on server success. `packages/views/modals/create-project.tsx:396` flushes editor input, captures the draft, awaits creation, and protects newer draft ownership at lines 417–422. | Reuse this server-first/draft-identity pattern for publishing progress. Persist the idempotency key with the pending command rather than generating a fresh key on every retry. |
| Delete debt | `packages/core/projects/mutations.ts:114` currently optimistically removes list/detail before the DELETE succeeds; it clears view state on success. API delete at `packages/core/api/client.ts:4534` has no explicit workspace argument. | This is existing debt contrary to the root contract. A P1 delete-touching slice should pin workspace and wait before cleanup; then clear all project-owned overview/progress/history/drafts and task projections. Do not reproduce it in new APIs. |

### Counter semantics are already server-owned and T1-aware

`server/pkg/db/queries/project.sql:69` computes `GetProjectIssueStats` from `admission_status IN ('not_required', 'accepted')` within the workspace/project set; line 72 counts terminal status keys into `done_count`. `server/internal/handler/project.go:81` obtains `projectTerminalIssueStatusKeys` before requesting these aggregates. The existing PRD documents terminal scope closure as done plus cancelled.

The UI presently labels that aggregate loosely:

- `packages/views/projects/components/project-issue-metrics.ts:3` returns `completedCount: project.done_count`; it does not compute from loaded tasks.
- `packages/views/projects/components/project-detail.tsx:259` consumes this helper; sidebar count at line 457 is `completedCount/totalCount`.
- `packages/views/projects/components/projects-page.tsx:131` sorts by `done_count/issue_count`; `ProgressRing` at line 192 renders the same numerator (line 216), with no-task dash.
- `packages/core/projects/stores/view-store.ts` owns project list display preferences, distinct from project-owned statistics.

Preserve `done_count` semantics for installed clients. Add explicit actual-completed and cancelled values (and an honest unavailable state on old servers) rather than silently redefining `done_count`. Calculate/sort with unrounded values. Keep project status, scope closure, and goal acceptance independent.

## Description templates, editor reuse, and drafts

- The current description lives in the project sidebar: `packages/views/projects/components/project-detail.tsx:475` uses `ContentEditor` with `value={project.description || ""}` and 1500 ms autosave through the generic mutation. It discards the editor's baseline argument and has no template picker, version conflict flow, acceptance section, or project-progress editor.
- `packages/views/editor/content-editor.tsx:122` explicitly defines `onUpdate(markdown, baseMarkdown)`: the base is what the dirty editor actually adopted, not necessarily the newest query-cache value. Its `ContentEditorRef` at line 252 exposes `getMarkdown`, `insertMarkdownAtEnd` at 295, `flushPendingUpdate` at 334, and authoritative-content adoption further below. These are sufficient for preview then append of a Markdown goal template without introducing a second target document or overwriting existing prose. Check the insertion boolean because the Tiptap instance is created after the imperative ref exists.
- `packages/views/issues/components/issue-detail.tsx:2258` provides an existing description conflict pattern: serialize saves, retain the newest queued draft, send `description_base`, protect the issue identity on late responses, and retain local text on `revision_conflict`. `RevisionConflictCompare` is already in `packages/views/issues/components/revision-conflict-compare.tsx`; reuse an appropriately shared rendering boundary rather than copying issue-specific state wholesale. Project description revision must be independent of unrelated icon/date/lead updates so old acceptance is only invalidated by actual description changes.
- `packages/core/projects/draft-store.ts:35` uses `createDraftStore` for a single new-project draft per workspace. It is not a per-project progress store. `packages/core/drafts/create-draft-store.ts:54` provides workspace-aware persistence, merge/default migration, rehydration, and automatic cleanup registration. `packages/core/issues/stores/comment-draft-store.ts:31` explains why a record ID must additionally be in keys; its `new:/reply:/edit:` keyed map at line 36 is the closer model for simultaneous Desktop progress composers.
- New progress drafts need workspace + project + create/correction identity, pending command key, base revision, body/type, and acceptance/evidence selections where applicable. Only input/intent goes into Zustand; published records and system statistic snapshots remain Query-owned. A late publish must not clear a newer edit or another project's draft.
- `packages/core/drafts/register-all-drafts.ts:17` eagerly imports registered stores so cleanup is complete even if a composer was never loaded. Add a new module-level draft store here. `packages/core/drafts/cleanup-registry.ts:53` deletes persisted drafts by workspace; `resetAllRegisteredDrafts` at 70 clears in-memory drafts and aborts uploads during logout. Deletion/revocation needs an explicit project/workspace-aware in-memory reset, not just storage-key removal.
- Mention markup already preserves typed references: `packages/views/editor/extensions/mention-extension.ts:91` parses `mention://type/id`, and line 108 serializes it. `ContentEditor` permits normal mentions by default and can disable suggestion creation while preserving nodes (`packages/views/editor/content-editor.tsx:181`). Reuse syntax/UI only: progress publishing must not call issue comment/send/run endpoints, and an agent mention must remain a reference. Member notifications and evidence authorization are server responsibilities.

## Overview and exact risk drill-down

### Existing five-mode task surface

- `packages/views/projects/components/project-detail.tsx:129` restores `useIssuesScope(project:<id>)`; actor kinds may already be member-only or agent-only. Lines 137–147 derive existing default-squad task creation choices. Keep those choices separate from management actions.
- Detail currently renders `ProjectSquadSection` then `ProjectIssueSurface` at lines 570–576; it has no overview/tasks section switch.
- `packages/views/projects/components/project-issue-surface.tsx:45` delegates to `IssueSurface` with `board`, `list`, `table`, `swimlane`, and `gantt`; it also owns linking existing tasks. Reuse this surface and retain resources/automation/default-squad entry points.
- `packages/core/issues/surface/query-plan.ts:10` states that table/list/board/swimlane membership is answered by the server Table channel; Gantt uses a separate legacy list projection. Project mapping at line 73 binds `project_id` and actor types, not health predicates.

### Why setting a few current filters is insufficient

- `packages/views/issues/surface/issue-surface.tsx:81` always loads an active saved view. At 95 it substitutes `view:<id>` for even a caller-supplied `surfaceKey`; at 111 it can override actor scope from the saved view. A unique `surfaceKey` by itself does **not** isolate a health drill-down.
- `packages/core/issues/stores/surface-view-store.ts:111` persists display/filter state per surface; `packages/core/issues/surface/scope.ts:63` gives projects separate all/member/agent keys. `packages/views/issues/surface/use-issue-surface-controller.ts:218` reads personal status/assignee/date/property filters, `showSubIssues`, and hidden status categories. At line 491 these are compiled into the server query, including `include_sub_issues`.
- `packages/core/types/api.ts:295` `IssueTableFilters` currently supports concrete status keys, explicit assignees and `include_no_assignee`; its date filter only accepts `created_at` or `updated_at`. It has no due-date-before-baseline-day predicate, invalid-reference predicate, health rule, or snapshot version. `include_no_assignee` must not be assumed equivalent to the PRD's missing/invalid member/agent/squad reference definition.
- `IssueSurface` exposes `clientFilter` (`packages/views/issues/surface/issue-surface.tsx:57`), but filtering a loaded/paged window cannot establish project-wide risk counts or exact result membership. Gantt intentionally shows a scheduled projection, so a no-date risk task cannot be silently discarded while the page claims to show the full risk set.

Recommended integration boundary for the technical design: a server-issued project health predicate/snapshot token and an explicit temporary drill-down context that bypasses active saved-view and personal membership constraints, while reusing task rendering/actions. Open an exact list/table result initially; clearly indicate applied health condition and any later display limitation. Include subtasks, clear search/actor/hidden-status constraints for the initial result, and preserve the user's normal saved preferences when leaving. The risk result must either honor the snapshot version or visibly refresh both total and membership with a new version. Do not present ordinary query fingerprints as a database snapshot guarantee.

Use `packages/core/issues/status-category.ts:20` `issueStatusCategory` for pure category interpretation when needed: unknown custom status returns null. Do not use `statusCategoryOfKey` at line 35 or `issueColumnCategory` at 55 for correctness metrics; their todo fallback is a rendering affordance. Prefer the authoritative server aggregate to fetching every task in the client.

## Realtime, revocation, and compatibility

| Concern | Source evidence | Required design coverage |
| --- | --- | --- |
| Project events | `packages/core/realtime/use-realtime-sync.ts:795` invalidates `projectKeys.all` for generic `project:*`; reconnect refresh includes projects at line 655. | Place P1 queries beneath owned prefixes or explicitly invalidate them; progress/state changes need a documented event contract with project/workspace identity. |
| Task changes | `packages/core/issues/cache-coordinator.ts:606` `invalidateIssueDerivatives` refreshes project keys only when `statusOrProjectChanged`. | Health also changes on task due date, assignment and deletion; member departure, agent/squad archival, timezone change and day rollover. Extend the dependency map rather than assuming current project refreshes cover these. |
| T1 admission | `packages/core/triage/cache.ts:11` invalidates normal issue/project/dashboard projections for `triage:updated`, parsed at line 45; generic issue edits are handled in realtime at line 1003. | Preserve T1 formal admission exactly; a pending/rejected/duplicate item never becomes a risk result through optimistic insertion. |
| Catalog/entity changes | Realtime generic handlers at `packages/core/realtime/use-realtime-sync.ts:767`, 779, 799, and 831 refresh agents, members, squads, or status catalogs, but not all project aggregates. | Invalid lead/assignee and reclassified statuses must refresh health without treating offline runtimes as unassigned. |
| Workspace loss | `packages/core/realtime/use-realtime-sync.ts:1272` handles deletion, 1292 handles current-member removal; `relocateAfterWorkspaceLoss` at 1253 first awaits a workspace list then full-page navigation. `packages/core/platform/storage-cleanup.ts:35` clears stored workspace state. | Existing full reload is not proof of immediate sensitive cache removal. P1 must cancel/remove protected query data and clear accessible draft/editor content promptly on 403/revocation, even if destination lookup or navigation fails. Coordinate a single owner/self-event guard rather than adding competing redirects. |
| Older backend | `packages/core/api/client.ts:1519` is T1 precedent: only the dedicated settings endpoint's 404 means unsupported; other errors propagate. Tests at `packages/core/api/triage-client.test.ts:144` lock this distinction. | P1 currently has no capability endpoint or flag. Define explicit supported/unavailable/error states. A resource 404/403/network/parse failure cannot be advertised as “P1 unsupported” or a successful empty overview. |
| Daemon version is different | `packages/views/chat/components/use-chat-project-context-support.ts:21` checks runtime CLI version for chat prompt context. | Do not repurpose this as backend P1 capability detection. |

### Platform entry points

- Shared path factory: `packages/core/paths/paths.ts:41` and 42 provide `projects()` and `projectDetail(id)`.
- Web: `apps/web/app/[workspaceSlug]/(dashboard)/projects/page.tsx` and `apps/web/app/[workspaceSlug]/(dashboard)/projects/[id]/page.tsx:6`; detail simply renders the shared `ProjectDetail`.
- Desktop: `apps/desktop/src/renderer/src/routes.tsx:133` and 138 wire project list/detail; `apps/desktop/src/renderer/src/pages/project-detail-page.tsx:8` loads the record for the native title and renders the same shared component. New section/risk query state should go through shared paths/navigation and both platform adapters; do not add a parallel root route or lose refresh/back/deep-link behavior.

### Mobile compatibility boundary

Read `apps/mobile/CLAUDE.md` before mobile work. Mobile may share types and pure schemas/helpers but owns hooks, state, queries, and realtime.

- `apps/mobile/data/queries/projects.ts:21` mirrors workspace-scoped keys, but its list cache stores `Project[]` (not the Web/Desktop response envelope). Project issue reads at line 63 use the normal formal-filtered backend issue endpoint.
- `apps/mobile/data/api.ts:900` and 937 parse project reads through the shared schemas. Existing writes at 956/963 use generic `fetch<Project>` without the same explicit schema validation; do not copy that gap for any P1 endpoint.
- `apps/mobile/components/project/project-header-card.tsx:51` and `apps/mobile/components/project/project-row.tsx:63` still use `done_count` as scope closure. Changing that field's meaning would change installed/mobile progress silently.
- `apps/mobile/data/realtime/project-ws-updaters.ts:63` replaces detail from an authoritative project event; list patching at 23 merges partials. P1 progress-summary additions must not be lost under mixed old/new events. Mobile's own `use-project-realtime.ts` and `use-projects-realtime.ts` own subscription/reconnect behavior.
- P1 mobile editing remains deferred by the PRD. Keep old status/property CRUD legal, preserve new server-side progress/acceptance when an older client updates other fields, and explicitly limit unsupported new editing. A new shared schema must not require P1 fields from every old backend or treat malformed snapshot data as an empty healthy project.

## Existing tests and the missing P1 matrix

| Existing canonical location | Current evidence | P1 extension |
| --- | --- | --- |
| `packages/core/api/project-execution.test.ts:27` | Old response/squad drift, malformed candidates, workspace and project identity. | Optional/additive P1 fields, malformed snapshot/progress response, endpoint capability/error distinction, mixed client contracts. |
| `packages/core/projects/mutations.test.tsx:27` | Delete success clears issue-surface view state. | Server-first deletion failure, captured workspace, versioned description failure, idempotent publish/correction, no late draft clearing. |
| `packages/views/projects/components/project-issue-metrics.test.ts:5` | Counters come from the project record. | Actual completion versus cancellation, missing-old-backend data, zero denominator; pure matrix belongs beside a core helper if shared with mobile. |
| `packages/views/projects/components/project-detail.test.tsx:373` | Empty task workspace, linking, default-squad creation, sharing, delete confirmation/admin gate. | Overview/task switch, exact risk navigation, completed warning, acceptance staleness, revocation, resources/squad/automation retained. |
| `packages/views/projects/components/projects-page.test.tsx:253` | List row navigation, inline-control isolation, modifier/new-tab behavior. | Correct independent counters/health/latest-progress and existing navigation preserved. |
| `packages/views/modals/create-project.test.tsx:377` | Unmount/workspace switch/newer-draft protection after create. | Template preview/appending and progress composer equivalent race cases. |
| `packages/views/issues/components/issue-detail.test.tsx:1933` and `revision-conflict-compare.test.tsx` | Conflict draft preservation and queued description save races. | Reuse the established interaction and add project-specific description-version/acceptance wiring. |
| `packages/core/triage/mutations.test.tsx`, `packages/core/triage/queries.test.ts`, `packages/core/realtime/use-realtime-sync.test.ts`, draft cleanup suites | T1 cache and lifecycle patterns. | Admission changes update all P1 aggregates; due-date/assignee/catalog/day changes converge; revoked cache/drafts cannot remain visible. |
| `e2e/project-squad-workspace.spec.ts`, `e2e/triage.spec.ts`, `e2e/triage-desktop.spec.ts`, `e2e/triage-branches.spec.ts` | Existing project/default-squad and T1 cross-surface baselines. | A P1 Web/Desktop journey including custom statuses, subtasks, saved-filter isolation, precise counts, progress retry/correction, and no AI execution from management/mentions. |

No tests were executed for this source-map-only task. Assertions about current behavior above come from inspected code; P1 acceptance evidence must be gathered during implementation, including browser verification for both platforms and production-like E2E serving per the Web spec.
