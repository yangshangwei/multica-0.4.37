# 项目自动化模板入口与预填 — technical slice

The authoritative API/data/UX contract is ../09-12-workspace-defaults/design.md. Read it before implementation.

## Responsibility
Direct template catalog and project sidebar automation section; initial project/squad props routed through shared/platform navigation, seed once and respect permissions; only Enable submits creation. Keep custom create and current template immutable fields. Do not edit project-detail.tsx; report the mount interface.

## Owned files
- packages/views/autopilots/components/autopilots-page.tsx
- packages/views/autopilots/components/template-create-autopilot-page.tsx
- packages/views/projects/components/project-automations-section.tsx
- apps/web/ autopilot template route wrapper
- apps/desktop/ autopilot template route wrapper
- packages/views/locales/{en,zh-Hans,ja,ko}/autopilots.json

## Boundaries
No new dependencies. React Query owns server state, navigation adapters own routes. Server mutations are workspace-scoped and maintain existing permission gates. Coordinate any contract change with leader before changing it.

Dependencies: Project.execution_squad shape.
