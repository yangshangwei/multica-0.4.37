# 项目执行小队持久化与幂等准备

Parent: ../09-12-workspace-defaults/prd.md

## Deliverable
Persist the approved JSONB project execution selection and implement the idempotent PUT configuration endpoint and automatic creation integration. Strict workspace/permissions, transaction reuse, customization preservation, recovery and concurrent/delete behavior.

## Acceptance
- [x] Parent requirements relevant to this slice are satisfied.
- [x] Changed behavior has canonical regression tests and observed passing evidence.
- [x] No edits outside owned files without coordination; existing user customizations preserved.
- [x] Evidence and remaining risks are recorded in evidence.md.

Dependencies: none beyond reviewed contract.
