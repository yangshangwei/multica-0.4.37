# 基线调研：智能体现有上下文机制（2026-10-26）

只读核查，未改代码。行号为当日快照；普通本机执行路径（`server/internal/daemon/collaborationrun/host_*.go`、`server/internal/daemon/execenv/collaboration.go`、迁移 738）当日为未提交改动且仍在编辑，行号可能漂移。

## 协作 run 当前拿到什么

- `/api/collaboration/runtime/runs/{id}/context` 返回 `CollaborationRuntimeContext`（`server/internal/cli/collaboration.go:127-143`，服务端 `server/internal/service/collaboration_squad.go:47-59`）：运行身份、标题、正文、智能体与小队、roster、本组冻结的人类指令 `inputs`、最近 200 条未删除评论、`dispatch_comment`。构建链路：`server/cmd/server/router.go:1400` → `handler/collaboration_runtime_business.go:9` → `service/collaboration_squad.go:146-203`。
- 评论查询无作者或可见性过滤（`server/pkg/db/queries/collaboration_squad.sql:42-49`）；`inputs` 来自冻结的 `prompt_snapshot`（`service/collaboration_runtime_capabilities.go:259-294`）。
- 服务端拒绝启动配置了 skills、MCP、自定义 env/args、runtime config、connected apps、autonomy 或 service tier 的智能体，以及有说明的小队；只放行 Instructions、Model、ThinkingLevel（`service/collaboration_runtime_invocation.go:54-116`）。
- 普通本机路径：在工作目录写 CLAUDE.md/AGENTS.md（边界说明、`multica issue get` 指引、智能体说明，`host_runner.go:116, 434-443`），prompt 为 `{issue_id, agent_id, owners, parent_run_id, inputs, attachments, git}` JSON（`host_runner.go:445-465`）。
- 运行时 CLI 白名单：`issue get`、`issue comment list`、`issue comment add`、`squad activity`，限本单，写入须 `--request-id`（`server/cmd/multica/cmd_collaboration.go:14-58`）。
- 运行时使用绑定 run 与 attempt 的 `mcb_` 令牌，每次调用复核（`service/collaboration_squad.go:84-106`）。

## run 之间延续什么

- 经服务端延续的只有标题、正文和评论（包括智能体在前序 run 发的评论）。run 结果写入 `collaboration_run_event`（`service/collaboration_runtime_capabilities.go:391-474`），不进入 `/context`。
- Linux 路径：resume 固定为 `fresh`（`server/pkg/protocol/collaboration_preparation.go:265`），`/work`、`/tmp`、`/home` 每次为新 tmpfs（`collaborationrun/controller_linux.go:182-189`）。
- 普通本机路径：provider 会话与工作目录只保存在本机（`host_session.go:60-120`、`host_preparation.go:44-81`），键含 runtime、provider、角色、session generation 与配置摘要（`host_session.go:18-45`），换运行时或机器即无会话、新工作目录。

## 普通任务的对照

- 工作空间上下文（`workspace.context`，迁移 006）与项目上下文（`handler/project_resource.go:917-939`）进入 brief（`execenv/runtime_config_sections.go:186, 418-440, 944`）；协作 run 不获得这些。
- 会话恢复仅限同一 (agent, issue) 且同一 `runtime_id`（`handler/daemon.go:2756-2770`）。
- Multica 没有智能体记忆功能；普通任务跨 run 的渠道是 issue `metadata`（`runtime_config_sections.go:446-452`）。Codex 原生记忆被关闭（`execenv/codex_memory.go:61-63`）。
- `packages/core/docs` 是产品文档阅读器，不是工作空间知识库，无 CLI 入口。

## 权限与外泄边界

- 协作读取均经 `ResolveIssueAccess`，非人类返回 404（`service/collaboration.go:65-70`）。普通任务路由经 `rejectLegacyCollaborationAccess`：写入 409、机器读取 404（`handler/collaboration_boundary.go:61-85`）。
- 机器调用的列表与搜索只得 `{id, workspace_id, number, identifier, title}`（`collaboration_boundary.go:25-38`）；协作单的插件事件被扣留（`service/plugin_event_dispatch.go:237, 266-272`）。
- run 记录对有 Read 能力的人可读，含非参与者的来源空间成员（`handler/collaboration_records.go:140-147`、`internal/collaboration/access.go:80-88`）。

## 与上限、配置相关

- 协作评论上限 65,536 字节（`packages/core/collaboration/text-limits.ts:3`）。
- 部署级数值限制走环境变量（如 `DATABASE_MAX_CONNS`、`RATE_LIMIT_AUTH`）；管理后台设置页只读展示部署配置（`handler/admin_settings.go:28-54`）。
- 服务端辅助 LLM 仅用于聊天标题与追问，可不配置，使用运营方账号（`server/pkg/llm/client.go`）。
- Claude 的 1M 上下文为显式变体（`server/pkg/agent/models.go:449-453`）；上下文溢出有独立失败分类（`server/pkg/taskfailure/classify.go:332-361`）。

## 未核实

- 普通本机路径下按键隔离的 CODEX_HOME 是否关闭 Codex 记忆；Hermes 的记忆处理。
- Mobile 普通任务列表是否包含协作单。
