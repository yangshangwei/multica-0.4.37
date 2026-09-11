# Built-in Skill Localization

Use `useSkillPresentation()` from `packages/views/skills/hooks/use-skill-presentation.ts` for workspace skill labels and purpose descriptions. Keep the original skill object for IDs, invocation names, editable fields, and mutation payloads.

## Identity and customization

- A name alone does not identify a built-in workspace skill. Require a known canonical name and matching `config.origin.type === "builtin_role_skill"` and `config.origin.name`.
- Compact `AgentSkillSummary` records omit `config`. Enrich them by UUID from the workspace skill query, or keep their stored text while metadata is unavailable.
- Only the built-in role-template catalog may call `getBuiltinRoleSkillPresentation(name, t)` without workspace provenance. Do not synthesize origin metadata for arbitrary records.
- Translate a saved description only when it matches the canonical English default. Preserve customized descriptions and renamed skills. The source-sync tests compare defaults with the embedded server SKILL.md files.
- Origin metadata is editable product metadata, not an authorization or integrity guarantee.

## Language and search

Production providers can mount only the current locale. `t(..., { lng: "another-locale" })` does not make an unloaded locale available. The presentation resolver reads secondary-language search text and source-description comparisons from the English and Chinese catalogs; current display strings still use the active i18next translator.

Use `searchText` for matching and `searchNames` for name-ranking tiers. Both languages must rank as names regardless of the active UI language, or a Chinese exact-name match can disappear behind description matches and a result limit.

## Editor and network boundaries

- Keep slash item `label` and `id` canonical: they become stored invocation markers. Render localized text separately.
- Cached or raw matching slash choices must not await a metadata refresh. Refresh online in the background; skip offline/paused metadata requests. Only an online cold-cache query that cannot match raw text needs to await provenance.
- Detail localization must stay outside draft seeding, dirty checks, and save payloads. Test both pristine and edited drafts across a locale change.

## Verification

Run the pure presentation/source-sync suite, skill list/detail and picker suites, slash suggestion/extension suites, tab presentation suite, and locale parity. Use a single-locale provider in at least one regression test so complete test resource bundles do not hide production failures.
