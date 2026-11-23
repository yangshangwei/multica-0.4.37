# Research: Task-scoped reporter/reviewer authorization

- Query: Reuse existing task identity, authorization, lifecycle and template provenance to enforce the smallest useful reporter/reviewer resource/action scope.
- Scope: internal source inspection only; no database, model, production or remote operations.
- Date: 2026-11-05
- Source baseline supplied by coordinator: `5aef9c19ad3e764d09cff939edfded53f8c513bd`, isolated checkout `/Volumes/artisan/code/2026/mc-agent-rnd-m1-20261105`.

## Findings

### Existing identity and authorization

| Source | Contract |
| --- | --- |
| `server/internal/middleware/auth.go:34,71,104` | Private authenticated-principal context plus server-stamped headers. Auth discards client X-Actor-Source. mat_ overrides user, agent, task and workspace headers from its database row. |
| `server/internal/auth/principal.go:16` | Principal records verified credential kind; AgentActor is deny-only legacy hint, never positive identity. |
| `server/internal/middleware/password_auth.go:50` | Password-mode mat_ branch independently loads token and checks account/binding provenance. A new capability must enter trusted context here as well as normal Auth. |
| `server/internal/auth/managed_binding.go:87` | Managed task credentials already check task/agent/status and execution binding; unbound task tokens return before these task checks. Preserve password version semantics. |
| `server/internal/middleware/workspace.go:76,234` | Task-token workspace cannot be overridden by slug/query/path. Workspace membership is currently evaluated for X-User-ID (runtime owner), not the task originator. |
| `server/internal/handler/handler.go:943` | mat_ author is trusted directly. Legacy JWT/PAT author resolution uses task/agent headers and checks that task belongs to agent, but these hints are not a new credential. |
| `server/internal/handler/agent_access.go:73,125,234` | Invocation judges top-of-chain human; owner/private/public-to semantics already exist. Originator becomes empty for terminal tasks. `loadedInvocationDecision` can be reused after loading current targets and proving current membership. |
| `server/internal/handler/handler.go:1124` | loadIssueForUser resolves UUID or canonical human identifier within authenticated workspace; preserve this loader rather than interpreting URL UUIDs as trusted row IDs. |

Do not replace runtime owner with originator in X-User-ID. They have different attribution and password-version meanings. A restriction is an additional intersection with both current member permissions and task scope. Require an actual nonempty originator for the new restricted profiles; do not borrow runtime owner, report creator, or unrelated autopilot creator. The existing public-to-workspace exception in invocation intentionally admits unattributed system agents, but that exception does not satisfy this new task capability's explicit human-authority requirement.

### Token minting and lifecycle gaps

- `server/pkg/db/queries/task_token.sql:1,6`: token rows bind task, agent, workspace, runtime owner and expiry; validation currently selects by hash/expiry only. No resource/action or claim-generation binding exists.
- Both daemon claim paths (`server/internal/handler/daemon.go:1911,3623`) call `TaskService.FinalizeTaskClaim` and mint a 24-hour mat_ token owned by the runtime owner.
- `server/internal/service/task.go:3735,3798`: FinalizeTaskClaim already locks runtime and exact task, checks dispatched state, runtime/issue/context/dispatched_at/claim_generation, performs execution admission, and atomically commits token plus delivery receipts. This is the natural place to derive frozen scope, never a client request field.
- `server/pkg/db/queries/agent.sql:833,985,1059`: claims/reclaims advance claim_generation. Snapshot this value into restricted credentials and compare every request; task ID alone does not distinguish turns/reclaims.
- `server/internal/service/task.go:870`, `server/internal/handler/daemon.go:4114,4804`: terminal token deletion is best effort. Restricted token validation must independently reject terminal, deleted, mismatched or superseded task rows even if deletion failed.
- Failed read is unknown authorization and must fail closed (503 for database fault, 401/403 for actual invalid authority), not drop into legacy behavior.

### Provenance and opt-in compatibility

- `server/internal/handler/agent_template.go:106,226`: public adoption accepts a template key, while server picks actual instructions/autonomy/skills and passes private `agentTemplateProvenance`.
- `server/internal/handler/agent.go:1340,1567`: template_key/template_version are separate trusted creation arguments. General create cannot claim them. Update permits instructions/name and human-only autonomy changes (`:1932`) but has no template identity setter.
- `server/internal/service/builtin_agent_templates_roster.go`: current progress-reporter is v1, code-reviewer v1, security-reviewer v1, migration-reviewer v2. Natural first opt-in revisions are reporter v2, code/security v2, migration v3. These are candidates, not a statement that all four must ship in the first child.
- `server/internal/handler/squad_template.go:486,506,544`: squad staffing reuses an existing agent by workspace/template_key as-is, ignoring new roster version. This is intentional customization protection. Do not silently upgrade reused older agents.
- `server/internal/service/builtin_agent_templates.go:69`: unknown/empty autonomy is deliberately unrestricted legacy behavior; do not reclassify these agents merely by a name, prompt, skill name, or autonomy string.
- Current role instructions are mutable workspace copies. New capability selection should use server-issued template provenance plus explicit minimum adopted version, and freeze the selected policy onto each token. A later human autonomy edit cannot silently broaden an already minted credential.

### What the current autonomy gates do not enforce

`server/internal/handler/agent_autonomy.go:42,68,132` checks minimum rank on selected handlers, permits unknown levels, and skips policy on lookup failure. issueDirectionFields are only status, assignee_type and assignee_id. Thus observer does not mean comment-only: title, description, priority, metadata, attachments and other routes require a distinct default-deny capability. Reporter contributor can currently create/delete/rerun and modify unrelated issues. Adding more prose or one status gate does not close these paths.

### Minimal useful read/action matrix (coordinator decision pending)

The coordinator is discussing exact product scope with the user. Latest proposed boundary is reporter frozen project reads/current report comment; reviewer current issue reads/comment; **no status changes in this first slice**. The earlier candidate reporter-only in_review transition is intentionally excluded from this recommendation pending product decision.

| Resource/action | Reporter candidate | Reviewer candidate |
| --- | --- | --- |
| GET current issue by UUID or valid identifier | allow | allow |
| GET current issue comments/timeline | allow | allow |
| GET source issue/detail/comments/timeline in report issue's frozen project | allow only same frozen project/workspace and current authority | deny |
| GET /api/issues with project_id exactly frozen project | allow with server-enforced AND scope; pagination supported | deny |
| Report has no project | current issue only; no implicit whole-workspace scope | current issue only |
| POST current issue comments | allow; no extra execution | allow; no extra execution |
| POST comments on another source issue | deny | deny |
| PUT issue including in_review; batch, metadata, labels, properties, create/delete/move, rerun/cancel | deny | deny |
| Comment edit/delete/resolve/reactions or attachments | deny in smallest slice | deny in smallest slice |
| Search/grouped/query/table/dashboard/workspace inventories | deny until independently scope-aware | deny |
| GET /api/docs/manifest and /api/docs/page | safe static documentation candidate | same |
| Agent/skill/MCP/plugin/autopilot/admin/token mint/billing routes | deny | deny |
| Any new or unclassified authenticated path/method | deny | deny |

`ListIssues` in `server/internal/handler/issue.go:1110,1159,1195,1427` supports project_id in both ordinary paginated and open_only branches; other project_ids/include_no_project clauses AND with it. Freeze the scope at claim time and recheck that the report issue is still in that project; moving the report must not retarget an old token. Either require a single exact project_id query parameter or inject the frozen one server-side before the existing list handler. Reject or prove parser equivalence for duplicates, aliases, escaped paths, method variants and POST /query rather than assuming query syntax is harmless.

This permits a project reporter to gather current issue facts without creating a new reporting system; deterministic period snapshots and workspace-wide report policy remain M2. A no-project report can truthfully state its limited coverage, not claim a complete workspace report.

### Comment delivery and delegation are separate

- `server/internal/handler/comment.go:1762,1794`: CreateComment preserves source_task_id lineage and exact allowed reply-parent checks for comment/coalesced-comment tasks. Keep these checks.
- `comment.go:1914,1955`: successful creation immediately calls triggerTasksForComment. Explicit mentions dispatch before agent fallback (`:2637,2675`). Simply allowing POST comments allows delegated writes.
- `comment.go:2694`: a plain agent result on a squad-assigned issue also wakes the assigned leader. This is an existing lifecycle return route; suppressing it changes orchestration behavior for the newly opted-in roles and must be documented.
- `comment.go:3210,3447`: editing may cancel/retrigger, so deny edit in the first small slice. Admin membership inherited from runtime owner would otherwise permit editing another author's comment.
- `comment.go:3657` and `comment_reconcile_test.go`: deferred completion/cancellation replays recompute trigger decisions from persisted comments after the original request/token ends. A request-only suppression flag is insufficient.
- `server/pkg/db/queries/comment.sql:440`: CreateComment atomically bumps issue revision and writes dispatch_eligible from admission state. The existing durable dispatch_eligible fence is a possible reuse point: add a server-only suppress-execution input that ANDs with admission (default false for all existing callers), then ensure immediate routing and later replay honor it. Do not blindly flip existing comments or overload the refusal explanation as triage when the reason is capability. Alternative is a durable task-policy lookup in every dispatch replay; that needs correct task retention semantics.

Latest coordinator preference is **no comment-triggered execution, including implicit leader wake, for first-slice restricted profiles**. Preserve actual comment delivery and return no misleading queued receipt. Explicit mentions may return blocked trigger_outcomes with an appropriate reason. Record that new reviewer profiles need a human/existing authorized coordinator to continue the workflow until a narrowly established leader-return capability is designed; old role versions keep their existing lifecycle behavior.

### Daemon/tool credential bypass surface

- `server/internal/handler/daemon.go:2212`: claim payload gathers plugin hooks and plugin Remote MCP connections before agent payload is built.
- `daemon.go:2303-2369`: agent CustomEnv/CustomArgs, saved mcp_config, enabled workspace MCP servers and task RuntimeMcpOverlay are merged into execution input.
- `daemon.go:2071`: any RemoteMCPConnections causes a separate mdt_ credential to be minted; it is committed with mat_ but authorized by a different DaemonAuth group (`server/cmd/server/router.go:1544`). A restriction only in ordinary Auth does not constrain this channel.
- `server/internal/handler/plugin_agent_hook.go:29`: daemon-authenticated task plugin hook performs signed external actions after task/workspace access.
- `server/internal/handler/plugin_mcp.go:130`: daemon task credential resolution exposes plugin MCP credentials. Task policy must be checked here too or restricted claims must be rejected before these tools/credentials are supplied.
- `server/internal/daemon/daemon.go:7076`: actual broker preparation consumes the RemoteMCPConnections from claim.
- Static skill content is already provided inline or through authenticated daemon skill-bundle resolution (`daemon.go:2404`). No unrestricted agent /api/skills grant is needed to deliver these role instructions.

For the bounded M1 API capability, either (a) withhold all plugin hooks, Remote MCP connections/token and agent/runtime MCP overlay from restricted claims and enforce backstops on daemon tool endpoints, or (b) reject such configured claims with an actionable unsupported-policy error until tool-scope enforcement is available. Do not return a superficially restricted mat_ alongside unrestricted external tool credentials. Which option is appropriate is a concrete product compatibility decision; stripping configured tools silently is undesirable.

This still is **not a shell sandbox**: the provider process has daemon-user filesystem and ambient command/network privileges, CustomEnv/CustomArgs can themselves supply credentials, and a user-installed CLI may have ambient settings. State this boundary explicitly. High-risk execution controls and full tool isolation require their own M1 work package.

### WebSocket and other auth surfaces

`server/internal/realtime/hub.go:757` accepts mul_ through PAT resolver and otherwise JWT parsing; mat_ is not accepted. `/ws` sits outside ordinary Auth (`router.go:1382`), so test its rejection as a regression rather than assume adding a gate to Auth covers it. DaemonAuth accepts mdt_/mul_/cloud/JWT, not mat_. Plugin public APIs have their own bearer shape. Restricted mat_ must not be exchanged for these credentials, and existing RequireHumanActor mint gates remain unchanged.

## Suggested implementation ownership

1. Credential/schema/service lane: one additive migration without FK/index, task_token SQL and sqlc output; pure versioned profile selector; FinalizeTaskClaim frozen policy/issue/project/originator/generation data from locked rows; secure claim rejection/omission of unsupported tool credentials.
2. API lane: trusted capability context in both auth modes; one default-deny authenticated API guard using resolved resources and exact methods; current task/generation/originator/runtime-owner membership/invocation revalidation; comment durable non-dispatch, immediate/preview/replay consistency; daemon plugin backstops if used.
3. Template/docs lane only after policy is frozen: selected role revision bumps, clear adopted capability instructions, role skills/source maps and bilingual docs. Preserve existing instance content/version and reused squad behavior.
4. Independent tests/review: real isolated database claim/router positives and negatives, password/nonpassword paths, old-version controls, malformed/unknown policy, task cancellation/reclaim/revocation, source project movement, URL/UUID/header forgery, default-deny route inventory, comment replay and external tool rejection. Compile-only is not handler execution evidence.

Keep ownership sequential for shared generated models and daemon.go. Auth-only helper tests cannot prove route wiring, actual immutable token minting, or absence of deferred task side effects.

## Related specs

- `.trellis/spec/server/security-boundaries.md`: machine credential minting, originator authority, lifecycle transaction contract, password credential provenance.
- `.trellis/spec/server/builtin-templates.md`: server provenance, preserved saved copies, reporter completion and observer distinctions.
- `.trellis/spec/server/database-test-isolation.md`: fresh task-owned DB per DB suite; go test -run executes TestMain; capture named pass/fail inventory.
- `.trellis/spec/guides/cross-layer-thinking-guide.md`: cross-layer schema/consumer boundaries.

## External references

None required: all findings are from the local implementation; no unfamiliar external API or dependency is proposed.

## Caveats / Not Found

- No persistent resource/action capability exists on ordinary task tokens today. Collaboration's independent token/attempt state machines must not be repurposed for ordinary issues.
- No full read-safe inventory was proven for every GET route; broad GET allowance would leak non-task resources and can reach operational probes. Default deny is needed.
- New reporter comment-only policy differs from current v1 prose that permits current-report in_review. The coordinator has explicitly proposed this narrowing to the user; do not claim it is already accepted or silently retain contradictory instructions.
- No live database tests were run, no model behavior was measured, and no runtime isolation guarantee is established by this research.
- Candidate schema/route design above needs independent design review and implementation-level race/error validation before acceptance.
