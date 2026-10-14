# 运行时与 CLI 文档修订记录

日期：2026-10-14。范围：D01、D02、D03、D05、D06、D07、D17。

本组修改限于分配的 8 篇中文 MDX 和本记录。修改前这 8 篇源文件没有未提交差异。已保留原审计证据，没有修改业务代码、测试、任务元数据、规范或共享生成包；生成和集成检查由主会话统一执行。

以下 7 项已完成文案修订与当前源码对照，可进入独立复核。这里的关闭结论仅针对原审计描述的不一致，不代表已运行真实 AI 工具或通过整包验收。

## D01：内网 skill 导入与更新

- 修订位置：`apps/docs/content/docs/skills.zh.mdx:33`、`:36`、`:60`；`apps/docs/content/docs/cli.zh.mdx:138`、`:145`、`:150`、`:281`。
- 结果：明确 URL 导入仅识别 GitHub、skills.sh、ClawHub 来源，网络可达不代表任意内网 Git 或压缩包 URL 受支持。内网操作示例统一使用本地 `.skill` / `.zip` 和 `--file`，保留本地文件夹、模板和运行时复制入口。从来源更新须有有效托管来源且服务器可访问，内网改为获取新版文件后重新本地导入。
- 依据：`server/internal/handler/skill.go:995` 的 `detectImportSource` 只适配三类来源，`:2380` 的 multipart 分支处理本地归档；`server/internal/handler/skill_refresh.go:57`、`:75` 同时校验来源类型和 URL；`server/cmd/multica/cmd_skill.go:152`、`:153`、`:508` 区分 URL 和本地文件导入并要求互斥。
- 关闭结论：移除了任意内网 URL 可导入或可刷新这一未实现能力的承诺。未新增来源、域名或联网要求。

## D02：backlog 与评论触发

- 修订位置：`apps/docs/content/docs/triggering-agents.zh.mdx:10`、`:18`、`:19`、`:20`。
- 结果：分开解释分配/状态变更和评论/回复；`backlog` 类别只让分配及同类别状态变化保持静默，正式任务上的显式提及和符合路由条件的回复仍可触发。说明 `/note` 必须位于开头、也可取消触发预览中的全部目标。待分拣记录须先接纳，不能靠状态变化获得执行资格。
- 依据：`server/internal/service/issue_trigger.go:100`、`:129`、`:135` 分别检查正式接纳、分配和离开 backlog 类别；`server/internal/handler/comment.go:2096`、`:2637` 检查正式接纳与评论资格而不检查 backlog；`:2673` 优先显式目标，`:2702` 处理回复智能体；`:1941` 识别首 token `/note`，`:1963`、`:1995` 应用用户取消的目标；`server/internal/admission/admission.go:13`、`:28` 定义独立的正式任务门禁。
- 关闭结论：不再把 backlog 描述成所有触发方式的暂停开关。

## D03：离线显示、重连宽限和重启回收

- 修订位置：`apps/docs/content/docs/daemon-runtimes.zh.mdx:57`、`:64`、`:65`；`apps/docs/content/docs/tasks.zh.mdx:80`、`:96`、`:149`、`:151`、`:157`。
- 结果：区分默认每 15 秒心跳、通常最迟约 3 分钟的离线显示，以及从心跳缺席起计算的默认 3 小时重连宽限。在途 task 不因离线标记立即失败；启动超过 5 分钟还要结合准备租约和运行时健康判断。守护进程实际重启并重新连接服务器后的回收不等待该宽限。等待本地目录的 task 同样受失联及重启回收影响。
- 依据：`server/internal/daemon/config.go:24` 定义默认心跳；`server/internal/service/task.go:198` 定义 150 秒新鲜度，`server/cmd/server/runtime_sweeper.go:24` 每 30 秒扫描、`:38` 定义 3 小时默认宽限、`:66` 定义 300 秒 dispatch 阈值。`server/pkg/db/queries/runtime.sql:260` 对离线超过宽限的 dispatched/running/waiting_local_directory 行执行回收；`server/pkg/db/queries/agent.sql:1485` 同时检查 dispatch 停留时间、准备租约和运行时存活/宽限（运行时记录缺失另有兜底）。`server/internal/daemon/daemon.go:6352`、`:6907` 在下载 skill 和准备环境期间续租；`server/pkg/db/queries/agent.sql:1430`、`server/internal/handler/task_lifecycle.go:27` 与 `server/internal/daemon/daemon.go:3807` 实现重新注册后的孤儿执行回收和失败后处理。
- 关闭结论：移除了“显示离线就失败”和无条件“dispatched 超过 5 分钟失败”。`自定义运行时配置` 受保护标题保持原样。

## D05：环境变量管理权限

- 修订位置：`apps/docs/content/docs/agents-create.zh.mdx:164`；`apps/docs/content/docs/cli.zh.mdx:269`。
- 结果：包括智能体的人类所有者和工作空间 owner/admin；明确所有者即使是普通 member 也可管理。保留服务器明文存储、列表不返回值、读取/修改审计说明。以智能体身份调用时，包括自己的变量在内均不可通过管理接口读取或修改。
- 依据：`server/internal/handler/agent_env.go:77` 的授权入口先拒绝 agent actor（`:92`），再接受工作空间 member 并检查所有权（`:97`、`:121`）；所有者比较使用已验证成员的 `UserID`。`:141` 开始的读取路径保留审计后才能返回明文的规则。
- 关闭结论：权限主体与现有授权逻辑一致。

## D06：Oh-My-Pi 托管 MCP

- 修订位置：`apps/docs/content/docs/providers.zh.mdx:40`、`:80`。
- 结果：能力表改为支持，补充 `.omp/mcp.json` 已存在时准备失败而非覆盖的限制。
- 依据：`packages/core/agents/mcp-support.ts:29` 包含 `omp`；`server/internal/daemon/execenv/omp_mcp.go:10` 写入工作目录下的配置并拒绝预先存在的路径；`server/internal/daemon/execenv/execenv.go:609` 在执行准备中调用该逻辑。
- 关闭结论：能力表和配置冲突说明与前后端一致。

## D07：ZeroClaw 支持范围

- 修订位置：`apps/docs/content/docs/install-agent-runtime.zh.mdx:43`、`:61`、`:85`；`apps/docs/content/docs/providers.zh.mdx:48`、`:58`、`:72`、`:78`、`:88`。
- 结果：补齐 `zeroclaw` 检测命令和 0.8.0 最低版本；模型和 MCP 由工具配置管理，Multica 侧模型不可选、MCP 不下发。补充多个工具智能体配置时的 alias 选择方式，以及持久化存储和 ACP resume 能力对会话恢复的限制。
- 依据：`server/internal/daemon/agents_probe.go:303` 检测 `MULTICA_ZEROCLAW_PATH` / `zeroclaw`；`server/pkg/agent/version.go:21` 定义 0.8.0；`server/pkg/agent/models.go:406` 禁用模型选择；`server/pkg/agent/zeroclaw.go:56` 提取 `--agent` / `--agent-alias`，`:88` 检查 ACP resume 能力、`:234` 忽略 Multica MCP、`:408` 在能力缺失时拒绝恢复并交由新会话恢复路径处理。
- 补充边界：`server/internal/daemon/execenv/context.go:340` 的 `skillsDirPath` 对 ZeroClaw 走通用 `.agent_context/skills/` 分支（`:443`）；`server/internal/daemon/execenv/runtime_config_sections.go:773` 仅列 skill 名称。能力表因此仅描述文件写入位置，并明确当前集成未配置专用原生发现路径，没有宣称已验证 ZeroClaw 自动加载绑定 skill。
- 关闭结论：已补齐原审计缺失的支持项及必要条件。真实 ZeroClaw 的 skill 发现与持久化会话行为未运行验证，保留为限制。

## D17：approval 命令目录

- 修订位置：`apps/docs/content/docs/cli.zh.mdx:181`、`:289`、`:296`。
- 结果：补入 `approval request/list/get/executed/cancel`，注明申请参数、状态过滤和结果记录；批准/驳回由有权管理该智能体的人在 Multica 界面中完成。`executed` 只记录实际结果。
- 依据：`server/cmd/multica/main.go:58`、`:85` 将 approval 注册在 Core 组；`server/cmd/multica/cmd_approval.go:29` 定义用途，`:257` 注册 flags，`:275` 注册五个子命令。`server/internal/handler/agent_approval.go:358` 拒绝机器凭据审批，`:375` 检查智能体管理权限；界面入口见 `packages/views/agents/approvals/agent-approval-card.tsx:162`。
- 关闭结论：完整顶层命令目录包含 approval，并未暗示 CLI 可自行批准申请。

## 已执行检查与交接

- 对照当前源码复核全部 7 项；检查了本组 8 篇源文档的完整 diff。
- `git diff --check -- apps/docs/content/docs/{agents-create,cli,daemon-runtimes,install-agent-runtime,providers,skills,tasks,triggering-agents}.zh.mdx`：通过，无空白错误。
- 本轮新增内部引用目标为已有 `/triage`，以及已存在的 `/skills#更新已导入的-skill`、`/environment-variables`、`/providers`；未改目录、图片或受保护标题。
- 未运行 generator、Vitest、类型检查、MDX 编译或 Go 检查；按分工由主会话在三组整合后统一生成、验证。没有新增仅断言文案的测试。
- 未进行真实 CLI 登录、工具执行、MCP 连接、3 小时断线实验或公网 skill 导入。约 3 分钟是默认心跳新鲜度与扫描周期的通常上界，不是运行环境中的时延测量。
