# 项目小队 API 类型与查询状态 — technical slice

The authoritative API/data/UX contract is ../09-12-workspace-defaults/design.md. Read it before implementation.

## Responsibility
Concrete response/request types, tolerant API boundary schemas, scoped configure mutation/invalidation, project draft selections and pure runtime/default readiness helpers. Export the names fixed in the parent design.

## Owned files
- packages/core/types/project.ts
- packages/core/api/schemas.ts
- packages/core/api/client.ts
- packages/core/projects/queries.ts
- packages/core/projects/mutations.ts
- packages/core/projects/draft-store.ts
- packages/core/projects/execution-squad.ts
- packages/core/projects/index.ts

## Boundaries
No new dependencies. React Query owns server state, navigation adapters own routes. Server mutations are workspace-scoped and maintain existing permission gates. Coordinate any contract change with leader before changing it.

Dependencies: fixed backend contract.
