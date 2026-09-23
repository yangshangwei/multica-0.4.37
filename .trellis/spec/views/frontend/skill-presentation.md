# Skill Presentation (category, icon, labels)

How a workspace skill's category, icon and labels are stored, parsed and rendered. Shipped with task `09-19-skill-library-categories`; task `09-22-skill-lifecycle-taxonomy` extended the category set (adding `design`/`quality` while keeping every prior key valid) and added batch label management.

## Storage

- Category and icon live in `skill.config.presentation = { category?, icon? }`, next to `origin` and `template_source`. No DB column, no migration.
- Labels are NOT in `config`. They are workspace labels (`issue_label` rows with `resource_type = "skill"`) linked through `skill_to_label`, embedded on `SkillSummary.labels` by `GET /api/skills`, and accepted as `label_ids` on `POST /api/skills`.
- A `presentation.tags` key is legacy from an earlier draft of this feature: the reader ignores it, the writer and the Go normalizer drop it. Do not reintroduce free-text tags.
- Product model: the category is the single primary dimension (one fixed-set value per skill); every other dimension — stage, stack, owning team — is labels. User-facing contract: the "Organizing skills" section of `apps/docs/content/docs/skills.mdx`. Labels never grant permissions.

## Single source of truth

- `packages/core/skills/presentation.ts` owns `SKILL_CATEGORIES`, `SKILL_ICON_NAMES` (kebab-case Lucide names, sorted) and `SKILL_CATEGORY_DEFAULT_ICON`.
- `server/internal/skill/presentation.go` mirrors them; `presentation_parity_test.go` reads the TS literals, so a list change must be made on both sides in one PR or the Go test fails.
- Adding a category value is a coordinated rollout: the Go validator 400s values it does not know, so the backend must ship before a web release that writes the new value (a dev API predating the edit rejects it the same way). An older client reading a newer value degrades to `other` on display while the JSONB keeps the raw value, so no data migration is owed on rollback.
- Views map names to components in `packages/views/skills/lib/skill-presentation-icon.ts` with **named** `lucide-react` imports. `Record<SkillIconName, LucideIcon>` makes the compiler reject a whitelist entry without a component.

## Parsing and writing

- Always read through `readSkillPresentationMeta(config)`. It never throws: missing/mistyped/unknown values fall back (category → `other`, icon → `null` = follow category). An icon equal to the category default is canonicalized to `null`.
- Always write through `writeSkillPresentationMeta(config, meta)`. It merges over the existing config so `origin` / `template_source` survive, omits an icon equal to the default, and drops the whole `presentation` key when everything is default. Bulk "set category" and the detail page save both go through it.
- The server validates `config.presentation` on create, update, import and template materialization (400 naming the field) and stores the normalized shape. Import seeds it from SKILL.md frontmatter `metadata.category` / `metadata.icon`, dropping invalid values silently.

## Rendering rules

- Colour always comes from the category (`SKILL_CATEGORY_TONE`, backed by `--skill-<category>` tokens in `packages/ui/styles/tokens.css`); the icon override changes shape only.
- `SkillPresentationIcon` is the per-skill tile. The entity icon `SkillIcon` (route icon) is unchanged and still means "a skill" in headers, empty states and pickers.
- Facet counts (category / origin / label / agent / creator) come from `useSkillListFacets(allRows)` over the UNFILTERED rows so the sidebar, chips and toolbar can never disagree.
- The category empty state renders only when `facets.categoryCounts[category] === 0`; a search or another filter that narrows a populated category shows the plain "No matches" state in both card and list views.
- The card grid virtualizes by row: columns come from a `ResizeObserver` on the scroll container and rows are chunked into lines of `columns` cards at a fixed `SKILL_CARD_HEIGHT`.
- Cards reuse the list row's hover contract (`group/row`) so `SkillRowActions`' kebab reveals on card hover without a second variant.

## Batch label management

- `ManageLabelsMenu` in `skill-list-actions.tsx` serves the selection bar. `deriveSkillLabelState` reduces the selection to `all | some | none` counting **editable** rows only (an all-locked selection disables the menu); `setSkillsLabel` derives the per-row add/remove set, skips rows already in the target state, and aggregates per-row failures into one partial toast.
- It calls `api.attachLabelToResource` / `api.detachLabelToResource` sequentially (no batch endpoint exists) and invalidates only `workspaceKeys.skills` when done. No optimistic updates: a multi-row outcome is not locally predictable.
- Deliberately not the `useAttach/useDetachResourceLabel` hooks: those invalidate per call and cannot aggregate failures across a loop. Follow this shape for future bulk actions that reuse per-resource endpoints.
- Accessibility note: tri-state rows expose `aria-pressed` (true only when `all`); the `Minus` partial indicator is visual-only. This matches the existing `AgentPickerRow` shape — a known gap, not a regression.

## Tests

| Behaviour | Canonical file |
| --- | --- |
| Enum / whitelist / reader / writer matrix | `packages/core/skills/presentation.test.ts` |
| TS ↔ Go list parity | `server/internal/skill/presentation_parity_test.go` |
| Go validate / normalize matrix | `server/internal/skill/presentation_test.go` |
| HTTP 400 wiring, stored shape, template + import seeding, `labels` embed, `label_ids` | `server/internal/handler/skill_presentation_test.go` |
| Facet counts | `packages/views/skills/hooks/use-skill-list-facets.test.ts` |
| Icon map completeness | `packages/views/skills/lib/skill-presentation-icon.test.ts` |
| Page wiring (filters, view toggle, empty states) | `packages/views/skills/components/skills-page.test.tsx` |
| Bulk-label tri-state / permission skip / failure aggregation | `packages/views/skills/components/skill-list-actions.test.tsx` |
| Built-in role-skill presentation defaults | `server/internal/service/builtin_agent_templates_test.go` |
| Visual acceptance (wide/narrow/dark, en/zh) | `e2e/skill-category-taxonomy.spec.ts` |

The e2e spec mirrors the category key order and the EN/ZH display names; renaming either must update that file in the same PR or the visual acceptance silently drifts.
