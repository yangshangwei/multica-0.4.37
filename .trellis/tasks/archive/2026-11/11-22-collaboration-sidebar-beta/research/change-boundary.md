# Change boundary and verification plan

- Behavior gap: collaboration is permanently expanded and has no maturity notice; the approved design presents a quiet collapsed directory with explicit Beta context.
- Behavior owners: shared `packages/views/layout/app-sidebar.tsx`, `sidebar-navigation-group.tsx`, core `navigation/sidebar-preferences.ts`, and the collaboration page header.
- Expected edits: those owners and their canonical tests; supported `collaboration.json` locale files; `notifications.tsx` only if needed to retain authoritative collapsed unread feedback without parallel state; focused E2E selectors only where collapse changes the existing journey's required navigation.
- Reuse existing disclosure primitives/preferences, protected notification query and global resume callbacks. Any small refactor must preserve other navigation group semantics and remain covered by their existing tests before editing.
- Exclusions: no platform routing rewrite, dependency change, backend change, feature gate or whole-sidebar cleanup.
- Verification: existing group regression suites first; new behavioral cases for collaboration disclosure and legacy preference hydration; related sidebar/workbench/notification/parity tests; core/views/Web/Desktop type checks and affected-file lint; one batched baseline/candidate visual review with at most one correction round.
- Baseline tracked user changes: `apps/web/next-env.d.ts`, `docs/plans/2026-11-18-pay-241-continuation.md`, `e2e/agent-template-marketplace-desktop.spec.ts`. Do not stage or revert these changes or unrelated untracked artifacts.

## Verification lint repair

The final review found an existing `no-unsafe-finally` violation in the already-modified `e2e/global-scope-navigation.spec.ts`: its outer test cleanup explicitly rethrows a protocol-row transaction error. Keep the affected file lint-clean by moving only that exact transaction cleanup into one awaited named helper. Preserve statement order, rollback/rethrow behavior, connection cleanup and subsequent workspace/account cleanup. No product behavior or other test is in scope; the existing two production journeys verify this cleanup path.
