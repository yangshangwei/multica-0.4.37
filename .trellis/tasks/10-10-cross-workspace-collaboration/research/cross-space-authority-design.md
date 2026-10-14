# Research: 跨空间单据的数据与授权设计

- Query: 满足 R01–R38 时，如何保留唯一业务任务，隔离账号、协作权限、个人运行资源和管理员代管权限？
- Scope: internal；核查 Go HTTP、SQL、daemon、实时和附件边界；不实施产品修改。
- Date: 2026-10-12
- 依据：当前 `prd.md` R01–R38；已读取根 `AGENTS.md`、`CLAUDE.md`、`.trellis/workflow.md` 及下列相关规范。下文“现状”是源码事实，“建议”尚未实现。没有运行测试、服务或真实 AI CLI。

## Findings

### 1. 结论与不可混用的身份

建议保留来源工作空间的一条 canonical `issue`，新增按单参与关系、运行资源授权及共享执行关联。业务处理责任、指令提交者、运行资源拥有者分别记录。跨空间读取走对象级授权，不能把协作者补成来源空间成员，也不能把环境拥有者的个人凭据交给执行进程。

| 现状 | 源码证据 | 设计约束 |
| --- | --- | --- |
| `user.id` 是全局账号 ID；`member.id` 是账号与工作空间关系的 ID | `server/pkg/db/queries/member.sql:10`、`:27` | 参与者、处理人、发起人、授权人均使用 `user.id`，不使用 `member.id` |
| `assignee_type='member'` 的 `assignee_id` 实际传入 `GetMemberByUserAndWorkspace.UserID` | `server/internal/handler/issue.go:4162` | 现有列已经存用户 UUID；当前阻碍是“必须为本空间成员”的应用校验，不是字段无法表示外部账号 |
| 人类 actor 返回 `("member", userID)`；任务创建使用解析后的 actor | `server/internal/handler/handler.go:921`、`server/internal/handler/issue.go:2861` | 人工创建单的 `creator_id` 是用户 UUID；`creator_type='agent'` 时不能把该 UUID 当人类 |
| `loadIssueForUser` 解析来源空间及 issue，本身没有成员查询；授权依赖外层路由 | `server/internal/handler/handler.go:1102`、`server/cmd/server/router.go:2018` | 不能把这个 loader 原样搬到 Auth-only 路由后当成完整授权器 |
| issue UUID 稳定，编号需要来源空间前缀及序号才能解析 | `server/internal/handler/handler.go:1150` | 跨空间入口使用 UUID；编号、状态目录、项目仍属于来源空间，不移动或复制 issue |
| `issue_subscriber` 管理通知订阅和退订墓碑；现有跨空间未读数仍 JOIN 成员关系 | `server/pkg/db/queries/subscriber.sql:1`、`:62`；`server/pkg/db/queries/inbox.sql:149` | 不能把订阅行或未读 inbox 当作协作者 ACL／个人待办 |
| 私有 agent 的 invoke 仅认可有效调用人是 owner；空间管理员无 invoke 例外 | `server/internal/handler/agent_access.go:73`、`:147` | 本单 grant 是独立、窄范围的执行授权，不能放宽通用 `canInvokeAgent` 或写入全空间 allowlist |
| 任务的 `originator_user_id` 表示本次授权人；已结束任务不再借出其 invoke 权限 | `server/internal/handler/agent_access.go:234` | 指令归因保持真实提交人，不改成当前处理人、整单发起人或资源 owner |

`source_workspace_id` 与 `execution_workspace_id` 必须在类型和 DTO 中明确区分。前者决定业务数据、状态和访问；后者决定 agent、runtime、installation 和 daemon 权限。供应商登录是第三条链，与 Multica 登录及本单业务权限分开。

### 2. 最小数据形状建议

下面是职责划分，不要求每项额外复制一份已有数据。表名为建议名；状态及列名由最终设计统一。

| 数据 | 最少键／事实 | 唯一性与保留规则 |
| --- | --- | --- |
| 现有 `issue` | UUID、来源 `workspace_id`、正文、来源状态 key；人工创建时保留原 `creator_type/id` | 整单唯一；协作后不得以普通移动／复制路径改变其身份与来源 |
| `issue_collaboration` | `issue_id`、权限／操作版本、当前交接引用、启用时间 | `issue_id` 唯一；存在即启用协作契约，也是 R37 的持久删除门禁依据 |
| 当前处理人 | 优先复用 `issue.assignee_type='member'`＋`assignee_id=user.id` | 协作专用写入验证“有效账号＋本单参与关系”；普通单继续原本空间成员校验。共享 agent 绑定移到 session/grant，不能再靠把 issue 指派给 agent 启动 AI |
| `issue_participant` | `issue_id,user_id,participation_epoch,joined_at,revoked_at`，增删操作者 | `(issue_id,user_id)` 唯一；退出／移除留墓碑，重新加入提升 epoch；不从成员关系或订阅自动复活 |
| 交接／权限事件 | `id,issue_id,version,actor_user_id,action,from_user_id,to_user_id,purpose,reason`；补充请求的 `reply_to_handoff_id` | 追加记录；补充交回引用本次请求的 requester，不能猜“上一处理人”。请求幂等键与 payload hash 可随事件／操作记录保存 |
| 共享 session | `id,issue_id,active_grant_id,paused_at,version,active_execution_id` | 每个 issue 一个逻辑 session；供应商 session ID 是执行细节，不作为业务会话主键 |
| 按单资源 grant | `id,issue_id,owner_user_id,execution_workspace_id,agent_id,runtime_id,grant_epoch,consented_at,revoked_at` | 确认的是完整资源组合。旧 grant 撤回后保留，不能原地改 owner/runtime；重新加入不会恢复 grant |
| 指令 command | `id,session_id,seq,submitted_by_user_id,participation_epoch,content,state,cancel_reason` | `(session_id,seq)` 唯一；内容／提交人不可被下一条评论覆盖；已停止／取消的指令不能通过重开复活 |
| 执行关联 execution | `id,command_id,attempt,task_id,grant_id/epoch,execution_workspace_id,agent_id,runtime_id`＋提交人和执行身份快照 | `task_id` 唯一；一条指令可对应受控的执行尝试。另存物理执行屏障，不能仅以 `task.status` 表示进程已退出 |
| 来源空间过程记录 | `execution_id,seq,type,payload`、结果、用量快照、附件 manifest | 来源 issue 拥有；与执行空间的 task／agent 删除生命周期分离。可以复用消息编码器，但不能只引用会被清理的 `task_message` |

复用 assignee 列可以避免两个可独立修改的“当前处理人”。代价是协作详情、来源空间列表和历史渲染必须从参与者／用户投影补全姓名，不能只依赖来源空间 member 列表。另一可行选择是在协作表单独存 `current_handler_user_id`，但这时必须明确旧 assignee 列不再表示协作处理责任，并统一所有 DTO 和统计，不能双写两套权威字段。

发起人及当前处理人属于参与者；运行环境 owner 确认授权时也必须已经合法加入。发起人和处理人账号可后来被禁用，历史关联仍保留，不能用“只允许有效账号存在”的 CHECK 删除这些事实。是否支持 agent 创建的存量任务启用协作，涉及人类发起人来源，见最后的条件方案。

每次 command 对应的执行保持 `originator_user_id=accountable_user_id=submitted_by_user_id`，符合 `server/migrations/197_agent_task_attribution_strict_constraint.up.sql:14` 的单向不变量。当前处理人单独用于业务待办；不能为了表现处理责任把 accountable 改成 handler。`trigger_evidence_kind/ref_id` 可指向不可变 command，现有字段允许新的证据类型（`server/migrations/184_agent_task_attribution.up.sql:78`）。

### 3. 权限矩阵与共享授权器

统一提供 `ResolveIssueAccess(actor, issue)`，分别返回 `can_read`、`can_comment`、`can_control_ai`、`can_transfer`、`can_add_participant`、`can_remove_participant`、`can_change_environment` 和生命周期动作能力；写事务必须重新计算。权限由实际登录账号取得，不能由请求体的 `user_id` 或前端按钮证明。

| 操作 | 授权依据 |
| --- | --- |
| 详情、讨论、已记录过程、结果、交付附件 | 账号有效，且“来源空间当前成员 **或** 本单当前参与者”；每项关联资源仍须属于本单 |
| 普通评论 | 同上；默认惰性，不触发执行，回复 AI／`@AI`／编辑后出现 mention 也不触发（R24、R36） |
| 提交 AI 指令、停止、明确恢复 | 当前有效参与者；新执行／恢复另检查业务未结束、资源 grant、停止屏障与队列状态。停止不能因 grant 已撤回而不可请求 |
| 邀请新账号、普通转交、切换环境 | 当前有效发起人或当前处理人；切换还需队列挂起、旧执行已确认结束、新 owner 有效 grant |
| 移除其他参与者 | 当前有效发起人；移除当前处理人与回交发起人必须一次提交（R32） |
| 主动退出 | 本人是有效参与者，且既非发起人也非当前处理人 |
| 完成、取消、重开 | 普通入口仅发起人；R38 使用独立平台代管动作，不给普通入口加通用 admin bypass |
| 授予／撤回本单资源 | 真实资源 owner；私有 runtime 同时保持 agent owner 与 runtime owner 一致的既有约束 |

同一账号退出协作后，若仍是来源成员，保留 read/comment、撤掉控制与管理能力。若来源成员身份和参与关系都失效，所有详情、消息、附件和实时通道均拒绝。空间 owner/admin 不因空间角色获得共享单管理或 AI 权限；平台观察员也没有代管写权限。

建议人类管理／指令入口同时使用 `RequireHumanActor` 和真实 actor 校验；现有 legacy agent headers 会影响 `resolveActor`，不能只看到 PAT/JWT 就把一次 agent 调用算成人类确认。机器的任务能力仅允许本单数据读取和本次执行产物写入，不能继承提交人的邀请、转交、完成、取消、重开权限。

### 4. 外空间执行：两个可行方案的比较

现有执行并非天然支持跨空间：

- `agent_task_queue` 没有 `workspace_id`；`GetAgentTaskInWorkspace` 按 `agent.workspace_id` 查，`ResolveTaskWorkspaceIDChecked` 却优先按 `issue.workspace_id` 查：`server/pkg/db/queries/agent.sql:796`、`:2567`；`server/internal/service/task.go:7029`。
- claim 在加载项目上下文前要求来源解析空间等于 runtime 空间；token 最终提交再检查等于 runtime 空间：`server/internal/handler/daemon.go:2034`；`server/internal/service/task.go:3761`。
- 借用式普通 issue enqueue 也使用 issue 空间锁 agent/runtime：`server/internal/service/issue_task_transaction.go:50`。SQL `lock_task_owner_rows` 只锁住所有引用的 owner／空间并验证存在，**并不**实现跨空间访问授权：`server/migrations/284_task_owner_row_fence.up.sql:94`。

| 方案 | 做法 | 收益与代价 |
| --- | --- | --- |
| A：task 直接指向唯一来源 issue | 保留 `task.issue_id=source issue`，为协作 task 增加明确执行空间／grant 绑定，重做所有 source/runtime 分叉的解析 | 可以复用较多 issue 历史聚合；必须覆盖 claim、token、HTTP task 读写、WS、usage、清理和旧自动触发，不能只放宽 workspace mismatch。一处漏掉就可能越权或误归属 |
| **B：显式协作执行关联（建议）** | task 留在外空间 agent/runtime；新增 execution 关联指向唯一来源 issue。该 task 使用显式 collaboration purpose，普通 `issue_id/chat_session_id` 不伪装成本地载体 | 能保留普通 task 的同空间契约，并隔离旧 issue/comment 自动触发。新增 claim/payload/finalize 分支及来源空间过程记录是必要成本；缺少 issue/chat 链接时必须先识别 collaboration purpose，不能掉入 quick-create |
| 复制一张隐藏 issue／把成员或 agent 复制到来源空间 | 同步两个业务载体，或改变资源 owner/绑定 | 非必要；引入状态、历史、附件、取消、统计双写，且不能自然解决个人资源授权。源空间代理 agent 仍受私有 runtime owner 和 namespace 限制 |

方案 B 的 `task.context` 只应放可信绑定 ID／非敏感调度参数；指令正文和共享结果由来源空间记录组装。`BuildCollaborationClaim` 分别验证 source issue、当前参与人、grant 和执行 namespace，再向 daemon 提供最小 `SourceIssue` DTO。绝不通过改请求头、复制成员或把 workspace 参数改成另一个空间达成运行。

**双向隔离都要成立**：外部协作者不能通过 AI 的业务 token 读取执行空间其他任务；执行空间其他成员也不能凭 task 属于该空间 agent，就从 `ListAgentTasks`、snapshot、原始 task messages 或 WS 看见来源单内容。通用执行列表最多给既有运维权限允许的无正文状态投影，完整内容必须过来源单授权。

### 5. 凭据模型：单工作空间绑定不是单据隔离

现有 `mat_` 在 claim 时保存 `UserID=runtime.OwnerID`、24 小时有效期，Auth 把该 user、agent、task、workspace 写入服务器可信头：`server/internal/handler/daemon.go:1906`；`server/internal/middleware/auth.go:83`；password 分支为 `server/internal/middleware/password_auth.go:50`。workspace middleware 再用这个 user 查询成员：`server/internal/middleware/workspace.go:228`、`:258`。

所以现有 token 即使不能切换工作空间，仍可通过普通 `/api/issues` 等接口使用 owner 的空间访问能力。任务表中的 task ID 不能自动限制所有路由只能读取该任务。本单共享不能直接沿用这一范围。

建议给协作执行明确的凭据用途和统一的默认拒绝路由门禁：

1. **本次执行业务能力**：绑定 `execution_id/task_id`、来源 issue/session、grant epoch、提交人及参与 epoch、执行 owner／账号版本和安装绑定。可在 `task_token` 增加严格 purpose 分支，也可独立 token 类型；关键是只允许本单投影、普通执行评论和本次成果上传，其他 API 默认拒绝。不能同时向子进程交付一枚普通宽范围 `mat_` 作兼容。
2. **daemon 资源控制身份**：保持现有 execution workspace、runtime owner、installation/binding/epoch 门禁；仅 daemon 持有。协作者无权获取 owner 的 PAT/JWT、MDT、环境变量或管理 API。`FinalizeTaskClaim` 的旧 token/runtime workspace 校验不放宽；新协作 finalize 在自己的明确分支校验两种用途。
3. **停止／退出证明能力**：在可启动执行前预发，固定本次 execution、fence、installation/key；撤账号、资源或参与权限后，仅允许读取本次 stop 指示、上传最小可信退出证明。不能读取正文、提交业务结果、恢复、续租或领取新任务。该通道不能依赖已撤回的业务 token；物理屏障保存在来源协作记录中，不随通用 task cancelled、TTL 或清理消失。

新业务能力必须在每次 API 调用及队列领取／开始时重验当前参与关系、账号、grant 和执行绑定。资源 owner 离开按 R31 撤 grant 并挂起；移除提交者按 R15/R16 取消待执行指令、请求停止其运行指令。停止确认与业务权限撤回分别记录，不能把权限失效或连接断开当作进程退出。

`taskScopedAuthToken` 已禁止回退 daemon 的个人 token，供应商登录／自定义环境是另一条链：`server/internal/daemon/daemon.go:152`、`:7594`、`:7639`、`:7670`。保留这一原则。按单 API 能力不等于 OS 沙箱：现有 agent 可使用本机工具状态；若要求防御有 shell 能力的执行主动读取宿主所有私人文件／凭据，需要另外定义执行沙箱目标，不能宣称仅靠工作目录或 API ACL 已解决。

### 6. HTTP、实时、附件及后台入口清单

建议新增账号级 `/api/collaboration/issues` 列表与 `/api/collaboration/issues/{issueUUID}` 对象路由，放在 Auth 后、普通 workspace middleware 外。列表账号来自会话，分开查询“当前处理待办”和“仍在参与”；不把来源空间加入 `/api/workspaces`，也不从 inbox 已读状态推导待办。名称仅为建议。

该对象路由内明确分开 participants resolve/add/remove/leave、handoff/补充交回/提交验收、command enqueue、pause/resume、grant consent/revoke、environment switch 和 lifecycle。所有变更请求只提交目标／意图、幂等 key 与期望版本；actor、来源空间和资源 owner 从可信记录解析。命令提交应返回 command ID、入队序号及真实状态；不能用普通评论返回值暗示执行已开始。管理员例外使用下一节的独立 admin 路由。

| 入口／现有位置 | 必须覆盖的边界 |
| --- | --- |
| 详情、comments、timeline、task-runs、usage、附件、子任务、标签、属性；`router.go:2063` | 对象读取统一 `ResolveIssueAccess`；只暴露本单 DTO。父子／关联任务逐项鉴权，不能因一个 issue 获准就开放整个项目、状态配置、成员、技能或资源目录 |
| `UpdateIssue`、batch-update、move、delete、batch-delete；`router.go:2061`；`issue.go:4336`、`:4989` | 协作标记下分派到动作级策略。字段 patch 不能绕过普通转交、结束／重开、R37 或原发起人不变。非协作单保持旧行为 |
| 评论创建／编辑／删除／resolve、触发 preview；`comment.go:1688`、`:3430`、`:3467`、`:3675` | 普通评论和指令独立保存；协作单普通评论不进入 mention／reply／assignee fallback。编辑或删除评论不能重写 command，也不能通过 survivor retrigger 启动执行 |
| rerun、quick action、lifecycle handoff、source-context retry、squad-evaluated；`router.go:2066`、`:2078`、`:2102`、`:2212` | 这些是独立的执行或业务写入口。协作单必须拒绝不适用入口，或经同一明确 command 意图／生命周期授权；禁止旁路建立第二会话 |
| task 取消／messages／agent 全量取消与列表；`daemon.go:4991`、`:5094`；`chat.go:1776`；`router.go:2314` | task ID 先解析成协作 execution 再验证本单；旧路径不能仅凭 agent owner／空间成员就返回正文或取消共享单队列。整机管理停止可以发生，但必须收敛到同一停止屏障 |
| daemon HTTP＋WS RPC claim/start/progress/messages/complete/fail/cancel-ack/session/GC；`router.go:1515`、`:1529`、`:1547` | 区分资源 namespace 与 source issue 业务能力。入队、领取、开始、回执、回收都验证关联与 generation；普通重试／自动 continuation 不得复活停止指令 |
| Plugin `/v1` 与 browser bridge；`router.go:103`；`plugin_action.go:241`、`:371`、`:603` | 当前 plugin issue PATCH 仅 title/description，不能误称它已有状态写权限。所有共享读取／普通评论仍走本单策略；旧 plugin 不自动继承协作环境授权或整个来源空间 |
| 后台状态写入、triage、自动执行；`service/issue_status_write.go:26`；`service/task.go:5855`；`queries/agent.sql:388` | 不能只在浏览器 handler 加校验。借用事务写入保持来源状态／iteration 记录；共享单执行插入与领取需要持久门禁，遵守既有 admission，不从 task 完成推导整单完成 |
| 管理员取消 task 与 R38 生命周期 | 平台运维取消不是整单完成／取消。R38 为新增专用动作；不借旧 task cancel 或账号恢复隐式转交 |

**实时。** 当前 `/ws` 要求来源连接空间成员，连接后自动加入整个 workspace room；task scope 按 issue 空间、chat scope 按 creator 验证：`server/internal/realtime/hub.go:826`、`:382`、`:1040`；`server/cmd/server/scope_authorizer.go:47`。不能让外部协作者伪装连接来源空间。建议增加按 issue 的显式 scope，或账号级连接承载该 scope；只有本单事件进入它，绝不自动订阅来源 workspace。订阅、重连及每次受保护发送前验证当前 read basis；撤权后丢弃已排队的失效 payload，跨进程通知失效，客户端清理本单缓存。现有 password 发送重验只有账号 token 有效性（`hub.go:938`），不能代替本单 ACL 重验。

**附件。** 现有上传要求空间成员，metadata/content 按 `attachment.workspace_id`，浏览器 download 自查成员：`server/internal/handler/file.go:444`、`:728`、`:775`。协作附件应以 source issue 为上传及下载授权主键，校验 comment/execution/attachment 都属于本单，并保留实际上传者；不能开放任意 owner-space `attachment_id`。

- 公开 `/uploads/*` 直接读取本地对象：`server/cmd/server/router.go:1381` → `server/internal/handler/file.go:940` → `server/internal/storage/local.go:266`。旧注释提到成员校验，不代表当前实现真的执行了该校验。
- `/signed-download` 只检附件 ID、到期、签名，不重验账号或参与关系：`server/internal/handler/attachment_capability.go:198`；其 proxy capability TTL 为该文件 `:54` 的 60 秒。对象存储 presigned URL 另用 `file.go:701` 的 TTL 配置，两者都不是实时撤权。
- 新协作对象应在不可公开直读的独立 namespace 保存，由每次请求验证身份／权限的受控 proxy 返回。Desktop 原生下载若必须用 capability，应把用户、权限版本与对象绑定并在兑付时查当前授权；原 signed-download／公开存储 URL 不可继续成为旁路。已下载副本无法收回，不将清缓存包装成撤回已交付副本。
- 已公开的旧附件转换不能只修改数据库 URL 或前端缓存；须复制到私有对象、处理旧公开 key／旧签名可用期，并证明没有其他合法引用依赖旧对象。细节属于下方的存量转换条件。

### 7. 精确账号查找

新增专用 resolve 操作，可限定在有邀请／转交权的本单操作中；新建流程则先验证创建者的来源空间权限。请求显式区分 `username`、`email`、`user_id`，只做单值相等查询，禁止空值、名称片段、通配符、分页目录及“默认列出最近账号”。返回必要的 ID／显示名／头像供确认，不返回账号目录、工作空间关系或密码字段。

- username 来自 `user_password_credential`；复用 `auth.NormalizeUsername` 的 trim、小写及 3–32 ASCII 校验：`server/internal/auth/password.go:22`；精确查询 `server/pkg/db/queries/password.sql:4`，唯一索引 `server/migrations/464_password_username_unique.up.sql:1`。
- email 当前查询是相等匹配：`server/pkg/db/queries/user.sql:5`。现有登录／邀请将 email 小写，见 `server/internal/handler/auth.go:293`、`server/internal/handler/invitation.go:72`。不要为了查找改造全库账号标识；若兼容历史大小写导致多个匹配，拒绝歧义，不任选一个。
- UUID 使用边界 UUID parser；密码模式的 email 可为空（`server/migrations/462_password_credentials.up.sql:1`），不能把显示名当 username。
- 查找无写入；用户确认后的邀请／转交事务再次验证账号有效、本单权限及当前版本。复用现有限流构件限制反复精确探测；禁止复用面向平台管理员的模糊 `/api/admin/users`。

### 8. 事务、失效与留存

**交接及撤权原子性。** 锁定同一 issue/协作聚合，再读当前 creator、handler、参与 epoch；移除当前 handler、改回 creator、参与失效、取消该人的待执行指令、撤 owner grant（适用时）、挂起及持久 stop 意图、审计必须一次提交。带 `expected_version` 和幂等 key；重放已完成请求只返回原结果，过期请求不能把后来的 handler 再改回 creator。补充交回也检查本次 handoff 引用与当前处理责任，不能仅用旧按钮中的账号。

**锁的兼容性。** 资源准备／外部 MCP 工作在锁外；事务内重验。来源与执行 workspace 按 UUID 排序拿删除 fence；入队仍遵守 `lock_task_owner_rows` 的 workspace→agent→issue→runtime 顺序。密码／托管路径另有 user→namespace→resource 顺序；R38 还须先走平台 advisory lock＋有序用户锁。不能机械拼出一条新顺序并声称天然安全：共享 issue/task/owner 的相反锁序采用现有 NOWAIT 和完整回滚后重试，所有相关 writer 使用相同协议。参考 `.trellis/spec/server/security-boundaries.md` 的生命周期事务，以及 `service/issue_status_write.go:17` 的 READ COMMITTED 锁后重读。已提交但响应不明不能盲重试；按持久操作键核对。

**R37 删除门禁。** 现有 `DeleteWorkspace` 在 `workspace.go:1127` 取得 `LockWorkspaceForDelete`，之后开始清理。就在该锁内、任何 destructive cleanup 前检查 `EXISTS issue_collaboration JOIN issue WHERE issue.workspace_id=...`；完成／取消和无其他参与者都不能跳过。启用／创建协作的事务拿同一 workspace 的 KEY SHARE，关闭“删空间检查后插入首张共享单”的竞态（`queries/workspace.sql:65`、`:82`）。拒绝需返回稳定业务码，保留全部记录。普通空间继续既有删除规则。

**记录归属。** 现有 `workspace_delete.sql:197` 会删任务的 usage/messages/token，再删 task；候选任务可能因 agent、issue 或 runtime 任一归属而被纳入。只阻止来源空间删除，不足以防外部执行空间清掉共享过程。因此方案 B 的 source execution/message/result/附件事实必须独立留存；执行空间退出、解绑或删除只终止资源使用和清理其通用运行记录，不能清理 source 记录或物理屏障。事件与用户显示快照不产生授权。

现有禁用使用 UPDATE，并撤销凭据、保留用户／成员／执行历史：`service/platform_account.go:173`、`:220`；`auth/password_revoke.go:15`。但撤销范围目前偏 runtime owner，没有本单参与 epoch／指令提交者维度。新调度器必须识别真实提交者和 grant owner 是否仍有效；不能因为机器 owner 仍登录就继续为已失效提交者启动指令。失效不删除历史，后续账号恢复也不能复活已取消指令。

建议首版共享单硬删除／批量删除采用明确拒绝或保留 tombstone 的专用策略，避免普通 DELETE 抹掉 R37 依据。若产品需要显式销毁共享记录，应单独确定权限及保留政策；当前 R37 只确认禁止直接删来源空间，没有完整定义主动销毁单据的生命周期。

### 9. R38：狭窄的管理员生命周期代管

沿用现有平台管理员身份：password 模式、已启用平台管理、完整 JWT/cookie 会话、当前账号与版本有效、当前 `super_admin`、有效管理组织。PAT、task token、`platform_observer`、空间 owner/admin 均不够。现状实现见 `handler/admin_auth.go:59` 与 `service/platform_admin.go:62`；组织归属用 `organization_workspace` 验证来源空间，不把“同一 Server”当作省略管理范围检查的理由。

新增显式代管入口（例如 `POST /api/admin/issues/{id}/collaboration-lifecycle`），仅接受 `complete|cancel|reopen`，非空原因、幂等 key、期望版本。事务内借用 `LockPlatformAdminMutation` 锁真实管理员与原 creator，重验当前平台权限／会话版本、来源空间范围、**原 creator 仍 disabled** 以及 issue 状态。原 creator 不改变。审计同时写实际管理员、原 creator、before/after、reason、操作 ID；复用 `admin_operation`／`admin_audit_event`（`server/migrations/465_platform_admin.up.sql:25`、`:57`），同事务追加共享历史。

普通来源状态的 `done/cancelled` 类别及自定义同类状态都触发 R35；未知类别不当作可运行，复用 `service/issue_status_write.go:87`。现有 `UpdateIssue` 明确规定连变为 cancelled 都不停止普通任务（`handler/issue.go:4104`），因此 R35 只在协作分支增加收尾，不能全球改变旧行为。重开保持队列挂起、原取消指令不复活、旧物理屏障继续有效；管理员不获得提交 AI 指令、资源授权、切换环境、邀请或普通转交能力。

| 账号状态 | 现有需求能够闭合的路径 |
| --- | --- |
| creator 有效，handler 被禁用 | creator 按 R33 转交，或按 R32 移除并回交；不需要 R38 |
| creator 被禁用，handler 有效 | handler 仍能按既有职责转交／邀请；平台管理员仅代做三种生命周期动作 |
| creator 与 handler 都被禁用，尤其二者同一人 | 管理员可以重开，但重开保留 disabled handler。其他协作者即使能控制 AI，也无邀请／转交／换环境权；**R38 本身无法产生有效接手人** |

最后一项是需要产品确认的真实缺口，不能在技术实现中偷偷给管理员转交权、自动把管理员改成 handler、自动替换 creator，或为恢复流转擅自恢复一个有意禁用的账号。现有 `/api/admin/users/{id}/restore` 是独立账号治理动作，不能当作本单重开的隐式步骤。

### 10. 新建与存量转换的条件方案

主会话正在确认范围；本研究不默认排除存量任务。

| 条件 | 需要增加的门槛 |
| --- | --- |
| 仅新建协作任务 | 创建时原子写 canonical issue、人工 creator／handler、参与关系、协作标记和 session；全部协作附件从受控 namespace 开始，旧 issue 沿原契约运行 |
| 同时支持已有 issue 转换 | 保留原 UUID／来源／历史，先明确人类 creator 的来源；盘点既有 queued/running/retired/retry/session、旧 task token、公开附件及外部链接。在旧执行退出得到可靠证明、旧旁路令牌撤销、附件保护和历史投影完成之前，不允许启用新共享执行 |

转换需要持久状态和可恢复幂等操作，不能一次设置 `is_collaborative` 就宣告完成；不能因为旧数据库 task 已 terminal 而判定无进程。agent 创建的存量 issue 不能把 `agent.owner_id` 默认当作整单发起人。已存在的并行执行、公开附件无法撤回的副本以及旧 Desktop/daemon 对 purpose 字段的理解，都需列入转换验收。未支持 collaboration purpose／所需停止证明的 daemon 应明确拒绝领取，不降级为普通 task。

### 11. 文件索引、规范及迁移约束

| 文件／目录 | 作用 |
| --- | --- |
| `server/internal/handler/handler.go`、`middleware/workspace.go` | 身份解析、实体 loader 与 workspace 外层授权 |
| `server/internal/handler/issue.go`、`comment.go`、`task_lifecycle.go` | 业务写入、评论触发和手动 rerun 旁路 |
| `server/internal/handler/agent_access.go`、`daemon.go`、`chat.go` | 私有资源 invoke、claim、任务读取／取消 |
| `server/internal/service/task.go`、`issue_task_transaction.go`、`issue_status_write.go` | 执行归属、token 原子提交、借用事务和状态写入 |
| `server/internal/middleware/auth.go`、`password_auth.go`、`internal/auth/password_revoke.go` | 机器凭据、版本失效与保留记录 |
| `server/internal/service/platform_admin.go`、`platform_account.go`、`handler/admin_auth.go` | 当前平台授权、账号禁用、锁与审计 |
| `server/internal/handler/file.go`、`attachment_capability.go`、`internal/storage/local.go` | 附件上传／元数据／公开及受控下载 |
| `server/internal/realtime/hub.go`、`server/cmd/server/scope_authorizer.go` | WS 连接、订阅及 task/chat 对象权限 |
| `server/pkg/db/queries/{member,user,password,issue,agent,task_token,task_message,subscriber,inbox,workspace,workspace_delete,organization}.sql` | 实际 ID、关系、读写、删除及索引需求 |
| `server/internal/handler/plugin_action.go`、`server/cmd/server/router.go` | Plugin 与全部 HTTP 入口注册 |

相关规范：

- `CLAUDE.md`：数据库迁移、UUID 来源、API schema、既有行为及真实 AI 测试隔离。
- `.trellis/spec/server/index.md`、`security-boundaries.md`：机器凭据不能变成人类凭据；真实 originator；锁内重验、借用事务、审计和平台管理员范围。
- `.trellis/spec/server/managed-installations.md`：user／namespace／resource 锁序、绑定版本、claim generation 和可信停止证明。
- `.trellis/spec/server/triage.md`：admission 与状态分开；所有 enqueue/merge/retry/claim/start 的持久门禁；通知不是业务状态。
- `.trellis/spec/server/project-execution-squad.md`：资源 owner、实际运行位置和配置权限不等于 invoke。
- `.trellis/spec/core/frontend/index.md`、`.trellis/spec/guides/cross-layer-thinking-guide.md`：共享 API、缓存权限边界及各层独立消费者核对。
- `apps/docs/content/docs/developers/conventions.zh.mdx`：user／member 命名、单数 snake_case 数据表及双向迁移。

所有新关联不增加 `FOREIGN KEY`／`REFERENCES`／级联动作，关系、删除与原子性由应用事务负责。**每个索引，包括新表唯一键，必须单独文件 `CREATE [UNIQUE] INDEX CONCURRENTLY`**；若需 PK，先单独并发建立唯一索引再附着，不能用内联 PK/UNIQUE 隐式绕过规则。迁移编号由实施时分配；每项提供安全 down，存在已形成共享历史时拒绝破坏性回滚。不要依据旧 `001_init` 的 FK／普通索引写法新增约束。

外部参考：本次没有联网检索；项目基线为 Go 1.26、PostgreSQL 17（根 `CLAUDE.md`）。后续实现涉及并发索引／锁细节可核对 PostgreSQL 17 的 `CREATE INDEX` 与 explicit locking 官方手册；此文没有声称做过数据库运行验证。

## Caveats / Not Found

- 已确认需求可以支撑本单 ACL、按单 grant、唯一会话与有限代管设计；现有代码尚无可直接复用的完整跨空间执行能力。
- 管理员重开时 creator／handler 均 disabled 的接手路径尚未定义；需要产品选择，不能扩大 R38。
- 新建专用还是包含存量转换等待主会话确认；后者增加旧执行屏障、凭据和公开附件迁移成本，不能默默略过。
- 共享单主动硬删除、agent 创建存量单的人类发起人，以及补充请求目标退出／禁用后的恢复策略，需在最终设计明确。安全默认是冲突并保留原记录，不自动复权或任选接手人。
- API 权限可以阻止平台数据越权；是否要求恶意 shell 级别的宿主文件隔离，不能由 R13 的按单授权文字直接推导为已经实现。
- 本文只完成静态研究；未运行测试、迁移、数据库写入、服务或 AI CLI。文件行号基于研究时工作区源码。
