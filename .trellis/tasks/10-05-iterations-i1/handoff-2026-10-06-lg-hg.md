---
status: in-progress
branch: codex/projects-p1
timestamp: 2026-10-06T19:36:41+08:00
head: da6c0fc65306279a1a62b243a590ea87aa956710
evidence: handoff-2026-10-06-lg-hg-evidence.json
---

# 迭代管理 I1：LG/HG 集成阶段交接

本文件记录 2026-10-06 19:36（北京时间）的接手位置。**FG 已通过；LG/HG 主要实现和多组定向验证已落地，正在补 T1 接受与迭代安排的联合事务。完整 LG/HG 尚未签收，CG/UG/FCG/VG 未通过，发布开关继续关闭。**

本轮只写交接文档与证据摘要，没有修改产品代码、重新运行产品测试、提交、推送或部署。旧 [handoff.md](handoff.md) 中的 S0/S1、“FG 未通过”、服务尚未实现等段落保留了历史状态；它仍可作为设计索引，但不能替代本文件的当前进度。旧任务 notes 也有追加的历史文字，按最新证据判定。

## 1. 工作目录、分支与写入所有权

- 工作目录：`/Volumes/artisan/code/2026/multica-0.4.37`。
- 开发分支：`codex/projects-p1`。
- 最新已提交基线：`da6c0fc65306279a1a62b243a590ea87aa956710`，`docs(iterations): make verified foundation evidence unlock the next phase`。
- 基础实现关键提交：`bc5a2929b`、`aca0762a2`。此前 writer 接入已在当前分支，不重做 S0–S2，不重新搬运旧分支。
- 父任务：`.trellis/tasks/10-05-iterations-i1`。沿用现有六个子任务，不再创建平行任务树。
- 本交接会话的 `task.py current --source` 返回 none；这只是本会话没有任务绑定，不代表 I1 不存在或未开始。新会话按 Trellis 当前会话机制绑定已有任务，不覆盖其他会话的绑定。
- `/Volumes/artisan/code/2026/multica-projects-p1` 在此前交接中是保留的 detached checkout，继续使用上面的主工作目录。

**并发状态：** 生成本交接时，原开发会话 `01a10f96-64ce-7463-b2da-646c7cbe5003` 仍为 `active`，其当前 turn 为 `inProgress`，正在写同一工作区。本轮没有停止该会话，也没有向它发送指令。新会话开始前先重新读取它的状态、最新消息和 git diff；若它仍拥有共享文件的写入，先完成只读接手检查，避免两个会话同时修改、生成或提交同一批文件。不能凭这份文档假定旧会话已经停下。

证据文件 [handoff-2026-10-06-lg-hg-evidence.json](handoff-2026-10-06-lg-hg-evidence.json) 保存了 33 个未提交路径的状态、逐文件 SHA-256 和 11 份日志摘要。这是活动工作区的逐文件观察，不是原子快照，也不是代码备份。后续源码、提交和日志可能继续更新；新证据优先。

## 2. 必读顺序

1. 根 `AGENTS.md`、`CLAUDE.md`、`.trellis/workflow.md`；修改某一层前读取对应 `.trellis/spec/` 规范。
2. 本文件和同目录证据 JSON。
3. [父 design](design.md)、[API 合同](api-contract.md)、[父 implement](implement.md)、[测试矩阵](test-spec.md)。
4. [FG 验收](../10-05-iterations-i1-foundation/fg-verification.md)。这是 FG 已通过的依据；foundation 的最终完成点仍是 FCG。
5. [LG 实施清单](../10-05-iterations-i1-lifecycle/implement.md)、[LG 验证记录](../10-05-iterations-i1-lifecycle/verification.md)。
6. [HG 实施清单](../10-05-iterations-i1-history/implement.md)、[HG 验证记录](../10-05-iterations-i1-history/verification.md)。
7. 后续阶段再读 closure、clients、verification 子任务的设计与上下文清单；原始产品语义见 `docs/plans/2026-10-04-work-management-prds/iterations-prd.md`。

## 3. 已完成与尚未完成

| 阶段 | 当前事实 | 剩余工作 |
| --- | --- | --- |
| FG：基础门槛 | 已验收；W01–W17 writer、事务事实、当前授权、持久操作/设置、兼容和基础容量证据已提交 | foundation 继续拥有共享整合，直到最终 FCG |
| LG：生命周期 | 创建/编辑、完整预览、开始、归属变更、计划取消/未使用删除已有服务；HTTP 路由和普通 Issue 创建/PUT 接入存在，已有定向通过记录 | T1 接受+安排接入、列表等合同逐项核对、完整真实入口和权限矩阵、最终集成签收 |
| HG：历史统计 | 持久事实投影、原始承诺、统计/图表、冻结 payload、受保护详情/任务/事件读取已有实现；与 LG 真实流程的 A–J 验证已有记录 | 汇合后的协议/授权/路由复验和正式 HG 签收；实际结束快照落库属于 CG |
| CG：结束与结转 | planning，未实施 | end、active cancel、handoff、disable，快照/去向/结转/operation/outbox 原子提交及投递 |
| UG：Web/Desktop | planning，未实施 | core schema/query/mutation、共享视图、双端路由、错误恢复和真实流程 |
| FCG/VG：最终整合与验收 | 未通过 | 完整合同矩阵、全量必要检查、Web/Electron E2E、容量与恢复、发布准备 |

LG/HG 的“有通过记录”不等于当前整个工作树已通过全部验收。基础的旧测试结果也不能覆盖后续改动。不要用子任务 still `in_progress` 推导没有实现，也不要用代码存在推导已经完成。

## 4. 当前实现入口和未提交改动

精确路径清单与哈希见证据 JSON。未提交改动应保留并接续，不得 reset、clean、覆盖或盲目全部暂存。

| 责任 | 主要文件 |
| --- | --- |
| LG 服务 | `server/internal/service/iteration.go`、`iteration_lifecycle_metadata.go`、`iteration_lifecycle_preview.go`、`iteration_lifecycle_apply.go` 及 lifecycle/review 测试 |
| 归属和类型 | `server/internal/iteration/membership.go`、`lifecycle_types.go`；借用调用者事务的准备/提交逻辑 |
| HG 历史 | `server/internal/iteration/history.go`、`history_capture.go`、`history_snapshot.go` 及 snapshot 测试 |
| HTTP 管理与读取 | `server/internal/handler/iteration_management.go`、`iteration_history.go` 及对应测试 |
| 普通 Issue 联合创建/归属写 | `server/internal/handler/issue_iteration_assignment.go` 及测试；`handler/issue.go`；`service/issue.go` 新增 `AfterCreateInTx` 钩子 |
| T1 当前切片 | `server/internal/handler/triage_iteration_assignment_test.go`；下一步检查 `triage_actions.go`、`triage.go` 及旧会话后续新增文件 |
| 共享整合 | `server/cmd/server/router.go`、`server/pkg/protocol/iteration.go`、`server/pkg/db/queries/iteration_{lifecycle,history}.sql` 及生成文件 |
| 任务记录 | lifecycle/history 的 `task.json`、`implement.md`、`verification.md` |

当前 router 已接入 create/detail/update、issues/events、preview/apply。快照检查时未见集合级 `GET /iterations` 路由；按父合同核对列表、过滤和 keyset 分页是否仍缺失，先检查新差异再实现，不能把详情读取当成列表已完成。

普通 Issue 当前语义：创建带目标时要求 `expected_iteration_revision`；显式归属 PUT 要求 issue `expected_revision`，实际移出/切换需要理由；完成项加入 active 需确认。复用原创建 duplicate guard，归属专用 PUT 拒绝混合普通编辑，省略新字段仍保留原快路径。普通创建原本的显式分配执行意图保留一次，迭代管理本身不触发执行。

## 5. 最优先接续点：T1 接受与迭代安排

原开发会话最近执行了真实数据库失败回归：

```sh
bash scripts/go-test-with-agent-cli-guard.sh -- go -C server test -race -p 2 ./internal/handler -run '^TestTriageIteration(Acceptance|Recorder|Target)' -count=1 -json
```

该命令当时明确设置了原会话自己的本地测试库 `DATABASE_URL`。新会话不要复用正在运行的测试库；数据库隔离要求见下文。

`/tmp/i1-lg-triage-red.jsonl` 记录包失败，3 个顶层测试失败、含子测试共 5 个 fail 事件；直接原因是 `400 unsupported acceptance field: current_iteration_id`：

- `TestTriageIterationAcceptanceSharesActionAndTransaction`：accept / accept_and_execute 应共享原 T1 action identity，接受与安排同一事务；重放不重复事件或执行。
- `TestTriageIterationRecorderFailureKeepsPending`：注入 recorder 失败应整体回滚，仍为 pending。
- `TestTriageIterationTargetCancelledWhileWaiting`：等待期间目标被取消应返回冲突，不能接受后再单独安排。

这是一份开发中的 RED 证据，不是最终无法修复的阻断。**接手第一步先检查这些用例和入口是否已被旧会话修好；若已通过，从其最新验证记录继续，不重复实现。**

落实时沿用父 API 合同：

1. `TriageActionInput.fields.current_iteration_id` 仅 accept/accept_and_execute 支持；先按目标正式 status 校验，再在原 T1 事务完成接受与安排。
2. 保留 T1 的 request_id 和稳定 action identity；迭代事件关联既有 action，不新增第二份 operation/execution 身份。
3. 执行显式意图沿用 T1 outbox，重放只执行一次；普通 accept 不产生执行。
4. 保留 T1 设置、成员和 I1/引用的既定锁序、重试预算、CAS；借用 membership helper，不嵌套开事务或重试。
5. 目标取消/版本变化、权限变化、recorder 失败时联合回滚；准入拒绝保留 `409 triage_review_required` 合同。
6. `iteration_assignment` 可选能力只有完整接入并验证后才真实声明；不能提前给客户端 false success。

## 6. 最新验证证据与限制

下表是本次从现有日志读取的结果，本交接没有重新运行它们。计数包含子测试且不同选择有重叠，不能相加当作唯一测试总数。日志文件名带 green/final 也不构成通过证明；以解析到的包结束结果为准。

| 日志 | 本次读取结果 | 用途 |
| --- | --- | --- |
| `/tmp/i1-lg-services-accepted.log` | 19 pass；service 包通过 | LG 服务、时钟、完整集合和既有执行保持 |
| `/tmp/i1-hg-final-race.jsonl` | 84 pass；54 顶层；两个包通过 | HG handler/domain 定向回归 |
| `/tmp/i1-hg-domain.jsonl` | 103 pass；34 顶层；包通过 | 完整 iteration 领域选择 |
| `/tmp/i1-lg-http-green2.jsonl` | 2 顶层 pass；包通过 | 当前手动流程与 HTTP 参数/开关检查 |
| `/tmp/i1-lg-assignment-race.log` | 55 pass；包通过 | 普通任务归属与相关写入回归 |
| `/tmp/i1-lg-assignment-regression.log` | 77 pass；包通过 | 关联授权等既有 handler 回归 |
| `/tmp/i1-lg-assignment-service-regression.log` | 3 pass；包通过 | 创建服务回归 |
| `/tmp/i1-lg-review-green.log` | 8 pass；包通过 | 并发/撤权/引用可用性审查修复 |
| `/tmp/i1-lg-hg-review-final.jsonl` | 58 pass；39 顶层；三个包通过 | LG/HG 审查后联合选择 |
| `/tmp/i1-lg-triage-red.jsonl` | 3 顶层失败，含子测试 5 fail；包失败 | 当前 T1 未完成接入的 RED |
| `/tmp/i1-lg-hg-sqlc4.log` | 原会话命令退出 0 | 最近 sqlc 生成；不能代替二次生成无漂移检查 |

HG verification 尾部仍写“green pending”，LG verification 仍有早于 HTTP/普通 Issue 接入的待办。这些文档落后于上表新日志；接手者核对源码后应补齐正式记录，而不是重复修复已经有 green 证据的问题。

审查修复的具体范围：保留协调人引用锁的 PostgreSQL 临时错误，让原事务重试器处理；RR preview 用有限次新事务重试并重新鉴权；agent/squad/leader 归档后的实时可用性进入预览比较，不能让旧预览继续提交。当前 green 记录覆盖这些问题，后续共享 SQL/事务改动仍应回归。

基础 FG 的 lint/typecheck、10,054 TS 用例、Go build/vet 和容量记录只证明当时已提交的基础。LG 的 1,000 项完整预览/安排/开始和 2,001 项拒绝是局部容量观察；当前操作上限为 2,000。它们不证明生命周期全链 P95、关闭性能或生产 SLA。完整 Go 仓库、Web/Electron E2E、远端 CI、实际发布未在本轮验证。

`/tmp` 日志可能丢失。JSON 保留其摘要、修改时间和哈希；丢失或源码已变时按对应阶段重跑，不把摘要当作当前版本通过证明。

## 7. 后续实施顺序

1. **接管并刷新：** 核对原会话是否仍在写、最新提交和未提交差异，识别它在本快照之后完成的工作；保持其实现和测试。
2. **完成 T1 联合事务：** 从现有 RED/新结果继续，覆盖成功、重放、执行一次、失败回滚、目标竞争和当前授权。
3. **签收 LG/HG：** 按 API/测试矩阵逐项核对列表/详情/预览/操作/普通 Issue/T1 的真实入口，补漏后统一 sqlc、路由和事件合同，执行受影响回归和只读审查；更新两条 verification 和里程碑。服务单独通过不能跳过集成签收。
4. **独立提交验证完的切片：** 明确区分接手的 I1 文件和其他脏文件，按项目 Lore 协议记录约束、已测和未测；不盲目 `git add -A`。用户本轮未请求推送、合并或部署。
5. **LG+HG 通过后推进 CG：** end/active cancel/handoff/disable；最终范围、快照、去向、累计结转、operation 和 outbox 同事务。验证响应丢失、重放、撤权后不再投递、关闭后历史不漂移及全空间集合不截断。
6. **UG 与最终 FCG/VG：** 客户端可围绕稳定 API 分段开发；完整 Web/Desktop 闭环须等 CG。完成 29 项 ITR-AC、工程矩阵、必要性能和恢复验证后才能宣称 I1 完成。发布启用仍需后续明确指令。

## 8. 不可破坏的合同

- 迭代管理动作不得启动、停止、重启既有 agent task；实际 execution_started 只由真实成功的执行开始记账。
- recorder/membership 等 helper 借用调用者事务，不自行提交、广播或开启重试；每次重试重新检查当前权限。
- 保留 P1 原有 RR/RC 差异、外层重试预算、CAS、NOWAIT、附件原子性和 T1 锁序；不能为统一代码而改变这些合同。
- 开始时冻结完整 O 和展示摘要；按 start event sequence 排除同时间戳的计划期变化。复用现有统计/时区算法，不另造公式。
- A–J 标准结果：original 8、current 9、effective 8、completed 5、original_completed 4；比例 62.5% / 50%。删除任务仍保留冻结标识/标题。
- closed 读取依赖合法的持久快照，不用 live JOIN 修补损坏或改写旧统计；关闭后元数据审计不得修改冻结范围/图表。
- 全集预览、开始、结转和禁用不按页截断；超过上限 413 且零变化。纯执行进度不应无故使预览失效。
- 复用 P1 的 planning_timezone；无新依赖、无新增 FK；索引仍用独立单语句 `CREATE INDEX CONCURRENTLY` 迁移。
- 默认测试通过 agent CLI guard，不触发真实用户 agent CLI；维持 `iterations_i1` 发布开关关闭。

## 9. 接手与验证命令

先执行只读检查：

```sh
git status --short --branch
git log -8 --oneline
python3 .trellis/scripts/task.py current --source
python3 .trellis/scripts/task.py list
git diff --stat
```

不能把当前 `.env`、`make up --ephemeral` 或旧测试库名称当作隔离证明。先用 `make status` / `make list` 核对已有环境；需要 DB 测试时创建并迁移新会话独占的本地测试库，显式设置 `DATABASE_URL`。旧会话用过 `multica_i1_w13_20261006`、`multica_i1_lg_20261006d`、`multica_i1_hg_20261006d`；不得在它仍运行时复用或删除这些库。

确认写入所有权和隔离数据库后，按实际变更定向执行：

```sh
# DATABASE_URL 必须已经指向本次独占且迁移完成的测试库。
bash scripts/go-test-with-agent-cli-guard.sh -- go -C server test -race -p 2 -parallel 2 ./internal/handler -run '^TestTriageIteration' -count=1
bash scripts/go-test-with-agent-cli-guard.sh -- go -C server test -race -p 2 -parallel 2 ./internal/service ./internal/handler ./internal/iteration -run '^(TestIterationLifecycle|TestIterationHistory|TestIssueIterationAssignment|TestTriageIteration|TestHistorySnapshot)' -count=1
go -C server build -p 2 ./...
go -C server vet -p 2 ./internal/iteration ./internal/service ./internal/handler
git diff --check
```

SQL 变化由一个负责人运行 `make sqlc`，记录生成目录哈希，再运行一次验证无漂移；第二次哈希稳定才是生成稳定证据。当前 generated 中有新增未跟踪文件，不能仅用 `git diff` 忽略它们。

汇合时按仓库要求运行 lint、typecheck、测试和静态检查，采用有限并发；若涉及 Mobile 则另跑其本包要求，根检查排除了 Mobile。完整 Go/浏览器/Electron、迁移和容量验收按 VG 计划推进，不能把上述定向命令当全量完成。

## 10. 新会话引导语

```text
继续开发 Multica 的「迭代管理 I1」，按最新交接接续实施。

工作目录：/Volumes/artisan/code/2026/multica-0.4.37
开发分支：codex/projects-p1
交接时 HEAD：da6c0fc65306279a1a62b243a590ea87aa956710

先读 AGENTS.md、CLAUDE.md、.trellis/workflow.md，然后完整阅读：
.trellis/tasks/10-05-iterations-i1/handoff-2026-10-06-lg-hg.md
以及同目录 handoff-2026-10-06-lg-hg-evidence.json。

这份交接生成时，原开发会话 01a10f96-64ce-7463-b2da-646c7cbe5003 仍在写同一工作区。先核对它是否仍运行、最新 git 状态和验证记录，确认写入所有权已移交后再修改共享文件。保留全部已有改动，后续新证据优先，不重做已完成切片。

FG 已通过，不重做 S0–S2。LG/HG 主要实现和多组回归已通过，但完整集成尚未签收。交接时最新切片是 T1 接受/接受并执行与迭代安排的联合事务，已有 triage_iteration_assignment_test.go 和 /tmp/i1-lg-triage-red.jsonl；先检查旧会话是否已经修好，再接着完成。随后核对列表等 API 缺口、完成 LG/HG 集成验收，再按既有计划推进 CG、UG、FCG/VG。

继续保持发布开关关闭；不推送、合并或部署。保留 P1/T1 锁序、CAS、NOWAIT、原有事务与重试预算；迭代管理不得启动或停止任务执行。使用独占本地测试库和 agent CLI guard，记录真实红绿与剩余门槛。

这是继续实施请求，不是重新规划。无需重复确认已确定的范围，按现有任务树持续推进并及时记录进度和验证结果。
```
