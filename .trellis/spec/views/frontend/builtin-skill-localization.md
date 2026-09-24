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

Use `searchText` for matching and `searchNames` for name-ranking tiers. English and Chinese names and the current locale's displayed name must rank as names regardless of the active UI language. Include the current localized description in search text too; a Japanese or Korean label must remain searchable with only that locale loaded.

Adding an embedded role skill also requires registering its canonical name in
`BUILTIN_ROLE_SKILL_NAMES`, adding all four locale entries, and keeping their Chinese default descriptions
identical to the embedded frontmatter. The source-sync suite discovers the
embedded directories and compares them with all four locale catalogs, then
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

Keep template session state in the root creation dialog so switching back to the
method chooser does not discard edits. Preview selection is separate from the
draft source. Only explicit template adoption replaces a draft, and only the
final create action writes a skill. Copies keep distinct names and informational
`config.template_source`, never official `origin` metadata or agent permissions.

Group the picker by provenance, derived purely from `presentation.isBuiltin`:
platform built-ins (name in the role-skill catalog, resolver returns non-null)
versus deployment-provided (operator-mounted, inline fallback with
`isBuiltin: false`). Render a group's section only when it is non-empty, and
split AFTER search filtering so both groups stay searchable. The deployment
group heading carries its count and an inline, external-link-free hint that these
come from the server's mounted skill-template directory (see the server
`builtin-templates.md` mounted-directory scenario and the self-host quickstart).
When no deployment templates exist — computed off the UNFILTERED catalog so an
empty search never hides it — show a persistent muted hint that an operator can
mount public templates. Do not add a `新建 skill` chooser entry or an in-UI
directory-config form: the channel is passive/filesystem-backed, and the mount
path is operator territory, not an end-user action.

An unconfirmed submission is independent from the editor step. Returning to
editing must retain the close warning and recovery action. Opening an older
recovered result must also protect any newer draft changes. Pin create/recovery
requests to the originating workspace (including clearing an ambient slug), and
never navigate on a late response after timeout, unmount or workspace change.

## Verification

Run the pure presentation/source-sync suite, skill list/detail and picker suites, slash suggestion/extension suites, tab presentation suite, and locale parity. Use a single-locale provider in at least one regression test so complete test resource bundles do not hide production failures.
