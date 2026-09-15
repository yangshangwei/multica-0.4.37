# Creating agents — source map

Evidence layer for `SKILL.md`. Every contract maps to `file:line` on the
current tree, the runtime effect, and a safe read-only check. Line numbers were
re-derived against this tree — re-derive again if the files move, the
surrounding context (not the number) is the anchor.

## Verification

Instruction update preconditions are implemented in
`server/internal/handler/instructions_precondition.go`, the `UpdateAgent`
handler, and `server/pkg/db/queries/agent.sql` (`UpdateAgent`). Paired
`expected_instructions`/`expected_updated_at` fields compare workspace, full
original body and timestamp atomically. `GetAgent`/`UpdateAgent` expose the
capability header and preserve timestamp precision. Regression evidence is in
`server/internal/handler/instructions_precondition_test.go`; 409 responses do
not publish an update. Older replicas do not enforce the optional fields.

```bash
# Conformance eval for this skill (and the shared template invariants):
go test ./internal/service -run TestCreatingAgentsSkillCoversAgentCreationContracts
go test ./internal/service -run TestBuiltinSkillsConformToTemplate
```

## CLI entry points — `server/cmd/multica/cmd_agent.go`

| Contract | Line | Behavior | Safe check |
|---|---|---|---|
| Create flags: `name`, `description`, `instructions`, `runtime-id` | 160–163 | Registered create flags; `name`/`runtime-id` enforced in `runAgentCreate` | `multica agent create --help` |
| `runtime-config`, `model`, `thinking-level`, `service-tier`, `custom-args` flags | 164–168 | `model` help: "Prefer this over passing --model in --custom-args"; thinking and Codex service-tier values are thin catalog-owned pass-throughs, with exact model compatibility checked by the daemon; empty = runtime default | `multica agent create --help` |
| Secret-safe env input: `custom-env`, `custom-env-stdin`, `custom-env-file` | 169–171 | `--custom-env` warns about shell history / `ps`; stdin and file modes keep secrets off the command line; mutually exclusive | `multica agent create --help` |
| Secret-safe MCP input: `mcp-config`, `mcp-config-stdin`, `mcp-config-file` (create) | 172–174 | Same three-channel pattern as `custom-env`; `--mcp-config` warns about shell history / `ps`; value must be a JSON object or `null` | `multica agent create --help` |
| MCP flags on `agent update` | 200–202 | Same three channels on update; `--mcp-config null` clears. Unlike `custom_env`, `mcp_config` IS settable via update | `multica agent update --help` |
| `thinking-level` / `service-tier` flags on `agent update` | 189–190 | Thin pass-throughs; an explicit empty string clears the saved override and restores the runtime/local Codex default | `multica agent update --help` |
| `max-concurrent-tasks` flags + validation | `cmd_agent.go` 179, 208; `cmd_agent_validation.go` 5–20 | Shared CLI helper enforces 1–50; create/update call it before their HTTP mutation and omitted create flags stay absent | `multica agent create --help`; `multica agent update --help` |
| `runAgentCreate` builds body + `POST /api/agents` | 533–628 | Only sets a body key when the flag `Changed`; validates `max_concurrent_tasks` at 605–611, then posts to `/api/agents` (617) | read 533–628 |
| Body assembly: description/instructions/runtime-config/custom-args/custom-env/mcp-config/model/thinking-level/service-tier | 548–611 | `model`, `thinking_level`, and `service_tier` are `Changed`-gated pass-throughs; omitted flags are not sent | read the `runAgentCreate` body assembly |
| `runAgentUpdate` sends `thinking_level` / `service_tier` / `mcp_config` | 630–725 | Each override key is added only when its flag is `Changed`; `max_concurrent_tasks` is range-checked at 693–699; `custom_env` is intentionally not a flag here | read the `runAgentUpdate` body assembly |
| `parseMcpConfig` / `resolveMcpConfig` helpers | 1216, 1244 | Validator (object-or-`null`, content-free errors) + three-channel resolver, mirroring `parseCustomEnv`/`resolveCustomEnv` | read 1216–1301 |
| `agent skills set` = replace-all | 922 | `PUT /api/agents/{id}/skills` (940); `--skill-ids ''` clears all (928–931) | `multica agent skills set --help` |
| `agent skills add` = additive | 947 | `POST /api/agents/{id}/skills/add` (968); requires ≥1 id (953–958) | `multica agent skills add --help` |
| `agent skills list` | 890 | reads bindings, no side effect | `multica agent skills list --help` |
| `agent env get` | 1024 | `GET /api/agents/{id}/env` (1034) | `multica agent env get --help` |
| `agent env set` | 1059 | `PUT /api/agents/{id}/env` with full `custom_env` map (1079) | `multica agent env set --help` |

## Role templates — `server/internal/service/builtin_agent_templates*.go`, `server/internal/handler/agent_template.go`

| Contract | Location | Behavior | Safe check |
|---|---|---|---|
| Roster of nine listed role templates | `builtin_agent_templates_roster.go` `builtinAgentRoleTemplates` | Product-ordered slice; `Listed: false` marks squad-leader definitions, which `AgentRoleTemplates()` filters out of the picker | `curl` / UI: `GET /api/agents/templates` |
| Progress Reporter defaults | `builtin_agent_templates_roster.go` `progress-reporter`; `builtin_role_skills/multica-progress-report/SKILL.md` | Contributor, concurrency one, one reporting skill; business sources read-only by instruction, only assigned report review closeout permitted after comment delivery | `agent_template_test.go` reporter creation and preserved-copy regressions |
| Role prompt copied, not layered | `builtin_agent_templates.go` `AgentRoleTemplate.Instructions` + `agent_template.go` `CreateAgentFromTemplate` | The embedded INSTRUCTIONS.md text is written into `agent.instructions` at create, so a release never overwrites a workspace's edit (the opposite trade-off from Mika in `builtin_agents.go`) | `multica agent get <id> --output json` — `instructions` holds the full role text |
| Default name follows the request language | `agent_template.go` `CreateAgentFromTemplate` + `agentRoleTemplateToResponse` | An omitted `name` falls back to `AgentRoleTemplate.Title(language)`, so `language=zh` stores 产品分析师; `DefaultName` remains the English fallback and the list response's `name` carries the same localized string the create would store | `GET /api/agents/templates?language=zh` then create without `name` |
| Provenance is server-set | `agent.go` `agentTemplateProvenance` + `CreateAgent` | `template_key`, `template_version`, `autonomy_level` are arguments to the shared create path, not fields of `CreateAgentRequest`; the public create cannot set them | `POST /api/agents` with `template_key` → response has an empty `template_key` |
| Template create reuses the ordinary create path | `agent.go` `createAgentFromRequest` | Runtime access, thinking/service-tier validation, name-conflict 409 and invocation permission all apply unchanged | read `createAgentFromRequest` |
| Declared agents cannot staff above their own level | `agent_template.go` `CreateAgentFromTemplate` + `agent_autonomy.go` `requireAgentMayGrantAutonomy` | Template staffing requires Coordinator, then caps the new role at the caller's declared level; human and undeclared callers preserve existing behavior | `e2e/agent-autonomy-gates.spec.ts` Coordinator ceiling cases |
| Role skills are materialized, never overwritten | `agent_template.go` `materializeRoleSkillsInTx` | Reuse-or-create keyed on skill name; there is no update path, so an edited workspace skill is bound as it stands. `config.origin.type = builtin_role_skill`, which `refreshableOriginSource` does not recognise — "update from source" cannot refetch it | `multica skill list --output json`; the row's `config.origin` |
| Concurrent materialization is lock-serialized | `agent_template.go` `lockRoleSkillMaterialization` | Advisory lock per workspace, because two templates can name one role skill and a unique-violation inside a transaction cannot be recovered from | read the lock helper |

## Autonomy enforcement — `server/internal/handler/agent_autonomy.go`

| Contract | Location | Behavior | Safe check |
|---|---|---|---|
| Applies only to agent actors | `agent_autonomy.go` `agentActorAutonomy` | Resolves the acting agent through `resolveActor`; a human request returns early and is never gated | read `requireAgentAutonomy` |
| An undeclared level passes everything | `builtin_agent_templates.go` `AutonomyAtLeast` | Empty or unrecognised level returns true for every requirement, so agents that predate role templates are unaffected | `go test ./internal/service -run TestAutonomyAtLeast_UndeclaredPasses` |
| Observer cannot move work | `issue.go` `UpdateIssue`, `BatchUpdateIssues`, `CreateIssue` | Status/assignee change and issue create require `contributor`; the check runs before the write. Both write routes share one predicate, `agent_autonomy.go` `requestTouchesIssueDirection`, keyed on FIELD PRESENCE in the raw body — an explicitly null `assignee_id` unassigns and decodes to a nil pointer, so a pointer-based gate would miss it | `go test ./internal/handler -run 'TestUpdateIssue_ObserverAgentCannotChangeStatus|TestBatchUpdateIssues_ObserverAgentCannotChangeStatus'` |
| A person is never gated on either route | `issue.go` both handlers | The gate returns early for a non-agent actor, so nothing about human issue editing changed | `go test ./internal/handler -run TestBatchUpdateIssues_HumanIsUnaffectedByAgentPolicy` |
| Automation and squads need coordinator | `autopilot.go` `CreateAutopilot`/`UpdateAutopilot`, `squad.go` `CreateSquad` | Standing automation and routing wiring are coordination decisions | `go test ./internal/handler -run TestCreateAutopilot_ContributorAgentCannotCreate` |
| An agent cannot raise its own ceiling | `agent.go` `UpdateAgent` | `autonomy_level` writes are rejected for machine credentials, because a task token carries the OWNER's user id and would pass `canManageAgent` | `go test ./internal/handler -run TestUpdateAgent_AgentActorCannotRaiseItsOwnAutonomy` |
| A machine credential cannot mint a human one | `router.go` `/api/cli-token` + the `/api/tokens` group under `RequireHumanActor`; `auth.go` `IssueCliToken` and `personal_access_token.go` `CreatePersonalAccessToken` repeat the check | Closes the laundering chain: a task token (or `mcn_` cloud PAT) gets 403 from the JWT exchange and from PAT create, list, renew and revoke, so it cannot re-enter as a human caller. The daemon's `mul_` PAT renewal and the browser-JWT CLI login are unaffected | `go test ./cmd/server -run TestMachineCredential` |
| The prompt and the enforcement agree | `builtin_agent_autonomy.go` `AutonomyBriefing` + `daemon.go` claim | The policy section is appended at claim from the binary, never stored on the row | `go test ./internal/handler -run TestClaim_InjectsAutonomyPolicyAfterTheAgentsOwnInstructions` |
| The prompt does not overclaim | `builtin_agent_autonomy.go` `autonomyObserverBody` | Names the covered issue, template, squad and automation operations; explicitly distinguishes these gates from process/credential isolation. Priority, parent and filesystem changes remain prompt-level restrictions | read the Observer body |

## Approval boundary — `server/internal/handler/agent_approval.go`, `server/cmd/multica/cmd_approval.go`

| Contract | Location | Behavior | Safe check |
|---|---|---|---|
| Only a person decides | `agent_approval.go` `DecideAgentApproval` + router `RequireHumanActor` | Checked twice — middleware and handler — because this is the single gate the mechanism rests on | `go test ./internal/handler -run TestDecideAgentApproval_AgentCannotDecide` |
| Execution requires approval AND operator | `agent_approval.go` `RecordAgentApprovalExecution` | `MarkAgentApprovalRequestExecuted` matches only `status='approved'`; the level check rejects a contributor holding an approval | `go test ./internal/handler -run TestRecordExecution` |
| Identity comes from the token | `agent_approval.go` `CreateAgentApproval` | An agent actor's `agent_id` is taken from the task token, never from the body | `go test ./internal/handler -run TestCreateAgentApproval_AgentIdentityComesFromTheToken` |
| CLI has no approve command | `cmd_approval.go` | `request`, `list`, `get`, `executed`, `cancel` only — an approve command would fail by design for every caller holding a task token | `multica approval --help` |

## Copy command — `server/cmd/multica/cmd_agent_copy.go`

| Contract | Line | Behavior | Safe check |
|---|---|---|---|
| `agentCopyCmd` (`copy <source-agent-id>`) + flag registrar | 21, 47, 54 | Own file with its own `init()` so `cmd_agent.go` line refs stay stable; `registerAgentCopyFlags` is shared with the tests | `multica agent copy --help` |
| Reads source via `GET /api/agents/<id>` | 95 | Composes over existing endpoints — no dedicated copy API | read `runAgentCopy` |
| Conversation starters copied without an override flag | `runAgentCopy` conversation-starter body assembly | Copies `conversation_starters` when the source response contains an array; `registerAgentCopyFlags` intentionally exposes no conversation-starter override | `multica agent copy --help` |
| Same-runtime vs cross-runtime rule | 114, 187 | `sameRuntime` copies `model`/`thinking_level`/`service_tier`; a different `--runtime-id` drops them and requires `--model` (empty allowed) | `multica agent copy --help` |
| Concurrency copy compatibility | `runAgentCopy`, `copiedAgentMaxConcurrentTasks` | Explicit `--max-concurrent-tasks` is validated before any request; valid source values are copied, while historical values outside 1–50 are omitted so create defaults to 6 | read the concurrency body assembly |
| Skills copied in the create transaction | 239 | Source skill ids sent as `skill_ids`, bound in the same `POST /api/agents` tx (267); `--no-skills` opts out | read `runAgentCopy` |
| Secrets never copied | 240–266 | `custom_env`/`mcp_config`/`runtime_config` set only from explicit secret-safe flags, never read from the source | `multica agent copy --help` |

## Create handler — `server/internal/handler/agent.go`

| Contract | Line | Behavior |
|---|---|---|
| `maxAgentDescriptionLength = 255` | 31 | Cap is 255 **Unicode code points** (comment: counted via `utf8.RuneCountInString`, matches Postgres `char_length`) |
| `AgentResponse` omits plaintext `custom_env` | 33–53 | Exposes only `has_custom_env` (52) and `custom_env_key_count` (53); comment cites MUL-2600 |
| `CreateAgentRequest` fields | 930–970 | Includes `model`, `thinking_level`, and Codex `service_tier` alongside the profile/runtime/permission inputs |
| Conversation-starter request and validation | `AgentConversationStarter`, `normaliseAgentConversationStarters`, create/update paths | `conversation_starters` is trimmed and validated as at most three complete label/prompt pairs before JSONB persistence; omission defaults to `[]`, update omission preserves, and `[]` clears |
| `name` required | 623–625 | 400 "name is required" |
| `description` ≤ 255 code points | 627–629 | `utf8.RuneCountInString(req.Description) > maxAgentDescriptionLength` → 400 |
| `runtime_id` required | 631–633 | `if req.RuntimeID == ""` → 400 "runtime_id is required" |
| `runtime_id` must resolve in workspace | 642–658 | parsed + `GetAgentRuntimeForWorkspace`; unknown → 400 "invalid runtime_id" |
| `thinking_level` provider-level validation | `agent.go` create/update paths | `!agent.IsKnownThinkingValue(runtime.Provider, req.ThinkingLevel)` → 400; fixed-vocabulary providers use an enum (Pi: `off|minimal|low|medium|high|xhigh|max`), Codex/OpenCode use safe-token syntax, and per-model gaps are deferred to daemon (MUL-2339) |
| `thinking_level` rejection copy | `agent.go` `thinkingLevelRejection` / `existingThinkingLevelRejection` | Splits "runtime has no reasoning control" from "unrecognised token" so a runtime-capability 400 does not read as a typo; both carry-over branches point at `thinking_level=""` (MUL-5770) |
| `service_tier` provider-level validation | `agent.go` create/update paths | Non-empty values are Codex-only safe tokens; exact per-model support is daemon-owned |
| Defaults: `{}` config/env, `[]` args | 688–701 | `RuntimeConfig`→`{}`, `CustomEnv`→`{}`, `CustomArgs`→`[]` when nil, before insert |
| `visibility` default | 635–636 | `if req.Visibility == "" { req.Visibility = "private" }` — access-control field, not the runtime prompt |
| `max_concurrent_tasks` create/default validation | `agent.go`; `agent_validation.go`; `internal/agentconfig/concurrency.go` | Shared 1–50 validator; a missing or explicit `null` field defaults to 6, while an explicitly supplied numeric 0/out-of-range value returns 400 |
| `max_concurrent_tasks` update validation | 1660–1666 | Omission preserves the existing value; a supplied value outside 1–50 returns 400 before persistence |
| `mcp_config` null-skip on create | 704–705 | raw JSON copied through unless the body value is the literal `null` |
| `mcp_config` redacted on read | 54, 848–851 | `redactMcpConfig` sets `McpConfigRedacted=true`; a private agent read by a member also redacts (494, 509) |
| Qwen Code managed-MCP injection | `pkg/agent/qwen.go` | Non-null `mcp_config` is written to a daemon-owned 0600 temporary JSON file and passed with `--mcp-config`; the file is removed after the process exits, while `null` preserves native inheritance. |
| Assigned workspace MCP servers folded into the agent's | `internal/handler/workspace_mcp.go` `ResolveAgentMcpConfig`; applied in `internal/handler/daemon.go` `buildClaimedTaskResponse` | Only servers bound to this agent AND enabled are folded in; union by name with the agent's own winning; both containers normalized onto `mcpServers`; read on every claim, so an assignment or toggle lands on the agent's next task |
| Workspace MCP library + assignment API | `internal/handler/workspace_mcp_api.go` | `GET /api/workspaces/{id}/mcp-servers` returns name / transport only, never the entry, for any role; `POST`/`PUT`/`DELETE` on the library are owner/admin; `GET`/`POST`/`PUT .../enabled`/`DELETE /api/agents/{id}/mcp-servers` manage one agent's assignments and admit the agent owner or a workspace owner/admin. Every write refuses agent actors. Deleting a library entry sweeps its bindings in the same transaction (no FK) |
| Effective-set regression guard | `internal/daemon/runtime_mcp_workspace_test.go` | Runs resolve -> `mergeRuntimeAndAgentMcpConfig` for OpenCode; catches a resolver that emits a container the daemon merge would not read |
| Random emoji avatar default | `agent_avatar.go` 11–32; `agent.go` 1127–1133 | Omitted, empty, or whitespace-only `avatar_url` becomes a cryptographically selected `emoji:<glyph>` sentinel; explicit values are preserved. |
| `CreateAgent` insert params | `agent.go` create path | Persists avatar_url, runtime_config, instructions, conversation_starters, custom_env, custom_args, model, thinking_level, service_tier, mcp_config, visibility, max_concurrent_tasks |
| `UpdateAgent` rejects `custom_env` | 910–913 | if `custom_env` present in body → 400 "use PUT /api/agents/{id}/env (or `multica agent env set`)" |
| `UpdateAgent` persists / clears `mcp_config` | 944–948, 1060–1061 | Tri-state from the raw body: key omitted → no change; literal `null` → `ClearAgentMcpConfig`; object → replace. No 400 like `custom_env` — `mcp_config` IS updatable here |
| `description` ≤ 255 on update too | 921–924 | same cap re-checked on update |

## Runtime model/thinking discovery — `server/pkg/agent/{models,thinking}.go`

| Contract | Line | Behavior |
|---|---|---|
| Codex model-list entry point | `models.go` 94–103 | `ListModels("codex")` uses cached daemon-local discovery instead of returning the fallback catalog unconditionally |
| Codex fallback catalog | `models.go` 301–354 | Used for Codex <0.122.0 and failed/malformed discovery; includes current verified visible models plus legacy `gpt-5.3-codex`, with a separate `Thinking` catalog on every model |
| Codex discovery version gate | `thinking.go` 280, 306–337 | `codex debug models --bundled` is used only for parseable versions ≥0.122.0; unsupported versions and command/parse/empty failures return the static model + thinking fallback |
| Codex catalog projection | `thinking.go` `parseCodexModelCatalog` | Hidden models are excluded; visible model, reasoning, and `service_tiers` metadata are preserved |
| Pi RPC model/thinking discovery | `models.go` `discoverPiModelsRPC` / `piThinkingFromRPCModel` | Starts an ephemeral `pi --mode rpc --no-session` process, requests `get_state` + `get_available_models`, preserves extension-registered providers, marks the current model as Default, and mirrors Pi's exact per-model `reasoning` / `thinkingLevelMap` rules. Older/forked Pi falls back to `--list-models` with no guessed thinking catalog. |
| Pi invocation effort | `pi.go` `buildPiArgs` / `piBlockedArgs` | A non-empty persisted value becomes `--thinking <level>`; custom `--thinking` flags are filtered so the first-class field is the sole owner. |
| Per-model thinking validation | `thinking.go` `ValidateThinkingLevel` | Accepts only values in the explicit model's `Thinking.SupportedLevels`; Pi empty-model validation resolves to the RPC current-model Default, while an empty Codex model fails closed because its effective `config.toml` model is unknown. |
| Dynamic Codex token gate | `thinking.go` `IsKnownThinkingValue` | Server persistence accepts syntactically safe Codex/OpenCode/Kimi and ACP-catalog tokens so new catalog values do not require a Multica release; Pi instead uses its fixed seven-token CLI vocabulary. Exact support remains a daemon-local per-model check. |
| Runtime reasoning capability | `thinking.go` `ThinkingControlSupported` | True for the fixed-enum providers (including Pi), the dynamic-catalog providers (Codex/OpenCode/Kimi), and the ACP-catalog providers in `acpCatalogThinkingProviders`. False for runtimes with no effort dial on the surface the daemon drives — Copilot discovers over ACP but executes through its own CLI, so no live ACP session exists to carry an effort. Provider granularity only: `hermes` reports true because jcode applies an effort, while Hermes Agent under the same provider advertises none and gets no picker — the per-session catalog settles it |
| Generic ACP effort catalog | `acp_effort.go` `parseACPEffortOption` / `annotateACPThinkingForSessionModel` | One provider-neutral parser reads the effort selector out of any ACP `session/new` response (option id or category `effort` / `thought_level`), taking the advertised values verbatim. CodeBuddy keeps a flag whitelist on top because it applies the level via `--effort`, not over ACP |
| ACP effort catalogs are per model, not per session | `acp_effort.go` `annotateACPThinkingForSessionModel` | Only the model marked `Default` (the advertised `currentModelId`) is annotated. ACP options may depend on each other, and reasonix v1.21.5 derives the catalog from the current model's provider entry — `deepseek-v4-flash` advertises `low`, `deepseek-v4-pro` does not, and some models expose no effort at all. Copying one catalog across models would offer levels the runtime then refuses. Other models keep `Thinking=nil` until per-model probing exists |
| ACP effort application | `acp_effort.go` `applyACPEffortOption` | Sends `session/set_config_option` with the id the session advertised, then re-reads `currentValue`. The read-back proves only that the session reports the new value — not that the runtime threaded it into its provider request; it catches Kimi ≤0.28.1 confirming `on` after being set to `max`. Failures warn and let the prompt through rather than failing the task. `stateIsCurrent=false` after a model switch skips the local vocabulary check, because the advertised list then describes the previous model |
| ACP capability is per runtime, not per provider | `agent.go` `acpThinkingDecision` / `ambiguousACPEffortProviders` | Three-state, consulted on create and both update branches. Catalog advertises an effort → allow; catalog advertises none → capability 400; **no catalog yet** → 400 for ambiguous providers only (`hermes`), because the cache is written solely by `ReportModelListResult`, so a CLI-only caller stays undiscovered indefinitely and "unknown" must not be read as "supported". `reasonix` is unambiguous and stays allowed while undiscovered |
| ACP effort opt-in | `thinking.go` `acpCatalogThinkingProviders` | A runtime joins only when its `Execute` calls `applyACPEffortOption` AND someone has confirmed from source or a real run that it acts on the setting — currently `reasonix` and `hermes` (GitHub #6720). This list, not the read-back, is where that verification is recorded. `hermes` is the case that shows why the catalog rather than a version gate decides: jcode advertises `reasoning_effort` and threads it into the provider request (verified v0.71.1/v0.73.0), Hermes Agent advertises no configOptions at all (re-verified v0.20.0, 2026-08-11), and both run under the same provider type. Speaking ACP is not sufficient: Copilot discovers over ACP but executes through its own CLI, so a catalog there would render a picker with nothing behind it |
| Per-model service-tier validation | `thinking.go` `ValidateServiceTier` | Accepts only a tier advertised for the explicit Codex model; empty model fails closed because config.toml is unknown |
| Daemon invalid-combination handling | `internal/daemon/daemon.go` 3860–3892 | Before execution, invalid `(provider, model, thinking_level)` combinations log a warning and omit the override rather than failing the task |

## Env endpoint — `server/internal/handler/agent_env.go`

| Contract | Line | Behavior |
|---|---|---|
| `authorizeAgentEnv` gate | 76 | loads agent, then applies the two checks below |
| Agent actors denied | 90–94 | `if actorType == "agent"` → 403 "agents may not access env management endpoints" (MUL-2600 impersonation guard); runs FIRST, so an agent is denied even when its backing human owns the target agent |
| Agent owner or ws owner/admin | 96–103 | `requireWorkspaceRole(..., "owner", "admin", "member")` then `canManageAgentEnv` → 403 otherwise (MUL-5438) |
| `canManageAgentEnv` predicate | 120 | workspace owner/admin, or `agent.owner_id == member.user_id`; a NULL `owner_id` never matches |

## Routes — `server/cmd/server/router.go`

| Contract | Line | Behavior |
|---|---|---|
| `GET /env` | 603 | `h.GetAgentEnv` (plaintext read, gated) |
| `PUT /env` | 604 | `h.UpdateAgentEnv` (full-map overwrite, gated) |

## Claim-time injection — `server/internal/handler/daemon.go`

| Contract | Line | Behavior |
|---|---|---|
| Fresh agent re-read on claim | 1109–1111 | `GetAgent(task.AgentID)` — claim uses persisted fields, not create output |
| Workspace skills FIRST | 1115 | `skills := h.TaskService.LoadAgentSkills(...)` |
| Built-ins appended | 1116 | `skills = append(skills, h.TaskService.BuiltinSkills()...)` |
| Runtime payload | `daemon.go` `TaskAgentData` | Carries `Instructions`, `Skills`, `CustomEnv`, `CustomArgs`, `Model`, `ThinkingLevel`, `ServiceTier`, and `McpConfig`; metadata-only fields remain absent |
| `custom_args` argv and safe launch log | `internal/daemon/daemon.go` `ExecOptions.CustomArgs`; `pkg/agent/launch.go` `Config.logAgentCommand` | Custom args normally reach the provider process argv. Launch logs preserve flag names but redact inline values and positional/value tokens; OS process-list exposure remains, so credentials belong in `custom_env`. |
| ZeroClaw agent alias pseudo-args | `pkg/agent/zeroclaw.go` `takeZeroclawAgentAlias` / `zeroclawBlockedArgs` | `--agent` and `--agent-alias` (separate or `=value`) are consumed from `custom_args` and sent as `session/new.agentAlias`; neither token reaches argv because `zeroclaw acp` rejects those CLI flags. Omit the selector for sole-agent auto-selection; use it when multiple agents exist without `[acp].default_agent`. |

## Skill loading — `server/internal/service/task.go`

| Contract | Line | Behavior |
|---|---|---|
| `LoadAgentSkills` | 1685 | `ListAgentSkills` + per-skill `ListSkillFiles` → content + supporting files for execution |

## Built-in skills — `server/internal/service/builtin_skills.go`

| Contract | Line | Behavior |
|---|---|---|
| `go:embed builtin_skills` | 10–11 | skills embedded at compile time |
| `loadBuiltinSkill` | 45 | reads `<name>/SKILL.md` (47) + walks sibling files into `Files` (56–68) |

## Persisted columns — `server/pkg/db/generated/agent.sql.go`

| Contract | Line | Behavior |
|---|---|---|
| `CreateAgent` INSERT | generated from `queries/agent.sql` | columns include `runtime_config, runtime_id, instructions, conversation_starters, custom_env, custom_args, mcp_config, model, thinking_level, service_tier` |
| `CreateAgentParams` | generated from `queries/agent.sql` | typed params include nullable `Model`, `ThinkingLevel`, and `ServiceTier` |
| `UpdateAgent` SET | generated from `queries/agent.sql` | COALESCE updates include model/thinking/service tier; dedicated clear queries restore each nullable override |
| `UpdateAgentCustomEnv` (called by the `UpdateAgentEnv` handler) | 2652 | `SET custom_env = $2` — the only write path for env values |
