# Autopilot runtime teardown race 独立审查

日期：2026-11-26。结论：**最终5文件修改可提交并进入新冻结SHA的CI/构建，未发现阻断提交的代码问题。** 这不是最终Linux全包CI或发布通过结论。

审查只读取源码、原始日志和回执，执行格式/摘要/事件集合核对，没有重跑重型DB测试或修改产品。

## 根因与改动边界

原CI45656329181的head为c920c5e5b0354258ad0a37d500655a6856d5ed5b。直接读取保留失败日志确认 `TestProjectAutopilotRunOnlyRacesRealRuntimeTeardown/with_project` 报 `check agent readiness: no rows in result set`。修复基线为b7563d8bbeb3abe0f73151a3d17f0414cbc4d89f；独立Git比较二者整个server树相同，不能混写两个SHA。

`dispatchRunOnlyOnce`先GetAgent/resolve leader，再独立读取GetAgentRuntime；另一事务可以在两读间完成TeardownRuntime和runtime DELETE。AgentReadiness只会从其runtime lookup返回错误，RuntimeLookup保留pgx原错误，故此次 `errors.Is(err, pgx.ErrNoRows)` 精确指向此读取缺失，不是任意数据库故障。

新增5行只把该pre-transaction缺失转换成已有errDispatchSkipped/target_unavailable。普通连接/查询/取消错误仍走原fmt.Errorf并保留%w链；共享AgentReadiness/RuntimeLookup、create_issue、metrics分类与其他调用者均未改。没有新增fallback或权限缓存。

该分支在创建事务和入队前返回，故不会绕过成功路径的runtime/agent/project锁、assignee和binding重检、squad状态、effective principal/member/grant锁及最终invocation授权。NOWAIT查询、IsProjectAssociationBusy、原完整事务重试次数与10/20/30ms等待均未改。已提交后teardown取消既有任务的路径也未改。

## 回归真实性与错误对照

新DBTX夹具只拦截生成SQL的GetAgentRuntime QueryRow边界，其他SQL仍用真实pool。正例在已返回agent之后，运行真实TeardownRuntime、DELETE并提交，再执行原GetAgentRuntime查询；没有用fake ErrNoRows替代删除。断言恰好一次初始runtime读取、runtime真实缺失、agent仍保留但解绑、typed reason、handleDispatchSkip持久化skipped及任务队列为空。固定边界不依赖sleep或竞争概率。

负例只在同一边界注入普通lookupFailure，要求errors.Is保留原错误、不得As成errDispatchSkipped、不得持久化为skip且没有任务。原并发teardown测试（有/无project）、两个提交顺序、关联锁、runtime状态/agent权限/binding/member撤销回归均完整保留，未删除断言/skip/扩大timeout。

红测原生产文件哈希与Git基线独立匹配；同一新测试在修复前只有runtime_deleted子例及父例失败，错误与CI相同，lookup_failure控制通过。生产5行修改后两子例及父例通过。新测试证明行为差异，不是复制实现的布尔断言。

## 独立证据复核

从原始JSONL逐条解析，而非只读verification汇总：

| 运行 | 真实命名run/pass/fail/skip | 包级结果 |
| --- | --- | --- |
| red-regression | 3 / 1 / 2 / 0 | exit1；1.394s，保留预期失败 |
| green-regression | 3 / 3 / 0 / 0 | exit0；1.297s |
| normal count20 | 440 / 440 / 0 / 0 | exit0；48.571s |
| race count20 | 440 / 440 / 0 / 0 | exit0；58.783s |

normal/race各6个顶层、22个唯一命名测试，每个恰好执行20次；run与pass计数逐名一致，无unfinished或漏项。父/子例都计入440，不能宣称440个不同测试。四份log摘要与companion receipt一致。记录的guard、-parallel=2、-count=20和10m预算没有削弱原场景。

`go vet ./internal/service`保留命令、空日志与exit0；本审查独立 `gofmt -l`两Go文件为空、`git diff --check`通过。Go1.27.0 Darwin/arm64配任务独占Linux/amd64 PostgreSQL，不能替代最终Linux race及漏洞门槛。未运行TypeScript typecheck，此server增量不涉及TS。

5个最终文件逐哈希匹配 `dispatch-race-verification.json`。其中生产Go/新测试同时匹配stress-source-hashes，后续短暂诊断输出实验已经撤销，未将其另存日志当接受证据。spec/builtin同步文字未声称被Go行为断言验证。

## 文档精确性修正

原新增builtin句子“runtime disappears during dispatch ... no task”过宽，可能覆盖事务提交后正常teardown。本评审提出限定，实施者已改为pre-transaction readiness check及“that dispatch enqueues no task”。最终SKILL、source-map和iterations规范都定位此读间隙，并保留ordinary-error/final-fence约束；已复核，不再有此文案问题。

## 资源与后续

已查看owner/精确容器ID及匿名卷清理记录。独立只读查询该精确容器和卷均为空；未触碰其他资源。原红测、失败CI及平台限制保留。新候选改变产品源码与内置文档，正式server/CLI/Desktop构件必须从新SHA重建；等待新的main CI/live scan和Windows原生结果。

## 来源摘要

| 文件 | SHA-256 |
| --- | --- |
| `server/internal/service/autopilot.go` | `26626c67bf6247b7b1ee0cf7228d307ace488caf746a95c403f4a2f02a2e364f` |
| `server/internal/service/project_association_dispatch_test.go` | `c1fdad8f6d64cd47c90d3860eeed64e60c85144a155d0b4862787282ebdcd5b6` |
| `server/internal/service/builtin_skills/multica-autopilots/SKILL.md` | `0f07a402d5bcbc186bf9470d98321e0d07d5bbf6cdc336dfd1905c88fdd093b4` |
| `server/internal/service/builtin_skills/multica-autopilots/references/autopilots-source-map.md` | `845b8262b7b7bd9b1254d0430f4115d9935ee42ea06187eda91f82f4ae10d795` |
| `.trellis/spec/server/iterations.md` | `0e0666795f9539873edbf2ff2420bc15116d065d67f161e669341bfc2fc60c11` |

原始证据目录：`/Volumes/artisan/code/2026/multica-0.4.37/.artifacts/release-v0.8.8/dispatch-race/`。命令、平台、逐名次数与日志哈希见本任务 `dispatch-race-verification.json`。

同批Windows两文件独立评审见 `windows-node-resolution-review.md`：提交可行，本机11项Node契约通过；PowerShell不可用，新增真实多PATH/反序/缺失fixture和Web启动仍需下一native run。
