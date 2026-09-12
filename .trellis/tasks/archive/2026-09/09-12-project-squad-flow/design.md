# 项目创建与首条任务执行流程 — technical slice

The authoritative API/data/UX contract is ../09-12-workspace-defaults/design.md. Read it before implementation.

## Responsibility
Implement the reviewed project-creation fields, task-ready detail section, explicit create-issue action with project/squad/todo, local runtime matching and retry/connection, projects first landing. Own project-detail mounts including ProjectAutomationsSection from automation lane. Keep project lead separate.

## Owned files
- packages/views/modals/create-project.tsx
- packages/views/projects/components/project-squad-picker.tsx
- packages/views/projects/components/project-squad-section.tsx
- packages/views/projects/components/project-detail.tsx
- packages/views/projects/components/projects-page.tsx
- packages/views/onboarding/onboarding-flow.tsx
- apps/desktop/src/renderer/src/components/window-overlay.tsx
- apps/web/ onboarding completion callers
- packages/views/locales/{en,zh-Hans,ja,ko}/{projects,modals,onboarding}.json

## Boundaries
No new dependencies. React Query owns server state, navigation adapters own routes. Server mutations are workspace-scoped and maintain existing permission gates. Coordinate any contract change with leader before changing it.

Dependencies: project-squad-core, project-squad-backend (integration).
