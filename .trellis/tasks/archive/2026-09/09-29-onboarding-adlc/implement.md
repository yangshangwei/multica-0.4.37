# ADLC Onboarding Implementation Plan

**Goal:** Explain SDLC to ADLC and make the optional profile questions relevant to AI-assisted software delivery.

**Architecture:** Add one presentation-only comparison in the shared onboarding view. Widen only the About you column to fit its two lifecycle tracks; keep all questionnaire state and persistence handlers intact. Reuse existing option controls and theme tokens.

**Tech stack:** React, TypeScript, Tailwind semantic tokens, i18next, Vitest, Playwright.

## Execution
1. Baseline existing About you, shell, and locale parity tests. Result: 86 tests passed.
2. Add `packages/views/onboarding/components/development-lifecycle.tsx`: compact SDLC sequence and ADLC sequence with a feedback return cue; collapse to three columns on narrow screens.
3. Update `steps/step-about-you.tsx`, the About you width in `components/step-shell.tsx`, and English/Chinese onboarding copy. Keep role/use-case IDs and handlers unchanged; add single/multiple selection hints.
4. Update existing component and E2E text locators for the revised copy without adding implementation-mirroring tests.
5. Run scoped Vitest, views typecheck/lint, diff checks, and the Impeccable detector. Inspect desktop, narrow, tablet, and English/dark variants; record visual verdicts under `.omx/state/onboarding-adlc/`.
6. Fix any concrete issues from that pass and confirm once. Record changed files, evidence, and remaining limitations.

## Tradeoffs
- A compact flow comparison preserves onboarding speed; the supplied dense infographic would overwhelm this form.
- Display copy may become development-focused, but stage names must never replace unrelated persisted use-case meanings.
- Existing shared UI tokens establish the visual style; no new global design system, dependencies, or backend changes are needed.

## Final implementation
- Added `packages/views/onboarding/components/development-lifecycle.tsx` with SDLC and ADLC tracks plus a feedback cue. Its container query uses six stages in a row only when the figure itself has enough room; tablet sidebars otherwise leave too little space.
- Updated `packages/views/onboarding/components/step-shell.tsx` with a wider About you column and an optional footer class override.
- Updated `packages/views/onboarding/steps/step-about-you.tsx` to show the comparison and single/multiple selection hints. All selection, validation, skip and persistence handlers remain intact.
- Updated only the questions/lifecycle sections of `packages/views/locales/en/onboarding.json` and `packages/views/locales/zh-Hans/onboarding.json`; the pre-existing welcome edits are outside this task.
- Updated visible-copy locators in `packages/views/onboarding/steps/step-about-you.test.tsx`, `e2e/onboarding-shell.spec.ts`, and `e2e/onboarding-smoke.spec.ts`. The Projects navigation assertion now allows 20 seconds for first-visit Next.js development compilation, consistent with the runtime-step wait.

## Verification notes
- Final checks passed: 147 tests across 16 onboarding/locale suites; views TypeScript check; scoped ESLint; all five onboarding Playwright scenarios; `git diff --check`. The Impeccable detector returned no findings.
- Browser evidence is in `.omx/state/onboarding-adlc/`: Chinese/English desktop and mobile, dark desktop, and English tablet at 768px/820px. The final visual verdict is 95/100. Browser checks report no page errors, no overflowing controls, and no overlapping stage labels in the corrected tablet layouts.
- The registered local environment is `check-20260928023904-13728`: web port 13493, API port 18573. Start its API with `make up C=api ENV_FILE=/Users/artisan/.multica/dev/envs/check-20260928023904-13728/check.env`; loading the checkout's default `.env` used port 13492 for CORS and prevented browser authentication.
- The first Projects navigation can trigger development compilation longer than Playwright's default five-second assertion window. The warmed flow passed, and the existing assertion now waits for the same destination with a bounded 20-second timeout.
- A read-only review identified the 768/820px sidebar breakpoint. Browser text-range measurements reproduced four overlapping stage labels; container queries removed all overlaps.
- Existing spec rules cover this change; no new cross-project rule or dependency was introduced. Native desktop runtime was not separately launched.
