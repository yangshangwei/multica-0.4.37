# 认证现状与设计依据

核对日期：2026-10-01。以下是源码观察，不代表规划中的平台管理能力已经实现。

| 已核实事实 | 源码位置 | 对后台设计的影响 |
| --- | --- | --- |
| 登录返回携带 `auth_version` 的 JWT，并设置认证 cookie | `server/internal/handler/auth_password.go:84` | 复用现有用户和密码体系，无需第二套管理员密码 |
| 密码模式注册/登录是现有公开路由 | `server/cmd/server/router.go:1413` | 后台只补平台授权；注册不得自动授予平台角色 |
| JWT 校验与派生凭据版本校验读取密码凭据 | `server/internal/auth/password_session.go:51`、`:72` | 禁用状态应加入同一权威校验链 |
| 凭据签发用源会话版本并锁用户行 | `server/internal/auth/password_session.go:123` | 管理撤销复用该行锁，避免迟到签发使用更新后版本 |
| 会话 context 分清 `jwt`、`pat`、`task_token` | `server/internal/middleware/password_auth.go:39` | 后台可严格允许 `jwt`，拒绝个人 PAT 和机器凭据 |
| `RequireHumanActor` 只拒绝 task/cloud actor，未知值放行 | `server/internal/handler/actor_guards.go:96` | 单独使用它不符合后台要求 |
| password CLI JWT 签发调用未检查 Kind 的会话锁，PAT 可进入此路径 | `server/internal/handler/auth.go:700`、`auth_password.go:415`；`server/internal/auth/password_session.go:125` | 必须在新增平台权限前收紧 JWT 签发的源类型，阻断 PAT 转换后进入后台 |
| CLI 浏览器登录使用收到的 JWT 创建 PAT | `server/cmd/multica/cmd_auth.go:313` | 正常 CLI bootstrap 不依赖 PAT → JWT；角色授予撤销后重新浏览器认证 |
| daemon 原地续期 PAT 使用独立 endpoint，共用会话锁 | `server/internal/daemon/client.go:650`；`server/internal/handler/personal_access_token.go:211` | 不可将 `LockPasswordSession` 本身改成 JWT-only，保留 PAT 续期 |
| 受限会话只开放 `/api/me`、logout、setup/change | `server/internal/auth/password_session.go:113` | 临时密码不能登录后台执行管理操作 |
| 认证失败区分 401 和数据库/依赖故障 503 | `server/internal/middleware/password_auth.go:15` | 不允许遇到故障回退旧权限或误报退出 |
| 密码登录在锁内比对 hash/版本，并检查临时封禁 | `server/internal/handler/auth_password.go:223` | 持久禁用沿用相同并发检查位置 |
| 密码恢复在服务启动前执行，密码经 stdin 输入 | `server/cmd/server/password_recovery.go:22` | 管理员初始化/部署恢复复用此命令生命周期 |
| 恢复设 `must_change_password` 并撤销派生凭据 | `server/cmd/server/password_recovery.go:63` | 后台恢复应共用事务语义，不签发目标会话 |
| `ChangePasswordCredential` 通过源版本 CAS 并递增版本 | `server/pkg/db/queries/password.sql:11` | 禁用/恢复还需独立版本提升查询，不能伪造一次改密 |
| 撤销保留身份/成员/安装历史，但取消 runtime 相关执行 | `server/internal/auth/password_revoke.go:15` | 界面区分服务端撤销结果和机器进程真实停止 |
| 发布撤销从请求用户读取断连对象 | `server/internal/handler/auth_password.go:441` | 操作别人时必须分离操作者与目标参数 |
| 封禁目前是源码临时 denylist | `server/internal/auth/temporary_disabled_users.go:12` | 新增持久状态时保留现有拒绝语义，不原样暴露名单 |
| 全局用户查询不代表全局权限；当前配置注释注明无平台管理员 | `server/pkg/db/queries/user.sql:9`；`server/internal/handler/handler.go:75` | 必须新增独立平台授权和投影查询 |
| auth-only 路由与 workspace 组已分层 | `server/cmd/server/router.go:1581`、`:1936` | 管理 API 可置于 auth-only 层并添加专属 guard |
| `admin` 已保留；现有 dashboard guard 等待 workspace | `server/internal/handler/reserved_slugs.json:29`；`packages/views/layout/dashboard-guard.tsx:28` | `/admin` 可用，但不能直接复用该 dashboard guard |
| 登录页支持安全的显式 `next` 优先跳转 | `apps/web/app/(auth)/login/page.tsx:79` | 后台回跳可复用 `/login?next=/admin`；仍需覆盖零工作空间测试 |

本轮未运行认证测试，也未变更运行服务、账号或凭据；这里只读取源码并产出设计。实现时须覆盖 `.trellis/spec/server/security-boundaries.md` 所列密码模式、派生凭据、双向 WebSocket 和回滚回归，不以这些观察代替测试结果。
