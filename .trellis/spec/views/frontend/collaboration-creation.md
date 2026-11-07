# Collaboration creation interaction

## 1. Scope / Trigger

Shared Web/Desktop `CollaborationCreationEntry`, its standalone panel and account
picker. Changes to async completion, picker semantics, native form layout or
portaled focus must preserve Core's original-intent and access contracts. Mobile,
live API acceptance and Electron Main are separate verification surfaces.

## 2. Signatures

- `CollaborationCreationPanel({ scope, sourceWorkspaceId?, onCreated, constrained?, onCancel? })`; standalone defaults remain unconstrained without a Dialog dependency.
- `PropertyPicker.listboxId?` and `contentClassName?` are generic opt-in relations/styles.
- `PickerItem.optionId?` enables `option`, selected/disabled semantics and composite tab order; `ariaLabel?` must include visible identity when it overrides descendants.
- Core `executeCollaborationCreation(client, scope, draft, recovery)` continues to own correlation, original keys, retirement and source/API/viewer fences.

## 3. Contracts

Source selection is locked by current-scope pending `create`/`recover` mutations
and applicable contentful unresolved creation/upload records. Core retires a
successful bundle before the view callback: records alone leave a gap. Idle
`input:null` recovery must not permanently lock other sources; its active GET
does. Keep the form's active/current-scope success guard.

Recipient search remains a localized native textbox controlling a named listbox
through `aria-controls`/`aria-activedescendant` (per WAI-ARIA 1.2). IDs combine
picker instance and normalized user ID. Selection and keyboard activity differ;
clear dangling active IDs, skip disabled items, preserve IME/229, Escape, typed
search priority and append-only paging. Ordinary picker actions keep their roles.

Only `user_id`, name, avatar and permitted external status are projected. Full
IDs distinguish equal names and equal UUID prefixes in rows, exact results,
selected triggers and accessible labels; wrap them, do not truncate the identity.
Retain the existing selected-name sentence as a separate text element.

Use opaque `faint-foreground` ring/border/outline locally. Portals need explicit
content styles. Tailwind `outline-none` keeps an outline-style override: width and
color alone do not draw an indicator; pair them with `outline-solid`. Do not
claim contrast from a token when the indicator is absent or alpha-composited.

Constrained dialogs retain normal-flow header/source/actions and scroll only the
body. Native submit/required validation stays intact. Settled mutation feedback
reveals its specific reason or recovery-action group without focusing or sending
requests; refresh/edit must not repeatedly grab scroll. Contained-focus handling
keeps controls/rings visible and excludes React portal events with a DOM contains
check. Standalone layout is preserved. Coarse audited targets are at least 44px.

Parent input updates immediately, queries use a trimmed 250ms debounce with
unmount cleanup. Raw/settled mismatch is pending: old rows cannot be selected
during the delay. Keep Query keys, AbortSignal, permission refresh, authorized
transient fallback, retry focus and authoritative-denial teardown.

## 4. Validation & Error Matrix

| Condition | Required result |
|---|---|
| Confirmed typed refusal | Specific localized cause/action, preserved draft/files, no automatic POST |
| Quota refusal | Explain limit/admin action; reason actually visible after top-of-form submit |
| Unconfirmed result | Retain original immutable request; explicit original GET/POST only |
| Source/account/API denial | Fence stale completion and private data; no unconditional cached fallback |
| Empty/removed/closed option | No dangling active descendant; no unintended selection |
| Debounce gap | Loading/pending candidates, no stale Enter/pointer commitment |

## 5. Good / Base / Bad Cases

- Good: delayed creation retains its owner, completes once and opens the original issue.
- Base: no sources/loading/error still permits Cancel; standalone panel keeps native submission.
- Bad: a visible fixed Submit with a hidden refusal, or an opaque color on `outline-style:none`, is not a completed repair.

## 6. Tests Required

Core delivery/access suites own receipt/epoch/storage matrices. Creation/account
component suites own source locking, original recovery, full identity, ARIA/IME,
quota plus upload preservation, native submission and debounce. Generic picker,
assignee, stage/project and quick-create suites protect opt-in compatibility.

Real browser rectangles/text ranges and painted rgba prove 44px targets, full
identity, visible reasons/actions and Tab focus at 390×740/320×568 in both themes.
Wait for transition settlement. BODY overflow is propagated to the viewport;
do not falsely clip absolute portals to the landing page's content-height box.
State isolated component/mock API/DOM-AX/synthetic touch separately from real
submission, physical devices, spoken assistive technology and Electron Main.

## 7. Wrong vs Correct

```tsx
// Wrong: retirement can precede this callback while a source change unmounts it.
const locked = pendingRecords.length > 0;
// Correct: include scoped mutation state while retaining all existing fences.
const locked = scopedCreationIsPending || hasContentfulUnconfirmedIntent;
```

See [original-intent contracts](../../core/frontend/collaboration-workbench.md)
and [shared navigation/IME contracts](./quick-create-actor-picker.md).

## Recipient search commitment

The account picker must not publish an option while the trimmed input differs
from the debounced query or while Query is presenting the previous query's
placeholder data. A typed search filters locally pinned self/selected/recent
accounts too; the empty query retains the self/recent browse shortcuts. Keep
same-query authorized cached rows available on background refresh errors.
Keyboard Enter, pointer selection and pagination share this ownership rule;
IME composition, exact lookup and full account identity labels remain intact.
`account-picker.test.tsx` owns delayed-search and keyboard/pointer regressions.

## Composer layout and attachment focus

Creation uses the single-surface composer with recipient and AI participation
after title/description in the scrollable body, outside advanced fields. The
source context and footer remain in normal flow. Only the attachment entry,
draft notice and cancel/create actions belong in the fixed footer: file lists,
sharing help, upload progress, retry/reselect controls and long errors scroll
with the body. A 32-file selection must not grow the fixed footer.

Splitting attachment presentation must retain one upload owner. Optional
composition props may place the trigger in the footer and details in the body;
do not mount a second picker, add another store, or portal form actions. When
compact mode hides its native file input, lost-focus recovery targets the
visible attachment button. Default callers keep the visible native input as
their recovery target. Never steal focus from another live control or leave an
invisible file input in the keyboard tab order.

Base UI may restore a disappearing control's focus to the modal root instead
of `document.body`. Treat only the owning dialog's root fallback as lost focus
for compact attachment settlement; unrelated dialogs and live controls remain
untouched. Coordinate initial modal focus explicitly for touch as well as
keyboard opens, and avoid fighting asynchronous source-option rendering.
For explicit compact upload cancellation, hand focus from the focused Cancel
button to the persistent attachment trigger before abort removes Cancel. A
post-settlement recovery alone can lose to Base UI's later restore-focus frame.
Only perform this handoff when Cancel owns focus; preserve another live target.

Compact recipient presentation still displays the full account ID, permitted
external marker and independent selected-name sentence. AI and selected-file
sharing explanations remain available. The advanced-fields summary exposes
retained user choices without counting revision/identity metadata; invalid
values still open the disclosure automatically.

Visual similarity does not change keyboard contracts: creation title Enter
submits natively, description Enter inserts a newline, and recipient/IME Enter
must not accidentally submit. Do not copy ordinary-task shortcut hints unless
the shortcut is implemented. Regression coverage includes compact and default
attachment focus after cancellation/recovery, no focus stealing, advanced
summary/invalid values, complete recipient identities and the keyboard paths.

Account popup content must fit the positioner's available anchor space, not
just the viewport's total height. Keep its list shrinkable/scrollable so a
320px layout exposes complete distinguishing IDs before selection. Verify
actual text ranges after scrolling and focus rings inside portaled content.
Long translated breadcrumbs must not consume the source selector's usable
width. A missing-title explanation belongs next to the title or visible
actions, not at the far end of the scrolling form.
