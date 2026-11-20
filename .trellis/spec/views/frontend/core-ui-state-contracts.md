# Core UI state, naming and recovery contracts

## Scope / trigger

Use this contract for shared task properties, detail sidebars, squad instructions,
collection actions, and their query recovery. The existing desktop interaction
contract remains in [desktop-core-accessibility.md](./desktop-core-accessibility.md).

## Signatures and owners

- `PropertyPicker` can own a named listbox when both `listboxId` and
  `listboxLabel` are supplied. `multiple` declares multi-selection. Existing
  `listboxId`-only consumers continue to own their child listbox.
- `PickerItem.optionId` opts a row into option/selected semantics. A toggle row
  outside a listbox uses the existing `toggle`/pressed contract.
- `AnimatedRightSidebar.returnFocusRef` identifies the visible opener. The
  retained body owns focus tracking, inert state and live reduced-motion values.
- `PopupVisibilityProvider` in UI is default-visible and gates only descendants
  of a retained surface. Native Base UI actions own ordinary popup closure; Select has the documented scope-only exception below.
- `squadDetailOptions(wsId, squadId)` and `squadMembersOptions(wsId, squadId)` own
  protected squad snapshots in Core. `SquadQueryFeedback` renders view feedback
  from the original Query result, without new fetches or copied server state.

## Contracts

### Retained sidebars and portals

A hidden detail sidebar is inert and excluded from the accessibility tree.
Pointer suppression and opacity are insufficient. Close owned body portals too,
including menus, previews, tooltips, Select lists and modal sheets/dialogs. Suppress delayed
popup final-focus behavior while hidden so it cannot restore a hidden anchor.
Only return focus if the closing surface owned it; preserve deliberate focus
moves to unrelated content.

Uncontrolled Base UI roots remain uncontrolled for their lifetime. Hide by
calling their native actions and gating portal interaction, never by alternating
`open={undefined}` and `open={false}`. Controlled parents may reject closure while
saving; their hidden popup must still be absent, and reopening the surface must
not resurrect a dismissed popup. Keep public actions refs intact. The scope's
default-visible callers retain existing behavior.

`Select` is the explicit exception: Base UI exposes only `unmount()` for it,
not a native close action. Only inside `PopupVisibilityProvider`, its wrapper
owns the open lifecycle consistently from mount, resets it on hide, and
suppresses a still-true controlled request until acknowledgment or a new trigger.
Value/defaultValue and native event details remain unchanged; callers outside
an explicit scope still use the native Root directly. Sheet uses the ordinary
Dialog close-action path. A nested Select leaves with its owning Sheet too.

Reduced-motion changes must stop an already running content spring with
`MotionValue.jump()`. CSS removal of outer flex-grow transitions alone leaves
the inner transform animation active.

### Picker semantics and stable input names

Selection and keyboard highlight are different facts. Expose the selected row
with `aria-selected`, and the keyboard candidate with `aria-activedescendant`.
A non-search list has its own keyboard focus; a search picker keeps focus in its
named search field. Home/End belong to the text caret inside a search field.
IME retains its own Enter/arrows. Fixed empty-value rows remain outside the
search default, while explicit arrows can still reach them.

Do not put profile links or remove buttons inside an option. Multi-select
pointer activation keeps a persistent focus target. Retry must focus a retained
search field before disabling/removing itself, so Escape remains in a nested
popup rather than reaching the parent dialog.

Pass a localized, stable accessible name to the actual `ContentEditor` root.
A named static composer shell or placeholder alone does not name its mounted
textbox. Disclosure controls expose `aria-expanded` and a stable controlled
region; inactive content can still unmount while an empty hidden target retains
the relationship. Use shared Tabs for manual activation and panel association.

### Squad reads and drafts

A missing response is unknown, not an empty roster or unavailable runtime.
Initial errors expose Retry; successful empty arrays alone justify empty copy.
Cached transient errors retain the same form/editor nodes. A cold retry retains
its error control while pending, using Query timestamps as in settings recovery.

Protected reads clear the exact Query snapshot on definitive access loss;
detail/members also clear on 404; the squad directory retains its existing 404 behavior. Use the Query captured before awaiting and
`setData(undefined, { manual: true, updatedAt: 0 })` so cancellation rollback
cannot restore denied data. Ignore cancelled and stale-auth failures. An
unrelated query or newer replacement must not be cleared.

Squad instruction state separates the last server snapshot from the unsaved
local document. A remote update adopts content only while clean. Dirty content
survives and receives a visible conflict notice. A successful save publishes the
actual response into the cache keyed by its response workspace/squad identity
before refetch; a slow refetch must
not reinstate the old text. New local typing during a save and failed submissions
remain available. The editor's emission debounce is not proof that the server
has saved the draft.

Read-only instructions use `ReadonlyContent`, not an editable editor behind
`pointer-events-none`. Losing management authority hides mutating controls and
invalidates pending confirmations; recovery cannot silently reopen them.

## Validation and error matrix

| Situation | Expected result |
| --- | --- |
| Sidebar closes with property popup focused | Popup and sidebar leave AX; visible opener receives focus |
| Focus already moved outside before closure | No focus stealing |
| Hide/show an uncontrolled popup | Closed on show; a new trigger can open it |
| Controlled popup refuses close | Hidden content remains unreachable; no stale resurrection |
| Reduced motion enabled mid-spring | Geometry/content reach final state without remaining motion |
| Search then arrow/Enter | Announced candidate matches submitted option; IME unaffected |
| Retry disables its focused button | Focus remains in retained search; first Escape closes only picker |
| Squad refresh changes instructions while dirty | Local text stays; conflict feedback appears |
| Save succeeds but refetch is delayed | Confirmed saved text remains; later local text survives |
| Roster cold failure | Named error/retry, no false zero count |
|403/404 then500 or retry cancellation | Denied exact snapshot stays absent |

## Good / base / bad cases

- Good: the selected status stays selected while arrows announce another candidate.
- Base: opening an ordinary popover outside a visibility scope behaves as before.
- Bad: replacing local instructions with every query update because onUpdate fired.
- Bad: a visually hidden side panel still exposes its portalled popup to Tab.

## Tests required

- Real Base UI primitive tests for sidebar popup lifecycle, controlled rejection,
  default-open behavior, focus ownership and live motion preference changes.
- PropertyPicker and consumer tests for named options, initial/active focus,
  multi-selection, IME, empty values, external reset and nested Retry/Escape.
- Component-host tests for emitted-but-unsaved drafts, cached failures, save
  rejection, pending refetch and newer edits. The editor suite owns debounce
  serialization timing; do not duplicate that implementation matrix in hosts.
- Core query tests for 401/403/404, cancellation and stale snapshots with a real
  QueryClient. Native AX/keyboard checks separately confirm visible behavior.

## Wrong vs correct

```tsx
// Wrong: visual hiding leaves the tab order and body portals alive.
<div className={!open ? "pointer-events-none opacity-0" : ""}>{children}</div>

// Correct: retained surfaces use the shared owner for focus and portal closure.
<AnimatedRightSidebar open={open} returnFocusRef={toggleRef}>
  {children}
</AnimatedRightSidebar>
```
