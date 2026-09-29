# ADLC Collaboration Implementation Plan

> Execution: implement the approved design in this session, with a bounded executor and independent review. User already authorized implementation.

**Goal:** Make the About You lifecycle explanation show a linear SDLC becoming a goal-driven collaboration graph once on entry.

**Architecture:** Keep all business markup and local animation lifecycle in the existing shared DevelopmentLifecycle component. Reuse installed motion/react or native CSS/WAAPI; keep any shared CSS in packages/ui/styles. Preserve semantic final content and localized labels. Bound the morph inside a stable diagram region to prevent onboarding scrollbar shifts.

**Tech Stack:** React, TypeScript, existing motion, Tailwind semantic tokens, Lucide, Vitest, Playwright.

## Motion thesis
- Focal moment: six sequential stages move/recompose into goal, shared context, three collaboration paths, and human delivery.
- Continuity: retain node identity during the transformation and reveal branch/feedback connections as it settles.
- Budget: one 2–3 second sequence on component entrance, no infinite animation, no questionnaire-triggered replay, cleanup on unmount. Reduced motion goes directly to final content.
- A11y: expose readable final meaning independently of animation; decorative paths and transient duplicate labels are hidden from assistive technology.

## Task 1: Implement shared diagram
Files: packages/views/onboarding/components/development-lifecycle.tsx; packages/views/locales/{en,zh-Hans}/onboarding.json; packages/ui/styles/base.css or a scoped shared stylesheet if needed; adjacent development-lifecycle.test.tsx.
1. Read package guidance and existing About You tests. Lock meaningful new lifecycle behavior with failing tests, avoiding assertions that just mirror CSS or mocks.
2. Replace two equal six-column rows with the approved branching diagram. Use brand and surface tokens, caption/body scale, stable geometry, readable mobile layout and feedback path.
3. Add one entrance morph and a reduced-motion static path; preserve labels on rerender and cancel pending work on unmount. Avoid extra global state, replay controls, or generic animation frameworks.
4. Update both locales together. Run the adjacent suite plus step-about-you and locale parity; run affected lint and typecheck.

## Task 2: Integrate and verify
1. Use a checkout-owned local environment, or a temporary isolated actual-component harness if the app environment is unavailable. Do not trust stale registered server ownership.
2. Inspect start/mid/final animation, final Chinese/English, narrow/wide and light/dark, reduced motion, and questionnaire rerender behavior.
3. Record visual verdict in .omx/state/adlc-collaboration/ralph-progress.json before any visual correction. One grouped fix pass and confirmation.
4. Independently review requirements, then code quality; resolve material findings. Check diff boundaries, report changed files and verification evidence.

## Rollback
Revert only this task's component, localized labels, and scoped styles. No migration or persistence changes exist.
