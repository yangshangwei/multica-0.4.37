# Project welcome design

## Surface and behavior

Operate mode. Preserve the established neutral Multica canvas, semantic typography, native controls and restrained status colors. Expand the empty projects body into an introduction and a passive project preview, following `TriageWelcome` and `IterationWelcome` rather than inventing a new visual world.

`ProjectsPage` owns the Query-derived welcome gate, documentation path and existing create-modal callback. `ProjectWelcome({ docsHref, onCreate })` owns only presentation. No new route, server request, store or seen flag. Hide the duplicate header creation action only while the welcome is visible.

Require successful, error-free, non-placeholder project data with an empty raw directory. Retained empty data after a failed refresh keeps the existing refresh alert and an ordinary empty state with the normal header action. Filtered empty results for an existing directory keep search/reset behavior.

## Content and composition

- Headline: “围绕一个目标，组织任务、智能体与 AI 小队” / “Bring issues, agents and AI squads together around a shared goal”.
- Intro explains common project context, optional repositories, and task assignment to an individual agent or an AI squad. Three small open rows explain goal/context, assignee choice and progress, without a grid of feature cards.
- Primary create button and secondary project-guide link use existing Button and AppLink primitives, with 44px targets.
- One preview figure labelled “项目示例” / “Example project”: “注册激活” / “Registration and activation”, a clearly illustrative account-service repository, three issue rows, one completed issue and matching 1/3 progress. Product agent owns flow definition, delivery AI squad owns registration API/activation email, QA agent owns expired-link/duplicate-signup validation.
- Every preview is static and outside the interaction order. Reuse status labels from the applicable namespaces. Do not use fake links, controls or claimed automatic completion.
- Container queries stack intro and preview in narrow desktop panes and Web windows. Semantic tokens cover both themes. Long names wrap and the body scrolls vertically without adding page overflow.

## Boundaries and trade-offs

Expected production edits: new `projects/components/project-welcome.tsx`, local module CSS and declaration if needed, the empty/header branch of `projects-page.tsx`, and `locales/{en,zh-Hans}/projects.json`. Existing tests protect loading/error/access/filter behaviors; extend the canonical page test for gating and interaction instead of duplicating matrices through a separate component suite.

Rejected: generic cross-domain welcome abstraction. The domains have distinct illustrations and existing consistent primitives; changing sibling pages would exceed this request.

Rejected: example auto-creation or template shortcuts. The user asked for explanation; real writes would add an unrequested workflow.

Rollback is removal of the local welcome component/import and restoration of the prior empty branch. No persistence or schema migration is involved.
