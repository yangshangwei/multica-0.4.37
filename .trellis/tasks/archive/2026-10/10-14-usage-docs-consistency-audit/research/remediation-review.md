# 文档修订独立复核

日期：2026-10-14。执行角色：独立 `trellis-check`，未再派生子代理。

本次读取完整的保存版 hook 上下文、原始 `findings.json`、任务 PRD / design / implement 和三组 `remediation-*.md`，对照当前共享工作区中的实现，复核 D01–D18 对应的修改及相关上下文。按主会话后续授权补入 `project-resources.zh.mdx`，最终源文件范围为 `remediation-baseline.json.source_paths` 中的 **19 个 MDX：18 篇中文文档及英文开发规范**。原始审计证据保持历史原貌。

结论：下面五处局部问题已修正，D01–D18 的本轮语义修订中没有发现尚未解决的阻塞项。19 个源文件的空白检查和 docs 类型检查通过。此结论是源码与文档语义复核结果，不代替生成包、MDX 编译、链接检查、Go 测试或真实运行验证；后者由主会话整合执行。

## Findings (fixed)

### D18：教程仍用旧图的四个选项描述当前界面

- File：`apps/docs/content/docs/tutorial.zh.mdx:284`。
- Issue：正文仍说“新建 skill 提供四种方式”，缺少当前的“从模板中修改”，并使用了与当前 locale 不同的“手动新建”“从本机运行时复制”。
- Evidence：`packages/views/skills/components/create-skill-dialog.tsx:76` 的 `MethodChooser` 实际列出 `manual/template/local/url/runtime` 五项；`packages/views/locales/zh-Hans/skills.json:334` 提供当前标签。独立查看 `tutorial-skill-new-modal.webp`，旧图确实只有四项。
- Fix：正文改为当前五项及对应中文标签，保留旧版截图的“四种方式”替代文本，并明确截图尚无模板入口。补充内网使用本地文件或部署模板、细节见 Skills 页。

### D06：Oh-My-Pi 的文件冲突缺少配置前提

- File：`apps/docs/content/docs/providers.zh.mdx:80`。
- Issue：新段落容易被读成每次 OMP 执行都写入 `.omp/mcp.json`，只要已有该文件就失败。
- Evidence：`server/internal/daemon/execenv/omp_mcp.go:10` 的 `prepareOmpMcpConfig` 先检查 provider，再对空或 `null` 的 MCP 数据直接返回；仅下发配置时才创建文件并拒绝覆盖。实际调用位于 `server/internal/daemon/execenv/execenv.go:609`。
- Fix：将写文件及冲突规则限定为“下发托管 MCP 配置时”，明确未下发时不因已有该文件产生此冲突。

### D17：审批执行登记遗漏权限条件

- File：`apps/docs/content/docs/cli.zh.mdx:296`。
- Issue：补入 `executed` 后只说明已获批和记录结果，未说明所涉智能体的自主权限及登记主体限制。
- Evidence：`server/internal/handler/agent_approval.go` 的 `RecordAgentApprovalExecution` 检查申请所属智能体或可管理该智能体的人，并要求 `AutonomyAtLeast(..., AutonomyOperator)`；更新语句要求 `approved`。`server/cmd/multica/cmd_approval.go` 的 `approvalExecutedCmd` 也明确该前提。中文标签 `运维执行` 来自 `packages/views/locales/zh-Hans/agents.json:400`。
- Fix：补齐已获批、`operator` 自主权限和登记主体三个条件；继续明确命令只登记实际结果，不代为执行动作。

### D12：限流表未明确区分邮箱邀请与分享链接

- File：`apps/docs/content/docs/environment-variables.zh.mdx:162`、`:163`、`:175`。
- Issue：“工作空间邀请”在新增分享链接说明后容易被理解为两种邀请都受同一邮件限流保护。
- Evidence：`server/internal/handler/invitation.go:137` 调用 `checkInvitationAdmission`；`server/internal/handler/invitation_rate_limiter.go:89` 按邀请人、工作空间、收件邮箱检查。`server/internal/handler/share_link.go` 的创建和加入路径不调用该邮件限流器。
- Fix：将相关额度和故障策略明确限定为“邮箱邀请”，保留 Redis 预检查失败才拒绝发送的边界。

### D13：项目资源页仍把原地目录锁泛化到全部执行

- File：`apps/docs/content/docs/project-resources.zh.mdx:15`、`:23`、`:42`、`:67`、`:69`、`:71`、`:75`、`:96`。
- Issue：此页的概述、原地模式和非 Git 目录说明仍承诺所有执行串行，与已修订安全模型中的对话免锁规则矛盾；“无限并发”“否则一直等待”也掩盖了正常并发门槛和离线回收。
- Evidence：`server/internal/daemon/local_directory.go:151` 的 `localDirectoryLockExempt` 仅以 `ChatSessionID != ""` 判定对话免锁；`resolveRealPath` 将符号链接归一。`server/internal/daemon/daemon.go:5454` 的工作树分支及 `:5471` 的对话分支跳过目录互斥；普通原地执行继续等待同一 daemon 的目录锁。`server/internal/daemon/execenv/execenv.go:492` 起分别准备普通目录、`LocalWorkDir` 和工作树。失联、重启的等待执行回收见 D03 的 SQL。
- Fix：统一限定为同一守护进程、同一真实目录的非对话执行串行；对话免锁但仍直接使用该目录，目录锁不阻止人工或其他进程写入。概述区分原地与工作树，工作树只免目录锁而仍受并发上限约束，等待段链接执行回收规则。没有改运行策略、目录校验、能力声明或 Git 行为。

## D01–D18 语义复核矩阵

以下“通过”表示当前文档对对应发现的描述与已读取的实现相符；主会话仍需完成生成及集成验收。路径均相对于仓库根目录，代码行号可能随其他会话修改移动，应同时按函数名定位。

| 发现 | 文档位置 | 独立核对的实现与边界 | 结论 |
| --- | --- | --- | --- |
| D01 | `skills.zh.mdx:33`、`:60`；`cli.zh.mdx:138`、`:150` | `server/internal/handler/skill.go:995` 的 `detectImportSource` 只选择 GitHub / skills.sh / ClawHub；归档走 multipart。`skill_refresh.go:57`、`:75` 检查来源类型与 URL；`server/cmd/multica/cmd_skill.go:508` 要求 `--file` 与 `--url` 二选一。网络可达不等于支持任意内网 URL，本地 `.skill/.zip` 是可用替代。 | 通过 |
| D02 | `triggering-agents.zh.mdx:10`、`:18` | `server/internal/service/issue_trigger.go:100`、`:129` 将分配与离开 backlog 类别分别处理；`server/internal/handler/comment.go:2637` 检查正式接纳而不检查 backlog；`:1941` 的 `/note` 必须是首 token，取消预览目标经 `filterSuppressedCommentAgentTriggers` 生效。没有把 backlog 当作全局静默开关。 | 通过 |
| D03 | `daemon-runtimes.zh.mdx:57`、`:64`；`tasks.zh.mdx:80`、`:149` | `server/cmd/server/runtime_sweeper.go:24`、`:38`、`:66` 分开扫描周期、3 小时重连宽限、5 分钟 dispatch 阈值。`server/pkg/db/queries/runtime.sql:260` 等待离线宽限；`server/pkg/db/queries/agent.sql:1430` 重启回收，`:1485` 结合准备租约和运行时健康判超时。`daemon.go` 的 `startTaskPrepareLeaseExtender` 与启动阶段调用为准备工作续租；`task_lifecycle.go:27` 处理孤儿回收。约 3 分钟离线显示没有被写成失败时间。 | 通过 |
| D04 | `self-host-quickstart.zh.mdx:191`；`desktop-app.zh.mdx:21`、`:33`；`environment-variables.zh.mdx:23` | Compose 默认 `legacy`；`server/internal/handler/auth_device.go:99` 默认自托管启用设备身份，`DeviceLogin` 排除 password 模式，`provisionDeviceIdentity` 区分未配置与已配置共享工作空间；`packages/core/platform/auth-initializer.tsx:425` 自动设备登录。密码、邮箱和日志取码各有明确条件，浏览器与 Desktop 的设备身份分别保存。 | 通过 |
| D05 | `agents-create.zh.mdx:164`；`cli.zh.mdx:269` | `server/internal/handler/agent_env.go:77` 先拒绝 agent actor，`:121` 允许工作空间 owner/admin 或已验证成员中的智能体所有者；明文读取写入审计后才返回，修改与审计同事务。普通 member 所有者的权限已写明。 | 通过 |
| D06 | `providers.zh.mdx:40`、`:80` | `packages/core/agents/mcp-support.ts:29` 包含 OMP；`omp_mcp.go:10` 及 execenv 调用支持注入并拒绝覆盖现有文件，空配置不进入写入。 | 通过，含本次条件补修 |
| D07 | `install-agent-runtime.zh.mdx:43`、`:61`、`:85`；`providers.zh.mdx:48`、`:58`、`:72`、`:78`、`:88` | `server/internal/daemon/agents_probe.go:303` 检测 ZeroClaw；`server/pkg/agent/version.go:21` 为 0.8.0；`models.go:406` 禁止 Multica 模型覆盖；`zeroclaw.go:56` 提取 alias，`:88` 检查 resume 能力，Execute 忽略传入 MCP。`execenv/context.go` 的 `skillsDirPath` 对该 provider 使用通用目录，说明没有冒充已验证的原生 skill 发现能力。 | 通过 |
| D08 | `members-roles.zh.mdx:27`、`:39`；`tutorial.zh.mdx:335` | `invitation.go` 限定邮箱及 admin/member，固定 7 天；`share_link.go` 以事务轮换已有链接，`share_link.sql` 重新检查启用、有效期、次数。`members-tab.tsx` 默认 168 小时并提供 24/168/720/永久选项、复制和撤销。`auth/password_session.go:142` 不允许尚需设置或改密的会话加入。区分“页面默认 7 天”与 API 可省略有效期，没有夸大为所有邀请都不能复制。 | 通过 |
| D09 | `comments.zh.mdx:32`、`:37` | `comment.go:2673` 的显式 agent/squad 分支先于 `@all` 和 member 提及返回；`@all` 自身不入队，也不吞掉显式目标。`/note` 和取消预览目标可让评论不触发执行。 | 通过 |
| D10 | `comments.zh.mdx:48` | `comment.go` 的 `strictContentEdit` 将成功的正文更新与取消旧输入 task 放在同一事务；旧客户端先取消再更新，失败时修复存活输入。`agent.sql:754` 涵盖 queued/dispatched/running/waiting/deferred 以及合并输入。删除取消错误只记录日志后继续，所以文档写“尝试取消”，未承诺取消所有子回复触发的执行或回滚外部结果。仅附件变化不重算正文触发。 | 通过 |
| D11 | `squads.zh.mdx:65`、`:79`；`tutorial.zh.mdx:246` | `comment.go:2673` 显式目标优先；`:2683` 的智能体结果才考虑队长兜底；人类回复优先父作者或线程承接。`routeAssignedSquadLeaderFallback` 检查运行条件和权限；`squad.go:1393` 按执行角色处理队长自触发。没有恢复“提及另一智能体仍额外唤醒原队长”的旧例外。 | 通过 |
| D12 | `environment-variables.zh.mdx:153` | `server/cmd/server/router.go:1427` 仅将通用 Redis IP limiter 接在邮箱、Google、设备路径；`middleware/ratelimit.go:60` 在无 Redis 或错误时放过这一层。`auth/password_config.go:44` 要求 single/shared，`password_limiter.go:50` 区分内存与 Redis；`auth_password.go:33` 对异常返回 503，额度耗尽返回 429。邀请预检查与后续消费的错误行为已区分。 | 通过，含邮箱邀请范围补修 |
| D13 | `security-model.zh.mdx:38`、`:42`；`project-resources.zh.mdx:42`、`:67` | 普通目录、工作树、原地目录与同 daemon 非对话目录锁相互区分；对话免锁不会得到副本，也不限制其他写入者。参见本报告 D13 补修依据。 | 通过，含配套页面同步 |
| D14 | `desktop-app.zh.mdx:160` | `apps/desktop/src/shared/runtime-config.ts:167` 接受 HTTP/HTTPS 且没有非环回 HTTP 禁令；`deriveWsUrl` 按协议派生 WS/WSS；`apps/desktop/src/main/server-switch.ts:22` 仅请求 `/health` 与 `/api/config` 做服务探测。HTTPS 为建议，API / WebSocket / Web 页面可达性分别说明。 | 通过 |
| D15 | 中英文 `developers/conventions:69`、`:72` | `packages/core/api/client.ts` 的 `getIssue` 校验后直接返回，`schemas.ts` 和 `types/issue.ts` 保留 snake_case；`issues/queries.ts:118` 为 `detail(wsId, id)`。两版均不再宣称统一转 camelCase 或省略工作空间参数。 | 通过 |
| D16 | 中文规范 `:119`、`:213`；英文规范 `:121`、`:215` | 不只检查标题，也检查了前面的产品名词/标识符总则以及后续所有角色、状态规范句。`packages/views/issues/utils/status-label.ts:22` 本地化内置 key、自定义名称保留、未知 key 回退；中文 issues / members locale 与两版标签表一致。CLI 和 API 标识符仍是英文。 | 通过 |
| D17 | `cli.zh.mdx:181`、`:289`、`:296` | `server/cmd/multica/main.go:58`、`:85` 注册 Core 组 approval；`cmd_approval.go` 五个子命令及 flags 与表一致。`DecideAgentApproval` 排除机器凭据并检查管理权限；执行登记额外要求 operator，已经补充。 | 通过，含登记权限补修 |
| D18 | `tutorial.zh.mdx:43`、`:269`、`:284` | `app-sidebar.tsx:175` 的资源导航，以及 `locales/zh-Hans/layout.json` 的资源与连接 / 运行时 / 技能库标签，与当前正文一致。五张旧图独立检查，创建方式已按当前 MethodChooser 修正。 | 通过，含正文与旧图差异补修 |

表中文档简称均位于 `apps/docs/content/docs/`。本次 reviewer 实际修改的是上述五篇 MDX 和本报告；三组先前的其他 MDX 修订保持原样，未写业务代码、测试、图片、任务元数据、spec 或生成包。

## 截图核对

使用本地图片工具独立查看，未修改图片，也未把旧图当作当前应用运行证明：

- `tutorial-runtimes-list.webp`、`tutorial-runtime-detail.webp`：旧英文运行时界面，可见旧导航与运行时信息，正文已给出当前入口。
- `tutorial-skills-list.webp`、`tutorial-skill-detail.webp`：旧英文 Skills 界面；详情仍带 GitHub 导入来源标签，不能据此推断内网可在线导入。
- `tutorial-skill-new-modal.webp`：清晰显示四个选项，未包含 template；背景导航模糊，没有依靠它确认当前分组。正文现在明确该图与当前五项的差异。

## Findings (not fixed)

没有尚未解决的本轮范围内语义问题，也没有需要改业务实现或产品决策才能关闭的 D01–D18 项。

实际类型检查输出包含仓库已有的 pnpm 配置警告：根 `package.json` 下的 `pnpm.onlyBuiltDependencies` 和 `pnpm.overrides` 被当前 pnpm 忽略。命令最终退出 0，没有 TypeScript 错误。此工具配置问题不属于文档修订范围，未修改包配置或 lockfile；若后续调整依赖安装策略，应单独处理。

验证限制：未启动真实智能体、调用真实模型、试验 3 小时断线、操作真实目录、连接 OMP MCP 或验证 ZeroClaw 的持久化及原生 skill 发现。也未登录真实部署、制造 Redis 故障、发送邀请邮件或使用安装版 Desktop 验收 HTTP/TLS/WebSocket。对应边界来自源码阅读，不是运行测量。

## Verification

- **Lint / 静态检查**：docs 的 `package.json` 没有独立 `lint` script；对 baseline 中最终 **19 个源文件**执行 `git diff --check -- <source_paths>`，退出 **0**、无空白错误。没有据此宣称执行过 ESLint。
- **TypeCheck**：在仓库根目录执行 `pnpm --filter @multica/docs typecheck`，实际运行 `tsc --noEmit`，等待进程结束确认退出 **0**。环境为 Node `v22.23.2`、pnpm `10.28.2`。除上述配置警告外，无错误输出。
- **Tests**：本 reviewer 未运行 docs Vitest、Go docs/handler、MDX 编译或构建，按分工交主会话在最终源文件稳定后统一执行。没有新增只断言文案字符串的测试。
- **生成与链接**：未写 `server/internal/docs/content/`，未运行会改共享产物的 generator / mdx / build。已向主会话发出“所有 MDX 编辑完成”信号；整包字节一致性、内部链接 / 锚点 / 图片和 39 页可达性以主会话最终输出为准。
- **Spec 交接**：建议主会话在 docs bundle spec 记录本轮最易再次漂移的契约：内网 URL 支持与网络可达性分开、认证按模式说明、离线显示与 task 回收分开、原地目录锁的对话例外、当前入口与旧截图分别表述。reviewer 未越过分工修改 spec。

所有 MDX 已稳定；本报告不把源码复核通过表述为整个 Trellis 任务已经验收完成。
