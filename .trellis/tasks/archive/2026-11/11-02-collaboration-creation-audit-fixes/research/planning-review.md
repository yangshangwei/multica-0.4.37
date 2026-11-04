# Research: independent creation-repair planning review

- Query: Challenge prd.md/design.md/implement.md against R1–R8, current source, shared callers and safe verification boundaries.
- Scope: internal, planning-only; read-only product/spec inspection, no processes or product changes.
- Date: 2026-11-02.
- Reviewed artifacts: task PRD, design and implementation plan plus creation-contracts.md and ui-boundaries.md as initially drafted.

## Verdict

**READY after the confirmation pass below.** The initial review found two correctness refinements and an inaccurate Core test command; the main session has incorporated them into the revised artifacts and captured current target inputs. The eight-item scope and chosen architecture are appropriate. No new user-owned product decision or repeated task-creation/implementation permission question is needed; the user explicitly authorized planning/review followed by implementation/check in this session.

## Required refinements

### B1 — R8 must treat the debounce interval as pending parent results

The current source-task picker has changed since the initial research. `CollaborationSourceInputs` now owns an inline `PropertyPicker` and computes `parentsReady = !parentsPending && !parentsError` (`field-inputs.tsx:112`). It hides stale candidates while pending and renders explicit loading/error/retry/empty states (`:122–141`). This contract belongs to the parallel `11-02-collaboration-source-picker` task.

The proposed separate raw input/debounced query is correct, but leaving `optionsPending` as only `query.isFetching || query.isPlaceholderData` creates a 250ms interval where the raw input has changed and the last query is still settled. During that interval the old parent rows remain selectable. `navigationResetKey` invalidation alone is insufficient: ArrowDown/Enter or a pointer can deliberately select an old row before the new query begins.

Required plan amendment:

- Treat `parentSearch.trim() !== debouncedParentSearch` as parent-result pending, in addition to the current fetching/placeholder state.
- Keep current `optionsError`, retry behavior, Query key, AbortSignal and authority cleanup intact.
- Add a named regression that changes the parent search then immediately presses ArrowDown/Enter or clicks a prior candidate **before 250ms**; no old parent selection may commit. The final settled query then exposes and selects its own candidates.
- Existing failed-search/retry and source-denial regressions must remain, with time handling adapted to the new debounce rather than removed.

This is a correctness prerequisite for R8, not an expanded product feature.

### B2 — Explicitly preserve the newer shared-file changes

During this review, source code already differs from the initial audit/research snapshots. The other source-picker task is now `in_progress` and edits the same creation/field-input/test/locale surfaces. These are inherited concurrent changes; implementation must start from the current bytes, not restore the earlier native parent search/select or earlier error branch.

Current behavior to preserve:

- `SelectedCreationSource` keeps authorized options after a transient search failure and keeps `CreationForm` mounted (`creation-form.tsx:81–88`). It drops that fallback on `isCollaborationDenied(query.error)`.
- `CreationForm` receives `optionsError` and `onRetryOptions`; parent inputs receive `parentsPending`, `parentsError`, `onRetryParents` (`:91–94`, `:169–171`).
- The parent selector is inline in `field-inputs.tsx`, with `chosenParent` display identity, `searchInputRef` for retry focus restoration, maxLength 200, empty-value clearing and pending/error candidate suppression (`field-inputs.tsx:96–145`).
- The shared `PropertyPicker.searchMaxLength` passthrough is already present (`property-picker.tsx:95/115/275`).
- New creation-form tests cover stale delayed search, mounted draft retry and authoritative source denial (`creation-form.test.tsx:170–260`). Existing field-editing selection assertions have also been adapted.

The transient-error fallback is consistent with the creation success fix: a non-denial refresh failure keeps the same mounted form and its `active` ref, so a valid delayed creation callback remains eligible. Authoritative denial still tears down the protected form and Core source-access handling still fences late results. Do not replace this with unconditional cached fallback or restore the earlier `if (query.error)` unmount branch.

Required planning/dispatch amendment: identify these changes as the implementation input; capture target-file snapshots/current diff before the code owner starts; preserve them in all edits and tests. Because this researcher may not execute git operations, the main session owns the exact git-diff snapshot and any ownership coordination. Current source hashes are recorded below.

### C1 — Replace two absent Core suite names

Every listed views test exists. The proposed `collaboration/delivery-draft.test.ts` and `collaboration/delivery-access.test.ts` do not exist. Their contracts are covered by these actual suites:

```bash
pnpm --filter @multica/core exec vitest run collaboration/delivery.test.ts collaboration/delivery-api.test.ts collaboration/delivery-form.test.tsx collaboration/access.test.ts
```

- `delivery.test.ts`: original key, storage-before-transport, source/account fences, rejection retirement, attachment recovery/cleanup.
- `delivery-api.test.ts`: malformed/wrong-source options and correlated original creation receipt.
- `delivery-form.test.tsx`: a remounted composer observes consumed-bundle retirement.
- `access.test.ts`: current account/API/epoch ownership.

The relevant views suites, four package manifests and `scripts/check-ui-wildcard-exports.mjs` exist. No tests were run by this planning review; paths were verified against repository files.

## R1–R8 review matrix

| Requirement | Assessment | Implementation/check watchpoint |
|---|---|---|
| R1: source lock | Sound after preserving B2 | Record retirement precedes mutation callback, so include pending mutation as well as contentful records. Match current account/API creation prefix. Idle input:null recovery must not permanently lock other sources; its active recovery must lock. Keep active/scope/source authority guards. |
| R2: ARIA | Sound | Retain opt-in textbox controlling the named listbox; actual option IDs must be unique per instance and stable across reorder. aria-selected is committed selection, not highlight. Clear active ID on disabled/removal/empty/close; footer/retry actions stay outside arrow candidates. Preserve IME/229 and committed-DOM navigation. |
| R3: focus contrast | Sound with browser proof | Local opaque faint-foreground is smaller than global tokens. Color and alpha both need override; the portal requires an explicit opt-in class. Measure actual focus on source/text/button/search surfaces after transitions; do not count token prediction as proof. |
| R4: rejection cause | Sound | Consume only typed confirmed refusals. Quota must describe the condition and administrator action; no auto-resubmit or new commercial modal. Unknown transport stays uncertain. English/zh-Hans copy and draft/file preservation need evidence. |
| R5: same-name identity | Sound | Full existing user_id is privacy-safe and unambiguous; no new username/email lookup. Preserve existing Selected account: <name> text as its own element. Exact-result/selected-trigger identity must match the submitted recipient. |
| R6: coarse targets | Sound after B2 | Parent search is now the portaled PropertyPicker input, not the old inline Input. Apply 44px sizing there as well as stage, recipient options and local close target. Measure DOMRect and verify touch without force. Fine-pointer density stays unchanged. |
| R7: persistent actions | Sound | Header/body/footer must be normal flow; native form association/required validation stays intact. Loading/error/no-source/recovery-only branches need accessible cancellation too. Standalone CreationPanel must not require a Dialog provider. Body padding must protect focus rings from clipping/occlusion. |
| R8: debounce | **Needs B1** | Raw/debounced mismatch is pending; hide/unselect stale rows throughout delay and transport. Preserve current error retry and source-denial behavior. Keep original source keys, cancellation and permission refresh. |

## Shared caller and environment review

- Generic PropertyPicker has custom headers/actions, ordinary issue fields, quick-create actors and label-create actions. An optional listbox relationship and caller styles fit the boundary; blanket option/listbox roles or global row resizing would not.
- Existing `property-picker.test.tsx`, assignee keyboard, stage/project/open-state and quick-create actor suites are appropriate compatibility checks; all paths exist.
- The new parent selector uses the same shared picker but need not acquire recipient option semantics. Its existing maxLength/navigation/reset/ref behavior must survive.
- Scoped focus/close overrides avoid changing UI tokens, Dialog/Input/Textarea/Button broadly. If those primitives must change after measurement, the main session must explicitly expand the caller check list first.
- Task-owned production component build + fresh Chromium context + loopback ephemeral static listener and mocked API meet the user's verification constraint. Do not start user Chrome, run make up/gc/down/destroy, submit real tasks or represent this as Electron/native/screen-reader/physical-touch acceptance.
- Initial `worktree-baseline.json.gz` preserves existing files but does not include newly dirty target files that changed after capture. Add a current target input snapshot; later baseline hash drift may be concurrent external work, not permission to revert it.

## Current target hashes observed during review

These are point-in-time hashes; concurrent changes can legitimately update them. The main session should recapture immediately before dispatch.

```text
packages/views/collaboration/creation-form.tsx
68656b0c6f124e4b6286c83149fecf984534f5420ed61e21c519f77daa179ca6
packages/views/collaboration/field-inputs.tsx
03110960e3fcde9fd44b12b2001b99ca37e81532e65c533385d83261ebba0404
packages/views/collaboration/creation-form.test.tsx
2b9b89bf86799a6dbd6993caa7d50e5abf5087cefd74e65195f13ab0edc5476e
packages/views/collaboration/field-editing.test.tsx
8b4ca9f5f716ea5256cf44ef91b12df2bdbf4390ab3245976a5453994cfb89ae
```

## Sources and caveats

- `.trellis/workflow.md`: plan artifacts/review before start, research isolation and native subagent context protocol.
- `.trellis/spec/core/frontend/collaboration-workbench.md:41–80,141–155`: original-intent recovery, actor/source correlation, settled rejection, opaque recipient projection.
- `.trellis/spec/views/frontend/quick-create-actor-picker.md:33–61`: committed candidates, typed versus external reset, paging, IME and focus.
- `.trellis/spec/ui/frontend/component-guidelines.md:50`: browser geometry proves layout, class strings do not.
- `triage-create-dialog.tsx:327–335,454`: header/body/footer and scoped close targets are existing patterns, but its pending-close policy is not automatically the policy for this repair.
- Existing normative WAI-ARIA links were supplied in ui-boundaries.md; this review does not repeat that fetch or claim real assistive-technology speech.
- No product/spec/other-task writes, git operation, test run, API call, listener or browser launch occurred during this review. Only this review document was written.

## Follow-up readiness gate

### Confirmation pass

The revised design now records inherited source-picker changes and exact pre-implementation input artifacts (`design.md:11–13`), current portaled parent search styling (`:37`), and pending-state admission across raw/debounced mismatch (`:53–55`). The revised implementation plan records preservation at step 1, hides stale rows during the delay and keeps existing late-response assertions at step 3.5, and uses five existing Core suites (`:54`), including the additional valid draft.test.ts.

`research/pre-implementation/manifest.json` and `inherited.diff.gz` exist. This researcher did not generate or run their git source; the main session owns those snapshots and should recapture changing target bytes before implementation dispatch if another writer remains active.

Reviewed revised artifact hashes:

```text
design.md
f01bae867b40637363920c1c80ee879d8856ddf0229d1142586e68d4b8cc498b
implement.md
9530aeb12dd75fd3ee2fe025ad18866036b7a8700e1b11ec80b469615549d202
```

B1/B2/C1 are resolved in planning. Keep the named immediate ArrowDown/Enter-or-pointer-before-250ms scenario explicit in the regression checklist; the main session has accepted it as required before task start. There are no remaining architecture or product-decision blockers. Implementation, browser measurements and independent trellis-check remain required completion gates; READY is a plan verdict, not a claim that fixes or checks have passed.
