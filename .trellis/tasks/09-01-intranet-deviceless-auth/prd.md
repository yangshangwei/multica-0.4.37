# 内网免登录（设备匿名身份）

## Goal

在内网自托管部署下，桌面端首次启动即自动获得一个「每台设备一个」的独立身份并直接进入应用：不出现登录页、不需要邮箱验证码。协作语义必须继续成立——分配、@提及、评论作者、inbox、活动记录都指向可区分的真实身份。

## 已确认决策

1. 身份模式：每设备匿名身份（不是单一共享账号，也不走反代注入 SSO）。
2. 生效范围：不做来源 IP/CIDR 限制，只由服务端单一开关控制，默认关闭。
   风险已明确告知并由用户确认：开关打开后，任何能访问后端端口的人都能自助获得身份并读写该 workspace 全量数据。
3. 推进方式：先规划（本文 + design.md + implement.md），评审通过后再实现。

## Requirements

### R1 服务端设备登录端点

- 新增公开端点 `POST /auth/device`，入参 `device_id`（必填）、`device_name`（可选）；返回与 `/auth/verify-code` 相同的 `LoginResponse`（`token` + `user`）。
- 仅当开关打开时可用；关闭时返回 403 且不写入任何数据。
- 同一 `device_id` 重复调用必须解析到同一个用户（幂等）。
- `device_id` 需校验格式（32–64 位十六进制），非法返回 400。
- 复用 `/auth/*` 现有限流中间件。

### R2 自动开通

- 首次为某 `device_id` 建立：`user` 行、共享默认 workspace（不存在时创建，含内置 issue 状态）、`member` 行。
- 三者在同一个应用层事务内完成，任一失败不留半成品。
- 默认 workspace slug 可配置，缺省 `intranet`（已确认不在保留 slug 列表内）；已存在则直接复用，不修改其任何属性。
- 首个开通设备为 `owner`，后续设备角色可配置，缺省 `member`。
- 设备用户必须同时被标记为已完成 onboarding。桌面端的硬不变量是 `onboarded_at != null` 才能进入 dashboard，否则会被 onboarding overlay 拦在应用外，"打开即用"不成立。
- `ALLOW_SIGNUP` / `DISABLE_WORKSPACE_CREATION` 不影响本路径：它们治理的是人工注册和用户自建 workspace。

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

## 明确不做

- 不改 web 端登录门。服务端开关与 config 字段是共享的，web 可作为后续增量任务。
- 不做来源 IP/CIDR 限制（用户已确认）。
- 不做匿名身份升级绑定邮箱或身份合并。
- 不解决 agent 需要模型出口的问题。内网无出网时 agent 依然跑不了，与本任务无关，需要单独的内网 LLM 网关方案。
- 不改桌面端指向内网后端的配置方式，仍是 `~/.multica/desktop.json` 的 `apiUrl` / `wsUrl`。
- 不新增数据库迁移。

## Acceptance Criteria

- [ ] 开关关闭时：`POST /auth/device` 返回 403；`/api/config` 不声明该能力；桌面端行为与当前完全一致（显示登录页）。
- [ ] 开关打开时：干净环境首启桌面端，零人工输入直接进入 issues 页面。
- [ ] 同一台机器重启后仍是同一身份：此前创建的 issue 作者与被分配人不变。
- [ ] 第二台机器首启后成为同一 workspace 的第二个成员，双方可互相分配 issue、@提及、收到 inbox 通知。
- [ ] 重复调用 `POST /auth/device` 不产生重复 `user` / `member` 行。
- [ ] 自动创建的 workspace issue 状态齐全，可正常建 issue 并在看板拖动。
- [ ] 服务端返回畸形 `/api/config` 或 `/auth/device` 响应时，客户端不崩溃并落到登录页（malformed-response 测试覆盖）。
- [ ] `cd server && go test ./internal/handler/... ./internal/middleware/...` 通过。
- [ ] `pnpm typecheck` 与 `pnpm test` 通过。
- [ ] SELF_HOSTING 文档写明开关、变量与风险。
