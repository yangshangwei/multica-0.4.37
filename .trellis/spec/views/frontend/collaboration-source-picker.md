# Collaboration source parent selection

## 1. Scope / trigger

Apply to the optional parent field in shared collaboration creation and source-field editing. `CollaborationSourceInputs` owns the selector; the existing creation/workflow callers own its protected source-option Query. Source authority and parent validation follow [the server contract](../../server/collaboration-source-editing.md).

## 2. Signatures

- `CollaborationSourceInputs` accepts the existing `options`, `value`, `current`, `parentSearch`, `onParentSearch` and `disabled`, plus optional `parentsPending`, `parentsError` and `onRetryParents`.
- `CollaborationSourceSelection.parentIssueId` remains a nullable UUID; search text is not a selected parent.
- `PropertyPicker.searchMaxLength?: number` forwards an optional HTML input bound. This selector passes 200; other callers keep their existing default.
- Creation and field-option endpoints accept `parent_search`; an empty query returns recent candidates, while `parents_truncated` identifies the bounded 200-row result.

## 3. Contracts

Use the existing searchable `PropertyPicker` and `PickerItem`, rather than a disconnected search input/native select. Opening immediately shows the current source's available rows. Closing resets search through the shared picker; clearing or reopening restores browsing. The trigger explains the optional parent relationship and retains a selected identifier/title when a later search excludes that row. A restored minimal reference or older response without an identifier still has a usable name.

Forward fetching/placeholder/error/retry state from the actual protected Query. Pending or failed searches expose no selectable old candidates. Distinguish no eligible tasks from no matching tasks, retain truthful recent-200 feedback, and focus the search input before retry controls disappear. Selection and clearing use explicit buttons, write only the parent UUID/null, and close the popup without submitting the containing form. Use the shared keyboard/IME/Escape behavior, wrapping text and 44px trigger/candidate/clear targets.

After a transient creation search failure, retain only existing Query data under the current Server/viewer/source key so the form and draft stay mounted. Never retain that data after `isCollaborationDenied`; the existing protected cleanup remains authoritative. Source-field editing can likewise retain its current source options for retry, but a failed Query must block source confirmation. Do not add a persisted source-catalog store or switch to generic ordinary-issue queries.

## 4. Validation / error matrix

| Condition | Required behavior |
| --- | --- |
| Open with empty search | Visible recent source task identifiers/titles |
| Current request pending or placeholder data | Loading; no old candidate can be selected |
| Empty successful unfiltered result | No available tasks in the source workspace |
| Empty successful search | No matching tasks |
| More than 200 matching rows | Bounded list and explicit refine-search explanation |
| Transient search failure | Local error/retry; retain selected parent and other form input |
| Authoritative source denial | No retained source catalogs or creation draft |
| Old search settles after a newer one | Its rows cannot replace the current query's results |
| Candidate selection / clear / picker Escape | UUID/null or dismissal only; no creation/update POST |

## 5. Good / base / bad cases

- Good: open the selector without knowing a task title, choose a recent task, then explicitly create its collaboration child.
- Base: search by title, number or displayed identifier; clear/reopen to browse again.
- Bad: show search results only inside a closed native select, select stale placeholder rows, or unmount the whole form after a transient search failure.

## 6. Tests required

- `packages/views/collaboration/field-inputs.test.tsx`: actual primitives, open-without-query, selection/clear, query feedback, retry focus, nested Escape/IME/Enter, Chinese explanation and retained minimal names.
- `packages/views/collaboration/creation-form.test.tsx`: actual source Query wiring, delayed old-query exclusion, mounted draft recovery and authoritative-denial cleanup.
- `packages/views/collaboration/field-editing.test.tsx`: the existing separate source/iteration actions and initial-field UUID submission.
- `packages/views/issues/components/pickers/property-picker.test.tsx` and locale parity: shared keyboard behavior and both supported locale keys.
- Server source-field tests own the complete-identifier and recent-window matrix; Core field/API/schema tests own protected read and compatibility boundaries.
- Production browser evidence separately verifies actual parent-option requests and persisted `parent_issue_id`, both locales, wide/narrow bounds and transient retry. Shared package tests do not establish installed Electron acceptance.

## 7. Wrong versus correct

Wrong: couple `onParentSearch` only to a native select that must be opened separately, or use the selected workspace's ordinary list query as a second catalog owner.

Correct: display the existing protected source options in the shared searchable picker and forward that Query's pending/error/retry state. Keep only the selected parent identity as user intent.
