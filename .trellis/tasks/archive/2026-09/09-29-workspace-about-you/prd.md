# Show About you when creating a workspace

## Problem

Creating a workspace currently opens the workspace step and marks About you as
completed without displaying it. The user wants to see About you first.

## Acceptance criteria

- New-workspace mode opens About you and pre-fills saved questionnaire answers.
- Continue or Skip moves to the workspace creation form, never an existing workspace.
- Back from workspace returns to About you; Back from About you cancels the flow.
- The progress rail allows returning to About you before workspace creation.
- After workspace creation, runtime-step back navigation remains disabled.
- First-run onboarding still opens Welcome; completion still lands on Projects.
- Add regression coverage and run onboarding tests, scoped lint, and typecheck.

## Scope

Use the shared onboarding flow for web and desktop. No new dependencies or visual
redesign. Preserve unrelated work already present in the checkout.

## Verification

- Before the fix, the new mode regression suite failed 5 tests because About you
  was absent. After the fix, all 100 tests in 16 onboarding-related files pass.
- Views typecheck passes. Scoped ESLint has no errors and one existing
  `handleWelcomeSkip` dependency warning for `t`; no unrelated lint changes made.
- Independent read-only review found no defects in entry, navigation, persistence,
  cancellation, runtime locking, or completion behavior.
- Spec review: existing shared-component and regression-test rules cover this fix;
  no new architecture convention is needed. The entry contract is documented in
  the flow props and tests.
- Native UI automation is unavailable in this session; verification used the real
  shared flow rendered in DOM tests, not a live desktop click-through.
- User acceptance: on 2026-09-29, the user manually tested the fix, confirmed it
  works, and requested committing the code and archiving the task.
