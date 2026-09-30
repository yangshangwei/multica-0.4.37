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
- Every group id needs a key under `page.groups` and every item key under `page.tabs` in **both supported** locale files — `locales/parity.test.ts` fails otherwise.
- A group whose items are all flag-filtered must not render its heading (handled by `visibleSettingsNavGroups`, covered by test).
- Labels belong to workspace administration because the page configures both task and skill labels. Platform-injected `extraDesktopTabs` belong to the final `desktop` group, which is absent on web when no entries are injected.
- Settings use one named vertical tablist with labelled groups. Below 768px, the same directory opens in a Sheet; focus starts on the selected entry and returns to the trigger after selection or dismissal. An open directory closes when the viewport switches to the desktop layout.
- A compact Sheet unmounts its tab triggers when closed. Give content panels an explicit accessible name so they remain named when Base UI cannot resolve a mounted tab's ID.
- The shared `Tabs` wrapper must forward `orientation` to the Base UI root. A `data-orientation` attribute alone changes styling but does not configure keyboard navigation or ARIA. Test real arrow-key behavior across settings groups, not only CSS attributes.
- Compact and coarse-pointer settings controls have a 44px minimum target height. Fine-pointer desktop rows stay dense, and long labels wrap rather than clipping.

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

Squad discovery is an explicit exception to ListGrid's usual two-zone layout:
members, creator and created-at columns appear at `@2xl`, `@4xl` and `@5xl`
respectively. Keep each header, row cell and skeleton placeholder at the same
tier, and explain the width requirement in the display popover. Preferences
remain saved even when a column is temporarily hidden by the container width.
Keep the narrow identity/leader layout free of horizontal overflow, including
44px coarse-pointer action targets. No JavaScript width observer is needed.

In the squad list, actor names and their decorative avatars form one AppLink.
Disable the nested ActorAvatar profile link and use rowLinkInteractiveProps so
keyboard and pointer activation reach the actor exactly once. Profile-card
details must remain visible without mouse hover. Active filters use a semantic
foreground/background pair and expose selected names plus result counts in a
localized status region; do not replace saved descriptions with template copy.

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

### Entity icon presentation

Projects also use `ProjectIcon` / `ProjectIconPicker` for display and editing. Their existing `icon` field stores `icon:<name>`; raw legacy emoji is display-only compatibility.

Agent and squad avatars use `ActorAvatar` in UI and the shared Lucide registry in `packages/ui/lib/avatar-icon.ts`. New icon choices persist as `icon:<allowlisted-name>`; uploaded image URLs remain supported. Legacy `emoji:` avatars are read as mapped Lucide icons, never rendered as emoji. Reactions are independent and continue using emoji.

`common/squad-avatar.tsx` only supplies a template default when no avatar is present. Agent/squad containers remain circular; Skill tiles keep rounded squares and category-controlled color. Skill icon components share the UI registry but retain their own backend-validated whitelist.

`AvatarUploadControl` exposes a Lucide picker for agents/squads. It persists via `onIconSelected` when supplied, otherwise `onUploaded`; an empty default preview is not a saved avatar and must not expose Clear. The generic actor directory exposes `getSquadTemplateKey` from the existing query cache.

Mobile uses generated Lucide nodes with its existing SVG renderer. Refresh after registry changes with `node apps/mobile/scripts/sync-avatar-icons.cjs`. Server validation and template defaults must match the shared icon names; backend support must ship before clients that save icon markers.
