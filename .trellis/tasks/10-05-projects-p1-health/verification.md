# Health 实施与 HG 验证

2026-10-05。前置 FG：`c4250df70`。本记录证明 health 局部实现和回归，不代表 P1 全部完成或已部署。

## 已实现

- `server/internal/projecthealth/`：单一 `Collect(ctx,q,project,now)`、纯 `Compute`、非递归 `StatisticsSnapshot`、精确风险 ID 集合和 canonical SHA256；接受调用者同一 RR qtx，不另取 pool、不建第二事务。
- `CollectProjectCounts` / `CollectCounts` 与概览共用 `countScope` 和状态类别解析。列表批量只查一次 catalog 和一次 project/status 聚合；空项目明确零，未知状态保留 N/U 并 complete=false；不重新定义旧 done_count。
- `handler/project_health.go`：授权工作空间/member fence/project shared lock 后读取概览、四风险精确分页；cursor 绑定 workspace/project/signal/version，版本变化返回新第一页，个人过滤不能改变集合。
- 正式集合由 foundation 已整合的 `project_health.sql` 固定 workspace + actual project + `not_required/accepted`；窄输入不载入 issue 正文，完整 Issue 只按本次风险页的 ID 在同一 qtx 读取。
- 含 archived 的自定义状态目录、member.user_id、同空间未归档 agent、未归档 squad + 有效 leader；私人 agent 是否可调用不参与“未分配”。运行环境按实际 agent/runtime 和完整 agent roster 单列，离线仍已分配。
- D 来自显式规划时区／缺省 UTC；due<D；日历七日涵盖春秋 DST；无发布时使用进入 in_progress 的时刻判断 stale，但 progress_age_days 保留 null。项目结束状态不抹去任务风险，进展更正不重置首发时刻。
- 验收摘要从稳定 published_at/id 排序取最近记录与当前描述记录，返回 current_revision；不偏爱 passed，作者使用 progress 共用的最小历史署名 helper，离开／删除不丢身份。
- 输入不可用：实际 PostgreSQL 的某统计输入错误在已授权后回滚，overview 返回 200 / complete=false / health=unavailable / 未知 counts=null；身份、连接或数据库整体不可用 503。精确 risk 和附统计发布仍拒绝不完整结果。`UnavailableError.Unwrap` 保留 PG 错误供 shared whole-tx retry。

## 测试过程与结果

所有下列实际数据库验收均使用本 worktree 的 `.env.worktree` 后覆盖 `.omx/projects-p1-test-env/health.env`（父任务新建的 health 独立数据库，已应用全部迁移）。命令不打印连接串，不运行真实 agent CLI。

```sh
set -a
source ../.env.worktree
source ../.omx/projects-p1-test-env/health.env
set +a
../scripts/go-test-with-agent-cli-guard.sh go test -race ./internal/projecthealth ./internal/handler -run '^TestProjectHealth|TestGolden|TestRiskOverlap|TestCalendarDays|TestDigest|TestOfflineReferences|TestCanonicalJSON|TestTimezoneDay|TestBatchScope|^TestTriageBoundaryFormalCollectionsAndExplicitSearch$|^TestTriageOrdinaryAcceptanceAdmitsProjectWithoutExecutionOrInheritance$|^TestCustomTerminalStatusCountsAsTerminalInSQL$' -count=1 -v
```

结果：exit 0；projecthealth 1.447s、handler 3.300s；8 个纯逻辑顶层测试、10 个 health 真实 DB 顶层测试和 3 个既有 T1/custom 状态兼容顶层测试全部通过，含 DST 与旧 custom 状态子场景；race 未报告竞争，CLI guard 未报告真实 agent 调用。

最后追加“历史作者已删除”断言后独立复跑：

```sh
../scripts/go-test-with-agent-cli-guard.sh go test ./internal/handler -run '^TestProjectHealthAcceptanceSelection' -count=1 -v
```

结果：exit 0，0.964s。该测试已同时覆盖 departed、deleted、最近旧描述验收、当前描述最新 failed、更正修订不改变首发顺序和统计 version。

静态检查：`go vet ./internal/projecthealth ./internal/handler` exit 0；`git diff --check` exit 0；新增 Go 文件均 gofmt。

RED 证据：先加入纯逻辑和真实 DB 测试时分别因所需 Input/Compute、Collect/overview/risk 未实现而编译失败；实现后通过。唯一 API 合同自审发现输入失败应为 200 不完整结果时，先改真实 PostgreSQL search_path 缺 catalog 测试，观察 expected 200 / got 503 的实际失败，再实现 typed unavailable 投影并复跑通过。首轮 custom fixture 缺必填 color 导致 setup 失败，只修正 fixture，未放宽产品规则。早期一次 `go test -run '^$'` 编译探针未显式加载 health 环境，执行了 TestMain、没有测试；不计入验收证据，后续编译检查均用 `go test -c`。

## 覆盖与边界

| 要求 | 证据 |
| --- | --- |
| N10/F6/C2/U2/.8、empty null、取消不等于验收 | `TestGoldenCompletionCancellationAndEmpty`、`TestProjectHealthFormalGoldenScope`；旧 DTO 8 闭合及新分项逐项断言 |
| T1 正式集合／真实项目／跨租户／父子 | formal golden、boundary、risk parent/child；两个既有 T1 正式集合／普通接受无执行测试 |
| B/O/A/R 交集去重、今天不逾期、精确分页／个人过滤隔离 | risk overlap、cursor scope/limit、纯 risk/date 测试 |
| archived custom／unknown／实际查询失败不伪造零 | custom unknown 与 PostgreSQL 缺 catalog search_path 测试；未知下钻 503 |
| 有效引用与离线分离、小队 leader／完整 agent roster | assignee lifecycle DB 测试、runtime 恢复改变 version、纯 offline 测试 |
| UTC/Shanghai/DST/七日/项目状态独立 | calendar/semantic input 纯逻辑测试 |
| canonical 不含采集时刻、顺序稳定、同数量成员改变可见 | digest/input-change/canonical precision；存储快照 JSON 不含另一个 snapshot 或验收对象 |
| 同一个 RR qtx，无两次时点拼接 | 第二连接改变任务后，当前事务第二次 Collect 保持旧 version，下一事务看到新状态/version |
| 最近和当前描述验收各自选取；修订署名 | acceptance selection 的真实 DB 测试 |
| old/new 分项来源一致、空项目、单个未知不污染其他项目 | batch-vs-overview、legacy additive fields 实际查询断言 |

仍由父任务／其他 lane 验证：真实 HTTP router 和 capability 的跨端端到端集成、附统计 preview/create/correct 的完整发布事务、全套后端回归、500 项目／10,000 任务性能与活跃分页压力。此处未宣称这些项目已经通过。采集当前复用 workspace 级 reference/catalog 查询；查询数固定但大 workspace 的输入量需按既定性能计划实测，未提前添加索引或依赖。
