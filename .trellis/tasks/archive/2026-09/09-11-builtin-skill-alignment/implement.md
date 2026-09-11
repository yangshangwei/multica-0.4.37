# Implementation and verification

1. Add regression coverage for role defaults and onboarding distribution/resolution before changing runtime behavior.
2. Update role defaults, ADR drafting instructions, and versions.
3. Narrow release checks and document credential prerequisites; update source maps.
4. Select platform skills from trusted task context in both claim formats and bundle resolution.
5. Run focused tests, relevant Go service/handler tests and static checks, plus repository lint/typecheck where available.
6. Review the diff, document the task-scope invariant in server specs, and report existing-workspace limitations.

Ownership: role_defaults owns roster, roster tests, Architect/Writer instructions, and ADR skill; product_skill_defaults owns release/autopilot guidance; primary agent owns runtime distribution, supporting tests, role-skill version registry, and task/spec records.
