# Desktop core audit fixes design

## Authority and scope

The audit and the user's creation/development instruction define one repair deliverable covering DCF-01–10. Preserve the incumbent Operate UI. Current dirty source already implements much of the repair; validate it rather than replacing it. Save starting files, hashes and git state before new edits. Independent lanes use this baseline and named requirements so unrelated work is not interpreted as this task's new change.

## Ownership

| Lane | Owned files | Responsibilities |
|---|---|---|
| Chat/list/detail accessibility | views/chat/components/{chat-window,chat-fab,chat-input,chat-queue,use-chat-input-focus}; views/issues/components/{list-row,list-view}; views/agents/components/{agents-page,agent-overview-pane,agent-batch-toolbar}; views/skills/components/skill-detail-page; adjacent tests and agents/issues/skills locale keys; editor/{content-editor,link-hover-card} visibility boundary and tests | DCF-01, task/agent DCF-03, detail DCF-08. Retain editor/upload ownership and route behavior. |
| Collection/project recovery | views/inbox/components/inbox-page; views/projects/components/{projects-page,project-detail}; adjacent tests and inbox/projects locale keys | DCF-02, project DCF-03, DCF-04. Reuse Query status/refetch, collection state, Checkbox and AppLink. |
| Desktop/timeline/layout | desktop renderer components/{tab-bar,desktop-layout}; views/triage/triage-page; views/issues/components/gantt-view; ui/styles/tokens.css; adjacent tests and desktop/issues locale keys | DCF-05–07, desktop DCF-08, DCF-09–10. Preserve pin/group invariants and range geometry. |
| Main coordinator | Task artifacts, evidence runner, audit follow-up, specs, navigation documentation, integration and commit selection | Real Electron evidence, final verification and ownership review. |

Edit locale keys in targeted subtrees, not whole-file rewrites. Issues keys are shared between two lanes: communicate before touching the other's keys. The starting global Dialog keepMounted change belongs to another task and is not required by this repair; do not modify or stage it.

## Contracts

### Hidden chat and focus

Use one visible condition from isOpen and bounds/expanded state for inert, aria-hidden, pointer interaction, initial focus and transient portalled controls. Keep editor/session mounted. React focus capture observes portalled descendants; restore only while chat owns focus. Resolve a connected opener or live remounted launcher. Persisted open state at mount must not steal focus. Real primitives and browser inert verification supplement jsdom.

### Query recovery

Query owns server data. An error without data is a failed collection, not []. An error with authorized cache is a refresh notice/retry above retained content. Inbox cannot redirect a missing selection until its active collection succeeds. Project 404 is not-found; transient 500 is load/refresh failure. Preserve existing access-denial/deletion protection. Persistent editors/dialogs must survive refresh errors.

### Selection, links and tabs

Use shared Checkbox for mixed state, naming and keyboard toggles. Hover reveal needs focus-visible/focus-within equivalents, never display:none. Name a row by its resource and select-all by its scope. Project titles use AppLink plus rowLinkInteractiveProps for singular navigation and platform modified-click behavior. Base UI Tabs owns roving focus/panel IDs; preserve manual activation, routes and mounted-content needs. Desktop tabs remain navigation buttons with aria-current and explicit reorder menu/shortcut commands.

### Gantt and desktop geometry

Gantt-specific foreground tokens avoid altering global status colors. Measure computed RGB and composite alpha over the real surface for every category fill, including neutral/translucent fills. Keep status icon/title alternatives.

Generate horizontal marks arithmetically only for viewport indices plus overscan; retain full-range scroll width. Existing TanStack Virtual windows rows. Stable issue IDs and keyboard index own traversal; pending focus resolves after a requested row mounts. Test ten-year endpoints in each zoom and traversal beyond overscan.

Reorder calls existing moveTab within pin/group constraints. Support menu click and keyboard shortcut with retained focus. Reduced motion switches the existing geometry owner to immediate values, including media-query changes. Triage uses --chat-launcher-clearance through pe-chat-launcher and wraps controls at narrow effective widths.

## Verification and rollback

Existing regressions establish the behavior baseline before new corrections. Add tests only for real behavioral gaps. Run lane checks, workspace typecheck and broader relevant tests after integration. Use isolated real Electron preload/renderer, separate profile and read-only operations; inject failures in browser responses and synthetic layout rows in the isolated renderer Query cache. Batch screenshot inspection once, record independent visual verdict, permit at most one repair-confirmation batch.

No schema/dependency migration. Rollback only selected task hunks; mixed files need hunk-level review before staging. Preserve the original time-specific audit and write a separate repair follow-up.
