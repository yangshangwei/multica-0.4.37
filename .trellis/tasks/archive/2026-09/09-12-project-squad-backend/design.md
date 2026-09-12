# 项目执行小队持久化与幂等准备 — technical slice

The authoritative API/data/UX contract is ../09-12-workspace-defaults/design.md. Read it before implementation.

## Responsibility
Persist the approved JSONB project execution selection and implement the idempotent PUT configuration endpoint and automatic creation integration. Strict workspace/permissions, transaction reuse, customization preservation, recovery and concurrent/delete behavior.

## Owned files
- server/internal/handler/project.go
- server/internal/handler/project_execution_squad.go
- server/internal/handler/squad_template.go
- server/pkg/db/queries/project.sql
- server/pkg/db/queries/squad.sql
- server/pkg/db/generated/
- server/cmd/server/router.go
- server/migrations/

## Boundaries
No new dependencies. React Query owns server state, navigation adapters own routes. Server mutations are workspace-scoped and maintain existing permission gates. Coordinate any contract change with leader before changing it.

Dependencies: none.
