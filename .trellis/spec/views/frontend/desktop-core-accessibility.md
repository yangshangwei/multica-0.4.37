# Desktop core interaction and recovery contracts

## Scope / trigger

Use these contracts when changing the floating chat, collection selection,
collection recovery, detail Tabs, desktop shell motion/reordering, or Gantt.
Web/Desktop share business views; desktop tab state and shell geometry remain
in the desktop app. Preserve editor, upload and route identity when hiding UI.

## Signatures and owners

- `ContentEditorProps.isVisible?: boolean` defaults to true. Floating
  `ChatInput` forwards its actual visibility. This flag controls formatting
  and link hover overlays; it does not recreate the editor or document.
- `useLinkHover(containerRef: React.RefObject<HTMLElement | null>, disabled?: boolean)`
  immediately returns invisible state while disabled and resets the retained
  anchor, listeners and timers.
- `useChatInputFocus(isOpen, windowRef, isVisible)` owns the composer nonce,
  focus capture and closing focus handoff. Visibility and focus ownership
  must agree with the actual mounted DOM, including portals.
- Collection recovery uses the existing Query's `data`, `isError`,
  `isFetching` and `refetch()`. No extra view-local API call or store is needed.
- Desktop reorder uses existing `moveTab(from, to)` and store pin/workspace
  rules. Menu commands and `Alt+Shift+ArrowLeft/Right` are equivalent entries.
- Shell geometry shares a Motion value for header padding/drag-region left;
  canvas margin has its own value. `jump()` cancels an active spring under
  `(prefers-reduced-motion: reduce)`; changing duration alone cannot cancel it.

## Contracts

Closed or unmeasured chat is inert and absent from the accessibility tree.
Its editor, session, pending uploads and unsent draft remain mounted. Header,
project/add/queue menus and editor body portals cannot stay interactive outside
that inert subtree. Reopening must not revive a stale hovered link or popup.

Restore focus only if chat owned it at closing. A deliberate move to the page
or explicit composer blur to body must not let an old focus record claim it.
Native removal of a focused header button can emit focusout **before** React
applies inert on the retained parent. An inert-only event guard is insufficient;
retain the close handoff through that mutation order. Mounting a persisted
open chat must not steal focus from the page.

Use one named shared Checkbox per selection action. Row names identify the
resource; scope names describe the actually listed/loaded/filtered collection.
Reveal hover controls on keyboard focus and preserve a selection column in
compact grids and their skeletons. Agent batch actions use a normal-flow compact footer with shared chat clearance; its count cannot shrink into character wrapping. Preserve enough scrollable list height for the header, group and one existing 64px row; the compact page may scroll vertically. Wide layouts keep the centered floating toolbar with explicit intrinsic width. Isolate the containing cell's click and
auxclick: Base UI can emit a click from its sibling hidden input.

Pointer-clickable rows also expose an `AppLink` on the title for Tab/Enter.
Apply `rowLinkInteractiveProps` to the nested link/interactive cell so activation
does not navigate twice or cancel native modified-link behavior.

Use shared Base UI Tabs for roving focus, orientation and panel association.
Agent/skill editors use manual activation: arrows/Home/End move focus;
Enter/Space activate. Keep existing draft confirmation and route parameters.
Desktop destinations are navigation buttons with exactly one `aria-current`;
do not apply incomplete ARIA tab roles to that shell.

Gantt keeps full scroll extent while generating dates only for the visible
window plus overscan. Virtual rows use stable issue IDs and expose list position.
Keyboard traversal must mount and focus the destination across virtual borders;
a later toolbar focus supersedes pending row focus. Coarse zoom cannot allocate
the full daily DOM. Category-based bars pair existing semantic fills with
`gantt-bar-foreground`/`gantt-warning-foreground`, verified in both themes.

## Validation and error matrix

| Situation | Required behavior |
|---|---|
| Query pending, no data | Loading; never claim a known empty collection |
| 500/network error, no data | Named alert and Retry; no false empty/not-found state |
| Transient error with authorized cache | Keep rows, selection, filters and editor nodes; show refresh alert/Retry |
| Successful empty response | Genuine empty-state copy and action |
| Detail 404 | Not-found distinct from transient load failure |
| Definitive 403/deletion | Existing protected-cache/draft guard wins over cached recovery |
| Inbox read fails | Do not redirect a selected missing notification before a successful read |
| Retry in flight | Disable repeat Retry without discarding retained work |
| Chat hidden | No keyboard/AX reachability, including body portals |
| Reduced motion turns on mid-spring | All geometry owners snap on the next frame |
| Reorder reaches pin/group edge | No crossing; menu boundary disabled and shortcut no-op |

## Core page audit fixes

- Task identifier resolution distinguishes transient failures from definitive
  401/403/404 responses. UUID and identifier caches must obey the same access
  decision; a retry cannot replace a retained editor with a skeleton.
- Chat sessions and messages expose separate loading/error/retry states in both
  the full page and floating window. Query timestamps keep cold retry controls
  mounted while Query returns to pending. Failed reads do not clear the active
  session, composer draft or authorized history. Triage queue, history and
  settings follow the same retained-content recovery contract.
- Project search remains reachable at 450 CSS px and 200% native zoom. Clearing
  search restores input focus. Every icon-only action has a localized name;
  row actions reveal on keyboard focus, and the task properties Sheet is named.
- Status fills and text colors are independent: use warning/info/success
  foreground tokens for small status text. Measure both themes and selected/
  hover backgrounds; do not reduce the opacity of an entire text badge.
- Manual task ordering exposes adjacent up/down actions on list, board and
  swimlane surfaces. Actions use the same rank/move anchors as dragging, honor
  sort direction, and stop before an unknown pagination boundary. Sorted views
  do not offer manual order actions. Lane moves preserve hidden preference
  entries and cannot cross pinned/orphan boundaries. Announce settlement and
  preserve focus through virtual row remounts without reclaiming toolbar focus.
- Compact scope selection exposes saved views and invokes the same builtin
  actions as the wide ViewBar, including clearing the active saved view.
- Project batch deletion awaits every outcome, prevents duplicate submission,
  removes successful IDs from selection, and retains failed selections with a
  localized summary. Never close the confirmation or clear selection on dispatch.

### Core audit recovery and keyboard details

- Filter inputs inside Base UI menus isolate text-editing keys from menu
  typeahead. Preserve Escape/Tab outside composition; IME candidate keys and
  composition Enter must neither activate a menu item nor commit a scalar filter.
- `labelChipColors(hex)` normalizes optional-`#` six-digit colors and chooses a
  foreground with WCAG relative-luminance contrast >=4.5. Malformed colors need
  an opaque fallback fill as well as a readable foreground. The former 0.55
  perceived-brightness threshold is not a contrast guarantee.
- Inbox uses a named semantic list and native selection buttons marked with
  `aria-current`; avatar/menu/archive controls remain independent siblings.
  Arrow navigation focuses the actual target button. At a virtual boundary,
  park focus on the list only until that target mounts, and cancel the handoff
  when another control acquires focus. Expose row position and total count.
- Project disclosures have stable `aria-controls`/`aria-expanded`; retaining
  the description editor while collapsed must also forward `isVisible=false`
  so formatting/link portals close. Compact project Sheets have a SheetTitle.
- Custom status ordering uses the existing adjacent-order controls and retains
  focus through pending/boundary states. Disable dragging and adjacent ordering
  while archived rows are displayed, and explain how to hide them: the actual
  mutation normalizes every active position to 1..n, so a local slice check
  cannot guarantee the visual order around unchanged archived positions.
- Gantt recovery notices precede the shared empty/content split, including
  `isScopeEmpty=true`. Notification preferences and both runtime usage queries
  distinguish initial loading/failure, successful empty data, and cached refresh
  failure. Query timestamps keep Retry mounted while a cold retry is pending;
  pending retries disable repeat submission and cached refreshes retain content.
- `projectGanttIssuesOptions(wsId, projectId, assigneeTypes)` erases the exact
  cached query snapshot on 401/403/404, not merely the current rendered array.
  A subsequent 500 or remount cannot restore denied rows; a successful fetch
  can supply new data. Capture that Query before awaiting the request and check
  `signal.aborted` before clearing: a cancelled older request must not erase a
  newer successful result on the same Query object.

Additional validation cases:

| Situation | Required behavior |
|---|---|
| Cached Gantt success -> 403 -> Retry -> 500 | No denied task rows reappear |
| Remount after access loss, new fetch pending | No previous snapshot is shown |
| Cancelled Gantt request later rejects 403 | New successful snapshot survives |
| Empty project with cached empty schedule, refresh fails | Retry notice remains reachable |
| Archived status rows are shown | All ordering paths disabled with explanation |
| Inbox arrow crosses the virtual mount boundary | Named target receives focus/current state |

Regression owners: `issues-header.search-filter.test.tsx`,
`issues-header.property-filter.test.tsx`, `labels/label-chip.test.ts`,
`inbox-list.test.tsx`, `inbox-list-item.test.tsx`, `project-detail.test.tsx`,
`issue-statuses-ordering.test.tsx`, `notifications-tab.test.tsx`,
`usage-section-recovery.test.tsx`, `issue-surface.test.tsx`, and
`packages/core/issues/queries.test.ts`. Native evidence:
`.impeccable/audit/2026-10-13-core/remediation-native.json`.

Canonical regressions: `canonical-id.test.tsx`, task detail/route suites,
`triage-page-recovery.test.tsx`, chat window/page suites, project list/detail
suites, `adjacent-order.test.ts`, `reorder-controls.test.tsx`, compact selector,
list/board/swimlane and surface controller suites. Native evidence lives in
`.impeccable/audit/2026-10-11-desktop-core/`.

## Good / base / bad cases

- Good: cached project refresh fails, the same filtered row/checkbox/editor
  remain mounted and a localized retry alert appears.
- Base: successful empty inbox remains an empty inbox.
- Bad: `data ?? []` is treated as an authoritative empty list after 500.
- Good: 1000 Gantt rows across 2020–2030 retain bounded DOM and Home/End works.
- Bad: month zoom still renders two daily arrays for the whole decade.
- Good: an invisible chat cannot leave a link-hover card with focusable buttons
  in document.body, and its unsent editor remains the same node.

## Canonical tests and native assertions

- `chat/components/use-chat-input-focus.test.ts`: transition/ownership matrix,
  explicit body blur, inert blur and native header-removal ordering.
- `chat/components/chat-window.test.tsx`, `chat-queue.test.tsx`: retained editor,
  history/add/project/queue/formatting popup closure and draft continuity.
- `editor/content-editor.test.tsx`: actual hover hook/body portal visibility,
  default callers unchanged, editor identity, hidden and reopened state.
- `issues/components/list-view.test.tsx`, `agents/components/agents-page.test.tsx`,
  `projects/components/projects-page.test.tsx`: named mixed selection, keyboard
  paths, filtered range and singular navigation. Browser CSS proves compact
  reachability; jsdom class names cannot establish it.
- `inbox/components/inbox-page-recovery.test.tsx`, project list/detail suites:
  real QueryClient/API retry, retained work and definitive-access cleanup.
- Agent overview and skill detail/file-tree suites: manual activation, panel
  IDs, nested orientation and unsaved editor/navigation behavior.
- Desktop `tab-bar.accessibility.test.tsx` owns command wiring/focus/current;
  tab-store tests own pin/group state boundaries. `desktop-layout.test.tsx`
  owns startup/live reduced motion and interruption of an already active spring.
- `issues/components/gantt-view.test.tsx`: long date range, real virtualizer,
  stable/far rows, zoom endpoints and superseded pending focus.
- Real Electron acceptance must separately prove inert/AX exclusion, removal
  focus order, keyboard reveal, effective 450px selection, enabled pagination
  hit testing, measured foreground/background contrast (>=4.5), native zoom and
  extreme scroll/row geometry. Synthetic renderer cache fixtures establish
  layout behavior, not API parsing or throughput.

## Wrong vs correct

```tsx
// Wrong: a failed initial request becomes an empty collection.
const { data: projects = [] } = useQuery(projectListOptions(wsId));
return projects.length === 0 ? <Empty /> : <Rows projects={projects} />;

// Correct: data presence distinguishes first failure from recoverable cache.
const query = useQuery(projectListOptions(wsId));
if (query.isError && query.data === undefined) return <LoadFailure onRetry={query.refetch} />;
return <RetainedRows projects={query.data ?? []} refreshError={query.isError} />;
```

Keep the recovery notice outside retained row/editor identity; don't key it by
revision/error state. Use existing collection primitives rather than inventing
the illustrative components above.
