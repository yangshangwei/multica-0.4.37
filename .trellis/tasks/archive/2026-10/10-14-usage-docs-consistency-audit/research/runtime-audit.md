# Research: Chinese usage documentation — agents, execution, runtimes, skills, automations and CLI

- Query: Audit the 11 assigned Chinese in-app usage pages against the current worktree implementation. Report demonstrable documentation/implementation mismatches, with exact citations and corrections.
- Scope: internal; static source inspection only. Assigned pages: `agents`, `agents-create`, `skills`, `triggering-agents`, `assigning-issues`, `tasks`, `daemon-runtimes`, `install-agent-runtime`, `providers`, `autopilots`, `cli` (`apps/docs/content/docs/*.zh.mdx`).
- Date: 2026-10-14
- Status: Completed by the main session after worker rate limits. R1–R6 independently rechecked; CLI/automation and remaining page coverage are recorded below.

## Findings

### R1 — Ordinary-member agent owners can read and update their own environment variables (P2)

- Documentation: `apps/docs/content/docs/agents-create.zh.mdx:164` says “只有工作空间 `owner`、`admin` 可以解锁和修改”. The abbreviated CLI reference repeats “仅 owner 和 admin” at `apps/docs/content/docs/cli.zh.mdx:268`.
- Implementation: `server/internal/handler/agent_env.go:97` accepts workspace member roles; `server/internal/handler/agent_env.go:101` calls `canManageAgentEnv`; `server/internal/handler/agent_env.go:121` permits workspace owner/admin **or** the agent's own human owner (`member.UserID == agent.OwnerID`). `server/internal/handler/agent_env.go:92` still rejects agent actors. `GetAgentEnv` and `UpdateAgentEnv` both use this gate at lines 142 and 191.
- Impact: A normal member who created an agent is incorrectly told they cannot reveal or rotate its credentials and must involve an administrator. The asserted sensitive-data readership is also narrower than the actual contract.
- Correction: State “智能体的人类 owner，以及工作空间 owner/admin 可以解锁和修改；运行中的智能体不能调用此管理接口”. Keep the server storage and audit caveats.

### R2 — Oh-My-Pi supports Multica-managed MCP (P2)

- Documentation: `apps/docs/content/docs/providers.zh.mdx:40` lists `Oh-My-Pi | omp | ✓ | — | .omp/skills/`, marking managed MCP unavailable.
- Implementation: `packages/core/agents/mcp-support.ts:29` includes `omp` in the supported-provider set. `server/internal/daemon/execenv/omp_mcp.go:10` handles provider `omp`, and lines 21–26 write the task's MCP configuration to `.omp/mcp.json`. This is called by both preparation paths in `server/internal/daemon/execenv/execenv.go:609` and `server/internal/daemon/execenv/execenv.go:905`. An existing `.omp/mcp.json` is refused rather than overwritten (lines 27–28).
- Impact: Users can wrongly reject this runtime for workflows that need workspace MCP servers, despite both the UI and task environment supporting it.
- Correction: Change the Oh-My-Pi managed-MCP cell to ✓; explain `.omp/mcp.json` and the existing-file conflict behavior if adding operational detail.

### R3 — The supported-tool and model-exception lists omit ZeroClaw (P2)

- Documentation: `apps/docs/content/docs/install-agent-runtime.zh.mdx:14` introduces “Multica 当前会检测以下命令”, but the table at lines 18–42 has no ZeroClaw. `apps/docs/content/docs/providers.zh.mdx:21`–47 likewise omits it; line 57 names only QwenPaw and MiniMax Code as tools whose models are runtime-managed.
- Implementation: `server/internal/daemon/agents_probe.go:303` probes `MULTICA_ZEROCLAW_PATH` / `zeroclaw` and inserts the `zeroclaw` runtime. `server/pkg/agent/version.go:21` requires version 0.8.0. `server/pkg/agent/models.go:406`–408 returns no model-selection support for `qwenpaw`, `mcode`, **and `zeroclaw`**. `server/pkg/agent/zeroclaw.go:241` explicitly ignores MCP supplied by Multica and relies on ZeroClaw's own configuration; lines 408 onward guard session resume by advertised persistence capability.
- Impact: The current UI can expose a built-in runtime absent from both supported-tool guides, with no explanation for its disabled model/MCP settings or minimum version.
- Correction: Add ZeroClaw / `zeroclaw`, minimum version 0.8.0, runtime-managed model selection and no Multica-managed MCP. Describe session resume with the persistence prerequisite; do not imply that MCP configured inside ZeroClaw is unsupported by ZeroClaw itself.

### R4 — Backlog does not suppress explicit comment triggers (P1)

- Documentation: `apps/docs/content/docs/triggering-agents.zh.mdx:15` introduces rules shared by all four trigger methods; line 19 states “处于 `backlog` 的 issue 不会触发执行；把它移到 `todo` 后，已分配的智能体才会开始”.
- Implementation: Assignment/status writes suppress backlog in `server/internal/service/issue_trigger.go:129`–137. Comment mention resolution is a separate path: `server/internal/handler/comment.go:3170` loads the named agent, lines 3181–3205 check invoke permission, archive/readiness and pending work and add the trigger, without a status-category check. `resolveCommentTriggerEnqueue` at line 2097 checks formal triage admission and persisted comment eligibility, not backlog; `enqueueSingleCommentTrigger` at line 2593 calls `EnqueueTaskForMention` for a resolved mention. This also agrees with the more precise explanation in `assigning-issues.zh.mdx:48`.
- Impact: A user can believe backlog is an execution safety barrier and mention an agent while still preparing the work, unexpectedly launching a task and consuming runtime credentials/quota.
- Correction: Scope the backlog rule to assignment and status-change triggers. Explicit mentions/replies can still run on backlog issues subject to invocation/readiness/admission gates; use `/note` for non-triggering discussion.

### R5 — Runtime offline display and running-task termination have different clocks (P1)

- Documentation: `apps/docs/content/docs/tasks.zh.mdx:155` says heartbeats stop, the runtime is offline within about three minutes, and “其上的 task 随之失败”. `apps/docs/content/docs/daemon-runtimes.zh.mdx:64` similarly says running tasks fail when the runtime is offline. `tasks.zh.mdx:153` says dispatched tasks fail after five minutes without stating the lease prerequisite.
- Implementation: `server/cmd/server/runtime_sweeper.go:38` sets the default reconnect grace to **3 hours**, independently of the offline-status freshness window. `sweepOfflineRuntimeTasks` passes this grace into the failure query at lines 315–317. `server/pkg/db/queries/runtime.sql:260` (`FailTasksForOfflineRuntimes`) only selects in-flight rows whose runtime heartbeat is older than the reconnect grace. The backstop at `server/pkg/db/queries/agent.sql:1510`–1518 likewise respects the grace. Dispatched expiry at `server/pkg/db/queries/agent.sql:1485`–1504 additionally requires an absent/expired prepare lease and either a healthy runtime or an expired reconnect grace.
- Impact: Users are told to expect failure/retry in minutes during a network interruption although the server deliberately keeps execution alive for hours. They may launch redundant work or misdiagnose a still-running/dispatched record as stuck. The unconditional five-minute statement also misclassifies legitimate long preparation.
- Correction: Separate status visibility from termination: offline is displayed after the freshness window; existing execution survives until the reconnect grace (default three hours) expires, unless an earlier explicit recovery/cancellation/other failure ends it. Explain the live prepare lease exception for dispatched work.

### R6 — Intranet URL skill import and refresh are not implemented (P1)

- Documentation: `apps/docs/content/docs/skills.zh.mdx:33` offers “从内网可达的 URL 导入 skill 压缩包，例如托管在内网 Git 服务上的地址”; line 60 says these intranet URLs can later be refreshed. `apps/docs/content/docs/cli.zh.mdx:150` repeats that `--url` should be an intranet Git-hosted skill archive.
- Implementation: `server/internal/handler/skill.go:995` (`detectImportSource`) recognizes only `skills.sh`, `clawhub.ai`, and `github.com` (including their `www` forms) at lines 1013–1018. Other normal dotted intranet hosts with paths return `unsupported source` at line 1024. The URL import handler calls this detector before fetching; uploaded multipart archives take a separate path. `server/internal/handler/skill_refresh.go:57` only recognizes `github`, `skills_sh` and `clawhub` origins, and line 80 sends the saved URL through the same detector.
- Impact: The documented primary intranet URL workflow fails before downloading, even when the administrator's archive is reachable. A single-label intranet hostname can instead be misclassified as a ClawHub slug by the fallback, which still does not add intranet URL support.
- Correction: For the current implementation, document downloading/copying the archive locally and using local import or `multica skill import --file ...`; runtime import and deployment templates also remain offline-capable paths. Do not promise arbitrary intranet URL import or refresh. Supporting that would require a separately authorized implementation change, not just replacing public hosts with placeholders.

## Related Specs

- `.trellis/spec/docs/frontend/index.md` and `.trellis/spec/docs/frontend/docs-bundle.md`: Chinese in-app source/bundle pipeline, intranet distribution requirements, stable product-linked anchors.
- `.trellis/spec/server/index.md`, `.trellis/spec/server/security-boundaries.md`: human/machine credentials and invocation principals.
- `.trellis/spec/server/builtin-templates.md`: role/automation template and progress-report contracts.
- `.trellis/spec/core/frontend/index.md`, `.trellis/spec/views/frontend/index.md`: shared behavior and permission/UI boundaries.
- `CLAUDE.md` and `.trellis/workflow.md`: repository conventions and research-only planning workflow.

## Caveats / Not Found

- No product/source documentation was edited, no service or real provider CLI was started, and no tests were run. Evidence is the current worktree, including other contributors' uncommitted changes, not a release tag.
- External vendor installation steps and vendor-specific Node requirements cannot be fully established from this repository alone. No external references were fetched for this internal audit.
- Preserve the `自定义运行时配置` and `过滤事件` headings if follow-up edits are approved; product anchor parity depends on them. Chinese source changes also require regenerating the committed embedded docs bundle in the implementation phase.

## Main-session completion

The main session completed static coverage of all 11 assigned pages after the worker service failed. Canonical report IDs and exact current excerpts are in `findings.json`; the report is `../audit-report.md`. The additional confirmed CLI directory omission is D17 (approval).

- `agents`: Access 与管理权限、自治等级、归档、运行状态与执行的区别. Code entry points: `server/internal/handler/agent.go`, `server/internal/handler/agent_env.go`, `server/internal/handler/agent_autonomy.go`.
- `agents-create`: 创建与模板、配置、并发、环境变量和 MCP 权限. Code entry points: `server/internal/handler/agent.go`, `server/internal/handler/agent_env.go`, `server/internal/service/builtin_agent_templates.go`.
- `skills`: 五类创建入口、内网来源、导入、refresh、模板副本和运行时同步. Code entry points: `server/internal/handler/skill.go`, `server/internal/handler/skill_refresh.go`, `packages/views/skills/components/create-skill-dialog.tsx`.
- `triggering-agents`: 四种触发、Access、静默评论与 backlog 边界. Code entry points: `server/internal/service/issue_trigger.go`, `server/internal/handler/comment.go`.
- `assigning-issues`: 分配确认、不启动选项、backlog 状态变更、Access、小队 leader. Code entry points: `server/internal/service/issue_trigger.go`, `server/cmd/multica/cmd_issue.go`.
- `tasks`: 任务状态、排队、重试、断线宽限、准备租约、执行与 Issue 状态分离. Code entry points: `server/internal/service/task.go`, `server/cmd/server/runtime_sweeper.go`, `server/pkg/db/queries/agent.sql`.
- `daemon-runtimes`: 心跳、排队和离线、并发、运行时共享、profile 配置. Code entry points: `server/cmd/server/runtime_sweeper.go`, `server/internal/daemon/config.go`, `server/cmd/multica/cmd_runtime_profile.go`.
- `install-agent-runtime`: 内置检测命令、路径覆盖、支持版本与外部账号前提. Code entry points: `server/internal/daemon/agents_probe.go`, `server/pkg/agent/version.go`.
- `providers`: 模型选择、OMP MCP、ZeroClaw、工具相关的能力声明. Code entry points: `server/pkg/agent/models.go`, `packages/core/agents/mcp-support.ts`, `server/internal/daemon/execenv/omp_mcp.go`.
- `autopilots`: 日报/周报计划、两种输出模式、Webhook JSON/幂等/事件推断、失败暂停阈值. Code entry points: `server/internal/service/builtin_autopilot_templates_roster.go`, `server/internal/handler/autopilot_webhook.go`, `server/cmd/server/autopilot_failure_monitor.go`.
- `cli`: 全局入口、issue/agent/skill/autopilot 操作、凭据权限、approval 完整目录. Code entry points: `server/cmd/multica/main.go`, `server/cmd/multica/cmd_agent.go`, `server/cmd/multica/cmd_skill.go`, `server/cmd/multica/cmd_approval.go`.
