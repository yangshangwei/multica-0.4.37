# Align collaboration task creation with the standard task composer

## Goal

Make the desktop/Web collaboration creation dialog feel like the existing
ordinary “New task” composer so users can create a cross-workspace task without
learning a second form language. The redesign must preserve the collaboration
workflow and its durable recovery behavior.

## Background and confirmed constraints

- The current surface is `packages/views/collaboration/creation-form.tsx` and
  uses the shared `Dialog`, `Input`, `Textarea`, picker, and attachment
  components.
- The ordinary composer in `packages/views/modals/create-issue.tsx` already
  establishes the product visual language: compact breadcrumb header, large
  borderless title/description editing, pill-like property controls, a compact
  attachment action, and a fixed footer.
- The current collaboration form has a long vertical journey, a full-width
  native file input, and a low-salience advanced-fields disclosure. Existing
  QA confirms draft persistence, source locking, recipient lookup, upload
  recovery, native submission, and constrained-dialog focus behavior.
- The collaboration creation contract in
  `.trellis/spec/views/frontend/collaboration-creation.md` remains authoritative:
  keep source/mutation locks, recipient ARIA/search behavior, scoped success
  guards, normal-flow header/source/actions, body-only scrolling, and 44px
  coarse-pointer targets.

## Requirements

### R1. Composer visual language

Use the ordinary composer’s hierarchy within the collaboration dialog:

- compact context/breadcrumb header with an accessible dialog title and close
  action;
- source workspace as a compact context control in the header area;
- borderless, prominent title input and roomy description editor;
- a compact collaboration section after the description, inside the scrollable body, with recipient and AI participation controls; selected recipients retain the complete visible account ID, external marker where permitted, and a separate selected-name sentence;
- a visible advanced-fields disclosure that retains all current initial-field
  controls, summarizes retained non-default values when collapsed, and opens automatically when a retained value is invalid;
- a compact attachment action in the fixed footer; selected files, sharing help, upload/recovery/error states remain in the scrollable body under one upload owner; compact-mode focus returns to the visible attachment action when a recovery control disappears;
- fixed footer actions with the saved-draft notice, Cancel, and the existing
  Create collaboration issue submit action.

### R2. Preserve behavior and data contracts

Do not change API payloads, query keys, draft storage, idempotency keys,
recovery actions, source access fencing, recipient lookup semantics, upload
semantics, or navigation after creation. Standalone unconstrained panel usage
must continue to work.

### R3. Accessibility and responsive behavior

- Keep every field associated with its existing accessible label, including
  compact controls whose visible text changes.
- Keep native form submission and required title validation. Preserve the deliberate difference from ordinary creation: title Enter submits here, description Enter inserts a newline, and recipient/IME Enter must not submit the form.
- Keep status/alert announcements and focus-visible styling, including portaled
  picker content.
- Keep the modal body independently scrollable while header, source context, and
  footer remain reachable; avoid horizontal overflow at narrow widths.
- Preserve at least 44px targets under coarse pointers.

### R4. Localization and consistency

Add any new composer copy to both `en` and `zh-Hans` collaboration locales and
keep locale key parity. Do not introduce a new dependency or a second state
store.

## Acceptance criteria

- [x] At desktop width, the collaboration dialog uses the composer hierarchy
  described in R1 and no longer presents the raw full-width native file input as
  the primary attachment affordance.
- [x] Title and description remain the first editing targets; recipient and AI
  policy remain visible without opening advanced fields; advanced initial fields
  remain available and retain their current values and validation behavior.
- [x] Source selection, recipient search/exact lookup, AI policy, attachments,
  retry/recovery, cancel/reopen draft persistence, native Enter submission, and
  post-create navigation continue to pass the existing collaboration tests.
- [x] Empty-source, loading/error, rejected creation, pending creation, upload
  recovery, and invalid-initial-field states remain reachable and correctly
  announced.
- [x] Compact upload activation and settlement/cancel/recovery return lost focus to the visible attachment action without stealing live focus; default callers retain native input behavior.
- [x] Recipient/AI help stays visible; complete same-name account identities wrap; advanced-field summaries expose retained non-default selections.
- [x] Browser evidence covers desktop plus 390×740 and 320×568 in both themes, long identities/files, 32 attachments, recovery and keyboard focus.
- [x] Narrow/coarse-pointer rendering keeps the body usable, footer actions
  reachable, controls at least 44px where required, and document/body free of
  horizontal overflow.
- [x] Both locale files pass parity checks; views typecheck, lint, focused tests,
  and `impeccable detect --json packages/views/collaboration` pass.

## Out of scope

- No API, database, collaboration state-machine, or mobile changes.
- No redesign of the ordinary task composer itself.
- No conversion of collaboration description content to the rich text editor;
  the existing plain textarea and payload format remain the source of truth.
- No new wizard step or desktop split-pane variant in this task; the selected
  direction is the composer-first single-surface variant.
- No changes to the existing attachment privacy, selection, upload, or recovery
  policy.

## Open questions

On 2026-11-07 the user selected audit option C and explicitly requested implementation of the existing task. This supersedes the earlier fixed-recipient-toolbar arrangement. No new task or further planning approval is needed.
