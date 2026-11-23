# 修复协作空间 Impeccable 审计问题并完成回归

## Goal

Correct the nine verified collaboration audit defects so users can reliably
select fields, keep typing during authorized refreshes, recover reads in place,
and reach confirmation actions on small screens. Deliver regression evidence
for the shared Web/Desktop behavior.

## Background and authorization

- Source requirement: the user asked for an Impeccable audit, then explicitly
  requested fixing the findings according to the recommendations and running
  regression tests.
- On 2026-11-18 the user explicitly consented to creating this Trellis task and
  entering planning. The record is currently `planning`.
- The authoritative finding/evidence mapping is
  `docs/audits/2026-11-17-collaboration-impeccable.md`.
- The baseline Chromium fixture uses the real shared components and CSS with
  isolated, synthetic data. It proves specific defects without performing live
  collaboration writes.
- Existing role/action, private-data authorization, original-request recovery,
  and shared Web/Desktop contracts remain applicable.

## Requirements and acceptance criteria

| ID | Priority | Verified defect and source | Observable acceptance |
| --- | --- | --- | --- |
| C01 | P1 | Custom select/multi-select search is displayed for >7 choices but does not filter; typing `Choice 10` and pressing Enter stores `choice-1`. `custom-property-picker.tsx:211,247`; collaboration entry `field-inputs.tsx:59`. | Exact and partial queries show only matching real choices. A query with no match announces that state and cannot select a nonmatch. Keyboard selection, clearing, reopen and previously selected values continue working in single and multiple selection. |
| C02 | P1 | Selected labels and custom multi-select rows expose no selected state. `field-inputs.tsx:24`; `property-picker.tsx:356`. | Auxiliary technology can identify initial, newly selected and deselected rows through semantic state. Existing account listbox/option navigation and clear actions remain correct. |
| C03 | P1 | Text/number/URL editor input loses its real property name. `custom-property-picker.tsx:437`; trigger label `field-inputs.tsx:56`. | The actual input is named after the property, with persistent field context. All three types keep their current commit, numeric and URL actions. |
| C04 | P1 | A 15-second candidate renewal removes and reconstructs the decision reason input, losing focus and selection. `knowledge-review.tsx:97,114`; `knowledge-queries.ts:23`; `knowledge-cache.ts:61`. | Renewing the same review tuple does not interrupt typing or selection. A changed tuple resets review confirmation and reasons. Expiry, unavailable reads and denial remove protected material and cannot authorize a decision. Hidden panels do not take focus. |
| C05 | P1 | A legal 1,000-byte multiline purge reason makes the confirmation exceed the viewport and hides actions. `memory-panel.tsx:218`; `alert-dialog.tsx:55`; `textarea.tsx:10`. | Legal multiline input can be edited while the title and cancel/confirm actions remain reachable at desktop, 320px mobile width, short windows and enlarged text, in both themes. No change to purge permission, snapshot or request semantics. |
| C06 | P2 | Invalid stage input is described as an unavailable selection without linking the actual field. `creation-form.tsx:254,368`; `field-inputs.tsx:145`. | Zero, fractions and values outside the stage range identify the field and correction in localized accessible feedback. Correcting the field enables normal submit. Unavailable labels/properties retain their distinct current messages. |
| C07 | P2 | Failed status candidates have no explicit retry within the workflow dialog. `workflow.tsx:302,254`; Core `queries.ts:46`. | A transient read failure offers a discoverable retry, retains the same dialog and typed draft, and blocks submit while pending or invalid. Denial still applies existing access cleanup. |
| C08 | P2 | Hidden mounted history pages and selected knowledge candidate bodies continue periodic reads. `detail-auxiliary.tsx:85`; `memory-panel.tsx:60`; `knowledge-queries.ts:72`. | Closing/switching the auxiliary panel stops unnecessary history/candidate reads. Reopening performs a current authorized read, without discarding drafts. Required toolbar root counts/errors/recovery continue working and knowledge lease expiry still erases private projections. |
| C09 | P2 | Label/custom-property popover rows and text input measure only 32px under coarse pointer. `field-inputs.tsx:25`; `property-picker.tsx:364`; `custom-property-picker.tsx:445`. | A coarse pointer gets >=44px interaction height for affected choices and inputs without horizontal overflow. Fine-pointer desktop density and keyboard selection remain usable. |

File anchors above are baseline locations under `packages/views/` unless a Core
or UI file is named. Regression ownership and exact paths belong in
`implement.md`; no new workflow, store, or persistence model is required.

## Preservation constraints

- Continue using the shared collaboration views, picker primitives, semantic
  color/typography tokens and English/Chinese localization.
- Keep server facts in Query and client input in the existing client-owned
  state. Never store a cleared knowledge body, quoted reason or capability in
  component state to preserve an editor.
- Keep exact candidate tuple, receipt/request identity, role permissions,
  uploads/cancellation and `hidden`/`inert` auxiliary behavior.
- Retain mounted forms across category changes; inactive content cannot take
  focus or launch nested previews.
- No new dependencies. Keep changes confined to the listed defects and their
  canonical regression/spec surfaces; do not include other pending work.

## Verification outcome

Each C01–C09 must have a recorded outcome linked to an actual behavior check.
New behavioral regressions must demonstrate the pre-fix failure before a
minimal fix, then pass with related existing suites. Geometry is verified in a
real browser with compiled original CSS rather than class-string assertions.
The final report distinguishes shared component/browser verification from any
production Web, native Electron, physical-device or screen-reader work that
was actually performed.

## Out of scope

- A visual redesign, replacing the established collaboration layout, adding
  new collaboration product behavior, or changing factual UI text.
- Backend workflows, stage execution/AI provider behavior, auth/migrations,
  runtime consent or deployed collaboration acceptance.
- Native mobile UI, new languages, physical-device certification or a complete
  WCAG certification.
- Unrelated ongoing edits, release/deployment, broad optimization or benchmark
  work beyond the verified unnecessary reads.

## Planning status

The audit and user requests resolve the intended scope and acceptance behavior;
no product clarification is needed. Planning must record the technical approach
and test boundaries, including the review-input/lease and hidden-read
preservation contracts, before implementation is activated.
