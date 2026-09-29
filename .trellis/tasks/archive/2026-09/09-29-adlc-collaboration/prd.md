# ADLC collaboration entrance

## Approved request
Implement concept 1's branching collaboration layout with concept 2's one-time SDLC-to-ADLC entrance morph. User explicitly approved implementation on 2026-09-29.

## Acceptance
- About You keeps its heading, questionnaire, navigation, and existing data semantics.
- A quiet SDLC reference precedes a clear ADLC collaboration diagram: goal/PRD on the left, shared context/skill above, coding/testing/agent review as three collaborative paths, human review/delivery on the right, feedback returning to goal. Agent orchestration is explicit.
- A short, bounded entrance visibly changes the linear SDLC presentation into the collaboration layout; ends within roughly 2–3 seconds and does not replay on questionnaire updates.
- Reduced-motion users see the final diagram without movement or waiting.
- Both English and Simplified Chinese are complete; narrow and wide containers, light and dark themes have legible labels and no overflow or moving page geometry.
- No new dependencies, routes, stores, network calls, or unrelated styling changes.
- Meaningful animation lifecycle/accessibility regression coverage plus existing About You behavior tests, affected type/lint/static checks, and browser evidence pass.

## References
- Concept preview: .omx/artifacts/adlc-concepts/adlc-directions.html (first panel).
- Current component: packages/views/onboarding/components/development-lifecycle.tsx.
- Authoritative copy and package boundaries: CLAUDE.md and apps/docs/content/docs/developers/conventions.zh.mdx.
