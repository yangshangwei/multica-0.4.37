# 开箱即用流程集成验证与文档同步 — technical slice

The authoritative API/data/UX contract is ../09-12-workspace-defaults/design.md. Read it before implementation.

## Responsibility
Integration, spec review followed by code quality review, regression and browser/visual evidence, docs/spec updates and Trellis lifecycle with scoped Lore commits. No real agent CLI, deployment or external communications.

## Owned files
- e2e/ workspace-defaults tests
- .trellis/spec/ affected guidance
- docs/ affected product docs
- parent progress.md and child evidence.md

## Boundaries
No new dependencies. React Query owns server state, navigation adapters own routes. Server mutations are workspace-scoped and maintain existing permission gates. Coordinate any contract change with leader before changing it.

Dependencies: all preceding slices.
