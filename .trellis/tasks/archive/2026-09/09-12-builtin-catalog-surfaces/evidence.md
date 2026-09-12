# Built-in catalog implementation evidence

Worktree: `/Volumes/artisan/code/2026/multica-workspace-defaults`

## Delivered

- Squads, agents, and skills show the real server catalog immediately below the page header, before the independently filtered workspace instances. Templates remain visible in empty workspaces and are excluded from instance counts and selections.
- Catalog entries show their purpose and enter the existing agent-template creation route or skill-template preview/copy flow. Instance links require `template_key`, a recognized built-in skill origin, or explicit skill `template_source`; matching a name alone is insufficient. Archived agent/squad instances are excluded.
- `UseSquadForProjectDialog` chooses an existing project or opens `create-project` with `squad_template_key`. Existing project configuration calls the real `useConfigureProjectSquad(wsId)` mutation only on confirmation.
- Runtime choices use `projectLocalDaemonIds`, `eligibleProjectRuntimes`, and `selectProjectRuntime` from core. The dialog waits for project resources, uses `system_key === "mika"` for the preferred assistant runtime, supports an explicit later connection, and does not select the first of several machines.
- Successful project responses navigate to the project for its configured, needs-runtime, or failed/retry state. Rejected requests retain the choice. Late responses cannot navigate or close a dialog after a workspace change or unmount.
- Query failures have retry actions. Pending catalogs, including paused requests, are not shown as empty. A failed squad-instance query is distinct from an empty workspace.
- `CreateSkillDialog.initialTemplateName` seeds only the preview and retains the existing draft, discard, and uncertain-submission lifecycle. An edited copy survives locale and catalog updates.
- `templateLanguageFor` now returns the supported `en | zh | ja | ko` union, normalizes regional locale variants, and uses the backend's English fallback for unknown locales.

## Changed files

- `packages/views/common/builtin-template-catalog.tsx`
- `packages/views/{squads,agents,skills}/components/builtin-{squad,agent,skill}-catalog.tsx`
- `packages/views/{squads,agents,skills}/components/{squads,agents,skills}-page.tsx` and their focused tests
- `packages/views/projects/components/use-squad-for-project-dialog.tsx` and its focused test
- `packages/views/skills/components/create-skill-dialog.tsx` and `create-skill-template-flow.test.tsx`
- `packages/views/agents/create/use-role-templates.ts` and its new pure locale test
- `packages/views/locales/{en,zh-Hans,ja,ko}/{squads,agents,skills}.json`

The leader explicitly approved the creation-dialog and locale-helper scope additions. Core, project creation/detail, onboarding, automation, and other locale namespaces were not edited in this lane.

## Regression evidence

Before implementation, the existing agent/skill page suites passed (2 files, 12 tests). The six new catalog tests then failed because the catalog regions did not exist. The new preselected-skill test failed because the dialog still opened the method chooser. The locale normalization test initially failed in six regional/unknown cases. The final error-path regressions failed for paused catalogs and failed squad-instance queries before those paths were corrected.

All commands below ran from the worktree root, without real agent CLIs or network writes.

```sh
corepack pnpm --filter @multica/views exec vitest run squads/components/squads-page.test.tsx agents/components/agents-page.test.tsx agents/create/use-role-templates.test.ts agents/create/template-create-agent-page.test.tsx skills/components/skills-page.test.tsx skills/components/create-skill-template-flow.test.tsx projects/components/use-squad-for-project-dialog.test.tsx locales/parity.test.ts
```

Passed: 8 files, 236 tests. After the two final query-state regressions were added and fixed, all affected suites were rerun:

```sh
corepack pnpm --filter @multica/views exec vitest run squads/components/squads-page.test.tsx agents/components/agents-page.test.tsx skills/components/skills-page.test.tsx locales/parity.test.ts
```

Passed: 4 files, 190 tests. The unchanged project-dialog, copy-flow, and locale-helper suites retain their passing results from the comprehensive run.

```sh
corepack pnpm --filter @multica/views typecheck
```

Passed on the final code (exit 0). Earlier failures from parallel files and the old string-valued language helper were resolved before the final run.

```sh
corepack pnpm --filter @multica/views exec eslint common/builtin-template-catalog.tsx agents/components/builtin-agent-catalog.tsx agents/components/agents-page.tsx agents/components/agents-page.test.tsx agents/create/use-role-templates.ts agents/create/use-role-templates.test.ts skills/components/builtin-skill-catalog.tsx skills/components/skills-page.tsx skills/components/skills-page.test.tsx skills/components/create-skill-dialog.tsx skills/components/create-skill-template-flow.test.tsx squads/components/builtin-squad-catalog.tsx squads/components/squads-page.tsx squads/components/squads-page.test.tsx projects/components/use-squad-for-project-dialog.tsx projects/components/use-squad-for-project-dialog.test.tsx
git diff --check
```

Both passed on the final code (exit 0).

```sh
node /Users/artisan/.agents/skills/impeccable/scripts/detect.mjs --json packages/views/common/builtin-template-catalog.tsx packages/views/agents/components/builtin-agent-catalog.tsx packages/views/skills/components/builtin-skill-catalog.tsx packages/views/squads/components/builtin-squad-catalog.tsx packages/views/projects/components/use-squad-for-project-dialog.tsx
```

Returned `[]`. The UI uses the existing semantic tokens, button/dialog/select primitives, and runtime machine grouping.

## Simplifications and remaining verification

One shared catalog layout owns loading, error, and empty presentation. Domain components own their existing registry query and metadata identity. Existing agent routes, skill-copy lifecycle, project picker, navigation adapter, and core runtime helpers are reused. No dependencies or stores were added.

Browser screenshots, narrow/Desktop visual verdicts, and the integrated backend flow remain with the parent verification lane. These checks did not execute a real runtime or agent. No commit or push was performed.
