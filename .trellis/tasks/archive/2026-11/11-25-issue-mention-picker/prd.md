# Issue-detail mention discovery

## Goal

Help users find the intended member, agent or AI squad from the issue-detail comment composer when the workspace contains many agents, using familiar discovery patterns from the issue-creation picker.

## Planning authority

The user authorized creating this Trellis task and entering planning on 2026-11-25, then approved the final plan for implementation with "按照计划推进". The user's screenshot is evidence of the existing interface, not a target design.

## Background and evidence

The current popup combines members, agents and squads under Users (`packages/views/editor/extensions/mention-suggestion.tsx:115`) and truncates the merged list to 20 (`packages/views/editor/extensions/mention-suggestion.tsx:353`). Its agent/squad search matches names, not saved responsibilities (`packages/views/editor/extensions/mention-suggestion.tsx:744`). This makes a large directory hard to browse and similarly named agents hard to distinguish.

The reference picker already builds saved-description/leadership metadata, ranks full-description and name/pinyin matches, and pages the full directory (`packages/views/modals/quick-create-actor-picker-model.ts:20`, `packages/views/modals/quick-create-actor-picker-model.ts:56`, `packages/views/modals/quick-create-actor-picker-model.ts:74`). Its shared contract is `.trellis/spec/views/frontend/quick-create-actor-picker.md`.

Baseline on 2026-11-25: 51 existing tests passed in the mention suggestion and actor discovery model suites. This validates the starting behavior, not the proposed feature. Details and command are in `research/current-behavior.md`.

## Scope

Cover the shared Web/Desktop issue-detail new-comment, reply and existing-comment/reply editing surfaces. The target is the composer in the supplied screenshot. Both normal issue pages and issue detail windows use the shared components.

Keep other editor hosts on their existing presentation, including Chat's current/recent context mode, task descriptions and the task-creation editor. The issue-creation picker may share pure discovery helpers but retains its existing UX and preference semantics.

## Requirements

- **R1 — Categories.** Keep a visible filter row: All / Agents / AI squads / Members / Issues. Agents includes individual agents that lead squads; within Agents offer All agents / Squad leaders. Leadership comes from the saved squad leader relationship, never the agent's name. The active category remains visible while scrolling. All members is a separate existing mention target, available in All and Members only; it is not the All category.
- **R2 — Search.** Continue typing after @ in the original editor. Combine category and query over the entire eligible local directory before limiting rendered rows. Agents/squads match exact name, name substring, name pinyin/initials, then their own full saved-description substring. Preserve existing member-name/pinyin and issue identifier/title search. Entering a query searches beyond favorite/recent shortcuts. Display a localized hint that typing after @ searches names or responsibilities.
- **R3 — Recognition.** Show avatar, name, type and a short inert preview of saved responsibilities. Identify squad leaders and the squads they lead. Long names/responsibilities remain available through accessible full text or a focus/hover disclosure. Missing descriptions get a neutral localized fallback; never invent capabilities. Preserve unavailable-runtime explanations.
- **R4 — Shortcuts and complete browsing.** With an empty query, surface up to 3 existing task-creation favorites and up to 5 recent mentions, then category groups with deduplicated remaining candidates. Read favorites only after scoped hydration; use mention recency rather than task-creation history. Show 50 directory rows per batch with an explicit Show more action and remaining local count. Every eligible agent/squad/member remains browsable, including when no shortcuts exist. Issue server-search limits remain separate and must not be described as a complete workspace issue count.
- **R5 — Input continuity and selection.** Changing category, expanding results or retrying must preserve the editor selection and draft. Selecting a row inserts exactly that mention and returns to typing. Preserve Arrow/Ctrl+N/J/P/K navigation, Enter/plain-Tab acceptance, modified-Tab behavior and IME ownership. While the categorized popup is open, Alt/Option+ArrowDown enters its filter controls without accepting a row; expose this shortcut in the popup hint. A first non-composition Escape dismisses only the popup. New controls have accessible names, pressed state and keyboard operation.
- **R6 — Authority and data readiness.** Preserve mention-specific permission/archive/runtime rules. Favorites and search cannot reveal forbidden agents or bypass disabled reasons. Loading and failed data are distinguished from successful empty results; retry retains usable cached candidates. Workspace/account changes discard stale popup data and selections. Choosing a mention does not send a comment, start execution, change assignment/defaults, or mutate creation favorites/history.
- **R7 — Consistency.** The behavior and localized Chinese/English copy are shared by Web and Desktop. Keep the popup inside the viewport, readable at 375px, and usable in both themes. Existing non-opted-in mention hosts and mention Markdown/node serialization remain compatible.

## Acceptance Criteria

- [ ] **AC1 (R1/R4):** With 550 synthetic agents/squads, category filters show only matching types; successive Show more actions reach the final matching local candidate without losing the active filter.
- [ ] **AC2 (R2):** An agent outside the first page matches by name, pinyin/initials and a responsibility substring beyond the displayed preview. Search covers the full directory and intersects the category.
- [ ] **AC3 (R3):** Two similarly named agents can be distinguished by their saved responsibility/leadership text; full long labels are accessible, and absent descriptions do not break layout.
- [ ] **AC4 (R4/R6):** Favorite and recent rows are deduplicated against directory groups; stale or unauthorized saved refs never appear. Mentioning does not change the quick-create default, favorites or recent history.
- [ ] **AC5 (R5):** After filtering, clearing a query, expanding a page or receiving asynchronous results, the highlighted identity equals the inserted identity. Choosing an unavailable candidate cannot insert it; picking any mention cannot submit the draft.
- [ ] **AC6 (R5):** Chinese IME composition (including IME-owned Escape), Enter/Tab acceptance, modifier-Tab, popup dismissal and draft/caret retention pass real-editor interaction checks. Alt/Option+ArrowDown reaches filters; keyboard navigation reaches Retry/Show more, activation preserves the query and resumes editing, and Escape clears suggestion state without closing the host.
- [ ] **AC7 (R6):** Cold loading/failure, cached refresh failure, late data and workspace/account changes show truthful feedback and never insert stale or forbidden candidates.
- [ ] **AC8 (R7):** Comments, replies and edits work on Web and Electron; 375px and desktop, light/dark, Chinese/English checks show no overflow. The existing default/context mention and serialization tests remain green.

## Out of scope

- New agent taxonomies, editable tags, AI recommendations, runtime-online filters or changes to execution eligibility.
- New favorite management in the mention popup, default-assignee actions, project recommendations or cross-device preference synchronization.
- Changes to mobile, Chat context search, task-description editors, backend APIs, database schema or mention serialization.
- A complete paginated workspace issue search; this task removes the discovery cap for the local agent/squad/member directory.

## Review status

The categories, shortcut reuse and comment/reply/edit scope were approved in the final planning review on 2026-11-25. Implementation may proceed. Material scope changes require updating the planning artifacts and a new review.
