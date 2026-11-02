# 协作来源任务点击浏览与搜索反馈修复

## Goal

Allow a user creating a collaboration issue to choose an optional parent in the source workspace without knowing its title or number beforehand, and make title/number searches visibly usable.

## Background

- The user reported that the field labelled "按标题或编号查找来源任务" looks like an isolated input and typing gives no visible response. They requested a task list when clicking the control.
- `packages/views/collaboration/field-inputs.tsx:92` currently separates the search input from a native parent select; results appear only in that select.
- Source-option reads already return the latest 200 eligible parents for an empty search, with `parents_truncated` for additional matches.
- `server/pkg/db/queries/collaboration_source_fields.sql` matches titles, bare numbers and UUIDs but does not match the complete identifier displayed to users.
- The user approved creating this task and continuing the repair. The requested click-to-browse behavior and title/identifier search resolve the product choices for this scope.

## Requirements

- R1: Explain the optional parent relationship in both supported locales and provide one discoverable task-selection control.
- R2: Clicking the control opens a visible list of the selected source workspace's recent eligible tasks without entering search text.
- R3: Typing a title, bare number or complete displayed identifier shows corresponding results. Clearing search or reopening restores browsing.
- R4: Loading, no tasks, no matches, truncation and failure each have visible feedback. A failure has a retry action. Pending or failed search must not permit choosing stale search results.
- R5: Selecting a result displays its identifier/title, retains its UUID for the existing parent relationship, and closes the list. Users can clear the optional parent.
- R6: Keep keyboard navigation, focus return, long titles and narrow layouts usable. Search and selection do not submit the surrounding form.
- R7: Use the same picker in creation and existing source-field editing; preserve source/account scoping, existing eligible-task rules and draft recovery.

## Acceptance Criteria

- [x] AC1 (R1/R2): Open a new collaboration form, expand initial fields and click the parent control; eligible task titles and identifiers are visible with no search input required.
- [x] AC2 (R3): Search by title, bare number and complete identifier; the intended source task is available, including identifiers using numeric prefixes and the legacy prefix fallback. A different workspace's prefix is not silently stripped.
- [x] AC3 (R3/R4): Clearing/reopening restores default browsing; loading, empty, filtered-empty, failed and truncated states are distinguishable.
- [x] AC4 (R4/R5): A delayed old query cannot be selected as the new search result. Selection and retry preserve the rest of the form and the selected parent.
- [x] AC5 (R5/R6): Mouse and keyboard selection set the same parent UUID without posting a creation/update request; clearing restores no parent and Escape returns focus.
- [x] AC6 (R6/R7): English/Chinese text and narrow/wide browser captures are usable; existing creation and source-field-editing regressions, lint and type checks pass.

## Constraints and Out of Scope

- Reuse the protected collaboration creation/field-options APIs, source workspace identity and current recent-200/truncation contract.
- No new dependencies, migrations, eligibility changes, execution/runtime changes, or unrelated cleanup.
- Preserve the pre-existing worktree changes and other sessions' task records.
