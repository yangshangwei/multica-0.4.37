# 技术设计方案

> 实施更新（2026-10-01）：用户已批准并启动开发。下文保留评审时的设计说明；当前完成项与未测边界以 [verification.md](verification.md) 为准。

## 方案选择

采用“新增用户名密码入口，复用用户和登录会话，复用欢迎引导”。注册、登录、旧账号绑定是三个操作，成功后汇入统一会话建立路径。密码登录只能查询已有账号，不能照搬 findOrCreateUser 自动开户。

| 方案 | 决策 |
|---|---|
| 跳过邮箱验证码直接登录 | 拒绝：没有身份校验。 |
| 将用户名伪装为设备邮箱继续自动登录 | 拒绝：仍依赖设备身份，且容易保留绕过密码入口。 |
| 新建一套用户、空间、权限系统 | 拒绝：重复业务模型，迁移风险大。 |
| 独立密码凭证 + 原 user UUID + 原会话和引导 | 采用。 |

## 总体组成

- 交互：ux-design.md。
- 模块与数据库：architecture.md。
- 旧账号与发布：migration-rollout.md。
- 测试：test-spec.md。

## 认证配置（拟新增契约）

建议新增 `MULTICA_AUTH_MODE=password|legacy`，未配置时保留当前部署行为，避免改变官方云及其他部署。此公司的目标部署显式设为 password；不是保留当前内网设备登录行为。

password 模式服务端统一拒绝 `/auth/device`、邮箱发码/验证和 Google 登录；与 `MULTICA_DEVICE_AUTH_ENABLED=true` 冲突时启动报错，不能按路由各自猜测优先级。邮箱白名单属于邮箱模式，不能隐式约束用户名。

`/api/config` 增加 `auth_mode`、`password_auth_available`、`password_signup_available`、`account_binding_available`。注册开放由 `ALLOW_SIGNUP` 控制，登录能力与注册能力分开。客户端字段缺失默认不支持密码，不能探测后自动降级免登录。绑定资格是私有 `/api/me` 的 `requires_account_setup`，公共 capability 不泄漏用户状态。

部署必须允许工作空间创建；如果 `DISABLE_WORKSPACE_CREATION=true`，发布检查判定不满足本 PRD，不能声称完整支持“注册后自行创建”。已有管理员策略导致创建被拒绝时，UI 如实提示，不绕过服务端。

## HTTP 接口草案

| 接口 | 输入 | 成功 | 失败及约束 |
|---|---|---|---|
| POST /auth/register | username, password, name | 201 LoginResponse；新 user + credential；无 workspace | 400 校验，409 username_taken，403 signup_disabled，429，503；无验证码/邀请参数 |
| POST /auth/login | username, password | 200 LoginResponse；仅已有账号 | 未知用户名/错误密码统一 401 invalid_credentials；429/503 |
| POST /api/me/password/setup | username, password, name | 200 LoginResponse；更新当前旧账号 | 仅合法迁移会话；409 username_taken/already_configured；不接受目标 user_id |
| POST /api/me/password/change | current_password, new_password | 200 LoginResponse；版本递增、撤销旧凭据 | 校验当前密码；受限临时密码会话允许访问；429/401 |
| GET /api/me | 无新增输入 | UserResponse 加可选 username、requires_account_setup、requires_password_change | 不返回哈希、盐、凭据版本内部数据 |

LoginResponse 复用 `{token, user}`，以及现有 cookie/CSRF/CDN cookie 写入和 auth store 逻辑。新增网络数据必须使用 zod + parseWithFallback；空 token、空 user.id 不能被 fallback 当成成功。

错误结构沿用现有 API error envelope，增加可机器识别的 code（若当前 envelope 不支持，先统一扩展再使用），不能仅靠中文消息分支。用户名占用在注册时明确提示；登录时不暴露账号是否存在。

## 事务与失败语义

- 注册：规范化/验证 → 限流 → 哈希 → 事务创建 user 和 credential → 提交 → 签发会话。任一步 DB 失败全部回滚。
- 注册事务提交后签发/返回失败，账号可能已存在；客户端提示改用登录，不宣称事务仍未发生，不无限重建账号。
- 绑定：验证受限旧会话 → 锁定当前 user → 检查未绑定 → 更新姓名并创建凭据/版本 → 按策略撤销旧凭据 → 提交 → 签发新 JWT。
- 用户名唯一性、每用户最多一条密码凭据由 DB 唯一索引保证，预查询只服务交互，不是并发保障。
- 注册成功并不设置 onboarded_at，也不调用设备共享工作空间 provision。

## 兼容边界

原 user.id、workspace membership、评论及任务外键式关联逻辑保持不变。密码凭证不进入通用 User 模型。API email 继续输出字符串，底层 NULL 映射为空字符串；前端优先 name / username，不展示空邮箱占位。邮箱为空的账号不能被“按邮箱”入口创建、找回或加入邀请。

Web 与 Desktop 共用 core API/store 和 views 表单，分别处理 cookie 与 token。移动端本轮不增加 UI；连接 password 部署必须给出不支持/升级提示或在发布说明明确不支持，不能静默放宽服务端认证。

## 简化策略与风险

不引入新的身份服务、邮件组件、邀请系统或全局管理后台。密码算法优先用现有 Go 标准库。必要的新工作集中在凭证存储、服务器切换隔离、令牌失效和可靠限流，这些不能通过“内网”假设省略。

设计尚待实施前评审，尚无产品代码或实测安全/性能结论。

## 运维恢复契约补充

受控服务端命令定位明确的 user UUID；无 credential 时为该 UUID 创建唯一 username 和初始版本1的凭据，不新建 user；有 credential 时保留 username、更新密码。设置密码哈希并将 `must_change_password=true`、session_version 递增，撤销旧凭据；命令不暴露网络接口。随后用户以临时密码登录，JWT 仍关联该账号，但服务端在所有请求中依据数据库标记只允许 me/password/change/logout。`/api/me/password/change` 验证临时密码并设置新密码，事务清除标记、再次递增版本，返回新会话；完成后才触发正常 onLogin、工作空间预热及 daemon 自动启动。普通用户修改密码也校验 current_password，但第一版可以不提供常驻设置入口。

注册正常账号的 must_change_password 默认 false，不增加任何正常注册步骤。受限状态必须服务端强制，不能仅靠前端弹窗。

## 2026-09-30 工程评审决策

用户确认本轮继续方案评审，首版仅本地/自托管。security-contract.md 固定迁移时间窗、个人派生凭据版本、事务锁、WS 推送校验、限流和服务器切换状态机；architecture.md 已同步关键取舍。范围不扩展到云 Fleet、管理员 UI 或自动账号合并。

认证响应/错误结构按原协议扩展，不重建用户或工作区。工程评审完成不代表实现获准或测试通过；task 保持 planning。
