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
