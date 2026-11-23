# Evidence and context

## User decisions

The initial proposal was an intro + case layout with one create action and project docs. The user corrected “官网改版” to “注册激活”, explicitly requested coverage of individual agents as well as AI squads, and authorized creating the task and continuing. No product choice remains unresolved.

## Source owners

- `packages/views/projects/components/projects-page.tsx:838`: one scoped `projectListOptions(wsId)` owns raw directory data. The old `showEmpty` at :932 excluded loading and initial errors but also admitted a failed refresh of retained empty data. Keep refresh/access recovery while requiring affirmative successful data for the welcome.
- `projects-page.tsx:951`: existing `DOCS_SLUGS.projects` + `wsPaths.docsPage` and `openCreateProject()` already provide the required actions.
- `packages/views/projects/components/projects-page.test.tsx:515`: canonical initial-read failure regression, followed by cached refresh/selection and real QueryClient access-loss tests. Extend this file rather than testing shared behavior in an app directory.
- `packages/views/triage/triage-welcome.tsx`: semantic container-query two-column layout, Button/AppLink, passive figure and labelled example.
- `packages/views/iterations/iteration-welcome.tsx` + `.module.css`: restrained project-like preview, three process rows and token-based local style.
- `packages/ui/styles/tokens.css`: role-named text tokens, neutral surface palette and status foreground tokens. Existing product identity is authoritative even though Impeccable found no PRODUCT.md or DESIGN.md in the views package.
- `apps/docs/content/docs/projects.zh.mdx`: project context is shared with issue execution; repositories are optional; project lead is distinct from assignee; each issue can be assigned to an agent or squad. Project creation never starts an agent.
- `e2e/fixtures/project-p1.ts` and `e2e/iteration-welcome.spec.ts`: API-backed isolated session, screenshots and no-overflow helper. Use these for actual Web evidence if environment startup succeeds.

## Skill use

Impeccable context already ran once for `packages/views/projects/components/projects-page.tsx`; do not rerun it. This is scoped `onboard` work against incumbent code. Read `/Users/artisan/.agents/skills/impeccable/reference/onboard.md` and `reference/craft-floor.md`; no unrelated PRODUCT.md or DESIGN.md initialization is needed. After the final UI batch, the main agent runs `/Users/artisan/.agents/skills/impeccable/scripts/impeccable detect --json <changed UI targets>` once and stores the visual verdict.

## Write ownership

The implementer owns the welcome component/style, projects-page branch, projects locale entries and canonical page tests only. Main session owns planning, QA evidence, specs and commit/lifecycle. Other work is active in this checkout: never revert unrelated files, broaden into another feature, or change dependencies. Native sub-agent dispatch satisfies the repository's auto implement/check mode; no external Trellis channel is needed for a static bounded review.
