# Project Squad Workspace Implementation Plan

**Goal:** Restore the project's task focus with compact squad management and a complete first-issue path.

**Architecture:** Shared React views reuse existing core query/mutation/readiness APIs. Add only the small shared issue-surface and picker affordances required by the approved project flow. No backend, dependency, platform-routing or global-state changes.

**Tech Stack:** React 19, TanStack Query, existing Zustand view preferences, Base UI primitives, shared semantic CSS tokens, Vitest/Testing Library and existing browser tooling.

## Task 1: Preserve behavior and implement squad management

Owner: executor, only `packages/views/projects/components/project-squad-section.tsx`, its test, and a narrowly scoped new project-squad query/state hook if necessary. Root owns locale files and other components.

- Run the existing squad suite before edits.
- Add failing tests for a compact default presentation, opening the manager, setting a non-first default while preserving other candidates, and closed-manager availability/error visibility.
- Consolidate common query reads and roster observations, using getProjectSquadReadiness and full invocation checks once per candidate. Keep query errors distinct from readiness failures.
- Move existing configuration/recovery into a scrollable Sheet, with descriptions and accessible row menus. Keep per-squad creation ordinary and explicitly requested.
- Adapt existing action tests to navigate the manager/menu; retain meaningful business invariants rather than deleting coverage.
- Run the focused suite and relevant lint/typecheck. Report exact changed files and tests, without committing or changing other lanes.

## Task 2: Task-first empty/populated project flow

Owner: root. Files: `project-detail.tsx` and tests; a focused new `project-issue-surface.tsx` and tests if separation improves clarity; `issues/surface/issue-surface.tsx` and tests; `issues/components/issues-header.tsx`; `modals/issue-picker-modal.tsx` and tests; the corresponding English/Chinese project/modal copy.

- Add regression tests for authoritative empty project behavior in table/gantt, retained status failure/filter recovery and canonical new-issue defaults.
- Add isScopeEmpty and headerActions slots to IssueSurface and an actions slot to IssuesHeader. Preserve existing renderHeader consumers and all default header wiring.
- Connect the project-specific empty state and the single populated-state primary creation action through controller.openCreateIssue.
- Reuse IssuePickerModal for association. Await selection, keep failures retryable, disable duplicate writes, retain issue assignee/status and exclude already-associated/ineligible project assignments.
- Test async picker selection and project wiring, including query invalidation inherited from useUpdateIssue.

## Task 3: Integration, review and evidence

- Add/update locale entries in en/zh-Hans; run locale parity.
- Run `pnpm --filter @multica/views exec vitest run projects/components/project-squad-section.test.tsx projects/components/project-detail.test.tsx` plus new surface/picker tests.
- Run core project/default/readiness suites, affected-package lint/typecheck and appropriate broader checks. Inspect failures before choosing any extra scope.
- Inspect a current browser render of the real shared components at desktop and narrow sizes, normal eight-squad/empty, management, populated and failure states. Use fixture interception if authenticated project access cannot be safely reused; do not invoke real agents.
- Run the Impeccable mechanical detector once over finished markup, then visual-verdict with screenshots and store evidence in `.omx/state/project-squad-workspace/ralph-progress.json`.
- Independent review checks business boundaries, stale/pending state, key identity, permissions, focus and small-screen behavior. Fix material findings, rerun affected checks.
- Update `.trellis/spec/server/project-execution-squad.md` with the shared UI semantics and record verification.md. Commit only this task's owned paths using the Lore intent/trailers convention. Complete/archive the task only after required work is verified.

## Completion record

All three execution tasks are complete. See verification.md for fresh checks, browser evidence, review and scope limits. Task/source commits are recorded in task.json after archival.
