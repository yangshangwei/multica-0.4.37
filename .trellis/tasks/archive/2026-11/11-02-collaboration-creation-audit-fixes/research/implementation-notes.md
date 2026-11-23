# Implementation notes

## Recovery entry

- Read the saved native hook output, current task artifacts/guidelines, trellis-before-dev and impeccable craft-floor.
- Current source already contains partial R1 source locking (scoped pending mutation + contentful records), and three useful R1 regressions from the interrupted implementer. Retained these edits.
- The inherited parent picker/loading/retry/denial behavior and searchMaxLength remain in place.
- Write boundary: planned shared views/locale/picker files and colocated tests only; no Core, global UI primitive/token, browser fixture, task lifecycle or git changes.
- Canonical Core safety suites passed 43 tests in the main session; no Core edits planned.
- Running the current creation-form suite before further edits.

## P1 progress

- Current creation-form suite: 1 file / 9 tests passed, including delayed creation, uncertain original-key recovery, and idle content-free recovery source locking.
- Recovered partial R2 changes: opt-in native textbox/listbox relation, stable option IDs, selected/disabled states, active-option synchronization and paging/footer focus regressions. Running account-picker + generic picker suites now.
- Added one local form-styles module to reuse opaque faint-foreground focus across the form and portaled account/parent pickers. Existing global tokens/primitives are unchanged; native portal search/options gain visible outlines. Actual geometry/contrast remains the main session's browser gate.

## P2 regression and implementation pass

- Before fixing P2: confirmed full-ID option lookup fails, Cancel is absent, parent input triggers an immediate query, and a queued old-source query still starts. Evidence: p2-red-confirmed.log.
- Corrected the quota fixture to return its actually uploaded metadata in draft reconciliation; both locale cases then fail specifically because the quota/admin explanation is absent (quota-red-confirmed.log).
- Added optional constrained layout/onCancel to the shared panel. Source selector stays outside keyed form so workspace changes retain its DOM/focus; header/footer remain normal flow, notices and fields scroll inside the form. Standalone panel keeps its original layout contract.
- Added 26-code confirmed-refusal labeling with safe fallback and localized recovery actions, full permitted account IDs, coarse target styles, and raw/settled parent-query debounce with mismatch suppression.
- Existing late-search regression now waits for the original request to actually start before issuing its replacement; all existing assertions remain.

## Validation checkpoint

- Targeted six-file suite: 107 tests passed.
- UI typecheck: pass. Full views lint: exit 0, with 27 pre-existing warnings outside this task (no lint errors).
- Views typecheck found two test-only Playwright-style exact options on Testing Library getByRole; removed those invalid options and rerunning.
- Updated members-only negative assertion to query role=option after the semantic repair, so it still catches an exposed external account. Caller compatibility suite includes that updated account suite.
- Main was notified that all UI/product files are ready for actual Chromium measurements.

## Implementer handoff

### Product files changed for this task

- collaboration/creation-form.tsx: scoped source lock, optional constrained normal-flow layout and Cancel, typed refusal rendering, debounced parent query with immediate stale-candidate suppression.
- collaboration/account-picker.tsx: stable option/active semantics, full permitted identity in visible and accessible labels, exact/selected identity, local focus and coarse targets; preserved footer/ref focus transitions and members-only checks.
- collaboration/field-inputs.tsx: reused portal focus/coarse classes and 44px stage target; preserved inherited parent popup/retry/source-denial behavior.
- collaboration/form-styles.ts (new): small local semantic-token focus/portal style constants.
- collaboration/labels.ts and both en/zh-Hans collaboration JSONs: all 26 known confirmed refusals plus factual fallback, full account-ID label and Cancel.
- issues/components/pickers/property-picker.tsx: retained recovered opt-in listbox relation, option ID/selection/disabled and active navigation; no blanket roles or resizing.
- Colocated creation-form/account-picker/property-picker tests: source lifecycle, ARIA/IME/paging, same-name/exact IDs, quota+files, cancellation/native Enter, immediate/delayed query and source cleanup.

### Exact verification

1. pnpm --filter @multica/views exec vitest run collaboration/creation-form.test.tsx — 9/9 pass before P2.
2. pnpm --filter @multica/views exec vitest run collaboration/account-picker.test.tsx issues/components/pickers/property-picker.test.tsx — 17/17 pass before P2.
3. pnpm --filter @multica/views exec vitest run collaboration/creation-form.test.tsx collaboration/account-picker.test.tsx collaboration/field-editing.test.tsx collaboration/attachment-picker.test.tsx issues/components/pickers/property-picker.test.tsx locales/parity.test.ts — 6 files / 107 tests pass (implementation-targeted.log).
4. pnpm --filter @multica/views exec vitest run collaboration/account-picker.test.tsx issues/components/pickers/assignee-picker.keyboard.test.tsx issues/components/pickers/stage-picker.test.tsx projects/components/project-picker.test.tsx projects/components/project-picker.open-state.test.tsx modals/quick-create-actor-picker.test.tsx — 6 files / 56 tests pass (implementation-caller-compatibility.log); account overlap is intentional, do not sum as unique tests.
5. pnpm --filter @multica/views typecheck and pnpm --filter @multica/ui typecheck — pass after removing two unsupported Testing Library exact options.
6. pnpm --filter @multica/views lint — exit 0, 27 pre-existing warnings outside owned changes (implementation-lint.log).
7. Owned-file ESLint — pass, no warnings/errors (implementation-owned-lint.log).
8. pnpm check:ui-exports and scoped git diff --check — pass.

No Core/global UI primitive/token/API/dependency/environment/commit/lifecycle/preview edits were made by this implementer. Main session owns current browser contrast/layout/touch/AX validation, final independent trellis-check, any necessary follow-up and final ownership/commit/spec record.

## Bounded browser correction

- Read visual-verdict-1.json (88/revise) and the amended design/plan before UI edits.
- Added explicit outline-solid to local native search/option/summary focus rules, retaining the opaque token, width and offset. Label popup reuses those classes; the launch Button has explicit opaque ring/border classes. No global primitives/tokens or custom-property public API changed.
- Added a nonempty-only creation feedback anchor and an error-transition effect confined to constrained forms, guarded by active/current API/account scope. It scrolls block:nearest on settled mutation feedback only, without focusing or starting any request. Query refreshes and later edits do not retrigger it.
- Three meaningful behavior cases: quota feedback once across query refresh/edit; unknown original-request actions; standalone no-scroll. Red confirmation: 2 constrained cases fail for missing scroll, standalone passes (correction-feedback-red.log).
- Running targeted suites and owned lint/types after this one corrective batch; actual painting and visibility remain the main session's final browser confirmation.

- First correction checks passed: 110 targeted tests, owned lint, and Views/UI typecheck.
- Tightened unknown-result reveal for 320px: the anchor targets the matching original request's recovery-action group rather than the tall combined error/recovery wrapper. Confirmed rejection targets its cause region; generic failures without a pending request target their own alert. A region regression first failed because the generic error was still included in the recovery scroll target (correction-recovery-region-red.log), then the refs were narrowed. No extra focus/API actions or default empty gap introduced.
- Final same-batch targeted/lint/types running after this narrower anchor adjustment.

## Corrective handoff checks

- Final bounded correction: six targeted files / 110 tests pass (correction-targeted.log).
- Owned lint, Views typecheck and UI typecheck pass (correction-owned-lint.log, correction-typecheck.log, correction-ui-typecheck.log).
- Scoped whitespace diff check passes.
- Product edits limited to form-styles.ts, creation-form.tsx and field-inputs.tsx; creation-form tests add the three meaningful feedback cases. All original source/scope/recovery/IME/debounce behavior remains.
- Main owns the final actual-browser outline/feedback/control visibility confirmation and independent check; no further UI polishing is requested by the implementer.

## Required 320px focus correction

- Read visual-verdict-2.json (89/revise) and updated R7 design before editing. Actual 320px keyboard evidence showed the textarea bottom outside the body clip; accepted 390px/contrast/identity/targets remain unchanged.
- CreationContents now handles focus capture only in constrained mode, for actual DOM descendants and native controls with positive layout dimensions. It reveals only out-of-inset targets with block:nearest, preserving focus and excluding portal-bubbled inputs. Local scroll padding/margin reserve the existing 3px ring without changing control sizes or header/footer/query/draft behavior.
- Added real-event wiring regression with bounded geometry: clipped vs visible textarea, portal search exclusion, standalone no-scroll and zero creation writes. Red result: constrained case fails for missing reveal, standalone passes (body-focus-red.log).
- Running relevant creation/account/field suites, owned lint and Views typecheck; main owns the two-theme 320px actual-browser confirmation.

- R7 body-focus handoff: creation/account/field suite passes 35 tests, owned lint and Views typecheck pass, scoped diff check passes. Product change is confined to CreationContents and two directly related test cases; actual 320px light/dark visibility remains the main confirmation gate.
