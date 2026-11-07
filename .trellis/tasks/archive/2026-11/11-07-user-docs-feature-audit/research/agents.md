# Agent, squad, MCP, skill and automation documentation audit

Audit date: 2026-11-07. Read-only product inspection of the current working tree; no runtime or real-model execution. This report is the only file written by this audit agent.

## Findings requiring documentation changes

### 1. MCP needs an end-user guide and discoverable navigation

The current Chinese docs have isolated CLI commands (`cli.zh.mdx:271,277`), administrator template publishing (`self-host-quickstart.zh.mdx:326`), and brief agent/provider configuration notes. They do not explain the complete existing end-user workflow. There is no `mcp.zh.mdx` or MCP navigation entry.

Verified behavior and source evidence:

- The sidebar has **MCP** and **能力市场**: `packages/views/layout/app-sidebar.tsx:179`; labels in `packages/views/locales/zh-Hans/layout.json:17`. The library route is `/<workspace>/mcp`; marketplace MCP tab is `/<workspace>/marketplace?type=mcp`: `packages/core/paths/paths.ts:106`.
- The MCP page manages saved workspace configurations; templates are browsed in **能力市场 → MCP**, filtered by deployment/builtin source, category and search: `packages/views/mcp/mcp-page.tsx:5`, `packages/views/settings/components/mcp-tab.tsx:271`, `packages/views/mcp/mcp-market.tsx`.
- Creating a library entry does not assign it to any agent. Save and assign are separate operations. The guided dialog saves the configuration, then lets the user select agents; they can skip assignment and return later. It preserves successful assignments and provides retries for failed targets: `packages/views/mcp/mcp-setup-dialog.tsx:249,315,639`.
- An agent's MCP tab exposes its own configuration, assigned workspace services, and runtime-discovered services. **添加** opens a chooser of existing library entries and templates: `packages/views/agents/components/tabs/mcp-config-tab.tsx:92,323,411,470,539`, `packages/views/mcp/mcp-agent-discovery.tsx:156`.
- Human workspace owners/admins can create/change/delete library entries. Binding services to an agent requires its human owner or workspace owner/admin; agent actors cannot perform either management action: `server/internal/handler/workspace_mcp_api.go:127,455`.
- Library configuration is write-only for all roles: responses expose safe names/transports/template provenance, never URLs, commands, arguments, headers or environment values. Editing requires supplying the complete replacement configuration. Renaming is separate and preserves the saved configuration: `server/internal/handler/workspace_mcp_api.go:17`, `packages/views/settings/components/mcp-tab.tsx:44,136`.
- Assigned workspace services can be toggled/removed per agent. Agent-owned configuration overrides same-named bound workspace configuration: `server/internal/handler/workspace_mcp.go:25`; managed names also override same-named runtime services, with UI notices: `packages/views/agents/components/tabs/mcp-config-tab.tsx:121,644`.
- Saving a template does not install its program on the runtime machine. The current administrator guide already explains this in `self-host-quickstart.zh.mdx:365`; reuse that distinction in user steps. Intranet users need administrator-provided executable/dependencies/service addresses. Do not supply public npm commands as a required path.
- Supported providers are explicitly enumerated in `packages/core/agents/mcp-support.ts:7`. The current provider comparison matches that set. Keep a cross-link rather than duplicating the whole table.

Recommended new guide sections: choose entry point; save from template/custom configuration; explicitly assign to agents; per-agent enable/disable; runtime requirements and name collisions; permissions and secret display; partial failure/template change troubleshooting. Link from `agents-create`, `providers`, index and `meta.zh.json`.

### 2. MCP and environment-variable display rules are incorrectly equated

`apps/docs/content/docs/agents-create.zh.mdx:184` says MCP configuration uses the same storage/display rules as environment variables. The read paths are materially different:

- Agent `custom_env` values never appear in list/get resources and are read through the audited env endpoint.
- Agent-owned `mcp_config` can appear in list/get to the human agent owner or workspace owner/admin, unless the workspace's unconditional-redaction setting hides it. Agent actors never receive it.
- Workspace MCP library configuration is write-only even for administrators.

Evidence: `server/internal/handler/agent.go:1099,1212,1699,1722`; `server/internal/handler/agent_env.go:64`; `server/internal/handler/workspace_mcp_api.go:17`.

Recommended correction: replace the blanket equivalence with a short distinction and link to the MCP guide. Do not claim that all MCP secrets use the env unlock/audit UI.

### 3. Squad delegation guide predates immediate dispatch receipts

`apps/docs/content/docs/squads.zh.mdx:83` says the mention will enqueue the selected agent and immediately proceeds to recording an evaluation. The current operating protocol requires receipt inspection before claiming successful delegation:

- Request JSON output from the comment creation call and inspect `trigger_outcomes`.
- Match each intended target by type and exact UUID (a squad mention uses the squad UUID).
- `queued`, `coalesced`, and `deferred` confirm admission/coverage, not that work has started or completed.
- Blocked, missing, malformed, unknown or contradictory receipts leave a target unconfirmed. A delegation-only turn records `action` only if all intended targets are handled; otherwise `failed` with the partial successes and unconfirmed targets explained.
- A saved comment alone is not proof of dispatch. Do not repost or blindly retry after a lost response. Reading a saved comment cannot recover its original dispatch receipt.

Evidence: `server/internal/handler/squad_briefing.go:52-83`; `server/cmd/multica/cmd_comment_trigger_outcomes.go:54-65`; tests in `cmd_comment_trigger_outcomes_test.go`.

Recommended changes: add a short receipt subsection to the squad execution flow and an operational note to CLI/comments or mention guidance. Preserve the distinction between task admission, execution, and actual delivery. Keep the existing stage-barrier explanation: `in_review` is not terminal; final acceptance remains human/integration-owned.

### 4. Human approval instructions omit permissions and practical UI steps

The approval mechanism is implemented and `agents.zh.mdx:86` already introduces it. Add operational detail:

- Open **智能体 → 审批**. The button is present with or without waiting requests: `packages/views/agents/components/agents-page.tsx:313-332`.
- Switch between waiting requests and all requests; expand the plan to review full text and previous decision/execution notes: `packages/views/agents/approvals/agent-approvals-page.tsx:40`, `agent-approval-card.tsx:195`.
- Approve/reject use a confirmation dialog and optional decision note; pending requests can also be withdrawn: `agent-approval-card.tsx:78,143,282`.
- Only the requesting agent's human owner or workspace owner/admin may decide the request; “a human” alone is insufficient. Agent/machine credentials cannot decide: `server/internal/handler/agent_approval.go:350-378`.
- Recorded execution requires an approved request and an Operator agent, and can only be recorded by that agent or a person managing it: `agent_approval.go:429-469`.
- Approval grants permission for one action; it does not itself execute a command. Existing handler persists approval/decision/execution-note records. Do not describe it as a host sandbox or an automatic deployment executor.

### 5. Restricted reviewers are documented, but squad return-flow pages should cross-reference them

`agents-create.zh.mdx:55` already accurately states current-issue-only reviewer v2/v3 and project-bound reporter v2 restrictions, including no status changes, no trigger-producing comments, and no external MCP/plugin tools. `autopilots.zh.mdx:34-40` covers the reporter-specific behavior.

The generic squad return-route explanation at `squads.zh.mdx:111,118` should explicitly point to these restricted role versions so users do not assume every reviewer result wakes the leader. Recommend a one-paragraph exception and cross-link, without copying the full policy matrix.

## Coverage that already exists

- Skill create/import five-way chooser, template previews/copies, deployment/builtin sources, local/runtime import, update permissions, classification/tags and bindings: `skills.zh.mdx` is substantial and current for the reviewed paths.
- Nine squad templates, role reuse, Access, security-remediation workflow and human stage acceptance: covered in `squads.zh.mdx`.
- Agent template roles, autonomy badges and lack of an autonomy editing control: covered and consistent with inspected UI.
- Automation schedules/Webhooks, modes, daily/weekly reports, trigger history and management access: covered in `autopilots.zh.mdx`; no additional high-priority gap found during this bounded review.
- Provider MCP compatibility table matches the current core allowlist.

## Do not present planned controlled artifact execution as shipped

`.trellis/tasks/11-06-controlled-artifact-execution/task.json` is in progress, describing a future approval-bound artifact packager/permit/claim/receipt flow. Its `implement.md` lists implementation steps; the inspected current handler/router/type trees contain no implemented controlled-artifact or release-candidate execution endpoint. The existing approval handler is only the authorization/audit mechanism described above.

The parent task history explicitly distinguishes accepted immediate squad receipts and least-privilege role scopes from future durable receipts and broader M1-M3 work: `.trellis/tasks/11-05-agent-squad-rnd-improvements/progress.md`. Document only the current supported approval UI and current immediate receipt behavior. If a parallel task later adds product code, re-audit that concrete implementation before adding user instructions.

## Verification scope

This is semantic/source review, not evidence that the live deployment or a real model obeys every instruction. No product source was changed and no tests were run by this audit agent. The final documentation task must regenerate the Chinese embedded bundle and run its structural/MDX validations after edits.
