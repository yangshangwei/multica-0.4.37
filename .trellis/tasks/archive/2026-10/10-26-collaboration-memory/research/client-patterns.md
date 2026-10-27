# Research: client patterns for collaboration memory

- **Query**: core/views/web/desktop/admin/mobile/docs patterns a `memory` panel and run marker must follow
- **Scope**: internal
- **Date**: 2026-10-26 (uncommitted tree; lines may drift)

## 1. `packages/core/collaboration`

- Parsing: every schema file exports `parseX = parseWithFallback<T | null>(data, Schema, null, { endpoint, redact: true })`, e.g. `schema.ts:368-409`, `run-records-schema.ts:98-103`, `content-schema.ts:140`. Versions are decimal strings `positiveVersion`/`nonnegativeVersion` (`version.ts:3-10`). Older-server fields use `.optional()`/`.catch(false)` (`schema.ts:170-212`).
- API: `ApiClient.fetchCollaboration` (`api/client.ts:931-956`) plus `fetchCollaborationOptionalHistory` (`:958-966`). An absent route returns `undefined`, so the result is null/unavailable. Only `{code:"not_found"}` counts as denial. Methods re-check identity, e.g. `sameResourceIdentity(result.issue_id, id)` (`:1153`).
- Keys have no `wsId` and are account-scoped: `["collaboration", apiScope, userId, "issue", issueId]` (`access.ts:19-24`). `apiScope` = server + ApiClient instance + session scope (`access.ts:11-17`). Sub-resources append to the issue key, e.g. `[...issue, "run", runId, "records"]` (`run-records-queries.ts:34`).
- Reads go through `protectCollaborationRead` (`access.ts:101-130`), which fences on epoch. A 401/403/404/410 response triggers `clearCollaborationAccess`, wiping that issue's queries and mutations (`:60-80`). Shared freshness: `staleTime 0`, refetch every 15 s (`queries.ts:11`, `content-queries.ts:7`).
- Writes: `prepare*` builds a `CollaborationDraft`. `execute*` saves it to storage before POST, removes it only after a validated receipt, then invalidates `collaborationKeys.scope` or `issue` (`operation.ts:13-48`, `draft.ts:44-62`). Each write sends an `Idempotency-Key: requestId` header plus four expected epochs (`api/client.ts:1298-1325`, `action.ts:31-40`). Recovery GETs `/operations/{requestId}` (`action.ts:72-83`). Pending drafts are routed by `payload.domain` (`pending.ts:14-41`).
- Conflict handling:
  - Issue edit: a 409 `stale_revision` returns a receipt instead of throwing (`operation.ts:33-36`). The form keeps its `baseline` and shows `staleBaseline`. "Use current version" is an explicit step and never auto-resubmits (`views/.../workflow.tsx:78-84,115-118,184-185`).
  - Comment edit: `expected_comment_revision` (`comment-actions.ts:18,34`). UI `stale` → `commentActions.conflict` + `useLatest` (`comment-list.tsx:69,105-106`).
- Byte limits: `collaborationTextByteLength` (UTF-8) and `COLLABORATION_COMMENT_MAX_BYTES = 65_536`, hardcoded (`text-limits.ts:3-12`).
- Realtime: `bindCollaborationRealtime` (`realtime/collaboration.ts:29-138`). `collaboration:changed` passes the `event_seq` gate (`:35-44`), then invalidates the whole `collaborationKeys.issue` prefix (`:53-58`). Mounted by `useCollaborationRealtime` in `account-workbench-page.tsx:38` and `source-bridge.tsx:38`.
- Files to mirror:

| Role | File |
|---|---|
| schema + parse | `run-records-schema.ts`, `content-schema.ts` |
| queries | `run-records-queries.ts`, `content-queries.ts` |
| write/draft/recovery | `comment-actions.ts`, `operation.ts`, `draft.ts`, `pending.ts` |
| invalidation | `invalidation.ts`, `realtime/collaboration.ts` |
| barrel | `index.ts`; package export `core/package.json:12-13` |
| fixtures | `test-fixtures.ts` (exported), `run-records-test-fixtures.ts` |
| tests (node env) | `run-records-schema.test.ts`, `run-records-queries.test.ts`, `edit-schema.test.ts` (malformed input → null, no private log text) |

## 2. `packages/views/collaboration`

- `CollaborationDetail` (`collaboration-detail.tsx:64-102`) is one vertical stack with no tabs. Order: header → Workflow → description (`RichContent`) → fields → children → reactions → attachments → `CollaborationExecutionPanel` → activity → comments. No collaboration file uses Tabs or Sheet. Anchors come from the URL hash `#run-`/`#comment-`/`#description` (`:46-56`; `core/paths/account.ts:28-44`), which only allows those three kinds.
- Detail mounts in two places: `account-workbench-page.tsx:111` and `source-bridge.tsx:45` (the latter from `issues/components/issue-detail.tsx:1168` when `issue.collaboration` is set).
- `run-records.tsx`: `CollaborationRunDisclosure` has a header row of spans (step, agent, mode, progress, proof) plus an open button (`:42-56`). `RunRecords` has a `<dl>` summary, permalink, resource, preparation, usage, items (`:70-97`). Runs are listed per group in `execution-panel.tsx:97`. The run DTO is `execution-schema.ts:95-127`.
- Markdown: rendering is always `RichContent resourcePolicy="restricted"` (`rich-content/rich-content.tsx:491`). Editing is always a plain `Textarea` (`workflow.tsx:160`, `comment-list.tsx:100`, `creation-form.tsx:130`). ContentEditor is not used.
- i18n: namespace `collaboration` (`locales/en|zh-Hans/collaboration.json`, registered in `locales/index.ts:2-3,70,103`). Keys use camelCase sections (`records.*`, `fields.changed`, `fields.useCurrentVersion`). `locales/parity.test.ts` enforces en/zh key parity.
- Tests sit beside each component as `*.test.tsx`. Siblings are mocked via `vi.mock` (`collaboration-detail.test.tsx:12-35`). E2E specs: `e2e/collaboration-*.spec.ts`.

## 3. Web/Desktop wiring

- Web: `apps/web/app/account/page.tsx` → `apps/web/platform/account-workbench-route.tsx:10-25`.
- Desktop: `routes.tsx:115`, `issue-window.tsx:26-35` → `platform/account-workbench-route.tsx:10-20`.
- Both only parse `AccountDestination` and render `AccountWorkbenchPage`, so a panel inside `CollaborationDetail` needs no platform wiring. Version deep links would require extending `parseAccountPath`/`accountPath` in core.

## 4. Admin settings

- Server: `handler/admin_settings.go:28-54` reads `AdminReadConfig` (`:10-20`), filled at `cmd/server/router.go:465-468` from env (`configuredAdminRetention("MULTICA_ADMIN_*")`). Configuration includes `read_only: true`.
- Client: `parseAdminSettings` (`core/admin/observability-schema.ts:60-67,92`), `useAdminSettings` (`observability-queries.ts:41,65`), and `AdminSettingsPage` (`views/admin/settings/settings-page.tsx:5-23`), which renders `MetricRows layout="settings"` sections (`observability/common.tsx:99`). No mutation controls are tested (`read-pages.test.tsx:24`).
- Routes: web `apps/web/app/admin/settings/page.tsx`. No desktop `/admin/settings` route was found.

## 5. Mobile

- Routes all live under `app/(app)/[workspace]/` (tabs: inbox, my-issues, chat, more). The only workspace-less screen is `select-workspace.tsx`. There is no account route and no "my work" view across workspaces: My Issues is workspace-scoped via `assignee_id`/`creator_id`/`involves_user_id` (`data/queries/my-issues.ts:26-54`).
- `git grep -i collaborat apps/mobile` finds nothing.
- Data: `data/api.ts` `fetch` (`:202-321`) adds `X-Workspace-Slug` whenever a slug is current (`:223-226`). Helpers `fetchValidated`/`fetchValidatedWith` (`:338-372`). Own `parseWithFallback` (`lib/parse-response.ts`). Keys `["issues", wsId, ...]` (`data/queries/issue-keys.ts`).
- Issue text edits are last-write-wins and send no base (`data/issue-edit.ts:3-16`).
- Realtime: own `data/realtime/ws-client.ts`, workspace-scoped (`workspace_slug` URL param, `:199-201`). It has no `subscribeScope`, unlike core `api/ws-client.ts:246-274`. Hooks mount in `[workspace]/_layout.tsx:75-87` or per screen.
- i18n: none. Copy is hardcoded English (`components/inbox/detail-label.tsx:10-11`).
- Markdown: render with `lib/markdown` `Markdown` (hybrid enriched + Shiki). Edit with `TextInput` / `AutosizeTextArea` (`issue/[id]/edit.tsx:166`, `components/issue/description-field.tsx`).
- Server behavior for a human source member:
  - `ListIssues` filters out collaboration rows only for machine callers (`issue.go:1111-1114,1387-1389`). Humans get full rows with an `IssueResponse.collaboration` projection (`issue.go:1238-1250,1678-1690`; `collaboration_source.go:28-52`).
  - Mobile parses that field via core `IssueSchema` (`core/api/schemas.ts:1275-1281`) but never branches on it.
  - `GetIssue` and `ListTimeline` pass `rejectLegacyCollaborationAccess` for any human with Read (`collaboration_boundary.go:61-85`; `CapabilitiesFor` `collaboration/access.go:79-90`).
  - `CreateComment` (`comment.go:1690`) returns 409 `unsupported_collaboration_action`. So does `UpdateIssue` for any field except `position`/`expected_revision` (`issue.go:3895`; `collaboration_boundary.go:121-140`).
  - Net effect: mobile shows a collaboration issue in its lists and opens the ordinary detail (`issue/[id].tsx:46-211`). Comment, edit, pickers and delete stay enabled there and their writes are rejected.
- `apps/mobile/CLAUDE.md` rules the design must address:
  - Pre-flight: read web code, then present the plan with parity points, then wait for an explicit go (`:12-40`).
  - Parity on counts, permissions, enums and identity, and annotate every divergence (`:42-76`).
  - Imports from core: types and pure functions only (`:5-10`).
  - Per-record realtime hook, patch over invalidate, mobile-owned updaters, reconnect invalidates only its own keys (`:198-283`).
  - `fetchValidated*`, three-segment key factory, `signal` forwarding, 30 s timeout (`:313-482`).
  - UI choice order: iOS native, then RNR, then ask. Forms go in formSheet routes with `SHEET_OPTIONS` (`:116-128,486-543`).
  - Check that new directories are git-tracked (`:431-439`).

## 6. Docs

- Glossary: the "Translate fully — concepts" table in `developers/conventions.mdx:157-175` and `conventions.zh.mdx:148+`. It has no collaboration term yet.
- Env vars: `apps/docs/content/docs/environment-variables.mdx` and `.zh.mdx`. The admin retention vars are in `platform-administration.mdx:90-99` and its `.zh` file. Also `.env.example` and `docs/offline-upgrade.zh-CN.md`.

## Caveats / Not Found

- Human comments: `CapabilitiesFor` grants `Comment` to any human source member (`access.go:88`), while `ControlAI` requires participant (`:90-92`).
- Not verified: mobile `DELETE /api/issues/:id`, and whether ListIssues (not search) includes a collaboration issue whose handler is external.
- Related spec: `.trellis/spec/core/frontend/collaboration-workbench.md`.
