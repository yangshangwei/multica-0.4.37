# Refine collaboration creation density and hierarchy

## Goal

Make the approved option C composer feel like a focused task editor rather than a tall form. The user's current screenshot shows excessive description whitespace, repeated labels/helper rows, an empty attachments section, a heavy full-width title focus rectangle and advanced fields partly below the fold.

## Confirmed scope and authorization

The user requested UI optimization with Impeccable and then explicitly approved a new optimization task. This is a bounded continuation of the already-approved option C, not a replacement workflow or product concept. The screenshot is the current visual feedback. Existing source and the archived composer task supply behavior contracts. No unresolved product decision blocks these reversible refinements.

## Requirements

- Keep title/description first, recipient and AI in the body, one attachment owner and fixed footer. Preserve native controls, full account identity, selected-name sentence, contextual AI/sharing explanations, draft/recovery and source locks.
- Reduce unnecessary vertical space: smaller empty description, tighter within-group spacing, a simpler empty attachment presentation and a compact footer that keeps draft status and actions readable.
- Give title a lighter editor appearance while preserving an opaque visible keyboard focus indicator. The required-title reason remains immediately visible and associated with the field/submit control.
- Keep the full account ID directly visible; compact presentation may use the control's full width for identity instead of forcing it into an avatar-indented column.
- Keep all attachment details/recovery states in the body. An empty composer need not display a bold empty attachments section, but sharing help remains visible before selection. Default attachment-picker callers retain existing presentation.
- At a representative 722x660 Chinese viewport with an empty draft, default self/AI and a reasonably long source name, the advanced-fields summary should be wholly visible without initial body scrolling. At 390x740/320x568 both themes/languages, content remains reachable, footer usable, identity untruncated and coarse controls >=44px.

## Acceptance criteria

- [x] Before/after captures show reduced empty spacing and lighter title focus with preserved hierarchy.
- [x] Default Chinese screenshot-sized surface exposes advanced fields without clipping; no document horizontal overflow.
- [x] Footer needs fewer rows at narrow English widths while preserving attachment entry, saved-draft notice, cancel and create.
- [x] Full recipient UUID and selected-name sentence remain visible, including same-name/long-identity states.
- [x] Empty attachments show sharing guidance without an empty bold section; selected/loading/error/recovery states still expose all necessary controls.
- [x] Existing collaboration+locale tests, Views typecheck/lint, scoped detector and real component browser checks pass.

## Out of scope

No API/storage/state-machine changes, no new dependencies, no rich-text editor migration, no ordinary composer redesign, no mobile or Electron Main changes, and no changes to keyboard/recovery semantics fixed by the previous task.

## Approved focus integration

During final verification another session supplied a guarded initial-focus correction and deterministic frame-order regression. The user explicitly approved including and verifying that patch. Extend the scope to prevent Base UI's deferred initial focus from overriding a user-selected live control while the source query resolves. Preserve normal title/source initial focus, close/opener return, and existing IME/upload recovery behavior.

- [x] The guarded initial focus and deterministic frame-order regression pass, and a focused real-browser delta run verifies asynchronous focus/lifecycle behavior before final commit.
