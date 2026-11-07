# Bounded visual refinement

## Change boundary and simplification plan

The smallest gap is presentation density in the existing shared Views composer. The semantic fields and recovery behavior are correct. Expected product scope: creation-form.tsx (spacing, title focus, description, header/footer and auxiliary copy placement), attachment-picker.tsx (empty composer legend treatment only), account-picker.tsx (opt-in compact identity layout). Local tests may change only for meaningful behavior regressions. Existing locales and copy are retained where possible. No new state, uploader owner, shared UI primitive changes or dependency.

Before edits, preserve the previous passing 349-test baseline and the current source hash evidence. Keep the user screenshot as visual truth; do not mistake the previous 94-point functional/visual verdict for user satisfaction. This pass removes redundant spatial grouping, not capabilities.

## Presentation choices

- Use an measured 34rem bounded composer height (subject to content geometry) and existing max-height viewport guard. Keep only the body scrolling.
- Bring title label and its short required explanation into one wrapping caption row, then use a larger role-token title with a 2px opaque bottom-border focus cue instead of a rectangular ring. Maintain input required, aria-describedby and focus-reveal behavior.
- Reduce the empty description minimum from 96px toward 64px while preserving field-sizing-content for longer text, label and visible focus.
- Use a consistent 16px outer group rhythm and tighter 4–12px within-group rhythm. Preserve the source/AI/sharing explanatory text; reduce its spacing and competing section weight.
- In compact recipient mode, allow full account identity to span the trigger's usable width beneath the name/avatar row. Default picker markup/semantics remain compatible; no shortened UUID or tooltip-only identity.
- Compact empty attachment details retain a semantically named group and visible sharing explanation, without a bold empty heading. When any file, loading, error, cancellation or unresolved upload exists, show normal named details and recovery UI.
- Footer groups attachment entry with draft status and keeps cancel/create together. On narrow screens give each group the full available row so English actions do not unnecessarily stack individually; wrap labels rather than clipping.

## Behavior and rollback

Do not alter initial focus, IME guards, upload cancellation's pre-abort focus handoff, query/mutation logic, form guards, draft restoration or recovery semantics. Revert only this task's product diff to restore the prior composer. Full identities and minimum target/focus contracts outrank density.

## Measured refinement

The first722×660 capture confirmed complete advanced-summary visibility but left71px below it. The final34rem shell reduces that gap to about39px. English320 actions needed248.03px with an8px gap versus248px available; use6px between actions. The source selector uses a256px desktop maximum rather than192px (clipped the supplied long source name) or unbounded flex width (separates the breadcrumb unnecessarily).

## User-approved concurrent focus correction

The existing Base UI initialFocus callback returned an element that the library focused unconditionally on a later animation frame. A user could focus the source selector before that frame, then lose focus to the title. The accepted patch returns false to suppress that unconditional frame, establishes the dialog root only while its opener still owns focus, and delegates deferred recovery to the existing lost-focus helper with a data-open guard. This keeps late source responses from taking focus from live controls or a closed dialog. The test deterministically queues animation frames to exercise the ordering instead of relying on timing luck.
