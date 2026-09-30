# Welcome Agent Handoff Implementation Plan

**Goal:** Apply approved copy option one to the welcome illustration in Chinese and English.

**Architecture:** Keep the existing shared StepWelcome and stacked card style. Show six chronological activity snapshots: human request, requirements confirmation, architecture, implementation, blocked security review, and QA awaiting human release approval. These are illustrative messages, not live workflow controls.

**Tech Stack:** React, TypeScript, existing i18n bundles and Lucide icons.

## Steps
1. Update the existing bilingual status regression tests for the approved stage labels.
2. Update both onboarding locale bundles and render the six cards with highlighted mentions. Use explicit illustrative stage labels and existing semantic colors.
3. Run welcome tests, locale parity tests, scoped lint and TypeScript checks. Inspect rendered desktop and narrow layouts; preserve readable text and existing responsive behavior.

## Constraints
No backend workflow changes, new dependencies, or edits to unrelated in-progress work. Preserve all approved Chinese copy and the current provider-avatar style.

## Verification
- Views TypeScript check and scoped ESLint passed.
- Welcome localization and locale parity suites: 62 tests passed.
- Design detector returned no findings; git diff whitespace check passed.
- Browser-mounted the actual shared component with production desktop styles and locale resources at widths 1440, 1024, and 390. Confirmed no horizontal overflow and release approval remains reachable by scrolling. Fixed the existing height constraint that otherwise clipped the sixth card.
- This validates the illustration, not authenticated onboarding navigation or backend release enforcement.
