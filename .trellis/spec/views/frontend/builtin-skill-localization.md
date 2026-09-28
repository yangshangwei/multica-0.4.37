# Built-in Skill Localization

Use `useSkillPresentation()` from `packages/views/skills/hooks/use-skill-presentation.ts` for workspace skill labels and purpose descriptions. Keep the original skill object for IDs, invocation names, editable fields, and mutation payloads.

## Identity and customization

- A name alone does not identify a built-in workspace skill. Require a known canonical name and matching `config.origin.type === "builtin_role_skill"` and `config.origin.name`.
- Compact `AgentSkillSummary` records omit `config`. Enrich them by UUID from the workspace skill query, or keep their stored text while metadata is unavailable.
- Only the built-in role-template catalog may call `getBuiltinRoleSkillPresentation(name, t)` without workspace provenance. Do not synthesize origin metadata for arbitrary records.
- Translate a saved description only when it matches the current Chinese default, its English display copy, or a recognized historical English default. Preserve customized descriptions and renamed skills. The source-sync tests compare the Chinese defaults with the embedded server SKILL.md frontmatter.
- Workspace copies keep historical defaults after a template update. Retain explicit translations for known shipped descriptions, matched by their full text; neither a version marker nor a matching prefix proves a description is unmodified.
- Origin metadata is editable product metadata, not an authorization or integrity guarantee.

## Language and search

Production providers can mount only the current locale. `t(..., { lng: "another-locale" })` does not make an unloaded locale available. The presentation resolver reads secondary-language search text and source-description comparisons from the English and Chinese catalogs; current display strings still use the active i18next translator.

Use `searchText` for matching and `searchNames` for name-ranking tiers. English and Chinese names and the current locale's displayed name must rank as names regardless of the active UI language. Include the current localized description in search text too; a Chinese label must remain searchable with only that locale loaded.

Adding an embedded role skill also requires registering its canonical name in
`BUILTIN_ROLE_SKILL_NAMES`, adding both supported locale entries, and keeping their Chinese default descriptions
identical to the embedded frontmatter. The source-sync suite discovers the
embedded directories and compares them with both supported locale catalogs, then
checks presentation coverage; do not use a second manually maintained subset
as proof that registration is complete. Otherwise the template picker treats it as
deployment-provided and workspace skills fall back to untranslated source text.

Do not create historical aliases merely because a locale contained incorrect
copy. First establish that the exact text shipped as a persisted default. The
experience-validation and migration-review descriptions were corrected only in
the Chinese display catalog; their embedded source descriptions were already
correct from introduction, and missing frontend registration had prevented the
incorrect display text from becoming the standard adoption default.

For a recognized default description, include both English and Chinese purposes
in search text regardless of which language is persisted. Do not add default
purpose keywords to customized descriptions.

## Editor and network boundaries

- Keep slash item `label` and `id` canonical: they become stored invocation markers. Render localized text separately.
- Cached or raw matching slash choices must not await a metadata refresh. Refresh online in the background; skip offline/paused metadata requests. Only an online cold-cache query that cannot match raw text needs to await provenance.
- Detail localization must stay outside draft seeding, dirty checks, and save payloads. Test both pristine and edited drafts across a locale change.
- Overview edits currently update stored metadata without rewriting SKILL.md
  frontmatter. Explicitly localizing an existing skill requires updating both
  descriptions and preserving the rest of its file; a display translation alone
  does not update what the agent loads.

## Creating a template copy

The authenticated `/api/skills/templates` catalog is a separate type without a
workspace UUID. Use its canonical name for selection and the existing built-in
presentation resolver for display. User-initiated copy creation may seed a NEW
draft's description from the current localized purpose; later locale or query
updates must not reseed it. Existing workspace edits still preserve stored text.

The skills page uses `SkillLibraryCatalog` for a compact deployment shelf and a
separate market view. Its source totals come from valid named entries in the
unfiltered template catalog; workspace header counts and facets still describe
workspace skills. Catalog cards open the shared creation dialog with a named
`initialEntry: { kind: "templates", templateName }`; New skill uses the method
chooser. A named seed selects a preview only. See
[Skill Market Discovery](./skill-market-discovery.md) for view preferences,
filter scopes, in-place copy creation and focus behavior.

Picker rows may use `builtin_role_skills.<name>.summary` through
`getBuiltinRoleSkillSummary`. This is display copy only. Recognize the same exact
current or shipped historical defaults as the full presentation resolver;
customized and unknown/deployment descriptions stay verbatim. Keep the original
template object, full `presentation.description`, search text, source-sync
defaults, instruction body and supporting files unchanged. Preview and adoption
use the full purpose, never the summary.

Keep template session state in the root creation dialog so switching back to the
method chooser does not discard edits. Preview selection is separate from the
draft source. Only explicit template adoption replaces a draft, and only the
final create action writes a skill. Copies keep distinct names and informational
`config.template_source`, never official `origin` metadata or agent permissions.

Group the picker into source tabs, derived purely from `presentation.isBuiltin`:
platform built-ins (name in the role-skill catalog, resolver returns non-null)
versus deployment-provided (operator-mounted, inline fallback with
`isBuiltin: false`). Keep both tabs available even when a source is empty, and
derive tab result counts after search filtering. An initial valid named seed
selects its source; otherwise use the first nonempty source, preferring platform
built-ins. This also applies when catalog data is already cached. The preview
must belong to the visible source/results, and adoption is disabled when none
remain. Search and source changes do not adopt or replace a draft.

Source labels must fit when translated text wraps. The shared `TabsList`
sets height with `group-data-horizontal/tabs:h-8`; a plain local `h-auto` cannot
override that more-specific variant. Override the matching horizontal variant
locally and let the triggers grow and stretch together. Keep narrow-screen
footer help above the actions so long adoption labels cannot squeeze it into a
thin column; preserve the desktop footer row.

The deployment tab contains an inline, external-link-free hint explaining that
templates come from the server's mounted directory (see the server
`builtin-templates.md` mounted-directory scenario and the self-host quickstart).
If no deployment templates exist in the UNFILTERED catalog, retain the muted
operator-mount hint in that tab even during search. An empty source and a search
with no matches are distinct states. Do not add a creation-method entry or an
in-UI directory-config form: the channel is passive/filesystem-backed, and the
mount path is operator territory, not an end-user action.

The template catalog changes outside workspace mutations: the server rescans
the mounted directory on each request, without a WebSocket invalidation event.
Its Query options must override the global infinite freshness so reopening,
foregrounding or reconnecting can discover changes. Poll every 30 seconds only
while the template picker is open and visible; the page's compact entry and
the copy editor must not poll. Selecting the deployment source refreshes
immediately, reusing an in-flight fetch. Refresh keeps source/search/selection
where applicable and must never reseed an adopted draft. Exercise this policy
with the production QueryClient defaults, not a bare library-default client.

Cold loading and a no-data query error must not claim zero templates or related
skills. Successful empty data can. Cached data remains usable during refresh
and after a failed refresh, with an explicit status/retry affordance. Mount the
creation dialog once, outside the page's conditional list-error body, keyed only
by workspace identity. Same-workspace failure/retry must preserve the edited
draft, unconfirmed submission and pending discard/navigation action.

Related workspace skills are all records with either a matching canonical name
and verified built-in presentation, or exact `config.template_source.name`.
Use `getRelatedWorkspaceSkills` to preserve query order and include each record
once; do not choose an arbitrary first match. Show zero only with query data,
otherwise loading/error, and render every named link for a nonzero result.
Official instances are related skills, not independent copies. Source metadata
does not prove content equality or grant permissions.

Related links use real `AppLink` anchors. Every Desktop adapter intent (including
background and middle opens, which can activate an existing tab) and Web
in-place push pass through the root busy/dirty/unconfirmed guard. Snapshot the
source workspace UUID/slug, destination ID/path, presented title and resolved
intent at the original gesture. Confirmation consumes that snapshot once,
resets/closes, then invokes the original adapter action; it must not call
creation recovery, `onCreated`, or a success toast. Cancel preserves the draft
and returns focus to the link. Web-native modified links retain browser behavior.
Workspace changes clear pending actions and reject stale snapshots.

The page supplies live creation-trigger refs for dialog final focus. Direct
browsing autofocuses search before the dialog can record its previous focus,
so relying on implicit focus restoration loses the opener. Resolve the current
trigger after list failure/retry; do not restore an old dialog's focus merely
because a workspace change unmounted it. Narrow preview Back restores its
selected row, falling back to search if the row disappeared.

Leaving the template editor must use the root back handler and move focus to
the persistent dialog before removing a focused editor control. The narrow
picker can keep its preview visible while hiding search, so search autofocus
cannot provide this handoff. Otherwise Base UI schedules popup focus restoration
on the next frame and can steal focus from a related link before Enter reaches it.

An unconfirmed submission is independent from the editor step. Returning to
editing must retain the close warning and recovery action. Opening an older
recovered result must also protect any newer draft changes. Pin create/recovery
requests to the originating workspace (including clearing an ambient slug), and
never navigate on a late response after timeout, unmount or workspace change.

## Verification

Run the pure presentation/source-sync and discovery suites, skill list/detail
and picker suites, slash suggestion/extension suites, tab presentation suite,
and locale parity. The root template-flow suite owns draft/recovery/navigation
guards and snapshots; `skills-page-template-session.test.tsx` owns the real-page
query-transition and opener-focus regressions. Render entry and phase labels
with only each active locale (`en`, `zh-Hans`) loaded, so complete test
resource bundles do not hide production failures.
