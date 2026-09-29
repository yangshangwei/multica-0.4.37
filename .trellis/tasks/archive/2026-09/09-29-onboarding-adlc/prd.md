# ADLC onboarding

## Goal
Make the right-hand About you step explain the shift from SDLC to an AI-driven development lifecycle, then help the user choose their role and starting goals.

## Requirements
- Show a concise, readable SDLC-to-ADLC comparison using the supplied lifecycle reference as content guidance.
- Explain goals/PRDs, context/skills, agent orchestration, coding, continuous testing, human review, and feedback.
- Frame role and use-case questions around software development while retaining the existing personalization meanings and IDs.
- Preserve single-select roles, multi-select goals, Other inputs, persistence, Continue validation, Skip, and the left progress rail.
- Support English and Simplified Chinese, light/dark themes, and narrow screens without horizontal overflow.
- Preserve the existing uncommitted welcome-screen edits and add no dependencies.

## Acceptance
- The evolution from SDLC to ADLC is visible before the profile questions.
- The comparison distinguishes human direction/review from agent execution without promising unimplemented automation.
- Existing selection and navigation tests pass with updated visible copy.
- Type checking, scoped lint, locale parity, and browser checks pass or have precise unrelated limitations recorded.
