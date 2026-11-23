# Regression test maintenance plan

## Scope and baseline

Fix the stale tests listed by the user after the 2026-09-30 QA run. Current baseline is `f03c78cf7`; original failure evidence and verified selector changes are in `.gstack/qa-reports/2026-09-30-full-e2e/`. Do not change product behavior, dependency manifests, unrelated tasks, or tests exposing the separate device-login and accessible-name defects.

## Changes

1. Update sidebar/MCP names, the squad creation chooser, skill card/list assertions, and the second-workspace welcome step against current components.
2. Keep `issue-assist-flow.spec.ts` as the canonical AI refinement suite. Remove three obsolete preview/adoption tests and their unused fixture; migrate any missing valid coverage, especially saving an unchanged manual draft after provider failure. Retain undo, cancellation, stale-edit preservation, persistence and dispatch assertions.
3. Repair three component tests: assert the semantic project icon, provide the current actor-hook mock shape, and scope the member selector to its popover.
4. Preserve original imports and fixture isolation. Do not add skips, retries, soft assertions or obsolete UI fallbacks to manufacture passes.

## Verification

- Reproduce the three component failures; rerun affected suites and full views Vitest.
- Build the current committed app in a dedicated worktree and run changed E2E files plus the canonical AI suite against a real API/database and deterministic provider.
- Run lint/typecheck and inspect the final diff. Report unrelated product failures separately.
- Preserve unrelated edits and stop task-owned services.

## Ownership

- Unit-test lane: the three named views component tests.
- E2E-selector lane: navigation, settings-preferences, missing-user-session, localized-template-defaults.
- Leader: AI consolidation, supplemental onboarding, documentation, integration and verification.
