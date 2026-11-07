# Technical design

## Change boundary

The smallest behavior gap is visual and interaction hierarchy: collaboration
creation works, but its dialog presents a tall traditional form that diverges
from the product’s existing task composer. The behavior actually lives in the
shared collaboration view components, not in the server or desktop shell.

Expected product files:

- `packages/views/collaboration/creation-form.tsx` — dialog shell, constrained
  layout, title/description hierarchy, collaboration/disclosure placement, and footer
  composition.
- `packages/views/collaboration/account-picker.tsx` — optional compact trigger
  presentation for this form while leaving existing workflow/property callers
  unchanged.
- `packages/views/collaboration/attachment-picker.tsx` — optional compact
  attachment affordance that hides the native input behind a design-system
  button while retaining the same upload/recovery model.
- `packages/views/collaboration/focus.ts` — recover from the owning modal root after a focused upload control disappears, while retaining live-control and cross-dialog focus. Existing `form-styles.ts` remains the shared focus-style reference.
- `packages/views/locales/en/collaboration.json` and
  `packages/views/locales/zh-Hans/collaboration.json` — new placeholder/context
  copy, kept in parity.
- `packages/views/collaboration/creation-form.test.tsx` and, if needed,
  `account-picker.test.tsx`, attachment and focus tests — regression coverage for the
  preserved interaction contracts and compact controls.

Explicitly outside the boundary: server/core packages, ordinary task composer
implementation, mobile, database/API payloads, and unrelated existing working
tree changes.

## Layout and data flow

```text
CollaborationCreationEntry
  Dialog (existing open/focus lifecycle)
    DialogContent (composer-sized shell)
      context header + accessible DialogTitle + close (no new expand behavior)
      CollaborationCreationPanel
        source context row (normal flow)
        scrollable CreationContents body
          notices
          borderless title + description
          compact recipient (full identity + selected-name sentence) and AI section
          attachment selection, sharing help, progress, recovery and errors
          initial fields disclosure with retained-value summary
          settled creation feedback and errors
        fixed actions footer
          compact attachment trigger
          saved-draft notice, Cancel, Create
```

The form remains the owner of the existing `useCollaborationDeliveryForm`,
React Query mutations, and `CreationContents` focus/scroll handling. The fixed footer is presentational composition only. Attachment entry and body details share exactly one existing upload owner; use a narrow composition prop/slot if needed rather than duplicated mounts, another store, or portaled form actions.

## Compatibility choices

- Add an optional `compact` account-picker presentation and an attachment
  `renderComposer({ trigger, details })` composition prop instead of changing
  their default markup. Existing workflow, property-field, and test
  callers therefore retain their current presentation and accessible names.
- Keep the source `<select>` native and labeled. It becomes a compact context
  control rather than a custom picker so source locking and native keyboard
  behavior remain unchanged.
- Keep the initial-fields `<details>` and all existing field components. Only
  its summary treatment, retained-value summary and placement change; invalid retained values still
  force it open.
- Keep `CreationContents` body-only scrolling and keep footer slots in
  normal flow. Do not make the modal itself scroll or portal form actions.
- Use existing token classes (`bg-surface-raised`, `bg-background`,
  `text-muted-foreground`, `ring-ring`, `faint-foreground`) and existing
  `Button`/`Input` primitives. No new CSS framework or dependency.

## Error and recovery behavior

Notices and settled feedback stay inside the scrollable body so the existing
focus and `scrollIntoView({ block: "nearest" })` behavior remains intact.
Upload/retry/reselect controls stay mounted in the scrollable body in compact mode. When upload/recovery controls disappear, compact mode recovers lost focus to the visible attachment trigger; default mode retains its native input target, and a still-live focus is never stolen. Keep the native required attribute and submit guards; a visible title-required hint explains the disabled Create button without relying on that disabled button to trigger validation bubbles.

## Rollback

The change is isolated to shared views and locales. Reverting the product-file
diff restores the current form without a migration or data cleanup. Tests prove that opt-in compact/composer presentation preserves default callers; modal-root recovery remains limited to the target's owning dialog.

## Approved option C and visual contract

- Title and plain description form the primary editing surface. Use existing role-named text tokens, semantic surfaces, and ordinary-composer spacing/width as reference; do not blindly impose its 440px height.
- Recipient and AI live after description in the scrollable body, outside advanced fields. They form a light compact group, not stacked bordered cards. Long recipient IDs wrap in the selected trigger and candidates; the independent selected-name sentence and AI policy explanation remain.
- Fixed footer contains only attachment entry, draft notice and cancel/create. All file lists (up to 32), sharing explanation, progress/recovery controls and long errors scroll with the body.
- Advanced summary exposes configured non-default fields without counting structural revision metadata as user selections. Invalid values still force disclosure open.
- Source remains a native labeled control in normal flow. Known source should lead to title editing; source-empty/loading/error paths still expose Cancel.
- Do not add expand/maximize or copy ordinary-task keyboard shortcuts. Preserve native Enter submission, plain-text payloads and standalone panel behavior.
- Browser acceptance uses desktop, 390×740 and 320×568, both themes, coarse targets >=44px, no viewport horizontal overflow, visible focus, long identities/files, many attachments and recovery states. Evidence must separate component/mock verification from real backend, Electron Main, physical touch and spoken assistive technology.

## Browser correction rationale

The first current-source browser pass exposed concrete behaviors that DOM tests could not establish. The local correction batch uses the existing Core IME helper before native form submission; constrains recipient content by the Base UI positioner's available height; lets translated breadcrumbs wrap without collapsing the source selector; places missing-title help beside the title; and explicitly coordinates modal initial focus with asynchronous source readiness. Attachment settlement recognizes the owning modal root as automatic lost-focus fallback, while another dialog or a live control remains untouched.
