# 资源授权与参与人表单

本轮补齐已有协作单的目标登记、逐资源同意/撤回、Switch 提案/同意/撤回/提交和参与人移除。沿用已批准任务，不启用公共执行能力。

## 实施边界

当前写入原语已经存在，缺少供表单使用的受控名册、资源候选和本人绑定读取。本轮新增 service/handler/query 读取与 core 严格 schema、原键恢复，再在 shared views 接线。参与人管理使用独立读取保留 disabled 账号，移除仍复用原 remove 操作；mention-targets 继续只列有效提及者。

- `GET /api/collaboration/issues/{id}/resources`：当前本单参与者可见的目标名册与授权摘要；grant ID/epoch 仅真实 owner 可见。
- `GET .../resources/options?execution_workspace_id=UUID`：仅发起人/处理人，明确资源空间必须已有成员权限；候选复用既有智能体与运行时可见规则，不跨空间发现，不自动邀请 owner。
- `GET .../resources/identity?target_id=UUID&agent_id=UUID[&proposal_id=UUID]`：仅实际本人，资源必须属于同单目标或提案；同一快照返回当前 managed/enrolled 身份，无合法注册时返回不可授权原因。
- `GET .../resources/operations/{requestId}`：仅本人原登记/授权/撤回收据；失权后只给七字段无内容收据。
- `GET .../participants`：本单当前未撤销参与关系的最小账号摘要与 `account_active`，包含 disabled 账号；返回同快照 `collaboration_revision`。账号不可用不会取得提及、流转或控制权，但当前发起人仍可移除。

所有写入继续在服务端重验，配置和授权变更不恢复执行。Switch 保留 typed target，选择当前实际配置形成不可变提案；原始确认捕获版本，未知响应只恢复原 key。候选读取失败只阻断资源选择。新增 SQL 由主会话串行生成；不新增迁移、依赖或普通资源目录权限。

## HTTP 与客户端契约

- Resources：`protocol_version:1`、`issue_id`、无损 `lifecycle_epoch/target_set_version`、`can_organize` 与 `targets`。每个目标固定 typed identity、target/config 版本、完整资源列表、授权状态及原因；资源附最小 agent/runtime/owner 显示摘要。`grant_id/grant_epoch/can_revoke` 仅实际本人得到，配置漂移时保留本人撤回入口。
- Options：`protocol_version:1`、`issue_id/execution_workspace_id` 和完整可见候选；复用现有 agent inventory predicate 及 runtime visibility。所有小队实际智能体资源均可见才列候选，既有未参加 owner 与 disabled owner 分别返回 `resource_owner_not_participant/resource_owner_unavailable`。选择不邀请、不授权、不运行。
- Identity：固定 `issue_id/target_id/agent_id/proposal_id`；`identity` 为当前 `managed|enrolled` 的 installation/binding/无损 epoch，或 null + 原因。仅目标/提案内本人资源可取；managed namespace 有历史时不退回 legacy enrollment，模糊的多 enrollment 不任选。
- Resource writes：继续使用既有 POST，分别为 `register_target/consent_resource/revoke_grant`。回执补齐 `request_id/action`，GET 恢复按原 actor/issue/key 查询。成功及持久拒绝均需严格匹配，未知或坏响应保留原 key，不自动生成新意图；已失权只接受原七字段 redacted 收据。
- Switch：沿用四种既有写入和原 GET 恢复。表单保留 typed target；候选来自明确资源空间的该同一目标。客户端核对当前组织者身份、提案归属和实际 owner，不能代签。显示摘要新增可选 `agent_name/runtime/owner`，不带配置正文、fingerprint 或安装身份；旧响应缺显示摘要仍可读。提交保留原确认版本，成功仍需 Resume。
- Switch 提案反映资源已有的当前绑定：在原智能体/小队管理中修改机器或成员后，回到本单选择同一目标形成提案；既有资源配置失效机制照常停止旧依赖组。表单不把输入目标改成另一个智能体，也不新增资源重绑协议。
- Participant management：正文读取权允许查看最小名单；移除须当前 creator + 显式 `remove_participants` + 原写入上下文和同快照名单版本，不能移除自己。移除当前 handler 的确认说明责任原子回交 creator。新增 `account_active:false` 只用于表示，不用于赋予权限。
- 资源 workspace 查询失败只影响该选择器；实际本单拒绝才隔离 Query、推进权限代次并清保护内容。管理区复用原执行 mutation、pending storage、原键恢复与共享 UI，没有另一套执行/窗口机制。
- 受主会话委托，详情 schema 已补可选严格 `realtime:{event_seq,authority_epoch}`（无损非负/正版本字符串）；缺省兼容，存在却畸形则不可用。

## 实际修改范围

- Server：新增 `collaboration_resources_view.go`、`collaboration_resource_options.go`、`collaboration_participants_view.go` 及资源 handler/回归；`collaboration_queue.go` 补原资源操作回执身份；`collaboration_switch_view.go` 共享有目的限制的原回执读取及显示摘要。Router 仅新增五个 GET 小块。
- SQL：`collaboration_resources.sql`、`collaboration_participants.sql`，主会话 sqlc pass1/pass5 生成对应文件。无迁移。
- Core：`resource*.ts`、`participant*.ts` 新 schema/请求/回归；Switch authority/显示解析与其回归；pending/index/API 的对应小块；realtime 详情 schema 与边界测试。API 不导入带请求副作用的 operation 模块。
- Views：新增 `resource-form.tsx`、`resource-panel.tsx`、`switch-panel.tsx`、`participants.tsx`；接入 execution-panel/workflow 与两份 locale 的 resources/participants，更新交互回归。不修改 creation、attachment、realtime 业务文件。
- E2E：`e2e/collaboration-resources-web.spec.ts` 与可复用 `e2e/fixtures/collaboration-resources.ts`。

## 本 lane 验证结果

- Core 定向 13 文件 / 123 项通过：resource、participant、Switch、pending、schema、operation、access、FIFO/Switch；最后补充的本单权威拒绝清缓存断言再跑 4 项通过。
- Views collaboration + locale parity 共 6 文件 / 81 项通过；最后对四个相关文件再次运行 75 项通过。新增共享交互覆盖登记、拥有者确认、丢响应原 GET 恢复、Switch 提交、disabled handler 移除与原版本确认。
- Core/Views `typecheck`、所有本 lane 修改的 TS/TSX ESLint、`git diff --check` 通过。Impeccable 静态检测四个新表单文件返回 `[]`；这不替代浏览器视觉验收。
- 真实 resources 专用 PostgreSQL 上，service 资源/Participants/授权/Switch 相关 `go test -race` 通过（17.395s）；handler 资源/Switch `-race` 通过（最终 2.824s）；`go vet ./internal/service ./internal/handler ./cmd/server` 通过。
- Go 日志在 `/var/folders/3n/gbt3p39s5pdc4l55js62gxnm0000gn/T/multica-cwc-product-20261018-nebz79rl/resources-{service-race,handler-race,go-vet}.log`。
- 一次全 core collaboration 运行有 572 项通过、delivery lane 当时尚未实现 API 方法的 4 项预期红灯；未改该 lane 文件。最终跨 lane 全套检查仍由主会话统一跑，不把这次全模块运行写成通过。

## Web / Desktop 验收交接

`pnpm exec playwright test e2e/collaboration-resources-web.spec.ts --list` 成功识别 1 条真实旅程；本 lane 未运行生产浏览器或原生 Electron。

主会话启动匹配生产 Web 后，在该环境设置 `COLLABORATION_E2E_API_URL`、`NEXT_PUBLIC_API_URL`、`DATABASE_URL`、`PLAYWRIGHT_BASE_URL` 和与 API 相同的 `MULTICA_DEPLOYMENT_ID`。API/数据库须 loopback 专用测试环境，数据库创建 gate 已显式开启；测试本身不改部署 gate。执行：

```bash
pnpm exec playwright test e2e/collaboration-resources-web.spec.ts --workers=1 --retries=0
```

旅程使用 TestApiClient 登录三个合成账号，公开 POST 创建正式协作单、邀请普通参与者；运行时和 enrollment 仅为本测试创建的无进程记录（32 字节测试 public key，不声称验证真实 enrollment challenge 或执行能力），智能体通过原 API 创建。不会改普通任务的协作标记，不安装或调用 CLI、AI 供应商。

实际 UI 顺序：登记目标 → 本人授权（服务器提交后故意丢响应）→ 重载、原 GET 恢复且只一次 POST → 撤回 → Switch 提案/本人同意/提交 → 保持 switch_hold → 普通参与者无组织/代签入口 → 展示并移除 disabled 当前 handler、责任回交 creator。末尾断言没有 run、无 ambient workspace 请求头、无 pageerror，并保存宽窄屏截图与非秘密身份附件。

`seedCollaborationResourceJourney` 和 `authenticateCollaborationResourcesPage` 可用于隔离 Desktop 旅程。测试数据保留在专有数据库供主会话审阅，后续由任务环境生命周期清理；不能直接删除含协作记录的工作空间。生产 Web/原生 Desktop 实际通过后再更新最终 AC 证据，父任务保持 `in_progress`。
