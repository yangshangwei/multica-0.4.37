# P1 项目关联写入与删除 fence

执行工作树：`/Volumes/artisan/code/2026/multica-projects-p1`。本记录仅覆盖 foundation 关联 writer 子 lane；不代表 P1 整体或部署完成。

## 范围和实现

- `server/internal/service/issue.go`：普通创建、父项目继承、SourceContext 创建共享 `CreateInTx`；事务先 workspace KEY SHARE，最终项目用统一 `LockProjectForAssociationNowait`。普通 `Create` 对 NOWAIT 拒绝最多重新执行四个完整事务，每次失败先回滚，不重试一般错误或网络异常。
- `server/internal/handler/issue.go`：单条、批量均通过 `updateIssueAtomically`；保留批量逐项事务语义，在 issue 行锁内合并最新字段后锁最终项目，失败整个事务回滚并有界重试；已删除目标返回 400，耗尽锁竞争返回 409。保留 T1 admission 检查、CAS、描述合并和执行规则。
- `server/internal/handler/triage_actions.go`：接受和候选校验均改为 foundation 统一 NOWAIT 查询，仍在最终事务锁项目；接受对 NOWAIT 有界重试。手工和 CSV 候选共用 `triage.go:createTriageItemInTx` → `triageValidateCandidates`。候选来源引用仍可在项目删除后保留，接受时不能复活已删除项目。
- `server/internal/handler/chat.go`：创建和换项目使用统一 SHARE，更新路径增加 workspace fence。
- `server/internal/handler/autopilot.go`、`autopilot_template.go`：普通/模板创建与编辑按 workspace → subscriber/member → project → agent/squad/autopilot 排序。schedule/webhook/trigger 编辑在最终事务重新锁项目及 automation，并拒绝删除后迟到的启用操作；已删除项目的自动化仍允许修改标签和保持触发器禁用。
- `server/internal/service/autopilot.go`：发现自动化实际创建直接使用 `CreateIssueWithOrigin`，并不经过 IssueService；补上该真实旁路。`create_issue` 和 `run_only` 最终事务均锁 workspace → project → autopilot，重新检查 `pause_reason=project_deleted`；run_only 的 task 和 run link 同事务提交后再通知。普通 paused 自动化的手工运行语义保留。

所有批量任务路径每笔最终事务仅持一个项目锁；T1 batch/CSV 仍逐项提交，不引入跨项目同事务锁集合。项目删除持项目 UPDATE，foundation 删除实现按 UUID 排序锁关联 automation。

## 保留的既有协议

- `server/pkg/db/queries/project_resource.sql`：create/update/delete 的 materialized project fence 使用更强 `FOR NO KEY UPDATE`；不降级为 SHARE，以保留 T1 资源快照不可变性。workspace 前置 fence 已由 foundation SQL owner 接入并重新生成。
- `server/internal/handler/project_execution_squad.go`：默认小队配置已有项目 UPDATE；该文件已交还 foundation owner 接入统计字段与 workspace 前置 fence，本 lane 不编辑。
- `server/internal/handler/lifecycle_handoff_transaction.go`：`writeLifecycleHandoffInTx` 先锁 workspace；`persistLifecycleHandoff` 已对 NOWAIT/死锁/snapshot change 进行四次完整重试，commit 错误不重试，无需修改。
- `server/internal/handler/onboarding_shim.go`：两个直接 CreateIssue 入口的 ProjectID 都是 invalid UUID，不写项目关联。

## 测试证据

新增测试文件：

- `server/internal/handler/project_association_concurrency_test.go`：15 个真实 handler 入口的最终 commit fence，以及与真实 DeleteProject 的 writer-first/delete-first 双顺序；4 个 NOWAIT 四次完整事务回滚场景。锁探针使用 NO KEY UPDATE NOWAIT，刻意排除旧 FK KEY SHARE 带来的假阳性。删除阻塞使用实际 backend PID + pg_blocking_pids 证明，而非固定延时猜测。
- `server/internal/service/project_association_dispatch_test.go`：两个执行模式 × project_deleted/ordinary_pause；两个模式最终事务持项目共享锁。

TDD 证据：

1. `/tmp/p1-association-red.log`：修复前 15:36 的真实入口测试中 11 条因“did not retain project association fence through commit”失败；T1 候选/接受既有锁已通过。早期 fixture 错误（路由 Context 被替换、issue counter）已在该有效 RED 前修正，不将那些错误算作功能缺陷证据。
2. `/tmp/p1-association-extended.log`：15 入口锁屏障 + 4 回滚场景通过（2.308s），发生在后续真实 DELETE 双顺序扩充之前。
3. `/tmp/p1-dispatch-red.log`：删除后两个模式仍从旧 active 快照创建新 issue/task，明确 RED。
4. `/tmp/p1-dispatch-green.log`：删除后跳过、普通 paused 手工运行保留，四场景 GREEN（0.775s）。
5. `/tmp/p1-service-regression.log`：dispatch 六场景、Autopilot 服务相关测试、`TestTriageProjectResourceWritesSerializeWithExecutionSnapshot` 通过（1.510s）。后续 workspace fence 调整需在最终回归中复核。

验证限制与运行环境：首次广义 handler 回归与另一进程复用 TestMain 固定 workspace，被对方 cleanup 删除，导致后半 membership/FK 失败；不能将该运行作为产品回归结论。父 lane 已创建并完成专属 association 测试库迁移，最终验证使用 `.env.worktree` + `.omx/projects-p1-test-env/association.env` 覆盖加载。部分编译尝试被其它 lane 尚未完成的 health/progress 测试符号阻断，未修改他们的代码或绕过检查。

最终隔离数据库运行结果如下。这里的“通过”只覆盖本 lane 的测试范围，不表示全仓库检查或 P1 整体验收完成。


## 最终验证（2026-10-05 16:29 Asia/Shanghai）

每次从工作树根执行 `set -a; source .env.worktree; source .omx/projects-p1-test-env/association.env; set +a`，使用父 lane 已完成全部迁移的本地专属 association 数据库；不改动 `.env.worktree`，不连接默认库。Go 测试都经过 `scripts/go-test-with-agent-cli-guard.sh`，未调用真实 agent CLI。

| 检查 | 结果 |
| --- | --- |
| handler：`Test.*Autopilot\|Test.*Issue.*Project\|Test.*Chat.*Project\|TestProjectAssociation\|TestProjectResource\|TestTriage`，count=1 | 179 个顶层测试、313 条含子测试 PASS；0 FAIL；15.441s。原始 JSONL `/tmp/p1-association-final.jsonl` |
| 后续触发器兼容修正：`TestProjectAssociation\|TestProjectDeletedAutomation\|Test.*AutopilotTrigger`，count=1 | 20 个顶层测试、60 条含子测试 PASS；0 FAIL；2.052s。`/tmp/p1-association-final-trigger.jsonl` |
| service：`Test(ProjectDeletionStopsStaleAutopilotDispatch\|ProjectAutopilotDispatchRetainsAssociationFence\|TriageProjectResourceWritesSerializeWithExecutionSnapshot\|Autopilot)`，count=1 | 38 个顶层测试、69 条含子测试 PASS；0 FAIL；1.519s。`/tmp/p1-service-final.jsonl` |
| `go vet ./internal/handler ./internal/service` | exit 0 |
| 本 lane 全部修改的 `gofmt`、`git diff --check` | 通过 |

上面各行有重叠测试，不能把计数相加当作独立场景总数。

最终矩阵包括 15 个真实 handler writer 的两个提交顺序、4 个 NOWAIT 整事务重试、2 个执行模式的锁持有，以及删除后 stale dispatch 拒绝/普通暂停手工运行保留。手工/CSV 候选共用入口由 Triage 整组回归及调用链核对共同覆盖。

双顺序测试捕获并修复了一个真实集成缺陷：DeleteProject 原先使用 REPEATABLE READ，在等待 writer 之前建立旧快照；chat writer 提交后，删除虽成功却遗漏新 chat 行。foundation owner 将仅删除事务改为 READ COMMITTED，保留相同 workspace/revocation/member/project 排他锁和有界重试。原断言未降低，聊天孤儿 RED 随后关闭；进展/健康仍使用 RR。

最后一轮触发器兼容测试先证明标签编辑被过度拒绝（409），随后将 `project_deleted` 拒绝限定为创建/启用触发器；禁用与标签编辑可以继续，启用仍为 409，持久 enabled 始终 false。

本 lane 未新增依赖、迁移、索引或独立锁系统；未修改任务 admission、已有执行状态或删除共享资源。未执行前端 lint/typecheck、全量 Go suite、浏览器验收和生产部署，这些属于父任务整合验证。

## 独立审查整改 BR-01 / BR-02（2026-10-05 18:05 Asia/Shanghai）

依照 `superpowers:receiving-code-review` 先核查审查调用链，再用真实数据库复现后修改。来源为 `10-05-projects-p1-verification/backend-review.md`。本轮只使用已迁移的 association 私有测试库，所有测试继续经过 agent CLI guard。

### BR-01：run_only 与真实 Runtime teardown 的反向锁

- **真实 RED**：`TestProjectAutopilotRunOnlyRacesRealRuntimeTeardown` 让 dispatch 取得 autopilot UPDATE 锁后暂停；另一连接取得实际 runtime/agent 锁并调用真实 `TeardownRuntime`。`pg_blocking_pids` 确认 teardown 等待 autopilot，再同时释放 dispatch。PostgreSQL 实际以 **40P01** 中止 teardown，旧 dispatch 插入了一个 task，runtime 未被删除。证据 `/tmp/p1-br01-red.log`；这不是静态推测或 mock teardown。
- **修复**：`service/autopilot.go` 保留 workspace → project → autopilot 的删除 fence；在 task INSERT 之前按 runtime → agent（必要时 squad）取得 SHARE NOWAIT。反向锁失败退出并回滚整个 `dispatchRunOnlyOnce`，最外层重做完整准备，最多四次；耗尽返回 skipped，不在已持锁事务里重试。task INSERT 所需 owner KEY SHARE 已被本事务的更强锁覆盖，因此不会再等待 teardown。
- **锁内复核**：重新检查当前绑定、assignee/leader、归档/运行时 readiness、真实 principal 的成员资格与调用权限；成员及 allowlist 复用现成 NOWAIT 查询保持到 commit。普通 paused 自动化的手工运行保持；project_deleted 仍拒绝新增执行。
- **额外 RED**：前置检查后移除 member，旧 owner 快捷授权仍插入 task；`/tmp/p1-br01-member-red.log`。锁内成员检查后该测试 GREEN。运行时离线、绑定变化、owner 权限变化同样测试，均不插入 task。
- **双顺序证明**：dispatch 先提交时真实 teardown 等待 owner fence，随后取消并保留既有 task 历史、清掉 runtime 关联；teardown 先提交时迟到 dispatch 跳过，不生成 task。额外重放原锁环，分别覆盖有项目和无项目自动化。commit 前锁探针同时证明 member/调用 grant 不可被并发写改。
- 本轮新增 runtime/agent NOWAIT SQL 与生成文件由父代理整合，本 lane 不提交这些共享文件。

### BR-02：项目 scope view / preference 的最终写入与清理

- **真实 RED**：在 `GetProjectInWorkspace` 的真实成功读取之后暂停请求，让真实 DeleteProject 完成，再恢复写入；旧 CreateIssueView 返回 201 并留下一个 view，旧 PutIssueViewPreference 返回 200 并留下一个 preference。证据 `/tmp/p1-br02-red.log`。
- **修复**：`issue_view.go` 的共享 `writeIssueViewScope` 仅对 project scope 启动最终事务，依次持 workspace KEY SHARE、project SHARE，再 INSERT/UPSERT 并提交。`issue_view_preference.go` 使用相同函数。workspace/my scope 保持原单语句语义。删除先提交时返回 404，不重建数据。
- **正常删除 RED**：用 Go 的只读 overlay 替换为审查冻结 HEAD `bb1c20dcb16751a64931f2321a05b8773cec3aaa` 的原 `project.go`，实际正常 DELETE 后仍留下一个 preference（`/tmp/p1-br02-cleanup-red.log`）。没有修改当前产品文件。该测试还使用真实 `pinned_item` 校验 view pins。
- 父代理新增 `DeleteProjectIssueViewPreferences` 查询并接入删除事务；当前源码下正常删除测试 GREEN。新增两 writer 纳入真实 DeleteProject 的双顺序矩阵，现为 **17 个入口 × 2 顺序**。
- `project_delete_concurrency_test.go` 新增第 10 个 `DeleteProjectIssueViewPreferences` 故障注入阶段；全部 10 个阶段都断言 preference 与原 issue/autopilot/trigger/progress 一起完整回滚。

### 本轮最终验证

| 检查 | 结果 |
| --- | --- |
| `go test -race ./internal/service -run 'TestProjectAutopilot\|TestProjectDeletionStopsStaleAutopilotDispatch\|Test.*Autopilot\|TestTriageProjectResourceWritesSerializeWithExecutionSnapshot' -count=1` | 48 个顶层 / 93 条含子测试 PASS，0 FAIL，无 race，3.492s；`/tmp/p1-br01-race.jsonl` |
| 随后新增无项目锁环覆盖：`go test -race ./internal/service -run '^TestProjectAutopilotRunOnlyRacesRealRuntimeTeardown$' -count=1` | 有项目 / 无项目两个子场景 PASS，无 race，2.027s；`/tmp/p1-br01-both-bindings-race.log` |
| `go test -race ./internal/handler -run 'TestProjectAssociation\|TestProjectScopeView\|TestProjectDeletionRemovesExistingViewPreferences\|TestProjectDeleteFailureRollsBackEveryCleanupStage\|Test.*IssueView' -count=1` | 16 个顶层 / 66 条含子测试 PASS，0 FAIL，无 race，3.782s；`/tmp/p1-br02-race.jsonl` |
| 相同 handler 范围不带 race | 16 个顶层 / 66 条含子测试 PASS，2.186s；`/tmp/p1-br02-final.jsonl` |
| `go vet ./internal/handler ./internal/service`，本 lane `git diff --check` / gofmt | exit 0 / 通过 |

上述重复范围不累加计数。全仓库、前端和部署仍由父任务整合验证；本轮不将它们标为通过。
