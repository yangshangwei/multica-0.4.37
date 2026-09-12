# 内置小队、智能体与 skill 直接展示 — technical slice

The authoritative API/data/UX contract is ../09-12-workspace-defaults/design.md. Read it before implementation.

## Responsibility
Expose server catalogs directly above instance listings with loading/error recovery, usable entry paths, authoritative origin/identity, use squad for existing/new project, explicit copies and existing custom-create affordances. Do not fabricate or count templates as actual instances.

## Owned files
- packages/views/squads/components/squads-page.tsx
- packages/views/agents/components/agents-page.tsx
- packages/views/skills/components/skills-page.tsx
- packages/views/projects/components/use-squad-for-project-dialog.tsx
- packages/views/locales/{en,zh-Hans,ja,ko}/{squads,agents,skills}.json

## Boundaries
No new dependencies. React Query owns server state, navigation adapters own routes. Server mutations are workspace-scoped and maintain existing permission gates. Coordinate any contract change with leader before changing it.

Dependencies: agreed create-project modal payload, project-squad-core (configure mutation).
