# 桌面端退出登录后的页面设计

## 背景与目标

桌面端已改为强制私有化后端（`e816922`）：首次启动必须在 `DesktopEndpointSetupPage` 填写 `apiUrl`，配置落到 `~/.multica/desktop.json`。但退出登录后的落地页仍是公有云形态的通用登录页：标题「登录 Multica」、无条件显示「使用 Google 登录」、只有邮箱验证码一条路，既看不出连的是哪个服务器，也没有任何切换服务器的出口。

目标是让退出登录后落在一个「属于这台私有化部署」的登录页：一眼能看出连的是哪个服务器，只提供该服务器真正支持的登录方式，并保留一个次要的「切换服务器」出口 —— 同时不要把退出登录变成一次重新配置。

## 现状（已核对代码路径）

1. 侧边栏「退出登录」→ `useLogout()`（`packages/views/auth/use-logout.ts`）清 storage/cache → `push(paths.login())`。
2. 桌面导航适配器拦截 `/login`（`apps/desktop/src/renderer/src/platform/navigation.tsx:190`）→ 直接调 `authStore.logout()`，不产生任何路由。
3. `authStore.logout()`（`packages/core/auth/store.ts:120`）置 `user=null, status="unauthenticated"`，**不** bump `retryGeneration`。
4. `AppContent`（`apps/desktop/src/renderer/src/App.tsx`）渲染 `<DesktopLoginPage />` → 共享 `LoginPage`，即截图那一页。

由此暴露的三个实际缺陷：

**D1 — Google 按钮无条件出现。** `apps/desktop/src/renderer/src/pages/login.tsx` 恒定传 `onGoogleLogin`，共享登录页在 `(google || onGoogleLogin)` 为真时就渲染按钮。Web 端是按 `googleClientId` 门禁的（`apps/web/app/(auth)/login/page.tsx:221`）。没配 `GOOGLE_CLIENT_ID` 的私有化部署上，这个按钮把用户推到外部浏览器后无路可走。

**D2 — device-auth 部署上，退出登录等于自锁。** `device_auth_available` 的部署用设备 id 换会话，而 `AuthInitializer` 只在「启动且无 token」时尝试（`packages/core/platform/auth-initializer.tsx`），`logout()` 不 bump `retryGeneration`，那个 effect 不会重跑。代码注释自己写明这类部署「没有邮件中继、也没有可达的 OAuth」，于是退出后登录页上两个入口都是死的，只能重启 App。

**D3 — 邮箱验证码是默认入口，但客户端无从判断它可用。** 验证码走 Resend 或 SMTP（`server/internal/service/email.go`），两者都没配的部署收不到码，而 `/api/config`（`server/internal/handler/config.go`）目前不声明邮箱登录是否可用。

## 方案对比

| 方案 | 退出后展示什么 | 结论 |
| --- | --- | --- |
| A | 登录页保留，但带服务器身份、按服务器声明渲染登录方式、页脚内联「切换服务器」 | **推荐** |
| B | 直接复用 `DesktopEndpointSetupPage`（用户最初的想法） | 不推荐，见下 |
| C | 两步式 pre-auth 向导（服务器 → 身份），首次运行与退出后同构 | 结构更重，等要做「多环境地址列表」时再演进 |
| D | 只改标题文案 + Google 门禁 | A 的子集，可作为第一批发布 |

### 为什么不建议 B

- 退出登录不会让 `desktop.json` 失效。那个页面是配置表单，配置仍然有效时把它推到用户面前，等于要求他重新输入已经填过的地址。
- 保存路径自带副作用：`saveRuntimeConfig` 写文件后触发窗口重载，`embedded` 模式还会先停 daemon。放进退出流程等于每次退出都做一次「重启应用」。
- 它没有登录入口 —— 保存完仍然落回登录页。B 不是替换，而是在退出和登录之间插了一步。

B 真正指出的问题是：当前登录页完全没有暴露「我连的是哪个服务器」和「我要换服务器」。A 补上这两件事，而不动主流程。

## 推荐方案 A：页面设计

全窗口，顶部 `DragStrip`，单卡片居中（`max-w-sm`），视觉沿用 `endpoint-setup.tsx`（`MulticaIcon bordered size="lg"` + 标题 + 描述）。

```
┌──────────────── DragStrip ────────────────┐
│                                           │
│                  [ ✳ ]                    │
│        登录 multica.example.com            │  ← apiUrl 的 host
│      使用你在该部署上的账号登录               │
│                                           │
│   邮箱                                     │
│   ┌─────────────────────────────────────┐ │
│   │ you@example.com                     │ │
│   └─────────────────────────────────────┘ │
│   [               继续               ]     │
│   [        使用 Google 登录        ]  ← 仅当 google_client_id 非空
│                                           │
│   ─────────────────────────────────────    │
│   🖧 https://multica.example.com            │
│      已连接 · 服务端 v0.4.37   [切换服务器]  │  ← 次要 text button
└───────────────────────────────────────────┘
```

元素规则：

1. **服务器身份放标题。** 取 `new URL(runtimeConfig.config.apiUrl).host`。用户的诉求就是「这不该像公有云登录页」，标题是唯一一眼能看到的位置。页脚再给完整地址，供内网排障时核对。地址只在本地渲染，不进 telemetry / 错误上报（沿用 `docs/desktop-runtime-config-design.md` 的约束）。
2. **登录方式按服务器声明渲染，不按客户端猜测。**
   - Google 按钮：`useConfigStore(s => s.googleClientId)` 非空才传 `onGoogleLogin`（修 D1）。
   - `deviceAuthAvailable === true`：不显示邮箱表单，改为单个「以本机身份继续（<设备名>）」按钮，点击调 `authStore.loginWithDevice(deviceId, deviceName)`（修 D2）。
   - 邮箱验证码：默认保留。
3. **「切换服务器」是次要动作，且就地展开。** 同卡片内切到 `DesktopEndpointSetupPage embedded initialApiUrl={当前地址}`，预填当前地址，不给空表单。不要用 `WindowOverlay`：`<WindowOverlay />` 挂在 `desktop-layout.tsx` 里，`user === null` 时整个 shell 都没挂载，overlay 不会渲染。
4. **服务端版本行**：`serverVersion` 仅自建部署返回，有值才显示；可附桌面端 `appInfo.version`。内网排障用。
5. 不传 `extra` slot（「想用桌面应用？下载」在桌面端里没有意义），保持现状。

## 状态与异常

| 状态 | 页面行为 |
| --- | --- |
| 已退出（默认） | 上述布局；焦点落在邮箱输入（device-auth 部署落在「以本机身份继续」） |
| 验证码步骤 | 沿用共享 `LoginPage` 的 code 步骤，标题/页脚保持服务器身份 |
| 服务器不可达（`getConfig` 失败或发码失败） | 「无法连接到 {{host}}」+「确认已接入内网/VPN，且服务器已启动」，并给「重试」和「切换服务器」两个动作。现在的 `errors.server_unreachable` 只有一句话、没有出路，这是私有化部署最常见的失败 |
| `status === "recovering"` | 保持现有 `DesktopAuthRecoveryPage`，不变 |
| 配置缺失 / 损坏 | 仍然是 `DesktopEndpointSetupPage` 阻塞页，不变。这是 B 该出现的唯一场合 |
| device-auth 部署 | 建议同时把侧边栏「退出登录」在该部署上改为「切换服务器」：设备身份下退出本身没有意义，一键就能回来 |

## 实现边界

- `packages/views/auth/login-page.tsx`：新增可选 `title` / `description` 覆盖与 `footer` slot，Web 端行为保持不变（不传即现状）。不要在这里读 Electron API。
- `apps/desktop/src/renderer/src/pages/login.tsx`：条件传 `onGoogleLogin`；组装标题（host）与页脚（地址 + 已连接 + 版本 + 切换服务器）；device-auth 分支；内联「切换服务器」视图状态。
- `apps/desktop/src/renderer/src/pages/endpoint-setup.tsx`：已有 `embedded` / `initialApiUrl`，直接复用，不改签名。
- 文案进 `packages/views/locales/{en,zh-Hans,ko,ja}/auth.json`，新增 `desktop.signin.*`：`title`（`登录 {{host}}`）、`description`、`connected`、`server_version`、`change_server`、`device_continue`、`device_hint`、`unreachable_title`、`unreachable_description`、`retry`。
- 后端（可选，修 D3）：`/api/config` 增 `email_auth_available`（`RESEND_API_KEY` 或 `SMTP_HOST` 非空）。**客户端 absent 必须读作 true** —— 老服务器邮箱登录就是唯一入口，猜 false 会隐藏唯一的登录方式；猜错方向只是多显示一个表单。这与 `device_auth_available` 的 absent→false 方向相反，原因是两者「猜错」的代价不对称。

## 测试与验收

- `packages/views/auth/login-page.test.tsx`：`title`/`description`/`footer` 覆盖生效；`google` 与 `onGoogleLogin` 都不传时不渲染 Google 按钮。
- 新增 `apps/desktop/src/renderer/src/pages/login.test.tsx`：`googleClientId === ""` 时无 Google 按钮；`deviceAuthAvailable` 时渲染「以本机身份继续」且不渲染邮箱表单；点击「切换服务器」展开且预填当前 `apiUrl`；标题显示 host 而非「登录 Multica」。
- 若做 `email_auth_available`：`server/internal/handler/config_test.go` 覆盖有/无 SMTP 与 Resend；客户端加一条 absent→true 的解析测试。
- 手工验收：配了 Google 的部署、没配 Google 的部署、device-auth 部署（退出后能回来）、服务器停机时退出登录、切换到另一个地址后重载与 daemon profile 是否跟随、中文/英文 locale。

## 分阶段发布

1. **阶段 1（纯 UI，低风险）**：Google 门禁 + 标题显示 host + 页脚服务器行。等于方案 D，先解决「看起来像公有云登录页」。
2. **阶段 2（缺陷修复）**：device-auth 的「以本机身份继续」入口 + 该部署上侧边栏退出项的处理。如果你的部署已经开了 device auth，这一阶段应该提到阶段 1 之前。
3. **阶段 3**：「切换服务器」内联入口 + 不可达状态的重试/切换出路。
4. **阶段 4（可选）**：后端 `email_auth_available` 声明，把邮箱表单也纳入「按服务器声明渲染」。
