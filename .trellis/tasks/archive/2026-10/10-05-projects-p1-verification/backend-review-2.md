# P1 后端复审（第二轮）

- 日期：2026-10-05。
- 工作树：`/Volumes/artisan/code/2026/multica-projects-p1`。
- 审查对象：HEAD `8fcea88de57cbebd89a8b2f65d98ec7230ab2b84` **加当前未提交的后端 SQL/handler/router 修改**；不是只审 HEAD。
- 对照：上一轮 backend-review.md、foundation/association-verification.md、当前 design/api-contract/test-spec 与批准的 ADR-05 C。
- 最新结论：**APPROVE**。本轮后端审查的 BR-01、BR-02、BR-03 均已关闭，BR-03 的 Web/Desktop core 消费依赖也已独立验证。此结论不替代 UI 剩余 RR-02/RR-03、全仓验证或最终性能报告。
- 本 lane 仅写本报告，未改产品源码或测试、未提交。

## BR-01 / BR-02 复核

**BR-01 已修复。** `service/autopilot.go:982` 将整个 run_only attempt 置于有界重试；最终事务保留项目/自动化 fence，`:1058` 起的 runtime/agent SHARE NOWAIT 及 squad/成员/授权目标 NOWAIT 不再等待与 teardown 相反的锁。任务 INSERT 所需的 owner KEY SHARE 已被当前事务持有的更强锁覆盖。任何 NOWAIT 失败退出 `dispatchRunOnlyOnce`、rollback 后才重新准备；耗尽返回 skipped。锁内重查 runtime 绑定、assignee、squad leader、readiness、当前 principal membership 和调用授权，既没有放松 project_deleted，也保留普通暂停的手动 run。

已读取真实 40P01 RED 说明与测试实现；本 lane 独立重跑真实 `TeardownRuntime` 有项目/无项目锁环、先提交双序、owner 变化测试，全部 PASS、无 race。未用假 teardown 代替生产路径。

**BR-02 已修复。** `handler/issue_view.go:127` 的 `writeIssueViewScope` 为 project scope 最终 INSERT/UPSERT 持 workspace→project fence；view/preferences 都复用它。`handler/project.go:914` 与 `queries/project.sql:132` 在同一删除事务清项目 preferences。现有双序矩阵已纳入 17 条 writer，并有普通删除 prefs/pins 与第 10 个故障回滚阶段。独立 race 回归 PASS。

## BR-03 原发现及整改追踪

以下为整改前证据；最新关闭范围见本节末尾追加记录。

### BR-03 — HIGH：真实路由中间件遮蔽 P1 权限错误分类，撤权后的 HTTP 失败不能清除保护内容

主定位：`server/cmd/server/router.go:2003`（所有 `/api/projects` 的外层 `RequireWorkspaceMember`）及 `server/internal/middleware/workspace.go:242`—`:244`（membership 缺失先返回无 code 的 404）。相关客户端：`packages/core/projects/access.ts:31`、`:35`、`:79`，只把 401/403 作为工作空间失权，把 `404 + project_not_found` 作为项目删除。

**触发 A（撤权）：** 成员已打开项目概览/进展/执行证据并有 Query 缓存；WebSocket 断连或撤权事件尚未到达，管理员将其移出工作空间；随后浏览器的 P1 refetch/发布请求经过真实 router。外层 membership 中间件直接返回 `404 {"error":"workspace not found"}`，P1 `runProjectTransaction` 内正确的 `403 forbidden` 根本不执行。核心 `protectProjectRequest` 不把此响应识别成访问丢失，也不清查询、证据候选或草稿；`project-overview.tsx:53` / `:70` 在仍有 data 时继续呈现旧内容，只提示刷新失败。这违反 PRD§16/P1-FR-08、FR-16 的失权后清除保护内容要求。服务端没有再次泄漏新正文，但客户端仍保留旧的受保护内容，不能用“handler 已返回403”证明真实路由已闭合。

**触发 B（操作权限）：** 管理员打开时区编辑后被降为普通 member，提交 PUT `/api/workspaces/{id}/planning-timezone`。该路由仍位于 `router.go:1741`—`:1743` 的 owner/admin 中间件组；`middleware/workspace.go:255`—`:257` 先返回无 code 的通用403。handler 新增 `project_permission_denied` 不会执行。`core/projects/p1-mutations.ts:23` 使用保护包装，因此将其误判为整个工作空间撤权，清掉本来仍可读的项目状态和本人草稿。这个分支会误删除可恢复输入。

**现有测试为何未发现：** 新 `project_permission_error_test.go` 直调 `UpdateProjectPlanningTimezone`；`TestProjectUpdateRealLeaveWorkspaceRevokesAccessAndStopsDelivery` 对 P1 列表/重放也直调 handler，仅 inbox 使用 membership wrapper。因此这两类测试各自验证了正确的内部处理，但未验证 production router 的前置响应。

**建议修复：** 在实际 P1 路由边界保留可靠、稳定的工作空间失权 code/状态，并区分当前成员的操作限制；或对已知工作空间的404做受控成员/工作空间复查后清内容。不要将任意 source/update 404 当项目删除。时区 PUT 可移到成员授权组，复用 handler 已有的事务内 owner/admin 检查，避免提前抹掉错误分类。若修改通用中间件，需明确兼容范围，不把数据库暂时失败冒充永久撤权。

**必须补的验证：** 经真实 `NewRouter` 或与生产完全一致的路由及中间件层发送请求，分别验证：①原成员撤权后 overview/list/replay/execution-evidence 返回能触发保护清理的合同；②member/降权管理员时区写入只拒绝操作且仍能读项目；③source-only403、source/update404仍不撤销 workspace；④数据库查询失败不清用户草稿。再用当前 core access wrapper 消费实际 wire fixture 验证缓存/草稿结果，不能仅 mock一个理想403响应。

证据为完整 production 路由→middleware→API client→Query 渲染调用链；本 lane 没有改测试或运行新的 router fixture。发现已及时发给主代理。

## 其它新增实现检查

- 通知 details 已改为 `map[string]string`，revision 十进制字符串；真实 inbox 混合旧/新通知响应回归通过，未再破坏既有解析合同。
- 真实 LeaveWorkspace 回归已补 pending/delivered 两种 outbox 状态；不再错误要求真实撤权物理删除既有 inbox，分别验证访问拒绝和无新投递。它关闭上一轮“简化撤权替身”的数据行为证据限制；上述真实路由错误合同仍需补。
- 新 execution evidence GET 要求工作空间/项目/update/revision 内确实引用 task，然后在同一 RR 中重新锁成员及 task/agent/allowlist 或 chat creator 授权，才返回任务和消息。外部 source/update404不误报 project_not_found，source-only403使用独立 code；跨空间、未引用任务、private-agent 和 chat 非 creator 回归 PASS。
- CLI 提供显式 expected-description-revision/expected-revision，校验安全整数，版本仅作为调用者传入的 precondition；不会自动获取最新版并绕过用户比较。428 指导读回审阅，不自动重试409。对应 CLI 组独立 PASS。
- ADR-05 C 的服务端变更只去掉版本变化时清空 last_id：仍校验所有 cursor 父身份/signal/version，在本请求 RR 中重新计算全正式集合和当前 suffix；最后返回当前 snapshot/version/total。锚点失效仍是排序边界；移入低 ID 的风险需从头刷新，这一点已写入批准合同。未新增缓存、表或绕过当前权限。相关 DB 回归 PASS。最终性能及 sticky UI 证据归对应 lane。

## 本 lane 独立验证

数据库仅使用父代理允许的 association 专库；先 source `.env.worktree`，再 source `.omx/projects-p1-test-env/association.env` 覆盖。service 与 handler 顺序运行，没有复用父 verification 库；所有 Go test 经 `scripts/go-test-with-agent-cli-guard.sh`，未执行真实 agent CLI。

| 命令（Go cwd=`server`） | 实际结果 |
| --- | --- |
| `go test -race ./internal/service -run "TestProjectAutopilot\|TestProjectDeletionStopsStaleAutopilotDispatch" -count=1` | PASS，2.523s |
| `go test -race ./internal/handler -run "TestProjectAssociation\|TestProjectScopeView\|TestProjectDeletionRemovesExistingViewPreferences\|TestProjectDeleteFailureRollsBackEveryCleanupStage\|TestProjectHealth\|TestProjectUpdate\|TestProjectOperationDenial\|TestProjectMachineAction\|TestProjectMissingMembership" -count=1` | PASS，5.310s |
| `go test ./cmd/multica -run "TestProjectUpdateRevisionFlagsRegistered\|TestRunProjectUpdate" -count=1` | PASS，0.492s |
| `go vet ./internal/handler ./internal/service ./cmd/server ./cmd/multica` | PASS，exit 0 |
| `git diff --check -- server` 及 `git diff --check bb1c20dcb..HEAD -- server` | PASS，无输出 |

没有 lsp_diagnostics/gopls，使用 Go 编译/vet 静态诊断。没有重跑全仓 Go、前端全量检查、浏览器或 C 性能；这些仍由主代理与对应 lane 验证，不能把本报告当作整体验收通过。

## BR-03 服务端修复复核（同日追加）

复核对象：HEAD `d61757952` 加当前未提交修改。**后端代码 APPROVE**，当前后端范围无未解决的必须修复项。

- `cmd/server/router.go` 已将 planning-timezone PUT 移至 membership 组；`handler/project_timezone.go:107`—`:115` 仍在 RR 内检查 owner/admin，普通成员准确得到 `403 project_permission_denied`，未放宽写权限。
- `middleware/workspace.go` 新 `writeWorkspaceAccessDenied` 保持隐匿语义及旧 HTTP 404 / `workspace not found` 文本，同时附稳定 `workspace_access_denied`。UUID membership 与 slug resolution 都仅将 `pgx.ErrNoRows` 归为失权/不存在；数据库故障为503，不以永久撤权擦除输入。
- 真实 JWT / production router 测试 `TestProjectPermissionErrorsSurviveActualWorkspaceRoutes` 经 PUT timezone→GET project→POST leave→GET project/overview，覆盖此前 handler-only 测试未覆盖的前置中间件。已读取 `.omx/p1-router-permission-red.log`：旧 router 确实产生无 code 403/404；已读取 `.omx/p1-router-permission-green.log`：middleware 1.710s、cmd/server 2.294s、handler 2.661s 均 PASS。执行者为父代理，报告不冒称本 lane 重跑。
- `workspace_access_error_test.go` 四个 UUID/slug × NoRows/DB-error 分支与生产错误区分一致。workspace deletion manifest 已补列五张 P1 表，与前轮已审的实际 DELETE SQL 对齐。
- 本 lane 再次执行 `git diff --check -- server`，PASS；按指令未重复整个旧矩阵，父代理继续全仓 Go 验证。

**历史交接状态（已由下方追加确认关闭）：** 此次复核时 `packages/core/projects/access.ts:31` 的 `isProjectAccessLost` 尚未消费 `404 workspace_access_denied`，Web/Desktop 仍需新增该精确分支及 wire-fixture/缓存清理验证。移动端 `apps/mobile/data/realtime/project-access.ts:60` 已加入对应分支并区分普通资源404。父代理已明确把 core 接线交 UI lane 收尾。不要把这里的后端 APPROVE 解读为 BR-03 端到端关闭；core 落地后只需对此分支做快速追加确认，不需重跑原来的所有锁/删除矩阵。

## BR-03 跨端依赖最终关闭（2026-10-05 22:08 Asia/Shanghai）

复核对象：HEAD `ff21a670c12b5ae9bc17832741ce49d41fa778e3` 加 UI 当前未提交的 access/session-cleanup 修改。**最终 APPROVE：BR-01、BR-02、BR-03 无剩余必须修复项。**

`packages/core/projects/access.ts:35` 已精准识别 `404 + workspace_access_denied`，通过现有 `clearProtectedProjectContent` 清工作空间保护查询及草稿；`:115` 对外重新抛错时保留该 code，供视图 guard 继续识别。普通 source/update404、操作级403、503 不进入撤权清理。此前已审的移动端对应分支与服务端真实 router wire code 一致，因此 BR-03 之前保留的 Web/Desktop 交接条件现已满足。

另外确认真实 `clearClientSessionData` 已调用 `resetProjectAccessSession`；sessionGeneration 纳入 epoch，并在旧请求 success/error 返回时检查，避免重置后旧响应重新填入新会话。这是本次读取的边界事实；其它 UI RR-02/RR-03 仍由 UI lane 独立验收。

本 lane 实际执行 `./node_modules/.bin/vitest run projects/access-lifecycle.test.ts`（cwd=`packages/core`）：**1 文件 / 8 测试 PASS，1.02s**。测试覆盖真实失权 code 清缓存，source/update404、两种操作/source403、503 保留内容，以及实际 session cleanup 与晚响应拒绝。`git diff --check` 对本次 access/session-cleanup 文件通过。按要求未重复后端旧矩阵，没有新增数据库运行、产品源码修改或提交。
