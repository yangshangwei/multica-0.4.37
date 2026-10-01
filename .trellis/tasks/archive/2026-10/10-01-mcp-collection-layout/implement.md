# MCP collection implementation plan

**Goal:** Deliver the user-approved collection layout with shared icon styling.

**Architecture:** Reuse the existing MCP workspace controller, query cache, shared collection header and UI primitives. Separate standalone presentation from settings presentation without duplicating business behavior.

**Tech stack:** React, TypeScript, Tailwind container queries, Lucide, React Query, Vitest, Playwright.

## 1. Baseline and implementation
- Read CLAUDE.md and implement.jsonl. Run existing mcp market, agent discovery and settings MCP component tests before changing code.
- Ownership: packages/views/mcp/*, packages/views/settings/components/mcp-tab.tsx and its test only; locales/settings only if necessary after reading naming conventions.
- Replace standalone max-width/nested title with the shared collection shell and custom action. Keep Settings entry embedded.
- Update market tabs/counts, bounded search, category metadata, three-column container grid and icon tiles. Preserve existing permission/flow/focus contracts.
- Add meaningful behavior tests only where shell/action/count wiring changes are not covered; do not add CSS-string tests.
- Run focused MCP tests, views lint and typecheck. Report changes and evidence without committing.

## 2. Browser verification (leader)
- Discover and verify the current checkout's local environment; do not reuse a mismatched API silently.
- Run existing e2e/mcp-market.spec.ts flows and capture wide/narrow, English/Chinese, light/dark pages. Add responsive assertions to the existing E2E only if useful for the new grid behavior.
- Compare Agents/Skills references and MCP screenshots; store a visual verdict under .omx/state/mcp-collection-layout/ralph-progress.json. Fix concrete defects in a bounded pass.

## 3. Review and finish
- Independent check using check.jsonl and approved criteria. Preserve existing sidebar edits owned by another task.
- Run required scoped checks and static analysis; report any unrelated failures accurately.
- Update .trellis/spec/views/frontend/mcp-market.md with the new layout contract and record verification evidence in the task. Complete task bookkeeping after verified delivery.
