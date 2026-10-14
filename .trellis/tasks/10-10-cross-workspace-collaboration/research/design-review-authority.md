# Research: 协作设计的数据与授权审阅

- Query: 审阅 design.md D01–D05、D08–D09 的数据归属、ACL、执行凭据、清理和管理员边界。
- Scope: internal；为核对凭据补读 D07，队列／supervisor 细节由另一审阅者负责。
- Date: 2026-10-12
- 基线：本次读取的 `design.md` 共 236 行；PRD 已含 R39、AC67–68。O11 仍未确认。没有修改设计或产品代码，没有运行产品测试。

## Findings

总体方案与已确认需求相容：唯一来源 issue、全局 user ID、参与关系与来源成员权分离、独立执行关联、来源历史保留和按用途隔离凭据均已写明。以下是实施前应补齐的契约，不是已经运行验证出的产品漏洞。

### A01 · P1：新建请求缺少独立的幂等范围

- 位置：`design.md:48`、`:168`、`:191`、`:208`。
- 原因：唯一操作键是 `(issue_id,actor_user_id,request_id)`，但创建请求尚无 issue ID；草稿上传也没有持久键／消费规则。创建提交成功而响应丢失后，仅说“用原 key 重放”不能说明如何找回已创建单据，可能再次建单或二次消费草稿。R39/AC67 的原子创建不能替代请求幂等。
- 最小修订：增加创建操作范围，例如 `(source_workspace_id,creator_user_id,request_id)`，保存 payload hash、唯一 issue UUID 和结果；在同一创建事务中消费该操作。创建后查询回执不要求调用者事先知道 issue UUID。draft 固定 uploader/purpose、不可变 ID 和一次绑定状态，重复创建返回原结果而不重新绑定。
- 验证场景：两个并发相同 key、提交后丢响应、同 key 改 handler、同 draft 换 key；均有确定结果且不产生第二张业务单据。

### A02 · P1：必须明确每次业务凭据请求的动态授权

- 位置：`design.md:61`、`:106`、`:131`、`:147`–`:154`。
- 原因：claim/payload/launch 明确重验当前 epoch；业务 HTTP 只写 purpose 白名单和“对象校验”，没有定义已签发凭据的每次请求判词。绑定 token 中的 epoch 不等于比较当前 epoch；也不能把机器读请求降级到“owner 仍是来源成员”的人类 read basis。否则移除后重新加入、撤 grant 后重新授权或整单重开，旧执行可能继续读写。
- 最小修订：指定唯一 `AuthorizeCollaborationBusinessCapability`：每次正文读取、过程／结果／附件写入都检查 token 的 run/task/command、作者当前账号及 participation epoch、owner 当前账号、grant/resource/lifecycle epoch、执行 namespace 和业务能力未撤销；写操作在同事务重验。任何一项失效拒绝，禁止回退人类 ACL 或普通 mat_。终结凭据只保留已有窄例外。
- 现状依据：普通 mat_ 在 `server/internal/middleware/auth.go:99` 后直接进入下游；绑定只保证 actor/task/workspace。不能把这个旧路径的成功当作新动态授权已经存在。

### A03 · P1：共享单未列明的人类写操作缺少默认策略

- 位置：`design.md:63`、`:98`、`:174`–`:179`、`:208`。
- 原因：动作表只授权读／评论及具名管理动作，旧入口防线却只明确 status/assignee/creator/source 等“受管字段”。title/description、priority、labels、metadata/properties、自己的普通评论修改和已交付附件删除没有主体矩阵。实施者可能继续使用旧空间成员／管理员规则，也可能全面禁用，二者结果不同。不能从 R36 的基础访问直接推导额外写权。
- 现状依据：`server/internal/handler/issue.go:3834` 接受普通正文／优先级更新；`server/internal/handler/plugin_action.go:371` 另有正文 PATCH，`:453` 的锁内授权基于原空间成员和安装权限；`server/internal/handler/file.go:1428` 允许 uploader 或空间管理员删附件。
- 最小修订：列出共享单字段及附件／评论动作白名单与允许主体；未列明的共享写入默认拒绝。明确 Plugin、旧 API 和机器凭据同样受限。普通评论编辑若保留，必须限定普通评论，不修改 command／交接／审计历史。此项是权限选择缺口，不把 PRD 未明确的字段政策冒充已确认需求。

### A04 · P2：auth epoch 与 installation 的来源没有覆盖现有认证模式

- 位置：`design.md:44`–`:46`、`:106`、`:133`、`:156`。
- 原因：设计依赖统一 author/owner auth epoch 和固定 installation，但没有定义 password／普通 JWT-PAT、managed／legacy daemon 的取值及准入。现有用户表没有通用 account epoch，`session_version` 在密码凭据表；无绑定历史的 daemon 合法返回空 managed binding。只写“升级后支持”不足以解释注册和验证身份。
- 证据：`server/pkg/db/generated/models.go:1901`、`:1936`；`server/internal/service/managed_runtime.go:25` 在无绑定历史时返回空 binding；`server/internal/middleware/auth.go:62` 仅在 password 分支注入 PasswordSession。进一步核查现有 enrollment：`server/internal/handler/installation.go:49` 要求密码会话，`server/internal/service/managed_installation.go:143` 再次拒绝非 password 来源；普通 JWT/PAT 不能直接靠升级调用它获得 managed identity。
- 最小修订：补认证×runtime 支持矩阵，维持已支持的人类登录方式。managed 路径复用已验证 binding/账号代次；legacy 共享执行需明确 capability enrollment：由当前受支持的人类凭据授权，daemon 以持久私钥完成服务器 challenge，登记稳定安装/launch 身份及撤销代次，再签发窄执行能力。可复用 proof 编码，但不能直接放宽现有 password-only 管理入口。缺失身份保持未准入，不能以 NULL/0 或 daemon_id 当已验证；不借此默默排除整个登录模式。若新增中立账号代次，明确其事务更新者。R38 仍仅沿 password/admin 域。

### A05 · P2：grant 需要可执行的资源关系不变量

- 位置：`design.md:44`、`:72`、`:131`、`:198`。
- 原因：“真实 owner”“完整组合”“runtime 绑定”尚不足以指导独立 collaboration claim 分支保留原私有资源规则。owner/account epoch 未变时，agent 的 owner/runtime 或 namespace 仍可能变化。
- 现状依据：`server/internal/handler/daemon.go:2221` 重验 agent.runtime_id，`:2235` 拒绝私有 runtime 与 agent owner 不同；`server/internal/service/managed_runtime.go:63` 重验当前 owner/workspace/daemon 与 binding。
- 最小修订：写成同一个 grant 校验谓词：agent/runtime 均属于 execution workspace、agent.runtime_id 匹配、当前资源有效；私有 runtime 保持 grant owner/runtime owner/agent owner 一致，managed namespace 与保存的 installation/binding/epoch 匹配。授权确认、绑定、更换、领取和业务调用共用谓词；owner/rebind/archive/namespace 变化使旧 grant 失效并收敛停止。若支持 owner 不同的公有组合，另列规则，不通过私有分支默认放行。

### 已核对且未发现新增冲突

- D01/R39：只允许显式原子新建；不存在把普通单改标记转换的入口。旧普通任务保留原执行及附件契约。
- D05/R37：来源 workspace 的删除锁内先检查所有共享记录，创建使用对应 KEY SHARE fence；终态不绕过，拒绝不先清理。
- D01/D05/D09：来源 transcript/result/附件与 engine task 分离；执行空间清理保留 run、物理屏障和终结验证行；双向内容隔离及公开下载旁路已明确。现有清理确实删除 task_message 等叶子，见 `server/pkg/db/queries/workspace_delete.sql:197`，实施必须按新归属接线。
- D03/R36：退出协作仍是来源成员时保留基础访问；不保留 AI 控制。对象读取不依赖旧 loader 名称暗示的权限。
- D05/R38：管理员需当前 super_admin、完整会话、来源组织范围、creator 仍 disabled 和原因；仅三种生命周期动作，不得读全量私有内容或代替资源授权。
- O11 在 `design.md:110` 和 `:201` 明确未确认；没有把管理员附带指定 handler 当作已接受。该分支继续等待主会话，不由审阅者选定。
- D08 的锁后重读、借用同事务、NOWAIT 和不确定提交恢复方向正确；具体 writer 的锁模式清单与并发证明仍属于实施验收，不因本次静态审阅视为已验证。

## Files Found / Related Specs

- `design.md`：被审技术契约；`prd.md`：R01–R39 与验收要求。
- `research/cross-space-authority-design.md`：前一轮源码研究；本次仅补查上述少量具体边界。
- `.trellis/spec/server/security-boundaries.md`：人类／机器凭据、originator、借用事务及平台管理员约束。
- `.trellis/spec/server/managed-installations.md`：安装身份、运行绑定、代次与可信终结证明。
- `CLAUDE.md`：无新增 FK、每条 concurrent index 单独迁移、API 和 UUID 规则。D02 的迁移方案符合这些要求。
- 外部参考：无；本次未联网检索。

## Caveats / Not Found

这是有界设计审阅，P1 表示在实现前补齐的关键契约，P2 表示明确后才能可靠接线的边界。没有审阅全部执行计划、重跑源码探索、执行产品测试或评估 UI。队列及 supervisor 的竞争问题由其他审阅者负责。旧研究中的“存量转换待确认”已被 R39 收敛，本次不再列为开放范围。

## 复核：2026-10-12（仅 A01–A05）

结论：五项原发现均在设计契约层关闭；本轮未发现对应修订与已确认需求直接矛盾。

| 发现 | 复核结果与修订证据 |
| --- | --- |
| A01 | 已关闭。`design.md:48`、`:54` 区分 create/issue scope，固定来源空间/creator/request_id、payload 和 resolved issue；draft 一次消费。`:185` 提供无需已知 issue UUID 的创建回执查询。 |
| A02 | 已关闭。`design.md:153` 明确每次业务调用比较当前作者/参与、owner、grant/resource/lifecycle、run/task/command、安装及撤销状态，写操作同事务重验；禁止人类 ACL 和宽 token 回退。`research/execution-contract.md:17` 同步该谓词。 |
| A03 | 已关闭。`design.md:79` 明确仅创建、普通追加评论和具名动作；未列字段、评论编辑/删除/resolve、已交付附件删除默认拒绝，旧 API/Plugin/空间管理员无例外。R39 仅新建，因此没有存量转换导致的既有编辑能力回退；这是首版技术范围，未冒充新增产品授权。 |
| A04 | 已关闭。`research/execution-contract.md:114` 定义中立 account fence、初始化及推进它的账号治理事件；`:118`–`:125` 区分 password/legacy 与 managed/enrollment，保持既有登录，独立 key challenge 提供稳定身份，不放宽原管理注册、不接受空版本。具体 writer 登记和接线仍由 P1/P4 实施验证。 |
| A05 | 已关闭。`research/execution-contract.md:25` 明确执行空间、agent.runtime、有效资源、runtime owner 的既有 invoke 资格及私有 owner 约束；`:127` 要求授权到业务调用共用校验，owner/archive/rebind/namespace 变化使旧组合失效。 |

“已关闭”只表示原设计缺口得到明确回应，不表示代码、迁移、并发或凭据已通过测试。O11 仍未回答，本次不批准附带指定接手人，也不选择管理员重开的备选分支。仅追加本文件，未进行产品测试或新一轮全量探索。
