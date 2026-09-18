# 私有 GitLab 登录、个人授权与 MR 归属：设计文档

> 状态：**暂缓，未实施**。2026-09-13 按用户要求保存设计，等待以后明确恢复推动。
> 需求与业务边界见[需求文档](2026-09-13-gitlab-personal-mr-requirements.md)。本文包含完整技术方案、验收标准和恢复步骤，不要求读者访问原聊天或隐藏工作目录。
> 整理时分支 `main`，提交 `23d771ca77851de35265db5d269bf2d1034e6a43`。源码定位记录本次调研基线，恢复工作时须重新核对当前实现与部署版本。

目标：本人经过验证并授权发起的运行，其新建 GitLab MR 的真实 `author.id` 必须等于本人绑定的 GitLab 数字用户 ID。

复用现有 `user`、工作区成员和任务归因。先提供私有 GitLab 身份登录，再单独授予个人 Git/MR 操作权限；工作区集成、个人授权与运行凭据各自承担明确职责。

部署选择尚未确定：每人独立运行环境 A 和共享环境中的受控执行 B 均保留；只有共享运行成为目标时才条件推荐 B。

阅读顺序：先看第 3–6 节的身份与迁移边界，再看第 7–8 节的执行约束；页面入口在第 13 节，恢复清单在第 15 节。

## 1. 当前事实与修订依据

以下为当前源码事实；后文的新增字段、入口和策略均为建议设计，不表示已经实现。

| 当前事实 | 证据 |
| --- | --- |
| 工作区 VCS 连接按 workspace + instance 唯一，重连覆盖 token、账号名和连接人；不是个人授权表。 | `server/pkg/db/queries/vcs.sql:14–30` |
| 连接管理校验 PAT、加密保存 PAT 与 webhook secret；`connected_by_id` 记录操作成员。 | `server/internal/handler/vcs.go:153–156,194–233` |
| `MULTICA_VCS_SECRET_KEY` 对应服务端加密能力，缺少时连接接口拒绝；不是张三的 GitLab 身份。 | `server/internal/handler/vcs.go:54,80–100,167–169` |
| GitLab token 校验访问 `/api/v4/user`，目前仅解析 username，未保存数字 subject。 | `server/internal/integrations/vcs/gitlab.go:205–236` |
| 设备登录用设备 ID 派生合成 email，查找/创建 user，然后签发与普通登录相同的 JWT。 | `server/internal/handler/auth_device.go:163–165,227–235,286–304` |
| 浏览器设备 ID 存 localStorage，桌面存 `device-identity.json`；源码明确它不是秘密。 | `apps/web/features/auth/device-identity.ts:4–15,61–79`；`apps/desktop/src/main/device-identity.ts:9–12,66–68` |
| JWT 仅有 sub/email/name/exp/iat；验证链没有登录方法或会话撤销版本判断。 | `server/internal/handler/auth.go:152–164`；`server/internal/middleware/auth.go:229–264` |
| 验证码登录包含开发固定验证码分支，不能因进入某个登录 handler 就视为完成真实身份验证。 | `server/internal/handler/auth.go:123–137,393` |
| CLI-token 从当前 user 再签发 JWT；PAT 创建仅拒绝 machine credential，未区分设备/已验证会话。 | `server/internal/handler/auth.go:659–694`；`server/internal/handler/personal_access_token.go:60–110` |
| 通用 human guard 只把 task_token、cloud_pat 当机器；未知来源被视为 human-equivalent。 | `server/internal/handler/actor_guards.go:106–119` |
| mat_ 绑定 task/agent/workspace，但其 UserID 目前来自 runtime owner；不能用 X-User-ID 选择发起人的个人 token。 | `server/internal/handler/daemon.go:1748–1755`；`server/internal/middleware/auth.go:77–114` |
| 当前 task token 创建记录没有 claim generation；旧 mat_ 不能单独证明调用属于当前 attempt/claim。 | `server/pkg/db/queries/task_token.sql:1–8` |
| 任务已有 originator、accountable、source、delegation、evidence；有效 originator 必须等于 accountable，审计 fallback 可以无 originator。 | `server/internal/attribution/attribution.go:147–191`；`server/internal/service/task.go:1258–1267` |
| 评论与 agent 子任务已有真人链路；quick-create 已记录 requester 并盖章 direct_human。 | `server/internal/service/task.go:464–481,573–583,1597–1603,1630–1660` |
| 直接人工操作/手动 rerun 优先归属于此次 actor；系统 retry 继承父任务原归因。 | `server/internal/service/task.go:504–512`；`server/pkg/db/queries/agent.sql:572–577,630–635` |
| queued 评论合并会原子改写成最新评论的整套归因，且当前只有一个 issue/agent pending 槽位。 | `server/internal/handler/comment.go:2360–2412`；`server/pkg/db/queries/agent.sql:1875–1878,1893–1908` |
| 自动化已有 immutable trigger.created_by 主体解析，每次 dispatch 检查当前 membership；rule_owner 只用于审计。 | `server/internal/service/task.go:620–687` |
| Composio 实际取 agent owner 的连接及 session；originator 参数仅保留审计，不能照搬为个人 GitLab 能力。 | `server/internal/integrations/composio/dispatch.go:53–68,86–125` |
| Git clone/fetch 的环境继承整个 daemon 环境与 credential helper；cache 以 workspace + repo URL 分区。 | `server/internal/daemon/repocache/cache.go:24–55,74–80,359–391,417–418` |
| agent 保留 daemon 的 HOME/XDG；custom_env 属于 agent 配置，不是逐次发起人的身份凭据。 | `server/internal/daemon/daemon.go:7592–7599,7623–7632` |
| MR 镜像有乱序 webhook 防回退；现有展示作者字段是 login/avatar。 | `server/pkg/db/queries/vcs.sql:68–100` |
| GitLab webhook 当前把顶层 user（事件操作者）写成 MR author，未读 object_attributes.author_id；他人操作可能污染作者显示。 | `server/internal/integrations/vcs/gitlab.go:49–73,95–96`；`server/internal/handler/vcs_webhook.go:164–176` |
| 现有 MR 侧栏显示仍受 GitHub sidebar 设置控制，是独立产品缺口。 | `packages/views/issues/components/issue-detail.tsx:2553–2566` |

外部依据：[GitLab OAuth](https://docs.gitlab.com/api/oauth2/)、[OAuth scopes](https://docs.gitlab.com/integration/oauth_provider/)、[MR API](https://docs.gitlab.com/api/merge_requests/)、[push options](https://docs.gitlab.com/topics/git/commit/#push-options)、[MR webhook](https://docs.gitlab.com/user/project/integrations/webhook_events/#merge-request-events)。
OAuth 建议 Authorization Code + PKCE；`read_user` 用于身份读取，`write_repository` 用于 Git over HTTP，REST 创建 MR 需要更宽的 `api`。
刷新会作废旧 access/refresh token；新 MR 作者取创建请求的认证用户，更新 MR 的常规 API 不提供修改原作者的能力。
必须在目标 GitLab 版本的受控测试环境确认 scopes、refresh、push options 和作者回包，不能凭未验证版本差异承诺上线。

## 2. 设计原则与方案取舍

原则：

1. 真人身份、工作区管理权、第三方个人授权分别建模；设备 ID 只表示客户端。
2. 使用已有用户主键和归因链；权限依据可验证授权记录，不能从责任人标签或显示名推断。
3. 一个产生 GitLab 副作用的运行只能有一个冻结主体；权限只能收窄，不能随评论、重试、设备或 runtime owner 改换。
4. clone/fetch/push/MR 都必须通过同一主体约束；未知、过期、撤销、主体不匹配时拒绝，不回落到共享 PAT/bot/其他人。
5. 外部副作用可追溯、可去重、可对账；历史 MR 作者和历史审计不伪装、不改写。

前三决策驱动因素：

1. “张三发起 → 张三 GitLab 作者”必须能在双人并发、失败重试及最终选定的部署方式下得到同一结果。
2. 尽量保留既有用户、成员权限、任务历史、webhook/镜像能力，避免再造身份体系。
3. 平衡个人凭据暴露面与部署成本；共享运行环境不能靠进程环境变量承担安全隔离。

| 可行方案 | 优点 | 代价与成立条件 |
| --- | --- | --- |
| A：每人独立 OS 用户/容器/VM，个人运行环境只挂本人凭据；任务严格路由到本人环境。 | 可以沿用多数 Git/CLI 工作流；凭据可留在个人机器；中央服务不集中托管所有人的 refresh token。 | 每人环境维护、调度和离线恢复成本较高；必须隔离文件、进程、agent 会话与网络出口，锁定实际 GitLab 账号；共享 HOME 或只换 env 不成立。 |
| B：服务端保管个人 OAuth，可信 VCS 执行服务完成受控 clone/fetch/push/MR；agent 只提交结构化操作与产物。 | 适配共享 daemon；集中做撤销、权限检查、作者核对及幂等；个人 token 不进入模型和通用 shell。 | 需要 Git 数据通道、受限操作 API、独立可信执行边界；服务端凭据库更敏感；agent 直连 GitLab/其他 VCS MCP 必须不可绕过该边界。 |

若用户选择共享运行，推荐 B：把个人凭据与受控外部操作集中在可信服务，降低多个 CLI/MCP 各自选身份的歧义；它不是 token 注入改名。
若选择每人独立运行，A 可直接作为正式方案，凭据留在个人环境；每个环境仍需执行同样的身份核验、冻结授权和操作账本。
A/B 共用身份登录、个人同意、授权范围、claim 代次、真实 MR 作者与幂等契约，差异在凭据驻留位置和 Git 操作部署边界。
如果 A/B 所选部署都不能证明必要隔离，就关闭个人 VCS 能力；任意宿主机 shell 或一人一套 env 本身不构成 A。
B 的强反方观点：Git 服务扩大控制面、集中凭据和代码处理，运维成本可能高于小团队为每人分配独立运行机器；应以目标部署验证后取舍。
不采用“给共享 agent 配一个 PAT”“设置 git user.email”“仅注入 GITLAB_TOKEN”：这些既不能完整约束身份，也不能阻止环境/工具回落。

## 3. 原方案保留与必须调整

保留工作区 GitLab 实例接入、webhook secret、MR/CI 镜像、问题关联、已有成员/项目权限；其现状见 `vcs.go:153–233`、`vcs.sql:68–100`。
把“设置 → 集成”解释为工作区连接管理；新增“我的 GitLab 授权”，仅本人可连接、查看主体和撤销，不向管理员展示 token。
工作区连接 PAT 只承担原有集成职责，不能用于任何个人运行的 clone/fetch/push/MR 创建；`connected_by_id` 不能变成全员代操作授权。
把“agent 能访问 GitLab”改为“这次运行被某个已验证成员授权访问指定项目”；更换 agent/daemon 不更换真人。
把原来的“注入个人环境变量”改为 A 的真实隔离或 B 的凭据不出可信服务；先补齐全链路，再开放个人 MR 创建。
把“登录后即可关联”改为可信真人重新认证后绑定；禁止设备会话和旧会话借换发升级为个人权限。
把私有 GitLab 身份登录列为 P1 必交付，不假设客户已配置邮件/Google 登录；只读身份登录成功不自动授予 Git/MR grant。
把“沿用 task initiator”细化为复用完整归因、识别审计 fallback、冻结运行授权；不另建一套可漂移的归因瀑布。
把“重试 MR 创建”改为具有持久操作记录的恢复；把“由后来接手者接管 MR 作者”改为保留作者、记录新操作者或另建 MR。
作者镜像修正升为本需求必选：webhook actor 与 MR author 分开保存，缺真实 author 时从创建回包/详情 API 核实；更丰富的 CI 展示仍可后置。
侧栏开关拆分到独立 UI 修订；它影响可见性，不构成个人身份安全的前提。

## 4. 身份与授权边界及建议数据契约

| 边界 | 授权来源 | 允许的职责 |
| --- | --- | --- |
| Multica 登录身份 | P1 私有 GitLab OAuth 身份证明 + 已有 user.id | 只用 read_user/等效最小 profile scope 识别主体并套用既有成员关系；登录本身不授予 Git/MR 操作。 |
| 个人 GitLab 操作授权 | 本人另行交互 OAuth 同意，校验同一 `/user.id` | 授予 Git/MR scope、有效期和撤销状态；管理员不能替成员授权或选择账号。 |
| 工作区 GitLab 连接 | 现有 workspace owner/admin 配置 | 实例/项目允许范围、webhook、镜像；不授予成员个人代操作权。 |
| 运行 VCS 授权 | 已验证成员的操作或明确武装的自动化 + 上述交集 | 在指定 workspace/project/branch/operation 上使用冻结主体。 |

建议个人身份唯一键为 `(server_registered_instance_id, gitlab_numeric_user_id)`；用户名/email 仅显示，可变更，不用于合并。
每个 `(user.id, instance_id)` 只允许一个当前个人绑定；同一外部身份不能静默绑定多个本地用户，冲突进入受控认领。
建议单独记录 binding、grant、run authorization、VCS operation；均引用已有用户/任务/工作区，不复制用户资料作为权限来源。
身份登录凭据与个人操作 grant 分开存储、撤销和审计；禁止把 read_user 登录 token 当 Git 写能力，禁止操作授权回调覆盖登录主体。
个人 grant 元数据保存 scopes、expires_at、binding/security epoch、refresh generation、状态；A/B 各自在个人凭据库/服务端凭据库加密保存 token，运行只保存引用和身份快照。
grant 管理入口使用已知真人来源 allowlist，并要求近期交互认证；不能沿用现有“未知来源也算 human”的 permissive guard。
OAuth state 一次性绑定 session/user、instance、callback 与操作目的；校验 PKCE、严格 redirect、有效期及重放；回调不能任意指定 user.id。
实例 URL 由管理员注册并规范化；限制授权/token/API 的目标与重定向，防止将凭据发往用户提交的另一地址，同时支持明确批准的内网 GitLab。
P1 的 GitLab 登录按已核实外部 subject 找本地映射；未映射用户走现有新用户/邀请规则，不能凭 email 自动领取设备账户或工作区权限。
已有正式账号可在可信近期认证后显式关联；旧设备账号的对应关系必须走下一节的受控认领，不能把 device session 当关联证明。
个人连接状态走 React Query；共享 UI 在 views，平台回调分别接 web/desktop；继续遵守 `CLAUDE.md:30–65` 的状态及边界规则。

## 5. 设备用户迁移与令牌衍生链

迁移目标是让真实员工接续原有业务身份，不把“知道某个设备 ID”当作证明；保留业务历史与审计原貌。
先盘点用户、成员关系、owner/admin、设备账号、有效凭据、未完成任务；产出可审核映射，不按姓名/email/设备 UID 自动合并。
首个旧 owner 映射须同时满足可信部署管理渠道对归属的确认和本人私有 GitLab OAuth 证明，并记录两份证据；device-only 管理员不能独自批准升级。
受信部署管理渠道是已有部署管理员控制的管理入口/恢复流程，不来自设备会话自报角色；本方案信任该管理渠道，不新增 root 对抗目标。
先实际完成新 GitLab 登录、验证 owner 管理操作及恢复路径，再撤销旧凭据；切换过程保持个人 VCS 关闭，避免权限空窗或管理员锁死。
切换事务停用旧设备映射/凭据 epoch，并为已验证真人签发当前 epoch 会话；切换后再次验证登录和 owner 操作，不能把唯一有效管理者一并锁出。
其他原设备 UID 可经可信真人证明及已验证管理者核对历史归属后保留；设备会话只能辅助定位，不能独自批准自身或其他人的认领。
若已有另一正式 user.id，优先保留两条历史记录与明确映射；必要的成员/所有权迁移作为单独事务和审计事件，不批量重写历史作者。
新增会话 auth_method、assurance、auth_time、session/root credential ID、user security epoch；服务端保存并验证状态，不能信任客户端自报 header。
assurance 由实际验证结果生成：有效 OAuth subject/校验链或确实验证过的其他真人凭据；开发固定验证码及测试旁路始终低 assurance，不按 handler 名一概升权。
所有缺少这些证据的旧 JWT 一律作为 legacy；即使对应 user 后来 verified，也不能自动获得个人 VCS 权限。
新设备会话始终低 assurance；禁止其创建个人 grant、批准 VCS run，以及兑换/续期成高 assurance JWT 或 PAT。
迁移盘点覆盖 device/legacy JWT → `/cli-token` JWT → mul_ PAT，以及由其注册/领取的 daemon、mat_、remote MCP 凭据；不能遗漏间接存活的能力。
真人凭据换发只继承、不提升其根的 assurance/epoch；只有新的可信交互认证能建立高 assurance 根，不允许普通 token 刷新冒充重新登录。
分别记录 human authorization root 与 runtime transport credential root：前者证明本人同意，后者证明机器可领取/运输任务；runtime owner 的 assurance 不决定任务发起人的权限。
历史凭据缺少 lineage 时按用户/安全 epoch 保守失效并要求重新认证；含旧 cookie、CLI JWT、PAT、缓存、daemon/task/远程能力和未完成授权。
撤销必须使旧设备重新登录也无法恢复已认领身份的高权限；移除/停用旧设备映射，设备 ID 后续最多作为客户端标签。
迁移切换时暂停旧运行的 GitLab 副作用；没有冻结 grant 的旧任务不因重试获得新用户权限，需重新授权为明确的新运行。
PAT/会话缓存命中仍校验 epoch 与撤销；不能只删数据库行却容忍高权限缓存继续访问。
回退策略只关闭新个人 VCS 功能并保留现有会话/历史可诊断状态；不恢复已经撤销的设备权限或把共享 token 再开放。

## 6. 运行主体、合并、重试与自动化

候选真人从已有 attribution.Result.UserID 解析；AccountableUserID、Source.Precise() 和显示 Initiator 都不能单独发放个人权限。
原因：现有 rule_owner 可被标为 precise 但无授权主体，见 `server/internal/attribution/attribution.go:64–88,149–159`；业务归因不是 OAuth 同意。
对个人 VCS 运行，在入队事务冻结 user、instance/subject、grant ID/epoch、根授权事件、workspace、目标范围；quick-create 未定 repo 时只允许后续收窄。
首次 clone 之前必须有完整 repo 范围；冻结的是身份和授权边界，不是某一代 access token，正常 refresh 可继续使用同一主体的新 token。
P3 新增专用 VCS capability，绑定 audience、run authorization、task ID、claim generation、runtime/lease ID、grant epoch 和到期时间；不能把现有 mat_ 当成已有此保证。
claim generation 由服务端在每次 claim/reclaim 时递增；capability 每次调用都对照数据库当前代次、活跃 task/lease 与授权，reclaim/terminal/revoke 后旧 capability 无效。
claim、每次外部操作、子任务和重试均核对当前 membership、项目权限、grant/epoch 与上述 capability；授权快照不意味着权限永久有效。
保持 originator/accountable 的现有一致性；个人 VCS 任务禁止跨主体重写其归因，已有 evidence 和新 grant 引用一起用于追溯。
同一冻结主体、相同 repo 范围的 queued 评论可合并；不同用户/不同 GitLab subject/不同 grant epoch 的评论必须延后形成独立授权运行。
考虑已有单 pending 槽位：用持久待处理事件及 completion reconcile 延后，不能直接插入第二个 queued task，也不能丢弃评论或返回伪成功。
上述约束同时覆盖 pre-claim merge、active planned-comment 注册及 completion reconcile；竞态路径见 `agent.sql:1842–1938`、`comment.go:2439–2445`。
其他成员的评论可作为上下文，但不能悄悄扩大当前运行的 VCS 权限；跨主体接手必须有新的授权事件。
系统 retry 保留同一冻结主体与逻辑操作 ID，每个 attempt 获取新执行 lease；禁止继承旧会话中的 token/credential helper。
人工 rerun 按当前点击者形成新的授权运行；如果乙接续甲的代码，push actor 可以是乙，但既有 MR 原作者仍是甲。
子任务只可继承父任务同一主体并收窄 scope；不因 agent owner、被提及成员、assignee 或 worker runtime 改换个人 token。
自动化可复用 `ResolveAutopilotTriggerPrincipal`，但已有 trigger_owner 的 invoke 权不等于个人 GitLab 授权，必须由本人另行明确启用项目/操作范围。
定时/webhook 运行每次检查创建者身份、membership、个人 grant、授权截止时间；编辑 cron 的其他成员不会接替主体。
没有可验证真人主体或有效个人授权时进入 authorization_required；保留本地产物/说明，禁止 clone 私有库、push 或创建 MR，不能转用 bot/机器 owner。

## 7. clone → fetch → commit → push → MR 的执行约束

以下具体描述 B；A 在本人环境内实现同等受控执行契约。可信边界是身份服务、凭据库和 VCS 执行器；通用 agent、仓库内容、shell、扩展/MCP 都视为不可信调用方。
信任受管服务端和宿主管理员，不扩大为对抗服务器 root；但不同用户的任务不能互读数据/凭据，旧 daemon 未具备所需协议和隔离能力时拒绝个人模式。
现有 mat_ 仅证明绑定的 task/agent/workspace/user；个人 VCS 调用必须附加 P3 的当前 claim capability，由服务端读取冻结授权，绝不用 runtime owner 选择个人 token。
执行服务提供结构化的受限 Git/MR 操作，不提供任意 shell、任意 API URL、sudo/impersonation、任意 git config 或 token 导出。
clone/fetch 在可信侧使用冻结主体的个人凭据；项目、remote、HTTPS 目标和重定向均服务端校验，submodule/LFS 也按相同规则逐项授权。
可信 Git 进程使用清空后显式构建的环境和隔离配置；禁宿主 credential helper、SSH agent、URL rewrite、extraHeader、任意 hooks、全局配置和 askpass 回落。
不把 token 放在 remote URL、日志、进程参数、prompt、产物或可由 agent 读取的配置；临时凭据通道归可信执行服务所有。
代码通过受限产物通道进入任务工作目录；执行服务只导入允许的对象/补丁，不能执行来自仓库或 agent 的 hooks/config/filter/远程命令。
任务隔离至少覆盖 OS 进程/文件权限、HOME/XDG、keychain/SSH socket、broker socket、共享卷、容器控制口及网络出口；仅换 env/CODEX_HOME 不够。
个人 VCS 运行不得借 glab、curl、Git push options、owner 的 Composio GitLab toolkit 或自配 MCP 绕过执行服务；用出口和能力配置约束并做攻击测试。
如果目标 OS/部署无法证明这些限制，B 的共享运行模式保持关闭，采用 A 的独立环境；不能把文档提示当作技术强制。
commit author/committer 属于 Git 元数据，可按本人已验证资料设置；它不证明 push actor，更不能代替 MR API 的认证身份。
push 使用冻结主体且仅允许授权项目/分支；GitLab 自己的仓库/保护分支权限仍是最后一道约束，不新增跨项目提权通道。
推荐受控 REST 创建 MR，便于字段验证与回包对账，但须明示 `api` scope 广度；agent 永远看不到这个 token。
备选受控 `merge_request.create` push option，可用较窄 Git scope，但把建 MR 与 push 耦合，回包、重试和功能覆盖要单独验证；不可同时开放未受控 CLI 路径。
创建前确认 OAuth `/user.id` 与冻结 subject 相等；创建后校验 MR `author.id`、project/source/target/branch，匹配才记录成功。
新镜像保存不可变数字 author ID；username/avatar 必须来自同一 author 的已核实资料；webhook 顶层 user 单独记录为 actor，不能覆盖作者或运行授权主体。
乙继续甲的 MR 时保留甲为作者并记录乙为后续操作者；若业务要求“乙发起的 MR 属于乙”，创建乙的新 MR 并关联历史，不声称能迁移作者。

## 8. 刷新、撤销、幂等与缓存

每个 grant 的 refresh 使用跨实例互斥/lease + CAS generation；单次刷新原子保存 access/refresh/expiry，晚到结果不能覆盖新 token。
远端已旋转 refresh 但响应丢失，或收到响应后本地提交前崩溃，不能靠 CAS 还原秘密；标记 reauthorization_required，暂停运行操作，要求本人重新授权。
只有能证明请求尚未发送时才重试旧 refresh；发送结果不确定时不自动重放旧 refresh，不伪称刷新成功，也不能从其他账号恢复。
撤销递增 binding/security epoch 并封禁新 lease；refresh 回包提交须核对未撤销状态，禁止“刷新成功”把 revoked 改回 active。
本地撤销先落库再调用 GitLab revoke；外部调用失败也保持本地不可用；成员移除、解绑、身份换绑按同一门禁处理。
操作 lease 获取、发送准入与撤销在数据库锁定同一授权门禁行、校验 epoch 并提交，形成统一顺序；预检查成功不能绕过 lease 事务重新校验。
已获得 lease 但尚未提交 send_admitted 的操作不算在途；撤销使这些 lease 失效，lease → revoke → send 必须拒绝发送；撤销后也不能取得新 lease。
send_admitted 是外部执行的线性化点，必须先持久化并短期有效；若此提交早于撤销，该操作算在途，即使网络发送稍晚，撤销回包也须列出并持续对账。
已准入/发送的远端请求可能完成，不能承诺撤回副作用；未准入的请求、旧 claim 的请求和过期 lease 不得以“之前检查过”为由继续。
403/权限不足、主体变化、失效 grant 都停止该操作并保留产物；网络重试不改变主体，也不能把 401 当成换用工作区 PAT 的信号。
逻辑 MR 创建具有持久唯一 operation ID，绑定授权根、repo、source/target branch；重试 attempt 共用，人工新运行有新的授权决策。
发送前记录 intent；并发请求串行；响应丢失时按已记录远端 ID/操作标识及项目分支查询核对作者，不能盲目重发 POST。
查不到且结果仍不确定时维持 reconciliation_pending；零个/多个候选均不能随便择一；恢复操作需证据，禁止以新幂等键掩盖重复。
Git push 使用已知 old/new SHA 与受限 ref 检查做重试对账；身份切换产生新分支/运行，不共用旧人的隐藏会话。
无凭据且不可变的对象 cache 按 `(workspace, instance, project, local user, GitLab subject, grant epoch)` 分区，保留现有 workspace 边界，不因同一个人而跨工作区复用。
分支/读取 scope 若不同，缓存还须按可读范围分区或仅投影已授权对象；不能直接向受限运行挂载包含其他范围的原始 cache。
可变工作树、refs、agent session、配置及凭据通道进一步按 `(run authorization, task, claim generation)` 隔离；同一个人在同库的不同授权运行也不能共用。
reclaim/retry 恢复时核对完整主体、workspace/project、操作范围、grant epoch 和 lineage，重新创建当前 claim 状态；只迁移必要对象/补丁，不复制旧 refs、会话秘密或凭据通道。
未通过当前权限检查不得因已有 cache 而读库；旧 cache 在切换时隔离、失效/清理，不能当作跨成员下载通道。
后续若共享无凭据只读 Git objects，另做访问检查和隔离评审；第一阶段不为节约磁盘引入跨主体共享。

## 9. 分阶段实施与门禁

P0：确认目标 GitLab 版本、登录/操作 scope、A/B 部署选择、隔离能力和迁移主体清单；冻结本文 AC，本轮不触发外部账户操作。
P1：直接交付私有 GitLab OAuth 身份登录，仅 read_user/等效最小 profile scope；映射既有 user.id，并接 web/desktop 登录入口与回调。
P1 同步交付可信会话与迁移门禁；位置 `auth.go:152,393,659`、`auth_device.go:286`、`middleware/auth.go:77,177,229`、`personal_access_token.go:60`、`actor_guards.go:106`、`vcs/gitlab.go:205`。
P1 门禁：可信部署渠道确认旧 owner + 本人 OAuth；新登录和 owner 操作实测成功后才失效 legacy 衍生链；开发固定验证码不能通过该门禁，历史引用不盲合并。
P2：另行交付个人 Git/MR grant 同意与生命周期/API schema；位置 `vcs/gitlab.go:205`、`handler/vcs.go:80`、`packages/core/vcs/queries.ts`、`packages/views/settings/components/integrations-tab.tsx`。
P2 门禁：身份登录不产生操作 grant；双人 subject、scope、refresh/revoke、旋转丢包和换绑冲突通过；工作区连接接口永远不能返回个人秘密。
P3：冻结运行授权及所有调度边界；位置 `service/task.go:504,627,1236,1638`、`handler/comment.go:2374`、`queries/agent.sql:534,1842,1931`、`handler/daemon.go:1748`。
P3 增加 VCS audience/claim generation/runtime lease capability 与双根记录，位置 `server/pkg/db/queries/task_token.sql:1–8` 为现状参考，采用独立 VCS capability 不误改 mat_ 既有语义。
P3 门禁：跨主体合并、retry/rerun/子任务/自动化通过；旧 claim token 在新 claim、terminal/revoke 后被拒绝；旧任务不能隐式升级。
P4：按所选 A/B 边界交付受控 Git/MR、真实隔离、两级缓存/运行分区、持久幂等及作者修正；位置 `daemon/repocache/cache.go:24,359,417`、`daemon/daemon.go:7592`、`integrations/vcs/gitlab.go`、`queries/vcs.sql:68`。
P4 门禁：错误宿主凭据无法旁路、完整链路身份一致、同人跨 workspace/scope 不串状态；撤销/lease/发送竞态及 author/actor webhook 测试通过。
P5：个人授权/执行人/MR 作者 UI、迁移演练、观测与发布开关；沿用 core → views → web/desktop wiring；CI 丰富化和 `issue-detail.tsx:2556` 的侧栏开关另行后置。
数据库设计遵守无 FK/级联、每个索引单独 `CREATE [UNIQUE] INDEX CONCURRENTLY` 迁移；见 `CLAUDE.md:114–116`，通过 sqlc 生成，不手改 generated。
阶段逐项验证后推进；本文件仅提供规划，不表示现在授权实施或运行上述迁移。

## 10. 可测验收标准

AC1：张三/李四在 A 的独立环境或 B 的共享部署同时执行，同一项目各建一个 MR；GitLab author.id 和 push actor 分别匹配各自冻结 subject。
AC2：只有设备 ID、旧 JWT、旧 JWT 换发的新 CLI JWT/PAT，以及缓存命中/伪造 header，均不能绑定或调用个人 VCS；无外部请求。
AC3：旧 owner 认领须可信部署渠道确认和本人 OAuth，device-only 管理员/固定验证码单独操作必失败；切换前后新登录与 owner 操作都成功，旧凭据失效，历史 issue/评论引用不变。
AC4：张三 queued run 遇李四新评论，冻结主体不变；李四事件持久延后为独立运行，跨 claim 竞态不丢事件、不误报 coalesced。
AC5：系统 retry 和三层子任务保持张三主体；李四手动 rerun 使用李四新授权；旧 MR 仍显示张三，不伪造新作者。
AC6：宿主预存 owner PAT、Git/SSH helper、glab、GitLab Composio 能力时，个人任务也无法借它们 clone/fetch/push/MR；错误个人 token 只产生拒绝。
AC7：20 个并发操作触发同一过期 grant，只成功推进一代 refresh；并发 revoke 后迟到 refresh 不恢复授权；新 lease 数为零。
AC8：MR 创建成功但回包丢失、worker crash、重复投递并发恢复后最多一个逻辑 MR；不确定结果停留待对账，绝不无证据重建。
AC9：成员被移除/个人授权撤销后新操作全部拒绝，缓存不能绕过；已发送操作可查到明确在途记录及最终结果。
AC10：GitLab 相同用户名在两个实例对应不同主体；改名保留 numeric ID；同 email 不合并，重复绑定冲突不改原用户。
AC11：无人授权的自动化不产生 GitLab 请求；本人显式授权的 trigger 使用其当前有效账号，其他成员改 cron 不替换主体。
AC12：MR 回包 author.id 不匹配时不报告成功，留下告警和远端结果记录；不尝试修改 author 来掩盖错误。
AC13：张三创建 MR 后，李四 review/merge/close 的 webhook 和乱序重放都不改变本地张三 author；事件 actor 正确显示李四，旧数据未知作者标为待核实。
AC14：仅配置私有 GitLab 时，P1 可完成 read_user 身份登录并进入原用户工作区；没有额外个人操作同意时 Git/MR grant 不存在，所有个人写操作被拒绝。
AC15：同一人在同库跨两个 workspace，以及两个不同操作/分支 scope 的并发运行，均不共享可变 refs、工作树、session/凭据；cache 命中和恢复也不能扩大任何一方范围。
AC16：同一 task 被 reclaim 为 generation N+1 后，N 的 VCS capability 即使未过期也被拒绝；terminal/revoke 同样失效，伪造 audience/runtime lease 不获得凭据或操作 lease。
AC17：可控并发分别复现 check → revoke → lease 和 lease → revoke → send，两者均不发请求；send_admitted → revoke → 实际网络发送则必须作为撤销前已准入在途操作列账。
AC18：远端旋转 refresh 成功后丢回包，或保存前 crash，系统进入 reauthorization_required，不重放旧 refresh、不恢复 active；本人重新授权后只能形成明确新授权版本。
AC19：设备用户升级后旧 cookie/JWT/PAT 仍被拒绝；重新生成设备 ID 不恢复权限；runtime owner 的认证状态变化不能替任务真人 root 增权。

## 11. 失败预演与验证计划

失败场景 1：上线后张三的 MR 仍属于机器 owner；根因是 clone、glab/SSH、Composio 或缓存旁路漏网；缓解为统一受控操作、真实隔离及 AC1/AC6 攻击矩阵。
失败场景 2：切换设备登录后旧 PAT 仍可使用新绑定，或管理员全部失去登录；根因是把 user.verified 当会话证明、衍生链漏撤销；缓解为 root/epoch、legacy 拒绝、先验证 owner、AC2/AC3。
失败场景 3：评论合并/响应丢失让李四接走张三身份或重复建 MR；根因是可变 originator、重试被当新操作；缓解为冻结主体、持久延后事件、幂等账本和 AC4/AC5/AC8。
Unit：主体资格/授权交集、真实验证 assurance、真人/运输双根、scope/URL、subject 唯一性、claim/audience 校验、两级缓存键及 MR 作者；放对应 Go helper/core schema。
Integration：fake OAuth/GitLab + 数据库覆盖最小身份登录与另行授权、callback 重放、refresh 旋转丢包/保存前 crash、claim 回收、双账号合并/重试、缓存失效、author/actor。
并发验证：以数据库 barrier 明确调度 AC17 的三种顺序，断言 lease/send_admitted/撤销的事务顺序与 HTTP 调用计数，不仅断言最终 status。
隔离验证：AC15 在同人同库跨 workspace/不同 scope 场景实际读取另一工作树、refs、session 和 cache 对象，确认不可见且恢复只迁移准许产物。
E2E：web/desktop 两名成员，按最终选择验证 A 或 B 的完整链路、人工接手、自动化、撤销和 owner 迁移；测试部署不依赖额外邮件/Google 登录，外部真实账户测试另行授权。
Observability：记录真人 root 与运输 root、task/claim generation、audience/runtime lease、instance/subject、grant epoch、operation 状态/准入顺序及 MR ID；不记 token/code/secret URL。
指标：author_mismatch、cross_principal_denied、legacy_denied、stale_claim_denied、refresh_conflict/uncertain、reconciliation_pending、duplicate_prevented；身份不匹配立即告警，pending 超约定窗口告警并禁止盲重试。
实施验证：窄 Go/TS 测试 → go vet/相关 lint 与 typecheck → pnpm test/make test → 目标部署隔离 E2E；是否运行与证据逐阶段记录。
此前只完成源码事实、方案引用和设计审查；上述拟议功能的测试均未执行。已有适配器单元测试的结果见文末，不代表新功能已验证。

## 12. 决策记录与实施前待确认项

Decision：P1 交付私有 GitLab 最小身份登录并安全接续原用户；个人操作 grant 另行授权，运行冻结主体，以当前 claim capability 执行并核验真实 MR author。
Drivers：真实作者可证明、多人并发/恢复正确性、原业务身份和历史保全；共享部署是待定选择，不是既定需求。
Alternatives considered：A 个人环境与 B 服务端受控执行均正式可行，共用身份/授权/作者/幂等契约；共享 PAT、环境注入和 commit 作者伪装不满足约束。
Why chosen：共享运行条件下推荐 B 统一可信执行边界；独立个人部署可选 A 减少中央凭据托管；选择以目标部署隔离验证为准。
Consequences：身份登录与操作同意分离；增加 claim/audience fencing、两级状态隔离和 DB 发送准入；refresh 丢失可能必须重新授权；MR 原作者不可迁移。
Follow-ups：实施前确认 GitLab 版本/scope、A/B 部署路线、员工映射、OS 隔离及对账时限；完成 P0 后再形成对应部署路线的实施任务，本轮不进入执行。
变更记录 v1：明确旧令牌衍生链、跨人合并、runtime/Composio owner、缓存及 author/actor；尚未独立批准。
变更记录 v2：按 Architect REVISE 将私有身份登录提至 P1，限制 owner 认领与实际 assurance，补双根/claim generation、workspace + run/claim 隔离、撤销准入竞态和 refresh 丢失恢复，并将 B 改为条件推荐。
复核记录：Architect 对 v2 返回 ACCEPT，Critic 独立审查返回 APPROVE；19 项 AC、3 项 pre-mortem 与分层验证计划完整。根代理核对 32 处完整路径/起始行引用均有效。没有运行产品测试、认证真实 GitLab 或修改业务代码/配置。

## 13. 登录、授权与任务入口

本节补齐最后一轮产品讨论。以下页面及按钮均是拟新增设计，不是当前界面使用说明。

### 13.1 入口与职责

| 入口 | 谁使用 | 操作与结果 |
| --- | --- | --- |
| 登录页 `/login`：“使用公司 GitLab 登录” | 普通成员 | 跳转到已登记的私有 GitLab 完成身份验证，按稳定外部 subject 找到/建立本地用户，再进入已有工作区或合法上手引导 |
| 设置 → 我的账号 → GitLab 账号（新增菜单） | 已验证本人 | 查看登录身份和个人代码授权，授予、重新授权或撤销代码操作权限 |
| 任务发起/恢复处：“授权 GitLab 并继续” | 本次真实发起人 | 缺少授权时保留需求，在首次私有仓库访问前暂停；完成授权后返回同一操作意图 |
| 设置 → 工作区 → 集成（现有） | owner/admin | 管理实例接入和 Webhook；不展示、代授或导出成员个人 Token |

当前设置页已经区分“我的账号”和“工作区”分组，可沿用其结构，见 `packages/views/settings/components/settings-page.tsx:57,68,217`。
当前 Web 共享登录组件为 `packages/views/auth/login-page.tsx`，Desktop 平台接线为 `apps/desktop/src/renderer/src/pages/login.tsx`；新入口遵守共享逻辑与平台回调的现有边界。

### 13.2 首次登录

1. 管理员预先登记可信实例、OAuth 应用与回调；普通成员不输入服务端密钥，也不提交公共 PAT。
2. 成员从 Multica 登录页进入 GitLab 身份验证；请求只授予身份读取所需的最小 scope。
3. 回调验证 subject、OAuth state、PKCE、有效期和目的，签发正确 assurance 的本地会话。
4. 找到已绑定账号后复用原 `user.id`。未绑定用户遵循邀请/注册规则；旧设备身份走第 5 节的受控认领，不能按 email 自动接管。
5. Desktop 使用系统浏览器完成提供方验证，再通过绑定发起客户端的一次性结果兑换返回应用；URL 中不携带长期 GitLab access/refresh token。

登录成功表示账号已绑定，不自动创建个人代码操作 grant。GitLab 已有登录会话时可以复用，不强制重复输入密码。

### 13.3 个人 GitLab 账号页

| 信息/状态 | 展示及可用操作 |
| --- | --- |
| 登录身份 | 实例、姓名/用户名、已验证状态；内部使用稳定数字 subject 判定，不以显示名认领账号 |
| 未授权代码操作 | “未授权”，提供“授权代码操作” |
| 已授权且可用 | 显示账号、有效状态和 Multica 允许的使用范围，提供管理与撤销 |
| 需要重新授权 | 说明授权已撤销、不能续期或权限不足，提供“重新授权”；不暴露 token 或内部错误堆栈 |
| OAuth 返回另一账号 | 明确提示当前授权账号与登录绑定不一致；不自动更换绑定，不继续原运行 |
| 只有登录身份、没有代码授权 | 仍可按成员权限使用 Multica；受保护的 Git/MR 操作保持不可用 |

身份登录 token 与代码操作 grant 分别记录和处理撤销。撤销代码操作不自动删除成员、任务或历史 MR。

个人绑定属于用户与 GitLab 实例，不因切换工作区复制多份身份；每次运行仍求个人授权与当前工作区/项目策略的交集。

需要诚实说明授权范围：若 REST 创建 MR 使用 GitLab 的较宽 `api` scope，Multica 的项目/操作限制由受控执行层实施，不能把它描述成 GitLab 原生的单项目 OAuth scope。

### 13.4 在任务中补授权

1. 第一次私有仓库 clone/fetch 或缓存访问之前，检查真实发起人、个人 grant 和目标范围。
2. 缺少授权时保留需求/草稿，提供“授权 GitLab 并继续”；其他成员只看到等待授权的状态，不能替本人批准。
3. 服务端保存一次性授权意图，绑定已验证用户、实例、原任务/草稿及请求范围；不能把任意回调中的 task_id 当作授权依据。
4. 授权成功后重新验证同一 subject、意图、成员关系及范围，再创建/继续明确授权的运行；不把设备模式的旧排队运行自动升级。
5. 用户取消、回调失效、账号不符或工作区变化时保留草稿并说明原因，不执行代码操作、不静默换用户或扩大范围。
6. 后续授权仍有效且范围未扩大时直接复用；正常刷新自动处理，只有需要本人重新同意时才中断。

任务页面显示“GitLab 执行账号”，结果处显示经核实的 MR 作者与链接。执行主体、MR 原作者、后续操作人可能不同，不能用一个姓名字段掩盖差异。

## 14. 与原 GitLab 集成能力的关系

原有 VCS 接入负责平台事件镜像。个人执行新增的是账号授权和受控操作链路，两者可以共同使用现有 MR 展示、关联和实时事件。

现有 GitLab 适配器以 Webhook 为主，流水线按 commit 汇总为单一摘要。复杂流水线/合成提交、可合并性、详情补拉等能力仍需另行设计，不能宣称与 GitHub 全量对齐。

现有自动完成规则以 title/body 的关闭关键字及跨提供方 open/draft 聚合为准，普通正文引用不是工作的正式 MR。身份设计不改变这些业务规则，团队完成节点仍待确认。

现有 GitHub 总开关/侧栏开关会隐藏共同的 MR 侧栏，但不等于停用 GitLab 后台关联。当前 VCS “断开”还会删除该连接的镜像/关联/CI 数据；后续若设计保留历史的暂停，不能复用该删除动作。

工作区加密密钥和已有加密工具可以复用，但个人凭据必须独立存储和授权读取。工作区 Token 不用于本人身份运行的兜底。

## 15. 暂缓状态与后续恢复清单

### 已完成的工作

- 对两张设置截图及当前相关源码进行只读调研，区分已有能力、配置缺失和待实现功能。
- 形成并修订个人身份设计；技术稿经过 Architect 复核和 Critic 独立审查，设计层面通过。
- 明确登录页、个人账号菜单和任务内补授权入口；这些入口尚未实现。
- 将需求与设计整理到普通仓库文档目录。

### 未开始的工作

- 没有新增或修改业务代码、数据库结构、页面、OAuth 应用或服务端配置。
- 没有连接、读取或修改用户私有 GitLab 账号与仓库，也没有创建实际 MR。
- 没有运行新方案的迁移、并发隔离、OAuth、真实 GitLab E2E 或发布验证。
- A/B 部署、旧员工映射、版本与 scope、完成节点、授权与对账期限均未最终确定。

### 已有验证的边界

前序调研运行过 `go test ./internal/integrations/vcs -count=1` 并通过，证明的是已有适配器测试，不是本设计的新身份与授权能力。

本次文档检查覆盖需求/设计相互链接、19 项技术 AC、引用路径和记录范围。源码曾发现本地 API 进程无法确认为同一检出，不能把源码调研或旧测试等同于当前私有 GitLab 部署已联调成功。

### 恢复时按顺序进行

1. 从配套需求文档和本文重新建立上下文，确认用户现在确实要继续；文档存在或方案审查通过不自动启动实施。
2. 重新检查当前源码、部署版本、设备登录与会话状态、GitLab 实例和网络，不直接沿用可能已过时的行号或假设。
3. 完成 P0：确定 GitLab 版本/许可与 scope、A/B/OS 选择、旧 owner/成员映射、项目范围、授权有效期和对账时限。
4. 为所选部署路线形成具体实施任务和协议设计；保留 AC1–AC19 对授权主体、并发、迁移和作者的约束。
5. 先验证可信登录和管理者恢复路径，再实施迁移与个人操作；按阶段收集测试证据，不提前开放无法证明隔离的模式。
6. 若业务改为验收/上线才完成，另行确定 MR 合并后的状态规则；不要将身份改动顺手扩展成未确认的流程改动。

实现时继续遵守根 `CLAUDE.md` 的包边界、API 兼容、数据库迁移和测试规则；本次没有增加依赖。

### 文档来源与权威位置

原始工作备忘位于 `.omx/context/gitlab-personal-mr-identity-20260913T034232Z.md` 和 `.omx/plans/gitlab-personal-mr-identity-review.md`，均被 Git 忽略，属于本地历史记录。

后续以本目录的[需求文档](2026-09-13-gitlab-personal-mr-requirements.md)和本文为入口。技术稿的独立审查结论仅针对方案；本次补充的入口说明与暂缓状态不意味着产品已经实现。
