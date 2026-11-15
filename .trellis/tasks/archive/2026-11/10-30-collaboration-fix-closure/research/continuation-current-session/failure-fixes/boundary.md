# Focused v11 browser failure repair

The observed gap is confined to stale browser-fixture interactions and one stale
exact-message assertion. The current shared parent selector is a searchable
button picker. Ordinary issue setup returns an `identifier` and `title`, while
the selector's source projection displays that identifier and title as its
option name. The optional stage field is labelled `Subtask stage number
(optional)`. The creation-refusal UI already preserves the draft and displays
the specific cause separately; its established summary ends at `preserved.`.

Owned code boundary:

- `e2e/fixtures/collaboration-editing.ts`: click the real parent picker, choose
  the fixture's exact displayed identifier/title, and use the current stage
  label in creation and editing. Keep the persisted parent UUID, stage,
  recipient, label, property and iteration assertions.
- `e2e/collaboration-delivery-web.spec.ts`: retain the existing one-line exact
  summary sync. Keep the 403/result correlation, retained file, enabled input,
  explicit new-key resubmission and old-key rejection assertions.

No product component, API, permission, persistence or locale behavior changes
are needed. No new abstractions or dependencies are needed. Existing component
regressions establish the actual picker/stage and refusal-message contracts;
the root session owns the new production Web/Electron browser run and full gate.
