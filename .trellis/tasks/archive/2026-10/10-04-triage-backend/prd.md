# 分拣台：事务与人工审核后端

## Scope

Settings, intake, atomic review/history, execute recovery, responsibility, notifications and batch operations.

## Source and acceptance

Parent: `../10-04-triage-t1/prd.md`. Full source: `docs/plans/2026-10-04-work-management-prds/triage-prd.md`. This child owns lane 1 from the parent implementation plan. Meet every applicable T1 requirement and supply executed tests, without narrowing parent scope. Shared API contract is `../10-04-triage-t1/api-contract.md`.

## Dependencies

Implementation starts after parent design review. Boundaries/client/UI follow the frozen schema/API contract; import depends on backend intake and schema helpers. Independent work may proceed in parallel. Shared-file changes require explicit owner coordination.
