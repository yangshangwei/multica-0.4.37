# MCP Market Implementation Plan

Goal: implement approved B core and C contextual entry on existing three templates.
Architecture: backend trusted recipe creation and persistent instance attribution; shared frontend market + setup/assignment flow, reused inside agent MCP.
Tech Stack: Go/sqlc/PostgreSQL, React/TanStack Query/Zustand, shared Base UI, Vitest/Playwright.

1. Backend/core lane: write failing source/identity/security/schema tests; add migration, SQL/sqlc, template metadata and trusted creation, summary fields, API/hook types; run targeted Go + core tests and core typecheck. Own server + core only.
2. Frontend lane: write failing flow tests; shared MCP market/setup components, workspace integration, en/zh-Hans copy; then agent contextual entry using same components. Own views (except e2e). Preserve manual editing, permission gates, no false connected status. Run MCP component tests and views typecheck/lint.
3. Integration lane: isolated dependencies/environment, baseline existing MCP checks, add real-API e2e market creation/assignment/rename/member scenario. Validate API contracts across lanes. Own e2e/task/docs and final integration fixes.
4. Verify targeted TS + Go tests, full affected-package typechecks, changed-file lint, go vet and diff whitespace/static checks. Start isolated API+web, test desktop and narrow browser flows, capture screenshots and visual-verdict JSON before visual fixes.
5. Independent code review, resolve material findings, update specs and evidence. Commit only isolated feature files with Lore intent/trailers. No publish/push/deploy.

Rollback: revert feature commit and migration before new binaries start; custom existing entries remain functional, no automatic conversion of historical instances.
