# Research: shared pickers and collaboration initial fields

- Query: Determine a minimal, backward-safe repair for audit findings C01, C02, C03, C06 and C09.
- Scope: internal; shared Web/Desktop Views components and existing Core validation contracts. Planning only.
- Date: 2026-11-18
- Active task: `.trellis/tasks/11-18-collaboration-impeccable-remediation`
- Source requirement: `docs/audits/2026-11-17-collaboration-impeccable.md`.

## Findings

The five findings can be repaired in the existing components. They do not need another picker implementation, a store, a dependency, a schema change or an app-specific workaround. The shared primitives serve many ordinary-task callers, so changes to their semantics must be opt-in where the existing interaction is not a toggle.

### Files found

| File | Responsibility |
| --- | --- |
| `packages/views/issues/components/pickers/property-picker.tsx` | Search shell, keyboard navigation, close/reset handling, generic rows and opt-in account-listbox semantics. |
| `packages/views/issues/components/pickers/custom-property-picker.tsx` | Shared single/multi-select, date, checkbox and text/number/URL value editors. |
| `packages/views/collaboration/field-inputs.tsx` | Collaboration label toggle rows, property editor integration and optional parent/stage controls. |
| `packages/views/collaboration/creation-form.tsx` | Draft owner, schema/catalog validity, initial-fields disclosure and creation gating. |
| `packages/core/collaboration/creation-schema.ts` | Authoritative immutable creation/initial-field validation. |
| `packages/core/collaboration/delivery-draft.ts` | Mutable draft validation; deliberately permits unfinished stage numbers. |
| `packages/views/collaboration/labels.ts` | Existing localized creation-rejection code mapping, including `invalid_stage`. |
| `packages/views/locales/{en,zh-Hans}/collaboration.json` | Existing stage guidance, invalid-stage reason and directory-failure messages. |
| `packages/views/locales/{en,zh-Hans}/issues.json` | Existing picker empty-result and custom-property strings. |
| `packages/views/locales/parity.test.ts` | Supported-language key parity. |

### C01: filter the actual custom-property candidates

`CustomPropertyValueInput` enables `searchable` when select/multi-select definitions contain more than seven options, but it renders the complete `options.map` and supplies no `onSearchChange` (`custom-property-picker.tsx:204-263`). `PropertyPicker` keeps the textbox value and calls the optional callback; it intentionally does not filter arbitrary children (`property-picker.tsx:287-303`).

Recommended design:

1. Add one local search value to `CustomPropertyValueInput`, and pass its setter to the single- and multi-select `PropertyPicker` branches.
2. Filter options using the existing local convention: trimmed, case-insensitive name matching. `CollaborationLabelInput` already uses `label.name.toLocaleLowerCase().includes(search.trim().toLocaleLowerCase())` (`field-inputs.tsx:25`). No shared option-search normalizer was found in the inspected picker/common/Core utility scope; a new abstraction or fuzzy-search dependency is unnecessary.
3. Decide whether to show the search input from the total configured option count, not the filtered count. Otherwise the textbox disappears after a useful query.
4. Render the existing `PickerEmpty` (`property-picker.tsx:417`) when a typed query matches no real options. It already uses `issues.pickers.no_results` in both supported locales. Do not add a selectable fake result.
5. Keep the fixed empty-value row mounted and marked `emptyValue`. A typed no-match Enter must neither clear the existing value nor select a hidden candidate. The current keyboard shell already excludes that row from typed first-match/unique-result fallback (`property-picker.tsx:183-199,245-254`). An explicit click or Arrow navigation may still select the clear action.
6. Keep filtering independent of value mutation. Existing selected values remain intact when filtered out; reopening or clearing the search restores them with the correct state.
7. Reuse the shell's existing open-to-closed reset (`property-picker.tsx:206-222`), which resets both its internal query and `onSearchChange("")`. Do not duplicate that reset in an unrelated effect. Callers are already keyed by property ID in collaboration (`field-inputs.tsx:41`).

The existing typed-search highlight behavior gives the first real committed match priority. A `navigationResetKey` based on the normalized query and current candidate identities may be used to guard a definition update while a popup stays open; this is an existing opt-in API. Preserve typed input priority, IME handling and selected values. Do not rewrite the keyboard shell for this repair.

### C02: distinguish toggle rows from selection/clear commands

`PickerItem.selected` currently controls a visual checkmark. Without `optionId`, the native button exposes no selection state; the checkmark is `aria-hidden` (`property-picker.tsx:352-377`). The required interactions are different:

| Row kind | Actual interaction | Correct state/role |
| --- | --- | --- |
| Account row with `optionId` | Composite account selection; focus stays on the search textbox. | Preserve `role="option"`, `aria-selected`, `aria-disabled`, `tabIndex=-1` and existing active-descendant relation. |
| Available collaboration label | Clicking adds/removes that label and leaves the popup open. | Native toggle button with `aria-pressed={selected}`. |
| Custom `multi_select` option | Clicking adds/removes that option and leaves the popup open. | Native toggle button with `aria-pressed={selected}`. |
| Fixed `emptyValue` row | Explicitly clear/unset, then close where the current caller closes. | Plain command button; preserve `data-picker-empty`. |
| Custom single-select option, checkbox Yes/No, parent, assignee | Select a value and close; clicking the selected row does not invert it. | Keep existing command behavior in this bounded repair; do not label every selected row as a toggle. |
| Unavailable selected label row | Remove a value that cannot be re-added from the current catalog. | Plain removal command with the existing unavailable-value context. Do not promise a reversible toggle. |

Recommended minimal API: add an explicit `toggle?: boolean` opt-in to `PickerItem`; use `selected` as its pressed state. Account/listbox mode takes precedence, and `emptyValue` remains a clear command. Default callers retain native button semantics. Do not blanket-add `aria-pressed={selected}` to every non-account row, and do not convert all pickers to listboxes.

Apply the opt-in to available collaboration label rows (`field-inputs.tsx:25`) and shared custom `multi_select` options (`custom-property-picker.tsx:253`). Mark the collaboration label “None” clear row as `emptyValue` (`field-inputs.tsx:24`) so its existing clear action obeys the shared typed-search contract. Existing keyboard Space/Enter on a focused toggle button remains native; there is no need for another key handler.

The ordinary shared `ActorPropertyPicker` has multi-actor toggles too, but collaboration intercepts actor/multi-actor definitions and uses its account picker (`field-inputs.tsx:54-58`). It is not necessary to expand this repair into ordinary actor-picker behavior. Its tests belong in the regression set because the generic primitive is shared.

### C03: label the input inside the popup

The collaboration property row associates `definition.name` with the trigger (`field-inputs.tsx:56-61`). That relationship does not label the actual popup input. `TextishPropertyEditor` supplies only a type-specific generic placeholder (`custom-property-picker.tsx:395-398,437-446`).

Add an instance-scoped React `useId()` in `TextishPropertyEditor`. Render a visible, wrapping label containing `property.name` inside `PopoverContent`, and associate it with the existing input through `htmlFor`/`id`. Keep the existing input/form row after the label. Do not derive the DOM ID solely from `property.id`: the same property may appear in several mounted task rows or detail surfaces.

No locale key is required for the label: the configured property name is already the authoritative business text. Keep number input type, decimal `inputMode`, `step="any"`, URL open action, draft initialization, trimming and Enter/clear semantics unchanged. A generic `aria-label="Value"` alone would leave out the field context and the requested persistent visible label.

### C06: use schema paths to expose the stage error

`CreationForm` discards `safeParse` error paths and combines schema/catalog validity in one boolean (`creation-form.tsx:254-258`). It disables creation and displays only `fields.invalidSelection` (`creation-form.tsx:306,368`), which describes unavailable catalog values even for a malformed stage.

The immutable schema already owns the range: optional/null or an integer from 1 to 2,147,483,647 (`creation-schema.ts:19-25`). The mutable form schema deliberately relaxes stage to `z.number().nullable().optional()` (`delivery-draft.ts:27-29`). This lets users keep and correct unfinished values. Do not tighten that draft schema or silently discard a bad stage.

Recommended design:

1. Retain the existing `CollaborationCreationFieldsSchema.safeParse(initial)` result. Derive stage-invalid from an error with `issue.path[0] === "stage"`; do not duplicate the numeric rules with a separate range validator.
2. Keep existing catalog checks for status, labels, custom properties and iteration. Separate catalog failure and non-stage schema errors from stage-only failure, so a stage error does not emit the misleading generic unavailable-selection message.
3. Add an optional presentation prop such as `stageError?: string` to `CollaborationSourceInputs`. The caller passes the localized stage reason. Existing workflow/source-editing callers remain backward-compatible without the prop.
4. Associate a visible inline error with the stage input: `aria-invalid`, an instance-scoped error ID in `aria-describedby` together with the current stage hint, and an adjacent `role="alert"` error paragraph. Follow the existing field-error pattern in `workflow.tsx:305-319` and `knowledge-proposal.tsx:117-136`.
5. Reuse `creation.rejectionReasons.invalidStage` through the existing `useCollaborationLabels().creationRejection("invalid_stage")` mapping (`labels.ts:25`). If the message is updated, include the supported upper bound as well as positive whole-number/blank guidance in both `en` and `zh-Hans`. This also improves the same server-rejection reason; a duplicate local-validation key is unnecessary.
6. Preserve automatic opening of the initial-fields section on invalid input. Correcting the value or clearing/removing the parent must clear the inline error and restore creation eligibility without dropping title, description, recipient or attachments.
7. A simultaneous directory failure must still show the existing generic unavailable-selection feedback. This is not permission to clear unknown values or submit against invalid authority.

Date/schema errors can be mapped to the existing `invalidStartDate`/`invalidDueDate` reasons if the implementation expands beyond the audited stage case. They are not a reason to add another validation subsystem. The acceptance focus for C06 is the stage input and preserved catalog failures.

### C09: add touch floors at the existing controls

The audit's final Chromium coarse-pointer fixture measured 32px label rows and a 32px text-value input; these measurements are already recorded in the audit evidence. Use `pointer-coarse:min-h-11` so the fine-pointer desktop density remains unchanged.

- Prefer the shared `PickerItem` base class for the row floor (`property-picker.tsx:364`): one change covers the audited label/custom-option rows and maintains a consistent coarse-pointer rule. Existing account/parent minimums remain compatible.
- Add the floor to `TextishPropertyEditor`'s existing `h-8` input (`custom-property-picker.tsx:445`). Keep `h-8` for desktop.
- The related URL open icon button should have coarse minimum width and height of 44px so the popup does not retain a smaller neighboring touch action.
- The shared search textbox is another input in these popups. If it is included in this repair, put its coarse floor on the existing input in `PropertyPicker` (`property-picker.tsx:282-298`), not on the app-wide Input component. Preserve the existing scrollable candidate owner.
- Wrap the new visible property label and long option content; measure the popup at 320px. Avoid class-only tests that claim to prove painted size or overflow.

44px is this task's ergonomic target. Do not present a 32px control alone as failure of the WCAG 2.2 AA 24px target criterion.

## Proposed write scope after task activation

Production files:

- `packages/views/issues/components/pickers/property-picker.tsx`
- `packages/views/issues/components/pickers/custom-property-picker.tsx`
- `packages/views/collaboration/field-inputs.tsx`
- `packages/views/collaboration/creation-form.tsx`
- `packages/views/locales/en/collaboration.json` and `packages/views/locales/zh-Hans/collaboration.json` only if improving the existing bounded-stage reason.

No Core implementation change is required. Other agents already have changes in the collaboration locale files and stage-code files; edit only the relevant stage-validation strings and preserve their other entries.

## Focused test plan

The tests should prove user-visible behavior, not implementation classes. Keep pure helper tests in the existing `.test.ts`; add actual custom editor behavior in a separate `.test.tsx` next to the shared component.

| Test file | Evidence to add or preserve |
| --- | --- |
| `issues/components/pickers/custom-property-picker.test.tsx` (new) | 8+ option single/multi-select filtering; exact/partial/case-insensitive/no-match input; Enter selects the visible match only; no-match Enter preserves the value; explicit clear; close/reopen reset; retained hidden selections; toggle state; text/number/URL input named by the actual property; Enter, decimal and URL behavior. |
| `issues/components/pickers/property-picker.test.tsx` | Explicit toggle true/false and state updates; default command and empty-value rows have no pressed state; existing IME, typed-search priority, unique-result, definition-change and close/reset regressions. |
| `collaboration/field-inputs.test.tsx` | Labels expose initial/add/remove pressed state; None stays a clear command; unavailable selected label remains removable; actual optional stage error relation; parent select/clear/IME/Escape remain correct. |
| `collaboration/creation-form.test.tsx` | Bad stage blocks creation with a named associated error; correction/blank restores eligibility with the draft retained; schema upper-bound and fractional triggers; catalog failure still has correct feedback. |
| `collaboration/field-editing.test.tsx` | Existing source-editing callers still work without the optional error prop; field values and initial draft restoration remain intact. |
| `collaboration/account-picker.test.tsx` | Existing `optionId` listbox state/active-descendant semantics and full identity survive the generic row change. |
| `locales/parity.test.ts` | Both supported locales remain structurally consistent. |

Focused command after the new UI test file exists:

```bash
pnpm --dir packages/views exec vitest run issues/components/pickers/property-picker.test.tsx issues/components/pickers/custom-property-picker.test.ts issues/components/pickers/custom-property-picker.test.tsx collaboration/field-inputs.test.tsx collaboration/creation-form.test.tsx collaboration/field-editing.test.tsx collaboration/account-picker.test.tsx locales/parity.test.ts
```

Compatibility command for the shared primitive:

```bash
pnpm --dir packages/views exec vitest run issues/components/pickers/assignee-picker.keyboard.test.tsx issues/components/pickers/actor-property-picker.test.ts issues/components/pickers/status-picker.test.tsx issues/components/pickers/stage-picker.test.tsx projects/components/project-picker.test.tsx modals/quick-create-actor-picker.test.tsx
```

Parent-owned visual verification should reuse the actual component fixture in `artifacts/qa/collaboration-impeccable-20261117/preview/main.tsx` or an equivalent current-source fixture. The final browser pass should assert the chosen option payload, accessible selected states, real field input names, at least 44px coarse rectangles, wrapping and document overflow at 320px, both themes, and fine-pointer desktop geometry. Wait for fonts, finite animations and painting before geometry/screenshot capture. The existing audit script is a defect reproducer, not automatically a passing regression suite after repairs.

## Related specs and references

- `.trellis/spec/views/frontend/collaboration-creation.md`: source locks, draft lifetimes, native submission, focus, portal style and 44px coarse controls.
- `.trellis/spec/views/frontend/collaboration-source-picker.md`: optional stage, parent identity, clear commands, search reset and protected Query recovery.
- `.trellis/spec/ui/frontend/input-styles.md`: preserve boundary/fill tokens and verify compiled focus styles rather than raw token contrast.
- `apps/docs/content/docs/developers/conventions.mdx` and `.zh.mdx`: shared package boundaries, instance/component naming and supported locale/product voice.
- Existing normative audit references: [WAI-ARIA 1.2 aria-pressed](https://www.w3.org/TR/wai-aria-1.2/#aria-pressed), [aria-selected](https://www.w3.org/TR/wai-aria-1.2/#aria-selected), [WCAG 3.3.1 Error Identification](https://www.w3.org/WAI/WCAG22/Understanding/error-identification.html), [WCAG 3.3.2 Labels or Instructions](https://www.w3.org/WAI/WCAG22/Understanding/labels-or-instructions.html). This internal research did not fetch external documentation or claim assistive-technology certification.

## Caveats / Not Found

- No production code or tests were changed in this planning pass, and no new passing-test claim is made. The prior audit's six files/78 tests were a baseline that did not cover the missing behaviors.
- Search and state changes affect shared ordinary-task editors as well as collaboration. Keep the repair in shared Views and include compatibility tests; app-specific fixes would leave the same component inconsistent.
- Do not repurpose `selected` into universal toggle semantics. `optionId`, native command rows and `emptyValue` have distinct existing contracts.
- Do not tighten the mutable stage draft schema; invalid-but-editable numbers must survive long enough for correction.
- A schema-path-based view error is sufficient for this bounded repair. No reusable field-error framework or new store was found to justify an abstraction.
- Runtime, source-workspace, original-request, upload and AI-adoption fences remain owned by existing Core/Views logic; picker presentation repairs must not weaken them.
- Native OS touch, spoken screen readers and installed Electron Main are separate evidence surfaces. Browser coarse-pointer geometry and DOM/AX results establish only the checks performed in that fixture.
