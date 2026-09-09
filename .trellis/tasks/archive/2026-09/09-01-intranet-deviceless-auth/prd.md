# 内网免登录（设备匿名身份）

## Goal

在内网自托管部署下，桌面端首次启动即自动获得一个「每台设备一个」的独立身份并直接进入应用：不出现登录页、不需要邮箱验证码。协作语义必须继续成立——分配、@提及、评论作者、inbox、活动记录都指向可区分的真实身份。

## 已确认决策

1. 身份模式：每设备匿名身份（不是单一共享账号，也不走反代注入 SSO）。
2. 生效范围：不做来源 IP/CIDR 限制，只由服务端单一开关控制。
   风险已明确告知并由用户确认：开关打开后，任何能访问后端端口的人都能自助获得身份并读写该 workspace 全量数据。
3. 推进方式：先规划（本文 + design.md + implement.md），评审通过后再实现。
4. **开关默认值（2026-09-01 用户变更）**：自托管默认**开启**，仅当本服务在为 multica.ai 提供服务时默认关闭。
   理由：本特性存在的意义就是那些根本完不成任何登录的部署（无邮件中继、无可达 OAuth），
   要求运维先发现一个环境变量名，等于给了他一个永远打不开的应用。官方云是例外——那里的
   「免登录」不是「办公室」而是「整个互联网」。`MULTICA_DEVICE_AUTH_ENABLED` 双向覆盖默认值。
5. **覆盖端（2026-09-01 用户变更）**：web 与桌面端都要免登录，不再只做桌面端。
6. **共享 workspace 默认值（2026-09-03 用户变更，ART-7）**：默认**不**再自动加入任何共享 workspace。
   设备登录只建立身份，首启照常进入 onboarding，由成员自己命名工作区；只有运维显式设置
   `MULTICA_DEVICE_AUTH_WORKSPACE=<slug>` 时，所有设备才加入同一个空间并跳过 onboarding。
   理由：session 不等于 workspace，把人直接放进一个谁都没选过的默认工作区，比多走一步命名更糟。

## Requirements

### R1 服务端设备登录端点

- 新增公开端点 `POST /auth/device`，入参 `device_id`（必填）、`device_name`（可选）；返回与 `/auth/verify-code` 相同的 `LoginResponse`（`token` + `user`）。
- 仅当开关打开时可用；关闭时返回 403 且不写入任何数据。
- 同一 `device_id` 重复调用必须解析到同一个用户（幂等）。
- `device_id` 需校验格式（32–64 位十六进制），非法返回 400。
- 复用 `/auth/*` 现有限流中间件。

### R2 自动开通

- 首次为某 `device_id` 建立 `user` 行。**默认到此为止**（2026-09-03 变更）：不建 `member`、不建 workspace、不标记 onboarding，客户端因此走 onboarding 引导成员自己创建工作区。
- 仅当配置了共享 workspace slug 时，同一次调用继续建立该 workspace（不存在时创建，含内置 issue 状态）与 `member` 行。
- 上述写入在同一个应用层事务内完成，任一失败不留半成品。
- 共享 workspace slug 由 `MULTICA_DEVICE_AUTH_WORKSPACE` 指定，**缺省为空 = 无共享 workspace**；保留或非法 slug 一律读作空（而不是回落到某个别的 workspace）；已存在则直接复用，不修改其任何属性。缺省显示名由 slug 反推（`acme-intranet` → `Acme Intranet`）。
- 首个开通设备为 `owner`，后续设备角色可配置，缺省 `member`；角色只在配置了共享 workspace 时才有意义。
- 只有加入共享 workspace 的设备用户才被标记为已完成 onboarding。桌面端的硬不变量是 `onboarded_at != null` 才能进入 dashboard——有共享 workspace 时这让"打开即用"成立；没有时被 onboarding overlay 拦住正是想要的状态，因为没有任何工作区可打开。
- `ALLOW_SIGNUP` 不影响本路径：它治理的是人工注册。`DISABLE_WORKSPACE_CREATION` 在 2026-09-03 变更后不再无关——缺省无共享 workspace 时，设备会被 onboarding 要求创建一个该开关禁止创建的工作区，只剩登出一条路。服务端启动时对这个组合发出告警，文档要求这类部署显式配置 `MULTICA_DEVICE_AUTH_WORKSPACE`。

### R3 服务端能力声明

- `/api/config` 增加 `device_auth_available` 布尔字段，仅在开关打开时为 true。
- 客户端 schema 必须 fail closed：字段缺失、类型错误或整个响应不可解析时一律视为 false，回落到登录页。

### R4 桌面端设备身份

- 主进程在 userData 下生成并持久化设备标识（随机 32 位十六进制）以及默认显示名（`<os user>@<hostname>`），通过 preload 暴露给渲染进程。
- 标识跨重启稳定。文件缺失或损坏时重新生成，语义上等价于一台新设备。

### R5 桌面端启动流程

- 无本地 token 且服务端声明 `device_auth_available` 时，自动完成设备登录并进入应用，不显示登录页。
- 服务端未开启该能力时，回落到现有登录页；现有邮箱验证码与 Google 登录路径行为不变。
- 服务端暂时不可达时沿用现有 recovering/重试机制，不得直接沉到登录页后卡死。
- 登录成功后的 daemon token 同步、workspace 列表预热、实时连接与既有登录路径完全一致。

### R6 文档

- `SELF_HOSTING.md`（必要时含 `SELF_HOSTING_ADVANCED.md`）补充开关、环境变量清单，以及"开关打开等于该 workspace 无鉴权"的显式风险说明。

### R7 web 端设备身份与启动流程（2026-09-01 追加）

- 浏览器侧生成并持久化设备标识（localStorage，随机 32 位十六进制），显示名 `web-<os>-<id 前 8 位>`。
  作用域是「这台机器上的这个浏览器 profile」，不是「这台机器」——清站点数据等价于换新设备。
- web 的会话在 HttpOnly cookie 里，客户端读不到，所以它没有「已登出」这个可先行判断的分支：
  唯一信号是 `getMe` 返回 401。设备登录必须从这个 401 接管，且每次挂载最多接管一次。
- `POST /auth/device` 必须同时下发与 `/auth/verify-code` 一致的会话 cookie，否则浏览器侧
  只拿到一个用不上的 bearer token，CDN 附件全部 403。
- localStorage 不可用时（隐私模式配额、被禁 cookie）不提供设备身份，回落到登录页——
  一个写不进去的存储会在每次刷新时铸造一个新成员。

## 明确不做

- 不做来源 IP/CIDR 限制（用户已确认）。
- 不做匿名身份升级绑定邮箱或身份合并。
- 不解决 agent 需要模型出口的问题。内网无出网时 agent 依然跑不了，与本任务无关，需要单独的内网 LLM 网关方案。
- 不改桌面端指向内网后端的配置方式，仍是 `~/.multica/desktop.json` 的 `apiUrl` / `wsUrl`。
- 不新增数据库迁移。

## Acceptance Criteria

已核验（2026-09-01，真实后端 + 一次性 PostgreSQL，非人工推断）：

- [x] 开关关闭时：`POST /auth/device` 返回 403；`/api/config` 不声明该字段；无任何写入（设备用户数不变）。
- [x] 开关**默认**打开：未设置 `MULTICA_DEVICE_AUTH_ENABLED` 时 `/api/config` 返回 `device_auth_available=true`，启动日志有 WARN 并带上关闭方法。
- [x] 重复调用 `POST /auth/device` 不产生重复 `user` / `member` 行（同一 device_id 两次调用返回同一 user id；库里 2 设备 = 2 user + 2 member）。
- [x] 第二台设备首启后成为同一 workspace 的第二个成员（A=owner，B=member，同一 `intranet` workspace）。
      ※ 2026-09-03 起这条只在显式配置 `MULTICA_DEVICE_AUTH_WORKSPACE` 时成立；缺省下两台设备各自命名自己的工作区。
- [x] 自动创建的 workspace issue 状态齐全（7 个内置状态）。
- [x] 设备用户 `onboarded_at` 非空——否则会被 onboarding 拦在 dashboard 外，"打开即用"不成立。
- [x] `POST /auth/device` 下发 HttpOnly 会话 cookie（`multica_auth` + `multica_csrf`），这是 web 端能用的前提。
- [x] 服务端返回畸形 `/api/config` 或 `/auth/device` 响应时，客户端 fail closed 落到登录页（malformed-response 测试覆盖）。
- [x] `cd server && go test ./internal/handler/... ./internal/middleware/...` 通过。
- [x] `pnpm typecheck`（9/9）与 `pnpm test`（5/5 task，411 文件 4892 用例）通过。
- [x] SELF_HOSTING 文档写明开关、变量与风险。

已核验（2026-09-03，ART-7 默认值变更；worktree 独立数据库 `multica_worktree_297`）：

- [x] 未配置 `MULTICA_DEVICE_AUTH_WORKSPACE` 时首启只建身份：无 `member` 行、不创建 `intranet` workspace、`onboarded_at` 为空（客户端因此进入 onboarding）。
- [x] 已完成 onboarding 的设备重复登录不会被打回 onboarding（`onboarded_at` 保留）。
- [x] 保留 / 非法 slug 读作空；配置了 slug 时显示名由 slug 反推。
- [x] `go build ./...`、`go vet ./internal/handler`、`gofmt -l` 干净。
- [x] `go test ./internal/handler/ ./internal/middleware/...`（ok 41.6s / 1.1s）与 `-run TestDevice` 全绿。
- [x] `scripts/selfhost-config.test.sh` exit 0。
- [x] SELF_HOSTING / SELF_HOSTING_ADVANCED / `.env.example` / helm values / compose / offline-bundle 全部改写为"缺省无共享 workspace"。

用户验收（2026-09-09；由用户实际测试确认，本归档会话未复跑）：

- [x] `scripts/helm-config.test.sh` 通过。
- [x] 干净环境首启桌面端，可走完 onboarding 并命名自己的工作区。
- [x] 干净浏览器首访 web，可零人工输入获得身份并进入 onboarding；配置共享 workspace 时可直接进入 issues 页面。
- [x] 同一台机器重启后仍保持同一身份，此前创建的 issue 作者与被分配人不变。
- [x] 双方可互相分配 issue、@提及并收到 inbox 通知。
- [x] 可正常创建 issue 并在看板拖动。
