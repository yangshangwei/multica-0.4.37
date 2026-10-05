# P1 后端独立验收审查

- 日期：2026-10-05
- 审查工作树：`/Volumes/artisan/code/2026/multica-projects-p1`
- 基线：`0af59c5d5`
- 冻结生产 HEAD：`bb1c20dcb16751a64931f2321a05b8773cec3aaa`
- 结论：**REQUEST CHANGES**。1 项 HIGH 新增锁序回归；1 项 MEDIUM 项目删除闭合缺口。未修改产品代码或测试，未提交。
- 范围：62 个变更的非测试后端文件，包括 P1 handler/health/service、查询及生成 SQL、536—549 迁移、runner/worker/flags/routes；并追踪现有来源授权、成员撤权、runtime teardown 与 scope-view 写入口。

## 必须修复

### BR-01 — HIGH：run_only dispatch 在 autopilot 锁之后重新取得 agent/runtime 锁，与真实 runtime teardown 构成环

定位：`server/internal/service/autopilot.go:1044`（新增最终事务锁）、`:656`（`LockAutopilotForUpdate`）、`:1047`（`CreateAutopilotTask`）。

触发条件：一个 active、run_only 的自动化绑定有效 agent/runtime；自动化经过前置 readiness 检查后，与用户删除该 runtime 或保留期 teardown 同时执行。

确定的锁图：

1. dispatch 调用 `lockAutopilotProjectForDispatch`，取得 workspace/project 共享锁及 autopilot **FOR UPDATE**。
2. teardown 在 `server/internal/service/runtime_teardown.go:62` 取得 runtime **FOR UPDATE**，`:66` 取得 user-agent **FOR UPDATE**。
3. dispatch 的任务 INSERT 调用 `server/pkg/db/queries/autopilot.sql:608` 的 `lock_task_owner_rows`，在 `server/migrations/284_task_owner_row_fence.up.sql:138` / `:148` 等待 agent/runtime **FOR KEY SHARE**。
4. teardown 在 `runtime_teardown.go:111` 执行 `PauseAutopilotsByUnboundAgents`，其 `autopilot.sql:94` UPDATE 等待 dispatch 已持有的 autopilot。

两方互等：`dispatch: autopilot → agent/runtime`；`teardown: runtime → agent → autopilot`。这个环不依赖项目 FK、非法数据或真实 agent CLI；本次把 task INSERT 纳入持 autopilot 锁的事务后新引入。即使自动化没有 project_id，helper 仍取得 autopilot 锁，因此影响所有 run_only 自动化。

影响：PostgreSQL 以 40P01 中止一方。该 dispatch 路径没有整事务 retry；`autopilot.go:592`—`:598` 将错误记为 failed run，或 runtime 删除返回失败。这不满足 P1 保留既有执行行为和关联锁序无环的验收要求。

建议：在最终 INSERT 前按与 teardown 兼容的顺序锁定/重验 runtime 和 agent，再取得 autopilot；或者对这种反向锁采用 NOWAIT 并整笔回滚后重新做 readiness/授权/项目检查。不要只在已持锁事务内部重试，也不要移除项目删除所需的最终 fence。具体方案需要同时照顾现有 runtime→agent 协议和 project 删除协议。

应补验证：两个真实连接、独立 fixture；先让 dispatch 持 autopilot 锁，再令 `TeardownRuntime` 持 runtime/agent 锁，通过锁屏障同时释放后续步骤。不得出现 40P01 或超时，不能插入指向已删除 runtime 的 task；反向顺序也验证一次。现有 `TestProjectAutopilotDispatchRetainsAssociationFence` 仅证明 project fence 存在，未覆盖这一环。

证据类型：完整生产调用链与 PostgreSQL 冲突锁静态证明；本 lane 未运行数据库复现。已向主代理发送具体锁图，供 owner 用隔离数据库加回归。

### BR-02 — MEDIUM：项目 scope-view 写入未参与删除 fence，preferences 也没有清理

定位：`server/internal/handler/project.go:911`（项目删除清 scope views）、`server/internal/handler/issue_view.go:199` / `:211`（普通 SELECT 后裸 INSERT）、`server/internal/handler/issue_view_preference.go:126` / `:138`（同样的检查后写入）。

触发场景 A：成员保存项目视图，在 `GetProjectInWorkspace` 成功后暂停；管理员完成 DELETE，已经运行 `DeleteIssueViewsByProjectScope`；保存请求恢复执行 `CreateIssueView` 并返回 201。`issue_view.scope_id` 没有 project FK，也没有应用层锁，结果是引用已删除项目的活动视图。它仍被 `CountIssueViewsByOwner` 计入额度，并可由 view ID 读取；项目页面已不存在。

触发场景 B：先为项目写 `issue_view_preference`，再正常删除项目，无需并发即可留下该项目 scope 的 preference。删除 SQL 只清 `issue_view` 及其 pins，整个删除链未清 `issue_view_preference`；其 upsert 也可在项目删除后按同样竞争时序重新插入。

影响：P1 design §8 明确要求清理 scope 视图并且所有关联写仅留下“加入失败”或“加入后解除”。当前删除事务即便换为 RC 也不能约束未取得项目锁的 writer，仍留下项目专属孤儿数据。这个写入口原已存在，本次 P1 删除完整性承诺尚未覆盖它；不将其描述为本次新增的 UI 回归。

建议：project-scope 的 CreateIssueView 和 PutIssueViewPreference 在最终写入事务内先取得 workspace/project fence，再校验并写入；项目删除事务一并清 `scope_type='project' AND scope_id=project_id` 的 preferences。保持 workspace/my scope 原语义，复用现有查询/事务模式，不新增依赖。

应补验证：把保存视图与 preference 加入项目删除并发矩阵，覆盖 writer-first / delete-first 两序，并检查 `issue_view`、`issue_view_preference` 和 view pins；另验证无并发删除能清现有项目 preferences。当前 `project_association_concurrency_test.go:130` / `:346` 的 writer 枚举未包含这两条入口。

证据类型：生产 SELECT/INSERT 与无 FK schema 的静态时序证明；未运行数据库复现。

## 已核对通过的设计点

- 新发布/历史/健康事务的第一条 SQL 设置 RR；workspace fence 后取得 actor 撤权 advisory fence，再对 member 做 locking read。旧 RR membership 与已删除 allowlist 行不会仅凭普通 SELECT 恢复授权。
- 来源 issue/execution/chat/agent/allowlist 均使用事务 queryer 和 NOWAIT 来源锁；历史会重新授权并去除无权来源的 label/href/current_version。chat 保持 creator-only，普通执行沿用可见性而非调用权限。
- 发布/更正 request identity 包含 actor/workspace；相同请求重放原 result_revision；更新 kind/验收描述版本不可改；预览 hash 包含规范化业务输入、当前证据版本、收件人及请求的统计版本。
- 描述真实变化缺 CAS 返回 428、过期返回 409；nullable 属性在项目锁内合并；旧非描述字段不回写旧描述。状态审计与状态变化同事务，不直接改任务或启动执行。
- 正式统计限定 `not_required/accepted`，unknown 保留 N/U 并标 incomplete；旧 `done_count` 保持 completed+cancelled。风险分页与 overview 共享 RR snapshot、版本变更回第一页。
- 删除改用 RC，在项目独占锁之后扫子记录；issue/autopilot/chat 主要新增关联 writer 的 fence 均与删除锁冲突。暂停自动化并禁用 trigger 不清历史执行。
- outbox 先 workspace→recipient fence/member→project，再 `FOR UPDATE SKIP LOCKED` 队列行；稳定 inbox ID 与 delivered 更新同事务；失败重试/12 次 dead-letter 明确；站内进展事件只发身份和 revision。
- 536—549 没有新增 FK/cascade；索引 up 均单独 concurrent 文件；537—549 已注册失效索引清理；down 的独占锁与保留数据 guard 已读。迁移实际运行证据以 migration lane 报告为准。

## 验证与证据限制

本 lane 实际执行，均在指定工作树内：

| 命令 | 结果 |
| --- | --- |
| `go test ./internal/projecthealth -count=1`（cwd=`server`） | PASS，0.572s |
| `go vet ./internal/handler ./internal/projecthealth ./internal/service ./cmd/migrate ./cmd/server`（cwd=`server`） | PASS，exit 0 |
| `git diff --check 0af59c5d5..HEAD -- server` | PASS，无输出 |
| `git rev-parse HEAD`（审查末尾） | 仍为 bb1c20dcb16751a64931f2321a05b8773cec3aaa |

当前工具没有 lsp_diagnostics，PATH 也没有 gopls；使用 Go 编译/vet 做静态诊断替代。没有访问 verification DB，没有并行运行共享 handler fixture，没有访问生产或执行真实 agent CLI。

另外一项验收证据限制：`project_update_concurrency_test.go:383` 的 `progressRevoke` 替身显式删除 inbox 并先取 workspace lock；真实 `revokeAndRemoveMember`（`workspace_revoke.go:43`）不走这两个步骤。因此该测试的“撤权后 inbox 行为 0”不能直接证明真实端点的删除行为。本次未把它当作已确认越权漏洞：当前 membership gate 仍拒绝撤权后的读取，outbox 的 recipient fence 仍能阻止撤权后投递。最终验收若声称覆盖真实撤权/恢复，应补至少一个真实 RemoveMember/LeaveWorkspace 入口测试，并分别断言 API 无权与 outbox 无新投递，避免混淆行保留和访问授权。

整改后需要 owner 重跑对应并发回归并再次审查。I1 真实迭代归属/结束快照不在当前实现中，本报告不声称已验证；Web/Desktop/mobile 与性能报告由对应 lane 提供。
