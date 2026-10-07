---
status: in-progress
branch: codex/projects-p1
timestamp: 2026-10-06T22:20:45+08:00
head: 115b4cd28277941a8d9ad8fda5acecf99b999152
evidence: handoff-2026-10-06-cg-evidence.json
---

# 迭代管理 I1：CG 接续交接

**核对结论：FG、LG、HG 已通过；下一业务阶段确实是 CG。** 用户提到的「原子结束、结转、整体禁用和通知」方向正确，完整范围还包括进行中取消、结束并开始下一期、真实关闭快照落库及逾期提醒。

**剩余工作不只有验收。UG 的 I1 Web/Desktop 产品实现尚未开始。** 推荐主线为 **CG → UG → FCG/VG**；客户端可以在稳定 API 上提前实施，但完整 UG 签收须等待 CG。双端 E2E、P95、全量必要检查与远端 CI 都没有 I1 完整通过证据，不能称为 I1 已完成。

本次仅核对源码、任务、Git、旧会话、现有日志及远端状态并写交接文档；未修改产品代码、重新运行产品测试、提交、推送或部署。

## 1. 接手基线

- 工作目录：`/Volumes/artisan/code/2026/multica-0.4.37`。
- 分支：`codex/projects-p1`。
- 当前 HEAD：`115b4cd28277941a8d9ad8fda5acecf99b999152`，LG/HG 签收文档提交。
- 产品实现提交：`1247d2728`（生命周期/历史）、`4c9be7652`（普通任务/T1 原子安排和最后审查修复）。
- 父任务：`.trellis/tasks/10-05-iterations-i1`；继续现有六个子任务，不另建任务树。
- lifecycle/history 已 completed；closure/clients/verification 为 planning；父任务与 foundation 仍 in_progress。foundation 保留至 FCG。
- 本交接会话 `task.py current --source` 返回 none，仅表示当前会话未绑定任务。继续实施时按 Trellis 会话机制绑定已有 closure 任务，不覆盖其他会话绑定。

原开发会话 `01a10f96-64ce-7463-b2da-646c7cbe5003` 本次读取为 **idle**，最后一轮 **completed**，无错误。它已提交 LG/HG，不再是旧交接所写的“正在补 T1 接入”。这是读取时状态，新会话开始仍须检查最新提交、差异及写入所有权。

本次开始时 tracked 工作树及暂存区均无差异，只有两份先前会话的 untracked 文件：

- `handoff-2026-10-06-lg-hg.md`
- `handoff-2026-10-06-lg-hg-evidence.json`

保留它们作为历史，不把其中的旧 RED 或 33 个未提交路径当成当前工作。不得 reset、clean 或盲目暂存所有文件。新交接和对应证据也尚未提交。

## 2. 必读顺序与证据优先级

1. 根 `AGENTS.md`、`CLAUDE.md`、`.trellis/workflow.md`。
2. 本文件和 [本次核对证据](handoff-2026-10-06-cg-evidence.json)。
3. [LG/HG 集成验收](lg-hg-verification.md)、[机器摘要](lg-hg-verification.json)、[FG 验收](../10-05-iterations-i1-foundation/fg-verification.md)。
4. 父 [设计](design.md)、[API 合同](api-contract.md)、[执行顺序](implement.md)、[测试矩阵](test-spec.md)。
5. [closure 实施清单](../10-05-iterations-i1-closure/implement.md) 及该目录的 prd/design/implement.jsonl/check.jsonl；写代码前读取涉及层的规范。
6. 随后读取 [clients](../10-05-iterations-i1-clients/implement.md)、[verification](../10-05-iterations-i1-verification/implement.md) 和 [原始 I1 产品需求](../../../docs/plans/2026-10-04-work-management-prds/iterations-prd.md)。

最新验收记录、对应提交和当前源码优先于历史状态文字。旧 dated LG/HG handoff、父 `verification.md`、`design.md` 开头和部分 task notes 仍含旧的“FG 未通过 / 下一步 LG/HG / T1 接入未完成”；这些不再是当前进度。设计/API 中的业务合同仍有效。不要重做 S0–S2、列表 API、普通创建确认或 T1 联合事务。

## 3. 阶段状态

| 门槛 | 已核实状态 | 接续内容 |
| --- | --- | --- |
| FG | passed | writer 收口、持久 operation、设置、授权/迁移与基础容量已有证据；foundation 仍负责共享整合 |
| LG | passed | create/edit/list/detail、preview/start/move、计划取消/未使用删除、普通任务及 T1 安排已完成 |
| HG | passed | 真实事实投影、原始承诺、统计/图表、受保护分页和冻结 payload 已验证；真实关闭落库属于 CG |
| CG | 尚未实施 | end、active cancel、handoff、disable、快照和 outbox 原子写入、投递及逾期提醒 |
| UG | 尚未实施 | I1 core schema/query/mutation、共享界面、Web/Desktop 路由及完整交互与兼容 |
| FCG | pending | LG/HG/CG/UG 汇合后的共享 SQL/router/sqlc 与回归；不是 CG 的启动前置 |
| VG | 尚未签收 | 29 项 ITR-AC、跨模块、迁移/故障恢复、双端 E2E、P95、全量检查和远端 CI |

源码核对：`server/internal/service/iteration_lifecycle_preview.go` 的 `normalizeLifecycleDraft` 当前只支持 start/move/delete 和 planned cancel；end/handoff/disable 仍拒绝，active cancel 尚未开放。`iteration_lifecycle_apply.go` 只把 cancel 接到 `CancelPlannedIteration`。已有 draft 类型、表、冻结 payload helper 或同名 `lifecycle_handoff.go` 不能证明 I1 CG 已实现。

`iterations_i1` 发布默认仍关闭，`atomic_handoff=false`；CG 实现并验收前不得提前声明原子交接能力，I1 完整交付前不得开启发布开关。

## 4. CG 具体接续范围与验收

继续使用现有 preview/apply、持久幂等与事务框架，不新增平行协议。主要入口：

- `server/internal/service/iteration.go`、`iteration_lifecycle_preview.go`、`iteration_lifecycle_apply.go`。
- `server/internal/iteration/transaction.go`、`membership.go`、`history*.go`。
- `server/internal/handler/iteration_management.go`、`iteration_operations.go`、`iteration_settings.go`。
- `server/pkg/db/queries/iteration*.sql`、生成代码、`server/pkg/protocol/iteration.go`、`server/cmd/server/router.go` 由一个负责人顺序整合。

按以下顺序落地并记录真实 RED/GREEN：

1. **结束 / 进行中取消。** 锁后重新读取完整集合并核对 preview hash、权限、revision、目标。同一事务冻结范围/原始承诺/事件/统计/图表/当时摘要及去向，释放源归属，应用合法结转，写稳定 operation 结果和通知 outbox。任一失败零变化；取消快照类型为 cancelled。
2. **结转和结束并开始下一期。** moves 恰好覆盖未完成 R，无重复、遗漏或额外终态。目标只能是 planned；完成/取消/移出的目标为 null 且计数不增，合法结转 +1，重试不能再增。handoff 可向多个计划结转，但仅一个显式目标成为下一 active，其新 O 同时包括原计划任务和流入任务。日期跨午夜、已有终态选择和目标竞争全部重新核对。
3. **整体禁用。** 覆盖当前期及全部未来计划、包括第一页以外的数据；全部移出、取消/冻结与 settings 关闭同成败，拒绝客户端自定去向。超过部署操作上限 2,000 返回 413 且零变化，禁止通过分页或分块偷偷完成。
4. **通知与逾期提醒。** 补开始/结束/取消/日期修改的通知接线。operation/recipient/kind 去重；outbox 投递前重新鉴权，稳定 inbox ID 与 delivered 标记同事务，撤权后清理且重试不重建保护内容。逾期仅提醒、不自动结束，按迭代时区每日去重；协调人为空回退 started_by，失权停止个人提醒；无外部邮件或聊天。
5. **竞态与恢复。** 两连接 barrier + 中间写点故障注入；覆盖目标取消、预览后 done、权限/成员变化、daemon 完成、删除和结束竞争。响应丢失后用原 request_id 查询或重试，不能另起身份；快照/事件/累计结转/通知只产生一次。真正关闭后再改状态、项目、标题、重开或删除任务，历史 digest 不漂移。

测试场景继承父 test-spec，重点 ITR-AC-05、15–20、22–23、27–28 及工程场景 3–8、14。本节是既有合同的执行摘要，不缩减父规格。

## 5. 后续 UG、FCG/VG

UG 需要实际建设 I1 客户端能力，不是只加测试。共用业务放 `packages/core` 和 `packages/views`，路由分别接 Web/Desktop；包括设置、迭代列表/详情、任务/T1/项目入口、预览确认、历史图表和数据表。涵盖 403/409/未知结果保留输入、键盘操作、长文本、旧服务能力探测和 malformed 响应。Mobile 新编辑页面后置，但旧客户端读取/编辑兼容及服务端门槛必须验收。

FCG/VG 完成共享 SQL、路由和 sqlc 整合，跑真实 Web 与 Electron 的创建→安排→开始→完成一项→预览结转→历史流程。P1 的双端通过记录不能代替 I1。

P95 要对 1,000 任务的 detail/preview/closure 和更大禁用全集做重复采样，记录硬件、数据库、数据分布、冷热、锁等待、查询计划和冻结的验收阈值。现有 LG 的 1,000 项成功、2,001 项拒绝只是容量/单次时延观察，不是 P95；PRD 的 2 秒为建议，不能直接写成达标。

## 6. 本次核对的验证证据

本次重新解析了已提交摘要所引用的五份 Go JSONL，**所有 SHA-256、测试计数和包结束结果都吻合**。这些是历史运行的核对结果，不是本会话重跑。

| 原始日志 | 核对结果 |
| --- | --- |
| `/tmp/i1-lg-hg-final-handler.jsonl` | 638 pass、2 skip，包 pass |
| `/tmp/i1-lg-hg-final-domain.jsonl` | 240 pass，三个包 pass |
| `/tmp/i1-lg-hg-integration-accepted.jsonl` | handler179 + service154 + router5 pass、2 skip，三个包 pass |
| `/tmp/i1-lg-hg-builtin-accepted.jsonl` | 1 pass，包 pass |
| `/tmp/i1-lg-hg-pool-fix-accepted.jsonl` | handler113 + router1 pass、2 skip，两个包 pass |

计数含子测试且选择重叠，不能相加成唯一测试总数。两个 skip 是既有 opt-in 的 `TestTriageQueueScaleBaseline`、`TestTriageWireFixtures`，不算通过。

最新审查修复了 settings 事务持有连接期间又向 pool 查询 capability 的问题；必须把调用者的 transaction-bound q 传入，不能靠扩大连接池掩盖。最后 114 pass 的回归覆盖该修复，详见 LG/HG 记录。

TS 日志末尾核对到 test 5/5、lint/typecheck 15/15 tasks successful，均 0 cached；已提交摘要记录 10,054 TS 测试通过，未包含 Mobile。Go 全量 build/vet 的 exit 0、84 个 sqlc 文件二次生成稳定来自已提交验收记录；本次未重跑，空 build/vet 日志本身不独立证明退出码。

远端只读核对：

- `git ls-remote --heads origin refs/heads/codex/projects-p1`：退出 0，无匹配分支。
- `gh run list --repo yangshangwei/multica-0.4.37 --commit 115b4cd28277941a8d9ad8fda5acecf99b999152 ...`：退出 0，结果 `[]`。
- 因此当前 HEAD 没有远端 CI 记录，不能说 CI 已绿或已失败；此查询不评价其他提交/分支。完整 I1 推送、合并、部署均未验收。

原始 `/tmp` 日志可能丢失；本次证据 JSON 保存哈希与核对结果。未来源码有改动，应针对新的实际提交重新验证。

## 7. 执行约束与验证入口

保持 P1/T1 的既有锁序、RR/RC 区分、CAS/NOWAIT 和外层重试预算。helper 借用调用者事务，不能嵌套提交/重试/广播。业务时间在全部锁后采样，sequence 为权威顺序。管理操作不得启动、停止或重启执行。

复用共享 planning_timezone；无新依赖、无新增 FK。新增索引仍为独立单语句 CONCURRENTLY 迁移，编号按实际当前最大值分配。不要改写已验证的 550–566 历史。

接手先运行只读检查：

```sh
git status --short --branch
git log -8 --oneline
git diff --stat
python3 .trellis/scripts/task.py current --source
make status
make list
```

需要数据库验证时使用新会话独占且已迁移的测试库，显式设置 DATABASE_URL。不要盲目复用记录中的旧测试库，也不要把环境名当作隔离证明。默认测试始终走 agent CLI guard，不触发本机真实 agent。

按实际变更先窄回归，再执行仓库要求：

```sh
# DATABASE_URL 已显式指向本会话独占测试库。
bash scripts/go-test-with-agent-cli-guard.sh -- go -C server test -race -p 1 -parallel 2 ./internal/iteration ./internal/service ./internal/handler ./cmd/server -run '^(TestIteration|TestIssueIterationAssignment|TestTriage|TestHistorySnapshot)' -count=1
go -C server build -p 2 ./...
go -C server vet -p 2 ./...
git diff --check
```

上述选择不是完整 CG/VG 验收，CG 新测试需明确加入。最终按 Makefile/package.json 跑必要全 Go、lint/typecheck/TS、双端 E2E、迁移与 make check；Mobile 按本包规则单独验证。SQL 变化由一个负责人生成 sqlc，二次生成核对无漂移。

继续实施不需要重问已批准范围；当前交接不授权推送、合并、发布或启用开关。后续提交按 Lore 记录已测与未测，保留他人文档，不使用全量暂存。

## 8. 新会话引导语

```text
继续开发 Multica「迭代管理 I1」。

工作目录：/Volumes/artisan/code/2026/multica-0.4.37
分支：codex/projects-p1
交接 HEAD：115b4cd28277941a8d9ad8fda5acecf99b999152

先读 AGENTS.md、CLAUDE.md、.trellis/workflow.md，再完整阅读：
.trellis/tasks/10-05-iterations-i1/handoff-2026-10-06-cg.md
及同目录 handoff-2026-10-06-cg-evidence.json、lg-hg-verification.md。

FG/LG/HG 已通过，不重做基础、T1 接入或历史统计。先核对最新 git 状态和旧会话写入所有权，沿用现有任务树，从 closure 的 CG 开始：原子结束、进行中取消、结转/结束并开始下一期、整体禁用、真实关闭快照、outbox 与逾期提醒。按父 design/api-contract/test-spec 执行并补真实数据库竞态、回滚、幂等和撤权验证。

UG 的 I1 Web/Desktop 实现仍待开发，不只是 E2E 未验收；CG 后继续 UG，再完成 FCG/VG 的双端 E2E、P95、全量检查及验收证据。远端 CI 尚无当前 HEAD 记录，发布步骤另行处理。

这是继续实施请求，不是重新规划或仅写计划。保留已有改动，按门槛持续推进，不重复确认已确定范围。使用独占测试库和 agent CLI guard；保持 P1/T1 事务约束、管理操作不干扰执行；发布默认关闭，不推送、合并、部署或启用开关。记录每阶段真实证据和剩余项。
```
