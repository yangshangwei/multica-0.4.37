# Research: security-remediation v1 pin source map (blast radius for v2)

- **Query**: every place in the repo that pins the v1 text, roster, versions, or the global counts of built-in roles / role skills / squads for the `security-remediation` squad template and `security-remediation-lead` role template
- **Scope**: internal (read-only: `git log`, `git show --stat`, `git grep`, file reads)
- **Date**: 2026-11-18
- **Branch / HEAD at research time**: `codex/cross-workspace-collaboration` (see section 0)

Status legend: every claim carries a `path:line` anchor. "Inference" marks a conclusion drawn from code rather than stated by it. "Not confirmed" marks something searched for and not found.

## 0. Baseline facts (registry state at research time)

HEAD `ccd28cbc0` (2026-11-18 07:37 +0800) on branch `codex/cross-workspace-collaboration`. Every line number in this document is a working-tree number. The working tree is dirty; `git status --porcelain` over the cited paths shows four cited files as modified, and no other cited file:

- `packages/views/locales/en/modals.json` and `packages/views/locales/zh-Hans/modals.json` (cited in section 2.5);
- `server/internal/service/delivery_completion_authorization.go` (cited in section 6.2 at `:670` and `:745`; the same two lines are `:634` and `:709` at HEAD);
- `server/pkg/db/queries/agent.sql` (cited in section 6.1 at `:41-54`; the uncommitted change is a single 12-line addition after line 1036, so the cited range is the same at HEAD).

### 0.1 Global registry counts

| Registry | Count | Where the count is fixed |
|---|---|---|
| Listed role templates (`Listed: true`) | 14 | `server/internal/service/builtin_agent_templates_roster.go:16-257` (keys at lines 16, 34, 52, 70, 88, 106, 124, 142, 160, 178, 196, 214, 232, 250) |
| Unlisted squad-leader role templates | 9 | same file, keys at lines 276, 293, 309, 325, 342, 358, 374, 390, 407 |
| All role templates | 23 | 14 + 9 (derived; pinned indirectly by `builtin_agent_templates_test.go:227-229`) |
| Squad templates | 9 | `server/internal/service/builtin_squad_templates.go` keys at lines 104, 141, 183, 224, 249, 274, 311, 356, 401 |
| Role skills | 15 | `server/internal/service/builtin_role_skills.go:59-75` (`builtinRoleSkillVersions` map) |

### 0.2 The v1 squad entry

- `server/internal/service/builtin_squad_templates.go:311-315` — `Key: "security-remediation"`, `Version: 1`, `DefaultName: "Security Remediation Squad"`, `AvatarEmoji: "🛡️"`, `LeaderTemplateKey: "security-remediation-lead"`.
- Seven member seats, in this order (`builtin_squad_templates.go:317-344`): `product-analyst` (317), `security-reviewer` (321), `diagnostician` (325), `implementer` (329), `qa-engineer` (333), `migration-reviewer` (337), `code-reviewer` (341). Each seat carries an `en` and `zh` role note.
- Titles `en: "Security Remediation Squad"`, `zh: "安全漏洞修复小队"` at `builtin_squad_templates.go:346-349`.

### 0.3 The v1 leader entry

- `server/internal/service/builtin_agent_templates_roster.go:374-378` — `Key: "security-remediation-lead"`, `Version: 1`, `Autonomy: AutonomyCoordinator`; no `Listed` and no `RoleSkills` line appears for this entry in the key/version grep (the unlisted leaders at 293, 309, 342, 358, 374 have no `RoleSkills:` line; 276, 325, 390, 407 do).

### 0.4 Current versions of the roles this squad seats

| Role key | Role `Version` | Role skill | Skill version |
|---|---|---|---|
| `product-analyst` | 1 (`builtin_agent_templates_roster.go:17`) | `multica-requirement-clarification` (`:23`) | 3 (`builtin_role_skills.go:60`) |
| `security-reviewer` | 2 (`:125`) | `multica-security-review` (`:131`) | 4 (`builtin_role_skills.go:65`) |
| `diagnostician` | 1 (`:89`) | `multica-debugging` (`:95`) | 1 (`builtin_role_skills.go:64`) |
| `implementer` | 1 (`:53`) | `multica-test-report` (`:59`) | 1 (`builtin_role_skills.go:62`) |
| `qa-engineer` | 1 (`:71`) | `multica-test-report` (`:77`) | 1 (`builtin_role_skills.go:62`) |
| `migration-reviewer` | 3 (`:251`) | `multica-migration-review` (`:257`) | 2 (`builtin_role_skills.go:73`) |
| `code-reviewer` | 2 (`:107`) | `multica-code-review` (`:113`) | 2 (`builtin_role_skills.go:63`) |
| `security-remediation-lead` | 1 (`:375`) | none | — |

Seven seats map to six distinct role skills because `implementer` and `qa-engineer` share `multica-test-report`. That "six" is itself pinned by a test (section 1, `squad_template_security_test.go:105-107`).

## 1. Tests that pin v1

Legend for the last three columns: does the assertion break when (V) a `Version` is bumped, (S) one seat is added to this squad's roster, (K) one new role skill is added to the registry. "rel" = the assertion reads the registry value at run time, so it follows the change instead of breaking.

### 1.1 Go tests that name `security-remediation` / `security-remediation-lead` directly

`git grep -n 'security-remediation' -- server` (excluding generated docs and the two embedded prompt files) returns exactly six Go test files; the first is the one already known to the caller.

| # | File:line | Test | What is asserted | V | S | K |
|---|---|---|---|---|---|---|
| 1 | `server/internal/service/builtin_security_remediation_templates_test.go:15` | `TestAgentRoleTemplates_SecurityRemediationLead` | `leader.Listed == false`, `leader.Version == 1`, `Autonomy == coordinator`, `MaxConcurrentTasks == 6` | breaks (lead bump) | no | no |
| 2 | `...builtin_security_remediation_templates_test.go:19` | same | `len(leader.RoleSkills) == 0` ("want none: working members own security methods") | no | no | breaks only if the lead itself gets a role skill |
| 3 | `...builtin_security_remediation_templates_test.go:22` | same | `DefaultName == "Security Remediation Lead"`, `Title("zh") == "安全漏洞修复负责人"` | no | no | no |
| 4 | `...builtin_security_remediation_templates_test.go:32-37` | `TestSquadTemplates_SecurityRemediationRoster` | `squad.TemplateKeys()` equals the literal ordered list `security-remediation-lead, product-analyst, security-reviewer, diagnostician, implementer, qa-engineer, migration-reviewer, code-reviewer` | no | breaks (any add / reorder) | no |
| 5 | `...builtin_security_remediation_templates_test.go:39` | same | `squad.Version == 1`, `squad.MaxAutonomy() == coordinator` | breaks (squad bump) | breaks only if the new seat is Operator | no |
| 6 | `...builtin_security_remediation_templates_test.go:42` | same | `DefaultName`, zh title `安全漏洞修复小队`, `AvatarEmoji == "🛡️"` | no | no | no |
| 7 | `...builtin_security_remediation_templates_test.go:60-156` | `...InstructionContracts`, `...ParentStatusOwnership` | substring contracts over each prompt body (known to the caller; not re-summarized). Line 138 pins the sentence `只有任务实际分配给本小队，且注入的 Squad Operating Protocol 授予父任务状态管理权时`; line 141 pins `保持当前负责人和状态不变`, `只交付评论和活动记录`, `不得自行改派任务来取得状态管理权` | no | no | no — but any prose rewrite must keep the markers |
| 8 | `server/internal/handler/squad_template_security_test.go:38` | `TestListSquadTemplates_SecurityRemediationLanguages` (en/zh/ja/ko) | catalog `Title` per language, `template.Version == 1`, `AvatarEmoji == "🛡️"` | breaks (squad bump) | no | no |
| 9 | `...squad_template_security_test.go:41` | same | `Leader.TemplateKey == "security-remediation-lead"`, `Leader.AutonomyLevel == "coordinator"`, `len(template.Members) == 7` | no | breaks | no |
| 10 | `...squad_template_security_test.go:56` | `TestCreateSquadFromTemplate_SecurityRemediationPrivateRoster` (en, zh) | `len(CreatedAgents) == 8`, `len(ReusedAgents) == 0` | no | breaks (becomes 9) | no |
| 11 | `...squad_template_security_test.go:59` | same | `Squad.TemplateKey == template.Key`, `Squad.TemplateVersion == 1`, `Squad.Name == template.Title(language)` | breaks (squad bump) | no | no |
| 12 | `...squad_template_security_test.go:62` | same | `Squad.Instructions == template.Instructions()` (rel) and `AvatarURL == "icon:shield-check"` | rel | no | no |
| 13 | `...squad_template_security_test.go:83` | same | per created agent: `template_version == role.Version`, name, instructions, autonomy, concurrency all read from the registry | rel | rel | rel |
| 14 | `...squad_template_security_test.go:86-97` | same | each agent `permission_mode == "private"`, requested runtime, owner; `autonomy != "operator"`; role note equals `member.Roles[language]`; leader bound as `LeaderID`; zero invocation targets | no | breaks only if the new seat is Operator (line 89) | no |
| 15 | `...squad_template_security_test.go:99` | same | created keys equal `template.TemplateKeys()` (rel) | — | rel | — |
| 16 | `...squad_template_security_test.go:102` | same | `squad_member` row count `== 8` | no | breaks (9) | no |
| 17 | `...squad_template_security_test.go:105-107` | same | `skill` rows `name LIKE 'multica-%'` in the workspace `== 6` ("want six reused methods") | no | breaks if the new seat brings a role skill not already among the six | breaks only via S |
| 18 | `...squad_template_security_test.go:114,141` | `TestCreateSquadFromTemplate_SecurityRemediationReusesCustomizedMembers` | staffs `maintenance` first, then this squad: `len(CreatedAgents) == 3 && len(ReusedAgents) == 5` ("new leader, analyst and code reviewer") | no | breaks (4/5) unless the new role is also a `maintenance` seat | no |
| 19 | `...squad_template_security_test.go:116-133,160-165` | same | the reused `implementer` agent row and the `multica-security-review` skill row are byte-identical (`to_jsonb`) before and after staffing | no | no | no |
| 20 | `...squad_template_security_test.go:189` | `TestCreateSquadFromTemplate_SecurityRemediationAutonomy` | a Coordinator agent caller gets 201 and `len(CreatedAgents) == 8`; observer / contributor callers get 403 and no rows | no | breaks (9); the 201/403 split also flips if `MaxAutonomy()` rises | no |
| 21 | `...squad_template_security_test.go:203-211` | `TestCreateAgentFromTemplate_SecurityRemediationLeaderIsUnlisted` | `POST /api/agents/from-template` with `security-remediation-lead` returns 400 | no | no | no |
| 22 | `...squad_template_security_test.go:238-240` | `TestSquadTemplates_SecurityRemediationStatusBriefing` | `strings.Count(leaderInstructions + briefing, permission) == 2` — the sentence at line 238 must occur exactly once in the copied leader body and exactly once in the copied squad body | no | no | no — prose constraint: exactly one occurrence per body |
| 23 | `...squad_template_security_test.go:243,248,251` | same | owning brief contains `Own the parent issue status` and `multica issue status <issue-id> in_review`; guest brief contains `Do NOT change this issue's status` and `保持当前负责人和状态不变` and neither owning string | no | no | no — `保持当前负责人和状态不变` currently comes from the two prompt bodies (`builtin_agent_templates/security-remediation-lead/INSTRUCTIONS.md:66`, `builtin_squad_templates/security-remediation/INSTRUCTIONS.md:70`) |
| 24 | `server/internal/handler/squad_template_access_test.go:15` | `TestCreateSquadFromTemplate_ReuseAccessWithOneConnection` | runs the reuse-authorization matrix for `bug-fix` and `security-remediation` with a pre-existing `implementer`; on 201 `ReusedAgents == [that implementer]` (line 81); on 409 agent / squad / skill counts are unchanged (lines 89-97) | no | no (a new role would not pre-exist) | no |
| 25 | `server/internal/handler/squad_template_test.go:61,64-69` | `TestListSquadTemplates_ReturnsTheWholeRoster` | `len(out.Templates) == 9` and the ordered key list with `security-remediation` at index 6 | no | no | no — breaks only if a squad is added / reordered |
| 26 | `server/internal/service/builtin_agent_autonomy_test.go:194-207` | `TestSquadTemplates_RosterIsCoherent` | same nine ordered keys at service level | no | no | no |
| 27 | `...builtin_agent_autonomy_test.go:220-234` | same | every seat: template exists, is `Listed`, has non-empty role note for every `TemplateLanguages` entry | no | constrains the new seat (must be listed, en+zh notes) | no |
| 28 | `...builtin_agent_autonomy_test.go:241-246` | same | every squad policy contains `## 父任务状态` and ``绝不将父任务标记为 `done` `` | no | no | prose constraint on the squad body |
| 29 | `...builtin_agent_autonomy_test.go:254,267,273-277` | `TestSquadTemplates_DiagnosticianPlacementMatchesFailureWorkflows` | this squad seats exactly one `diagnostician`; squad policy contains `原因未知`, `诊断工程师`, `实现工程师` | no | no | prose constraint on the squad body |
| 30 | `...builtin_agent_autonomy_test.go:286,294-297` | `TestSquadTemplates_LifecycleEvidenceRouting` | this squad seats `migration-reviewer` | no | no (breaks on removal) | no |
| 31 | `...builtin_agent_autonomy_test.go:446,455-460` | `TestSquadLeads_ExplainDiagnosticianHandoff` | lead body contains `诊断工程师` and `原因未知` | no | no | prose constraint on the lead body |
| 32 | `...builtin_agent_autonomy_test.go:520,532` | `TestSquadTemplates_MaxAutonomyMatchesRoster` | `security-remediation` `MaxAutonomy() == AutonomyCoordinator` | no | breaks only if the new seat is Operator | no |
| 33 | `server/internal/service/builtin_agent_templates_test.go:223` | `TestAgentRoleTemplates_DefaultRoleSkills` | `"security-remediation-lead": nil` in the 23-key skill map | no | no | breaks if the lead gets a skill |

### 1.2 Registry-wide count / roster pins (break when a listed role or a role skill is added)

| # | File:line | Test | Literal pinned | Breaks on new listed role | Breaks on new role skill |
|---|---|---|---|---|---|
| A | `server/internal/service/builtin_agent_templates_test.go:22-30` | `TestAgentRoleTemplates_ListedRosterIsFourteen` | `len(AgentRoleTemplates()) == 14` | yes | no |
| B | `...builtin_agent_templates_test.go:31-41` | same | ordered 14-key list (`product-analyst ... migration-reviewer`), compared by index | yes if inserted before index 14; an append past the end is not compared by this loop but line 24 already fails | no |
| C | `...builtin_agent_templates_test.go:202-229` | `TestAgentRoleTemplates_DefaultRoleSkills` | 23-key map role -> skills; `len(AllAgentRoleTemplates()) == len(want)` (line 228) and every template must have a map entry (lines 231-239) | yes | yes for any role whose `RoleSkills` changes |
| D | `...builtin_agent_templates_test.go:282-300` | `TestRoleSkillTemplates_RosterIsFifteen` | sorted literal list of 15 skill names, `len(RoleSkillTemplates()) == len(want)` | no | yes (count and sorted position) |
| E | `...builtin_agent_templates_test.go:712-746` | `TestRoleSkillTemplates_PresentationDefaults` | 15-entry map name -> `{category, icon}`; count equality (731); unknown name fails (736); category / icon must pass `skill.IsCategory` / `skill.IsIconName` (743) | no | yes — the new SKILL.md frontmatter `metadata` category and icon must be added here and be on the whitelist |
| F | `...builtin_agent_templates_test.go:632-650` | `TestRoleSkillTemplates_EveryEmbeddedSkillIsRegistered` | every directory under `builtin_role_skills/` has a `builtinRoleSkillVersions` entry and the two counts match | no | a directory without a version-map entry (or the reverse) fails |
| G | `...builtin_agent_templates_test.go:581-627` | `TestRoleSkillTemplates_FilesMatchSource` | each template's content and supporting files equal the on-disk bundle | no | rel |
| H | `...builtin_agent_templates_test.go:175-199` | `TestAgentRoleTemplates_RoleSkillsExist` | at most ONE role skill per template (line 177-181: "the roster caps this at 1"); skill name has `multica-` prefix (188); non-empty frontmatter description (191); `Version >= 1` (194) | constrains the new role | constrains the new skill |
| I | `...builtin_agent_templates_test.go:89-124` | `TestAgentRoleTemplates_AutonomyDefaults` | 14-entry map role -> autonomy, iterated from the map (a missing new role is not detected); exactly one template defaults to Operator (lines 116-124) | only if the new role is Operator | no |
| J | `...builtin_agent_templates_test.go:44-62` | `TestAgentRoleTemplates_WorkloadBackedSpecialistsAreListed` | `experience-validation-engineer` = Contributor + `[multica-experience-validation]`; `migration-reviewer` = Observer + `[multica-migration-review]`; both `Listed` | no | no |
| K | `...builtin_agent_templates_test.go:67-83` | `TestAgentRoleTemplates_UnlistedAreSquadLeaders` | every unlisted template is some squad's leader and is Coordinator | constrains: a new unlisted template must lead a squad | no |
| L | `...builtin_agent_templates_test.go:132-170` | `TestAgentRoleTemplates_InstructionsCarryTheContract` | every role body contains `## 不负责的事项`, `## 完成标准`, `## 需要人工介入的情况`, plus `## 职责` or `## 如何分配工作`; listed roles also `## 输入` and `## 交付格式`; no `{{` | constrains the new role body AND the rewritten lead body | no |
| M | `...builtin_agent_templates_test.go:654-672` | `TestAgentRoleTemplates_LocalizedCopyIsComplete` | every template has en + zh title and description, `MaxConcurrentTasks == agentconfig.DefaultMaxConcurrentTasks`, non-empty `AvatarEmoji` | constrains the new role | no |
| N | `...builtin_agent_templates_test.go:450-454` | `TestDiagnostician_EvidenceContract` | no other listed role may share the diagnostician's `AvatarEmoji` | constrains the new role's avatar | no |
| O | `server/internal/service/builtin_template_languages_test.go:14-47` | `TestTemplateCatalogsRetiredLanguagesFallBackToEnglish` | `ja` / `ko` title and description equal English for every role and squad; seat `Roles` maps contain only `en` / `zh` keys (lines 36-41) | constrains new role + new seat note | no |
| P | `server/internal/service/builtin_agent_autonomy_test.go:469-481` | `TestSquadTemplates_StaffOnlyListedWorkingRoles` | every seat key is in `AgentRoleTemplates()` | the new seat must be a listed role | no |
| Q | `...builtin_agent_autonomy_test.go:488-505` | `TestSquadTemplates_LeadersAreOneToOne` | one leader per squad; every unlisted template leads a squad | no | no |
| R | `server/internal/handler/agent_template_test.go:57-58` | `TestListAgentRoleTemplates_ReturnsTheRosterWithInstructions` | `GET /api/agents/templates?language=zh` returns exactly 14 templates | yes | no |
| S | `server/internal/handler/skill_template_test.go:26-45` | `TestListSkillTemplates_ReturnsVerbatimCatalogWithoutDatabase` | `GET /api/skills/templates` returns exactly the literal ordered 15 names | no | yes |
| T | `server/internal/service/builtin_role_skills.go:137-138` | (comment, not a test) | "the slice is fifteen entries long" | no | comment goes stale |
| U | `server/internal/service/builtin_agent_templates_roster.go:9` | (comment, not a test) | "Deliberately fourteen roles, not twenty." | comment goes stale | no |

Count-relative tests that follow the registry and do NOT break: `server/cmd/server/skill_template_test.go:48-50` (`len(service.RoleSkillTemplates())`), `server/internal/service/skill_template_dir_test.go:57,74,130,226` (`len(RoleSkillTemplates())` and `+1`), `server/internal/handler/project_execution_squads_test.go:249-286` (iterates `service.SquadTemplates()` and materializes every squad concurrently under an 8-second context at line 254; no literal count — inference: an extra seat adds work inside that budget), `server/internal/service/builtin_agent_autonomy_test.go:540-550` (`TemplateKeys()` length = members + 1).

### 1.3 Other squads' pins that only matter if the new role is seated elsewhere too

- `server/internal/service/builtin_agent_autonomy_test.go:324` — review-gate `leader.Version == 3 && squad.Version == 3`; `:335` — `len(review-gate.Members) == 6` and a `checks` map with one routing keyword per seat (`:327-334`); every seat needs a table row (a line starting with `|`) containing the role's zh title and the keyword in BOTH the leader body and the squad body (`:338-361`).
- `server/internal/service/builtin_agent_autonomy_test.go:251-256` — diagnostician is seated in exactly `bug-fix`, `maintenance`, `security-remediation`, `incident` and in no other squad (`:270-272`).
- `server/internal/service/builtin_agent_autonomy_test.go:282-288` — required lifecycle seats per squad (`review-gate`, `release`, `maintenance`, `security-remediation`, `incident`).
- `server/internal/handler/squad_template_test.go:189-193` — `bug-fix` after `feature-delivery` creates exactly 2 agents; `:252-257` — `review-gate` after `feature-delivery` creates 5 / reuses 2.

### 1.4 Go test not found

- No Go test in `server/` asserts a count of 23 for all role templates as a literal; the only total-count check is the map-length comparison at `builtin_agent_templates_test.go:228`. Search: `git grep -n -E '(!=|==) *23([^0-9]|$)|want 23|twenty-three|23 (role|template)' -- 'server/*_test.go'` returns five lines, none about role templates: `server/internal/daemon/collaborationrun/preparation_recovery_linux_test.go:174` (an exit code), `server/internal/handler/quick_create_parent_test.go:205` (a policy revision), `server/internal/integrations/lark/http_client_test.go:957` (API error code 230071), `server/internal/migrations/collaboration_targets_test.go:86` (an applied-migration count) and `server/internal/realtime/sharded_stream_relay_test.go:237` (relay entries). `builtin_agent_templates_test.go` was also read in full, together with the sweep `git grep -n -e 'AllAgentRoleTemplates()' -e 'AgentRoleTemplates()' -e 'SquadTemplates()' -e 'RoleSkillTemplates()' -- server` (all callers listed above).

## 2. Precedent for adding a listed role + role skill

Sources: `git show --stat` on each commit below, and `git log --oneline -- server/internal/service/builtin_agent_templates_roster.go server/internal/service/builtin_role_skills.go`. The file lists are the commits' own; nothing was replayed or tested.

### 2.1 The precedent commits

| Commit (full hash) | Date (+0800) | Subject | What it added | Files |
|---|---|---|---|---|
| `e348b009a` (`e348b009aef1db3a22c7b8cda84df387c3fadf17`) | 2026-09-23 22:32 | feat(agents): 让失败诊断形成可交接的 RCA 证据 | listed role `diagnostician` + role skill `multica-debugging` | 22 (+451/-32) |
| `bfc218434` (`bfc2184347b83ff89ee09ab26fdc5dea3e947bc2`) | 2026-09-23 23:45 | Route unexplained failures through diagnostician squads | seats `diagnostician` in `bug-fix`, `incident`, `maintenance` | 5 |
| `b80b297ef` (`b80b297ef2abbe197cd8454abeda75e05036e52c`) | 2026-09-24 05:44 | Close lifecycle evidence gaps before expanding specialist roster | listed roles `reliability-engineer`, `agent-evaluator` + four new role skills (`multica-agent-evaluation`, `multica-incident-learning`, `multica-reliability-engineering`, `multica-rollout-and-canary-verification`); records the workload gate as NOT yet met for the next two specialists | 95, of which 33 outside `.trellis/tasks/` (+2086/-63) |
| `c21d03a43` (`c21d03a439af06e44eeb3c42617b499868bef88c`) | 2026-09-24 10:31 | feat(agents): add experience-validation and migration-reviewer specialist roles | listed roles `experience-validation-engineer`, `migration-reviewer` + role skills `multica-experience-validation`, `multica-migration-review`, and their squad seats | 28 (+451/-72) |
| `476eaee40` (`476eaee406ef5bd943070766986d0bd23d746672`) | 2026-10-27 20:12 | feat(squads): coordinate security repairs through a reusable template | the v1 `security-remediation` squad + the unlisted `security-remediation-lead` (no new listed role, no new role skill) | 50, of which 32 outside `.trellis/tasks/` (+2132/-197) |

### 2.2 File lists, grouped

**`e348b009a` — `diagnostician` + `multica-debugging` (22 files)**

- Registry: `server/internal/service/builtin_agent_templates_roster.go`, `server/internal/service/builtin_role_skills.go`
- Embedded bodies (new): `server/internal/service/builtin_agent_templates/diagnostician/INSTRUCTIONS.md`, `server/internal/service/builtin_role_skills/multica-debugging/SKILL.md`
- Go tests: `server/internal/service/builtin_agent_templates_test.go`, `server/internal/handler/agent_template_test.go`, `server/internal/handler/skill_template_test.go`
- Frontend: `packages/views/locales/{en,ja,ko,zh-Hans}/skills.json`, `packages/views/skills/lib/skill-presentation.ts`, `packages/views/skills/lib/skill-presentation.test.ts`
- Docs: `apps/docs/content/docs/agents-create.{mdx,zh.mdx,ja.mdx,ko.mdx}`
- Generated docs: `server/internal/docs/content/pages/agents-create.json`
- Built-in product skill: `server/internal/service/builtin_skills/multica-creating-agents/SKILL.md`, `server/internal/service/builtin_skills/multica-creating-agents/references/creating-agents-source-map.md`
- Spec: `.trellis/spec/server/builtin-templates.md`, `.trellis/spec/views/frontend/builtin-skill-localization.md`
- e2e: none. The commit message carries the trailer "Not-tested: 未运行浏览器 E2E 或真实模型 RCA 行为评测".

**`bfc218434` — seating the diagnostician (5 files)**

- `server/internal/service/builtin_squad_templates.go`
- `server/internal/service/builtin_squad_templates/{bug-fix,incident,maintenance}/INSTRUCTIONS.md`
- `server/internal/service/builtin_agent_autonomy_test.go`
- Commit message trailer: "Rejected: Add diagnostician to every squad | RCA is only appropriate for failure workflows".

**`b80b297ef` — `reliability-engineer`, `agent-evaluator` + four role skills (95 files; the 33 outside `.trellis/tasks/` are listed)**

- Registry: `server/internal/service/builtin_agent_templates_roster.go`, `server/internal/service/builtin_role_skills.go`, `server/internal/service/builtin_squad_templates.go`
- Embedded bodies: `server/internal/service/builtin_agent_templates/{agent-evaluator,reliability-engineer}/INSTRUCTIONS.md`; role skills under `server/internal/service/builtin_role_skills/` (from `git show --name-status b80b297ef`) — added: `multica-agent-evaluation`, `multica-incident-learning`, `multica-reliability-engineering`, `multica-rollout-and-canary-verification`; modified: `multica-architecture-decision-record`, `multica-progress-report`, `multica-release-check`, `multica-security-review`; `server/internal/service/builtin_squad_templates/{incident,release,review-gate}/INSTRUCTIONS.md`
- Go tests: `server/internal/handler/{agent_template_test.go,skill_template_test.go,squad_template_test.go}`; `server/internal/service/{builtin_agent_autonomy_test.go,builtin_agent_templates_test.go,lifecycle_handoff_fixture_test.go}`; `server/internal/service/testdata/lifecycle/lifecycle_handoff_fixtures.json`
- Frontend: `packages/views/locales/{en,ja,ko,zh-Hans}/skills.json`, `packages/views/skills/lib/skill-presentation.{ts,test.ts}`
- e2e: `e2e/agent-role-template.spec.ts`
- Built-in product skill: `server/internal/service/builtin_skills/multica-creating-agents/SKILL.md` and its `references/creating-agents-source-map.md`
- Spec: `.trellis/spec/server/builtin-templates.md`
- Commit message trailers: "Rejected: Add experience-validation-engineer and migration-reviewer now | recent cycles lack the required accessibility/QA-gap and schema/API migration deliverables." and "Directive: Re-run the workload gate after two qualifying release cycles before adding either specialist to the default roster."

**`c21d03a43` — `experience-validation-engineer`, `migration-reviewer` + their role skills (28 files)**

- Registry: `server/internal/service/builtin_agent_templates_roster.go`, `server/internal/service/builtin_role_skills.go`, `server/internal/service/builtin_squad_templates.go`
- Embedded bodies (new): `server/internal/service/builtin_agent_templates/{experience-validation-engineer,migration-reviewer}/INSTRUCTIONS.md`, `server/internal/service/builtin_role_skills/{multica-experience-validation,multica-migration-review}/SKILL.md`
- Squad bodies edited (seating): `server/internal/service/builtin_squad_templates/{maintenance,release,review-gate}/INSTRUCTIONS.md`
- Go tests: `server/cmd/server/skill_template_test.go`; `server/internal/handler/{agent_template_test.go,skill_template_test.go,squad_template_test.go}`; `server/internal/service/{builtin_agent_autonomy_test.go,builtin_agent_templates_test.go}`
- Frontend: `packages/views/locales/{en,ja,ko,zh-Hans}/skills.json`
- Docs: `apps/docs/content/docs/agents-create.{mdx,zh.mdx,ja.mdx,ko.mdx}`, `apps/docs/content/docs/skills.mdx`
- Built-in product skill: `server/internal/service/builtin_skills/multica-creating-agents/SKILL.md` and its `references/creating-agents-source-map.md`
- Spec: `.trellis/spec/server/builtin-templates.md`
- Not in this commit: any `server/internal/docs/content/pages/*.json`, `packages/views/skills/lib/skill-presentation.ts` or its test, any `e2e/` file.
- Follow-up commit that completed the frontend side: `1b4f98fcd` (`1b4f98fcdd6d3d3ddd15321c44781f17896702ee`, 2026-09-24 23:45 +0800, "Keep official skills discoverable and browser regressions reliable") added `"multica-experience-validation"` and `"multica-migration-review"` to the name list in `packages/views/skills/lib/skill-presentation.ts` (found with `git log -S'multica-migration-review' -- packages/views/skills/lib/skill-presentation.ts`). Its 13 files: `.trellis/spec/views/frontend/builtin-skill-localization.md`; `e2e/auth-device.spec.ts` (new), `e2e/desktop-settings.spec.ts`, `e2e/localized-template-defaults.spec.ts`, `e2e/settings-preferences.spec.ts`, `e2e/skill-template-creation.spec.ts`; `packages/ui/components/ui/data-table.tsx`; `packages/views/issues/components/data-table-resize.test.tsx`; `packages/views/locales/zh-Hans/skills.json`; `packages/views/skills/components/skills-page.test.tsx`, `packages/views/skills/components/template-skill-create-panel.test.tsx`; `packages/views/skills/lib/skill-presentation.ts`, `packages/views/skills/lib/skill-presentation.test.ts`.
- Commit message: "Add two workload-gated specialist roles after the two-cycle evidence gate was met ... bump the roster to 14 listed roles / 15 role skills."

**`476eaee40` — the v1 squad itself (50 files; the 32 outside `.trellis/tasks/` are listed)**

- Registry: `server/internal/service/builtin_agent_templates_roster.go`, `server/internal/service/builtin_squad_templates.go`
- Embedded bodies (new): `server/internal/service/builtin_agent_templates/security-remediation-lead/INSTRUCTIONS.md`, `server/internal/service/builtin_squad_templates/security-remediation/INSTRUCTIONS.md`
- Go tests: `server/internal/service/builtin_security_remediation_templates_test.go` (new), `server/internal/service/builtin_agent_autonomy_test.go`, `server/internal/service/builtin_agent_templates_test.go`, `server/internal/handler/squad_template_security_test.go` (new), `server/internal/handler/squad_template_access_test.go`, `server/internal/handler/squad_template_test.go`
- Handler source: `server/internal/handler/agent_template.go`
- Frontend: `packages/views/common/squad-avatar.{tsx,test.tsx}`, `packages/views/locales/{en,zh-Hans}/squads.json`, `packages/views/squads/components/builtin-squad-catalog.{tsx,test.tsx}`
- e2e: `e2e/security-remediation-squad.spec.ts` (new), `e2e/squads-audit.spec.ts`
- Docs: `apps/docs/content/docs/{agents-create,security-model,squads}.{mdx,zh.mdx}`
- Generated docs: `server/internal/docs/content/pages/{agents-create,security-model,squads}.json`
- Built-in product skill: `server/internal/service/builtin_skills/multica-squads/SKILL.md`, `server/internal/service/builtin_skills/multica-squads/references/squad-source-map.md`
- Spec: `.trellis/spec/server/builtin-templates.md`, `.trellis/spec/server/index.md`
- Commit message: "Tested: ... 32 selected production-Web E2E cases covered; docs build and bundle parity".

### 2.3 Other commits that touched the roster or the skill-version map after `c21d03a43`

`cd95c8e40` (2026-11-05), `028e93bb0` (2026-11-05), `5701a7695` (2026-10-27), `372576209` (2026-09-27, the i18n reduction that retired `ja` / `ko`), `7fdbf274e`, `119478843`, `ed0fa7789` (all 2026-09-24). Earlier: `e2d3f8891` (2026-09-15, progress-reporter). Their file lists were not expanded.

### 2.4 Checklist: what a new listed role + role skill touches (union of the precedents, adjusted to the present tree)

Present-tree adjustments: `packages/views/locales/` holds only `en` and `zh-Hans` (plus `index.ts`, `mcp.test.ts`, `parity.test.ts`); `apps/docs/content/docs/` holds only `.mdx` and `.zh.mdx` for `agents-create`, `agents`, `security-model`, `skills`, `squads`. The `ja` / `ko` files the September commits edited no longer exist.

| Group | Path | What the precedents changed there |
|---|---|---|
| Registry | `server/internal/service/builtin_agent_templates_roster.go` | the new `Listed` role entry (key, `Version`, autonomy, `RoleSkills`, avatar, en/zh title and description); the head comment at `:9` states the count |
| Registry | `server/internal/service/builtin_role_skills.go:59-75` | the skill's `builtinRoleSkillVersions` entry; the comment at `:137-138` states the count |
| Registry | `server/internal/service/builtin_squad_templates.go` | the seat (`SquadRoleSlot` with en + zh role note) in each squad that staffs the role |
| Embedded body | `server/internal/service/builtin_agent_templates/<key>/INSTRUCTIONS.md` | new file; heading contract in section 1.2 row L |
| Embedded body | `server/internal/service/builtin_role_skills/<name>/SKILL.md` | new file; frontmatter `description` and `metadata` category / icon (section 1.2 rows E, H) |
| Embedded body | `server/internal/service/builtin_squad_templates/<squad>/INSTRUCTIONS.md` and the squad leader's `INSTRUCTIONS.md` | routing text for the new seat |
| Go tests | section 1.2 rows A, B, C, D, E, R, S; section 1.1 rows 4, 9, 10, 16, 17, 18, 20 when the role is seated in this squad | literal counts and ordered lists |
| Frontend | `packages/views/locales/{en,zh-Hans}/skills.json` key `builtin_role_skills.<name>` | `name`, `description`, `summary` per locale |
| Frontend | `packages/views/skills/lib/skill-presentation.ts:6-22` | literal `BUILTIN_ROLE_SKILL_NAMES` list (15 names) |
| Frontend | `packages/views/skills/lib/skill-presentation.test.ts` | edited in `e348b009a`, `b80b297ef` and `1b4f98fcd` |
| Docs | `apps/docs/content/docs/agents-create.{mdx,zh.mdx}` | the role-count sentence (`:28`) and the role table (`:30-45`); see section 3.3 |
| Docs | `apps/docs/content/docs/skills.mdx` | `c21d03a43` changed one sentence from "one of the seven built-in role templates" to "one of the fifteen built-in role-skill templates"; the present file carries no such count (removed by `eb6b3d5af`, see section 3.3) |
| Generated docs | `server/internal/docs/content/pages/agents-create.json` (and any other page whose source changed) | regenerated bundle (section 4) |
| Built-in product skill | `server/internal/service/builtin_skills/multica-creating-agents/SKILL.md` and `references/creating-agents-source-map.md` | roster count and per-role statements |
| Built-in product skill | `server/internal/service/builtin_skills/multica-squads/SKILL.md` and `references/squad-source-map.md` | squad roster statements (touched by `476eaee40`) |
| Spec | `.trellis/spec/server/builtin-templates.md`, `.trellis/spec/views/frontend/builtin-skill-localization.md` | counts, role list, skill list |
| e2e | `e2e/agent-role-template.spec.ts` | edited in `b80b297ef` only |

### 2.5 Present-day pins that postdate or sit outside the precedent commits

These exist in the tree now and reference the role / role-skill registries; the precedent commits did not have to satisfy all of them.

- `packages/views/skills/lib/skill-presentation.test.ts:23-27` reads the directory `server/internal/service/builtin_role_skills/` from disk; `:64-68` requires `Object.keys(catalog.builtin_role_skills).sort()` to equal those directory names for BOTH `en` and `zh-Hans`; `:70-72` requires `getBuiltinRoleSkillPresentation(name)` to recognise every embedded name (which goes through `BUILTIN_ROLE_SKILL_NAMES`); `:243-251` requires `zh.builtin_role_skills[name].description` to equal the `description` in that skill's `SKILL.md` frontmatter exactly; `:344-358` requires a non-empty `summary` shorter than the description in both locales.
- `packages/views/modals/quick-create-scenario.ts:4-19` — literal `agentScenarios` list of the 14 listed role keys; `packages/views/modals/quick-create-scenario.test.ts:23-31` pins it; one scenario string per role under `create_issue.scenarios` at `packages/views/locales/en/modals.json:278,286,287` and `packages/views/locales/zh-Hans/modals.json:275,283,284` (both JSON files are modified in the working tree; these are working-tree line numbers).
- `packages/views/skills/components/template-skill-create-panel.test.tsx:129-130` and `packages/views/skills/lib/skill-template-discovery.test.ts:31,100` reference built-in role-skill names.
- e2e: `e2e/agent-role-template.spec.ts:54-62,213-231,300-322`; `e2e/localized-template-defaults.spec.ts:143-199,205-206,354,385`; `e2e/skill-template-creation.spec.ts:224`; `e2e/progress-reporting.spec.ts:246` (reads an embedded `INSTRUCTIONS.md`). `e2e/agents-discovery.spec.ts:6-22` uses its own mock roles and is not bound to the registry.
- Built-in product skills: `server/internal/service/builtin_skills/multica-creating-agents/SKILL.md:133` ("fourteen listed role templates"); `server/internal/service/builtin_skills/multica-creating-agents/references/creating-agents-source-map.md:50` ("Roster of fourteen..."), `:56-58` (per-role version statements), `:65`; `server/internal/service/builtin_skills/multica-squads/SKILL.md:176,181,208,214` ("adds one unlisted Coordinator leader and reuses seven"); `server/internal/service/builtin_skills/multica-squads/references/squad-source-map.md:139,143` ("`security-remediation` v1 has a dedicated unlisted..."), `:157`.
- `server/internal/service/builtin_skills_test.go:401-413,454-483` pins strings of the built-in product skills; none of the pinned strings is "fourteen" or names this squad.
- `server/internal/service/resource_catalog.go:80-82,304-306` — the admin resource catalog treats role-skill names as built-in and reserved.
- `server/internal/handler/agent_avatar.go:27-44` — `templateIconAvatar(emoji string) string` turns a template's `AvatarEmoji` into the stored `icon:<name>` avatar through a literal 28-key emoji map (`:30-39`) and returns the `bot` icon for an emoji that is not a key (`:40-43`). Callers: `server/internal/handler/agent_template.go:198`, `server/internal/handler/squad_template.go:424,534`. The 23 role templates' `AvatarEmoji` values are at `server/internal/service/builtin_agent_templates_roster.go:20-410` (the lead's at `:377`) and this squad's at `server/internal/service/builtin_squad_templates.go:314`; a read-only comparison of the two registry files with the map found all 23 role emoji and all 9 squad emoji among its keys. `server/internal/handler/squad_template_test.go:166` compares the stored avatar with the output of the same function; no test requires a template emoji to be a key of the map (A18 under "Unknown / not found"). A separate TypeScript emoji map starts at `packages/ui/lib/avatar-icon.ts:134` (`LEGACY_EMOJI_ICONS`).

## 3. Frontend / docs / e2e that hard-code this squad

Sweep used for this section: `git grep -l -i -E 'security-remediation|security_remediation|Security Remediation|安全漏洞修复' -- . ':!.trellis/tasks'` lists 32 tracked files. Outside `server/` Go and embedded prompt files they are: 6 docs sources, 3 generated pages, 2 e2e specs, 6 files under `packages/views`, 2 built-in product skill files, 1 spec file, 1 audit document and 1 QA artifact. None is under `apps/web`, `apps/desktop`, `apps/mobile`, `packages/core` or `packages/ui`.

Change types referred to below: (V) squad or leader version bump, (S) one seat added to this squad, (R) a new listed role and/or role skill, (P) prompt prose rewritten.

### 3.1 Frontend (`packages/`)

| File:line | What is there | Follows the server, or literal |
|---|---|---|
| `packages/views/squads/components/builtin-squad-catalog.tsx:22-32` | client-side summary map keyed by the nine squad keys; `:29` maps `security-remediation` to `catalog.summaries.security_remediation` | literal key list; the summary is client copy, not the server description |
| `...builtin-squad-catalog.tsx:71` | `summaries.get(template.key) ?? template.description` | the server description is shown only for a key missing from the map |
| `...builtin-squad-catalog.tsx:61,74,75` | roster is `[template.leader, ...(template.members ?? [])]`; member count is `roster.length` through `profile_card.member_count`; roster titles are joined with " · " | server-driven; follows a roster change |
| `...builtin-squad-catalog.tsx:78,101` | "Apply to project" opens `UseSquadForProjectDialog` with the server template object | server-driven |
| `packages/views/locales/en/squads.json:186` | `"security_remediation": "Triage security risks, repair them outside production, and verify with independent review."` | literal |
| `packages/views/locales/zh-Hans/squads.json:181` | `"security_remediation": "分诊安全风险，在非生产环境修复，并由独立成员验证和审查。"` | literal |
| `packages/views/locales/en/squads.json:137-138`, `zh-Hans/squads.json:136` | `member_count_one` / `member_count_other` ("{{count}} members"), zh `member_count_other` ("{{count}} 名成员") | count is interpolated |
| `packages/views/squads/components/builtin-squad-catalog.test.tsx:134-167` | pins en title "Security Remediation Squad" and summary (`:137-138`), zh title "安全漏洞修复小队" and summary (`:143-144`), against a mocked template (`:148-156`); requires the entry NOT to show the server description (`:162`) and the dialog to show the server description and instructions (`:165-166`) | literal copies of the two locale strings; the roster is a mock (`:88` "3 members" is the generic fixture) |
| `packages/views/common/squad-avatar.tsx:14` | `"security-remediation": "shield-check"` | literal key → icon |
| `packages/views/common/squad-avatar.test.tsx:49` | `["security-remediation", "shield-check"]` | literal |
| `packages/ui/lib/avatar-icon.ts:139` | emoji 🛡 → `shield-check` | literal; the Go side pins `icon:shield-check` (section 1.1 row 12) |
| `packages/views/modals/quick-create-scenario.ts:21-30` | `squadScenarios` lists eight squad keys; `security-remediation` is not among them | literal; a squad with this key matches nothing at `:42-43` and gets the description-based `custom_squad` / `generic_squad` string (`:57-60`); a `security-remediation-lead` agent matches nothing at `:46-47` |
| `packages/views/modals/quick-create-scenario.test.ts:33-36` | the same eight squad keys | literal |
| `packages/views/modals/quick-create-scenario.ts:4-19`, `quick-create-scenario.test.ts:23-28` | the 14 listed role keys (section 2.5) | literal |
| `packages/core/types/squad.ts:32-33`, `packages/core/api/schemas.ts:2385` | `template_key?`, `template_version?` typed and schema-parsed | no display (section 6.2) |

- No squad-side version string exists in the UI: `git grep -n -i -E '"[^"]*version[^"]*":|版本' -- packages/views/locales/en/squads.json packages/views/locales/zh-Hans/squads.json` matches only `zh-Hans/squads.json:182`, the release-squad summary. "版本 1" / "version 1" for this squad appears only in docs (3.3).
- No product deep link targets the squad docs sections: `git grep -n -E 'squads|security' -- packages/core/docs/slugs.ts` is empty (section 4.3).

What each change type touches here:

- (V) nothing in `packages/`.
- (S) nothing literal; the catalog's count and roster line follow the API.
- (R) not specific to this squad: the role-skill and role-key literals in sections 2.4 and 2.5.
- (P) the two locale summaries and their copies in `builtin-squad-catalog.test.tsx:138,144` and `e2e/security-remediation-squad.spec.ts:22,29` are independent strings; nothing derives them from the prompt bodies or from the registry description.

### 3.2 e2e

`e2e/security-remediation-squad.spec.ts` (116 lines; one test per locale, `en` and `zh-Hans`):

| Line | Pinned value | Follows the server, or literal |
|---|---|---|
| `:20`, `:27` | titles "Security Remediation Squad" / "安全漏洞修复小队" | literal |
| `:21`, `:28` | catalog region name ("Squad templates" / "AI小队模板") and member-count text `"8 members"` / `"8 名成员"`, asserted at `:70` | literal — the count is the leader plus seven seats |
| `:22`, `:29` | the two catalog summaries, asserted at `:69` | literal copies of the locale strings |
| `:23-24`, `:30` | button labels and dialog names ("Use Security Remediation Squad for a project" / "将安全漏洞修复小队用于项目") | literal |
| `:59-61` | template looked up by key `security-remediation` through `/api/squads/templates?language=` | key literal |
| `:67` | `catalog.getByRole("listitem")` `toHaveCount(9)` | literal squad count |
| `:71` | roster line built from `[template.leader, ...template.members]` titles | server-driven |
| `:72` | `svg.lucide-shield-check` | literal icon |
| `:76-77` | dialog shows `template.description` | server-driven |
| `:86-88` | PUT body `{ squads: [{ template_key: "security-remediation", runtime_id, language }] }` | key literal |
| `:95` | saved selection `{ state: "configured", template_key: "security-remediation" }` | literal |
| `:98` | staffed squad `{ name: labels.title, template_key: "security-remediation", template_version: 1 }` | literal version |
| `:108-110` | after a rename the squad keeps `template_key: "security-remediation"` | literal |

Other specs:

- `e2e/squads-audit.spec.ts:48` — `["security-remediation", "Security remediation squad"]` in the list of nine keys staffed through `POST /api/squads/from-template` (`:50`); `:170` expects nine squad rows (`toHaveCount(9)`).
- `e2e/builtin-agent-concurrency.spec.ts:31-41` iterates the role catalog and `:44-52` staffs every squad in the squad catalog; every template and every stored agent must have `max_concurrent_tasks` equal to `SHARED_DEFAULT = 6` (`:8`, `:33`, `:39`, `:59-62`). Catalog-driven; it constrains the concurrency default of any new role.
- `e2e/agent-autonomy-gates.spec.ts:230-234` pins `product-analyst` = Observer, `implementer` = Contributor, `release-engineer` = Operator and `feature-delivery-lead` not listed; `:237-260` posts `template_version: 999` for `product-analyst` and expects the stored `template_version` `toBe(1)` (`:259`).
- Mock fixtures with a literal `template_version: 1` that do not read the registry: `e2e/agent-role-template.spec.ts:152,181`, `e2e/localized-template-defaults.spec.ts:167`, `e2e/agents-discovery.spec.ts:59`.
- Specs that staff other squads by key (not this one): `e2e/agent-autonomy-gates.spec.ts:487,669,718,734,778,803`, `e2e/project-squad-workspace.spec.ts:43`, `e2e/projects-p1-health.spec.ts:100`, `e2e/squads-design.spec.ts:22,29`, `e2e/localized-template-defaults.spec.ts:102,444`.
- Role / role-skill registry references in e2e are listed in section 2.5.

What each change type touches here:

- (V, squad) `e2e/security-remediation-squad.spec.ts:98`.
- (V, a seated role) `e2e/agent-autonomy-gates.spec.ts:259` if the role is `product-analyst`.
- (S) `e2e/security-remediation-squad.spec.ts:21,28` (asserted at `:70`). Inference from `builtin-squad-catalog.tsx:74`: with eight seats plus the leader the page renders "9 members" / "9 名成员".
- (R) the specs in section 2.5; nothing in the two squad specs counts roles or role skills.
- (P) `:22,29` only if the locale summaries change; the dialog text at `:77` is the server description.

### 3.3 Docs sources (`apps/docs/content/docs/`)

Only `*.zh.mdx` feeds the embedded bundle (section 4.1); the English `.mdx` files serve the public docs site.

| File:line | Text | Change types that make it stale |
|---|---|---|
| `squads.mdx:14` / `squads.zh.mdx:14` | "Nine templates are built in:" / "内置九个模板：" | none of V/S/R/P (squad count) |
| `squads.mdx:24` | "\| **Security Remediation Squad** \| Lead, Product Analyst, Security Reviewer, Diagnostician, Implementer, QA Engineer, Migration Reviewer, Code Reviewer \| Triage dependency, code and other security risks; repair outside production, verify independently, or escalate \|" | S (roster named seat by seat), P (third column) |
| `squads.zh.mdx:24` | "\| **安全漏洞修复小队** \| 队长、产品分析师、安全审查员、诊断工程师、实现工程师、测试工程师、迁移审查员、代码审查员 \| 分诊依赖、代码和其他安全风险，在非生产环境修复、独立验证，或交人工处理 \|" | S, P |
| `squads.zh.mdx:32` | "九个模板共用同一批角色，重叠的座位复用，不会重复创建。" | none (squad count) |
| `squads.mdx:36` | "a `coordinator` agent can staff the other seven but not these two" | only if this squad's highest autonomy becomes Operator |
| `squads.mdx:40` / `squads.zh.mdx:38` | "A Multica update never overwrites it." / "Multica 的版本升级不会覆盖它。" | none (states the copy rule, section 6.1) |
| `squads.mdx:42` / `squads.zh.mdx:40` | headings "## Security remediation" / "## 安全漏洞修复" — the anchors `#security-remediation` and `#安全漏洞修复` | linked from `agents-create.mdx:51`, `security-model.mdx:36`, `agents-create.zh.mdx:51`, `security-model.zh.mdx:36`; the zh anchor is also the generated toc id (`server/internal/docs/content/pages/squads.json:14-15`) |
| `squads.mdx:44` | "The **Security Remediation Squad** (`security-remediation`, version 1) ... Its dedicated leader coordinates seven existing specialist roles. The roster's highest autonomy is Coordinator; it includes no Operator." | V ("version 1"), S ("seven"), R ("existing") |
| `squads.zh.mdx:42` | "**安全漏洞修复小队**（`security-remediation`，版本 1）负责安全分诊，以及开发或测试环境中的修复。专用队长协调七个现有工作角色，最高自治等级为 Coordinator，不含 Operator。复用的角色保留已保存的指令、skill、运行时和 Access。" | V ("版本 1"), S ("七个"), R ("现有") |
| `squads.mdx:46-58` / `squads.zh.mdx:44-56` | triage paragraph (`confirmed` / `suspected` / `unknown`), the three-row risk-type table (`squads.mdx:48-52`, `squads.zh.mdx:46-50`), stop and approval boundaries, deliverables (parent to `in_review`), stop conditions | P; S where a routing step names a role |
| `squads.mdx:60` / `squads.zh.mdx:58` | relation to Maintenance / Bug Fix / Review Gate, and "autonomous model judgment and instruction compliance for this workflow remain unverified" / "真实模型的自主判断和指令遵守仍未验证" | P |
| `agents-create.mdx:28` / `agents-create.zh.mdx:28` | "**Use a template** starts from one of fourteen built-in roles:" / "**使用模板**会从内置的 14 个角色中选择一个开始：" | R |
| `agents-create.mdx:30-45` / `agents-create.zh.mdx:30-45` | the role table: header at `:30-31`, one row per listed role at `:32-45` (14 rows) | R |
| `agents-create.mdx:51` | "The [Security Remediation Squad](/squads#security-remediation) reuses seven of these specialists with a dedicated squad-only Coordinator leader. It adds no selectable specialist role or role skill. ..." | S ("seven"), R ("adds no selectable specialist role or role skill") |
| `agents-create.zh.mdx:51` | "[安全漏洞修复小队](/squads#安全漏洞修复)复用其中七个工作角色，另配一名仅用于小队的 Coordinator 队长；不增加可单独选择的工作角色或角色 skill。..." | S, R |
| `security-model.mdx:34-40` | heading "## Security remediation workloads" (`:34`); link (`:36`); "The template uses a Coordinator leader and existing Observer/Contributor roles, with no Operator." (`:38`); stop conditions (`:40`) | R ("existing"), S if the seat's autonomy is not Observer or Contributor, P (`:40`) |
| `security-model.zh.mdx:34-40` | "## 安全修复工作负载" (`:34`); link (`:36`); "模板使用 Coordinator 队长和现有 Observer/Contributor 角色，不含 Operator。" (`:38`); stop conditions (`:40`) | same |

- `skills.mdx` and `skills.zh.mdx` carry no role-skill count or list at research time: `grep -n -i -E 'fourteen|fifteen|十四|十五|14 个|15 个|multica-...'` over the two files matches only the example name `multica-code-review-copy` (`skills.mdx:52`, `skills.zh.mdx:54`). The sentence "one of the fifteen built-in role-skill templates" that `c21d03a43` wrote into `skills.mdx` was removed by `eb6b3d5af` (2026-09-29, "refactor(skills): distinguish workspace content from reusable templates"; found with `git log -S'fifteen built-in role-skill templates' -- apps/docs/content/docs/skills.mdx`).
- `agents.mdx` and `agents.zh.mdx`: the same pattern, extended with `nine |九个|role skill|角色 skill|squad template|小队模板`, matches nothing.

### 3.4 Generated pages

The three generated pages that carry the zh text are listed in section 4.5. Confirmed substrings: `server/internal/docs/content/pages/squads.json:5` contains "版本 1"; `:14-15` hold the toc title and id "安全漏洞修复"; `agents-create.json:5` contains "14 个角色" and "七个工作角色"; `security-model.json:5` contains "不含 Operator". They change exactly when the matching `*.zh.mdx` changes.

### 3.5 Built-in product skills and the spec

These files are embedded in the binary (product skills) or are the maintained contract (spec). `CLAUDE.md` ("Coding Rules") requires the product skills' `SKILL.md` and `references/*-source-map.md` to be updated in the same PR as the behaviour they document.

| File:line | Text | Change types that make it stale |
|---|---|---|
| `server/internal/service/builtin_skills/multica-squads/SKILL.md:175-177` | "Nine templates ship with the binary: ... `security-remediation`, `release`, `incident`." | none |
| `...multica-squads/SKILL.md:180-182` | "`diagnostician` is intentionally seated only in `bug-fix`, `maintenance`, `security-remediation`, and `incident`" | none unless the diagnostician seat moves |
| `...multica-squads/SKILL.md:208-210` | this squad's diagnosis rule | P |
| `...multica-squads/SKILL.md:214-225` | "`security-remediation` adds one unlisted Coordinator leader and reuses seven working roles. ..." followed by the routing summary and the status-ownership rule | S ("seven"), P |
| `...multica-squads/SKILL.md:227-228` | provenance statement (section 6.1) | none |
| `...multica-squads/SKILL.md:232-236` | "splits the nine in two ... The other seven have a `coordinator` ceiling" | only if this squad's highest autonomy becomes Operator |
| `server/internal/service/builtin_skills/multica-squads/references/squad-source-map.md:136-142` | diagnostician by workflow; "the other five templates do not provision it" | none unless the seat moves |
| `...squad-source-map.md:143-156` | "`security-remediation` v1 has a dedicated unlisted `security-remediation-lead` v1 (Coordinator, concurrency two) and seven existing working roles, with no Operator or new role skill. Its two instruction files require exact current-roster Markdown mentions, one dispatch with `action`, actual member task/comment provenance before advancing, and final `no_action`. ..."; names the two test files at `:154-155` | V (both "v1"), S ("seven"), R ("no ... new role skill"), P |
| `server/internal/service/builtin_skills/multica-creating-agents/SKILL.md:133` | "the platform's fourteen listed role templates" | R |
| `...multica-creating-agents/SKILL.md:167` | "Code Reviewer v2, Security Reviewer v2 and Migration Reviewer v3" | V of those three seated roles |
| `...multica-creating-agents/SKILL.md:197`, `:201-202` | "The listed `migration-reviewer` role (template version 2)"; "The ADR role skill is version 6." | V (see section 6.5 for the value at `:197`) |
| `...multica-creating-agents/references/creating-agents-source-map.md:50` | "Roster of fourteen listed role templates" | R |
| `...creating-agents-source-map.md:56` | "Template v2 with code skill v2 / security skill v4" | V of `code-reviewer`, `security-reviewer` or their skills |
| `...creating-agents-source-map.md:57` | "template v3 / skill v2" (Migration Reviewer) | V of `migration-reviewer` or its skill |
| `.trellis/spec/server/builtin-templates.md:89-90` | "Diagnostician brings the listed roster to fourteen roles and the role-skill catalog to fifteen entries." | R |
| `.trellis/spec/server/builtin-templates.md:155-162` | heading "### Security remediation v1"; "`security-remediation` is the ninth built-in squad. Its dedicated unlisted `security-remediation-lead` is version 1, Coordinator, with no new role skill. The seven seats reuse Product Analyst, Security Reviewer, Diagnostician, Implementer, QA Engineer, Migration Reviewer and Code Reviewer. Keep it before the Operator-bearing release/incident entries, preserve existing template versions, and retain private creation and workspace-copy reuse." | V, S, R |
| `.trellis/spec/server/builtin-templates.md:164-176` | routing scope and the status-ownership rule for both instruction bodies | P |
| `.trellis/spec/server/builtin-templates.md:178-187` | regression anchors (the two test files, catalog and project-adoption behaviour, `state: configured`) | none unless tests are renamed |
| `.trellis/spec/server/builtin-templates.md:364`, `:410` | "The fifteen `builtin_role_skills/*/SKILL.md` bodies"; "`GET /api/skills/templates` returns the fifteen role-skill templates" | R |

- Observed difference inside these files: `squad-source-map.md:144` says "concurrency two" for the leader. The registry value is `agentconfig.DefaultMaxConcurrentTasks` (`server/internal/service/builtin_agent_templates_roster.go:379`), which is 6 (`server/internal/agentconfig/concurrency.go:6`), and the Go test pins 6 (`server/internal/service/builtin_security_remediation_templates_test.go:15`).
- Go tests that pin strings of the product skill files are noted in section 2.5 (`server/internal/service/builtin_skills_test.go`); `grep -n -i -E 'fourteen|nine|seven|security-remediation' server/internal/service/builtin_skills_test.go` is empty.

### 3.6 Dated records that name the squad

- `docs/audits/2026-11-05-agent-squad-rnd-audit.zh-CN.md:9` — "当前注册表有 **23 个角色模板：14 个公开工作角色、9 个小队队长；9 个小队模板；10 个自动化模板**。可靠性工程师、智能体评测工程师、体验验证工程师、迁移审查员、进展报告员和安全漏洞修复小队均已存在。" A dated audit; it is the only tracked non-task file found that states the totals 23 / 14 / 9 / 9 together.
- `artifacts/qa/collaboration-multi-account-20261109/environment/source-hashes.json:2027,6648,6724` — sha256 values for `e2e/security-remediation-squad.spec.ts` and the two embedded `INSTRUCTIONS.md` files as of that QA run. `git grep -l 'source-hashes' -- . ':!.trellis/tasks'` lists only the artifact's own `README.md` and `.trellis/workspace/artisan/journal-1.md`; no test or script reads the file.

### 3.7 Cross-reference: every non-Go place per change type

| Change | Places (this section) | Go tests (section 1) |
|---|---|---|
| Squad `Version` 1 → 2 | `e2e/security-remediation-squad.spec.ts:98`; `squads.mdx:44`; `squads.zh.mdx:42` and generated `squads.json:5`; `squad-source-map.md:143`; spec `builtin-templates.md:155` | 1.1 rows 5, 8, 11 |
| Leader `Version` 1 → 2 | `squad-source-map.md:143-144`; spec `builtin-templates.md:158` | 1.1 row 1 |
| One seat added | `e2e/security-remediation-squad.spec.ts:21,28` (via `:70`); `squads.mdx:24,44`; `squads.zh.mdx:24,42`; `agents-create.mdx:51`; `agents-create.zh.mdx:51`; generated `squads.json:5`, `agents-create.json:5`; `multica-squads/SKILL.md:214`; `squad-source-map.md:144`; spec `builtin-templates.md:159-160` | 1.1 rows 4, 9, 10, 16, 17, 18, 20 |
| New listed role and role skill | `agents-create.mdx:28,30-45,51`; `agents-create.zh.mdx:28,30-45,51`; `security-model.mdx:38`; `security-model.zh.mdx:38`; generated `agents-create.json:5`, `security-model.json:5`; `multica-creating-agents/SKILL.md:133`; `creating-agents-source-map.md:50`; `squad-source-map.md:145`; spec `builtin-templates.md:89-90,158-159,364,410`; frontend literals in sections 2.4 and 2.5 | 1.2 rows A-E, R, S |
| Prose rewrite of the two bodies | docs paragraphs `squads.mdx:44-60`, `squads.zh.mdx:42-58`; `multica-squads/SKILL.md:208-225`; `squad-source-map.md:145-153`; spec `builtin-templates.md:164-176`. The catalog summaries (`squads.json` locales, `builtin-squad-catalog.test.tsx`, e2e `:22,29`) are separate strings | 1.1 rows 7, 22, 23, 28, 29, 31; 1.2 row L |

## 4. Generated docs

### 4.1 What is generated, and from what

- Generator: `scripts/generate-docs-bundle.mjs` (110 lines).
  - `:20-22` default roots: sources `apps/docs/content/docs`, assets `apps/docs/public/images/docs`, output `server/internal/docs/content`.
  - `:54` only files ending in `.zh.mdx` become pages; `:30` and `:72` read `meta.zh.json` (root and per-directory).
  - `:36` the output root is removed (`rmSync(outputRoot, { recursive: true, force: true })`) and rebuilt in full on every run.
  - `:38-40` one `pages/<slug>.json` per page holding `slug`, `title`, optional `description`, `body` (after `jsxToDirective`), `toc` (`:57-64`).
  - `:42-46` assets are copied to `assets/images/docs/`; `:48` `manifest.json` is the nav plus the asset list.
  - `:102-105` JSON is written with two-space indentation and a trailing newline.
  - `:107-110` the CLI entry prints `Generated <n> docs pages and <m> assets.`
- Spec: `.trellis/spec/docs/frontend/docs-bundle.md:16-22` (pipeline diagram); `:25-26` "The generator packs **Chinese pages only** (`*.zh.mdx`). Other-language `.mdx` files serve the public docs site and never enter the binary."; `:33-35` "`server/internal/docs/content/` is **committed**: CI proves the embedded bytes have not drifted from the source. Always regenerate and commit after editing `.zh.mdx` or `meta.zh.json`."
- Inference from `:54`: an edit to an English `.mdx` alone changes no generated file; an edit to `squads.zh.mdx`, `agents-create.zh.mdx` or `security-model.zh.mdx` changes the matching `pages/<slug>.json`.

### 4.2 Regeneration command

- `package.json:33` — `"generate:docs-bundle": "node scripts/generate-docs-bundle.mjs"`.
- CI invokes it as `pnpm generate:docs-bundle` (`.github/workflows/ci.yml:300`); the spec writes it as `node scripts/generate-docs-bundle.mjs` (`.trellis/spec/docs/frontend/docs-bundle.md:162`, `:195`).
- No Makefile target and no `scripts/*.sh` caller: `git grep -n -E 'docs-bundle|generate:docs|internal/docs' -- scripts Makefile` returns only lines inside the generator itself.

### 4.3 The drift check

- The enforced check is a CI step, not a unit test: `.github/workflows/ci.yml:295-306`, step "Verify in-app docs bundle is up to date". It runs `pnpm generate:docs-bundle` (`:300`), then `git diff --exit-code -- server/internal/docs/content` (`:301`), then fails when `git ls-files --others --exclude-standard -- server/internal/docs/content` lists any untracked generated file (`:302-306`). Comment at `:297-298`: "Pages and assets are committed for go:embed. Rebuilding here catches source edits, stale files, and incomplete meta navigation in one check."
- The step runs when the `frontend` path filter matches (`:296`); that filter includes `apps/docs/**` (`:50`), `packages/**` (`:51`), `scripts/generate-docs-bundle.mjs` (`:60`) and `server/internal/docs/content/**` (`:61`).
- Tests that touch the bundle but do NOT compare the committed bundle with the `.zh.mdx` sources:
  - `apps/docs/lib/docs-bundle/generate-docs-bundle.test.ts:24-77` runs the generator against a temporary fixture (2 pages, 1 asset) and checks the output shape.
  - `apps/docs/lib/docs-bundle/docs-anchor-parity.test.ts:34` reads the committed bundle; it checks that every `DOCS_SLUGS` entry is published and that two anchors resolve (`:56-59`, `webhookEventFilters` and `customRuntimeProfiles`). `git grep -n -E 'squads|security' -- packages/core/docs/slugs.ts` returns nothing, so no product deep link targets the squad pages.
  - `server/internal/docs/embed_test.go:10-45` (`TestEmbeddedBundle`) loads the embedded manifest and requires page `daemon-runtimes` and asset `images/docs/runtime-machine-detail.webp`; `:69-147` validate manifest and resource structure on synthetic file systems.
  - `server/internal/docs/binary_scope_test.go:22-52` pins that `cmd/server` imports the docs package and `cmd/multica` does not.
- `.trellis/spec/docs/frontend/docs-bundle.md:163` names "`apps/docs` vitest (`docs-anchor-parity.test.ts`, bundle drift test)". A vitest file that performs a source-versus-bundle drift comparison was not found: `ls apps/docs/lib/docs-bundle/` lists `docs-anchor-parity.test.ts`, `generate-docs-bundle.test.ts`, `jsx-to-directive.test.ts`, `mermaid-fences.test.ts`, `parse-page.test.ts`, and `git grep -n -E 'server/internal/docs/content|internal/docs' -- apps/docs packages/views/docs packages/core/docs` returns only `docs-anchor-parity.test.ts:34`.

### 4.4 Where "98 个生成文件逐字节一致" comes from

- The phrase is a result row in the v1 task: `.trellis/tasks/archive/2026-10/10-26-security-remediation-squad/phase2-results.md:22` — "| 文档 | 62 项测试、类型检查、99 页生产构建通过；98 个生成文件逐字节一致 |".
- It records a manual procedure, described at `.trellis/tasks/archive/2026-11/11-05-agent-task-least-privilege/role-doc-verification.md:14`: "`node scripts/generate-docs-bundle.mjs`: 39 pages, 58 assets. The generator has no --check mode, so a second generation to a fresh private directory was compared byte-for-byte: all 98 files match." The same count appears at `.trellis/tasks/archive/2026-10/10-14-usage-docs-consistency-audit/audit-report.md:467` ("39 页、58 个资源，共 98 个文件逐字节一致").
- The spec states the procedure at `.trellis/spec/docs/frontend/docs-bundle.md:168-169`: "For a multi-page content audit, generate into a temporary output directory and compare all files byte-for-byte."
- 39 pages + 58 assets = 97. Inference: the 98th file is `manifest.json` (the generator writes pages, assets and exactly one manifest, `scripts/generate-docs-bundle.mjs:38-48`).
- The number is historical. Counts at research time: `git ls-files server/internal/docs/content | wc -l` = 157, of which `pages/` = 44, `assets/` = 112, plus `manifest.json`; sources are 44 `*.zh.mdx` files and 112 files under `apps/docs/public/images/docs`. Intermediate records: `.trellis/tasks/archive/2026-11/11-07-user-docs-feature-audit/progress.md:6` ("临时生成内容 103 文件逐字节一致") and `docs/audits/2026-11-07-user-docs-coverage.zh-CN.md:63` ("44 页、58 个资源，新增 5 页").

### 4.5 Generated pages that carry this squad's text today

- `server/internal/docs/content/pages/squads.json:5` (body) and `:14-15` (toc entry with title and id `安全漏洞修复`).
- `server/internal/docs/content/pages/agents-create.json:5` (body).
- `server/internal/docs/content/pages/security-model.json:5` (body).
- Each page body is a single JSON string on line 5, so any edit to the page shows as a one-line diff of the whole body.
- `git status --porcelain -- server/internal/docs apps/docs scripts/generate-docs-bundle.mjs` printed nothing at research time (these paths are clean in the working tree).

## 5. The gate for adding a new listed role

The spec and the code refer to the gate; its definition lives only in archived task research. `git grep -n -i -E 'workload.gate|two-cycle|workload-gated|WorkloadBacked|workload-backed' -- .trellis/spec server packages apps docs e2e scripts` returns four lines: `.trellis/spec/server/builtin-templates.md:112`, `server/internal/handler/agent_template_test.go:96` (test name), `server/internal/service/builtin_agent_templates_test.go:44` (test name) and `:55` (error text).

### 5.1 Statements in code and spec (verbatim)

- `server/internal/service/builtin_agent_templates_roster.go:9-13` — "Deliberately fourteen roles, not twenty. Technology-specific variants (frontend, backend, mobile, data) are NOT separate templates: they are the same Implementer with different skills and project resources attached. Splitting by stack multiplies prompts that all say the same thing and leaves a team picking between agents that differ in one paragraph."
- `server/internal/service/builtin_agent_templates_test.go:20-21` — "TestAgentRoleTemplates_ListedRosterIsFourteen pins the product decision. New listed roles must address distinct work rather than duplicate a role for each stack." The literal is at `:24` (`len(listed) != 14`).
- `server/internal/service/builtin_agent_templates_test.go:178-179` — "The stated mitigation for prompt bloat. Two default skills per role is a product decision, not an oversight to discover in production." The check is `len(template.RoleSkills) > 1` at `:177`, error text "the roster caps this at 1" at `:180`.
- `server/internal/service/builtin_agent_autonomy_test.go:464-468` — "TestSquadTemplates_StaffOnlyListedWorkingRoles guards the decision the role roster rests on: a new squad is a new ROUTING POLICY over listed roles, never an excuse to mint a one-off role. A new unlisted working role would arrive here first because a squad seat is the only place it becomes reachable without editing the picker's own test."
- `.trellis/spec/server/builtin-templates.md:89-90` — "Diagnostician brings the listed roster to fourteen roles and the role-skill catalog to fifteen entries."
- `.trellis/spec/server/builtin-templates.md:100-101` — "Reliability Engineer and Agent Evaluator are the two additional listed roles with independent evidence deliverables."
- `.trellis/spec/server/builtin-templates.md:111-112` — "Experience Validation Engineer and Migration Reviewer are listed specialists after the two-cycle workload gate was met."
- `.trellis/spec/server/builtin-templates.md:320-321` — "These remain composable responsibilities rather than five new listed roles."

### 5.2 The gate's definition (archived task research, verbatim)

- `.trellis/tasks/archive/2026-09/09-24-lifecycle-finalization/research/workload-gate.md:5-8` — "本轮复核使用最近两个已归档发布周期的记录。角色进入 listed roster 需要连续两个周期同时满足：" / "体验验证：每周期至少两条真实浏览器/桌面关键路径，并出现截图、失败分类或可访问性/跨端证据；角色交付物必须独立记录路径、环境、证据和结果风险。" / "迁移审查：每周期至少一项 schema、API 或客户端兼容迁移，并有独立的旧/新契约、兼容窗口、校验以及回滚/重试限制证据。"
  - `:19` (conclusion) — "两个周期都超过体验路径的最低数量，并且已有截图/失败分类/跨端差异证据；两个周期都有独立的客户端/API/schema 迁移和兼容、校验或回滚材料。因此本轮 workload gate 达标，新增 `experience-validation-engineer` 与 `migration-reviewer` listed roles。"
  - `:21` (caveat) — "这不是生产质量或真实模型通过声明：发布记录仍是脱敏仓库证据，未读取生产指标、凭据或客户数据；..."
- `.trellis/tasks/archive/2026-09/09-24-lifecycle-closure/research/workload-gate.md` (final text):
  - `:5` — "本次只统计仓库内已经归档的两个最近发布周期，不把当前未提交的计划当作工作量证据："
  - `:10` — "这些记录证明发布周期确实反复消费浏览器、桌面和跨平台验证，但不能自动证明新的 listed role 已经必要。门槛还要求独立交付物和现有 QA 的排队/覆盖缺口。"
  - `:16-17` — verdict rows, both "**达到 listed gate**".
  - `:21-24` — "若未来连续两个周期出现独立交付物缺失，需要重新评估 listed role：" followed by the experience and migration deliverable lists.
  - `:26` — "若未来门槛失守，暂停新副本创建并将专项路由降级为现有角色组合；现有 workspace 副本不被删除或覆盖。当前版本已完成四语模板、权限、skill、squad 和正负向 roster 测试。"
  - `:32` — "暂不新增角色或 MCP 的场景：隐私/合规、FinOps、客户支持反馈和自动生产控制。这些目前没有独立且稳定的交付物证据；隐私/供应链由安全与发布审查覆盖，..."
- The same file as first committed in `b80b297ef` (path at that commit: `.trellis/tasks/09-24-lifecycle-closure/research/workload-gate.md`; read with `git show b80b297ef:<path>`), when the gate was NOT met:
  - `:16-17` — both verdicts "**未达到 listed gate**"; the experience row gives the reason "没有逐周期可访问性结果，也没有 QA 排队、覆盖缺口或独立体验报告被现有流程明确交付", the migration row "这些是发布/安装验证，不是 schema/API/客户端迁移审查；没有两周期各一项的兼容窗口、校验查询、重试/孤儿数据或独立迁移报告".
  - `:21` — "只有在连续两个后续周期同时满足下列条件，才重新评估 listed role："
  - `:23` — "体验：每周期至少两条需要真实浏览器/可访问性/跨端证据的关键路径，且出现可复核 QA 排队或覆盖缺口；交付物必须包含路径、截图/日志、可访问性结果和跨端差异结论。"
  - `:24` — "迁移：每周期至少一项 schema、API 或客户端兼容迁移；交付物必须包含旧/新契约、兼容窗口、校验、回滚/重试条件和人工审批点。"
  - `:26` — "在门槛达到之前，不新增默认角色、不复制四语模板、不增加权限或 MCP。需要专项判断时使用现有角色组合和临时任务，完成后把实际交付物计入下一次 gate。"
- `.trellis/tasks/archive/2026-09/09-24-lifecycle-finalization/design.md:23` — "先从两个最近发布周期、迁移记录和 E2E/浏览器证据计算 gate，再决定是否新增 listed role。体验角色要求每周期至少两条真实浏览器/可访问性/跨端关键路径并有 QA 覆盖缺口；迁移角色要求每周期至少一项 schema/API/客户端迁移并有独立兼容窗口、校验和回滚交付物。任一条件未达标时只保留现有角色组合和负向 roster 断言。"
- `.trellis/tasks/archive/2026-09/09-24-lifecycle-finalization/prd.md:12` — "专项角色：以至少两个真实发布/迁移周期的记录重算 workload gate。达标时新增 `experience-validation-engineer` 与/或 `migration-reviewer` 的完整 listed role、skill、本地化、权限、模板和负向/正向 roster 测试；未达标时保留明确的路由级能力、复评条件和不出现在默认 roster 的测试。" Acceptance line at `:30`.
- `.trellis/tasks/archive/2026-09/09-24-lifecycle-dual-loops/design.md:17` — "首批只增加 `reliability-engineer` 和 `agent-evaluator`，因为两者有稳定、互不重叠的证据产物。`experience-validation-engineer` 与 `migration-reviewer` 先受 workload gate 控制：未达到阈值只提供 skill/路由；达到阈值后再加入 listed roster。契约兼容、威胁建模、供应链、产品结果、灾备先作为 skill 或 squad 路由，不按技术栈拆角色。"
- `.trellis/tasks/archive/2026-09/09-24-lifecycle-dual-loops/research/lifecycle-gap-matrix.md`:
  - `:20` — "明确不新增：按 frontend/backend/mobile 拆角色、自动生产操作、凭据型 MCP、无独立产物的“协调员”角色。"
  - `:24` — "... 门槛仍然保留：如果后续周期没有独立交付物，不应继续新增同类角色。"
  - `:26-34` — table "下一批候选（先做 skill，暂不新增默认 role）" with columns 优先级 / 场景 / 当前缺口 / 建议落点 / 升格条件. Row `:30` (隐私、合规和数据治理): 升格条件 "连续两个周期有独立的数据分类、保留/删除或合规审批交付物". Row `:32` (供应链、许可证和 SBOM): 建议落点 "扩充 security/release 的交付模板，必要时再拆 `multica-supply-chain-review`", 升格条件 "依赖/插件发布成为固定门禁，且有独立 owner 和失败样例".
  - `:36` — "不建议现在新增 frontend/backend/mobile 专家、单独协调员或“通用性能工程师”：它们没有独立边界，现有 implementer、QA、architect 和 reliability 已能承接。"
- `.trellis/tasks/archive/2026-09/09-24-lifecycle-closure-v2/research/lifecycle-scenario-audit.md:28` — "这些场景下一步应优先形成 skill/交接格式和失败样例，再根据连续两个周期的独立交付物、排队或返工证据决定是否升格为 listed role。"
- `.trellis/tasks/archive/2026-09/09-24-lifecycle-closure/design.md:25` — "体验/迁移角色先通过 workload gate 产出结论；只有证据达标才新增 listed roster、四语文案、权限、skill 和测试。"; `prd.md:11` — "... 未达阈值不新增 listed role、权限、skill 或 MCP。"
- Commit trailers: `b80b297ef` — "Rejected: Add experience-validation-engineer and migration-reviewer now | recent cycles lack the required accessibility/QA-gap and schema/API migration deliverables." and "Directive: Re-run the workload gate after two qualifying release cycles before adding either specialist to the default roster."; `c21d03a43` — "Add two workload-gated specialist roles after the two-cycle evidence gate was met ...".

### 5.3 What the gate requires, restated from those passages

1. Evidence base: the two most recent ARCHIVED release cycles in the repository; uncommitted plans do not count (`09-24-lifecycle-finalization/research/workload-gate.md:5`, `09-24-lifecycle-closure/research/workload-gate.md:5`).
2. Both cycles must satisfy the condition at once ("连续两个周期同时满足", finalization `workload-gate.md:5`), and the condition has two parts per candidate role:
   - a role-specific workload threshold — experience: at least two real browser / desktop critical paths per cycle; migration: at least one schema, API or client-compatibility migration per cycle (finalization `workload-gate.md:7-8`);
   - independent deliverables for that role — experience: path, environment, evidence, result risk; migration: old/new contract, compatibility window, validation, rollback / retry limits (same lines).
3. Workload volume alone is stated to be insufficient: "不能自动证明新的 listed role 已经必要。门槛还要求独立交付物和现有 QA 的排队/覆盖缺口。" (closure `workload-gate.md:10`). The first-round wording added a reviewable QA queueing or coverage gap for the experience role (`b80b297ef` version `:23`; finalization `design.md:23`).
4. Until the gate is met: no new default role, no localized template copies, no added permissions or MCP; existing role combinations and skill / routing are used, and the actual deliverables are counted toward the next gate run (`b80b297ef` version `:26`; dual-loops `design.md:17`; finalization `design.md:23`).
5. When the gate is met, the stated package is the complete listed role: role, skill, localization, permissions, templates, and positive and negative roster tests (finalization `prd.md:12`).
6. After listing, the gate stays in force: two consecutive cycles without independent deliverables trigger re-evaluation (closure `workload-gate.md:21`), and a lost gate pauses new copies and downgrades the routing while existing workspace copies are neither deleted nor overwritten (closure `workload-gate.md:26`).
7. For scenarios other than the two gated roles, the recorded order is skill / handoff format and failure samples first, then promotion to a listed role based on two consecutive cycles of independent deliverables, queueing or rework evidence (closure-v2 `lifecycle-scenario-audit.md:28`; per-candidate 升格条件 in `lifecycle-gap-matrix.md:30-34`).

### 5.4 What the passages do not state

- No generic numeric threshold for an arbitrary new listed role. Numeric thresholds exist only for the experience and migration roles (finalization `workload-gate.md:7-8`); other candidates carry one qualitative 升格条件 each (`lifecycle-gap-matrix.md:30-34`).
- No code or test evaluates the gate. What the test suite enforces is the outcome: the literal 14 (`builtin_agent_templates_test.go:24`), the ordered key list (`:31-41`), the one-skill cap (`:177-181`) and listed-only seats (`builtin_agent_autonomy_test.go:469-481`).
- The `.trellis/spec/` tree does not restate the gate's thresholds; the only spec mention is `builtin-templates.md:111-112`.

## 6. Version-bump semantics

### 6.1 (i) Are existing workspace agents and squads affected by a bump?

No code path found rewrites an existing workspace row from the registry. The registry text and version are copied at creation time.

**Squad row**

- `server/internal/handler/squad_template.go:425-438` is the single write of a template squad: `CreateSquadFromTemplate` with `Instructions: in.Template.Instructions()`, `TemplateKey: in.Template.Key`, `TemplateVersion: in.Template.Version`. Comment at `:432-434`: "The leader briefing is COPIED here, like the role instructions: the squad's routing policy is workspace-owned from this moment on and no release rewrites it."
- The SQL is an INSERT only: `server/pkg/db/queries/squad.sql:6-15`.
- `server/internal/handler/squad.go:47-51` — "TemplateKey and TemplateVersion record the built-in squad template this squad was staffed from. Provenance only — the instructions on the row are the workspace's from the moment it is created."
- `server/internal/service/builtin_skills/multica-squads/SKILL.md:227-228` — "`squad.template_key` / `template_version` record the provenance. They are not status: a staffed squad is an ordinary squad."

**Agent rows**

- New agent, squad path: `server/internal/handler/squad_template.go:526-547` (`CreateAgent` with `Instructions: template.Instructions()`, `TemplateKey`, `TemplateVersion: template.Version`, `AutonomyLevel`).
- New agent, single-role path: `server/internal/handler/agent_template.go:200-224` (`agentTemplateProvenance{Key, Version, Autonomy}`), written at `server/internal/handler/agent.go:1568-1570`.
- Existing agent: `server/internal/handler/squad_template.go:486-511`. The comment at `:507-510` reads "Reuse as-is. Notably we do NOT re-apply the template: the workspace may have edited this agent's instructions, lowered its autonomy, or moved it to another runtime, and staffing a second squad is not consent to undo that."
- The reuse lookup has no version predicate: `server/pkg/db/queries/agent.sql:41-54` (`WHERE workspace_id = $1 AND template_key = $2 AND kind = 'user' AND archived_at IS NULL ORDER BY created_at ASC LIMIT 1`).
- `server/internal/handler/agent.go:87-92` — "They are provenance only: the instructions on the row are the workspace's, and a newer template never rewrites them."
- `server/internal/service/builtin_agent_templates.go:12-21` — "a later release that rewrites a template can never overwrite that edit; agent.template_key / template_version record where the copy came from, so a future upgrade can show a diff instead of silently replacing prose a team has tuned."

**Role skills**

- `server/internal/handler/agent_template.go:297-354` (`materializeRoleSkillsInTx`): an existing workspace skill with the same name is returned unchanged (`:305-312`); only a missing one is written, with `config.origin = {type, name, version}` (`:335-340`). Comment at `:284-288`: "there is no update path here at all, so no release can rewrite tuned prose".
- `server/internal/service/builtin_role_skills.go:22-26` and `:40-42` ("Version is bumped when the content changes materially. Recorded on the materialized row; never used to overwrite one.").

**Migrations**

- `git grep -n -l 'template_version' -- server/migrations` lists migrations 450 (agent), 451 (squad), 455 (autopilot) and 460 (workspace MCP). `git grep -n -E 'template_key|template_version' -- server/migrations` minus those four returns one line, `server/migrations/512_workspace_mcp_template_source.up.sql:5`, which is on the workspace-MCP table. No migration found that rewrites agent or squad instructions or versions.

**Spec**

- `.trellis/spec/server/builtin-templates.md:150-152` — "Increment template and role-skill versions for material behavior changes. These defaults apply when creating agents or materializing a missing role skill; existing workspace copies are reused without overwriting customized content."
- `:201-205` — "`POST /api/squads/from-template` reuses existing workspace agents by template key and preserves their runtime, edits and skills. Supplying a new runtime to that request does not relocate reused members."
- `:340-343` — "Version changes identify new defaults; they never re-materialize or overwrite existing workspace copies, even when a new agent binds an older skill by name. `TestCreateAgentFromTemplate_ObserverRevisionsPreserveWorkspaceCopies` verifies new versions and unchanged old instructions, content, config, files and bindings."
- `:372-377` — "Language-only edits keep the existing behavior version; bundle hashes already track content changes. Materialization must continue to reuse existing workspace copies unchanged. Updating an existing copy is an explicit content update scoped to the intended workspace, with unrelated frontmatter and customizations preserved."
- `:653` — "Role template upgrades never overwrite existing workspace copies; new defaults require explicit instance upgrade for old agents."

**Tests that demonstrate the copy rule**

- `server/internal/handler/agent_template_test.go:129-204` (`TestCreateAgentFromTemplate_ObserverRevisionsPreserveWorkspaceCopies`): a second agent of the same role gets the current body (`:188`), the first agent's edited instructions are unchanged (`:193`), both bind the same pre-existing skill row (`:196-200`), and that skill row is `reflect.DeepEqual` before and after (`:203`).
- `server/internal/handler/squad_template_security_test.go:116-133,160-165` (section 1.1 row 19).

**Consequences that follow from those code paths (inference; for (a) and (b) no test was found that exercises them with an older-version row, (c) is exercised on the single-role path)**

- (a) After a leader bump: `POST /api/squads/from-template` always inserts a new squad row stamped with the current squad version and body (`squad_template.go:425-438`), but its leader comes from the reuse lookup. A workspace that already holds a `security-remediation-lead` agent created at v1 gets that agent as leader, still carrying the v1 instructions and `template_version = 1` (`squad_template.go:486-511`, `agent.sql:48-54`). The same applies to each reused seat.
- (b) After a squad bump, project adoption: `PUT /api/projects/:id/execution-squads` with a `template_key` tries the selection's own `squad_id` first when one is supplied (`server/internal/handler/project_execution_squad.go:552-561`), then lists existing squads with that key (`:562`; SQL `server/pkg/db/queries/squad.sql:43-48`, `WHERE workspace_id = $1 AND template_key = $2 AND archived_at IS NULL`, oldest first, no version predicate) and returns the first one that passes `loadInvocableProjectSquad` (`:566-570`). `materializeSquadTemplateInTx` runs only when none qualifies (`:576`). A workspace with a usable v1 squad is therefore handed that v1 row by this path.
- (c) A role-skill name the workspace lacks is materialized at the release's version; a name it already has is reused with whatever content and origin version it holds (`agent_template.go:305-312`). This one is exercised on the single-role path: `server/internal/handler/agent_template_test.go:171-176` rewrites the materialized skill row with origin version 1 (the registry versions in that table are 2 and 6, `:136-137`), `:183-186` creates a second agent from the same role, `:196-200` requires both agents to bind that same skill row and `:201-205` requires the row to be unchanged. On the squad path, `server/internal/handler/squad_template_security_test.go:118-120,160-165` edits the content, description and config of `multica-security-review` and requires the row to be unchanged after staffing; it does not change the origin version.
- Search for a test covering (a) or (b): `git grep -n -E 'SET template_version|template_version *= *[0-9]' -- 'server/internal/handler/*_test.go' 'server/cmd/server/*_test.go'` returns only `server/cmd/server/agent_artifact_integration_test.go:247` (a task-capability fixture). Not found.

### 6.2 (ii) Is there an upgrade-diff or "newer template available" surface?

Not found. What consumes the stored version today:

- Server non-test hits for `template_version|TemplateVersion|templateVersion` (excluding generated sqlc and migrations), agent and squad rows only:
  - response fields — `server/internal/handler/agent.go:91-92,232`; `server/internal/handler/squad.go:50-51,94`;
  - writers — `server/internal/handler/agent.go:1569`; `server/internal/handler/squad_template.go:437,545`;
  - authorization — `auth.TaskCapabilityProfile(agent.TemplateKey, agent.TemplateVersion)` at `server/internal/auth/task_capability.go:126`, `server/internal/handler/daemon.go:2307`, `server/internal/handler/task_capability.go:34`, `server/internal/service/task_capability.go:22,71`, `server/internal/service/artifact_authority.go:318`, `server/internal/service/delivery_completion_authorization.go:670,745` (working-tree numbers; the file has uncommitted changes and the same two lines are `:634` and `:709` at HEAD).
  - The remaining hits are other tables: autopilot templates (`handler/autopilot.go`, `handler/autopilot_template.go`, `service/report_snapshot_collect.go:214-224`, `service/report_snapshot_collection_audit.go:61`) and workspace MCP recipes (`handler/workspace_mcp_api.go`).
- No row-versus-registry comparison: `git grep -n -E 'TemplateVersion *(<|>|!=|==)|\.Version *(<|>|!=) *(agent|squad|existing)\.' -- server ':!*_test.go' ':!server/pkg/db/generated'` returns only nil checks in `workspace_mcp_api.go` and two autopilot report-snapshot lines.
- `TaskCapabilityProfile` (`server/internal/auth/task_capability.go:23-39`) is the one runtime decision that depends on the stored version: `progress-reporter` at `>= 2`, `code-reviewer` and `security-reviewer` at `>= 2`, `migration-reviewer` at `>= 3`; every other key returns "". Its table is pinned by `server/internal/auth/task_capability_test.go:16-24` (v1 gives "", v2 gives the profile, `migration-reviewer` v2 gives "", v3 gives reviewer, `custom` v999 gives "").
  - Inference: because the comparisons are `>=`, raising the version of one of those roles keeps its profile. A key the switch does not name (this includes `security-remediation-lead` and any new role key) has no profile at any version.
  - Literal role keys in non-test Go outside the two registries: `git grep -n -E '"(security-reviewer|code-reviewer|migration-reviewer|diagnostician|qa-engineer|implementer|product-analyst|security-remediation-lead|security-remediation)"' -- 'server/*.go'` minus tests and the two registry files returns `auth/task_capability.go:29,33` and one unrelated JSON tag (`service/collaboration_stage_view.go:38`).
- Frontend: `template_version` is typed and schema-parsed only — `packages/core/types/agent.ts:515`, `packages/core/types/squad.ts:33`, `packages/core/api/schemas.ts:2196` (`AgentSchema`), `:2385` (`SquadSchema`). `git grep -n -E 'template_version' -- packages/views` outside MCP returns one test fixture (`packages/views/agents/create/use-create-template-agent-submit.test.tsx:70`).
- The one place a template version is displayed: `packages/views/agents/create/template-create-agent-page.tsx:315-318` renders `role_templates.version` ("Template v{{version}}" at `packages/views/locales/en/agents.json:400`, "模板 v{{version}}" at `packages/views/locales/zh-Hans/agents.json:390`) from the catalog's `template.version`. The value follows the registry; this is the listed-role picker, so the unlisted leader does not appear there.
- No squad-side version string: `git grep -n -i -E '"[^"]*version[^"]*":|版本' -- packages/views/locales/en/squads.json packages/views/locales/zh-Hans/squads.json` matches only the release-squad summary at `zh-Hans/squads.json:182`.
- No "newer template" copy: `git grep -n -i -E 'newer (template|version)|template (update|upgrade)|upgrade (available|template)|outdated template|新版模板|模板更新|模板升级|有新版本' -- packages/views packages/core apps/web apps/desktop server/internal ':!server/internal/docs'` returns collaboration-stage, issue-field and project-conflict strings and no template surface.
- Statements that place an upgrade surface in the future or in a manual workflow: `server/internal/service/builtin_agent_templates.go:19-21` ("so a future upgrade can show a diff"); `server/internal/service/builtin_role_skills.go:25-26` ("so a future upgrade path can tell a pristine copy from an edited one without a migration"); `.trellis/spec/server/builtin-templates.md:353-359` ("Use the explicit maintenance workflow in `scripts/localize-agent-instructions.md` for existing instructions. Template key/version alone cannot identify an unmodified copy: compare the full historical body ... Agent/squad PUT supports paired `expected_instructions` and `expected_updated_at` fields with an atomic SQL comparison.").
- The maintenance workflow those statements point to is an operator-run script, not a product surface: `scripts/localize_agent_instructions.py`, documented in `scripts/localize-agent-instructions.md`. What it does with template provenance:
  - `:301-329` (`catalog_from_git`) reads `builtin_agent_templates_roster.go` and `builtin_squad_templates.go` twice — at a baseline commit (`:314`) and in the working tree (`:315`) — and, for every `*/INSTRUCTIONS.md` under `builtin_agent_templates/` and `builtin_squad_templates/` (`:310-316`), takes each version with the regular expression `Key:\s*"<key>",\s*Version:\s*(\d+)` (`:318`). A key missing from either roster is skipped (`:319-321`). The entry keeps the baseline body as `source`, the working-tree body as `target`, and both versions (`:322-329`). The source layout that this expression reads today is `Key:` followed directly by `Version:` (`server/internal/service/builtin_agent_templates_roster.go:374-375`, `server/internal/service/builtin_squad_templates.go:311-312`).
  - `:105-135` (`build_plan`) lists `/api/agents` and `/api/squads` (`:111-112`), reads each row's `template_key` and `template_version` (`:118`), and selects a row automatically only when `type(version) is int and version == template["source_version"] == template["target_version"] and before == template["source"]` (`:126-128`). A row whose body already equals the target is reported as `already_chinese_default` (`:124-125`); every other row keeps `custom_or_unknown` (`:120`).
  - `scripts/localize-agent-instructions.md:25-29` states the same three conditions; `:31` adds "版本相同但正文改过的记录不会选中；版本不同，即使名称相同也不会自动升级。"; `:33` says "工具只写正文"; `:52` says each PUT carries only `instructions`, `expected_instructions` and `expected_updated_at`.
  - The documented baseline is `2a1b56735` (`scripts/localize-agent-instructions.md:15`; commit date 2026-09-12 10:16 +0800). Neither `security-remediation` nor `security-remediation-lead` is in the roster files at that commit (A22 under "Unknown / not found"). Inference: with that baseline both keys are skipped at `:319-321` and their rows are reported as `custom_or_unknown`. The script was not run.
  - The script's own test pins the version rule with a literal catalog: `scripts/test_localize_agent_instructions.py:99-104` sets `target_version = 2` on a `code-reviewer` entry and requires the row to stay unselected. The documented way to run it is by hand (`scripts/localize-agent-instructions.md:76`); no CI, Makefile or package script that runs it was found (A20).
  - `docs/audits/2026-11-05-agent-squad-rnd-audit.zh-CN.md:85` lists this workflow in its "实例升级" row, next to `server/internal/service/builtin_agent_templates.go:12`.

### 6.3 (iii) Tests that pin versions of the roles this squad reuses

| Role (registry role / skill version) | Literal pins found | Breaks on |
|---|---|---|
| `security-reviewer` (2 / `multica-security-review` 4) | `server/internal/service/builtin_agent_templates_test.go:346` in `TestReadOnlyRoleTemplates_CapabilityDeliveryContract` (role 2, skill 4; compared at `:359`); `:415` in `TestGovernanceSkills_FailureExampleContracts` (skill 4) | role bump, skill bump |
| `code-reviewer` (2 / `multica-code-review` 2) | `...builtin_agent_templates_test.go:345` (role 2, skill 2) | role bump, skill bump |
| `migration-reviewer` (3 / `multica-migration-review` 2) | `...builtin_agent_templates_test.go:347` (role 3, skill 2); `server/internal/handler/agent_template_test.go:136` (role 3, skill 2; asserted at `:148`, `:158`, `:163`) | role bump, skill bump |
| `diagnostician` (1 / `multica-debugging` 1) | `...builtin_agent_templates_test.go:447` (role 1), `:459` (skill 1); `server/internal/handler/agent_template_test.go:233` (`TemplateVersion != 1`), `:250` (origin version 1) | role bump, skill bump |
| `product-analyst` (1 / `multica-requirement-clarification` 3) | no Go pin found; e2e `e2e/agent-autonomy-gates.spec.ts:241,259` creates a `product-analyst` and expects the stored `template_version` `toBe(1)` | role bump (e2e) |
| `implementer` (1 / `multica-test-report` 1) | none found | — |
| `qa-engineer` (1 / `multica-test-report` 1) | none found | — |
| `security-remediation-lead` (1 / none) | section 1.1 row 1 | lead bump |
| squad `security-remediation` (1) | section 1.1 rows 5, 8, 11; e2e `e2e/security-remediation-squad.spec.ts:98` | squad bump |

- Searches behind "none found": `git grep -n -E 'multica-test-report|multica-requirement-clarification' -- 'server/*_test.go'` filtered for `version` or a trailing number returns nothing; `git grep -n -E '"(qa-engineer|implementer|product-analyst)"' -- 'server/*_test.go'` filtered the same way returns nothing; `git grep -n -E '(\.Version|TemplateVersion|template_version) *(!=|==) *[0-9]' -- 'server/internal/service/*_test.go' 'server/internal/handler/*_test.go'` lists, for role and squad templates, exactly: `agent_template_test.go:233,394`; `squad_template_security_test.go:38,59`; `builtin_agent_autonomy_test.go:324`; `builtin_agent_templates_test.go:274,314,447,459`; `builtin_security_remediation_templates_test.go:15,39` (table-driven pins such as `:344-347` and `:413-417` are not matched by that pattern and were read directly).
- Pins on templates outside this squad, for completeness: `progress-reporter` 2 and `multica-progress-report` 5 (`builtin_agent_templates_test.go:274,314,344,416`; `handler/agent_template_test.go:394,414`); `architect` 2 and `multica-architecture-decision-record` 6 (`handler/agent_template_test.go:137`; `builtin_agent_templates_test.go:413`); `multica-release-check` 4 (`:414`); `multica-reliability-engineering` 2 (`:417`); `review-gate` leader 3 and squad 3 (`builtin_agent_autonomy_test.go:324`).
- Registry-relative assertions that follow a bump: `server/internal/handler/agent_template_test.go:119` and `:329` (`TemplateVersion != role.Version`), `server/internal/handler/agent_category_test.go:94-96`, `server/internal/handler/squad_template_security_test.go:83`.
- Fixtures that write an explicit `template_version` onto a row and do not read the registry (unaffected by a registry bump): `server/internal/auth/task_capability_test.go:20,53`; `server/internal/handler/task_capability_test.go:92`; `server/internal/handler/task_capability_runtime_lookup_test.go:14,44`; `server/internal/handler/comment_delegation_test.go:405`; `server/internal/service/task_capability_test.go:14`; `server/cmd/server/task_capability_targeted_test.go:88,189,227-228,240`; `server/cmd/server/task_capability_review_test.go:18`; `server/cmd/server/task_capability_test.go:18`; `server/cmd/server/report_snapshot_integration_test.go:38,101,156,186,193`; `server/cmd/server/report_snapshot_read_barrier_test.go:132,190`; `server/cmd/server/durable_delegation_integration_test.go:524`; `server/cmd/server/agent_artifact_integration_test.go:247`. e2e mocks with a literal version: `e2e/agent-role-template.spec.ts:152,181`, `e2e/localized-template-defaults.spec.ts:167`, `e2e/agents-discovery.spec.ts:59`.

### 6.4 (iv) Tests that pin a role skill's `config.origin.version`

- `server/internal/handler/agent_template_test.go:161-165`, inside `TestCreateAgentFromTemplate_ObserverRevisionsPreserveWorkspaceCopies` (`:129`): `SELECT (config->'origin'->>'version')::int FROM skill WHERE id = $1` must equal the case's `skillVersion` — `multica-migration-review` 2 (`:136`) and `multica-architecture-decision-record` 6 (`:137`). The same test then overwrites the skill's origin version to 1 through the API (`:174`) and requires the row to stay unchanged after a second agent is created (`:201-204`).
- `server/internal/handler/agent_template_test.go:240-252`, inside `TestCreateAgentFromTemplate_DiagnosticianCopiesAndPreservesWorkspaceContent` (`:210`): `config->'origin'->>'type'` must equal `roleSkillOriginType` and the origin version must be 1, with presentation `quality` / `microscope`.
- `server/internal/handler/agent_template_test.go:411-416`, inside `TestCreateAgentFromTemplate_ProgressReporterUsesServerDefaults` (`:370`): `config->'origin'->>'version'` must be the string "5".
- Writer under test: `server/internal/handler/agent_template.go:335-340`; origin type constant at `:374`.
- Fixtures that carry an origin version without reading the registry: `server/cmd/server/skill_template_test.go:76` (`"version":2` for `multica-release-check`), `packages/views/skills/lib/skill-presentation.test.ts:59` (`version: 1`), `packages/views/skills/components/skill-detail-page.test.tsx:286,318` (`version: 1`), `e2e/skill-library-accessibility.spec.ts:42` (`version: 1`).
- Of the skills this squad's roles attach, origin-version pins exist for `multica-migration-review` (2) and `multica-debugging` (1). No origin-version pin was found for `multica-security-review`, `multica-code-review`, `multica-test-report` or `multica-requirement-clarification`; their registry-level pins, where they exist, are in 6.3. Search: `git grep -n -i -E "origin.{0,40}version" -- 'server/*_test.go'` and the same pattern over `e2e packages/views packages/core` (A10 under "Unknown / not found").

### 6.5 Spec statements whose version numbers differ from the registry

Recorded as observed values only.

- Migration Reviewer role: `.trellis/spec/server/builtin-templates.md:115` ("template version two") and `:333` ("Migration Reviewer template version 2"); `:77-78` says "Migration Reviewer v3"; registry `server/internal/service/builtin_agent_templates_roster.go:251` is 3.
- Migration Review skill: spec `:338` ("Migration Review's skill remains version 1"); registry `server/internal/service/builtin_role_skills.go:73` is 2.
- Progress-report skill: spec `:328` ("Progress-report version 4"); registry `server/internal/service/builtin_role_skills.go:68` is 5.
- Migration Reviewer role, in the embedded product skill: `server/internal/service/builtin_skills/multica-creating-agents/SKILL.md:197` ("The listed `migration-reviewer` role (template version 2)"); the same file says "Migration Reviewer v3" at `:167`, and `references/creating-agents-source-map.md:57` says "template v3 / skill v2".
- Leader concurrency, in the embedded product skill: `server/internal/service/builtin_skills/multica-squads/references/squad-source-map.md:144` ("concurrency two"); the registry and test value is 6 (section 3.5).

## 7. Squad templates and human members

Short answer, with the evidence below: a built-in squad template can only name role templates as seats, `POST /api/squads/from-template` has no members field, and neither creation UI collects a person. A human is added to the staffed squad afterwards through `POST /api/squads/{id}/members` or the squad detail page's add-member dialog.

### 7.1 Can a built-in squad template seat a human member?

No field for it exists.

- `server/internal/service/builtin_squad_templates.go:28-38` — `SquadRoleSlot`, "one non-leader seat in a template's roster", has exactly two fields: `TemplateKey string` ("the role template the seat is filled from", `:30-34`) and `Roles map[string]string` (`:35-37`: "the localized `squad_member.role` note. It is context for the leader's roster and a label in the UI — it grants nothing.").
- `server/internal/service/builtin_squad_templates.go:40-52` — `SquadTemplate` fields are `Key`, `Version`, `DefaultName`, `AvatarEmoji`, `LeaderTemplateKey`, `Members []SquadRoleSlot`, `Titles`, `Descriptions`. There is no member-type, user or person field.
- `server/internal/service/builtin_squad_templates.go:73-82` — `TemplateKeys()` returns "every role template this squad needs, leader first"; it is the provisioning order.
- Materialization writes agent members only: `server/internal/handler/squad_template.go:395` iterates `in.Template.TemplateKeys()`; the leader is inserted with `MemberType: "agent"`, `Role: "leader"` (`:446-451`); every seat is inserted with `MemberType: "agent"` and the localized role note (`:465-470`).
- The catalog response carries role entries only: `server/internal/handler/squad_template.go:36-44` (`key`, `version`, `name`, `title`, `description`, `avatar_emoji`, `instructions`, `leader`, `members`) and `:50-55` (per role: `template_key`, `title`, `name`, `autonomy_level`, `avatar_emoji`, `role`).
- Tests that hold every seat to a listed role template: section 1.2 row P (`server/internal/service/builtin_agent_autonomy_test.go:469-481`) and section 1.1 row 27 (`:220-234`).

### 7.2 Does `POST /api/squads/from-template` accept extra members?

No.

- Request struct, `server/internal/handler/squad_template.go:114-129`: `template_key` (`:115`), `runtime_id` (`:119`), `name` (`:121`), `permission_mode` (`:126`), `invocation_targets` (`:127`), `language` (`:128`). No members field.
- Decoding: `json.NewDecoder(r.Body).Decode(&req)` at `:149`. `DisallowUnknownFields` does not appear in `squad_template.go` (`grep -n DisallowUnknownFields server/internal/handler/squad_template.go` is empty). Inference: an extra JSON key such as `members` is ignored, not rejected; no test of that was found.
- Handler `CreateSquadFromTemplate`, `:141-286`: workspace member required (`:143`); agent callers need Coordinator autonomy (`:166`) and must be allowed to grant the roster's highest level (`:172`); provisioning at `:222`; the analytics member count is `len(template.Members)+1` (`:278`).
- Response, `:134-138`: `squad`, `created_agent_ids`, `reused_agent_ids`.
- Route: `server/cmd/server/router.go:2412`, comment `:2410-2411` ("Staffs the roster and the squad in one transaction, reusing any agent the workspace already has for a role.").
- Client and type: `packages/core/api/client.ts:3914` (`createSquadFromTemplate`); `packages/core/types/agent-template.ts:85-95` (`template_key`, `runtime_id`, `name?`, `permission_mode?`, `invocation_targets?`, `language?`). The comment at `:90-91` describes `permission_mode` as "Applied to every agent this call creates"; these two fields are Access settings on created agents, not squad membership.
- Project adoption, `PUT /api/projects/:id/execution-squads` (`server/cmd/server/router.go:2396`): one selection is `{ template_key?, squad_id?, runtime_id?, language? }` (`packages/core/types/project.ts:15-20`); the server reads `Selection.TemplateKey`, `.SquadID`, `.RuntimeID`, `.Language` (`server/internal/handler/project_execution_squad.go:528-553`). No members field.
- Manual creation, `POST /api/squads` (`server/cmd/server/router.go:2408`): the request is `{ name, description, leader_id, avatar_url }` (`server/internal/handler/squad.go:270-275`) and the handler adds only the leader, as `MemberType: "agent"` (`:336-342`). No members list there either.

UI callers:

- Template staffing page: `packages/views/squads/create/template-create-squad-page.tsx:93-102` calls `api.createSquadFromTemplate({ template_key, runtime_id, name, permission_mode, invocation_targets, language })`. The page renders the template's role seats keyed by `member.template_key` (`:309`). The call carries no member input.
- Apply-to-project dialog: `packages/views/projects/components/use-squad-for-project-dialog.tsx:97-104` builds `{ template_key, runtime_id?, language }` per squad and calls `configure.mutateAsync({ id: targetProjectId, squads })`. No member input.

### 7.3 Where a human member is added afterwards

**API**

- `POST /api/squads/{id}/members` → `AddSquadMember` (`server/cmd/server/router.go:2419`; `server/internal/handler/squad.go:1001-1094`):
  - caller must be a workspace member (`:1003`), pass `canManageSquad` (`:1012`), and, if the caller is an agent, hold Coordinator autonomy (`:1019`; comment `:1016-1018`: "deciding who is on a squad is deciding who work gets routed to");
  - body `{ member_type, member_id, role }` (`:1027-1031`); `member_type` must be `agent` or `member` (`:1036-1038`);
  - for `member`, `member_id` is a user id validated with `GetMemberByUserAndWorkspace`; a miss returns 400 "member not found in this workspace" (`:1066-1073`);
  - a duplicate returns 409 "member already in squad" (`:1082-1084`); success returns 201 and publishes `EventSquadUpdated` (`:1090-1093`).
- Sibling routes: `GET /members` (`router.go:2417`), `GET /members/status` (`:2418`), `DELETE /members` (`:2420`), `PATCH /members/role` (`:2421`).
- Client and types: `packages/core/api/client.ts:6895` (`addSquadMember(squadId, { member_type, member_id, role? })`), `:6899` (`removeSquadMember`), `:6903` (`updateSquadMemberRole`); `packages/core/types/squad.ts:1` (`SquadMemberType = "agent" | "member"`), `:71-75` (`AddSquadMemberRequest`).

**UI**

- Squad detail page, `packages/views/squads/components/squad-detail-page.tsx`: the mutation at `:130-135` calls `api.addSquadMember(squadId, { member_type: input.type, ... })`; `availableMembers` are workspace members not already in the squad (`:190`); `AddMemberDialog` is rendered at `:256-260` and defined at `:474-615`, with a members section (`:552`; choosing a person sets `{ type: "member", id: m.user_id, name: m.name }` at `:558`), an agents section (`:570`, `:576`) and an optional role field (`:595-602`); the add-member button sits on the members tab (`:1116`).
- Manual creation page, `packages/views/squads/create/manual-create-squad-page.tsx:95` (`api.createSquad`) then `:107-108` (`api.addSquadMember` for each selected member with `member_type: m.type`).

**How a human member is treated downstream (anchors only; the caller has read `squad_briefing.go`)**

- Leader briefing roster: `server/internal/handler/squad_briefing.go:317-338` switches on member type; `case "member"` (`:329`) renders `formatRosterRow(user.Name, "member (human)", role, "", formatMention(user.Name, "member", userID))` (`:338`), with the comment at `:334-336` "Humans have no Multica skills, so no skills segment is rendered." The skill-name loader skips non-agent members (`:283-286`).
- Project squad validation collects the leader and agent members only: `server/internal/handler/project_execution_squad.go:595-599` (`if member.MemberType == "agent"`).
- Tests that put a human in a squad (not template-specific): `server/internal/handler/squad_briefing_test.go:219` (`MemberType: "member"`) with the roster assertion at `:351-352`; `server/internal/handler/squad_membership_test.go:74`; `packages/core/api/schemas.test.ts:1003`.
- No test adds a human to a squad staffed from a template: `git grep -n -E '"member"' -- server/internal/handler/squad_template_test.go server/internal/handler/squad_template_security_test.go server/internal/handler/squad_template_access_test.go server/internal/handler/project_execution_squads_test.go` returns only `squad_template_access_test.go:25,27,31,42`, where `"member"` is a workspace role or an invocation-target type.

## Unknown / not found

Nothing in this research was executed against a database, a build, a browser or a network (task constraint). Part A lists what was searched for and not found, each with the search that was run. Part B lists statements in sections 1-7 that rest on reading code and are not confirmed by a run or by a covering test. Part C lists material that exists and was not read.

Coverage of the searches: `git grep` reads the tracked files of the working tree at HEAD `ccd28cbc0` (section 0). Untracked files are outside it. `git ls-files --others --exclude-standard -- server packages e2e apps scripts docs .github` lists 22 untracked files; piping that list to `grep -l -i -E 'security-remediation|安全漏洞修复|builtinRoleSkillVersions|AgentRoleTemplates\(|builtin_role_skill'` matches none of them.

### A. Searched for and not found

**A1. A Go test that asserts the literal total 23 for role templates.**
- Search: `git grep -n -E '(!=|==) *23([^0-9]|$)|want 23|twenty-three|23 (role|template)' -- 'server/*_test.go'`
- Result: five lines, none about role templates (listed in 1.4). The only total-count check found is the map-length comparison at `server/internal/service/builtin_agent_templates_test.go:228`.

**A2. A definition of the workload gate outside archived task research; code or a test that evaluates the gate; a numeric threshold that applies to an arbitrary new role.**
- Search: `git grep -n -i -E 'workload.gate|two-cycle|workload-gated|WorkloadBacked|workload-backed' -- .trellis/spec server packages apps docs e2e scripts`
- Result: four lines — `.trellis/spec/server/builtin-templates.md:112`, `server/internal/handler/agent_template_test.go:96` (a test name), `server/internal/service/builtin_agent_templates_test.go:44` (a test name) and `:55` (error text). The definition, with numeric thresholds for the experience-validation and migration roles only, is in the archived files quoted in 5.2.

**A3. A vitest that compares the committed docs bundle with its `.zh.mdx` sources (the "bundle drift test" named at `.trellis/spec/docs/frontend/docs-bundle.md:163`).**
- Search: `ls apps/docs/lib/docs-bundle/`; `git grep -n -E 'server/internal/docs/content|internal/docs' -- apps/docs packages/views/docs packages/core/docs`
- Result: five test files (`docs-anchor-parity.test.ts`, `generate-docs-bundle.test.ts`, `jsx-to-directive.test.ts`, `mermaid-fences.test.ts`, `parse-page.test.ts`); the grep returns only `apps/docs/lib/docs-bundle/docs-anchor-parity.test.ts:34`, which checks published slugs and two anchors (4.3). The enforced drift check found is the CI step at `.github/workflows/ci.yml:295-306`.

**A4. A Makefile target or `scripts/*.sh` wrapper for bundle generation; a check-only mode in the generator.**
- Search: `git grep -n -E 'docs-bundle|generate:docs|internal/docs' -- scripts Makefile`; `grep -n -E -- '--check|process\.argv' scripts/generate-docs-bundle.mjs`
- Result: the first returns only lines inside the generator itself (4.2); the second returns only `scripts/generate-docs-bundle.mjs:107`, the main-module guard.

**A5. An upgrade-diff or "newer template available" surface.**
- Search: `git grep -n -i -E 'newer (template|version)|template (update|upgrade)|upgrade (available|template)|outdated template|新版模板|模板更新|模板升级|有新版本' -- packages/views packages/core apps/web apps/desktop server/internal ':!server/internal/docs'`
- Result: collaboration-stage, issue-field and project-conflict strings; no template surface (6.2).

**A6. A comparison of a stored row's template version with the registry version in server code.**
- Search: `git grep -n -E 'TemplateVersion *(<|>|!=|==)|\.Version *(<|>|!=) *(agent|squad|existing)\.' -- server ':!*_test.go' ':!server/pkg/db/generated'`
- Result: nil checks in `server/internal/handler/workspace_mcp_api.go` and two autopilot report-snapshot lines (6.2). Outside server code, the operator-run script `scripts/localize_agent_instructions.py:126-128` compares a row's `template_version` with versions parsed from the roster source (6.2).

**A7. A migration that rewrites agent or squad instructions or template versions.**
- Search: `git grep -n -l 'template_version' -- server/migrations`; `git grep -n -E 'template_key|template_version' -- server/migrations`
- Result: migrations 450 (agent), 451 (squad), 455 (autopilot) and 460 (workspace MCP); outside those four, one line, `server/migrations/512_workspace_mcp_template_source.up.sql:5`, on the workspace-MCP table (6.1).

**A8. A test that staffs a squad, or adopts one for a project, over an existing agent or squad row that carries an older template version.**
- Search: `git grep -n -E 'SET template_version|template_version *= *[0-9]' -- 'server/internal/handler/*_test.go' 'server/cmd/server/*_test.go'`
- Result: only `server/cmd/server/agent_artifact_integration_test.go:247`, a task-capability fixture (6.1).

**A9. Go tests that pin the version of `product-analyst`, `implementer` or `qa-engineer`, or the registry version of `multica-test-report` or `multica-requirement-clarification`.**
- Search: `git grep -n -E '"(qa-engineer|implementer|product-analyst)"' -- 'server/*_test.go'` and `git grep -n -E 'multica-test-report|multica-requirement-clarification' -- 'server/*_test.go'`, each filtered for `version` or a trailing number; `git grep -n -E '(\.Version|TemplateVersion|template_version) *(!=|==) *[0-9]' -- 'server/internal/service/*_test.go' 'server/internal/handler/*_test.go'`
- Result: the first two return nothing after the filter; the third lists the pins recorded in 6.3, none for those keys or names. The stored version 1 of `product-analyst` is pinned in e2e only (`e2e/agent-autonomy-gates.spec.ts:259`).

**A10. Tests that pin `config.origin.version` for `multica-security-review`, `multica-code-review`, `multica-test-report` or `multica-requirement-clarification`.**
- Search: `git grep -n -i -E "origin.{0,40}version" -- 'server/*_test.go'`; the same pattern over `e2e packages/views packages/core`
- Result: for built-in role skills the Go matches are `server/internal/handler/agent_template_test.go:162,164,174,243,245,250,251,411-414` only, which cover `multica-migration-review`, `multica-architecture-decision-record`, `multica-debugging` and `multica-progress-report` (6.4). The remaining Go matches are unrelated identifiers or non-built-in origins (`server/internal/handler/collaboration_skill_invalidation_test.go:85`, `server/internal/service/collaboration_skill_invalidation_test.go:87`, `server/internal/migrations/collaboration_skill_invalidation_test.go:62`). The TypeScript matches for built-in role skills are fixtures with a literal `version: 1` (6.4).

**A11. Hard-coding of this squad under `apps/web`, `apps/desktop`, `apps/mobile`, `packages/core` or `packages/ui`; role-key or role-skill-origin literals under `apps/`.**
- Search: `git grep -l -i -E 'security-remediation|security_remediation|Security Remediation|安全漏洞修复' -- . ':!.trellis/tasks'`; `git grep -n -l -E 'migration-reviewer|product-analyst|security-reviewer|builtin_role_skill' -- apps/mobile apps/web apps/desktop`
- Result: the first lists 32 tracked files, none under those five directories (section 3 preamble); the second is empty.

**A12. A squad-side template-version string in the UI.**
- Search: `git grep -n -i -E '"[^"]*version[^"]*":|版本' -- packages/views/locales/en/squads.json packages/views/locales/zh-Hans/squads.json`
- Result: only `packages/views/locales/zh-Hans/squads.json:182`, the release-squad summary (3.1).

**A13. A product deep link into the squad sections of the docs.**
- Search: `git grep -n -E 'squads|security' -- packages/core/docs/slugs.ts`
- Result: empty (3.1, 4.3).

**A14. A template field or request field that seats a human; a human `squad_member` row written by template materialization; a test that adds a human to a template-staffed squad.**
- Search: `git grep -n -i -E 'human|member_type|MemberType|"member"' -- server/internal/service/builtin_squad_templates.go`; `git grep -n -E 'MemberType' -- server/internal/handler/squad_template.go`; `grep -n DisallowUnknownFields server/internal/handler/squad_template.go`; `git grep -n -E '"member"' -- server/internal/handler/squad_template_test.go server/internal/handler/squad_template_security_test.go server/internal/handler/squad_template_access_test.go server/internal/handler/project_execution_squads_test.go`
- Result: the first matches only a comment and description / seat-role strings (`:16,244,326,371,396,408`); the second matches `:448` and `:467`, both `MemberType: "agent"`; the third is empty; the fourth returns only `server/internal/handler/squad_template_access_test.go:25,27,31,42`, where `"member"` is a workspace role or an invocation-target type (7.1-7.3).

**A15. A `security-remediation` entry in the quick-create squad scenarios.**
- Search: `git grep -n 'security-remediation' -- packages/views/modals`
- Result: empty; the literal list is `packages/views/modals/quick-create-scenario.ts:21-30` (3.1).

**A16. A count of role skills in `skills.mdx` / `skills.zh.mdx`; counts of roles, role skills or squads in `agents.mdx` / `agents.zh.mdx`.**
- Search: `grep -n -i -E 'fourteen|fifteen|十四|十五|14 个|15 个|role-skill templates|个角色技能' apps/docs/content/docs/skills.mdx apps/docs/content/docs/skills.zh.mdx`; `grep -n -i -E 'fourteen|fifteen|twenty-three|十四|十五|14 个|15 个|23 个|九个|9 个' apps/docs/content/docs/agents.mdx apps/docs/content/docs/agents.zh.mdx`
- Result: both empty. The counts found in docs sources are on `agents-create.{mdx,zh.mdx}` and `squads.{mdx,zh.mdx}` (3.3).

**A17. A consumer of `artifacts/qa/collaboration-multi-account-20261109/environment/source-hashes.json`.**
- Search: `git grep -l 'source-hashes' -- . ':!.trellis/tasks'`
- Result: the artifact's own `README.md` and `.trellis/workspace/artisan/journal-1.md`; no test or script (3.6).

**A18. A test that requires every template `AvatarEmoji` to be a key of the `templateIconAvatar` map.**
- Search: `git grep -n -E 'templateIconAvatar|templateIconByEmoji|agent_avatar' -- 'server/*_test.go'`
- Result: `server/internal/handler/squad_template_test.go:166`, which compares the stored avatar with the output of the same function, and `server/internal/migrations/builtin_agent_avatar_migration_test.go:64,89,96,107` (migration 459). See 2.5.

**A19. A pin of "fourteen", "nine", "seven" or this squad's key in the product-skill string test.**
- Search: `grep -n -i -E 'fourteen|nine|seven|security-remediation' server/internal/service/builtin_skills_test.go`
- Result: empty (3.5).

**A20. An automated entry point that runs the Python test of the instruction-localization script; a test of that script's roster parsing.**
- Search: `git grep -n -E 'python3? +-m +(unittest|pytest)|pytest ' -- .github Makefile package.json 'scripts/*.sh'`; `git grep -n -l -E 'test_localize_agent_instructions'`; `grep -n -E 'catalog_from_git|builtin_agent_templates|builtin_squad_templates' scripts/test_localize_agent_instructions.py`
- Result: the first is empty; the second lists `scripts/localize-agent-instructions.md` (the manual command at `:76`) plus source-hash manifests under `.trellis/tasks/archive/` and `artifacts/qa/`; the third is empty (the test builds literal catalogs at `scripts/test_localize_agent_instructions.py:74,189`).

**A21. This squad or its lead in the lifecycle hand-off code and fixtures.**
- Search: `git grep -n -E 'security-remediation|migration-reviewer|security-reviewer|diagnostician|code-reviewer|qa-engineer|implementer|product-analyst' -- 'server/internal/service/lifecycle_handoff*' server/internal/service/testdata/lifecycle`
- Result: two lines, neither naming the squad — `server/internal/service/lifecycle_handoff.go:51` (a comment mentioning the diagnostician) and `server/internal/service/testdata/lifecycle/lifecycle_handoff_fixtures.json:73` (`"owner": "implementer"`).

**A22. This squad or its lead in the roster files at the instruction-localization script's documented baseline commit.**
- Search: `git show 2a1b56735:server/internal/service/builtin_squad_templates.go | grep -n -E 'Key: +"security-remediation"|Key: +"security-remediation-lead"'`; `git show 2a1b56735:server/internal/service/builtin_agent_templates_roster.go | grep -n -E 'security-remediation-lead'`
- Result: both empty; both files exist at that commit (`git cat-file -e` exits 0 for each). See 6.2.

### B. Stated in sections 1-7 but not confirmed

- **B1. Every "breaks" / "goes stale" entry in sections 1, 2.5, 3 and 6** is a reading of the assertion or literal at the cited line. No test suite, build, e2e run, docs generation or server process was executed, so none of those entries is an observed failure.
- **B2. An unknown JSON key on `POST /api/squads/from-template` is ignored, not rejected.** Inferred from `json.NewDecoder(r.Body).Decode(&req)` at `server/internal/handler/squad_template.go:149` and the absence of `DisallowUnknownFields` in that file (A14). No test sends such a key; no request was made.
- **B3. Consequences (a) and (b) in 6.1** — a newly staffed squad row led by an existing agent row created from the older lead template, and project adoption returning an existing squad row that carries the older squad version — follow from `server/internal/handler/squad_template.go:425-438,486-511`, `server/pkg/db/queries/agent.sql:41-54`, `server/internal/handler/project_execution_squad.go:552-576` and `server/pkg/db/queries/squad.sql:43-48`. No covering test was found (A8) and nothing was run. Consequence (c) is exercised on the single-role path only (`server/internal/handler/agent_template_test.go:171-176,183-205`); `server/internal/handler/squad_template_security_test.go:118-120,160-165` covers an edited skill on the squad path without changing its origin version.
- **B4. `TaskCapabilityProfile` keeps a role's profile after a version bump.** Inferred from the `>=` comparisons at `server/internal/auth/task_capability.go:26,30,34`. The test table at `server/internal/auth/task_capability_test.go:20` covers `progress-reporter` 1 and 2, `code-reviewer` 1 and 2, `security-reviewer` 2, `migration-reviewer` 2 and 3, and `custom` 999; no higher version of a named key is in the table.
- **B5. The 98th file in "98 个生成文件" is `manifest.json`.** Inferred from 39 pages + 58 assets = 97 and the generator writing one manifest (`scripts/generate-docs-bundle.mjs:38-48`); the cited records give only the total (4.4).
- **B6. How the instruction-localization script treats this squad's rows** (6.2). Read from `scripts/localize_agent_instructions.py:118-128,316-329`; the script and its unit test were not run, and the roster parsing has no test (A20).
- **B7. Line numbers in four files with uncommitted changes** (`packages/views/locales/en/modals.json`, `packages/views/locales/zh-Hans/modals.json`, `server/internal/service/delivery_completion_authorization.go`, `server/pkg/db/queries/agent.sql`) are working-tree numbers (section 0). HEAD numbers were recorded only for `delivery_completion_authorization.go` (6.2); `agent.sql:41-54` precedes that file's uncommitted change; the two `modals.json` anchors (2.5) have no HEAD counterpart recorded.

### C. Exists, not read

- **C1.** File lists of the commits in 2.3 (`cd95c8e40`, `028e93bb0`, `5701a7695`, `372576209`, `7fdbf274e`, `119478843`, `ed0fa7789`, `e2d3f8891`); only hash and date are recorded.
- **C2.** `scripts/localize_agent_instructions.py` outside `:1-40`, `:100-140`, `:270-330`; `scripts/test_localize_agent_instructions.py` outside `:30-31`, `:74`, `:93-110`, `:186`, `:189`; `scripts/skill-eval/README.md` beyond `:26` (`--skills server/internal/service/builtin_role_skills`).
- **C3.** `server/internal/service/lifecycle_handoff.go` and `server/internal/service/testdata/lifecycle/lifecycle_handoff_fixtures.json` beyond the two lines in A21.
- **C4.** The bodies of the seven seated roles' `INSTRUCTIONS.md` and of their six role skills' `SKILL.md`. A name search over the embedded directories (`git grep -l -i -E 'security-remediation|安全漏洞修复' -- server/internal/service/builtin_agent_templates server/internal/service/builtin_role_skills server/internal/service/builtin_squad_templates server/internal/service/builtin_skills`) lists four files: the lead's and the squad's `INSTRUCTIONS.md` and the two `multica-squads` product-skill files (3.5). Wording that the squad's prompts share with those bodies without naming the squad was not compared.
- **C5.** `.trellis/spec/views/frontend/builtin-skill-localization.md` — it appears in the precedent commits' file lists (2.2); no line anchors were taken from it.
- **C6.** Autonomy-ceiling behaviour for an Operator-level seat beyond rows 20 and 32 of 1.1.
- **C7.** `apps/mobile` beyond the two name sweeps in A11.
- **C8.** How agents act on the v1 prompt text (real-model behaviour); no transcript, run or evaluation was examined.
