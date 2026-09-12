# Core execution evidence

## Changes
Project execution DTO/request, tolerant schemas and explicit workspace-scoped read/write APIs, safe identity checks, configure mutation/cache invalidation, draft selection, canonical runtime eligibility/default/readiness helpers. Reused existing runtime binding/access predicates and captured-workspace request initializer; no dependency added.

## Red -> green
- New API tests initially failed 12 cases; no configure method, no parsing/workspace capture existed.
- Configure mutation test failed because the hook was absent.
- Added contradiction/resource-boundary regressions: 4 observed failures before fixing.
- Final focused suite: api/project-execution.test.ts, projects/execution-squad.test.ts, projects/execution-mutations.test.tsx, api/schemas.test.ts, api/skill-template-client.test.ts: 5 files / 196 tests passed.
- corepack pnpm --filter @multica/core typecheck: passed.
- corepack pnpm --filter @multica/core exec eslint api/client.ts api/schemas.ts api/project-execution.test.ts projects types/project.ts types/index.ts: passed.

## Remaining integration
Full workspace tests and browser flows run after UI lanes settle. Project response is additive; older/malformed configuration cannot enable dispatch. Resource reads fail rather than pretending missing constraints mean no constraints.
