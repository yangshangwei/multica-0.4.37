# 用户名密码注册 Implementation Plan

> 执行时按 Trellis reviewed planning → task.py start → 实现/检查流程推进；可结合 superpowers:executing-plans。用户已批准开发，实施与验证记录见 verification.md。

**Goal:** 服务器连接后以用户名、密码、姓名自助注册，复用现有欢迎引导并保留旧用户数据。

**Architecture:** 原 user 与业务数据不重建，独立密码凭据、统一 JWT/cookie；服务端统一认证模式。共享 core/views，Desktop/Web 负责平台接线。

**Tech Stack:** Go 1.26 / Chi / sqlc / PostgreSQL；React / Zustand / TanStack Query / Electron / Next.js；Go testing、Vitest、Playwright。

## 执行约定

下列“新增”均为建议路径，不表示文件已经存在。每阶段先补该层的行为回归测试，确认预期失败，再实现、运行窄检查并审查差异。只在共享契约稳定后并行独立 UI 与服务端切片；不得并行修改同一 auth/schema 文件。提交按仓库 Lore protocol，在独立工作树实现，按实际验证结果提交。

## 1. 固定基线和契约

已有：CLAUDE.md、research/current-state.md、server/internal/handler/auth_device.go、packages/core/platform/auth-initializer.tsx。

- [x] 读取 JSONL spec/research；复核 task 文档与当前 HEAD。
- [x] 枚举 email、JWT、PAT、daemon、WS 消费点，将撤销矩阵补入迁移实施记录。
- [x] 固定 password 模式/config/error/interface 契约；将产品或范围变更反馈到设计。
- [x] 记录现有 auth/device/onboarding 窄测试基线。

## 2. 数据层与密码原语

新增：server/internal/auth/password.go、password_test.go；server/pkg/db/queries/password_credential.sql；server/migrations/<next>_*.sql。
修改：server/pkg/db/queries/user.sql、sqlc 生成文件、用户响应/邮箱消费者。

- [x] 先测 KDF 编码/校验/边界及 nullable email 映射。
- [x] 分迁移实现 email nullable、credential 表和两条 concurrent unique index；版本初始1。
- [x] 使用标准库 PBKDF2，不引入新依赖；基准 600k 工作因子并记录结果。
- [x] make sqlc；明确查询凭据，不将 hash 加入公共模型。
- [x] 验证新/老用户与索引有效性；此阶段不要打开 password 开关。

命令：`make sqlc`；`go test ./internal/auth`（工作目录 server）；`go vet ./internal/auth`。

## 3. 服务端模式、注册、登录、限流

新增：server/internal/handler/auth_password.go、auth_password_test.go；server/internal/middleware/password_ratelimit.go、password_ratelimit_test.go。
修改：server/internal/handler/config.go、auth.go、server/cmd/server/router.go、server/cmd/server/main.go。

- [x] 写 T01/T03/T05/T08 失败测试，DB fixture 使用 internal/testutil。
- [x] 实现注册事务、明确用户名冲突；login 不自动创建账号。
- [x] 复用会话签发/响应；拒绝空邮箱的邮箱登录与邀请误用。
- [x] password 模式路由和 handler 双重门禁；加入 capability；无 Redis/故障限流有明确行为。
- [x] 窄测试通过后审查错误/日志不会泄漏密码。

命令（server）：`go test ./internal/handler ./internal/middleware ./cmd/server -run 'Test(Password|Register|Device|MachineCredential)' -count=1`。测试命名与实现保持对应，DB 测试需使用隔离已迁移数据库。

## 4. 绑定、会话失效与运维恢复

新增：server/internal/handler/auth_password_setup.go、auth_password_setup_test.go；受控服务端密码恢复命令（最终落点在核查 cmd/server 与 cmd/multica 运维边界后确定）。
修改：server/internal/middleware/auth.go、auth.go、对应 token 查询/WS 认证路径。

- [x] 先写版本缺失、重放、并发、机器凭据和业务 gate 拒绝测试。
- [x] 实现版本校验和旧 JWT 受限迁移；绑定目标来自会话、保留 UUID。
- [x] 实现 PAT/daemon 等存量凭据撤销和旧 WS 失效，不能只让登录页退出。
- [x] 实现运维恢复与强制更改临时密码受限分支；完整契约见 design.md。
- [x] 演练绑定后签发失败、恢复登录、无旧会话的人工恢复。

命令（server）：`go test ./internal/auth ./internal/handler ./internal/middleware ./cmd/server -count=1`；`go vet ./internal/auth ./internal/handler ./internal/middleware ./cmd/server`。

## 5. 共享 API 与认证状态

修改：packages/core/api/client.ts、schemas.ts、auth/store.ts、config/index.ts、platform/auth-initializer.tsx 及同目录测试。

- [x] malformed response、成功epoch、旧401竞态、password 禁 device fallback 测试先行。
- [x] 新增 register/login/setup/change 方法，使用现有 schema fallback 并校验可用 session。
- [x] 受限绑定/改密码状态优先于业务数据预热和 onLogin daemon 同步。
- [x] UserSchema 的 username/setup 字段可缺省，email 保持字符串边界。

命令：`pnpm --filter @multica/core test`。

## 6. 表单与平台接线

新增：packages/views/auth/password-auth-form.tsx、password-account-setup.tsx 及对应测试。
修改：packages/views/auth/login-page.tsx、locales/en 与 zh 认证文案；apps/desktop/src/renderer/src/pages/login.tsx、App.tsx、stores/window-overlay-store.ts、components/window-overlay.tsx；apps/web/app/(auth)/login/page.tsx 与所需平台绑定入口。

- [x] 先测表单字段、可访问性、错误和成功导航。
- [x] 平台壳使用共享组件；桌面新绑定流程不添加预空间路由。
- [x] 复用 OnboardingFlow，保留 about_you 问卷和 onboarded_at 语义。
- [x] 老用户按服务器状态进入绑定，不能仅按邮箱后缀猜测。
- [x] 验证 cookie/bearer、已有 CLI 授权回调，password 模式不露邮箱入口。

命令：`pnpm --filter @multica/views test`；`pnpm --filter @multica/desktop test`；`pnpm --filter @multica/web test`；`pnpm typecheck`。

## 7. 服务器切换隔离

修改：apps/desktop/src/renderer/src/pages/endpoint-setup.tsx、platform/session-teardown.ts、main/index.ts 的 runtime-config IPC；复用 packages/core/platform/session-cleanup.ts。

- [x] T12 捕获 B 收到 A token 的失败测试。
- [x] 实现切换前退出旧会话、取消请求、清缓存、daemon/WS 和多窗口协调。
- [x] 验证保存失败、重载失败、旧响应晚到、cookie 和 bearer 均不跨源。
- [x] 补健康探测能力兼容提示；不把旧 health HTTP 200 等同协议兼容。

命令：`pnpm --filter @multica/desktop test`；`pnpm --filter @multica/core test`。

## 8. 发布资料与综合验收

修改：.env.example、docker-compose.selfhost.yml、SELF_HOSTING.md、相关认证帮助；如触及内置技能行为同步其引用。
新增：e2e/password-registration.spec.ts、迁移与切换集成测试（按既有测试设施选择正确层）。

- [x] 检查 migration-rollout.md 所有发布前条件和兼容矩阵；真实部署验证保留为发布门禁。
- [x] 执行 test-spec.md 的本地可执行验收并记录覆盖限制；未调用真实 agent CLI 或发送公司邮件。
- [ ] 在目标部署完成容量、已发布客户端升级和运维终端等发布前验收，见 verification.md。
- [x] `pnpm lint`、`pnpm typecheck`、`pnpm test`、`make test`、`pnpm knip`。
- [x] `pnpm exec playwright test e2e/password-registration.spec.ts`（实现文件后运行）；按仓库要求 `make check`。
- [x] 独立安全/架构复核，确认绑定 gate、机器边界、限流、nullable email、回滚和凭据撤销。
- [x] 记录通过/失败/未测项和新增文件；完整流水线的单个 E2E 定位问题修正后，全站复验通过。
- [ ] 合并与部署验收完成后归档任务。

## 回滚点

数据库阶段保留兼容但未启用模式；服务端功能可在 password 未启用时验证；客户端发布后才切换目标部署。已经产生密码账号后，回滚必须保留 nullable email 与 credential 兼容，不能降到旧设备免登录。详细恢复边界见 migration-rollout.md。

## 评审后补入各阶段的要求

- 阶段 1：读取 security-contract.md，固定错误码及凭据种类 context；时间窗用绝对 UTC 配置，password 禁 Cloud Fleet。
- 阶段 2：增加 PAT/task_token.auth_version、daemon_token.user_id/auth_version 的 additive migrations；旧值为 0/NULL。禁止把旧 daemon owner 猜填为当前 owner。
- 阶段 3：实现原子固定窗 limiter、明确 single/shared 配置、可信代理解析和共用 KDF 门禁。
- 阶段 4：签发事务锁定用户并复核来源版本；包括 task/Remote MCP/member CallbackGrant。password 绕过 PAT/daemon 缓存；WS 发送前校验；保留独立 plugin actor。不能调用 revokeAndRemoveMember 删除账号业务授权。
- 阶段 5：正常/绑定/临时密码会话显式分流，旧成功回包也有代际保护；Desktop bearer 与 Web cookie 传输策略明确分开。
- 阶段 7：主进程串行切换、只允许主窗口发起、处理 HttpOnly cookie、禁用切换中 daemon sync，保存前等待清理，清理失败不激活 B。
- 阶段 8：恢复子命令随现有 server 镜像/离线包交付；完成新增竞态、静默 WS、多窗口及 same-host/different-port 测试再验收。

上述补充已纳入实现。部署环境验收保持独立，未在生产启用。
