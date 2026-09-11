# Align built-in agent skills with role permissions and task scope

## Goal

Apply the five reviewed Multica-only skill recommendations: role defaults, ADR drafts, lead bindings, proportional release checks, credential prerequisites, and Mika onboarding distribution.

## Requirements

- Technical Writer edits documentation on an isolated branch. Architect stays read-only and delivers proposed ADR drafts in issue comments for an implementer or human to save.
- Only Feature Delivery and Discovery leads default to requirement clarification; the other six use their existing routing instructions.
- Full release checks apply to releases. Independent high-risk actions use proportionate checks with explained N/A items, retaining Operator and per-action human approval requirements.
- Credential disclosure instructions require explicit user intent, Operator authority, and approved secret_access.
- Only Mika tasks in onboarding conversations receive the built-in onboarding skill, consistently across inline claims, slim claims, bundle resolution, retries, and follow-up turns.
- Preserve customized workspace agents and skill copies. No schema changes, dependencies, or real-agent execution.

## Acceptance Criteria

- [x] Default role/skill registries match the approved responsibilities.
- [x] Ordinary agents and ordinary Mika tasks cannot receive or resolve the built-in onboarding skill; Mika onboarding tasks can.
- [x] Existing scoped skill reads and error handling remain intact.
- [x] Relevant regression tests and static checks pass.

## Notes

- User explicitly approved implementation of the reviewed recommendations.
