# Current skill taxonomy research

## Existing ownership

- The category enum, default category, default icons, tolerant reader and canonical writer live in `packages/core/skills/presentation.ts:15-178`.
- The Go backend mirrors the enum/icon contract and rejects invalid writes in `server/internal/skill/presentation.go:19-222`; `server/internal/skill/presentation_parity_test.go:66-77` prevents TS/Go drift.
- Categories are presentation metadata in `skill.config` rather than columns. The list query returns `config` as JSONB (`server/pkg/db/queries/skill.sql:8-16`), and the response passes it through (`server/internal/handler/skill.go:224-241`).
- UI category counts are derived from unfiltered rows in one shared facet helper (`packages/views/skills/hooks/use-skill-list-facets.ts:38-83`). The list page performs search, label, category, usage, source, agent and creator filtering in one pipeline (`packages/views/skills/components/skills-page.tsx:827-907`).
- Category selection is single-select in the sidebar but the stored filter shape is an array shared with the multi-select toolbar (`packages/core/skills/stores/view-store.ts:49-67`, `packages/core/skills/stores/view-store.ts:143-160`).
- Category colors are centralized in `packages/views/skills/lib/skill-presentation-icon.ts:121-147` and their design tokens in `packages/ui/styles/tokens.css:37-44`, `packages/ui/styles/tokens.css:221-226`, and `packages/ui/styles/tokens.css:297-302`.

## Existing label capabilities

- Workspace labels already have a `skill` resource scope and carry name, description, color and usage count (`packages/core/types/label.ts:8-34`).
- Skill-label membership is relational through `skill_to_label` (`server/migrations/162_resource_labels.up.sql:17-22`), not embedded in presentation config.
- The backend already batches label reads for the skill list and exposes attach/detach primitives (`server/pkg/db/queries/issue_label.sql:216-258`).
- Skill summaries include labels with an old-server-compatible fallback (`packages/core/api/schemas.ts:3405-3411`).
- Creation can atomically attach `label_ids`; details use `ResourceLabelPicker`; list search and filters already consume embedded labels (`server/internal/handler/skill.go:326-334`, `packages/views/skills/components/skill-detail-page.tsx:540-545`, `packages/views/skills/components/skills-page.tsx:827-874`).
- The missing management affordance is bulk label assignment: the current batch toolbar only supports category updates (`packages/views/skills/components/skill-list-actions.tsx:659-773`).

## Compatibility findings

- Six stored keys can be preserved while their localized display names change; this updates the information architecture without rewriting existing JSONB.
- Replacing all keys would make old Desktop builds parse every new value as `other` and would make the new backend reject old writes unless a compatibility layer were added. The repository explicitly requires response drift tolerance for installed clients (`CLAUDE.md:77-79`).
- Adding only `design` and `quality` limits drift. Old clients fall back to `other`; they also omit `config` during ordinary content/name saves unless presentation changed (`packages/views/skills/components/skill-detail-page.tsx:1028-1060`), so passive edits do not erase the new category.
- Role skills are copied into a workspace and deliberately never overwritten on later releases (`server/internal/service/builtin_role_skills.go:12-26`). Changing template metadata therefore affects future materializations only; silently reclassifying existing copies would violate that ownership contract.

## Recommended decision

1. Preserve the six stable keys and add `design` and `quality`.
2. Treat category as the single primary navigation/ownership dimension.
3. Reuse workspace skill labels for all secondary axes; keep label values opaque to code in this iteration.
4. Add bulk label management so the taxonomy is practical on an existing library.
5. Defer structured label groups until there is evidence that a flat filter cannot support real workspaces.
