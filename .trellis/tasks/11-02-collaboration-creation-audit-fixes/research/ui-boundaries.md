# Research: creation UI boundaries

- Query: Minimal shared-boundary repair for recipient semantics, visible focus, touch targets and long-form actions, preserving other callers.
- Scope: mixed (repository source/specs and normative WAI-ARIA guidance).
- Date: 2026-11-02.
- Write scope: this research file only; no product, fixture, process or git changes.

## Findings

### 1. Recipient semantics can preserve the native textbox contract

`PropertyPicker` is a generic Popover shell, not exclusively a listbox. Its built-in search remains a native text input named `Filter options` (`property-picker.tsx:267`). Its scrolling children container owns keyboard candidate discovery (`:146`, `:152`, `:286`); the footer is deliberately outside it (`:129`, `:287`). `CollaborationAccountPicker` currently supplies a named `listbox` inside those children (`account-picker.tsx:140`) but `PickerItem` renders a button without option semantics (`property-picker.tsx:327`). The input has no controls or active-descendant relation.

Recommended opt-in contract:

- Add an optional controlled-listbox ID to `PropertyPicker`; leave non-opted-in search roles and behavior unchanged.
- `CollaborationAccountPicker` allocates a per-picker stable listbox ID with `useId`, supplies it to the named listbox and PropertyPicker, and sets `aria-multiselectable` for multiple-account selection.
- Opted-in search retains the native textbox role and existing `Filter options` name, gains `aria-controls` referencing that listbox, `aria-haspopup="listbox"` when appropriate, and `aria-activedescendant` referencing the actual mounted active option. Do not add `aria-expanded` to a plain textbox merely to mimic a combobox; a true APG combobox would require the combobox role and a broader selector change.
- Add small optional ID/option semantics to `PickerItem` so account rows render stable `id`, `role="option"`, `aria-selected={selected}`, and `aria-disabled={disabled}`. A scoped `className` prop is useful for this surface's touch/focus styles; do not globally resize rows. Keep the actual native button and existing click semantics.
- Base `aria-selected` on the committed selection, not the navigation highlight. Expose navigation through `aria-activedescendant`; keep the existing checkmark and other selected-state visual signal. Arrowing alone must not change the recipient or selected set.
- Option IDs should include the picker instance and account identity, avoiding index-derived identity and collisions between simultaneous pickers. Account identity is already normalized by `accountIdentity` (`account-picker.tsx:40`). A keyed PickerItem-local `useId` is another safe stable option identity if it remains stable across rerenders.
- If row option buttons use `tabIndex=-1` in this composite pattern, keep search/header/footer controls tabbable and verify selection by Enter plus pointer; do not change other callers' tab behavior.
- Decorative avatar/checkmark content must not obscure account identity or introduce focusable descendants. ActorAvatar is noninteractive (`packages/ui/components/common/actor-avatar.tsx:55`); an explicit option name or a decorative avatar wrapper avoids repeated initials/name in the accessible option name.

Normative source retrieved during this research:

- [WAI-ARIA 1.2, aria-activedescendant](https://www.w3.org/TR/wai-aria-1.2/#aria-activedescendant) explicitly permits this relation: “The element with DOM focus is a combobox, textbox or searchbox with aria-controls referring to an element that supports aria-activedescendant, and the value of aria-activedescendant refers to an owned element of the controlled element.” It also says the active descendant should remain visible or scroll into view.
- [APG Combobox Pattern](https://www.w3.org/WAI/ARIA/apg/patterns/combobox/) describes the distinct combobox-role pattern, input/popup association and maintained DOM input focus. We intentionally preserve a textbox controlling a listbox for this scoped repair, rather than claim APG combobox-role conformance.

This recommendation was accepted by the parent session after reviewing fixture compatibility. It satisfies the audit's missing option/selected/active relationship while preserving existing native textbox selectors.

### 2. Reuse current keyboard bookkeeping; do not replace it

`paintHighlight` owns the numeric active position and visual highlight (`property-picker.tsx:160`). `syncNavigation` resolves committed DOM candidates (`:171`); it runs in a layout effect (`:189`) and at keydown (`:216`) to protect against a child layout effect dispatching Enter before the shell's effect. Typed search gives first-real-result selection priority; external changes invalidate stale index and the unique-result fallback (`:173–185`). Append-only display pages can intentionally preserve an active row. All these paths must synchronize or clear active-descendant state as well as paint.

The existing `isImeComposing` guard protects Enter/arrows including Safari 229 (`:211`). Search clears on the actual open-to-closed transition (`:193`), not only the Popover onOpenChange callback. Preserve this transition reset and clear any active ID when closed, empty, disabled or when an active candidate is removed. Keep `scrollIntoView({block:"nearest"})` on arrow navigation (`:225`, `:231`). The existing footer actions must never become arrow-selectable options.

Relevant canonical specs:

- `.trellis/spec/views/frontend/quick-create-actor-picker.md:33–61`: typed search vs external reset, append-only paging, in-popup focus after an auxiliary control is removed, first Escape dismissing only the picker, actual primitive tests and unchanged non-opted-in callers.
- `.trellis/spec/views/frontend/triage-create-form.md:77`: rendered geometry for scroll/footer and scoped 44px touch controls.
- `.trellis/spec/ui/frontend/component-guidelines.md:50`: real geometry, not class-string assertions, proves layout.
- `CLAUDE.md`: business UI in views, UI primitives stay independent, semantic tokens, role-named type scale and one canonical layer for each test behavior.

### 3. The shared shell has mixed callers; do not globally assign listbox roles

Callers include issue status/priority/stage/project/assignee/actor/custom-property pickers, collaboration labels, quick-create actors and chat actors. The issue label picker includes a create-and-attach action among its children (`label-picker.tsx:282`), plus a Manage footer (`:250`). Headers can contain filter and other action buttons. A blanket listbox/option conversion changes semantics outside this task and can incorrectly mark actions as options.

Existing tests that should remain green after the optional shared change:

- `packages/views/issues/components/pickers/property-picker.test.tsx`: stale/external/early Enter, typed search priority, append-only preservation, unique fallback without opt-in, IME and ref forwarding.
- `assignee-picker.keyboard.test.tsx`, `stage-picker.test.tsx`, `projects/components/project-picker.test.tsx` and `.open-state.test.tsx`.
- `modals/quick-create-actor-picker.test.tsx`: nested dialog/Escape and focus transitions.

Other-session fixtures/call sites must remain untouched:

- `e2e/fixtures/collaboration-recipient-picker.mjs:5–15` (currently untracked, another session's change): button named `Recipient`, listbox named `Recipient`, popup textbox named `Filter options`, exact-lookup actions and the exact `Selected account: <name>` text.
- `e2e/fixtures/collaboration-offline-handoff.ts:111`.
- `e2e/collaboration-boundaries-web.spec.ts:483`.
- `e2e/collaboration-recipient-picker-web.spec.ts:91`.

The selected-account identity supplement should be a separate span/line while preserving the existing exact name sentence as its own text element. Otherwise exact-text fixture checks may fail even when the named controls remain stable.

### 4. Scoped solid focus styling is smaller than changing global tokens

The existing light `--ring` is `oklch(0.705 0.015 286.067)` (`tokens.css:206`); dark ring is `oklch(0.552 0.016 285.938)` (`:297`). All of Input, Textarea and Button apply `focus-visible:ring-ring/50`, not an opaque ring (`input.tsx:12`, `textarea.tsx:10`, `button.tsx:9`). PropertyPicker's native search uses `outline-none` and no replacement focus styling (`property-picker.tsx:281`). The shared base default is also `outline-ring/50` (`base.css:660`). Merely nudging the global ring token to a value that clears 3:1 at full opacity does not establish that /50 controls pass.

An existing semantic non-text token already provides an appropriate neutral contrast floor: `--faint-foreground` is pinned at light L=.606, dark L=.60 (`tokens.css:187–198`, `:286–290`), with the comments documenting worst-surface 3:1 coverage. Reuse this token for an opaque focus ring and focused border/outline on the creation surface instead of mutating global tokens.

Static, predictive sRGB calculations against raised-dialog backgrounds, **not browser measurements**:

| Theme | Existing solid ring | Existing 50% composite ring | Solid faint-foreground |
|---|---:|---:|---:|
| Light white surface | ~2.63:1 | ~1.55:1 | ~3.87:1 |
| Dark raised surface | ~3.46:1 | ~1.82:1 | ~4.22:1 |

Scope form focus overrides to creation controls; override both color and alpha, including focused input borders where applicable. Portaled PropertyPicker content is outside the DialogContent DOM subtree, so a dialog descendant class alone cannot cover picker search/options/exact lookup/footer. Apply a small opt-in content class or explicit local focus classes there too. Tailwind `cn` merges same-property variants on direct primitive class overrides; descendant override geometry/contrast still needs actual compiled CSS verification.

Avoid changing global Button/Input/Textarea or all Dialog close buttons if local classes/descendant styles satisfy this task. These primitives are used throughout the app and global changes would materially enlarge review and verification scope.

### 5. Touch and scroll can reuse a known local form pattern

Current recipient row height comes from `py-1.5` plus 20px body leading (`property-picker.tsx:334`, `tokens.css:114–115`), typically 32px. Add `pointer-coarse:min-h-11` to recipient option hit targets through the optional local class. The identity supplement can grow row height naturally; use `min-height`, not a fixed height, to keep long content accessible.

The stock Dialog close is a Button `size="icon-sm"` (28px) (`dialog.tsx:67–76`, `button.tsx:54–55`). A local DialogContent descendant override for coarse close `min-h-11 min-w-11` preserves the 16px glyph. Reserve header space for the new 44px hit target to avoid title/help overlap.

`CollaborationSourceInputs` supplies its two audited short inputs without local size classes (`field-inputs.tsx:107`, `:117`). Add coarse minimum height there; the selects already have `min-h-11` (`:11`). This component is also used by collaboration source editing, so field-editing tests are relevant.

Current CreationEntry scrolls the entire DialogContent (`creation-form.tsx:29`), so header/close and submit travel with a tall form. Existing `triage-create-dialog.tsx:327–335,454` is a minimal repository-backed structure:

- DialogContent flex column, `max-h-[90dvh]`, `overflow-hidden`.
- Header `shrink-0`, title/help grouped with sufficient close-button clearance.
- Body `min-h-0 flex-1 overflow-y-auto overscroll-contain`, with padding so focused rings are not clipped.
- Actions `shrink-0` outside the scroll body, including a localized cancel/close button and submit.

CreationPanel is reused standalone in component tests and account/workbench surfaces. Keep it functional without a Dialog context. A dialog-specific optional `onCancel` passed through or a form-native action arrangement is safer than rendering unconditional DialogClose inside the standalone panel. Retain native form association and required-field validation; do not move submission to a click-only path. Use normal-flow header/footer, avoiding overlapping absolute actions or new scroll observers.

## Required evidence and regression shape

Component behavior:

- Recipient listbox ID/control relation; mounted option count, selected/disabled values, per-instance stable IDs, active-descendant after ArrowDown/ArrowUp/typed filtering, cleared on empty/removal/close, and no selection during composing Enter/arrows.
- Actual selection via Enter and pointer; max-selected and members-only behavior unchanged; multi-select uses the proper listbox attribute.
- Keep async cached rows/retry paging, exact lookup, escape/trigger focus and original Filter options textbox contract.
- Optional PropertyPicker API leaves non-opted-in callers' native roles, keyboard fallback and refs untouched.
- Existing `creation-form.test.tsx`, `account-picker.test.tsx`, `field-editing.test.tsx` remain the shared form regression layer; augment only named missing behavior, rather than duplicate the core original-key/scope matrices.

Browser geometry/interaction:

- Light/dark after transition settle; collect actual focused border/ring colors and alpha-composite against adjacent surfaces (source, title, description, parent/stage, recipient trigger/search/row, submit and close).
- Desktop and 390×740 / 320×568; no horizontal overflow; submit and close/cancel in the initial view and after long content/expanded initial fields scroll; Tab traversal keeps each focused control within the body viewport and outside header/footer occlusion.
- Coarse pointer/touch: rows, close, search and stage at least 44px; click centers and near target edges; picker opens, intended option selects, close works at bottom without forced clicks.
- Nested Escape closes picker first; later Escape closes form and returns focus; active option relationship is visible in DOM/AX.
- Include one unaffected shared picker and dialog in checks if shared code changes; CSS geometry assertions must use measured boxes rather than class matching.

## Caveats / Not Found

- Several UI/component/quality spec files still contain placeholders. Source-backed contracts above come from active quick-create/intake specs and CLAUDE.md, not inferred missing rules.
- No native assistive-technology speech, physical touch, iOS Safari or Electron Main acceptance was performed by this research. Planned isolated Chromium/DOM/AX evidence does not establish those boundaries.
- No preview or live service was started, no API request was sent, and no existing fixture was edited.
- Scoped focus styling must cover portaled content explicitly. A changed global token or dialog descendant selector alone is insufficient proof.
