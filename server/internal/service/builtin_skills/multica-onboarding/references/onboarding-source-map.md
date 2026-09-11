# Onboarding source map

| Contract | Source |
| --- | --- |
| Onboarding is not part of the general platform skill set. | `server/internal/service/builtin_skills.go`: `BuiltinSkills` |
| Only the task's own built-in Mika receives onboarding when its chat has a product-authored kickoff. Display names and message text are not identity. | `server/internal/service/builtin_skills.go`: `TaskBuiltinSkills`; `server/pkg/db/queries/chat.sql`: `ChatSessionHasOnboardingKickoff` |
| The kickoff remains session provenance for continuation and retry turns, including retries with a different input owner. | `server/pkg/db/queries/chat.sql`: `ChatSessionHasOnboardingKickoff` |
| Inline claims, slim claims, and onboarding bundle resolution use the same task scope; a failed scope read returns an error. General builtin downloads need no scope query. | `server/internal/handler/daemon.go`: `buildClaimedTaskResponse`, `ResolveTaskSkillBundles`; `server/internal/service/task.go`: `LoadAgentSkillBundles`, `LoadRequestedAgentSkillBundles` |
| The product authors the hidden kickoff and the opening. | `server/internal/handler/mika_onboarding.go`; `server/internal/service/task.go`: `OpenMikaOnboardingChat` |

Runtime regression coverage: `server/internal/handler/daemon_builtin_skills_scope_test.go`
exercises both claim formats, bundle resolution, identity, session provenance,
continuations, and retries. `server/internal/service/builtin_skill_scope_test.go`
covers read failures and general downloads without additional scope reads.
