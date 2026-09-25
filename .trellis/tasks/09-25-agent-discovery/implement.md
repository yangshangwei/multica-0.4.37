# Agent discovery implementation plan

Goal: complete the approved first iteration of role identification, squad filtering and template/member separation.
Architecture: shared core types/query/derivations own data, views own UI, apps use existing navigation adapters. Preserve existing mutations, identities and access semantics.
Tech stack: Go/Chi, React, TanStack Query/Virtual, Zustand, Base UI, Vitest and Playwright. No new dependencies.

## Sequence

- [x] Review screenshot/source and capture approved scope in PRD.
- [x] Inspect Trellis specs, current code, API completeness and test environment.
- [x] Write technical design, explicit ownership, context manifests and verification plan.
- [ ] Start Trellis task after confirming prior user approval covers this first iteration.
- [ ] Data lane: failing tests, complete squad membership response and parser/query, discovery derivation helpers, filter/group/default preference contract.
- [ ] Template lane: failing instance/navigation tests, template picker links/counts/loading/error/scroll restoration; update role-template E2E.
- [ ] Directory lane: failing filtering/wiring tests, remove catalog, group rows, role/squad narrowing and membership labels, explicit Display and 30-day labels.
- [ ] Leader: add all locale keys, integrate lanes and resolve type/test wiring issues.
- [ ] Run bounded scoped tests, lint/typecheck/static checks, affected Go checks.
- [ ] Prepare owned production Web/API and verify directory/template flows, multiple memberships and responsive rendering; save screenshots and visual verdict.
- [ ] Independent integrated check and fixes, rerun only affected verification.
- [ ] Update docs/spec and verification.md with evidence and material limitations.
- [ ] Create atomic Lore commit, complete/archive task and record journal according to Trellis.

## Commands

- pnpm --filter @multica/core test agents/ api/squad-members.test.ts (adapt exact files to canonical suites)
- pnpm --filter @multica/views test agents/
- pnpm --filter @multica/core typecheck; pnpm --filter @multica/views typecheck
- pnpm --filter @multica/core lint; pnpm --filter @multica/views lint
- go test ./internal/handler -run 'Squad.*' -count=1 (server working directory; isolated test DB if required)
- pnpm exec playwright test e2e/agent-role-template.spec.ts e2e/agents-discovery.spec.ts --workers=1 --retries=0 (owned production environment)
- git diff --check; relevant detector and no-unused export checks

## Verification cautions

Member preview is truncated and cannot drive filtering. Unknown role data never becomes a guessed capability. Do not overwrite existing preference arrays. Browsing links do not invoke mutations. Previous browser inspection could not use Desktop renderer in Chrome; use a production Web environment with recorded source identity. Do not build while edits or another registered Web build are running in this checkout.
