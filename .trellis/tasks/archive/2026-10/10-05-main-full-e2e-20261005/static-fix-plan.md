# Existing E2E strict-type repair

## Scope

Repair the six existing TypeScript errors reported by the standalone strict E2E compile. Ownership is limited to `iframe-scroll-bridge.spec.ts`, `onboarding-shell.spec.ts`, `quick-create-actor-picker.spec.ts`, `retained-languages-desktop.spec.ts`, and `upstream-backports-cli.spec.ts`.

## Plan

1. Inspect the reported values and preserve each test's existing assertions and behavior.
2. Use named evaluation arguments to preserve their individual types, checked token/entity narrowing, a checked desktop bridge shape, and checked child-process exit-code handling.
3. Run the standalone strict TypeScript compile for the five owned specs, inspect the exit code and output, and review the final diff.

No production files, dependencies, browser execution, service state, or other agents' edits are included. Existing tests are the behavior contract; this repair does not add redundant browser tests for type guards.

## Verification

- The five owned specs passed `pnpm exec tsc --noEmit --target ES2022 --module ESNext --moduleResolution bundler --esModuleInterop --skipLibCheck --strict` (exit 0).
- Evidence: `.gstack/qa-reports/2026-10-05-branch-expansion/static-fix-typecheck.log`; only the existing pnpm configuration warning remains.
- `git diff --check` passed for all five specs. The reviewed diff preserves product assertions and adds no casts.
- Browser execution was intentionally left to the running parent regression suite. No new repository-spec rule emerged from these localized type repairs.
