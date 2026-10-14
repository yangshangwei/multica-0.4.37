# Automation Template Discovery

`AutopilotTemplateCatalog` is shared by the empty automation list and the
template creation picker. Keep the populated management list independent of
the catalog, and do not change the generic built-in catalog for this feature.

- Detect first use from the successfully loaded, unfiltered automation list.
  Loading, errors and filtered-out instances are not an empty workspace.
- Presentation groups are local metadata keyed by template key. Preserve server
  ordering and keep unknown templates in All with conservative output wording.
- `run_only` means no issue is created up front. Some patrol templates can create
  or update follow-up issues. Do not promise that they never create tasks.
- Template adoption keeps the prompt, schedule and output mode server-owned.
  Navigation must retain project and assignee defaults; submission creates and
  enables only after the user confirms the configuration.

## Restoring a Two-Step Gallery

The picker and configure steps share a pathname. Desktop captures and replaces
plain scroll entries on every URL transition, including query changes, so
leaving configuration can erase the gallery's ordinary scroll offset.

Before selecting a template, write the gallery's group and scroll offset through
the existing platform view-state adapter. Preserve explicit All and zero values,
and restore only after template content is loaded. A URL containing stale group
or offset defaults must not override the latest saved selection on native Back.
On web, the platform adapter must key render-time reads and view-state writes
by Next's rendered pathname, not the mutable browser URL during navigation.
The empty-page entry uses the allowlisted `return_to=autopilots` marker so the
page's Back button returns to the same route that owns its saved gallery state.

`autopilot-template-catalog.test.tsx` covers restoration when the plain-scroll
adapter returns no entry. `template-create-autopilot-page.test.tsx` covers
origin routing and creation defaults. `e2e/autopilot-template.spec.ts` verifies
native and in-flow Back through the production web application.

## Discovery and focus lifecycle

### Scope / trigger

The catalog serves both the empty automation list and the creation picker.
Keep executor, timezone and default-schedule/editability guidance before the
cards. The recommended marker belongs to the title group, not a separate
footer row. Both entry points use the catalog's labelled blank action; compact
or coarse input keeps its 44px target. Keep local solid semantic focus styling
instead of changing the application's global ring token.

### Signatures

The existing shared restoration writer accepts an optional target pathname:

```ts
setViewState?(
  entryKey: string,
  value: string | undefined,
  routePathname?: string,
): void;
```

`useViewStateWriter()` forwards the same arguments. An omitted path preserves
the current-route behavior; an explicit path targets that route within the
adapter's tab. Pass a trusted pathname from the workspace path builder, without
query strings or a hash. Web defaults to the rendered pathname. Desktop explicit
writes use the adapter-bound tab, even if another tab becomes active.

### Contracts

- Before selecting a template, write `autopilot-template-focus` alongside the
  existing group/scroll entries.
- Restore only after real content is ready: scroll first, then focus the original
  card with `preventScroll`. If absent, use the active filter; if the registry is
  empty, use the blank action.
- Consume the pending focus once. Preserve a newer focus owner in another
  control/dialog, and never reapply the handoff on a later refetch.
- Key the create shell by semantic picker/configure phase. Its retained Back
  button otherwise blocks restoration; a click-only blur misses native Back.
- After a creation response supplies a valid nonempty automation ID, clear only
  the origin's pending-focus key before navigating. The origin is
  `paths.autopilots()` for `initialReturnTo === "autopilots"`, otherwise
  `paths.newAutopilotTemplate()`. Preserve group/scroll.
- Failed or unreadable creation responses retain the handoff. A fresh picker
  visit after confirmed creation must not focus an old template.
- Known patrol output copy may state conditional issue creation/update.
  Unknown templates/modes retain conservative fallback wording.

### Validation and error matrix

| Situation | Required behavior |
| --- | --- |
| Return through in-flow or native Back | Same category, scroll and original card focus |
| Templates still loading or failed | Wait; never restore against skeletons |
| Target removed, other templates remain | Focus active filter |
| Successful empty registry | Focus existing blank action |
| Dialog/another control acquired focus | Consume stale handoff without taking focus |
| Data refetch after restoration | No repeated focus |
| Confirmed creation from either origin | Clear only that origin's focus entry |
| Rejected or unreadable creation | Keep the origin entry |
| Another Desktop tab becomes active | Explicit cleanup still targets the owning tab |

### Good / base / bad cases

- Good: creation from the empty list clears its `/autopilots` focus entry while
  preserving the creation-picker entry, filters and scroll.
- Base: callers omitting the third argument keep existing route-scoped behavior.
- Bad: clearing only the configure route leaves the empty-list token alive.
- Bad: broadening the focus guard to override every active control fixes Back
  by breaking dialogs and deliberate user focus.

### Tests required

- `autopilot-template-catalog.test.tsx`: delayed data, one-shot consumption,
  explicit All/zero, removed target, empty registry and focus-owner protection.
- `template-create-autopilot-page.test.tsx`: keep the real page mounted across
  both Back paths; use route-plus-entry-key state to test confirmed, rejected
  and unreadable creation from both origins.
- Shared and Web `platform/scroll-restoration.test.tsx`: forwarding, unchanged
  defaults and explicit-origin isolation.
- Desktop `platform/tab-coordinator.test.ts`: owning-tab isolation, including
  cleanup while that tab is inactive.
- Browser evidence must measure actual focus contrast and touch target geometry.
  Native zoom is distinct from a smaller emulated viewport.

### Wrong vs correct

```ts
// Wrong: the current route can differ from the gallery that owns the token.
writeViewState("autopilot-template-focus", undefined);

// Correct: clear only after validating the creation response, on its origin.
writeViewState(
  "autopilot-template-focus",
  undefined,
  initialReturnTo === "autopilots"
    ? paths.autopilots()
    : paths.newAutopilotTemplate(),
);
```
