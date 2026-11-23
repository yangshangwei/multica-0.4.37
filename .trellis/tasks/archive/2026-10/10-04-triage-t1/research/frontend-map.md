# T1 frontend implementation map

Research date: 2026-10-04. Read-only source inspection in the task worktree; this artifact is the only file written by the researcher. Scope is the full T1 requirements in `docs/plans/2026-10-04-work-management-prds/triage-prd.md`, including FR-26 CSV import. No feature behavior or checks are claimed complete here.

## Constraints and relevant specs

- Read `CLAUDE.md` first. Shared page logic goes in `packages/views/triage/`; server state, API types, queries and mutations go in `packages/core/`. Views may not import either platform router. No new dependencies without explicit request.
- `.trellis/spec/views/frontend/component-guidelines.md` makes settings IA, group ordering, locale parity and accessible compact settings navigation explicit contracts. `.trellis/spec/views/frontend/quality-guidelines.md` documents callable-store mocks and proxy-store pitfalls.
- `.trellis/spec/core/frontend/index.md`, `.trellis/spec/views/frontend/index.md`, `.trellis/spec/web/frontend/index.md`, `.trellis/spec/desktop/frontend/index.md` identify the affected packages. Many generic guides remain templates, so CLAUDE.md and actual code are authoritative.
- `.trellis/spec/web/frontend/e2e-run-environment.md`: E2E target must be a production Next build/start, not next dev.
- Read `apps/docs/content/docs/developers/conventions.mdx` and `.zh.mdx` before naming new code or editing Chinese UI copy.

## Shared page and route wiring

| Responsibility | Existing location/interface | Required T1 integration |
| --- | --- | --- |
| Workspace URL builders | `packages/core/paths/paths.ts`, `paths.workspace(slug)`, `useWorkspacePaths()` | Add `triage()` and optionally a history URL helper; prefer `/{slug}/triage?view=history` if queue/history share the shell. Keep issue identity in the existing issue detail URL. |
| Web page | `apps/web/app/[workspaceSlug]/(dashboard)/issues/page.tsx` is a thin shared-view export | Add equivalent `triage/page.tsx` using the dashboard shell. |
| Desktop page | `apps/desktop/src/renderer/src/routes.tsx`, `appRoutes` under `:workspaceSlug` | Add `path: "triage"`, shared view and handle title. This is a session route, not an overlay. |
| Page identity/tab icons | `packages/core/paths/route-icons.ts`, `WorkspacePageKey`, `NavLabelKey`, `WORKSPACE_PAGES`; `packages/views/layout/route-icon-components.tsx` | Register triage page and icon so desktop tabs have a translated title and stable icon. Existing `Inbox` icon can be reused without a new icon name; a distinct icon requires updating the component mapping too. |
| Sidebar | `packages/views/layout/app-sidebar.tsx`, local `NavKey`/`NavLabelKey`, `workNav`, `NavRow` | Add enabled-only work-group row with a global actionable count from a dedicated query, not a filtered list length. Sidebar already renders counters on personal navigation rows. |
| Exports | `packages/views/package.json`, `packages/core/package.json` use explicit subpath exports | Add `@multica/views/triage`, `@multica/core/triage` exports. |
| Shared navigation | `packages/views/navigation`, `useNavigation()`, `AppLink` | Use path helpers and adapter push/replace. Preserve queue filters/selection when navigating back from compact detail. |

Update canonical path/route tests in `packages/core/paths/{paths,consistency,route-icons,tab-subject,tab-presentation}.test.ts`, sidebar tests and desktop route wiring as applicable. Merely adding a router destination does not update the desktop tab registry or sidebar union types.

## Reusable split detail surface

`packages/views/inbox/components/inbox-page.tsx` is the closest layout reference: a resizable list/detail pair, compact list/detail switching, URL selection, scrolling, error boundary and next-item selection. Reuse the shape rather than copying notification semantics.

`IssueDetail` in `packages/views/issues/components/issue-detail.tsx:972` accepts:

```ts
issueId: string;
onDelete?: () => void;
onDone?: () => void;
defaultSidebarOpen?: boolean;
layoutId?: string;
highlightCommentId?: string;
highlightRequestToken?: number;
leadingAction?: ReactNode;
```

The inbox embeds it with `key={issueId}`, `defaultSidebarOpen={false}`, a unique `layoutId`, and a compact Back action through `leadingAction`. A wrapper must supply a definite flex height (`flex-1 min-h-0`), **not another overflow scroll container**: IssueDetail owns its scroller, virtualized timeline and restoration. It already supplies description editing, comments, attachments, activity history, properties, breadcrumbs and not-found behavior. Avoid implementing a second editable issue copy.

IssueDetail currently contains normal task actions and subscribes to `issueListOptions(wsId)` for navigation. It needs an admission-aware display/action policy on the issue itself so direct issue URLs also explain pending/rejected/duplicate state and suppress execution/status-completion affordances. Host-only triage props cannot protect a direct issue URL. A small slot for triage history/action footer may be appropriate; avoid requiring the monolithic detail component to own queue state. The queue should advance only after a confirmed successful decision; preserve selection, reason and accept-field drafts on failures/conflicts. Do not reuse inbox optimistic archival behavior.

## Settings and capability decisions

- Settings live at `/{slug}/settings?tab=triage`, not a new settings router page. Add the static nav item in `packages/views/settings/components/settings-nav.ts` (`SETTINGS_NAV_GROUPS`) and `<TabsContent value="triage">` in `settings-page.tsx`. Put it under workspace administration per the PRD; update the canonical group/order test.
- `settings-layout.tsx` exports `SettingsTab`, `SettingsSection`, `SettingsCard`, `SettingsRow` and `SettingsSaveState`. Use these for enable switch, source description, acceptance status, explicit priority requirement, responsibility mode and member selection.
- `useCurrentMember(wsId)` from `@multica/core/permissions` returns `{userId, role, member, isLoading}`. Owner/admin is the existing settings gate; see `workspace-tab.tsx`. `canUpdateWorkspaceSettings()` exists in `permissions/rules.ts` but is not currently exported from the public barrel. Do not infer access from merely being the configured triage owner.
- No triage capability, settings field or admission state currently exists in the inspected UI types. Use the dedicated settings/config API contract (with strict `enabled === true`) to drive navigation and actions. Keep historical browsing available when disabled; disabled is not equivalent to no history or missing backend capability.
- Public deployment flags use `useFeatureEnabled()` but workspace triage enablement is a workspace server setting. Do not add a global deployment flag as a substitute for it.
- Acceptance-status selector must restrict to active backlog/todo categories. Existing `StatusPicker` offers all active categories and has no filter prop, so extend it with a narrowly scoped option filter or use `useStatusOptions(wsId)` plus existing picker primitives. Do not expose invalid execution categories and rely solely on server rejection.
- There is no iteration UI to integrate in this T1 worktree; omit the selector as required when iteration capability is absent.

## Pickers, create and duplicate selection

| Component/helper | Reuse notes |
| --- | --- |
| `packages/views/issues/components/pickers/priority-picker.tsx` | Controlled `priority`/`onUpdate` field picker. Do not invent medium when none fails acceptance. |
| `.../assignee-picker.tsx` | Controlled `assigneeType`, `assigneeId`, `onUpdate(Partial<UpdateIssueRequest>)`; contains permission/runtime affordances for agents and squads. Use for candidate/execution assignee, not human triage owner. |
| `.../label-picker.tsx` | Has draft mode `selectedIds` + `onSelectedIdsChange` without issue mutation; use draft mode in atomic accept form. |
| `packages/views/projects/components/project-picker.tsx` | Controlled project picker with disabled/read-only support. |
| `.../pickers/start-date-picker.tsx`, `due-date-picker.tsx`; `packages/core/issues/date.ts` | Existing pure calendar-date rules. Snooze is an instant with timezone and must not reuse date-only parsing as if it were a timestamp. |
| `packages/views/modals/issue-picker-modal.tsx` | `open`, `onOpenChange`, `title`, `description`, `excludeIds`, optional `filterIssue(issue)`, async `onSelect`. Searches via `api.searchIssues({q, limit:20, include_closed:true, signal})`, debounces 300 ms, handles selection failure without premature close. Add formal-admission filtering and pasted issue-link resolution; current search alone is not a link resolver. Server must enforce same-space formal-target eligibility. |
| `packages/core/issues/batch.ts` | `commonIssueFields(issues)` handles genuine unassigned vs mixed priority/status/assignee. Reuse for selected-row defaults but do not reuse normal bulk issue mutation semantics for triage decisions. |

Manual creation currently lives in `packages/views/modals/create-issue.tsx`. `ManualCreatePanel` takes `data?: Record<string,unknown>`, consumes the shared issue draft, and sends `useCreateIssue().mutateAsync(...)`; `CreateIssueModal({onClose,data})` wraps it. It also previews run triggers and performs follow-up custom-property/label work. Triage needs an explicit creation mode and dedicated server route/validated input marker: suppress the run hint and execution-mode switch, keep ordinary creation unchanged, atomically send allowed candidate/label/attachment fields, and display a tracked pending result. Do not default through quick-create agent mode or allow persisted create-mode preference to turn an intentional triage submission into immediate execution. Existing source/attachment/editor validations should be reused.

## API, query and mutation architecture

- Add triage types in `packages/core/types/triage.ts`, a public type export, schema definitions, ApiClient methods, and `packages/core/triage/{queries,mutations,index}.ts`. `Issue` lives at `types/issue.ts:160`; `CreateIssueRequest` is at `types/api.ts:6`. Extend Issue with admission fields needed by all task surfaces and parse them defensively for older backends.
- ApiClient is `packages/core/api/client.ts`; schemas are plural `packages/core/api/schemas.ts`; the `parseWithFallback` helper is singular `api/schema.ts`. `getIssue()` demonstrates safe `fetch<unknown>`, nullable parse fallback and throwing on malformed success. A mutation may never turn a malformed 2xx into a fabricated successful decision/import. Include malformed-response tests.
- Model keys on `agentApprovalKeys` in `core/agent-approvals/queries.ts`: `all(wsId)`, settings, counts, queue(filters,sort,page), history(filters,page), detail(id), import(batchId). Every workspace key and every server list parameter must participate in the key. Counts must be independent of UI filters.
- Agent approval mutations are explicitly nonoptimistic because they create authorization decisions and conflicts must be visible (`core/agent-approvals/mutations.ts`). Apply the same policy to all triage actions; accept/reject/duplicate/reopen/snooze must await server confirmation before selection or count changes. New hooks should take explicit `wsId`.
- On success use authoritative response to update issue detail and triage detail, then invalidate queue/count/history and impacted issue/project projections. `core/issues/cache-coordinator.ts` and `issueKeys` (`core/issues/queries.ts`) are the canonical issue-cache entry points; do not manually insert triage items into ordinary list caches. A rejected/conflicted mutation should refetch current detail while retaining the form draft.
- Realtime integration belongs to `packages/core/realtime/use-realtime-sync.ts` and typed events in `types/events.ts`. Register triage event invalidation and reconnect invalidation for triage keys. Do not mirror queue rows to Zustand. A <=60s refetch or due-aware refresh is still required for snoozed rows; relying on user reload is insufficient for an open page.
- Accept-and-execute needs explicit response state distinguishing accepted/started from accepted/start-failed plus idempotent retry-start command. The UI must not rerun acceptance on the retry button.
- Batch preview and execution need per-row version/token identity and categorized outcomes. Preserve the original selected IDs, show issues before confirm, allow valid-only confirmation, and retain retry IDs only for failures. Do not expand selection to all matching rows.

## CSV import

No CSV parsing dependency or import facility was found in `packages/core` / `packages/views`; manifests do not declare Papa Parse or similar. The only issue CSV helper is `buildIssueTableCsv` in `packages/views/issues/components/table-view-model.ts` with escaping tests in the adjacent test file, used by `table-view.tsx` to download a UTF-8 BOM Blob through an anchor.

Recommended boundary: server owns decoding, RFC-style CSV parsing, row limits, mapping validation, workspace object resolution, external-ID duplicate policy, authoritative preview and idempotent per-row commit. Native Go `encoding/csv` can avoid new dependencies. Browser may read headers with a bounded pure parser if mapping UX requires it, but must not split by comma/newline (quoted cells and embedded newlines). Reject invalid UTF-8 before preview (e.g. TextDecoder fatal mode if decoding in browser); lock 5 MB / 1,000 data-row limits in design. No GBK support is necessary under current requirements if the error explains conversion.

UI needs file selection, mapping adjustment/ignore, row status preview, explicit retain-warning and override-duplicate selection, progress, and result counts. State/iteration columns must be reported ignored. Keep a stable batch identity for all retries. Failed-row CSV must preserve source columns and escaping; append useful failure reason and protect spreadsheet formula cells if introducing a new generic exporter. Preview has no issue/notification writes. Same-title candidates are informational, unlike external-ID duplicate default skips.

## Keyboard, accessibility and localization

- `@multica/core/shortcuts` exports `isEditableShortcutTarget()` and `isPortalLayerShortcutTarget()`. Apply both, also require queue focus. Global document key listeners without a focused queue check would violate FR-20 (in particular while typing in embedded IssueDetail).
- `J/K`, `1/2/3`, `H`, `C` must retain visible labeled buttons; rejection/duplicate shortcuts open required dialogs. Restore focus after successful decision to next row or the distinct empty-state heading. Compact controls need 44px targets.
- New namespace wiring: `packages/views/locales/en/triage.json`, `zh-Hans/triage.json`; register runtime imports in `locales/index.ts` and type imports/global `I18nResources` augmentation in `i18n/resources-types.ts`. Use `useT("triage")` selector API. Also add shared navigation strings in both `layout.json` and settings tab strings in both `settings.json`.
- Distinguish loading/error, disabled, empty queue, filters with no results and snoozed-only backlog. History must show the decision-time reason and changed fields from historical data, not a rendering of current issue fields labelled as past state.

## Verification and implementation ownership recommendation

Keep one frontend owner for API/core plus one view owner only after the transport/type contract is agreed; `api/client.ts`, `api/schemas.ts`, `types/index.ts`, realtime, sidebar and locale registries are shared collision points and should have explicit ownership.

Suggested frontend deliverable boundaries:

1. Types/schema/API/query/mutation contracts with boundary tests and cache/retry behavior.
2. Settings + shared routing/nav/count + enabled/read-only gates.
3. Queue/detail/actions/history + direct issue admission affordances + manual pending creation.
4. CSV mapping/preview/commit/results and batch actions, tested against stable backend contracts.
5. E2E integration/visual checks on Web plus desktop routing/shared view parity.

Existing regression references: `core/api/{schema,schemas}.test.ts`, `core/issues/{mutations,revision-realtime,cache-coordinator}.test.*`, `core/realtime/use-realtime-sync*.test.*`, `views/inbox/components/inbox-page.test.tsx`, `views/issues/components/issue-detail.test.tsx`, `views/modals/{create-issue,issue-picker-modal}.test.tsx`, `views/settings/components/{settings-nav,settings-page}.test.*`, `views/locales/parity.test.ts`, `e2e/issues.spec.ts`, `e2e/settings-integration-catalog.spec.ts`. Add canonical pure tests for mapping/date/version/CSV matrices in core, keep component tests focused on wiring, retained input/conflicts, compact Back/focus and selected-only batch behavior. Final tests must cover the full T1 acceptance set, not just the happy path or a disabled feature.
