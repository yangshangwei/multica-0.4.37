# Project flow implementation evidence

Worktree: `/Volumes/artisan/code/2026/multica-workspace-defaults`
Branch: `feat/workspace-defaults`
Date: 2026-09-12
Status: implementation verified at the component and type boundaries; ready for leader integration review. No commits or pushes made.

## Behavior delivered

- New-workspace runtime completion goes to Projects on Web and Desktop. Connected setup still bootstraps Mika; skipped-runtime setup retains the welcome/install guide signal. Returning-user skip semantics remain intact.
- The create-project form shows Resources and Execution squad below the description and before the property toolbar. Lead remains a separate project property.
- New choices default to `feature-delivery`; catalog `squad_template_key` or `squad_id` prefills override retained drafts. Explicit `{}` or `null` drafts opt out. The registry forwards modal data.
- The shared picker offers built-in templates, invocable existing squads, and no squad. It follows the backend wire/invoke intersection without requiring edit permission: another owner's shared squad remains available to a regular member with invocation access. It uses the established machine-grouped runtime picker, core runtime eligibility, and actual `system_key=mika` identity for the preferred runtime. It does not guess between machines, does not silently rebind an incompatible retained runtime, and preserves a selection during a runtime-query error.
- Project creation submits the choice with resources in the normal create request. Missing runtime or failed preparation still opens the saved project for recovery.
- Project detail reads the complete squad status roster and checks every agent's invocation access and actual runtime against local resources using the core helper. Pending/paused/error query states disable dispatch, with query retry available while viewing or editing. Configuration requires current workspace choices and resource queries; an unavailable saved roster does not lock explicit replacement or clearing. Changing a default uses the scoped configure mutation and leaves existing issues alone. A failed malformed configuration without a retained template/squad cannot automatically retry as an empty clear.
- Project create completion is scoped to its mounted workspace and submitted singleton-draft identity. Unmounts, workspace changes, and edits during the request cannot cause a late result to clear a newer draft or navigate away from it. Pending description edits are flushed before both identity checks.
- Hand to squad opens ordinary `create-issue` with project, squad assignee, and `status: todo`, preserving the existing server trigger-preview flow.
- Normal project issue creation, including the center empty-state action, uses the configured invocable squad and `todo` as a fallback. Scope/query-plan, explicit caller, and group defaults take precedence. Explicit assignee/unassigned view constraints and scopes excluding squads suppress the project fallback; no positive prefill from saved-view filters was introduced.
- `ProjectAutomationsSection` mounts below Resources and receives project ID plus the saved squad ID.
- Projects has a prominent first-project action and setup hint. All added/changed project, modal, and onboarding copy exists in the four locales.

## Changed files

- `packages/views/modals/create-project.tsx`, its existing tests, `registry.tsx`, and new `registry-project-prefill.test.tsx`.
- New `packages/views/projects/components/project-squad-picker.tsx` and `project-squad-section.tsx`, with regression tests.
- `packages/views/projects/components/project-detail.tsx`, `projects-page.tsx`, and their tests.
- Leader-approved issue-surface extension: `types.ts`, `issue-surface.tsx`, `use-issue-surface-controller.ts`, `use-issue-surface-actions.ts`, and new `create-defaults.ts`, with surface/controller tests and a canonical node-only merge suite.
- `packages/views/onboarding/onboarding-flow.tsx` and new `onboarding-flow-completion.test.tsx`.
- Leader-approved label expectation updates in onboarding `components/step-shell.test.tsx`, `steps/step-runtime-connect.test.tsx`, and `steps/step-platform-fork.test.tsx`.
- `apps/web/app/(auth)/onboarding/page.tsx`, `apps/web/app/(auth)/workspaces/new/page.tsx`, and `apps/desktop/src/renderer/src/components/window-overlay.tsx`.
- `packages/views/locales/{en,zh-Hans,ja,ko}/{projects,modals,onboarding}.json`.

## Verification

Observed failing regressions before implementing the new behavior: Projects completion destination, visible setup fields, default/prefilled requests, runtime inheritance and authorization, retained query-failure selection, explicit task dispatch, actual-machine mismatch, preparation retry, future-only defaults, detail mounts, and registry prefill.

Final focused command:

```sh
DEBUG_PRINT_LIMIT=300 corepack pnpm --filter @multica/views exec vitest run modals/create-project.test.tsx modals/create-project-local-mode.test.tsx modals/registry-project-prefill.test.tsx projects/components/project-squad-picker.test.tsx projects/components/project-squad-section.test.tsx projects/components/project-detail.test.tsx projects/components/projects-page.test.tsx onboarding/onboarding-flow-completion.test.tsx onboarding/onboarding-flow-mode.test.tsx onboarding/steps/step-runtime-connect.test.tsx onboarding/steps/step-platform-fork.test.tsx onboarding/components/step-shell.test.tsx
```

Latest full-slice run: **12 files, 85 tests passed**, exit 0 (17:28 local run). Subsequent bounded recovery verification is recorded below. The pre-existing jsdom canvas `getContext` warning remains in onboarding shell tests.

Additional observed checks:

- `corepack pnpm --filter @multica/views typecheck` — passed.
- `corepack pnpm --filter @multica/web typecheck` — passed.
- `corepack pnpm --filter @multica/desktop run typecheck:web` — passed.
- Focused ESLint across all changed shared source/tests — exit 0, zero errors; two pre-existing exhaustive-deps warnings in `onboarding-flow.tsx` and `project-detail.tsx`. The new components have no lint warnings.
- `git diff --check` — passed.
- Locale key comparison — 49 execution-squad keys aligned in all four languages; added modal/page keys present.
- Impeccable mechanical detector over the six changed shared UI surfaces — `[]`, recorded in `impeccable-detect.json`.

## Reuse and remaining integration work

Reused existing template hooks, machine-grouped RuntimePicker, core selection/readiness helpers, the scoped React Query configure mutation, navigation adapters, and ordinary issue creation. Added no dependencies, alternate execution API, frontend store, or agent-running side effect.

The leader owns final backend integration, browser/narrow viewport screenshots and visual verdict, and the overall task-state/commit lifecycle. This lane did not run a real agent CLI, provision a real runtime, or claim screenshot verification. The picker/readiness suite uses explicit fakes; API authorization remains enforced by the backend.

## Follow-up review verification

Observed RED for the regular-member shared-squad choice, complete-roster query wiring, paused query states, availability retry, malformed-config retry, and all three stale-create-result cases before their fixes. Also observed RED for saving a replacement choice while local-resource constraints were unavailable; the editor now remains disabled until the required queries recover, and retry stays accessible.

After the fixes, the full focused command above passed 85 tests. `corepack pnpm --filter @multica/views typecheck` and focused ESLint on the six follow-up source/test files both passed; that focused lint run had zero warnings. `git diff --check` passed. Web/Desktop platform wiring did not change during this correction pass.

Confirmed `ProjectDetail` already renders `<ProjectSquadSection key={project.id} ... />`, so editing state is discarded when navigating to another project; no redundant reset mechanism was added.

## Deleted-default recovery correction

Added a regression with the saved squad absent from the current list and its roster query failing with HTTP 404. Both explicit clearing and selecting another existing squad failed first because the picker was disabled. Split the configuration prerequisites from saved-roster readiness: current workspace choices/resources still gate Save, while the old roster continues to gate dispatch.

Only `project-squad-section.tsx`, its test, and this evidence changed in this pass. `corepack pnpm --filter @multica/views exec vitest run projects/components/project-squad-section.test.tsx` passed **15 tests** (17:42 local run). Focused ESLint on the two files, views typecheck, and their diff whitespace check passed. No broader implementation changes were made.

## Normal project issue defaults and dialog visibility

Observed RED for the normal controller payload, center New Issue action, ProjectDetail fallback wiring, and partial assignee overrides mixing the fallback's type/id with a newer selection. Added `fallbackCreateDefaults` through the existing surface/controller path. A shared merge helper is used at both merge points; any assignee override owns the complete pair, and cleared/incomplete identities become unassigned.

ProjectDetail only supplies fallback data for a configured, existing, unarchived squad with an invocable leader. The merge order is fallback, scope/query plan, explicit caller, then action/group defaults. Current view-store assignee/include-unassigned constraints include saved-view state; those constraints and a Members-only scope suppress the project fallback entirely. This does not update existing issues or bypass the ordinary create modal and server trigger preview.

The browser's persisted 78/revise verdict and both reference screenshots were inspected before the bounded layout correction. `ContentEditor` owns a `min-h-full` root; placing it directly in the modal's scrolling body made it consume the full body before Resources and Execution squad. It now sits in a definite scrolling region (`h-24`, `h-48` when expanded), preserving long-description editing while exposing project configuration on open. The leader owns the updated viewport assertions and screenshots.

Verification (18:12 local run):

```sh
DEBUG_PRINT_LIMIT=300 corepack pnpm --filter @multica/views exec vitest run issues/surface/create-defaults.test.ts projects/components/project-detail.test.tsx modals/create-project.test.tsx issues/surface/use-issue-surface-controller.test.tsx issues/surface/issue-surface.test.tsx
```

Result: **5 files, 96 tests passed**. Views typecheck passed. Focused ESLint across the new/modified surfaces and modal passed with one pre-existing `project-detail` effect-dependency warning; no new warnings. The scoped diff whitespace check passed.
