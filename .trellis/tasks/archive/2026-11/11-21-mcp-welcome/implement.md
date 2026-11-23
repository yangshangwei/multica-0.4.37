# Execution plan

## Boundary

The behavior gap is the successful-empty standalone MCP page, currently a small centered message. The owning code is `packages/views/settings/components/mcp-tab.tsx`; a new visual component belongs in `packages/views/mcp/`. Expected changes are that integration, local welcome TSX/CSS/typing/tests, bilingual `settings.json` keys, focused MCP state tests, and a focused browser journey. Documentation/spec updates record verified behavior only. Do not alter other collection pages, backend/core logic, dependencies, global styling or Settings structure.

## Ordered work

- [x] Load Trellis and impeccable context; inspect current MCP and reference welcome implementations.
- [x] Create task and record the already-reviewed scope and explicit development authorization.
- [x] Complete bounded business/test-harness research and curate implement/check context.
- [x] Review planning artifacts and activate the task.
- [x] Dispatch the Trellis implementer; add meaningful failing regressions before changing the presentation (3 expected red, 57 passing checks).
- [x] Implement the local welcome and existing action wiring; focused 137 tests, broader 248 tests, targeted lint and views typecheck pass.
- [x] Verify compiled Web and Electron against the Skill and Agent references, including both locales and themes and narrow scrolling. Retain the final screenshots and visual verdict.
- [x] Run the Impeccable detector on the finished UI. The final scan reported zero findings.
- [x] Complete independent Trellis quality review. Correct the wording about assignment to reflect that runtimes may also load local configurations, then rebuild and verify both clients.
- [x] Sync the MCP specification and retain the verification report. Keep the feature commit and Trellis closeout scoped to this task.

## Commands and evidence

- `pnpm --filter @multica/views exec vitest run settings/components/mcp-tab.test.tsx mcp/mcp-welcome.test.tsx locales/mcp.test.ts locales/parity.test.ts`
- `pnpm --filter @multica/views typecheck`
- Targeted ESLint for changed TSX/tests, then repo-supported broader lint/type checks as practical.
- Focused Playwright MCP welcome spec with production-Web build/start and recorded checkout provenance; inspect both locales/themes at desktop and narrow widths.
- `/Users/artisan/.agents/skills/impeccable/scripts/impeccable detect --json <changed UI targets>`
- `git diff --check` and final task-owned diff review.

## Recovery

Do not overwrite the existing dirty worktree or stage unrelated artifacts/journals. Capture pre-existing failures separately. Do not stop unrelated running environments. If a browser harness is unavailable, try an isolated built renderer or scoped fixture harness before reporting a concrete verification limitation. No destructive database operation is needed for this feature.
