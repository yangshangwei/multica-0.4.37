# Project directory welcome

## Owners and signatures

Shared Web/Desktop `ProjectsPage` owns the workspace-scoped project directory
query, loading/access/refresh feedback and create/documentation actions.
`ProjectWelcome({ docsHref: string, onCreate: () => void })` owns presentation
only: bilingual introduction, process notes and a passive registration and
activation project example. It must not create data, query an example, dispatch
an agent or persist a seen flag.

## Directory state contract

The welcome requires `projectQuery.isSuccess`, non-placeholder data and an empty
raw project directory. React Query changes status to error when a refresh fails
even when cached data remains. Do not replace this affirmative check with
`!isLoading && projects.length === 0`: defaulted empty arrays and retained empty
data do not prove a successful read.

| Directory observation | Presentation |
| --- | --- |
| Successful, non-placeholder empty array | Project welcome |
| Initial pending query | Existing loading/list skeleton |
| Initial read failure with no data | Load error and retry |
| Failed refresh with retained empty data | Refresh alert, ordinary empty state and header creation |
| Failed refresh with retained projects | Refresh alert and retained rows/filters/selection |
| Placeholder empty data | Ordinary empty/loading presentation, never welcome |
| Workspace project access denied | Existing permission-loss feedback |
| Raw directory contains projects but filters match none | No-results state and reset action |

No separate first-ever-use claim is possible: deleting every project can return
the directory to the successful empty state. No extra history request is needed.

## Actions and example semantics

The welcome primary button invokes the same `openCreateProject` callback as the
normal header, opening `create-project`. Hide the duplicate header action only
while the welcome is visible. The optional help link uses `AppLink` with
`wsPaths.docsPage(DOCS_SLUGS.projects)` so desktop navigation stays in the app.

Repositories are optional. Each issue can have a member, individual agent or AI
squad as its selected assignee. Project lead is separate from task assignment;
creating a project or browsing this example never starts execution. Do not imply
that choosing candidate squads broadcasts an issue to all of them.

The figure is explicitly illustrative. Its three task rows show product agent,
delivery AI squad and QA agent as separate assignees. Completed count and bar
percentage derive from the same static rows. Rows contain no interactive links
or controls. Status text accompanies color; the progress bar has an accessible
label and value.

## Layout and verification

Use the existing semantic color and role-based typography tokens. Container
queries respond to actual content width, including narrow Desktop tabs. Stack
the introduction and figure when space is limited, allow vertical scrolling and
keep actions at least 44px high. Translated headline terms must remain readable
without clipping or breaking Chinese product words in half.

`projects/components/projects-page.test.tsx` is the canonical regression owner
for welcome gating, create-modal/docs wiring, passive example, access and
refresh/no-results recovery. `locales/parity.test.ts` owns key parity. Browser
evidence records bilingual light/dark wide/narrow views, keyboard creation,
actual docs navigation and the transition to a populated directory. A shared
component and passing Desktop typecheck prove integration, not native Electron
visual acceptance; report the renderer actually used.
