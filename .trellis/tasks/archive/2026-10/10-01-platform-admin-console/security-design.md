# 平台管理后台：认证、授权与账号安全设计

状态：设计提案，尚未实现；适用首期 `MULTICA_AUTH_MODE=password`。遵守 `.trellis/spec/server/security-boundaries.md`，不改变原工作空间授权模型。

## 1. 平台门禁

新增 `requirePlatformAdmin`/`requirePlatformRead`，路由级与敏感 handler 均使用同一权威 helper。它从服务端 context 取得 `PasswordSession`，要求 `Kind == "jwt"`、`Version > 0`、`Setup == false`、`Change == false`，查询账号未禁用、当前密码版本和有效平台绑定；不信任请求头、请求体或前端传来的 role/user ID。

`RequireHumanActor` 不是此门禁：它明确允许 `mul_` PAT，未知 actor source 也放行（`actor_guards.go:96`）。密码鉴权已经产生 `jwt`/`pat`/`task_token` 类型（`middleware/password_auth.go:39`），应复用权威类型作白名单，不维护不完整 token 黑名单。

必须同时封住凭据转换路径：当前 `/api/cli-token` 的 password 分支仅调用不检查 Kind 的 `LockPasswordSession`（`auth_password.go:415`、`password_session.go:125`），有效 PAT 可以兑换 JWT。新增平台角色前，应在该 JWT 签发入口要求源 `Kind == "jwt"`，测试 PAT → JWT → admin 链路被第一步拒绝；只保护后台路由不足。JWT/cookie → CLI JWT 的正常交接保留，PAT 自身续期不受此入口收紧影响。

历史上由 PAT 兑换的 JWT 没有来源标记，不能仅靠关闭新兑换入口消除影响。首次授予或提升平台角色（含 bootstrap）时，在同事务提升目标密码版本、撤销派生凭据并提交后断连，要求重新密码登录；旧 JWT 因版本不匹配不能随新角色取得权限。授予接口和页面明确告知该账号现有桌面/CLI 会话需要重新认证。

已核实 CLI 常规登录是浏览器 JWT → `/api/tokens` 签发 PAT（`cmd/multica/cmd_auth.go:313`），daemon 续期是 `/api/tokens/current/renew`（`internal/daemon/client.go:650`），均不需要 PAT → JWT。收紧逻辑只放在 JWT 签发入口，不能把公共 `LockPasswordSession` 整体改成 JWT-only，否则会破坏 PAT 续期。启用平台角色前必须验证浏览器/桌面交接、CLI bootstrap、PAT 登录与续期；旧会话被角色授予撤销后通过 `multica login` 重新走浏览器 JWT 认证，不设计 PAT 自动升级到管理员会话的回退。

| 调用者 | 平台只读 API | 平台写 API |
| --- | --- | --- |
| 有效 `super_admin` JWT/cookie | 允许限定元数据 | 允许相应能力；敏感写再次复核密码 |
| 有效 `platform_observer` JWT/cookie | 允许限定元数据与审计摘要 | 403 |
| 普通用户、workspace `owner`/`admin` | 403 | 403 |
| 有效 `mul_` PAT、`mat_` task token，即使属于超级管理员 | 403 | 403 |
| `mdt_`、`mcn_`、plugin token，或任何不被密码 Auth 接受的凭据 | 401 | 401 |
| 失效/撤销 JWT、已禁用账号的旧凭据 | 401 | 401 |
| 临时恢复会话、迁移设置会话 | 403，仅允许现有设置/改密/profile/logout 路径 | 403 |

未支持的认证模式返回 403 `admin_mode_disabled`。cookie 写沿用 CSRF 校验；JSON 请求校验 Content-Type、大小与未知字段；管理响应 `no-store`，不签发可绕过版本检查的下载/CDN 能力。

## 2. 角色与私有内容

平台角色和工作空间角色独立。只读角色不能改告警负责人、确认告警或导出私有内容；这些均是写操作或额外数据访问。`super_admin` 允许账号、平台角色、终端接单控制、单次执行取消及告警处置。

平台查询返回固定字段白名单：UUID/业务编号、状态、脱敏归属、计数、耗时、模型/版本、受控错误码、时间戳。自由文本标题、任务描述、聊天、消息、日志、附件、原始异常、命令和本地路径默认不返回。若当前用户也具有原资源访问权，可经既有资源 API 打开内容；不得因平台角色添加到所有工作空间，也不得把原 handler 的成员校验去掉。

ID 关联、下载、导出、搜索命中摘要和 WebSocket 都执行同样边界。不存在与不可见的私有资源统一使用原资源 404 语义，避免用平台接口探测内容。

## 3. 不缓存授权与事务锁顺序

管理 API 每次查当前角色和状态；不从 JWT role claim、PAT cache 或长 TTL 角色缓存授权。UI 的 `admin/me` 仅用于渲染。角色撤销成功后下一次请求拒绝，前端清缓存并展示无权状态。

所有角色变更、账号禁用/恢复和强制密码恢复共享一个事务级平台管理员 advisory lock；随后将操作者和目标用户 UUID 排序，以 `FOR UPDATE` 获取用户行，再读取/修改角色和凭据。普通管理写至少锁操作者用户行并重查其 session version/状态/角色；角色变更必须锁同一用户行。这使排队中的旧权限写请求无法越过已提交的撤权。

锁内核验 last-admin、幂等记录、目标预期版本，再写业务数据、`admin_operation` 与 `admin_audit_event`；失败全部回滚。普通用户改密/凭据签发仍使用现有用户行锁，不反向取得平台 advisory lock，避免锁序反转。密码 KDF 在锁外执行，锁内再次比对用于验证的 hash、源版本与状态。

最后有效管理员计数定义：存在 `super_admin` 绑定、账号未禁用、密码凭据存在且无需设置/强制改密。撤销、降级、禁用或强制恢复后该计数不得为零，否则 409 `last_super_admin`。两个管理员同时互相降级/禁用最多一个成功。普通后台禁止自我禁用及自我强制恢复；改自己密码使用既有个人设置流程。

## 4. 禁用、恢复与派生凭据

禁用事务：锁及重验 → 目标 `disabled_at`/原因 → `session_version + 1`（检查溢出）→ `RevokePasswordCredentials(targetUserID)` → 操作与审计 → 提交。恢复账号清除禁用状态并再次提升版本，不解除 `must_change_password`，不恢复旧 JWT/PAT/task/daemon token，也不补建原 runtime 会话。

持久禁用检查必须覆盖登录、`CheckPasswordJWT`、`CheckPasswordVersion`、`LockPasswordSession`、派生凭据签发、plugin member callback、daemon claim，以及实时/daemon WS 入站和出站授权。已有临时 denylist 只有在持久数据迁移与拒绝行为一致后才能移除；其他认证模式的现有封禁不能被意外放开。

账号禁用不删除用户、成员关系、历史任务或插件安装。现有撤销 helper 会取消该用户 runtime 关联的服务端执行并置离线（`auth/password_revoke.go:15`）；这不证明离线机器进程已停止。管理界面分别显示访问已撤销与进程停止是否确认。

提交后断连必须使用 **targetUserID**，审计归属使用 **actingAdminID**；不能原样复用从请求 `X-User-ID` 取断连对象的 `publishPasswordRevocation`（`auth_password.go:441`）。改为显式目标参数或等价 helper，个人改密传自己 ID。跨实例连接通过现有版本复核失效；本机主动断连只是加速，不是唯一撤销机制。

## 5. 账号恢复与管理员初始化

拟定接口：`POST /api/admin/users/{id}/recover-password`；body 包含管理员当前密码、目标临时密码、原因和预期版本，幂等键统一置于 `Idempotency-Key` header，不允许修改用户名。密码字段只在内存完成 KDF，禁止进入日志、审计、操作记录或幂等 hash；幂等摘要仅含非秘密字段，同一成功 key 再次提交只返回原结果，不重新设置密码。

锁内重验操作者密码快照与目标凭据版本；将目标 hash 替换、`must_change_password=true`、版本提升并撤销派生凭据，完成后只返回操作状态，不返回目标 JWT。目标用临时密码登录后只能改密，成功改密再提升版本、撤销临时会话并签发自身正常会话。管理员交付临时密码使用企业已有受控渠道，后台不自动发邮件或即时消息。

角色授予/撤销、禁用/恢复账号、强制恢复都在同请求中要求当前管理员密码复核，并复用现有 KDF 并发限制和按操作者/IP 限流。复核失败不改变目标；响应不区分目标密码情况。公开注册不可接收平台角色字段。

首次管理员由新增 `platform-admin bootstrap --user <UUID> --reason ...` 命令初始化：在监听器启动前执行、要求现有完成密码设置的账号、全局锁内仅在无有效管理员时授予、记录 deployment-operator 类型审计；不按注册顺序、邮箱后缀或默认密码提权。

现有 `password-recover` CLI 必须复用相同账号恢复事务与审计。常规 CLI 同样执行最后管理员保护；唯一管理员丢失密码时允许显式 `--break-glass` 的部署侧恢复例外，要求本机部署权限、目标 UUID、原因、受保护 stdin 临时密码，审计标明恢复期间无可用管理员。它只恢复目标凭据，不绕过强制改密、不自动给任意用户授予角色。恢复账号后核验至少一个可用管理员，并建议配置第二个具名恢复管理员。

## 6. API 与异常语义

| 状态 | 含义与处理 |
| --- | --- |
| 200 | 查询成功；本地账号/角色变更已原子提交，返回当前对象/操作；不是远端执行完成证明 |
| 202 | 持久远程管理操作已受理，返回 operation ID，后续查询结果 |
| 400 / 415 | 参数/UUID/期限/未知字段不合法，或非 JSON |
| 401 | 没有有效认证、版本撤销；前端清登录会话 |
| 403 | 有效身份但凭据类型/平台角色/改密阶段/CSRF 不允许；密码复核失败用独立 code，不使整个会话自动登出 |
| 404 | 平台可见目标不存在；原内容 API 延续不可见即 404 |
| 409 | 最后管理员、幂等 key 与非秘密请求摘要不一致、预期版本冲突、当前状态不允许 |
| 429 | 限流，返回 Retry-After |
| 503 | 认证/数据库/审计依赖不可用，拒绝继续；不得伪装成普通 401 或使用旧角色缓存放行 |

审计不可用时管理写回滚。提交结果不确定时客户端用幂等 key 查询，不自动重放；操作记录不存明文密码或密码可离线枚举的摘要。拒绝尝试可独立记安全事件，数据库故障时记录脱敏结构化服务日志。

## 7. 必测边界

- JWT/cookie/PAT/task/daemon/cloud/plugin 类型矩阵；PAT 不能先兑换 JWT 再进入后台；伪造 `X-User-ID`/role/source；无 workspace 管理员；非 password 模式拒绝。
- 角色撤销与进行中管理写、两个管理员互降/互禁、管理员刚复核密码就被撤销、初始化并发；旧 PAT 兑换 JWT 在后续授予角色后失效；断言并发结果和审计一致。
- 禁用/恢复/恢复密码后旧 JWT、PAT、task/daemon 凭据、plugin callback、双向 WS 都失败；503 保持可重试而不是误报封禁。
- 管理员 A 操作 B，只断开 B；A 的会话仍有效；不删除 B 的成员与历史；最后管理员常规恢复拒绝、部署恢复显式记录。
- 私有内容在列表、详情、搜索、下载、导出和实时事件都不泄露；读角色所有写入口拒绝；未知响应字段安全退化。
