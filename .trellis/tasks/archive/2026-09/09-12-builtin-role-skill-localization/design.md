# Built-in role skill presentation design

The user approved presentation localization while retaining canonical English identifiers and skill bodies. This extends the existing interface rather than introducing a new visual design.

## Shared presentation

Add a pure resolver in `packages/views/skills/lib/skill-presentation.ts` and a thin `useSkillPresentation()` hook in `packages/views/skills/hooks/use-skill-presentation.ts`.

The hook returns a stable `presentSkill(skill)` function. The input requires `name` and `description`, with optional `config` so compact agent summaries degrade to their original text. Its result contains `name`, `description`, `searchNames`, `searchText`, and `isBuiltin`.

Only the seven known names with matching `config.origin.type === "builtin_role_skill"` and `config.origin.name === skill.name` qualify. Localized copy lives under `builtin_role_skills` in the existing skills locale namespace. English names remain the canonical identifiers. The English description is also the default-description comparison: customized descriptions are preserved. Chinese and English default descriptions are searchable when the stored description is still the built-in default; both names and the canonical identifier are always searchable for a recognized built-in.

Production mounts only the current language bundle, so secondary-language search copy and canonical description comparisons come directly from the English/Chinese locale catalogs. Current display uses i18next. Separate bilingual search names keep name-ranking tiers independent of the active language.

## Surface integration

- The workspace list resolves presentation once per row, searches its bilingual text, sorts by the displayed name, and uses the displayed name for navigation labels. Built-ins show a compact purpose line within the existing row height.
- Detail breadcrumbs and headings use the displayed name. A translated purpose is shown separately from the raw editable properties. Language changes must never enter the dirty-state or update payload.
- The shared skill picker resolves both labels and search. Agent assignment rows enrich compact summaries by ID from the workspace skill query they already load.
- Any invocation selector must retain the canonical name in its command value; localize only its displayed label and search metadata.

## Compatibility

This is a read-only presentation layer. No skill row, source body, origin metadata, assignment, or runtime slug is rewritten. Missing or unrecognized provenance falls back to the stored text. No inference from a name alone is allowed for workspace skill records.

## Verification

Pure tests cover all seven mappings, provenance boundaries, custom names/descriptions, and bilingual search. Component tests cover language changes, shared picker selection identity, workspace list search/navigation, and detail editing without translated values leaking into saves. Existing skill and agent suites plus locale parity, lint, typecheck, static analysis, and a bounded browser visual check complete verification.
