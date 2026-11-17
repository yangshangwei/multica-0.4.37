# Collaboration audit remediation design

## Approach and alternatives

Repair each defect at its current owner and reuse the existing picker, Query,
dialog, localization and client draft machinery. This keeps Web/Desktop behavior
aligned and gives every change a canonical test boundary.

Two alternatives were rejected:

1. Adding collaboration-specific duplicate property pickers would localize the
   diff but leave the shared defect in ordinary issue fields and create a second
   keyboard/search contract.
2. A broad picker/modal/query refactor or a global polling change would enlarge
   the regression surface without being necessary for C01–C09. Change optional
   behavior only where the current contract needs it.

## Boundaries and ownership

| Surface | Expected files | Required preservation |
| --- | --- | --- |
| Shared property choices | `views/issues/components/pickers/property-picker.tsx`, `custom-property-picker.tsx` and component suites | Existing `optionId` listbox relations, empty-value keyboard rules, IME, commit behavior and archived/unknown-value handling |
| Collaboration initial fields | `views/collaboration/field-inputs.tsx`, `creation-form.tsx`, their suites and both collaboration locales | Source/recipient/file/draft identity and current server catalog validation |
| Knowledge review and visibility | `views/collaboration/knowledge-review.tsx`, `knowledge-panel.tsx`, `knowledge-proposal.tsx`, `knowledge-source-picker.tsx`, `detail-auxiliary.tsx`, `core/collaboration/knowledge-queries.ts` and suites | Exact tuple, private Query-only projections, current authority, lease erasure and input isolation |
| Context history and confirmation | `views/collaboration/memory-panel.tsx` and suite | Root state for toolbar feedback, saved editor, selected version, snapshot-pinned purge/restore and explicit receipt recovery |
| Workflow read recovery | `views/collaboration/workflow.tsx` and suite | Same form instance, current status options/version and current action/permission guards |
| Evidence and specs | Existing audit fixture/report and focused `.trellis/spec/` docs | Separate component/mock evidence from live production/runtime acceptance |

No UI package business imports, app/platform routes, stores, new dependencies,
backend API fields, migrations or runtime operations are introduced.

## C01/C02/C03/C09: existing picker contract

Search belongs to the property-specific caller because `PropertyPicker` is a
shell whose children may contain groups, virtual choices or asynchronous rows.
Use its existing `onSearchChange` and reset-on-reopen behavior and filter the
current configured choices. Keep the fixed empty row distinct from a genuine
search match. Use the existing localized empty-state primitive for no matches.

Expose multi-selection as explicit toggle-button state when no listbox option
role exists. Preserve current `optionId`/`aria-selected` semantics. Clear and
other command rows must not acquire misleading pressed state. Apply the same
state to available collaboration labels and custom multi-select choices.
Unavailable selected values remain one-way removal commands with their current
unavailable-value context.

Text/number/URL popup editors use an instance-specific label/ID linked to the
real `property.name`, with current input type, numeric step and URL action.
Bound popup width on narrow screens. Add the 44px coarse-pointer floor to the
affected row/input interaction shape while preserving fine-pointer density.

## C06: field-specific validation

Keep `CollaborationCreationFieldsSchema` and current catalog checks authoritative.
Do not maintain a second numeric range/validity implementation. Preserve the
schema parse result so stage errors can be attributed to its actual path and
given a localized inline message with `aria-invalid`/`aria-describedby`.
The correction is a whole number from 1 through the current schema limit, or
empty when no stage is required. The generic unavailable-selection message
continues to represent catalog/identity failures; it must not falsely explain a
stage formatting error. Validity and submit guards remain equivalent except
for better feedback.

## C04/C08: client editor versus protected projection

The decision component remains keyed by the last exact reviewed tuple. Current
authorized candidate data remains exclusively in Query, and only that current
tuple can supply approve/reject/withdraw permission or prepare an intent.

The user-owned decision reason must not lose its actual textbox node or cursor
when a same-tuple renewal temporarily erases protected material. Keep client
input separate from candidate-derived body and action gates. Existing human
draft text or the client's current editing target can keep that textbox present;
do not disable it merely because a renewal is pending. Keep the reason container
at a stable React position even as approval/action siblings are hidden or shown.
No previous
candidate, body, quoted decision reason or capability may be copied into local
state. A changed tuple resets review confirmation/reason. During unavailable or
unauthorized reads no old candidate action becomes available.

Key the review itself by the selected candidate ID so selecting another candidate
clears the previous client decision immediately, even while the new read is
pending. Keep the existing exact-tuple key within that selection.

Propagate the auxiliary category's `active` state through the source knowledge
collection and its selected review. Use an optional enabled/read-visibility
argument for the selected candidate hook, defaulting to existing standalone
behavior. Hidden review must not focus, poll, refocus-retry or renew on expiry.
Keep Core projection lifecycle bindings and timers so hidden private data still
expires and later reads require fresh authority. Do not change the necessary
root list/current-context reads that feed toolbar errors, pending recovery and
known counts.

The retained proposal source picker can observe the same candidate key as its
review and has independent historical memory/comment/run discovery reads. Pass
`active` through the proposal as well and gate those optional observers, so a
second hidden source-picker observer cannot defeat the review visibility rule.
An already-started user write/source-building operation remains governed by its
existing original intent and current-scope guards; hiding does not cancel it.

Context historical pages and a selected version fetch require active panel
visibility; closing or category switching preserves page/version/editor state
but disables unnecessary history observers. Reopening reads through the same
protected query. Do not cancel another visible consumer's shared Query merely
because this observer becomes hidden.

## C05: local confirmation geometry

Give this context restore/purge confirmation a viewport-bounded width and height.
Retain the title and action area as nonshrinking flex children; put reason,
failure and stale-version content in the scrolling middle. Bound the textarea's
intrinsic growth so a legal multiline reason can scroll within the available
area. The footer and label use the current shared AlertDialog components and
wording. Other global confirmations do not change.

## C07: retry within the existing form

The status options query already owns retries through `refetch()`. Connect it
to `CollaborationQueryState`, display the loading state in the same field group,
and keep the form mounted. Confirmation remains blocked until current candidate
data is valid and not fetching. No new action request or replacement key is
sent on a read retry.

## Test architecture and compatibility

Pure validation/authorization matrices stay in Core. Shared DOM suites verify
the input name/state, search-to-commit behavior, DOM/focus continuity and form
retry wiring; use the actual shared primitives. Geometry/touch target assertions
run against compiled original CSS in Chromium. Test both locales when wording
changes and both themes/narrow/enlarged layouts in one bounded visual batch.

No server wire or durable draft shape changes are expected. Any optional hook or
picker argument defaults to current behavior at existing callers. Changed
knowledge review display rules must be documented beside the exact tuple/lease
contract; they do not change authorization or expiry.

## Risks and rollback

- Shared pickers have many consumers: preserve listbox, empty-row, IME and
  ordinary issue tests and avoid global role changes for command rows.
- Keeping editor nodes during renewal must not keep private projection facts or
  allow stale decisions. Test expiry, denial and tuple change independently.
- Hidden Query disabling must not block visible consumers or toolbar facts.
  Keep root status reads and test the current/hidden observer boundary.
- Locale files already contain another session's uncommitted changes. Edit only
  the necessary keys, preserve the baseline, and never stage unrelated hunks.
- The shared checkout has other ongoing work. Source snapshots and task-specific
  diff review identify this task's changes; do not reset, stash or overwrite
  other work, restart the user's app, or modify the running database.

All changes are reversible source edits. Rollback is per ownership group; there
are no database, storage or release operations to undo.
