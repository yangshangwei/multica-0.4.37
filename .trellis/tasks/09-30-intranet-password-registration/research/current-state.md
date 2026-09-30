# 当前实现依据

核查日期：2026-09-30。只读追踪，不代表新方案已实现。以下均为已有文件；拟新增文件列于 implement.md。

| 领域 | 代码依据 | 当前行为与设计影响 |
|---|---|---|
| 设备身份 | apps/desktop/src/main/device-identity.ts:14、:66 | 16 随机字节转 hex，保存 userData/device-identity.json；不是人员身份。 |
| 设备开户 | server/internal/handler/auth_device.go:163、:197、:286 | 合成邮箱映射，设备 ID 可直接换 JWT，可能 provision 共享空间；新注册不能复用此授权条件。 |
| 服务器设置 | apps/desktop/src/renderer/src/pages/endpoint-setup.tsx:94；apps/desktop/src/main/index.ts:805、:831 | 已有健康探测及配置保存/reload，未有完整密码协议版本兼容检查。 |
| 切换风险 | packages/core/platform/core-provider.tsx:89 | 全局 multica_token hydrate 新 API；当前保存服务器地址没有完整清理旧令牌，必须修复。 |
| 清理复用 | packages/core/platform/session-cleanup.ts；apps/desktop/src/renderer/src/platform/session-teardown.ts | 复用退出及缓存清理，不另写一套遗漏分支的清理。 |
| 邮箱登录 | server/internal/handler/auth.go:369 | 验证码验证后 findOrCreateUser，再签 JWT/cookie；不能去掉验证即当密码登录。 |
| 用户 schema | server/migrations/001_init.up.sql:5；server/pkg/db/queries/user.sql | email UNIQUE NOT NULL，通用 SELECT *；密码应放独立凭据表，无邮箱须明确处理。 |
| JWT | server/internal/handler/auth.go:153；server/internal/middleware/auth.go:230 | 已有签名/过期验证，缺少 session_version；Logout 清 cookie 不会吊销既有 JWT。 |
| 认证能力 | server/internal/handler/config.go:116；packages/core/api/schemas.ts:966 | 公共 config 可扩展可选字段；缺失不能判定支持密码。 |
| API 边界 | packages/core/api/client.ts:947；packages/core/api/schema.ts；packages/core/api/schemas.ts:2664 | 复用 LoginResponse、fallback 与 auth epoch，必须校验 session 可用。 |
| 共享认证 | packages/core/auth/store.ts；packages/core/platform/auth-initializer.tsx | bearer/cookie、重试和设备 fallback；password 模式必须关闭 fallback。 |
| 登录 UI | packages/views/auth/login-page.tsx；apps/desktop/src/renderer/src/pages/login.tsx；apps/web/app/(auth)/login/page.tsx | 共享表单与平台壳；保留成功回调、CLI 登录衔接与恢复路径。 |
| 欢迎引导 | packages/views/onboarding/onboarding-flow.tsx；packages/core/onboarding/step-order.ts | welcome → about_you → workspace → runtime；about_you 是职业/用途问卷，不是账号姓名。 |
| 引导持久化 | packages/views/onboarding/steps/step-workspace.tsx；apps/desktop/src/renderer/src/App.tsx:216 | 桌面是 overlay；创建/恢复空间沿用现有规则；注册不能提前标记 onboarded_at。 |
| 限流缺口 | server/internal/middleware/ratelimit.go:52；server/cmd/server/router.go:1404 | 无 Redis 不限流、Redis 错误放行；密码入口需 fail-closed 策略。 |
| 密码依赖 | server/go.mod:3 | Go 1.26.6，未有 x/crypto；标准库 PBKDF2 可避免新依赖。 |
| 机器边界 | .trellis/spec/server/security-boundaries.md | 机器凭据不能铸造人类凭据；绑定必须收紧为 JWT/cookie，而非笼统 human actor。 |
| 部署 | .env.example；docker-compose.selfhost.yml；SELF_HOSTING.md | 新模式需同时配置透传，不能只改 .env 模板。 |

## 实现前的技术核验

1. 盘点所有 email nullable 编译影响与直接 DB 消费者，确认对外字符串兼容。
2. 枚举所有可续签凭据、WS 及 daemon 链路，明确绑定/重置的撤销事务与在途任务终止行为。
3. 在实际内网 CPU 测 KDF 和限流；确认单实例/多实例部署配置。
4. 验证 Electron 多窗口切换广播、cookie 清理和 daemon endpoint 更新顺序。
5. 检查安装客户端的最低版本/升级提示现有实现，不假设旧客户端能理解新 capability。

这些是验证任务，不得在未测前报告安全或兼容性通过。
