# Component Guidelines

> How components are built in this project.

---

## Overview

<!--
Document your project's component conventions here.

Questions to answer:
- What component patterns do you use?
- How are props defined?
- How do you handle composition?
- What accessibility standards apply?
-->

(To be filled by the team)

---

## Component Structure

<!-- Standard structure of a component file -->

### Convention: Settings page navigation IA lives in `SETTINGS_NAV_GROUPS`

**What**: The settings page's information architecture (groups, items, order, icons, feature-flag gating) is owned by a single declarative structure in `packages/views/settings/components/settings-nav.ts`. `settings-page.tsx` only renders what `visibleSettingsNavGroups()` resolves — it must not carry its own tab arrays.

**Why**: The `?tab=` query value is a **public URL surface**, consumed by two parties outside this file:

- User bookmarks / shared links (`/settings?tab=issue-statuses`).
- The Go backend: `server/internal/handler/github.go` (`githubSettingsURL`) redirects the GitHub App install callback to `/settings?tab=github`. The frontend keeps that URL working via `LEGACY_WORKSPACE_TAB_REDIRECTS: { github: "integrations" }`.

**Contract**:

- Each static item resolves its tab value as `value ?? key`. Renaming a `?tab=` value breaks every bookmark and the backend callback — treat it as a breaking URL change, not a refactor.
- New `LEGACY_WORKSPACE_TAB_REDIRECTS` entries are allowed **only** for backend-driven URLs (a Go handler builds them). Internal links get edited at their source instead; never add a redirect for a link we control.
- Group and item order is part of the design contract. The canonical ordering matrix is pinned in `settings-nav.test.ts` (node environment, no DOM); `settings-page.test.tsx` asserts only headings/wiring and points at that file.
- Every group id needs a key under `page.groups` and every item key under `page.tabs` in **all four** locale files — `locales/parity.test.ts` fails otherwise.
- A group whose items are all flag-filtered must not render its heading (handled by `visibleSettingsNavGroups`, covered by test).

**Wrong**:

```ts
// Adding a redirect for a link this repo owns:
const LEGACY_WORKSPACE_TAB_REDIRECTS = { issue: "preferences" }; // ❌ edit the link instead
```

**Correct**:

```ts
// Backend-driven URL only — comment must name the Go builder:
const LEGACY_WORKSPACE_TAB_REDIRECTS: Record<string, string> = {
  lark: "integrations",
  github: "integrations", // server/internal/handler/github.go, githubSettingsURL
};
```

**Tests required when touching the IA**: update the order matrix in `settings-nav.test.ts`; if a `?tab=` value changed, grep the repo (incl. `server/`, `e2e/`) for the old value and check `githubSettingsURL`-style backend builders.

---

## Props Conventions

### Squad discovery and template identity

The squad page keeps workspace instances separate from the template catalog.
`?view=templates` opens the catalog; the default URL opens saved squads. Preserve
unrelated query parameters and the hash when replacing the current view. A
template-instance link must return to the catalog through browser Back.

Match instances by `template_key`, never by the editable squad name. Concise
catalog summaries are display copy only: pass the original template into project
configuration and keep saved names, descriptions, and instructions untouched.
Use `AppLink` with `rowLinkInteractiveProps` inside clickable rows so keyboard
activation and modifier clicks do not invoke the row navigation fallback twice.

Column defaults apply to fresh preferences. Do not migrate an existing
`hiddenColumns` array when changing which provenance columns are initially shown.

<!-- How props should be defined and typed -->

(To be filled by the team)

---

## Styling Patterns

<!-- How styles are applied (CSS modules, styled-components, Tailwind, etc.) -->

(To be filled by the team)

---

## Accessibility

<!-- A11y requirements and patterns -->

Interactive content supplied through a platform slot still inherits its React
menu context. Use `DropdownMenuItem` for actions inside `HelpLauncher` so arrow
keys, typeahead and dismissal work. A plain button can navigate successfully
while remaining absent from keyboard navigation and leaving the popup open.
Only `DropdownMenuLabel` (`Menu.GroupLabel`) requires a `DropdownMenuGroup`;
do not generalize that requirement to all menu parts. Verify menu actions with
the real primitives, as `sidebar-version.test.tsx` and
`e2e/desktop-settings.spec.ts` do, rather than replacing the whole menu with divs.

---

## Common Mistakes

<!-- Component-related mistakes your team has made -->

(To be filled by the team)
