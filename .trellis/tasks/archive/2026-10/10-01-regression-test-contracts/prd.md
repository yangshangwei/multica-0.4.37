# Align regression tests with current UI contracts

## Goal

Fix the 11 stale E2E cases and 3 unit tests identified in the 2026-09-30 QA report, preserve canonical behavior coverage and keep known product defects visible.

## Requirements

- Align the requested stale tests with current UI semantics without changing product implementation.
- Keep device-login navigation and duplicate accessible-name defects separate and visible.
- Preserve existing API, persistence, permission, undo and cancellation assertions.

## Acceptance Criteria

- [x] Current sidebar/MCP names, squad chooser, skill summary/title/list and second-workspace Welcome flow are covered.
- [x] AI refinement has one canonical E2E suite and retains all applicable coverage from the obsolete suite.
- [x] Three affected component tests pass using semantic icon assertions and scoped controls.
- [x] Changed E2E suites run against an isolated real app/API/database with the deterministic provider enabled.
- [x] Lint, typecheck and broader tests run; unrelated failures are reported.
- [x] No product/dependency changes or unrelated edits are included.

## Notes

- Keep `prd.md` focused on requirements, constraints, and acceptance criteria.
- Lightweight tasks can remain PRD-only.
- For complex tasks, add `design.md` for technical design and `implement.md` for execution planning before `task.py start`.
