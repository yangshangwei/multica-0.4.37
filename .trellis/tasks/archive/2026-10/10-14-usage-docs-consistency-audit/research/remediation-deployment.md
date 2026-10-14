# 部署与规范组修订记录

日期：2026-10-14。负责 D04、D12、D13、D14、D15、D16。

本组直接复核当前共享工作区的源码后修订文档；原始 `findings.json` 和阶段 A 报告保持为历史证据。源码核对和限定范围静态检查已完成；文档包生成、MDX 编译、测试与独立复核由主会话统一执行。

## 文件范围

- `apps/docs/content/docs/desktop-app.zh.mdx`
- `apps/docs/content/docs/environment-variables.zh.mdx`
- `apps/docs/content/docs/security-model.zh.mdx`
- `apps/docs/content/docs/self-host-quickstart.zh.mdx`
- `apps/docs/content/docs/developers/conventions.zh.mdx`
- `apps/docs/content/docs/developers/conventions.mdx`
- 本记录。

上述六个 MDX 在本组开始时均无已有未提交修改。本组未写业务代码、测试、生成包、任务元数据、spec 或其他页面。`auth-setup.zh.mdx` 仅作为已有配置说明读取和引用。

## D04：默认登录流程

- 修订位置：`self-host-quickstart.zh.mdx:191` 的第 4 步、常见问题及后续链接；`desktop-app.zh.mdx:21` 的数据共享前提和安装流程；`environment-variables.zh.mdx:23` 的最小配置补充说明。
- 结果：区分默认自托管设备身份、显式 `password`、`legacy` 关闭设备身份后的邮箱验证码流程。日志取码只放在未配置邮件服务的邮箱路径下。浏览器和 Desktop 默认保存不同设备身份；清理本地数据不会自动恢复旧身份的数据。没有引入账号迁移操作。
- 当前源码：`docker-compose.selfhost.yml:118`、`:137` 的默认值；`server/internal/handler/auth_device.go:99` 的开关解析、`:198` 的模式门禁、`provisionDeviceIdentity` 的设备账号和共享工作空间分支；`server/internal/handler/config.go:157` 的客户端能力声明；`packages/core/platform/auth-initializer.tsx:425` 自动设备登录；`server/internal/auth/password_config.go:25` 的密码模式约束；`server/internal/handler/auth_password.go` 的 `PasswordRegister` / `PasswordLogin`。
- 身份存储依据：`apps/web/features/auth/device-identity.ts` 的 `loadOrCreateWebDeviceIdentity` 使用当前站点存储；`apps/desktop/src/main/device-identity.ts` 的 `deviceIdentityFilePath` / `loadOrCreateDeviceIdentity` 使用应用用户数据目录。
- 限制：未操作真实账号、SMTP 或安装版客户端；完整认证配置复用 `/auth-setup`，不从教程复制迁移步骤。

## D12：按模式描述限流

- 修订位置：`environment-variables.zh.mdx:153` 的 Redis 与限流段。
- 结果：`legacy` 的通用逐 IP 限流依赖 Redis，缺少 Redis 或运行时失败时跳过这一层检查；`password/single` 使用进程内存；`password/shared` 必须配置 Redis，运行时失败返回 `503`；密码超限为 `429`。邀请单独列出内存后端及限流预检查失败的 `503`。保留实时事件在无 Redis 时使用进程内存的说明。
- 当前源码：`server/internal/auth/password_config.go:44` 的 `single/shared` 校验；`server/internal/auth/password_limiter.go:50` 的 Redis/内存分支；`server/internal/handler/auth_password.go:33` 的 `503/429`；`server/cmd/server/router.go:1427` 的通用认证路由接线；`server/internal/middleware/ratelimit.go:60` 的无 Redis 及错误放行；`server/internal/handler/invitation_rate_limiter.go:89` 的限流预检查、`:175` 的 `503`，以及该文件的内存 / Redis 工厂。
- 源码复核细化：通用 limiter 还覆盖 `/auth/google` 和 `/auth/device`；设备登录使用 `RATE_LIMIT_AUTH_VERIFY`，各接口独立计数。因此正文明确列出邮箱、Google、设备身份，不把 legacy 层误说成仅邮箱。密码客户端 IP 解析使用 `server/internal/auth/password_limiter.go:106` 的 `MULTICA_TRUSTED_PROXIES`，变量说明一并补齐。
- 限制：未制造 Redis 故障。邀请的描述限定为预检查失败；成功预检查之后扣额度时的短暂错误允许当前请求继续（`consumeInvitationGates`），不把邀请路径写成每个 Redis 错误都拒绝。

## D13：原地目录与串行范围

- 修订位置：`security-model.zh.mdx:38` 的托管工作目录说明及 `:42` 的原地模式例外。
- 结果：区分普通托管目录、独立 Git 工作树与 `local_directory/in_place`。原地执行会直接修改当前分支和未提交文件，目录锁不产生副本，也不阻止人工或其他进程写入。新增到 `/project-resources#任务如何共享这个目录` 的链接。
- 当前源码：`server/internal/daemon/execenv/execenv.go:492` 准备普通 workdir，`:500` 将 `LocalWorkDir` 直接作为执行目录，后续 `PrepareLocalWorktree` 分支准备独立工作树；`server/internal/handler/project_resource.go:119` 的两种模式契约；`server/internal/daemon/local_directory.go:237` 按真实路径生成锁键。
- 源码复核细化：`server/internal/daemon/local_directory.go:151` 的 `localDirectoryLockExempt` 对 `ChatSessionID` 非空的执行免锁，`server/internal/daemon/daemon.go:5471` 实际跳过排队。因此正文将串行范围限定为同一守护进程内的非对话执行，并说明对话执行跳过目录锁。
- 限制：未启动真实智能体、修改真实用户目录或验证外层容器/虚拟机隔离。

## D14：Desktop 的 HTTP 与 WebSocket 条件

- 修订位置：`desktop-app.zh.mdx:160` 的连接提示。
- 结果：将 HTTPS/WSS 安全建议与客户端连接条件分开。Desktop 支持内网 HTTP API 并推导 `ws`；API 与 WebSocket 地址需要可达，反向代理需转发 WebSocket 升级请求；打开 Web 页面或授权链接时还需要可达的 `appUrl`。
- 当前源码：`apps/desktop/src/shared/runtime-config.ts:167` 的 HTTP/HTTPS 校验未限制环回地址，`:110` 将 HTTP/HTTPS 分别推导为 WS/WSS；`apps/desktop/src/main/server-switch.ts:22` 的 `probeServer` 请求 `/health` 和 `/api/config`，无额外 HTTPS 门禁。`apps/desktop/src/shared/runtime-config.test.ts:84` 含非环回 HTTP 解析用例，本组仅阅读该既有测试。
- 限制：未执行安装版 Desktop 的远程 HTTP/HTTPS 连接、证书或反向代理验收；配置解析支持某协议不等于任意部署配置都能连通。

## D15：API 字段与查询键

- 修订位置：中英文 `developers/conventions` 的 `TypeScript` 段，均在 `:67`。
- 结果：API 与领域数据字段保留 `snake_case`，局部变量和参数使用 `camelCase`；不再声称 API client 统一改写键名。工作空间范围的 Query key 必须携带 `wsId`，示例为 `issueKeys.detail(wsId, id)`。
- 当前源码：`packages/core/api/client.ts:1894` 的 `getIssue` 经 `IssueSchema` 校验后直接返回；`packages/core/api/schemas.ts:1274` 和 `packages/core/types/issue.ts:161` 保留 `workspace_id` 等字段；`packages/core/issues/queries.ts:118` 的工厂签名为 `detail(wsId, id)`。
- 限制：`client.ts` 在本组执行期间由其他会话编辑，`getIssue` 行号从审计时的 1875 移至 1894；已重新读取当前函数确认上述行为未改变，后续引用应优先定位符号。

## D16：标识符与显示名

- 修订位置：`developers/conventions.zh.mdx:118` 的核心区分和 `:213` 的角色/状态段；英文对应段为 `:121`、`:215`。
- 结果：API、数据库、CLI 参数和代码标识符保持英文；内置角色与状态的 UI 标签跟随语言。两版文档列出同一组中文标签，示例改为“所有者”“进行中”。状态显示复用 `useStatusLabel(wsId)`，自定义状态保留工作空间设置的名称；未知/未加载的自定义状态回退原 key。
- 当前源码：`packages/views/issues/utils/status-label.ts:22` 的内置 i18n 与自定义目录查找分支；`packages/views/locales/zh-Hans/issues.json:86` 和 `packages/views/locales/zh-Hans/members.json:3` 的显示名；`packages/views/settings/components/members-tab.tsx` 的 `useRoleLabels` 使用 locale 标签。
- 限制：只同步相同契约的英文段落，不开展英文全站审计。两版角色/状态标题已更新；在文档源及产品 docs 链接范围内搜索未找到旧标题的入站片段引用。

## 验证记录与交接

已运行并读取退出码：

```bash
git diff --check -- \
  apps/docs/content/docs/desktop-app.zh.mdx \
  apps/docs/content/docs/environment-variables.zh.mdx \
  apps/docs/content/docs/security-model.zh.mdx \
  apps/docs/content/docs/self-host-quickstart.zh.mdx \
  apps/docs/content/docs/developers/conventions.zh.mdx \
  apps/docs/content/docs/developers/conventions.mdx
```

最终源文档修改后运行，退出码 `0`，无空白错误。逐项人工对照上述源码；另用临时只读脚本确认新增/保留的三个链接目标标题存在：`auth-setup` 的“开启密码登录”、`project-resources` 的“任务如何共享这个目录”、`self-host-quickstart` 的“3. 选择访问方式”。没有新增逐字断言产品文案的测试。

主会话接手：统一生成嵌入包，执行 docs 测试、typecheck、MDX 编译、Go docs/handler 检查、全量链接/锚点/资源检查与独立 `trellis-check`。本组不据源码核对宣称这些共享检查或真实运行验证已通过。
