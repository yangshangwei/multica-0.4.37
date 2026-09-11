# Design

Keep role instructions, materialized role skills, and platform skills separate. Increment changed template and role-skill versions without overwriting workspace copies.

Select onboarding availability using persisted kickoff provenance in the task's chat session and Mika's system identity. Use the same task scope for both claim formats and bundle resolution. Keep onboarding available during follow-up and retry turns in that conversation; do not infer scope from user text or agent display names.

The general platform skill catalog excludes onboarding. Its catalog entry remains embedded for scoped delivery. Ordinary builtin resolution must not add workspace-skill reads or unnecessary chat queries.

Permission enforcement is unchanged: align guidance with policy without introducing a filesystem sandbox or redesigning credential APIs. All changes are reversible code/content updates; existing customizations are not migrated.
