# MCP agent-selection implementation

## Scope and sequence

- [x] Restore source request and implementation authorization from session `01a1300a-44c`.
- [x] Inspect the actual screenshot, saved-configuration branch, project conventions and existing picker patterns.
- [x] Run pre-change MCP regression baseline: 41 tests pass.
- [x] Curate implement/check context; start this task after artifact review.
- [x] Dispatch a Trellis implementer with ownership of the picker/setup wiring, adjacent tests, and two settings locales. Add meaningful failing behavior tests before implementation.
- [x] Implement search, scoped bulk selection, selected-only review, counts, compact identity rows and bounded list scrolling.
- [x] Preserve contextual selection, eligibility, empty/error states, focus handoff, permissions, busy guards, explicit assignment and partial retry.
- [x] Run picker and all existing MCP/locale suites; views typecheck/lint and applicable repository static checks.
- [x] Validate actual shared components in a task-owned browser fixture; collect wide/narrow, light/dark and localized screenshot/interaction evidence. Persist visual-verdict before another visual edit.
- [x] Dispatch independent Trellis check/review; resolve task-local findings and rerun affected checks.
- [ ] Update the existing MCP spec with the selection/filter invariants, record evidence, commit task-owned files with Lore-compatible conventional intent and archive/journal safely.

## Commands

- `pnpm --filter @multica/views exec vitest run mcp locales/mcp.test.ts locales/parity.test.ts`
- `pnpm --filter @multica/views typecheck`
- `pnpm --filter @multica/views lint`
- `pnpm typecheck` / `pnpm lint` when shared view integration is ready.
- `pnpm knip` with pre-change comparison, and `git diff --check`.

Browser scripts, logs and screenshots live in `.omx/artifacts/mcp-agent-selection/`. Do not start/stop the mismatched human environment or rewrite application routes just to expose a test fixture. Reuse installed tooling; no added dependency.

## Review gates

- Every new control has translated copy, a keyboard-accessible name, deterministic selection behavior and permission/busy disabling.
- Tests exercise observable selection/payload results, rather than duplicating implementation matrices across layers.
- Check code/model types and contextual ID/name-only compatibility before browser validation.
- Keep unrelated existing Trellis task edits and worktree state untouched. Stage task-owned files explicitly.
