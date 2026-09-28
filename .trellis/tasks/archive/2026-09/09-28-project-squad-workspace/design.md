# Project squad workspace design

## Product intent and selected direction

Operate mode for people managing a project. The initial focus is the next issue or current issue work. The approved review establishes the existing dark/light token system and task views as visual authority.

Use a compact project squad summary and an on-demand right-side sheet. Compared with expanding accordions or a card grid, the sheet bounds the main canvas height at every squad count and preserves task position while users configure candidates. Existing project properties remain in their current sidebar. No new visual theme or animation system is introduced.

## Main canvas

The breadcrumb remains followed by a compact summary containing an icon, squad count, readiness information and the default's name, plus Manage squads. Names truncate within the available width with full accessible text. Common successful state uses quiet text. Unknown/check failures are distinct, and an unavailable default has a visible explanation/management entry.

The issue surface occupies the remaining height. A populated project gets one primary New issue action in the existing toolbar. A truly empty, unfiltered project replaces unnecessary toolbar/content with a centered first-issue message, New issue and the secondary Link existing issue action. Creation always calls the surface controller so saved-view, grouping and explicit-assignee precedence remains intact.

## Squad management

The Sheet reuses the UI package's dialog, scrolling and focus behavior. Its header explains candidates and the future-issue default. Rows show name, actual description if present, current availability and an explicit New issue default marker. Name opens squad details. A labeled overflow menu provides use-for-new-issue, set-default, change, runtime/details and remove actions. Setup/connection failures retain direct recovery controls and messages. Editing uses the current ProjectSquadPicker.

The normal main view has no per-squad dispatch controls. The manager can open ordinary issue creation with project/squad/todo after full readiness checks, closing the sheet first. Loading, errors and paused prerequisite queries block dispatch. A permanently missing roster must not prevent replacing/removing its configured choice. All configuration writes share one pending guard.

## State and data

Keep server-owned data in React Query. Consolidate the current shared membership/agent/squad/runtime/resource reads and observe one complete roster per configured squad. Derive each row and the summary from the same getProjectSquadReadiness function; do not mirror child readiness into local or Zustand state. Reuse projectSquadSelection and the existing configure mutation. Local state covers sheet visibility and actual editing/pending interaction only.

The first candidate stays the default, changed by explicit ordered-list mutation. No new API or persisted preference is needed. Do not reorder candidates for display by status.

## Empty-state integration

IssueSurface gains an optional authoritative isScopeEmpty signal, passed from project.issue_count === 0, and a headerActions render slot. IssuesHeader gains a small actions slot. Existing default header wiring remains in IssueSurface. Error/loading states precede emptiness; active filters precede custom empty content. Header suppression applies only to a true empty default project with no actor filter, active saved view or user filters. Table/Gantt continue to own their usual data flow for populated scopes.

Use the existing IssuePickerModal with an awaited selection contract. During association disable duplicate selection; close only on success. A failed write leaves a retry path and an inline/toast explanation. Prevent silent reassignment from another project by limiting selection to unassigned/currently permitted candidates or explicitly confirming that move; prefer unassigned candidates for this initial flow. Never alter issue assignee or status.

## Scope boundary and simplification

Remove duplicated always-visible runtime/helper paragraphs and repeated main dispatch buttons. Move current functionality into the manager without dropping authorization or recovery. Avoid a new management store, generic dashboard framework, polling layer, or automatic task routing. Project attention metrics are separately deferred until their real data contract is defined.

## Accessibility and responsiveness

Use existing Sheet/Menu/Button semantics and localized accessible labels. Menu actions stay keyboard reachable. Focus returns to Manage squads on dismissal. The sheet and task canvas use min-height/overflow deliberately. At narrow sizes the summary wraps within two normal lines and management remains scrollable; anomalous explanation may add a short line. Do not hide recovery just to meet a fixed height.
