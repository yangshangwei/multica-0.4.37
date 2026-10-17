# Desktop core audit remediation

The user authorized fixing all nine findings in report.md and declined Trellis task creation for this session. This plan records the bounded change without creating or starting a task. Existing unrelated edits and release work remain owned by their authors.

## Behavior boundary

Restore keyboard input, visible focus, named controls and selection/disclosure semantics; make label foregrounds readable; provide a non-drag workflow-status ordering path; distinguish failed reads from successful empty/default data. Preserve the incumbent desktop UI, navigation, data identity, permissions, virtualization and drafts. No dependencies, backend changes, route changes or visual redesign are needed.

## Ownership and implementation

1. Keyboard/labels lane: F01 filter inputs in issues-header; F02 agent row focus; F03 board header action names; F04 label-chip contrast. Own adjacent tests and issues locale additions.
2. Settings/runtime lane: F03 member action names; F06 custom status ordering; F08 notification preferences and runtime usage recovery. Own settings/runtimes components, tests and those locale namespaces.
3. Inbox/project lane: F05 notification selection/focus semantics; F03 project Sheet naming; F09 disclosure states. Own inbox/project components, tests and those locale namespaces.
4. Main: F07 Gantt read recovery in the shared issue surface, integration review, locale coordination, desktop validation, specification update and final report.

Review refinements: the Gantt query in `packages/core/issues/queries.ts` must
erase denied cached data, with an AbortSignal guard against cancelled older
requests. ProjectDescription forwards the existing editor visibility prop while
retaining drafts. Archived status display disables ordering because backend
position normalization can cross an otherwise locally guarded archived boundary.

The smallest owning component/query boundary receives each repair. Reuse current collection recovery, menu key handling, reorder helpers, Base UI primitives and semantic tokens. Any local extraction must remove duplication or make a tested behavior explicit, without introducing parallel abstractions.

## Validation

- Reproduce behavioral defects in meaningful tests before implementation: menu typing, actual contrast ratios, virtual notification selection, ordering boundaries, cold failure/retry and cached refresh failure. Existing native audit evidence establishes the visual defects.
- Run targeted Vitest suites after each lane; verify localizations and component integration once lanes land.
- Run views/core/desktop type checks, scoped ESLint, git diff whitespace validation and Impeccable detector over changed targets. Do not modify unrelated failures silently.
- In one batched Electron inspection, verify original failing paths, both themes, 1440px window and native 900px/200% zoom. Preserve the real renderer and use isolated profile/network fixtures; no real data writes or agent execution.
- Check cached failure retention, retry-in-flight disabled states, 403/404 behavior where the existing resource guard applies, and keyboard focus continuity.
- Produce a structured visual verdict before any further visual iteration. Fix verified remaining issues in one batch; allow one final confirmation round.
- Record final changed files, simplifications, test results, native evidence and remaining verification limits. Stop the audit harness and remove its temporary profile.
