# Continuation E2E synchronization

- Date: 2026-11-22 (Asia/Shanghai)
- Scope: E2E navigation entries and capture readiness only. No production source, dependencies, runtime configuration, services, staging or commits changed by this lane.

## Changes in this continuation

1. `e2e/fixtures/sidebar-navigation.ts`: preserve the concurrently completed visible-trigger/hidden-desktop guard, wait for the actual compact responsive branch before toggling, then require the sheet to be in the viewport, opaque, transition-complete and hit-testable. A closed sheet no longer needs a mounted navigation landmark before its trigger can open it.
2. `e2e/global-scope-navigation.spec.ts`: reuse that helper and assert the requested locale. All captures now wait for finite application/popup animations, loaded fonts and two actual animation frames. This applies equally to Playwright Web screenshots and native `capturePage`. Rename the initial native window handle so it does not shadow the browser `window` inside `page.evaluate` during strict TypeScript checking.

All original restoration, filter, read-error recovery, explicit anchor, account creation, Feedback, query-only dismissal and native error assertions remain. No arbitrary delays, test skips or substituted business assertions were introduced.

## Existing entry sync reviewed

The previously changed E2E paths already use the shared New-menu entry helpers. Remaining direct New Issue buttons are explicitly scoped to content/header surfaces; dialog-title selectors remain unchanged. The pre-existing `e2e/agent-template-marketplace-desktop.spec.ts` edits were preserved and excluded from this lane's task-owned checks.

Reviewed task-related paths:

- `e2e/auth-qa.spec.ts`
- `e2e/auth.spec.ts`
- `e2e/collaboration-account-desktop.spec.ts`
- `e2e/collaboration-business-scenarios.spec.ts`
- `e2e/collaboration-creation-default.spec.ts`
- `e2e/collaboration-delivery-web.spec.ts`
- `e2e/collaboration-multi-account-closure.spec.ts`
- `e2e/collaboration-order-export-20261109.spec.ts`
- `e2e/collaboration-order-export-leave-20261109.spec.ts`
- `e2e/collaboration-recipient-picker-web.spec.ts`
- `e2e/core-business-agent-loops.spec.ts`
- `e2e/fixtures/collaboration-editing.ts`
- `e2e/fixtures/collaboration-knowledge.ts`
- `e2e/fixtures/collaboration-native.ts`
- `e2e/fixtures/collaboration-pay-creation.ts`
- `e2e/fixtures/collaboration-retained-attachments.ts`
- `e2e/fixtures/iterations-i1.ts`
- `e2e/global-scope-navigation.spec.ts`
- `e2e/helpers.ts`
- `e2e/issue-assist-flow.spec.ts`
- `e2e/issue-mutation-branches.spec.ts`
- `e2e/issues.spec.ts`
- `e2e/iterations-audit-desktop.spec.ts`
- `e2e/main-supplemental-qa.spec.ts`
- `e2e/mcp-desktop.spec.ts`
- `e2e/navigation.spec.ts`
- `e2e/quick-create-actor-picker-phase2.spec.ts`
- `e2e/quick-create-actor-picker.spec.ts`
- `e2e/quick-create-autonomy.spec.ts`
- `e2e/quick-create-blocked-feedback.spec.ts`
- `e2e/settings-preferences.spec.ts`
- `e2e/task-lifecycle-qa.spec.ts`
- `e2e/upstream-selected-fixes.spec.ts`
- `e2e/workspace-access-branches.spec.ts`
- `e2e/fixtures/sidebar-navigation.ts`

## Verification

- PASS: Playwright discovery across the 27 changed spec files: **116 tests**, with no test bodies executed by this lane. A final exact discovery of `e2e/global-scope-navigation.spec.ts` finds both Web/Desktop cases.
- PASS: strict TypeScript for the two files changed in this continuation using `pnpm exec tsc --noEmit --strict --skipLibCheck --esModuleInterop --allowSyntheticDefaultImports --module esnext --moduleResolution bundler --target es2022 --lib esnext,dom,dom.iterable --jsx react-jsx --types node --resolveJsonModule e2e/global-scope-navigation.spec.ts e2e/fixtures/sidebar-navigation.ts`.
- PASS: root-scoped ESLint on those two exact files: zero errors/warnings. Root ESLint uses the existing base config with explicit E2E dev-dependency handling and an invocation-only allowance for the existing cleanup `finally` throw; repository configuration is unchanged.
- PASS: `git diff --check -- e2e`.
- Baseline comparison: all **35** selector-synced E2E paths were linted, then their HEAD versions were linted with the identical root-scoped configuration. Both contain the exact same **6** findings: undeclared root `zod`/`@multica/core` imports in `fixtures/collaboration-pay-creation.ts`, and existing empty Playwright fixture destructuring in `mcp-desktop.spec.ts`, `quick-create-actor-picker-phase2.spec.ts`, `quick-create-actor-picker.spec.ts`, `workspace-access-branches.spec.ts`. No new findings; these were left outside scope. The broad lint is **not** reported as clean.
- An initial package-directory ESLint invocation ignored paths outside its base; it was discarded as verification and replaced by actual root-scoped ESLint API execution. An exploratory broad TypeScript invocation with insufficient DOM/ES library/JSX options was also not counted as passing; the final strict two-file command above passed.

Machine-readable lint comparison, final source hashes and full path list: `continuation-e2e-lint-comparison.json`. Discovery: `continuation-e2e-discovery.log`. Initial/baseline logs are retained without treating them as acceptance.

## Handoff

Main owns the rebuilt Web/Desktop runtime, actual journey execution and inspection of fresh compact screenshots. The R2 captures were insufficient visual proof because they showed outgoing navigation/transition frames despite passing functional checks; the new bounds/paint assertions must pass in the fresh journey. No browser run or runtime restart was performed by this lane.
