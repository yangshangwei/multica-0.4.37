# 执行计划：内网免登录（设备匿名身份）

每一步都是纯增量且受开关保护，任一步可独立回滚（`git revert` 对应 commit）。开关默认关闭，因此在最后一步之前，云端与现有自托管部署的行为都不会变化。

## S1 服务端开关与能力声明

- [x] 在 handler 配置里读取 `MULTICA_DEVICE_AUTH_ENABLED` / `MULTICA_DEVICE_AUTH_WORKSPACE` / `MULTICA_DEVICE_AUTH_WORKSPACE_NAME` / `MULTICA_DEVICE_AUTH_ROLE`，缺省值见 `design.md`。
- [x] 开关为 true 时在服务启动日志打一条 WARN：该部署无鉴权（配置了共享 workspace 时额外说明其中数据可读写；slug 非法被忽略时再补一条）。
- [x] `AppConfig` 增加 `DeviceAuthAvailable bool` + `device_auth_available,omitempty`，在 `GetConfig` 中按开关赋值。
- [x] 测试：`server/internal/handler/config_test.go` 覆盖开关开/关两种 `/api/config` 响应。

验证：`cd server && go test ./internal/handler/ -run 'Config'`

## S2 服务端 POST /auth/device

- [x] 新增 `server/internal/handler/auth_device.go`：请求解析、`device_id` 格式校验（32–64 hex）、开关关闭返回 403。
- [x] 开通事务：`GetUserByEmail`/`CreateUser` → `GetWorkspaceBySlug`/`CreateWorkspace` + `issuestatus.Ensure` → `CreateMember`（已存在跳过） → `MarkUserOnboarded`，全部走 `h.Queries.WithTx(tx)`。
- [x] commit 之后 `issueJWT`，并复用既有 analytics 事件与 `notifyDaemonWorkspacesChanged`。
- [x] 在 `server/cmd/server/router.go` 公开组挂载 `r.With(authRL).Post("/auth/device", h.DeviceLogin)`，紧邻现有 `/auth/*`。
- [x] 测试 `server/internal/handler/auth_device_test.go`，用 `testutil.Call` + `dbfx`：
  - [x] 开关关闭 → 403，且没有新建 user/workspace/member
  - [x] 首次调用 → 201/200 返回 token 与 user，workspace 与 member 建好，issue 状态齐全，`onboarded_at` 非空
  - [x] 同一 `device_id` 二次调用 → 同一 user id，不新增 member 行
  - [x] 不同 `device_id` → 两个 user，同一 workspace，角色分别为 owner 与配置角色
  - [x] 非法 `device_id`（空、过短、含非 hex、超长）→ 400
  - [x] workspace 已存在时复用且不修改其属性
  - [x] `ALLOW_SIGNUP=false` 时该路径仍可开通

验证：`cd server && go test ./internal/handler/ -run 'Device' -count=1`

**评审门：** 服务端契约（端点形状、开通语义、角色策略）在这里定稿，继续之前先确认。

## S3 core：契约与登录方法

- [x] `packages/core/api/schemas.ts`：`AppConfigSchema` 加 `device_auth_available: BooleanWithDefaultSchema(false).optional()`；`EMPTY_APP_CONFIG` 补 `device_auth_available: false`（fail closed）。
- [x] `packages/core/api/client.ts`：`deviceLogin(deviceId, deviceName?)`，走 `parseWithFallback` + 现有 login response schema。
- [x] `packages/core/config/index.ts`：`configStore` 加 `deviceAuthAvailable` 与 setter；`auth-initializer` 的 config 载入处按 `cfg.device_auth_available === true` 写入。
- [x] `packages/core/auth/store.ts`：`loginWithDevice()`，token 持久化与 `api.setToken` 与 `loginWithToken` 一致。
- [x] 测试：`packages/core/api/schema.test.ts` 补 `/api/config` 字段缺失/类型错误 → false，以及 `/auth/device` 畸形响应不抛穿；`packages/core/auth/store.test.ts` 补 `loginWithDevice` 成功与失败路径。

验证：`pnpm --filter @multica/core test`

## S4 core：启动时自动设备登录

- [x] `packages/core/platform/auth-initializer.tsx` 新增可选 prop `deviceAuth?: { deviceId: string; deviceName?: string }`。
- [x] 改写「无 token」分支：有 `deviceAuth` 时先 `await loadConfig()`，`deviceAuthAvailable` 为 true 则调用 `loginWithDevice`；否则维持现状 settle 为 unauthenticated。
- [x] 网络类失败接入既有 `scheduleRetry` / `online` 机制，保持 `recovering` 状态；403/400 判定为不支持并落登录页。
- [x] 确认 `CoreProvider` 透传该 prop。
- [x] 测试 `packages/core/platform/auth-initializer.test.tsx`：
  - [x] 无 `deviceAuth` → 行为与今天完全一致（回归保护）
  - [x] 有 `deviceAuth` 且服务端声明可用 → 自动登录并 authenticated
  - [x] 有 `deviceAuth` 但服务端未声明 → unauthenticated
  - [x] 已有 token → 不调用 `deviceLogin`
  - [x] config 请求失败 → 不误判为可用，且恢复后仍能完成设备登录

验证：`pnpm --filter @multica/core test`

## S5 桌面端设备标识与接线

- [x] 新增 `apps/desktop/src/main/device-identity.ts`：纯函数（生成 32 hex、清洗显示名、解析已存文件）与 fs 读写分离；默认显示名 `<os user>@<hostname>`；文件缺失或损坏则重新生成。
- [x] 主进程注册 `device-identity:get` 的 ipc handler，`apps/desktop/src/preload/index.ts` 暴露 `desktopAPI.deviceIdentity`（沿用 `runtime-config:get` 的 `sendSync` pattern），同步更新 `index.d.ts`。
- [x] `apps/desktop/src/renderer/src/App.tsx` 把 `deviceAuth` 传给 `CoreProvider`。
- [x] 测试 `apps/desktop/src/main/device-identity.test.ts`（首行 `// @vitest-environment node`）：生成格式、跨调用稳定、损坏文件重建、显示名清洗。

验证：`pnpm --filter @multica/desktop test && pnpm typecheck`

## S6 文档

- [x] `SELF_HOSTING.md` 增一节：内网免登录的开关与四个环境变量、启用步骤、以及「等于该 workspace 无鉴权」的风险说明，并点明不受 `ALLOW_SIGNUP` 约束。
- [x] 若 `SELF_HOSTING_ADVANCED.md` 的 Signup Controls 一节会让读者误以为能拦住这条路径，就地加一句交叉引用。

## S7 全量验证与手工端到端

- [ ] `cd server && go test ./internal/handler/... ./internal/middleware/...` —— **未执行：本机没有 Go 工具链**（`go`/`gofmt` 均不存在，Homebrew 也没装）。测试已写好，需在有 Go + PostgreSQL 的环境或 CI 上跑。
- [x] `pnpm typecheck` —— 9/9 workspace 通过。
- [x] `pnpm test` —— 5/5 task 通过（含本次新增的 21 个用例）。
- [x] `pnpm lint` —— 0 error；仅本次未触碰文件的既有 warning。
- [ ] 手工：本地起后端并置开关为 true，用两份不同的 Electron userData 模拟两台设备，验证 PRD 验收标准里的双设备协作项（互相分配、@提及、inbox）。 —— **未执行：需要一个跑起来的后端**（本机无 Docker/PostgreSQL）。
- [ ] 手工：置开关为 false 重启，确认桌面端回到登录页且邮箱验证码登录仍正常。 —— **未执行：需要一个跑起来的后端**（本机无 Docker/PostgreSQL）。

## S11 默认开启 + web 端免登录（2026-09-01 用户变更）

用户要求：「Web 版和桌面版我都不需要登录，请帮我默认开启」。这推翻了 PRD 里的两条决策
（默认关闭、不改 web 端登录门），两条都已在 `prd.md` 就地改写并标注变更来源。

- [x] 服务端默认值反转：`DeviceAuthEnabledFromEnv()`（`auth_device.go`）取代 router 里的
  `os.Getenv(...) == "true"`。自托管默认开，仅 `MULTICA_APP_URL`/`FRONTEND_ORIGIN` 指向
  multica.ai 时默认关。**falsey 拼写全部识别**（`false`/`0`/`no`/`off`，大小写与空格无关）：
  默认这个方向下，一个没被识别的 `0` 会把运维正要关掉的东西给他打开。
- [x] 启动 WARN 带上关闭方法（`MULTICA_DEVICE_AUTH_ENABLED=false`）——默认开启意味着运维
  从没设过这个变量，没有别的途径知道它叫什么。
- [x] web 设备标识 `apps/web/features/auth/device-identity.ts`：localStorage 存 32 位 hex，
  显示名 `web-<os>-<id 前 8 位>`。存储不可读或不可写时返回 null 而不是发一个新 id ——
  写不进去的存储会在每次刷新时铸造一个新成员，把一个人的 issue 摊到一堆成员上。
- [x] `web-providers.tsx` 把 `deviceAuth` 透传给 `CoreProvider`。
- [x] `auth-initializer.tsx` 从 401 接管设备登录。这是 web 唯一能走到设备路径的方式：
  它的会话在读不到的 HttpOnly cookie 里，「已登出」不是它能先行判断的分支。
  `deviceAuthAttempted` 闩锁保证每次挂载只接管一次——否则设备会话自己被拒、或设备登录成功
  后 workspace 列表 401，都会回头再铸一个身份，每次多留一行 member。
  `resume()` 取代两处硬编码的重试目标，让退避与 `online` 事件重试的是真正挂起的那次登录。
- [x] 部署面默认值同步：`.env.example` 留空即默认开启并写明关闭方式；Helm `values.yaml`
  的 `deviceAuth.enabled` 从 `false` 改为 `""`（空值取服务端默认）。
- [x] 测试：`auth-initializer.test.tsx` 新增 cookie 模式 6 例（401 接管、无设备身份、
  部署不提供、活跃会话不打扰、workspace 401 不二次铸造、token 过期重铸）；
  `device-identity.test.ts` 10 例；`auth_device_test.go` 的 `TestDeviceAuthEnabledFromEnv` 11 例。

### 真实验证（2026-09-01，非人工推断）

一次性 PostgreSQL（`pgvector/pgvector:pg17`，独立端口，跑完删除）+ 真实后端进程：

| 项 | 结果 |
| --- | --- |
| 默认（未设变量）`/api/config` | `device_auth_available=true`，启动 WARN 带关闭方法 |
| `POST /auth/device` 首次 | token 320 字节、`onboarded_at` 非空、下发 `multica_auth`+`multica_csrf` HttpOnly cookie |
| 同 device_id 二次 | 同一 user id（幂等） |
| 第二个 device_id | 同一 `intranet` workspace，A=owner / B=member（该轮显式配置了 slug；2026-09-03 起缺省不再共享） |
| 库内真值 | 2 device user、2 member 行、7 个 issue status |
| `MULTICA_DEVICE_AUTH_ENABLED=false` | 无 WARN、config 不含该字段、端点 403、零写入 |
| `go test ./internal/handler/... ./internal/middleware/...` | ok 36.8s / 1.4s，exit 0 |
| `go build ./...` / `go vet` | 通过 |
| `pnpm typecheck` | 9/9 |
| `pnpm test` | 5/5 task，411 文件 4892 用例 |
| `scripts/dev-env.test.sh` / `scripts/selfhost-config.test.sh` | exit 0 |

仍未核验：桌面端与浏览器的真实 UI 首启（本轮只验证到 HTTP/DB 契约层），以及双设备在 UI 上
互相分配 / @提及 / inbox 的协作项。

### 已知取舍

- web 首次访问站点根路径 `/` 时，`multica_logged_in` cookie 尚不存在，proxy 放行到落地页，
  设备登录完成后由 `RedirectIfAuthenticated` 跳进 workspace——会闪一下落地页。这是既有行为
  （任何首次登录的用户都会经历），第二次访问起 proxy 用 cookie 在服务端直接重定向。
- `isOfficialCloudDeployment()` 精确匹配 `multica.ai`，不含 `www.multica.ai`。沿用既有 daemon
  配置的判定口径，未在本次改动中扩大。

## 提交划分

1. `feat(server): add device auth switch and capability declaration`（S1）
2. `feat(server): add POST /auth/device with device identity provisioning`（S2）
3. `feat(core): add device login contract and store method`（S3）
4. `feat(core): auto device login on tokenless boot`（S4）
5. `feat(desktop): persist device identity and wire device auth`（S5）
6. `docs: document intranet device auth for self-hosting`（S6）
7. `feat(web): sign in from a browser device identity`（S11 web 部分）
8. `feat(server): default device auth on for self-hosted deployments`（S11 默认值部分）

## 回滚点

- S1–S2 之后回滚：删除端点与字段即可，无数据残留（开关未开时不会有设备用户）。
- S3–S5 之后回滚：客户端 fail closed，回退后即回到登录页。
- 已产生设备用户后回滚：数据保留，按 `@device.multica.local` 后缀可批量清理。

## S8 离线部署交付物（追加）

`make offline-bundle` + `scripts/offline-bundle.sh`：从当前 checkout 构建 backend/web 镜像、
拉取 compose 里钉住的数据库镜像、`docker save | gzip` 成一个归档，并把 compose 文件、
`.env.example`、生成的 README（含本次 commit）与 MANIFEST（镜像清单 + 校验和）一起放进
`dist/offline/`。镜像名从 compose 文件与 build overlay 里读回来，不在脚本里重复写死。

`SELF_HOSTING.md` 新增「Air-gapped / Offline Deployment」一节：联网机器侧（导出 bundle、
`pnpm package` 打桌面包、带上 agent CLI）、气隙内侧（`docker load`、`.env` 的镜像名覆盖与
必填项、端口绑定陷阱、启动与验证、客户端 `desktop.json`），以及离线状态下必然失效的能力清单。

`scripts/selfhost-config.test.sh` 扩了两组断言：`MULTICA_DEVICE_AUTH_*` 只要写进
`.env.example` 就必须映射进 backend service（照抄既有 LLM 变量的漂移守卫），以及离线 bundle
的 `--dry-run` 必须解析出 compose 实际启动的三个镜像。`.github/workflows/ci.yml` 的路径过滤
加上了 `scripts/offline-bundle.sh`。

验证：`bash scripts/selfhost-config.test.sh` 通过（exit 0）；`bash scripts/offline-bundle.sh
--dry-run` 正确解析三个镜像名。真实的 `make offline-bundle` 未跑——本机没有 Docker 守护进程。

## S9 跨平台交付（追加）

`scripts/offline-bundle.sh` 增加 `--platform`（Makefile 透传 `PLATFORM=`），**默认 linux/amd64
而不是构建机架构**：arm Mac 上按主机默认产出的 arm64 镜像会在 x86 服务器上 `docker load` 成功、
容器全部 `exec format error`，而那时 bundle 已经被人扛进气隙了。平台同时传给
`docker compose build`（`DOCKER_DEFAULT_PLATFORM`）与 `docker pull --platform`（数据库镜像是
多架构的），写入 MANIFEST，并在生成的 README 里明确写出「服务器不是这个架构就别继续」。
目标平台与主机不一致时，计划输出会提前说明走模拟、会很慢。

`SELF_HOSTING.md` 补充：镜像默认架构与理由；桌面安装包按平台各自构建（对齐 CI 的
windows-latest / ubuntu-latest 做法），每台构建机需要 Node 22 + pnpm + Go 工具链，Windows 用
`CSC_IDENTITY_AUTO_DISCOVERY=false pnpm package -- --win --x64`；未签名 NSIS 的 SmartScreen 提示
与 macOS Gatekeeper 提示；Windows 的客户端配置路径是 `C:\Users\<you>\.multica\desktop.json`。

验证：`--dry-run` 在两种 `--platform` 下输出正确，模拟提示按主机架构（arm64）正确触发；
`bash scripts/selfhost-config.test.sh` 仍然通过（exit 0）。

## S10 实际产出与真实验证（追加）

本机装上 Go 1.27 与启动 Docker Desktop 后，早先两个「未执行」项都补齐了：

- `cd server && go build ./...` 通过，`go vet ./internal/handler/... ./internal/middleware/...` 通过。
- 起一次性 postgres（`pgvector/pgvector:pg17`，arm64，独立端口 55432，跑完删）跑完 944 个迁移后，
  10 个测试函数全部 PASS（含 16 个子用例）：开关关闭 403 且零写入、非法 device_id 400、
  开通含 7 个内置状态与 `onboarded_at`、三次登录幂等（含大写 id）、双设备 owner/admin、
  复用既有 workspace、`ALLOW_SIGNUP=false` 仍可开通、名称回退与清洗、config 声明开关。

产出物：

- `dist/release/linux-amd64/`：`multica-images.tar.gz`（283MB，**仅 amd64**）+ compose + `.env.example`
  + MANIFEST（含 sha256）+ README。三个镜像的架构逐一从归档里核验过。
- `dist/release/windows-x64/`：`multica-desktop-0.1.0-windows-x64.exe`（169MB，NSIS one-click，
  per-user）+ blockmap + README（含 sha256 与 `desktop.json` 指引）。内置
  `multica.exe` 确认是 PE32+ x86-64。

期间修掉的两个真 bug：

1. `docker compose build` 会先插值整个 compose 文件，backend 的 `JWT_SECRET` 是 `${JWT_SECRET:?}`，
   构建机上没有 `.env` 时直接中止。改为构建期喂占位值（JWT_SECRET 是运行时环境变量而非 build arg，
   不进镜像）。
2. `docker save` 按 tag 保存会把本地存在的**所有**架构一起写进归档。本机为了跑测试拉过 arm64 的
   数据库镜像，于是第一版归档同时含 amd64 和 arm64，白多 153MB。改为 `docker save --platform`。
   归档在 x86 服务器上仍然可用（load 后按平台选择），但体积没必要。

另外 `VERSION`/`COMMIT` 现在支持环境变量覆盖：这个工作目录不是 git 仓库，所以本次产出的
MANIFEST 与安装包版本是 `dev` / `0.1.0` / `commit unknown`，无法回答「服务器上是哪个 build」。
正式分发建议在 git checkout 里构建，或 `VERSION=v0.4.37 COMMIT=abc1234 make offline-bundle`。

## 执行状态（收尾记录）

已完成 S1–S6，并额外补了部署接线（否则开关在任何已文档化的自托管路径里都打不开）：
`docker-compose.selfhost.yml` 的 backend 环境变量、Helm `values.yaml` + `configmap.yaml`
的 `backend.config.deviceAuth.*`、以及 `.env.example`。

本机可跑的检查全部通过：`pnpm typecheck`、`pnpm test`、`pnpm lint`。

两项未执行，都是环境缺失而非跳过：

1. Go 测试与编译 —— 本机没有 Go 工具链，服务端代码未经编译器验证，只做了逐行人工复核
   （符号存在性、导入使用、`:=` 重声明、事务与 switch 分支）。
2. 双设备手工验收 —— 需要一个运行中的后端（本机无 Docker/PostgreSQL）。

另：工作目录不是 git 仓库，Phase 3.4 的提交无法执行。
