# Project tasks first with compact AI squad management

## Authorization and problem

On 2026-09-28 the user requested a new Trellis task and design/implementation based on the project multi-squad review. The accepted review is `.impeccable/critique/2026-09-28T11-52-28Z__iews-projects-components-project-squad-section-tsx.md`.

Eight configured squads currently occupy most of the project viewport. Each repeats runtime information and a primary creation action. The project should prioritize creating, viewing and progressing issues while preserving access to squad configuration and availability recovery.

## Scope

Implement the review's first delivery: a bounded squad summary, a focused management sheet, explicit future-issue default selection, one primary issue creation action, a project-specific empty state and association of an existing issue. Preserve shared Web/Desktop behavior and the existing theme.

This delivery does not add orchestration, auto-routing, project approval metrics, backend endpoints or dependencies. Existing exact issue status counts/views remain available. A later project-attention feature must use real issue/execution/permission data rather than squad readiness; it is outside this task's completion criteria.

## Requirements

1. With 1, 8 or 20 squads, the normal summary remains one or two compact rows instead of rendering all configuration rows above issues.
2. Summarize the squad count, current availability and the future-issue default. Unknown/loading/error data must never imply ready or zero failures. A default failure remains visible while the manager is closed.
3. A keyboard-accessible management sheet provides the full list, actual squad descriptions, details, add/change/remove/retry/connect and an explicit Set as default action. Normal runtime details and infrequent actions are disclosed on demand.
4. Reordering the default replaces the existing ordered selection with the selected item first, preserving identities/provenance and all other items. Existing issues are not reassigned. Removal only removes the project association.
5. The main page does not render one dispatch button per squad. Populated projects have one main New issue action using the canonical IssueSurface controller; the manager may offer a secondary action for a specific squad.
6. A genuinely empty project shows Create your first issue and Link an existing issue. Table and Gantt receive the same project empty experience without fabricating a client filter.
7. Filtered empty, actor tabs, saved views, pending data and status-catalog failures retain their recovery/navigation affordances.
8. Linking an existing issue awaits persistence, preserves its assignee/status, refreshes project/issue queries, shows failure and allows retry. Already-associated issues are excluded; assigning an issue from another project must be explicit to the user.
9. English and Simplified Chinese copy stay aligned. Use semantic tokens and existing components. Sheet contents scroll independently; long names and small viewports remain usable.

## Verification

- Existing squad dispatch/readiness/recovery and project create-default suites remain green after adapting interaction paths.
- New behavior tests cover collapsed content, manager interaction, default ordering, errors while closed, per-squad create, no automatic dispatch, filtered vs true empty, async link success/failure and no reassignment.
- Run views/core focused tests, locale parity, relevant lint and TypeScript checks; expand to repository checks where appropriate.
- Verify representative desktop/narrow screenshots and interaction flows. Record a visual verdict relative to the approved design and the supplied screenshot's incumbent style.

## Completion

Implementation, independent review, verification evidence and updated domain guidance are recorded in this task; only this task's changes are committed. Unrelated work already present in the checkout is preserved.
