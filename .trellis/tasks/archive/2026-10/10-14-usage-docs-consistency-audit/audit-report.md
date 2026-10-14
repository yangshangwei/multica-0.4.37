# 使用说明与当前代码一致性审计

> 阶段 A 的修订前历史记录。后续 D01–D18 已完成修订与验证，当前结果见 [修订结果](remediation-report.md)。

日期：2026-10-14。范围：应用内发布的全部 39 篇中文文档及其主要操作和契约。

**结论：不完全一致。文档源与应用内嵌入包保持同步，但正文存在 18 项需要修订的问题，涉及 17 篇源文档。**其中 4 项 P1、12 项 P2、2 项 P3，包含事实不符、过时入口和重要边界遗漏。

核查以当前工作区为准，包括本轮之前已有的未提交修改。分支为 `codex/cross-workspace-collaboration`，基线提交为 `e7323b33d4f8b493aaddbb50604eb6b0d54bf006`。这不是对正在运行的服务器或已发布安装包的版本证明。

## 优先修订

| 编号 | 优先级 | 问题 |
| --- | --- | --- |
| D01 | P1 | 内网 URL 导入和更新 skill 的流程没有实现 |
| D02 | P1 | backlog 被误写为所有触发方式的暂停开关 |
| D03 | P1 | 运行时离线被误写为几分钟内结束执行 |
| D04 | P1 | 快速上手和桌面安装仍把邮箱登录当成默认路径 |
| D05 | P2 | 普通成员中的智能体 owner 也能管理自己的环境变量 |
| D06 | P2 | Oh-My-Pi 的 Multica 托管 MCP 已受支持 |
| D07 | P2 | 工具清单和模型例外缺少 ZeroClaw |
| D08 | P2 | 成员说明遗漏分享邀请，并把邮箱邀请的限制泛化 |
| D09 | P2 | @all 不能覆盖同条评论里的显式智能体提及 |
| D10 | P2 | 编辑触发评论会取消旧内容对应的未完成执行 |
| D11 | P2 | 小队页声称显式转交其他智能体时仍自动唤醒队长 |
| D12 | P2 | Redis 限流说明未区分邮箱认证和密码认证 |
| D13 | P2 | 独立工作目录并不覆盖 local_directory 的原地模式 |
| D14 | P2 | Desktop 并没有禁止远程 HTTP 连接 |
| D15 | P2 | API 命名转换和 Query key 示例与代码不符 |
| D16 | P2 | 状态与角色的显示规则已本地化，规范页仍要求英文 |
| D17 | P3 | 号称全部顶层命令的 CLI 目录缺少 approval |
| D18 | P3 | 教程的运行时和技能入口仍沿用旧导航 |

P1 表示会阻断重要操作或造成明显的执行/运维误判；P2 表示具体行为、权限、能力或必要前提不准确；P3 表示可发现的入口、目录或名称过时。

## 问题与代码依据

以下“文档说明”是对原文的摘录或概括；精确原文行同时保存在 `research/findings.json` 的 `source_excerpt` 字段中。

### D01 · P1 · 内网 URL 导入和更新 skill 的流程没有实现

类型：功能承诺不符。

文档说明：

- [skills.zh.mdx:33](/Volumes/artisan/code/2026/multica-0.4.37/apps/docs/content/docs/skills.zh.mdx:33)：从内网可达的 URL 导入 skill 压缩包。
- [skills.zh.mdx:60](/Volumes/artisan/code/2026/multica-0.4.37/apps/docs/content/docs/skills.zh.mdx:60)：来源地址（必须是内网可达地址，例如内网 Git 服务上托管的压缩包）。
- [cli.zh.mdx:150](/Volumes/artisan/code/2026/multica-0.4.37/apps/docs/content/docs/cli.zh.mdx:150)：内网 Git 服务上的 skill 压缩包。

**当前实现：**任意可达的内网 Git/压缩包 URL 不会因为网络可达而获得支持；普通内网域名会在下载前被拒绝。

代码依据：

- [skill.go:995](/Volumes/artisan/code/2026/multica-0.4.37/server/internal/handler/skill.go:995)：detectImportSource 只识别 skills.sh、clawhub.ai、github.com；其他普通内网域名返回 unsupported source。
- [skill_refresh.go:57](/Volumes/artisan/code/2026/multica-0.4.37/server/internal/handler/skill_refresh.go:57)：refresh 只处理既定的三类来源，并复用来源识别。

**影响：**按使用说明执行 URL 导入或后续更新会失败。

**建议修订：**当前版本应指导用户将压缩包下载到本机，使用本地导入或 multica skill import --file；部署模板和运行时复制也可使用。任意内网 URL 支持需要另行实现。

### D02 · P1 · backlog 被误写为所有触发方式的暂停开关

类型：触发边界错误。

文档说明：

- [triggering-agents.zh.mdx:15](/Volumes/artisan/code/2026/multica-0.4.37/apps/docs/content/docs/triggering-agents.zh.mdx:15)：四种方式共用以下规则。
- [triggering-agents.zh.mdx:19](/Volumes/artisan/code/2026/multica-0.4.37/apps/docs/content/docs/triggering-agents.zh.mdx:19)：处于 backlog 的 issue 不会触发执行。

**当前实现：**普通正式任务在 backlog 状态下，显式 @智能体或符合路由条件的回复仍可启动执行；待分拣记录有独立门禁。

代码依据：

- [issue_trigger.go:129](/Volumes/artisan/code/2026/multica-0.4.37/server/internal/service/issue_trigger.go:129)：仅分配和离开 backlog 的状态变更路径检查 backlog。
- [comment.go:2096](/Volumes/artisan/code/2026/multica-0.4.37/server/internal/handler/comment.go:2096)：评论入队检查正式接纳与评论资格，没有 backlog 门禁。
- [comment.go:3170](/Volumes/artisan/code/2026/multica-0.4.37/server/internal/handler/comment.go:3170)：显式提及在权限、归档、运行能力检查后产生触发目标。

**影响：**用户可能在准备中的任务里提及智能体，意外启动本机执行。

**建议修订：**把 backlog 规则限定到分配与状态变更。明确评论触发的例外；需要完全静默的评论使用 /note 或取消触发预览中的目标。

### D03 · P1 · 运行时离线被误写为几分钟内结束执行

类型：超时策略过时。

文档说明：

- [tasks.zh.mdx:155](/Volumes/artisan/code/2026/multica-0.4.37/apps/docs/content/docs/tasks.zh.mdx:155)：失去心跳的运行时最迟约 3 分钟被判为离线，其上的 task 随之失败。
- [tasks.zh.mdx:153](/Volumes/artisan/code/2026/multica-0.4.37/apps/docs/content/docs/tasks.zh.mdx:153)：dispatched 停留超过 5 分钟按失败处理。
- [daemon-runtimes.zh.mdx:64](/Volumes/artisan/code/2026/multica-0.4.37/apps/docs/content/docs/daemon-runtimes.zh.mdx:64)：正在执行的 task 会在运行时被判为离线后失败。

**当前实现：**网络中断导致离线标记后，默认仍保留在途执行 3 小时供重连；真正的 daemon 重启有单独回收路径。有效准备租约也会延缓 dispatched 超时。

代码依据：

- [runtime_sweeper.go:38](/Volumes/artisan/code/2026/multica-0.4.37/server/cmd/server/runtime_sweeper.go:38)：默认重连宽限为 3 小时，与 UI 离线标记分开。
- [runtime.sql:260](/Volumes/artisan/code/2026/multica-0.4.37/server/pkg/db/queries/runtime.sql:260)：在途任务需运行时失联超过重连宽限才进入失败集合。
- [agent.sql:1485](/Volumes/artisan/code/2026/multica-0.4.37/server/pkg/db/queries/agent.sql:1485)：dispatched 超时还要求准备租约缺失/过期，并满足运行时健康或宽限已过。

**影响：**维护者会错误预期几分钟后失败/重试，可能重复派发或把正常保留中的任务当成故障。

**建议修订：**分别说明心跳可见性、重连宽限、准备租约、实际重启回收；引用环境变量页已正确给出的 MULTICA_RUNTIME_RECONNECT_GRACE=3h。

### D04 · P1 · 快速上手和桌面安装仍把邮箱登录当成默认路径

类型：默认操作流程错误。

文档说明：

- [self-host-quickstart.zh.mdx:193](/Volumes/artisan/code/2026/multica-0.4.37/apps/docs/content/docs/self-host-quickstart.zh.mdx:193)：输入邮箱获取验证码。
- [desktop-app.zh.mdx:33](/Volumes/artisan/code/2026/multica-0.4.37/apps/docs/content/docs/desktop-app.zh.mdx:33)：安装后使用和 Web 版相同的邮箱登录。

**当前实现：**默认自托管通过设备身份进入；显式 password 部署使用用户名和密码。只有关闭设备入口等相应配置后才按邮箱验证码流程操作。

代码依据：

- [docker-compose.selfhost.yml:118](/Volumes/artisan/code/2026/multica-0.4.37/docker-compose.selfhost.yml:118)：认证模式默认 legacy。
- [docker-compose.selfhost.yml:137](/Volumes/artisan/code/2026/multica-0.4.37/docker-compose.selfhost.yml:137)：设备认证开关默认留空。
- [auth_device.go:99](/Volumes/artisan/code/2026/multica-0.4.37/server/internal/handler/auth_device.go:99)：开关未设置时，非官方云默认启用设备身份。
- [auth-initializer.tsx:425](/Volumes/artisan/code/2026/multica-0.4.37/packages/core/platform/auth-initializer.tsx:425)：客户端自动执行设备身份登录。
- [password_config.go:10](/Volumes/artisan/code/2026/multica-0.4.37/server/internal/auth/password_config.go:10)：另有 password 模式。

**影响：**新部署用户找不到教程所说的邮箱入口，也可能错误理解清理设备数据后的账号关系。

**建议修订：**按部署认证模式分三条流程，并链接当前已经更新的 auth-setup 页面。

### D05 · P2 · 普通成员中的智能体 owner 也能管理自己的环境变量

类型：权限描述错误。

文档说明：

- [agents-create.zh.mdx:164](/Volumes/artisan/code/2026/multica-0.4.37/apps/docs/content/docs/agents-create.zh.mdx:164)：只有工作空间 owner、admin 可以解锁和修改。
- [cli.zh.mdx:268](/Volumes/artisan/code/2026/multica-0.4.37/apps/docs/content/docs/cli.zh.mdx:268)：env get/set ... 仅 owner 和 admin。

**当前实现：**权限包括智能体的人类 owner，哪怕其工作空间角色只是 member；运行中的智能体 actor 不可读写管理接口。

代码依据：

- [agent_env.go:97](/Volumes/artisan/code/2026/multica-0.4.37/server/internal/handler/agent_env.go:97)：接受工作空间 member 角色后继续判断智能体所有权。
- [agent_env.go:121](/Volumes/artisan/code/2026/multica-0.4.37/server/internal/handler/agent_env.go:121)：工作空间 owner/admin 或智能体的人类 owner 均可管理。
- [agent_env.go:92](/Volumes/artisan/code/2026/multica-0.4.37/server/internal/handler/agent_env.go:92)：智能体 actor 仍被管理接口拒绝。

**影响：**普通成员被误导为无法查看或轮换自己的凭据，文档也低估了实际可读主体。

**建议修订：**写明“智能体的人类 owner，以及工作空间 owner/admin”，保留明文存储与审计说明。

### D06 · P2 · Oh-My-Pi 的 Multica 托管 MCP 已受支持

类型：能力表过时。

文档说明：

- [providers.zh.mdx:40](/Volumes/artisan/code/2026/multica-0.4.37/apps/docs/content/docs/providers.zh.mdx:40)：Oh-My-Pi 的 Multica 管理 MCP 列为 —。

**当前实现：**前端和运行时都支持为 Oh-My-Pi 注入托管 MCP；已有同名配置文件会明确报冲突。

代码依据：

- [mcp-support.ts:29](/Volumes/artisan/code/2026/multica-0.4.37/packages/core/agents/mcp-support.ts:29)：omp 位于 MCP 支持集合。
- [omp_mcp.go:10](/Volumes/artisan/code/2026/multica-0.4.37/server/internal/daemon/execenv/omp_mcp.go:10)：为 omp 写入 .omp/mcp.json。
- [execenv.go:609](/Volumes/artisan/code/2026/multica-0.4.37/server/internal/daemon/execenv/execenv.go:609)：执行准备调用 OMP MCP 配置写入。

**影响：**用户会错误排除可以完成 MCP 工作流的工具。

**建议修订：**将能力表改为支持；如解释文件行为，注明 .omp/mcp.json 的覆盖冲突限制。

### D07 · P2 · 工具清单和模型例外缺少 ZeroClaw

类型：支持范围遗漏。

文档说明：

- [install-agent-runtime.zh.mdx:14](/Volumes/artisan/code/2026/multica-0.4.37/apps/docs/content/docs/install-agent-runtime.zh.mdx:14)：Multica 当前会检测以下命令；表中没有 ZeroClaw。
- [providers.zh.mdx:21](/Volumes/artisan/code/2026/multica-0.4.37/apps/docs/content/docs/providers.zh.mdx:21)：工具能力表没有 ZeroClaw。
- [providers.zh.mdx:57](/Volumes/artisan/code/2026/multica-0.4.37/apps/docs/content/docs/providers.zh.mdx:57)：模型由运行时管理的例外仅列 QwenPaw 和 MiniMax Code。

**当前实现：**ZeroClaw 已是内置支持的运行时，模型由工具配置管理，且有版本与会话能力要求。

代码依据：

- [agents_probe.go:303](/Volumes/artisan/code/2026/multica-0.4.37/server/internal/daemon/agents_probe.go:303)：检测 MULTICA_ZEROCLAW_PATH / zeroclaw。
- [version.go:21](/Volumes/artisan/code/2026/multica-0.4.37/server/pkg/agent/version.go:21)：最低版本 0.8.0。
- [models.go:406](/Volumes/artisan/code/2026/multica-0.4.37/server/pkg/agent/models.go:406)：ZeroClaw 不接受 Multica 侧模型选择。
- [zeroclaw.go:241](/Volumes/artisan/code/2026/multica-0.4.37/server/pkg/agent/zeroclaw.go:241)：不使用 Multica 注入的 MCP，由工具自身管理。

**影响：**界面出现支持列表没有的工具，用户无法解释模型选择不可用及安装版本要求。

**建议修订：**补齐检测命令、最低版本、模型/MCP/会话条件；与环境变量页已有的 ZeroClaw 说明对齐。

### D08 · P2 · 成员说明遗漏分享邀请，并把邮箱邀请的限制泛化

类型：关键入口遗漏。

文档说明：

- [members-roles.zh.mdx:29](/Volumes/artisan/code/2026/multica-0.4.37/apps/docs/content/docs/members-roles.zh.mdx:29)：在设置 → 成员中输入邮箱...发送邀请。
- [members-roles.zh.mdx:35](/Volumes/artisan/code/2026/multica-0.4.37/apps/docs/content/docs/members-roles.zh.mdx:35)：邀请链接只出现在邮件里，产品界面中没有查看或复制它的入口。

**当前实现：**邮箱定向邀请之外，owner/admin 可以在界面创建、查看、复制和撤销分享邀请链接；密码账号通过这种链接加入。

代码依据：

- [members-tab.tsx:312](/Volumes/artisan/code/2026/multica-0.4.37/packages/views/settings/components/members-tab.tsx:312)：ShareLinkRow 展示链接并提供复制。
- [members-tab.tsx:778](/Volumes/artisan/code/2026/multica-0.4.37/packages/views/settings/components/members-tab.tsx:778)：支持创建、撤销与复制分享链接。
- [members-tab.tsx:961](/Volumes/artisan/code/2026/multica-0.4.37/packages/views/settings/components/members-tab.tsx:961)：成员页有分享链接区域。
- [router.go:1787](/Volumes/artisan/code/2026/multica-0.4.37/server/cmd/server/router.go:1787)：注册分享链接创建、列表、撤销 API。

**影响：**无邮件或密码模式部署的管理员可能误认为产品没有可用的邀请途径。

**建议修订：**分别介绍邮箱定向邀请与分享链接，包括角色、有效期和撤销；“无法复制”最多限定到邮件邀请本身。

### D09 · P2 · @all 不能覆盖同条评论里的显式智能体提及

类型：触发边界遗漏。

文档说明：

- [comments.zh.mdx:32](/Volumes/artisan/code/2026/multica-0.4.37/apps/docs/content/docs/comments.zh.mdx:32)：@all ... 不触发任何智能体。

**当前实现：**@all 自身不启动智能体，并阻止隐式兜底；同条评论明确提及的智能体或小队仍会执行。

代码依据：

- [comment.go:2666](/Volumes/artisan/code/2026/multica-0.4.37/server/internal/handler/comment.go:2666)：先解析并返回显式 @agent/@squad，之后才判断 @all。
- [comment_trigger_preview_test.go:1109](/Volumes/artisan/code/2026/multica-0.4.37/server/internal/handler/comment_trigger_preview_test.go:1109)：已有回归用例明确要求 @all 加显式 @agent 仍然入队。

**影响：**将 @all 理解成整条评论的静默开关，会造成意外执行。

**建议修订：**补上“仅抑制隐式路由，不抑制显式 @智能体/@小队”的边界；引用预览和 /note。

### D10 · P2 · 编辑触发评论会取消旧内容对应的未完成执行

类型：重要副作用遗漏。

文档说明：

- [comments.zh.mdx:52](/Volumes/artisan/code/2026/multica-0.4.37/apps/docs/content/docs/comments.zh.mdx:52)：编辑评论后 ... 按新内容重新计算触发。
- [comments.zh.mdx:55](/Volumes/artisan/code/2026/multica-0.4.37/apps/docs/content/docs/comments.zh.mdx:55)：内容有误时编辑即可。

**当前实现：**内容编辑不只是改变显示文本；旧内容关联的在途任务可被取消，再重新组织剩余评论和新内容的执行。

代码依据：

- [comment.go:3381](/Volumes/artisan/code/2026/multica-0.4.37/server/internal/handler/comment.go:3381)：严格内容编辑在事务中取消由原评论触发的执行。
- [agent.sql:754](/Volumes/artisan/code/2026/multica-0.4.37/server/pkg/db/queries/agent.sql:754)：取消 queued、dispatched、running、waiting_local_directory、deferred，包括合并批次包含该评论的任务。
- [comment.go:3433](/Volumes/artisan/code/2026/multica-0.4.37/server/internal/handler/comment.go:3433)：之后恢复其他仍有效评论并按新正文重新计算触发。

**影响：**用户修改错字或补充说明时，可能中断已经运行的工作。

**建议修订：**明确提醒编辑/删除触发评论可能取消旧执行；需要不中断地补充要求时，建议新增评论。

### D11 · P2 · 小队页声称显式转交其他智能体时仍自动唤醒队长

类型：协作路由错误。

文档说明：

- [squads.zh.mdx:94](/Volumes/artisan/code/2026/multica-0.4.37/apps/docs/content/docs/squads.zh.mdx:94)：智能体发结果时顺手 @了另一个智能体，队长仍会被唤醒。

**当前实现：**就这条评论而言，显式路由优先，不会额外唤醒未被提及的原队长。无显式路由的成员结果评论才可能走队长兜底。

代码依据：

- [comment.go:2673](/Volumes/artisan/code/2026/multica-0.4.37/server/internal/handler/comment.go:2673)：存在显式 agent/squad 提及时直接返回相应目标。
- [comment.go:2683](/Volumes/artisan/code/2026/multica-0.4.37/server/internal/handler/comment.go:2683)：队长隐式兜底位于该提前返回之后。
- [comment.go:3204](/Volumes/artisan/code/2026/multica-0.4.37/server/internal/handler/comment.go:3204)：显式提及分支只加入对应目标，不追加原队长。

**影响：**依赖所述例外设计的交接流程，可能等不到队长的预期跟进。

**建议修订：**移除过时例外，解释显式路由与队长兜底的优先级；需要队长参与时明确提及。

### D12 · P2 · Redis 限流说明未区分邮箱认证和密码认证

类型：认证模式范围错误。

文档说明：

- [environment-variables.zh.mdx:157](/Volumes/artisan/code/2026/multica-0.4.37/apps/docs/content/docs/environment-variables.zh.mdx:157)：未设置 Redis ... 认证限流不生效。
- [environment-variables.zh.mdx:169](/Volumes/artisan/code/2026/multica-0.4.37/apps/docs/content/docs/environment-variables.zh.mdx:169)：Redis 临时不可用，认证限流仍选择放行。

**当前实现：**password/single 不依赖 Redis 也有限流；password/shared 的 Redis 失败会拒绝认证，不会放行。文档的描述只能限定在 legacy 邮箱相关路径。

代码依据：

- [password_limiter.go:54](/Volumes/artisan/code/2026/multica-0.4.37/server/internal/auth/password_limiter.go:54)：shared 模式要求 Redis；其他有效模式使用内存 bucket。
- [auth_password.go:33](/Volumes/artisan/code/2026/multica-0.4.37/server/internal/handler/auth_password.go:33)：限流器失败返回 503；超限返回 429。
- [password_config.go:44](/Volumes/artisan/code/2026/multica-0.4.37/server/internal/auth/password_config.go:44)：密码部署必须选择 single 或 shared。

**影响：**部署者会误判密码登录的防护和 Redis 故障时的实际可用性。

**建议修订：**分模式列出限流后端、开关和故障策略，链接 auth-setup 的准确说明。

### D13 · P2 · 独立工作目录并不覆盖 local_directory 的原地模式

类型：隔离条件遗漏。

文档说明：

- [security-model.zh.mdx:38](/Volumes/artisan/code/2026/multica-0.4.37/apps/docs/content/docs/security-model.zh.mdx:38)：每个任务 ... 拥有独立 workdir，并发任务不会在同一份 checkout 上打架。

**当前实现：**普通托管执行与 worktree 执行拥有独立工作目录；local_directory/in_place 直接读写关联目录，串行互斥并不等于独立副本。

代码依据：

- [execenv.go:492](/Volumes/artisan/code/2026/multica-0.4.37/server/internal/daemon/execenv/execenv.go:492)：LocalWorkDir 不为空时直接使用用户路径作为 workDir。
- [project_resource.go:119](/Volumes/artisan/code/2026/multica-0.4.37/server/internal/handler/project_resource.go:119)：in_place 明确直接写用户工作副本，只做按目录串行。

**影响：**只读安全模型页的用户会高估本地目录模式对工作副本的保护。

**建议修订：**为独立 workdir 说明增加适用条件，明确原地模式的例外并链接项目资源页。

### D14 · P2 · Desktop 并没有禁止远程 HTTP 连接

类型：连接条件错误。

文档说明：

- [desktop-app.zh.mdx:152](/Volumes/artisan/code/2026/multica-0.4.37/apps/docs/content/docs/desktop-app.zh.mdx:152)：远程自托管实例未使用 HTTPS ... Desktop 无法建立连接。

**当前实现：**客户端允许可达的 HTTP API 地址并推导 ws；HTTPS 是部署安全建议，未使用它并非一律无法连接的技术条件。

代码依据：

- [runtime-config.ts:167](/Volumes/artisan/code/2026/multica-0.4.37/apps/desktop/src/shared/runtime-config.ts:167)：正式配置解析允许 http 和 https，不限定环回地址。
- [runtime-config.test.ts:84](/Volumes/artisan/code/2026/multica-0.4.37/apps/desktop/src/shared/runtime-config.test.ts:84)：使用非环回 HTTP 地址验证配置。
- [server-switch.ts:22](/Volumes/artisan/code/2026/multica-0.4.37/apps/desktop/src/main/server-switch.ts:22)：连接探测直接请求配置地址的 /health 与 /api/config，没有额外 HTTPS 门禁。

**影响：**内网用户会把可工作的 HTTP 配置误判为客户端不支持，排障方向错误。

**建议修订：**区分安全建议和连接所需条件；保留 HTTPS 建议，准确说明 API 可达性与 WebSocket 配置。

### D15 · P2 · API 命名转换和 Query key 示例与代码不符

类型：开发契约描述错误。

文档说明：

- [conventions.zh.mdx:69](/Volumes/artisan/code/2026/multica-0.4.37/apps/docs/content/docs/developers/conventions.zh.mdx:69)：api client 在边界处转成 camelCase；TS 代码内部一律 camelCase。
- [conventions.zh.mdx:72](/Volumes/artisan/code/2026/multica-0.4.37/apps/docs/content/docs/developers/conventions.zh.mdx:72)：issueKeys.detail(id)。

**当前实现：**API/domain 数据保留 snake_case；本地变量通常用 camelCase。详情查询键必须含工作空间 ID。

代码依据：

- [client.ts:1875](/Volumes/artisan/code/2026/multica-0.4.37/packages/core/api/client.ts:1875)：IssueSchema 解析后直接返回，无键名转换。
- [schemas.ts:1274](/Volumes/artisan/code/2026/multica-0.4.37/packages/core/api/schemas.ts:1274)：schema 保留 workspace_id 等 snake_case 键。
- [issue.ts:161](/Volumes/artisan/code/2026/multica-0.4.37/packages/core/types/issue.ts:161)：内部 Issue 类型也使用 snake_case。
- [queries.ts:118](/Volumes/artisan/code/2026/multica-0.4.37/packages/core/issues/queries.ts:118)：实际调用签名 detail(wsId, id)。

**影响：**照文档读取字段会得到 undefined；复制示例会缺参数并丢失工作空间维度。

**建议修订：**准确区分数据字段与本地变量命名；将 Query key 示例修正为 issueKeys.detail(wsId, id)。

### D16 · P2 · 状态与角色的显示规则已本地化，规范页仍要求英文

类型：开发契约描述错误。

文档说明：

- [conventions.zh.mdx:213](/Volumes/artisan/code/2026/multica-0.4.37/apps/docs/content/docs/developers/conventions.zh.mdx:213)：角色名 + 状态名（小写英文，不翻）。
- [conventions.zh.mdx:220](/Volumes/artisan/code/2026/multica-0.4.37/apps/docs/content/docs/developers/conventions.zh.mdx:220)：UI 里展示这些值时保持英文。

**当前实现：**内部 key 和 CLI 参数保持英文；用户可见的内置状态、角色使用当前界面语言，自定义状态使用用户设置的名字。

代码依据：

- [status-label.ts:22](/Volumes/artisan/code/2026/multica-0.4.37/packages/views/issues/utils/status-label.ts:22)：内置状态使用 i18n。
- [issues.json:86](/Volumes/artisan/code/2026/multica-0.4.37/packages/views/locales/zh-Hans/issues.json:86)：待规划、进行中、审核中等中文标签。
- [members.json:3](/Volumes/artisan/code/2026/multica-0.4.37/packages/views/locales/zh-Hans/members.json:3)：所有者、管理员、成员的中文角色名。

**影响：**遵守旧规范会把已经本地化的界面改回英文，产生新的不一致。

**建议修订：**把标识符与展示标签分开规定，与 issues.zh.mdx:91 的现有说明一致。

### D17 · P3 · 号称全部顶层命令的 CLI 目录缺少 approval

类型：命令目录遗漏。

文档说明：

- [cli.zh.mdx:233](/Volumes/artisan/code/2026/multica-0.4.37/apps/docs/content/docs/cli.zh.mdx:233)：下表覆盖当前全部顶层命令。

**当前实现：**approval 已是顶层命令，但完整目录没有列出它。

代码依据：

- [main.go:85](/Volumes/artisan/code/2026/multica-0.4.37/server/cmd/multica/main.go:85)：注册 approval 顶层命令。
- [cmd_approval.go:29](/Volumes/artisan/code/2026/multica-0.4.37/server/cmd/multica/cmd_approval.go:29)：提供高风险动作审批申请与跟踪。
- [cmd_approval.go:275](/Volumes/artisan/code/2026/multica-0.4.37/server/cmd/multica/cmd_approval.go:275)：注册 request/list/get/executed/cancel 子命令。

**影响：**读者无法从完整参考找到申请和跟踪审批的入口。

**建议修订：**补入 approval 及其子命令，并说明批准/驳回通过人类界面完成。

### D18 · P3 · 教程的运行时和技能入口仍沿用旧导航

类型：教程界面名称过时。

文档说明：

- [tutorial.zh.mdx:43](/Volumes/artisan/code/2026/multica-0.4.37/apps/docs/content/docs/tutorial.zh.mdx:43)：侧边栏底部的 配置 → 运行时。
- [tutorial.zh.mdx:269](/Volumes/artisan/code/2026/multica-0.4.37/apps/docs/content/docs/tutorial.zh.mdx:269)：打开 Skills 页面。

**当前实现：**当前中文界面使用“资源与连接 → 运行时”和“技能库”等入口名称。

代码依据：

- [app-sidebar.tsx:175](/Volumes/artisan/code/2026/multica-0.4.37/packages/views/layout/app-sidebar.tsx:175)：技能、MCP、运行时归入 resourceNav。
- [app-sidebar.tsx:931](/Volumes/artisan/code/2026/multica-0.4.37/packages/views/layout/app-sidebar.tsx:931)：渲染资源分组。
- [layout.json:61](/Volumes/artisan/code/2026/multica-0.4.37/packages/views/locales/zh-Hans/layout.json:61)：当前分组名为资源与连接。

**影响：**新用户难以按原文定位操作入口。

**建议修订：**按当前导航更新教程中的目的地名称，并在修订时人工核对旧截图。

## 逐页覆盖

每页均核查主要可验证说明。表中未列问题表示在该核查范围内没有发现确定性偏差，不代表所有配置组合均完成运行验证。

| 页面 | 核查重点 | 结果 |
| --- | --- | --- |
| [欢迎](/Volumes/artisan/code/2026/multica-0.4.37/apps/docs/content/docs/index.zh.mdx) | 概览入口、文档链接、任务与执行的基础分工 | 未发现额外确定性偏差 |
| [核心概念](/Volumes/artisan/code/2026/multica-0.4.37/apps/docs/content/docs/concepts.zh.mdx) | Issue/task、运行时、小队、项目、自动化的核心关系 | 未发现额外确定性偏差 |
| [Multica 的工作方式](/Volumes/artisan/code/2026/multica-0.4.37/apps/docs/content/docs/how-multica-works.zh.mdx) | API/daemon 分工、排队、结果返回、task 与 Issue 状态分离 | 未发现额外确定性偏差 |
| [完整教程](/Volumes/artisan/code/2026/multica-0.4.37/apps/docs/content/docs/tutorial.zh.mdx) | 创建流程默认模式、仓库、浮动聊天、运行时与技能入口、小队和自动化 | D18 |
| [自托管快速上手](/Volumes/artisan/code/2026/multica-0.4.37/apps/docs/content/docs/self-host-quickstart.zh.mdx) | Compose 初始化、默认登录、健康探针、反向代理、模板挂载、升级命令 | D04 |
| [工作空间](/Volumes/artisan/code/2026/multica-0.4.37/apps/docs/content/docs/workspaces.zh.mdx) | 创建与设置权限、离开/移除的运行时和任务清理、数据范围 | 未发现额外确定性偏差 |
| [成员与权限](/Volumes/artisan/code/2026/multica-0.4.37/apps/docs/content/docs/members-roles.zh.mdx) | 角色管理、最后一名 owner 限制、邮箱邀请、分享邀请入口 | D08 |
| [Issues](/Volumes/artisan/code/2026/multica-0.4.37/apps/docs/content/docs/issues.zh.mdx) | 状态分类与触发、权限、task 生命周期、项目/迭代关系、删除 | 未发现额外确定性偏差 |
| [分拣台](/Volumes/artisan/code/2026/multica-0.4.37/apps/docs/content/docs/triage.zh.mdx) | 默认开关、人类审核、正式接纳门禁、稍后处理、CSV、接纳与执行分离 | 未发现额外确定性偏差 |
| [项目](/Volumes/artisan/code/2026/multica-0.4.37/apps/docs/content/docs/projects.zh.mdx) | 状态、正式任务统计、健康指标、描述修订、验收、删除与小队关联 | 未发现额外确定性偏差 |
| [项目资源](/Volumes/artisan/code/2026/multica-0.4.37/apps/docs/content/docs/project-resources.zh.mdx) | Git 与本地目录、Desktop 文件选择、in_place/worktree、能力门禁与回放限额 | 未发现额外确定性偏差 |
| [迭代管理](/Volumes/artisan/code/2026/multica-0.4.37/apps/docs/content/docs/iterations.zh.mdx) | 手动生命周期、权限、任务加入限制、统计、关闭快照和操作限额 | 未发现额外确定性偏差 |
| [智能体](/Volumes/artisan/code/2026/multica-0.4.37/apps/docs/content/docs/agents.zh.mdx) | Access 与管理权限、自治等级、归档、运行状态与执行的区别 | 未发现额外确定性偏差 |
| [创建和配置智能体](/Volumes/artisan/code/2026/multica-0.4.37/apps/docs/content/docs/agents-create.zh.mdx) | 创建与模板、配置、并发、环境变量和 MCP 权限 | D05 |
| [Skills](/Volumes/artisan/code/2026/multica-0.4.37/apps/docs/content/docs/skills.zh.mdx) | 五类创建入口、内网来源、导入、refresh、模板副本和运行时同步 | D01 |
| [小队](/Volumes/artisan/code/2026/multica-0.4.37/apps/docs/content/docs/squads.zh.mdx) | 模板、小队成员/leader 权限、显式路由与兜底、归档转交 | D11 |
| [让智能体开始工作](/Volumes/artisan/code/2026/multica-0.4.37/apps/docs/content/docs/triggering-agents.zh.mdx) | 四种触发、Access、静默评论与 backlog 边界 | D02 |
| [分配 issue 给智能体](/Volumes/artisan/code/2026/multica-0.4.37/apps/docs/content/docs/assigning-issues.zh.mdx) | 分配确认、不启动选项、backlog 状态变更、Access、小队 leader | 未发现额外确定性偏差 |
| [评论](/Volumes/artisan/code/2026/multica-0.4.37/apps/docs/content/docs/comments.zh.mdx) | 编辑/删除权限、触发重算、旧执行取消、@all 与显式提及 | D09、D10 |
| [在评论中 @智能体](/Volumes/artisan/code/2026/multica-0.4.37/apps/docs/content/docs/mentioning-agents.zh.mdx) | 显式和隐式路由、5 分钟延迟兜底、合并、/note、@all 和 Access | 未发现额外确定性偏差 |
| [与智能体对话](/Volumes/artisan/code/2026/multica-0.4.37/apps/docs/content/docs/chat.zh.mdx) | 私聊所有者读取门禁、发送时重新校验 Access、离线排队、归档和删除 | 未发现额外确定性偏差 |
| [收件箱与订阅](/Volumes/artisan/code/2026/multica-0.4.37/apps/docs/content/docs/inbox.zh.mdx) | 通知类别和偏好、已读/未读、归档、站内通知与执行的区别 | 未发现额外确定性偏差 |
| [执行任务](/Volumes/artisan/code/2026/multica-0.4.37/apps/docs/content/docs/tasks.zh.mdx) | 任务状态、排队、重试、断线宽限、准备租约、执行与 Issue 状态分离 | D03 |
| [守护进程与运行时](/Volumes/artisan/code/2026/multica-0.4.37/apps/docs/content/docs/daemon-runtimes.zh.mdx) | 心跳、排队和离线、并发、运行时共享、profile 配置 | D03 |
| [安装 AI 编程工具](/Volumes/artisan/code/2026/multica-0.4.37/apps/docs/content/docs/install-agent-runtime.zh.mdx) | 内置检测命令、路径覆盖、支持版本与外部账号前提 | D07 |
| [AI 编程工具对照](/Volumes/artisan/code/2026/multica-0.4.37/apps/docs/content/docs/providers.zh.mdx) | 模型选择、OMP MCP、ZeroClaw、工具相关的能力声明 | D06、D07 |
| [自动化](/Volumes/artisan/code/2026/multica-0.4.37/apps/docs/content/docs/autopilots.zh.mdx) | 日报/周报计划、两种输出模式、Webhook JSON/幂等/事件推断、失败暂停阈值 | 未发现额外确定性偏差 |
| [自托管 Git 代码托管](/Volumes/artisan/code/2026/multica-0.4.37/apps/docs/content/docs/vcs-integration.zh.mdx) | 提供方、Webhook 签名、PR/MR 状态和关闭意图、代码凭据范围 | 未发现额外确定性偏差 |
| [桌面应用](/Volumes/artisan/code/2026/multica-0.4.37/apps/docs/content/docs/desktop-app.zh.mdx) | 登录方式、独立 profile、正式连接配置、更新 URL、HTTP/HTTPS 连接前提 | D04、D14 |
| [安全模型](/Volumes/artisan/code/2026/multica-0.4.37/apps/docs/content/docs/security-model.zh.mdx) | OS 权限与沙箱说明、task token、工作目录与原地执行的例外 | D13 |
| [环境变量](/Volumes/artisan/code/2026/multica-0.4.37/apps/docs/content/docs/environment-variables.zh.mdx) | 认证/Redis 分支、连接池、重连宽限、Compose 传递、daemon 配置和模板目录 | D12 |
| [登录与注册](/Volumes/artisan/code/2026/multica-0.4.37/apps/docs/content/docs/auth-setup.zh.mdx) | 三种认证入口、密码配置和限制、设备身份、注册规则、TTL/Compose 生效步骤 | 未发现额外确定性偏差 |
| [平台管理后台](/Volumes/artisan/code/2026/multica-0.4.37/apps/docs/content/docs/platform-administration.zh.mdx) | 密码会话门禁、首次管理员、凭据文件、保留天数与显式恢复入口 | 未发现额外确定性偏差 |
| [故障排查](/Volumes/artisan/code/2026/multica-0.4.37/apps/docs/content/docs/troubleshooting.zh.mdx) | 排队/目录等待、并发、健康/WS、日志命令、数据库回填入口 | 未发现额外确定性偏差 |
| [使用 CLI](/Volumes/artisan/code/2026/multica-0.4.37/apps/docs/content/docs/cli.zh.mdx) | 全局入口、issue/agent/skill/autopilot 操作、凭据权限、approval 完整目录 | D01、D05、D17 |
| [认证与令牌](/Volumes/artisan/code/2026/multica-0.4.37/apps/docs/content/docs/auth-tokens.zh.mdx) | cookie/PAT/task token、30/90 天时限、7 天续期阈值、退出与吊销区别 | 未发现额外确定性偏差 |
| [参与开发](/Volumes/artisan/code/2026/multica-0.4.37/apps/docs/content/docs/developers/contributing.zh.mdx) | 版本要求、Make 命令、传统 worktree 路径分配、验证流程与数据库限制 | 未发现额外确定性偏差 |
| [项目架构](/Volumes/artisan/code/2026/multica-0.4.37/apps/docs/content/docs/developers/architecture.zh.mdx) | 包分层、路由适配、Query/Zustand、API 兼容与 task 链路 | 未发现额外确定性偏差 |
| [开发规范](/Volumes/artisan/code/2026/multica-0.4.37/apps/docs/content/docs/developers/conventions.zh.mdx) | API 字段形状、Query key、角色/状态本地化和规范边界 | D15、D16 |

代码入口清单见 [coverage.json](/Volumes/artisan/code/2026/multica-0.4.37/.trellis/tasks/archive/2026-10/10-14-usage-docs-consistency-audit/research/coverage.json)。

## 已验证一致的部分

- 中文 MDX 与嵌入包：39 页、58 个资源，共 98 个文件逐字节一致；目录与源文件对应。
- 内部引用：237 个内部链接、35 个锚点、57 次图片引用未发现缺页、断锚或缺图。检查基于 Markdown 正文与生成 TOC，排除了代码块。
- 当前 `auth-setup` 已正确描述三类认证入口、密码限流配置、设备身份不受邮箱白名单限制，以及 `AUTH_TOKEN_TTL` 在 Compose 中需要显式传递。
- 分拣台的人类审核、普通接纳不启动执行、CSV 行数限制与正式任务边界，和当前实现对应。
- 迭代采用手动生命周期，普通人类成员可维护、设置由管理员控制；取消不计入有效范围，关闭后的事实保留为快照。
- 项目统计排除待分拣记录，完成和取消分开计数；删除项目保留业务任务，删除权限限制为 owner/admin。
- 直接 Chat 的会话所有权、发送时的 Access 校验及离线排队；自动化的每日/每周默认计划、Webhook JSON/幂等和 7 天/50 次/90% 的失败暂停门槛。
- PAT 默认相关时限、7 天续期阈值、24 小时 task token，以及退出登录和 PAT 吊销的区别。

## 验证记录

| 检查 | 实际结果 |
| --- | --- |
| `pnpm --filter @multica/docs test` | 9 个测试文件、62 个测试通过 |
| `go test ./internal/docs/...` | 通过，包含嵌入包和 CLI/服务端依赖范围检查 |
| 临时目录生成并比较嵌入包 | 98 个文件，0 差异；没有覆盖仓库中的生成包 |
| 内部链接、锚点和图片引用 | 0 缺页、0 断锚、0 缺图 |
| 报告引用与覆盖清单 | 所有文件/行号有效，39 页无遗漏或额外页面 |

证据文件：[test-results.json](/Volumes/artisan/code/2026/multica-0.4.37/.trellis/tasks/archive/2026-10/10-14-usage-docs-consistency-audit/research/test-results.json)、[bundle-and-links.json](/Volumes/artisan/code/2026/multica-0.4.37/.trellis/tasks/archive/2026-10/10-14-usage-docs-consistency-audit/research/bundle-and-links.json)、[source-baseline.json](/Volumes/artisan/code/2026/multica-0.4.37/.trellis/tasks/archive/2026-10/10-14-usage-docs-consistency-audit/research/source-baseline.json)、[final-verification.json](/Volumes/artisan/code/2026/multica-0.4.37/.trellis/tasks/archive/2026-10/10-14-usage-docs-consistency-audit/research/final-verification.json)。

## 局限与需要区分的情况

- 审计期间其他会话继续修改设置页和 API 客户端。主会话复查了与本报告有关的修改，并重新定位成员分享链接、API 解析和工作空间退出的引用位置；18 项结论保持不变。当前源码文件哈希已写入最终证据，后续并行修改可能使行号再次变化。
- 本轮为静态审计及文档相关测试。未启动应用、数据库、真实 AI 工具或外部服务；未执行浏览器/桌面操作验收、数据库 handler 集成测试、外部链接可达性检查或全部供应商安装步骤。图片文件存在性已检查，截图里的旧界面未逐张核对。
- 文档中的 Windows Defender“误报”判断无法仅凭代码确认；恢复隔离文件或安全排除项不属于本轮执行范围。
- `developers/architecture` 要求导航型操作等待服务端确认；`workspace-tab.tsx:294` 的“离开”仍先导航。根 `CLAUDE.md` 已将它标为已知例外，不能为迎合存量代码而放宽总体规范。本轮没有把它计作应修改规范的确定性错误。
- 未对跨账号协作新增功能做多账号运行验收；本报告针对现有文档的描述，不将未覆盖新功能的所有细节都计作缺陷。
- 子代理的部分任务因调用限流及模型服务不可用而中断，主会话已接手对应范围并复核确认问题；没有将失败的子代理任务当作独立审核通过的证据。

## 修订安排建议

先修 D01–D04：它们影响按文档完成工作及对执行状态的判断。随后统一权限、触发路由、认证模式、工作目录条件与能力表，最后处理 CLI 目录和教程入口。

修订应以当前功能为边界。D01 若希望保留“任意内网 URL 导入”的承诺，需要另行确认和实现功能；本轮没有扩大到该实现。修改中文正文后应同步重新生成嵌入包并运行文档校验。

本轮新增的只有本任务目录内的规划、研究和报告文件，没有修改业务代码、现有 MDX 正文或嵌入包。Trellis 任务保持 planning；文档修复和产品变更尚未启动。
