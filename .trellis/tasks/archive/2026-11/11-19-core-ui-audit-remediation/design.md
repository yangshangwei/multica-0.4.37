# Design: Core UI audit remediation

## Approach

Repair behavior at shared component boundaries so Web and Electron inherit the same contract. Use existing Query state instead of introducing duplicate fetches, reuse `ReadonlyContent`, Base UI picker semantics, existing focus/inert patterns, and semantic design tokens. Keep local draft state separate from the last server snapshot and only adopt remote values while clean.

## Boundaries

- `packages/views/layout/animated-right-sidebar.tsx`: hidden subtree semantics and reduced-motion hook.
- `packages/views/issues/components/pickers/*`: listbox/option state, query recovery, stable labels.
- `packages/views/issues/components/issue-detail.tsx`, `comment-input.tsx`, `reply-input.tsx`: names and disclosure relationships.
- `packages/views/squads/*`: draft/server reconciliation, loading/error states, readonly instructions, tabs and multi-select semantics.
- `packages/ui/styles/tokens.css`, `packages/ui/components/ui/button.tsx`, collection header: contrast and touch targets.

## Compatibility

No API shape changes. Existing query keys and mutation flows remain authoritative. Query errors preserve cached data. Existing localized strings are reused where possible; add only required labels if no suitable key exists.

## Verification

Write focused failing tests before each behavior change, run them red, implement the smallest fix, then run the affected suites. Finish with package typecheck/lint and the existing contrast/brand tests. Manual desktop AX verification will re-check the hidden sidebar, named editors, picker state, and quick-create parity.

## Evidence-driven boundary extensions

- Portal contents render outside the sidebar DOM. A default-visible, UI-only popup visibility scope is permitted for Popover/Menu/HoverCard/Dialog/Tooltip wrappers, used only by the retained sidebar. Hiding must close/reset only owned popups without changing ordinary callers. The project detail caller may pass its toggle focus ref and description visibility; issue detail caller wiring is coordinated separately.
- Existing identical brand-filled filter sites in project list, autopilot list and inbox filters are included in the token pairing repair. Forced-light landing overrides must retain a coherent light palette.
- Squad detail/members options may move into Core workspace query factories with exact-query 401/403/404 cache boundaries and cancelled-request safety. No network schema or global API behavior changes.

## Review followups

Independent review confirmed two additional existing popup consumers inside project sidebar: squad-manager Sheet and knowledge-policy Select. They are included in the same visibility scope; Select has a different native imperative API and must preserve a stable open-state mode without changing its value ownership. Review also applies the existing401/403 exact-query boundary to squadListOptions so assignee retry cannot revive denied squads, and clears pending squad archive confirmation when management authority is lost.
