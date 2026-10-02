# Curated MCP Catalog Publishing Implementation Plan

Goal: publish two useful keyless HTTP recipes and a maintainable offline release gate.
Architecture: extend the embedded roster and shared presentation; preserve the existing API and storage.
Tech stack: Go tests/Make, React/TypeScript, Vitest, Playwright, Markdown.

- [x] 1. Backend (root): add failing roster/HTTP metadata assertions in
  server/internal/service/builtin_mcp_templates_test.go and
  server/internal/handler/workspace_mcp_template_test.go; run focused Go tests.
  Add recipes in builtin_mcp_templates.go. Strengthen release invariants and
  exercise real HTTP creation/rename/replacement through existing testutil fixtures.
- [x] 2. Frontend (bounded implement agent): add failing category/HTTP setup tests,
  add documentation category in packages/views/mcp/mcp-market.tsx and en/zh-Hans
  settings locales, map new icons in common/mcp-template-icon.tsx. Update existing
  Web/native E2E roster expectations and HTTP save/assignment evidence. Run
  focused Vitest, views typecheck and scoped lint. No backend/docs edits.
- [x] 3. Publishing docs and entry point (root): add make check-mcp-catalog using
  guarded offline Go tests; document official review, versions, backend delivery,
  withdrawal and rollback under docs and link from contributing. Update built-in
  creating-agents skill source map and Trellis MCP contract. No new dependencies.
- [x] 4. Verify/review (root + read-only reviewer): integrate changes, run scoped
  Go/TS tests and static checks, production Web and native Electron evidence,
  record visual verdict and verification.md, resolve review findings. Commit only
  this task's changes using Lore trailers; no remote push or production release.
- [x] 5. Resumed final review: reproduce unbraced POSIX/Windows placeholder
  bypasses in the publishing gate; add failing regression cases, share the
  corrected test-only detector across commands/arguments/URLs, and document it.
  Verify actual corrupt recipes fail before committing and archiving the task.

Focused gate (server cwd):
`bash ../scripts/go-test-with-agent-cli-guard.sh -- go test ./internal/service ./internal/handler -run '^(TestMcpServerTemplates_|TestListMcpServerTemplates_|TestMcpMarket)' -count=1`

Views checks: `pnpm --filter @multica/views test mcp/ settings/components/mcp-tab.test.tsx locales/mcp.test.ts`; `pnpm --filter @multica/views typecheck`.
Use isolated local DB for lifecycle tests; run E2E against task-owned production Web.
