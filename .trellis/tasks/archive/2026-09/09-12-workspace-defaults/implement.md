# Workspace defaults implementation plan

Goal: implement approved project-first setup using existing built-in templates.
Architecture: additive project config, transactional reusable materializer, shared core and views, thin platform wiring.
Tech stack: Go/Chi/sqlc/PostgreSQL, React/TypeScript/TanStack Query, pnpm/Vitest/Playwright.

## Dependency graph
backend and core start from the fixed contract. Project flow uses core, integrates backend. Catalog UI uses core and agreed modal prefill. Automation uses project default shape. Verification integrates all five.

## Work
- [x] Inspect baseline and create isolated worktree.
- [x] Create Trellis parent and six children.
- [x] Finish artifacts and review technical contract (five corrections applied).
- [x] Install locked dependencies and check focused baseline (6 existing project/onboarding tests passed).
- [x] Backend regression tests, additive migration/queries/sqlc and idempotent configuration.
- [x] Core schema/client/mutation/default helper regressions and implementation.
- [x] Project creation/details/onboarding and task dispatch UI plus tests.
- [x] Three catalog sections and project-use action plus tests.
- [x] Automation templates/project section/prefill plus no-early-enable tests.
- [x] Integrate, review spec compliance then code quality; fix findings.
- [x] Typecheck, lint, TS/Go tests and Go vet; targeted browser flows and visual review.
- [x] Update specs/docs/source maps affected by the changed behavior.
- [x] Commit only task-owned changes using Lore trailers, archive children and record journal.

## Validation commands
Use corepack pnpm (10.28.2), not ambient pnpm.
- corepack pnpm install --frozen-lockfile
- corepack pnpm --filter @multica/core exec vitest run <focused tests>
- corepack pnpm --filter @multica/views exec vitest run <focused tests>
- corepack pnpm typecheck
- corepack pnpm lint
- corepack pnpm test
- make sqlc
- make test (isolated worktree database)
- go vet ./... (server/)
- corepack pnpm exec playwright test <targeted specs>
- git diff --check

## Session continuity
Read parent PRD/design/implementation/progress before resuming. Each child has its own PRD, design and implementation checklist. Children report commands/results in evidence.md. Update the parent ledger after each verified slice. Native agents only own bounded independent files; leader owns integration and task states. User already authorized the planning and implementation phases.
