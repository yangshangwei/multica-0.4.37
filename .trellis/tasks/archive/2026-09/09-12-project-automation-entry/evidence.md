# Automation entry implementation evidence

Date: 2026-09-12
Worktree: `/Volumes/artisan/code/2026/multica-workspace-defaults`
Branch: `feat/workspace-defaults`

## Result

- The automation page renders the built-in catalog between its title and instance list, including the empty state. Each registry key opens the existing template configuration flow; custom creation remains available.
- `ProjectAutomationsSection` lists only this project's non-archived automations in the current workspace, displays status and the next run from existing list data, and opens existing automation details for pause/resume/run-now controls. Its add action carries the project and default squad into template setup without a mutation.
- Template setup accepts `initialProjectId`, `initialAssigneeType`, and `initialAssigneeId`. Both platform routes use the shared `TemplateCreateAutopilotRoute`, which reads `project_id`, `assignee_type`, and `assignee_id` through the navigation adapter and forwards these props. Template selection and Back preserve the URL defaults.
- Defaults seed only after workspace data and membership are available. Explicit user edits, including clearing the project while data is loading, win over defaults. Refetches and changed initial props do not replace edits. The form remounts its draft on a workspace change.
- Project, agent, and squad identities are checked against the current workspace. Assignees must be unarchived, runtime-bound, and invokable by the current member; squads additionally require an eligible leader. Admin editing access does not bypass private invocation rules. The final CTA rechecks current eligibility, so revocation disables it.
- The template prompt, cron, mode, and title remain server-controlled. Only **Enable automation** calls the existing transactional template creation mutation. No paused automation or trigger is pre-created.
- Added copy in English, Simplified Chinese, Japanese, and Korean using the existing `autopilots` namespace and semantic UI tokens.

## Project mount contract

```tsx
import { ProjectAutomationsSection } from "./project-automations-section";

<ProjectAutomationsSection
  projectId={project.id}
  defaultSquadId={project.execution_squad?.squad_id ?? null}
/>
```

The project lane owns placement below resources and all `project-detail.tsx` changes.

## Changed files

- `packages/views/autopilots/components/autopilots-page.tsx`
- `packages/views/autopilots/components/autopilots-page.test.tsx`
- `packages/views/autopilots/components/autopilot-template-catalog.tsx`
- `packages/views/autopilots/components/template-create-autopilot-page.tsx`
- `packages/views/autopilots/components/template-create-autopilot-page.test.tsx`
- `packages/views/autopilots/components/pickers/agent-picker.tsx`
- `packages/views/autopilots/components/index.ts`
- `packages/views/autopilots/template-create-defaults.ts`
- `packages/views/projects/components/project-automations-section.tsx`
- `packages/views/projects/components/project-automations-section.test.tsx`
- `apps/web/app/[workspaceSlug]/(dashboard)/autopilots/new/template/page.tsx`
- `apps/desktop/src/renderer/src/routes.tsx` (automation template import/element only)
- `packages/views/locales/{en,zh-Hans,ja,ko}/autopilots.json`
- `e2e/autopilot-template.spec.ts` (authorized integration follow-up)

## Verification

The initial regression run observed eight expected failures for absent catalog, presets, and Enable wording, plus the missing new project-section module; eight existing tests passed. Implementation then passed the focused suite.

Initial implementation verification command:

```sh
DEBUG_PRINT_LIMIT=700 corepack pnpm --filter @multica/views exec vitest run autopilots/components/template-create-autopilot-page.test.tsx projects/components/project-automations-section.test.tsx autopilots/components/autopilots-page.test.tsx autopilots/components/autopilot-dialog.project.test.tsx
```

Initial result: **4 files, 27 tests passed**, exit 0. Coverage includes read-only template payloads, no early mutation, project/squad prefill, navigation round trips, preserved edits on refetch/changed props/delayed data, private-leader rejection, permission revocation, project-scoped lists and retry, direct catalog entry, and the existing custom dialog's project binding.

Other implementation checks:

- `corepack pnpm --filter @multica/views typecheck` — exit 0. An earlier run saw concurrent project-lane errors, reported to the leader; the final run is clean.
- `corepack pnpm --filter @multica/views exec eslint autopilots/components/autopilots-page.tsx autopilots/components/autopilot-template-catalog.tsx autopilots/components/template-create-autopilot-page.tsx autopilots/components/template-create-autopilot-page.test.tsx autopilots/components/autopilots-page.test.tsx autopilots/components/pickers/agent-picker.tsx autopilots/components/index.ts autopilots/template-create-defaults.ts projects/components/project-automations-section.tsx projects/components/project-automations-section.test.tsx` — exit 0, no findings.
- Impeccable mechanical detector on the catalog, project section, template page, and automation list — exit 0, `[]` findings.
- `git diff --check` restricted to owned paths — exit 0.

## Simplifications and handoff

Shared URL parsing/preservation avoids separate Web/Desktop default implementations. The existing queries, permission rule, creation mutation, custom form, and detail controls are reused. No dependency, core API, server, or database change is introduced by this slice.

Browser screenshots, visual-verdict, Web/Desktop integration checks, and broader repository verification remain with the leader. The E2E integration follow-up below resolves the earlier selector and mock-fixture handoff.

## E2E integration follow-up

Ownership was explicitly extended to `e2e/autopilot-template.spec.ts` only. Its template CTA now selects `Enable automation`. The local login helper returns the actual workspace ID and authenticated user ID; these populate the mocked agent and created automation. The agent has the supported `workspace` visibility and a `public_to` workspace invocation grant. Runtime, agent listing, and creation remain intercepted, so no live daemon is introduced. All assertions forbidding template-controlled fields in the create body are preserved.

Verification: the installed TypeScript compiler API reported no syntactic or semantic diagnostics for this spec under strict ES2022 / ESNext / Bundler options, with no emit. `git diff --check -- e2e/autopilot-template.spec.ts` exited 0. No browser test was run, as requested; the leader owns the isolated integration run. `fixtures.ts`, `helpers.ts`, and other files were not modified by this follow-up.

No deployment, commit, push, real agent CLI, or account-consuming smoke test was run.

## P2 review follow-up: membership-query recovery

Reproduced the cold-form failure using the actual `useCurrentMember` hook: member loading failed while agents, squads, and projects succeeded, but the form showed no error or Retry. The previous always-admin hook mock concealed this state. Before the production change, the new regression failed with `Unable to find role="alert"` after asserting all four query statuses.

The template form now observes `memberListOptions(wsId)` alongside the existing membership hook, includes its failure in the existing choices error, and refetches membership on Retry. Suggested-assignee seeding waits for successful membership data and a known user/role; a load failure no longer consumes the pending default. Existing user edits and workspace/invocation guards remain intact.

Final post-review verification:

- The same four-file Vitest command above — **4 files, 28 tests passed**, exit 0. The new regression retries membership successfully, restores the suggested squad, preserves an explicitly cleared project, and confirms no creation occurs during recovery.
- `corepack pnpm --filter @multica/views typecheck` — exit 0.
- `corepack pnpm --filter @multica/views exec eslint autopilots/components/template-create-autopilot-page.tsx autopilots/components/template-create-autopilot-page.test.tsx` — exit 0.
- Scoped `git diff --check` — exit 0.

This follow-up changed only the template page, its focused test, and this evidence record. Leader-owned E2E files were not edited or run. The automation code is frozen after this verification pass.
