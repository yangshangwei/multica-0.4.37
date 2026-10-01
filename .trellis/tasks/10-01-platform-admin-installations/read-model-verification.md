# S02 只读台账切片验证

2026-10-02。工作区 `/Volumes/artisan/code/2026/multica-platform-admin`；当前提交基线 `fa28e28a267bc8c7d76d9b8c55ef2d08e000514e`，父任务正在同工作树集成 S02/S03/S05。此记录只覆盖台账读取与页面，不替代接入协议、桌面/daemon 或整个 S02 验收。

## 实现

- 新增 `GET /api/admin/installations`、`/installations/{id}`、`/installations/unassociated`，复用 S01 每请求平台权限检查。仅查询当前内部组织和配置中的部署 UUID；游标也绑定部署、actor、组织、资源与筛选。
- 管理安装列表按 `(created_at DESC,id DESC)` 分页，默认50、最大100。按名称/ID、生命周期、客户端活跃、版本、OS、分组和创建时间筛选。缺少部署身份返回503，旧部署的游标不能在新部署使用。
- 每个安装只出现一次；runtime 仅由 active binding 的 workspace + daemon + owner 精确关联。运行时数独立 SQL 计数，未关联旧端单独分页，绝不根据名称/IP/device_info/日活标识猜测归属。
- 客户端轴只用签名上报接收时间，180秒内活跃；无上报保持 unknown。daemon 轴复用 LivenessStore 批量查询及绑定/runtime心跳证据。配置中的 liveness 读失败为 unavailable，不批量标 offline。
- 执行准备度核查安装生命周期、admission、绑定能力版本、账号当前版本/禁用/成员关系、未过期绑定 daemon 凭据，以及 runtime.status 和现有 `RuntimeClaimFreshnessSeconds=150` 的 DB 心跳条件。Redis alive 不会绕过实际领取使用的 DB freshness gate；不承诺具体任务或并发空位。
- 每条轴返回 state、observed_at、source、freshness、reason_code；客户端关闭与 daemon 可达、客户端活跃与引擎损坏分别可呈现。旧观察不标 fresh，未知/源故障不映射成健康或离线。
- 仅读 metadata 白名单；不返回 public_key、证明/种子/token、runtime名称、device_info、完整metadata、日志或本地路径。离线原因只提取受控 `not_executable` 码。运行时关联、绑定历史、验证用户记录均有边界，截断时明确标注。
- 三个 Web 页面：台账列表、安装详情、未关联运行时。分开的轴和新鲜度、URL筛选、分页、状态缺失、故障重试与最近观察时间均已实现；没有远程shell、升级或接单策略写按钮。原生桌面验证由后续集成负责。

## 已执行检查

- task-owned PostgreSQL `multica_platform_admin_s01_test`；使用独立fixture workspace/安装/绑定/runtime，生命周期清理仅针对这些fixture。没有启动真实daemon或agent、没有模型请求。
- 首先观察新方法缺失、旧状态观察被错误标fresh、轴值混用等失败，再实现/修正。
- `go test ./internal/handler -run 'TestAdminInstallation' -count=1` 通过。3个顶层测试覆盖独立状态轴、缓存/DB心跳区别、liveness故障、无凭据、未知证据、同安装双provider去重、owner错配转未关联、详情投影、凭据撤销、跨部署游标隔离与缺部署503。
- 最终 `go test -race ./internal/handler -run 'TestAdminInstallation' -count=1` 通过，4.026s；紧接着 `go vet ./internal/handler` 通过。曾被父任务正在新增而尚未sqlc生成的 SubmittedInstallationID 字段短暂阻断，生成后已成功重跑。
- core：`vitest run admin/installation-schemas.test.ts admin/installation-queries.test.ts` 2文件、8测试通过；包括未知枚举不伪造退休/停止策略、不同轴值归为unknown、秘密字段剥离、旧会话响应与跨组织响应拒绝。
- views：`vitest run admin/installations` 3文件、4测试通过；覆盖三轴独立、无控制按钮、数据库故障保留最近观察且不展示旧健康、未知明细与下钻链接、未关联列表明确“最近上报状态”。
- core/views `tsc --noEmit --pretty false` 通过；相关文件 eslint 通过；`git diff --check` 与 Trellis context validate 通过。
- impeccable 机械检测 `detect.mjs --json packages/views/admin/installations` 返回 `[]`，不代表已完成真实浏览器截图/visual-verdict。

## 集成与未测

父任务已集成三个路由、ApiClient方法、共享导出、双语 installations namespace和sqlc。页签/后台导航及 production build/start 由父任务统一处理。

本只读切片已完成指定生产快照的浏览器检查和截图/visual-verdict；整个S02的原生/安装态验收仍由父任务和S07汇总。该记录没有把服务协议基本测试或此台账夹具当作原生 Windows/macOS、真实机器持钥、1000终端容量或百万任务压力验收。S04控制、S06告警、S07全量/容量/原生验证仍由相应任务负责。

变更范围：`server/internal/handler/admin_installation.go`及测试、`server/pkg/db/queries/admin_installation.sql`、`packages/core/admin/installation-*`、`packages/views/admin/installations/*`、`apps/web/app/admin/installations/*`、task内两个locale-read片段。本切片没有编辑根协议/密码/密钥服务，也没有提交。

## 在途计数补充（2026-10-02）

按仓库 `CountRunningTasks` 的明确语义，`waiting_local_directory` 仍占并发容量，须计入在途执行。已在管理查询的既有 preparing/dispatched/running 集合中补入该状态，没有减少已有计数范围。真实fixture先观察计数0的失败；父任务sqlc合并后，连同执行筛选回归重跑通过（1.345s）。准备阶段当前持久化为dispatched；此修复不改变任务调度/领取语义。


## 生产只读/真实协议 smoke（2026-10-02）

`e2e/platform-admin-installations-read.spec.ts` 已在与S03相同的已证明生产快照 source `2d7a14b1612dab58d77a70352a12d9f8ce7270a2a3d9af56ea47e9c02fb87905` / Web `4HqC2LA83Zf-dnxnfTGP6` 运行通过：1场景，0失败，2.7s。

使用新建普通password账号，真实Ed25519挑战/接入/绑定、mdt注册两个假provider、签名桌面心跳和daemon heartbeat/deregister请求；证明metadata proof不能当管理员Bearer、两个runtime只计一个安装、窗口不活跃但daemon可达、客户端活跃而环境不可用、legacy runtime不猜测合并、私有name/path/key/token不出管理DTO。没有运行实际agentCLI/模型，专属fixture已清理，没有平台角色变更。

桌面/详情/未关联页面截图符合现有后台样式；移动端表格发现安装UUID溢出与状态标签重叠。已先保存82/revise verdict，再将本切片名称单元格改为稳定宽度/可换行，链接block+break-all；已加入浏览器几何断言并通过现有4项组件测试与定向lint。新生产构建复跑1/1通过（2.7s），几何断言及截图确认UUID不再覆盖状态标签；最终visual-verdict为93/pass，无待修视觉项。API PID1467/Web PID2095，详细启动来源见 `.omx/reports/platform-admin/final-s02-s03-s05/`。

证据：`.omx/reports/platform-admin-installations-read/{browser-verification,visual-verdict}.json` 与5张PNG；迭代状态 `.omx/state/platform-admin-installations/ralph-progress.json`。原生main/daemon IPC、profile drain、Windows/macOS安装态、混合Node+Go并发锁和S07容量验收仍不在此结果中。


补充：父任务已运行macOS真实4个Node+4个Go混合锁竞争测试，8次临界区进入无重叠、残留ticket为0（1.175s），证据 `.omx/reports/platform-admin/mixed-lock-interop.json`。这关闭混合holder覆盖缺口，但不替代Windows ACL/进程或安装态Electron验收。
