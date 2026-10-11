# Pending-task creation form

## Scope and trigger

These contracts apply to `TriageCreateDialog`, its editors and shared metadata
pickers. The form creates pending intake; accepting or executing a task remains
a separate flow. See [server triage](../../server/triage.md).

## Component signatures

- `TriageFieldsEditor` accepts optional `candidateMode`, `pickerAffordance`,
  `idPrefix` and date-field `errors`. Defaults preserve acceptance presentation
  and its `triage-priority-field` focus target. Creation uses `idPrefix="triage-create"`.
- `PropertyPicker.triggerAriaLabel` names the field; its selected value is a
  separate accessible description. Optional `triggerAriaDescription: string`
  supplies plain text when rendered content includes decorative avatar initials.
- `usePickerTrigger` resolves custom render children before wrapping the selected
  value. Priority/assignee cold triggers pass the same attributes through
  `DeferredPopup` so field names exist before interaction without starting their
  queries early. Keep custom content precedence and unlabelled defaults intact.
- `DialogContent.closeLabel` and `FileUploadButton.showLabel` are opt-in;
  existing callers retain their defaults.

## Submission and accessibility contracts

Creation maps project/assignee selections to `candidate_project_id`,
`candidate_assignee_type` and `candidate_assignee_id`. Labels and payload field
names do not change. Candidate wording is limited to creation; acceptance keeps
formal project and assignee labels.

Title and description have persistent visible labels connected to the real
contenteditable roots. Title exposes `aria-required`. Field errors use
`aria-invalid` and `aria-describedby`, and submission focuses the first invalid
field. Concurrent fields editors must use distinct ID prefixes so each label,
error and focus target belongs to its own form.

Read current editor Markdown on submission. Block while an upload is active or
a request is in flight. Include only uploaded attachment IDs whose links remain
in that Markdown. The same payload retains its request ID after failure; an
edited payload is a new intention. Invoke success callbacks and close only after
the mutation succeeds. Unknown failures keep the draft and a readable alert.

## Validation and error matrix

| Input or response | Result |
| --- | --- |
| Empty trimmed title / over 500 Unicode code points | Localized title error; no request |
| Description over 1 MiB UTF-8 / NUL in title or description | Error on the offending editor; no request |
| Nonempty source URL without an explicit HTTP(S) authority or malformed URL | Source-link error; no request |
| Invalid `YYYY-MM-DD` or impossible calendar date | Error on that date field |
| Start date after due date | Errors on both date fields |
| Known 4xx validation code or exact supported message | Localized field error |
| Combined title/description limit message | Attribute only limits actually violated by the submitted draft |
| Unknown, ambiguous or non-4xx failure | Global creation-failure feedback; do not blame valid fields |

Date validation must match Go's date-only calendar rather than JavaScript date
normalization. URL validation must not accept WHATWG repairs such as
`http:example.com`. The pure matrix lives in
`triage-create-dialog.validation.test.ts`; do not duplicate it in DOM tests.

## Good, base and bad cases

- Base: empty title keeps the create button disabled; the keyboard shortcut
  still validates and explains the missing title.
- Good: failed creation retains candidate values, Markdown and retry identity.
- Bad: assigning the combined server limit message to every editor, or sharing
  date-control IDs between the intake and acceptance forms.

## Verification

`triage-create-dialog.test.tsx` mounts real editors/pickers and covers bilingual
names, concurrent fields, error wiring/focus, candidates, retries, duplicate
submission and upload lifecycle. Editor and PropertyPicker tests cover shared
prop forwarding; `deferred-trigger.test.tsx` covers names before first
interaction and deferred queries; locale parity tests cover translations.

Use rendered geometry for the scroll body and fixed footer. At coarse pointer
sizes, picker and close targets must measure at least 44px. Scope these styles
to the intake form; descendant overrides must not shrink picker triggers.
Component-harness checks do not replace real screen-reader or device acceptance.

## Wrong versus correct

Wrong: use a picker’s current value as its entire accessible name, or derive its
selected description from avatar initials plus the visible name.

Correct: name the field "Candidate assignee" and describe the selected value
as "Alex"; keep both the visible label and selected text available.
