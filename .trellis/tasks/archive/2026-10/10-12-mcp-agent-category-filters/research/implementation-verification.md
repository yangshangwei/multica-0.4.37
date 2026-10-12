# Scoped implementation verification

The category picker implementation is ready for the main session's production-component browser batch and independent check.

## Files changed

- `packages/views/mcp/mcp-agent-picker.tsx`: optional saved category/template fields; existing directory-helper classification; represented preset/custom chips; full-directory inventory counts; category + search + selected-only projection; incomplete preset counts/results; retained selected chips through metadata/directory recovery; horizontal toolbar scrolling and pressed/focus/coarse-pointer states.
- `packages/views/mcp/mcp-setup-dialog.tsx`: gate the existing role/squad template metadata queries to the saved general picker when an eligible template-dependent row has no explicit category; preserve cached catalogs on refresh failure; wire retry/readiness without changing assignment IDs or mutations.
- `packages/views/agents/create/use-role-templates.ts`: optional `enabled` options with default `true`; retain existing locale keys and 30-minute stale times.
- `packages/views/mcp/mcp-agent-picker.test.tsx`: seven named projection/recovery/localization regressions. Core owns the canonical classification matrices.
- `packages/views/mcp/mcp-market.test.tsx`: renamed role/category wiring, hidden cross-category submission payload, metadata enablement, initial-error/retry/cached-refresh integration, no squad/system choices, contextual suppression and category busy/permission guards. Test QueryClient identity is retained across rerenders.
- `packages/views/agents/create/use-role-templates.queries.test.tsx`: three real-hook tests for enablement, existing default/language keys, cached refresh failure and retry.
- `packages/views/locales/en/settings.json` and `packages/views/locales/zh-Hans/settings.json`: category/count/recovery/filtered-empty copy. Preset names reuse `agents.discovery.roles`.

## Evidence

- Before implementation, the new assignment category regression failed because `Agent categories` did not exist.
- Before uncertainty handling, three new picker tests failed for incomplete counts and retention of a selected preset/custom chip during metadata recovery.
- `pnpm --filter @multica/views exec vitest run mcp locales/mcp.test.ts locales/parity.test.ts agents/create/use-role-templates.test.ts agents/create/use-role-templates.queries.test.tsx`: **274 tests passed, 15 files**.
- After adding category controls to the existing busy/permission assertions, `pnpm --filter @multica/views exec vitest run mcp/mcp-market.test.tsx -t 'disables every selection control|keeps workspace selection and search disabled'`: **2 tests passed**, 40 skipped.
- `pnpm --filter @multica/views exec tsc --noEmit --pretty false`: **passed** after the final test wiring.
- Scoped ESLint covering all six changed TS/TSX files: **passed**.
- `/Users/artisan/.agents/skills/impeccable/scripts/impeccable detect --json packages/views/mcp/mcp-agent-picker.tsx packages/views/mcp/mcp-setup-dialog.tsx`: **no findings (`[]`)**. The subsequent small focus-ring padding/class adjustment did not change the visual direction.
- `git diff --check`: **passed**.

The existing pnpm configuration warning remains: root `pnpm.onlyBuiltDependencies` and `pnpm.overrides` are ignored by the installed pnpm version. It did not fail these checks.

## Browser/check handoff

Metadata requests live in `McpSetupDialog`; no actual squad instance/membership query is added. English labels are `Agent categories`, `All (n)`, `General-purpose agents (n)`, `Specialists (n)`, `Coordinators (n)`, and the unchanged custom category name. Chinese uses `智能体分类`, `全部（n）`, `通用智能体（n）`, `专业角色（n）`, `统筹角色（n）`; category counts use full-width parentheses. Unknown preset counts use `n+`, while All/custom inventory remains usable and exact. Retry uses the existing `Retry` / `重试` label.

No browser inspection, full repository checks, core taxonomy tests, backend tests, process restarts, commits, task/spec edits or finish/archive operations were performed by this implementer; those remain with the main session. Runtime source is stable, and there are no known scoped implementation blockers.
