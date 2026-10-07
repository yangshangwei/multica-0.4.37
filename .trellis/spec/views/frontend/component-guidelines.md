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

### Integration catalog and provider detail navigation

`IntegrationsTab` owns the catalog and mounts a provider form only when that
provider is selected and deployment-enabled. `settings-integration-navigation.ts`
owns URL parsing and construction: cards push `?tab=integrations&integration=…`,
main-tab switches replace and clear `integration`, and unrelated query values and
fragments survive. Selecting the already-active Integrations tab must also return
to the catalog; Base UI does not emit `onValueChange` for this case.

The backend's `?tab=github` callback resolves to GitHub **detail**, not just the
umbrella tab. The existing Lark callback resolves similarly when enabled.
Composio retains its separately gated surface and existing one-shot `connected`
and `error=composio_connect_failed` callback handling.

Deployment configuration loads independently of authentication. The initial
`false` messaging/VCS flags do not prove that a provider is unavailable. Show the
catalog and an explanatory status for an unavailable selection, but retain the
requested URL so a later configuration response can reveal its detail. Never
mount hidden providers or allow planned cards to open a form.

GitHub connection status comes from the workspace-scoped installation query,
gated by known membership. Keep loading/error states separate from disconnection,
and keep connection state independent of the workspace feature master switch.
Live cards expose their status through `aria-describedby`; returning to the
catalog restores the prior card's focus. Shared GitHub sections use h3 below the
detail's h2. Its repository shortcut uses the same URL helper to clear provider
selection without dropping other URL context.

Tests: URL matrices live in `settings-integration-navigation.test.ts`, shell
wiring in `settings-page.test.tsx`, catalog behavior and slow configuration in
`integrations-tab.test.tsx`, and GitHub behavior in `github-tab.test.tsx`.
`e2e/settings-integration-catalog.spec.ts` verifies production Web navigation,
both locales, responsive layout, persisted feature preferences and mocked
installation changes. Base UI assigns switch IDs to hidden checkboxes: use
`getByRole("switch", { name })` for browser interaction, not an ID locator.

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

### Welcome page viewport sizing

The welcome intro must size against the viewport independently of its six-stage
illustration. Keep its height intrinsic with a viewport minimum and desktop
`self-start`; centering it against the full illustration height pushes the CTA
below the fold at 1024×768 in English. Keep the outer row non-shrinking inside
the desktop overlay's flex scroll container so every stage remains reachable.
Verify both the initial CTA and the final stage using that real container shape,
including narrow screens and enlarged text. Size hero type against its content
container, and allow the emphasized phrase to wrap when enlarged beyond it.

<!-- How styles are applied (CSS modules, styled-components, Tailwind, etc.) -->

(To be filled by the team)

---

## Accessibility

### Platform administration navigation

`admin/admin-shell.tsx` owns the six platform navigation destinations and their
active child groups. Reuse that directory in the desktop sidebar and compact
Sheet. Mark only the current destination with `aria-current="page"`; a group
link can remain visually selected without claiming to be the current page.
Protected destinations and content require the ready administration state.
Close an open compact Sheet when access is lost so its modal backdrop cannot
obscure the denial or unavailable message. Preserve current-link initial focus,
Escape/trigger focus return and dismissal when switching to the desktop layout.
Canonical interaction coverage lives in `admin/admin-shell.test.tsx`.

Administration filter forms share validation through `useAdminFilterFeedback`.
Pass the current effective query so navigating between saved views clears draft
errors. Reset buttons must trigger a native form reset and clear feedback before
removing URL filters; navigation alone does not clear a draft when the URL is
already unchanged. `admin/filter-feedback.test.tsx` covers reset, presets and
back-navigation behavior. Pure validation in `packages/core/admin/view-params.ts`
matches server UTF-8 byte limits and normalizes accepted time-zone aliases to
canonical IANA names. Date-only account ranges use UTC and include the selected
last day, capped at now for today.

Use the shared `adminTouchLinkClass` with a block-level display for content links,
including links inside closed disclosures or optional execution lineage. The
shell's touch selectors cover form controls and summaries, not arbitrary links.
Keep secondary IDs available in details while prioritizing names and statuses
on narrow lists. Validate list return destinations before retaining their query.

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

### Administration-only visual surfaces

The Electron renderer does not mount `@multica/views/admin`, but its Tailwind
source scan includes the entire views package. Administration presentation must
therefore stay in the locally imported `admin/admin-visual.module.css`; never
move its custom properties to global tokens or shared primitives. Its sibling
`.css.d.ts` declares only that module, not a wildcard asset type.

Apply the local theme to both AdminShell and the mobile Sheet portal. A portal
does not inherit custom properties from the shell's DOM subtree. Use rectangular
OKLab mixing for tinted neutral surfaces: cylindrical OKLCH interpolation can
rotate neutral-to-accent blends toward an unintended hue.

The overview's highlighted metrics retain accessible value descriptions,
current-state drilldown parameters, and the distinction between unknown data
and zero. Other administration pages receive a single content surface, without
changing their forms or tables. After a visual change, verify administration
routes in both locales/themes and check a separate desktop build for absence of
administration-specific CSS, in addition to the desktop regression suite.

Administration list refinements keep display and query semantics separate.
`AdminFilterSummary` accepts an explicit field allowlist and reads the submitted
URL, not draft form values or generated default observation windows. Never show
cursor tokens as active filters. Date-only account summaries display the same
inclusive UTC end date as the input while requests retain the exclusive bound.
Account reset keeps its current directory (including `/admin/administrators`);
refresh on later pages returns to the first page without discarding filters.

Audit code dictionaries must use own-property lookup and preserve unknown codes.
Known labels are presentation only: keep raw action/result/phase, full IDs and
snapshots in the event disclosure. `applied` does not prove remote execution
confirmation. Long unknown codes require `overflow-wrap:anywhere` inside flex
headings; `break-words` alone retains their oversized min-content width.

Installation state and freshness are independent. Use semantic status colors
only for fresh, known observations; stale/unavailable/unknown readings stay
neutral. Short identifiers are visual only: keep full accessible names and a
selectable/copyable identifier in the disclosure. Clipboard failure must retain
manual copy access. Execution technical disclosures preserve permission checks,
query context and all raw identifiers; controls retain their original lifecycle.


### Resource publication and password reset confirmations

Resource uploads use the admin-scoped API/query layer and multipart requests;
never set Content-Type manually on FormData. Endpoint and actor changes fence
responses and clear editor drafts. Retain files for recoverable validation errors;
a lost publication response must check the original operation receipt rather than
silently submit again. Receipt lookup remains usable when catalog refresh fails.

A withdrawal pins the revision the administrator selected. Polling must not
replace that revision in an open confirmation. Show drift and require explicit
cancel/reselection; publish instead pins its validated preview revision. Tests in
`resources/resources-withdrawal.test.tsx` cover both polling and pre-poll409 races.
Resource tabs retain44px targets on narrow/coarse-pointer layouts through scoped
admin CSS, without modifying shared Tabs primitives.

Password reset confirmation is UI-only; the existing recover-password action and
original operation key remain authoritative. Distinguish the target temporary
password from the administrator password. After errors clear secrets, and for an
uncertain retry require the same temporary password with unchanged reason/username.
Do not weaken forced change, session/token revocation, target-role/self guards or
concurrent account-version checks when changing this form.

### Project overview identity owns editor lifetime

`ProjectDetail` keys `ProjectOverviewPanel` by workspace/project identity. Cached
same-tab A-to-B navigation preserves the tab container, so project identity must
remount the overview's composer, preview, correction target, cursor and timezone
input together. Never key by revision or reset a global draft store.

`ContentEditor` reads defaultValue at mount and flushes pending changes through
its outgoing callback on unmount. Keep that callback bound to A while mounting
B's editor with B's existing draft key. Otherwise A's text can be published under
B even though storage keys themselves are correctly scoped.

The canonical `project-detail.test.tsx` regression seeds both project queries,
navigates before the editor debounce expires, verifies B's preview/body, and
returns to A to prove its draft survived. A loading fallback that happens to
unmount the tree must not be the mechanism that makes the regression pass.
