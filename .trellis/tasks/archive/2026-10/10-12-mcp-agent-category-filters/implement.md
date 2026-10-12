# Category filter implementation

- [x] Inspect current picker, supplied reference and authoritative directory classification helpers/spec.
- [x] Run existing MCP/locale baseline: 248 tests, 12 files pass.
- [x] Curate context and activate task.
- [x] Dispatch one Trellis implementer owning picker/setup wiring, optional template-hook gating, related tests and two MCP settings locales.
- [x] Add meaningful failing category/wiring regressions before implementation.
- [x] Implement represented category chips/counts, classification metadata recovery and composition with search/selected review, preserving submission and scoped bulk semantics.
- [x] Run focused MCP/template-hook tests, locale parity, views lint/typecheck and applicable repository checks/static comparison.
- [x] Validate actual shared components in a production browser fixture with synthetic categories/template catalogs; persist visual verdict before another UI edit.
- [x] Dispatch independent Trellis checker and resolve task-local findings.
- [x] Update the existing MCP spec and collect implementation/browser evidence.
- [ ] Commit scoped files with inline Lore/OmX trailers, then archive/journal safely.

## Verification record

- Focused MCP/template/locale/core suites: passed (295 tests across the task-owned focused files).
- Full `@multica/views` suite: passed (546 files, 6694 tests).
- Root `pnpm typecheck`: passed across 9 packages.
- Root `pnpm lint`: passed with existing warnings only; no errors.
- Production component browser fixture: passed in 5 wide/narrow, English/Chinese,
  light/dark and long-label scenarios; browser errors 0.
- `pnpm knip`: reports the pre-existing baseline unused files/dependencies; no
  task-owned additions were reported. Keep this known repository-level gap visible
  until the baseline is cleaned separately.

Validation commands: `pnpm --filter @multica/views exec vitest run mcp locales/mcp.test.ts locales/parity.test.ts agents/create/use-role-templates.test.ts`; `pnpm --filter @multica/core exec vitest run agents/category.test.ts agents/discovery.test.ts`; `pnpm typecheck`; `pnpm lint`; scoped ESLint; `pnpm knip` with a pre-change baseline; `git diff --check`.

Only run tests that exist; add a dedicated hook test only if hook behavior changes. Browser scripts/logs/captures live under `.omx/artifacts/mcp-agent-category-filters/`. Keep unrelated Trellis task changes untouched; stage this task's files explicitly. No backend or desktop-process restart is needed for this view change unless live HMR verification reveals a concrete need.
