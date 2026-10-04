# Restore the V2 welcome illustration

**Goal:** Restore the software-development welcome page selected by the user from commit `510a85b36` (2026-09-29).

**Approach:** Restore the five activity cards, provider avatars, issue IDs, localized issue statuses, timestamps, staggered arrangement and bilingual caption. Preserve the current responsive intro, 44px actions, cancellation, pending states and workspace terminology. Keep all other onboarding copy and behavior unchanged.

**Scope:** `packages/views/onboarding/steps/step-welcome.tsx`, its existing test, and the illustration/caption keys in `packages/views/locales/{en,zh-Hans}/onboarding.json`. No dependencies or backend changes.

1. Restore the existing bilingual status regression tests and verify they fail against the six-stage illustration.
2. Restore the V2 illustration and caption; remove the superseded six-stage keys and approval-only presentation.
3. Run welcome/onboarding regression tests, locale parity, views typecheck, scoped ESLint, whitespace checks and the UI detector. Render the historical reference and restored component; inspect desktop, narrow, short-window and dark layouts and check actions.

**Evidence:** Save isolated browser previews and verification results under `.omx/state/welcome-v2-restore/`. Do not modify unrelated work or publish a release.

## Verification

- Red/green: restored status tests failed against the six-card implementation, then passed after restoration.
- Welcome, onboarding mode/completion and locale parity: 4 files, 76 tests passed.
- Views TypeScript check and scoped ESLint passed; whitespace check and mechanical UI detector had no findings.
- Both locale illustration objects and captions exactly match `510a85b36`; all other locale content matches the pre-change HEAD.
- Browser: 11 cases covering widths 320–2048, Chinese/English, web/desktop props, dark mode, short windows; enlarged text, reduced motion, keyboard continue, web continue, download target, cancel, skip and pending-state checks passed.
- First browser pass found the historical architecture card extending 1.7px beyond the 1024px panel. Increasing panel padding from 32px to 40px removed clipping; final browser errors: none.
- Visual verdict: 96/pass; evidence is in `.omx/state/welcome-v2-restore/`.
- Scope/limits: browser validation mounts the real shared component with current styles and local callbacks; no authenticated end-to-end navigation, backend changes, release or deployment. The optional delegated reviewer could not start because its configured model is unavailable; the source diff was reviewed directly.
