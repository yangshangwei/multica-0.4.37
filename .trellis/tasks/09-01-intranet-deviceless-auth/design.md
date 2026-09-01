# 技术设计：内网免登录（设备匿名身份）

## 总体形状

服务端多一条「凭设备标识自助换取 JWT」的登录路径并顺带开通身份；桌面端在「本地没有 token」这一分支里多一次自动登录尝试。门后的一切保持不变：`middleware.Auth` 仍是唯一入口，JWT、`workspace_id` 过滤、membership 门禁、daemon PAT 兑换全部沿用。

这是本设计最重要的选择：**免登录不是绕过鉴权，而是自助发 token**。

- 门后的服务端代码与全部前端 hooks 零改动。
- 出问题时影响面收敛在"发 token"这一处，一个开关可整体关闭。
- 审计链完整——每个动作依然归属一个具体 user id，协作语义因此成立。

被否决的替代方案：在 `middleware.Auth`（`server/internal/middleware/auth.go:51`）里当开关打开时注入固定的 `X-User-ID`。代码更少，但在鉴权层开洞、且丢掉设备区分（退化成单一共享账号），与需求冲突。

## 契约

### POST /auth/device（新增，公开路由组，复用 authRL 限流）

```
Request:  { "device_id": "<32-64 hex>", "device_name": "artisan@mac-mini" }
200:      { "token": "<jwt>", "user": { ...UserResponse } }
400:      invalid device_id（格式非法）
403:      device auth is not enabled on this instance（开关关闭）
429:      限流
```

响应形状与 `/auth/verify-code` 的 `LoginResponse`（`server/internal/handler/auth.go:100`）完全一致，客户端因此可复用既有解析路径。

### GET /api/config（新增字段）

```go
DeviceAuthAvailable bool `json:"device_auth_available,omitempty"`
```

`omitempty` 让云端与旧客户端看到的响应形状保持不变，沿用 `workspace_creation_disabled` 的既有惯例（`server/internal/handler/config.go:31`）。

### 环境变量

| 变量 | 缺省 | 说明 |
| --- | --- | --- |
| `MULTICA_DEVICE_AUTH_ENABLED` | `false` | 唯一总开关 |
| `MULTICA_DEVICE_AUTH_WORKSPACE` | `intranet` | 共享 workspace slug（已确认不在保留 slug 列表内） |
| `MULTICA_DEVICE_AUTH_WORKSPACE_NAME` | `Intranet` | 首次创建时的显示名 |
| `MULTICA_DEVICE_AUTH_ROLE` | `member` | 后续设备的角色（`admin` / `member`）；首个开通设备取 `owner` |

## 设备身份如何落库（关键取舍）

不新增表、不加迁移：把设备标识编码进 `"user".email` 的唯一键。

```
email = device-<device_id>@device.multica.local
name  = <清洗后的 device_name>，缺省 device-<device_id 前 8 位>
```

- 直接复用 `GetUserByEmail` + `CreateUser`，唯一键天然提供幂等；
- 零 schema 变更，也就完全不触碰"新表不加 FK"、"索引必须 `CONCURRENTLY` 且单独成文件"这些迁移硬约束。

代价：成员列表与详情页会显示这个合成邮箱。这些用户可通过 `@device.multica.local` 后缀识别，将来若要做"升级绑定真实邮箱"，筛选条件也是这个后缀。

被否决的替代方案：新建 `device_identity` 映射表。语义更干净，但要付一个迁移加一条 `CREATE UNIQUE INDEX CONCURRENTLY` 单独迁移文件的成本，对一个开关级功能不划算。

## 开通事务

在单个事务内完成（`h.TxStarter.Begin`，与 `CreateWorkspace` 同一 pattern，见 `server/internal/handler/workspace.go:254`）：

1. `GetUserByEmail` / `CreateUser`
2. `GetWorkspaceBySlug` / `CreateWorkspace` + `issuestatus.Ensure` —— 复用 `CreateWorkspace` 的 seeding，遵守 MUL-6243 的不变量：workspace 不得在没有状态目录时可见
3. `CreateMember` —— 已存在则跳过；workspace 由本次调用创建时取 `owner`，否则取配置角色
4. `MarkUserOnboarded` —— `COALESCE(onboarded_at, now())` 已经是幂等的

`Commit` 之后才 `issueJWT`，并复用既有的 analytics 事件与 `notifyDaemonWorkspacesChanged`。

为什么必须 stamp onboarding：桌面端的硬不变量是 `onboarded_at != null` 才能进入 dashboard（`apps/desktop/src/renderer/src/App.tsx:215-260` 的 overlay 路由），否则设备用户会被 onboarding overlay 拦住，"打开即用"直接不成立。

## 数据流（桌面端首次启动）

```
main      ensureDeviceIdentity() 读写 userData/device-identity.json
            ↓ (ipc sendSync，沿用 runtime-config 的既有 pattern)
preload   desktopAPI.deviceIdentity = { deviceId, deviceName }
            ↓
renderer  AuthInitializer 的"无 token"分支
            ↓ await loadConfig()
          deviceAuthAvailable ? api.deviceLogin(id, name)
                                  → storage.setItem("multica_token")
                                  → getMe → onAuthSuccess → warmWorkspaces → DesktopShell
                              : 现状（unauthenticated → DesktopLoginPage）
```

失败处理：`deviceLogin` 抛网络类错误 → 进入既有 `recovering` 重试机制（含 `online` 事件加速）；抛 403/400 → 判定为该服务端不支持，落登录页。

## 包边界与落点

| 位置 | 改动 |
| --- | --- |
| `packages/core/api/client.ts` | `deviceLogin()`，`parseWithFallback` + 复用 login response schema |
| `packages/core/api/schemas.ts` | `AppConfigSchema` 加 `device_auth_available`，`EMPTY_APP_CONFIG` 补 `false` |
| `packages/core/config/index.ts` | `configStore` 加 `deviceAuthAvailable` 与 setter，与 `vcsIntegrationAvailable` 同形 |
| `packages/core/platform/auth-initializer.tsx` | 新增可选 prop `deviceAuth?: { deviceId: string; deviceName?: string }` |
| `packages/core/auth/store.ts` | `loginWithDevice()`，与 `loginWithToken` 相同的 token 持久化路径 |
| `apps/desktop/src/main/device-identity.ts` | 新文件：标识生成与持久化（纯函数与 fs 分离，便于单测） |
| `apps/desktop/src/preload/index.ts` | 暴露 `deviceIdentity` |
| `apps/desktop/src/renderer/src/App.tsx` | 把 `deviceAuth` 传进 `CoreProvider` |

`packages/core` 不得引用 electron / `process.env`，因此设备标识必须由 app 层注入而不是 core 自己去读。`packages/views` 无改动，`DesktopLoginPage` 保留为回落路径。

## 兼容性

- 旧客户端 + 新服务端：多一个未知 JSON 字段，`AppConfigSchema` 是 `.loose()`，无影响。
- 新客户端 + 旧服务端（无此字段）：fail closed → `false` → 登录页，与今天行为一致。
- 新客户端 + 开关关闭的新服务端：同上。
- 桌面端已有 token 时不触发设备登录，不打扰已登录用户。
- 云端不受影响：开关默认 `false`，`omitempty` 不改变响应形状。

## 安全后果（已确认接受）

开关打开即意味着该部署的默认 workspace 没有鉴权：任何能访问后端端口的人都可以自助获得身份、读写全量数据，并触发 agent 任务（消耗 runtime 与模型额度）。用户已明确选择不做来源限制，缓解手段因此只剩三项：

1. 开关默认关闭，必须显式打开；
2. 打开时在服务端启动日志打 WARN，说明当前无鉴权；
3. 文档显式写明风险，并点明 `/auth/device` 不受 `ALLOW_SIGNUP` 约束。

## rollout / rollback

- rollout：服务端置 `MULTICA_DEVICE_AUTH_ENABLED=true` 并重启。桌面端不需要新配置，能力靠 `/api/config` 协商。
- rollback：置 `false` 并重启，桌面端下次启动自动回到登录页。已签发的 JWT 在 TTL 内仍然有效（与其他登录路径一致，不为此单独做吊销）；需要立即失效则轮换 JWT secret。
- 数据：已开通的设备用户与 workspace 保留，不做自动清理；如需清理，按 `@device.multica.local` 后缀筛选。
