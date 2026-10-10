# Triage duplicate target selection

## Scope and owners

`TriageDuplicatePicker({ wsId, workspaceSlug, excludeId, value, onChange })`
owns the shared Web/Desktop inline candidate list. `TriageActionDialog` retains
the target reference and reason, validates with `duplicateReference`, and sends
the existing triage decision only after explicit confirmation. Selecting a row
must never mark the source duplicate on its own.

## Signatures

- `triageDuplicateTargetsOptions(wsId, search)` returns core infinite query options.
- `api.getIssue(reference, { workspaceId, signal })` optionally pins an explicit
  workspace header and cancellation signal. Existing callers remain compatible.
- `api.listIssues({ workspace_id, q, limit, offset, sort_by, sort_direction })`
  supplies paginated browsing and title search; no backend API change is needed.

## Candidate contracts and identity

`triageDuplicateTargetsOptions(wsId, search)` in core owns an infinite React
Query under `triageKeys.duplicateTargets(wsId, search)`. Use `api.listIssues`
with explicit `workspace_id`, bounded `limit`/`offset`, and stable server sort.
Do not set `open_only` or `top_level_only`: terminal tasks and formal subtasks
are valid duplicate targets. The backend enforces formal admission and access;
the picker also excludes the source and rows outside its current workspace.

Compute the next offset from raw response windows and `total`, before local
exclusions. A filtered-empty page is not proof that no later candidates exist.
`issueListOptions` only loads the first status pages and must not be used as an
exhaustive candidate source. Preserve the load-more affordance for keyboard users
and cases where the first visible page is too short to scroll.

Workspace prefixes permit 1–10 alphanumeric characters, including a numeric
first character. Complete identifiers and UUIDs resolve through `getIssue` with
an explicit workspace header, returning one candidate or none. Do not reduce a
full identifier to its bare number for list search: that also matches title text
and produces unrelated candidates. Keep the original reference for submission.
Scope-check pasted issue links through `duplicateReference`, and accept only
valid identifier/UUID identities for submission: title search text is not a
selected task. A slash inside a title such as `Fix UI/UX` is not a task link.

## Interaction and recovery

Empty focus opens and fetches candidates. Typing debounces the effective query;
hide old results during that interval and keep cache keys distinct per workspace
and search. Clearing the input restores the unfiltered list. Selecting fills the
identifier and closes the list. Escape from any picker control closes the list
before the outer dialog; Tab moves out normally. Keep actual accessible names
nonempty: cmdk's generated labels can override `aria-label`, so supply Command
root and List labels. cmdk also hardcodes `aria-expanded`: a slotted native input
must expose the actual popup state. Isolate nested button key events from cmdk's
Enter selection and return focus to the input before a pagination/retry control
disables or disappears.

## Validation and error matrix

| State | Feedback |
|---|---|
| Pending initial request or debounce | Loading; no old selectable results |
| Successful empty unfiltered result | No eligible tasks in this workspace |
| Successful empty search | No matching tasks |
| Invalid or other-workspace link | Scoped-reference validation message |
| Exact reference returns HTTP 404 | No matching tasks; other errors remain errors |
| Initial/refetch failure | Error with retry |
| Next-page failure | Retain loaded rows, explain more failed, retry that page |
| More raw rows available | Scroll pagination and keyboard-reachable load more |

## Good, base and bad cases

- Good: focus empty input, browse all pages, choose a completed formal task,
  and explicitly confirm the duplicate decision with the retained reason.
- Base: type a title or paste a scoped identifier/UUID task link; select the
  result or submit the valid reference directly.
- Bad: submit arbitrary title text, show another workspace's cached result,
  hide a later raw page after local exclusions, or dismiss the dialog on picker Escape.

## Required regression surfaces

- `packages/core/triage/duplicate-targets.test.ts`: all pages, closed/subtask
  inclusion, query identity, numeric prefixes, exact lookup and 404 versus real errors.
- `packages/core/api/schema.test.ts`: `getIssue` explicit workspace header,
  cancellation and malformed-response behavior.
- `packages/views/triage/triage-ui.test.ts`: canonical reference parser matrix.
- `packages/views/triage/triage-duplicate-picker.test.tsx` and
  `triage-action-dialog.test.tsx`: actual primitives, focus/keyboard, search races,
  exclusion, errors, retry and explicit submit.
- `e2e/triage-duplicate-picker*.spec.ts`: real API windows and decisions in Web
  and native Electron, both locales and bounded narrow/wide screenshots.

## Wrong versus correct

```ts
// Wrong: a complete identifier becomes a broad numeric title search.
api.listIssues({ workspace_id: wsId, q: reference.split("-").at(-1) });

// Correct: pin identity resolution to the same workspace as the candidate cache.
api.getIssue(reference, { workspaceId: wsId, signal });
```
