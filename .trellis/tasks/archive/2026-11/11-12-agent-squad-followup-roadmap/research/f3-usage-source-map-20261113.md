# Research: F3 用量来源与计量证据

- Query: 追踪 producer → adapter → daemon/transport → usage writer → SQL → hourly/API，识别父预算可用事实与不可证明部分。
- Scope: internal；只读源码与静态测试断言。唯一输出为本文件，未执行产品/DB/Provider 测试、真实运行、策略操作、迁移或发布。
- Date: 2026-11-13。
- Active task: `.trellis/tasks/11-12-agent-squad-followup-roadmap`；读取当前 prd/requirements 与 `.omx/plans/agent-squad-next-f3-20261113.md`，未读取 implement/check JSONL。
- 当前决定：root 转达用户选择“长期绑定父事项”；本文仅为该设计提供来源证据，不构成 F3 实施授权，不恢复账单对账范围。
- 工作树身份：root 于 `2026-11-13T09:36:18Z` 观察 HEAD=`feb87aee75fa6b9dd051a4e35bf7df8b101edb75`，分支 `codex/cross-workspace-collaboration`；392 项 dirty、暂存为空、当时本任务目录干净。
- root 后续复核同 HEAD，本文主要 agent/daemon/handler/service/usage SQL/scheduler 源码无 dirty，仅另有 collaboration 测试并行改动。研究代理未运行任何 git 命令；最终相关 dirty 由 root 再核对，文件 SHA-256 见末尾。

## Findings

### 1. 可直接用于方案的结论

1. 普通 task 的持久用量是 `(task_id, provider, model)` 的可覆盖当前值，不是计费事件。重复上报替换，乱序旧值也能覆盖新值；无 claim/event/revision/source 身份。
2. `TaskService.CaptureTaskUsage` 只累计监控指标，真实 DB writer 在 handler；重复请求即使 DB 行不变，metrics 仍重复累计。
3. `cost_usd_ticks` 的列注释称“provider-reported”，但 Reasonix 的 `estimatedCost` 也进入此列；费用值存在不证明实报，更不证明覆盖本次全部调用。
4. adapter → daemon 存在有正费用但四类 token 全 0 时整行被跳过的路径；合法免费 0 与未报告也无法由普通 wire 区分。
5. 同次 daemon 执行的 fresh-session fallback 会把两次 usage 相加；一次有费用、一次缺费用时，合并后的正费用掩盖部分未知费用。
6. usage HTTP 上报只尝试一次，逐模型写失败仍可返回 200；与终态回调、持久 terminal recovery 不原子，不能把完成或取消当作结算完成。
7. hourly 是按首次 usage 入库时间分桶的当前投影，重算/删除/重新归属会改写历史显示；其 watermark/event_count 均不证明上游计量完整。
8. UI 静态价格、服务器 metrics 静态价格与本地自定义价格没有被用量记录引用的价格版本；它们只能提供显式标为估算的候选规则。

### 2. Files found：主链与职责

| 层 | 文件 / 符号 | 真实行为 |
| --- | --- | --- |
| 通用输出 | `server/pkg/agent/agent.go:184` `TokenUsage`；`:207` `Result` | 每 model 四类 int64 token、int64 cost；每次 Execute 返回一个 Result，usage map 以 model 为键。 |
| ACP | `server/pkg/agent/acp_usage.go:20`、`:55`、`:167` | 内部保留字段存在性，解析兼容字段、合并累计快照；导出 TokenUsage 时不携带存在性。 |
| ACP 收口 | `server/pkg/agent/hermes.go:1480`、`:2059`、`:2075` | terminal prompt 顶层 usage/_meta 与 usage_update 合并；输出只有数值。多种 ACP backend 复用此 client。 |
| 其他 producer | `server/pkg/agent/claude.go:702`、`codex.go:3602`、`reasonix.go:738` | 各自转换 CLI stream、app-server/本地 rollout、厂商状态事件，含不同累计/增量规则。 |
| daemon 组装 | `server/internal/daemon/daemon.go:8041` | model map → TaskUsageEntry，provider 取运行选择；零 token 跳过，不检查是否有正费用。 |
| daemon 上报 | `server/internal/daemon/daemon.go:5252`、`client.go:550` | runner 返回后，先尝试 usage，再处理失败/取消/完成；没有执行中持续 usage 上报。 |
| wire | `server/internal/daemon/types.go:365`、`server/internal/handler/daemon.go:4646` | 请求只有 provider/model/四类 token/cost，无 session、attempt、claim、source sequence、是否完整。 |
| transport | `server/internal/daemon/client.go:1151`、`:1166` | usage 走一次 HTTP POST；成功时 respBody=nil，正文丢弃，不读取逐项写入收据。 |
| RPC 边界 | `server/internal/handler/daemon_rpc.go:48` | WS RPC switch 仅处理 `tasks.claim`，没有 usage RPC；不能把 claim RPC 的恢复契约套在 usage 上。 |
| HTTP 路由 | `server/cmd/server/router.go:1590` | `POST /api/daemon/tasks/{taskId}/usage` → `ReportTaskUsage`。 |
| DB writer | `server/internal/handler/daemon.go:4671` | 验 task/workspace/managed runtime 后逐行 Upsert；失败 warn+continue，最后仍 `200 {status:ok}`。 |
| metrics | `server/internal/service/task.go:878`、`server/internal/metrics/business.go:585` | SQL 成功后按每次请求的整份 usage 增加 Prometheus counter；没有按 task 去重或冲正。 |
| 原始当前值 | `server/pkg/db/queries/task_usage.sql:1` | ON CONFLICT 覆盖 token/cost，刷新 updated_at，保留原 created_at。 |
| 聚合 | `server/migrations/213_task_usage_authoritative_cost.up.sql:77`、`272_rollup_task_usage_hourly_xact_lock.up.sql:38` | 基于脏键重算 hourly，事务 advisory lock 4246，按水位推进。 |
| 调度 | `server/internal/scheduler/jobs_task_usage.go:36` | 5 分钟 cadence/delay，latest_only，25 分钟超时、3 attempts；调用 SQL 并记录前后水位。 |
| API | `server/internal/handler/agent.go:741`、`daemon.go:5325`；`task_usage.sql:68` | 单次执行保留 model/nullable cost；issue summary 与 dashboard/hourly 返回合计及 uncosted token。 |
| 估价 | `packages/views/runtimes/utils.ts:212`、`:680`；`packages/core/runtimes/custom-pricing-store.ts:12` | 当前静态 USD/百万 token 费率与本地自定义费率，页面现算。 |

### 3. 字段合同、单位与丢失语义

| 字段 | 当前权威与单位 | 粒度 / 缺失 / 局限 |
| --- | --- | --- |
| input_tokens | adapter 对 producer 原值的归一化整数 token | 意图为非缓存 input；Codex 扣 cached input，ACP 仅能证明包含关系才扣除；并非所有 producer 的统一原生定义。 |
| output_tokens | producer 输出 token，经 adapter 处理 | Codex rollout fallback 将 output+reasoning_output 相加，RPC path 只取 output；无独立 reasoning 标记。 |
| cache_read_tokens | producer 缓存读 token | ACP 可接受多种 camel/snake 字段；缺字段最终成为 0，无法再区分未支持与确认为 0。 |
| cache_write_tokens | producer 缓存写 token | 无 cache TTL/请求级 tier；某些 producer 不提供；静态价格无法从合计恢复这些维度。 |
| cost_usd_ticks | `1 tick = 10^-10 USD`，Go/SQL int64/BIGINT | 普通 handler 仅正值有效，0/缺失/负值→NULL；列值还可能来自 provider estimate，不能仅因非 NULL 认定 actual。 |
| provider | daemon 当前 backend/provider 字符串；handler normalize；空则查询 runtime | 不保证等于实际计费上游；非空来值不在此与模型账单主体核对。旧值 fallback 查询失败可仍写空 provider。 |
| model | Result map key，多数来自 producer 或配置 fallback | 可为 `unknown`/generic alias；模型切换可能折叠为一个 key。没有 resolved SKU、定价档或精确模型版本证明。 |
| task_id | URL 指定并读取现存 agent_task_queue | task retry 可能新 ID，reclaim 可能同 ID；没有内层 launch/call 维度。父事项关系不随 usage 冻结。 |
| created_at | DB 首次 INSERT 的 `now()` | 是最早收到该 task/provider/model 的时间，不是 provider 实际发生时间或最后结算时间。 |
| updated_at | 每次 Upsert 的 `now()` | 是可变处理时间，不是 producer revision/严格单调 sequence；不能区分重复、纠正、旧报告。 |
| source / version | 普通 usage 类型/SQL 中没有 | metrics 的 source 是任务来源标签；不是 meter adapter/version、费率版或 provider 事件标识。 |
| uncosted_* | 聚合中 cost IS NULL 的行对应 token 合计 | 只按整行有无 cost 拆分；不能揭示部分有费用、部分未知的合并行，也不计完全未上报的执行。 |
| task_count / event_count | DISTINCT task / COUNT(*) 当前源行 | event_count 不是 producer 事件数；跨 model/hour 再求和的 task_count 不是独立执行数。 |

依据：`agent.go:184`、`acp_usage.go:177`、`codex.go:3618/3931`、`daemon.go:8043`、handler `daemon.go:4664/4694`、SQL `task_usage.sql:52`、migration213 `:134`。
HTTP 数字使用 Go int64，但 handler 未增加 token 非负/完整性/重复 model 等专用校验；不能把 JSON 可解码等同有效计量。
`TaskUsageData.CostUsdTicks *int64` 的读模型可以表示 0；普通写端却将 0 转 NULL，因此读端注释中的“免费 0”不证明 producer→存储已支持免费零价。

### 4. Producer 与 adapter：累计、增量和更正

| 路径 | 可证实转换 | 计量证据缺口 |
| --- | --- | --- |
| ACP 共用 | `acp_usage.go:55` 对 output/cache/cost 取 maxima；input 结合 total 证据选规范/含糊候选。`:117` 顶层 usage 优先，缺或 0 可用 _meta 补，费用取较大值。 | 可消除重复快照并接纳较丰富晚到值；降价/减少 token 的纠正不会生效。max 还可能拼出未在单条事件出现过的组合。 |
| ACP 归一化 | `acp_usage.go:250` 仅在 total=input+output 且 cached≤input 时 input 减 cached；`:266` 多别名可解析整数/数字字符串及 float 后转整数。 | 无 total 则不扣；不同版本字段定义、累计 reset、浮点溢出/取整合同未冻结，不能推广为所有 ACP 真实调用保证。 |
| Hermes/Grok 等 | `hermes.go:690/771/1480/2066` 将 prompt 与 update 合并，再按 effectiveModel 或 unknown 输出；`:2075` 只返回 TokenUsage。 | 原始帧、来源字段 presence、时序和范围全部丢失。Grok 传 ticks 并不能证明其他 ACP 提供相同权威。 |
| Reasonix | `reasonix.go:483` 没 ACP usage 才用 status fallback；`:738` 解析 schemaVersion=1/sequence；`:784` 忽略较小 sequence，等序可覆盖。`:776` 把 USD estimatedCost 四舍五入为 ticks。 | schemaVersion/sequence/estimated 标签不出 Result；已有任意 ACP usage 时 status 中费用不能补入。实报/估算被同列混合。 |
| Claude | `claude.go:271` resultUsage 非空就替换之前 usage；`:702` 优先 result.modelUsage，否则 result.usage+model/config。输出四类 token。 | 普通 adapter 的该转换不携带 cost、原始事件/调用 ID；terminal 总值覆盖 stream，不证明每模型子调用完整或 resumed session 范围。 |
| Codex RPC | `codex.go:3435/3602` 从 turn.usage 等提取后相加；`:3418` turnCompleted guard 防现代完成事件重复；input 减 cached。 | legacy task_complete `:3357` 自身未见相同 guard；无 cost，配置模型优先。不能统一假定所有 RPC 事件是同一种累计单位。 |
| Codex rollout fallback | `codex.go:1897/3668` RPC input/output 全 0 时扫描当前 thread 的本地 rollout；`:3859` 从 startTime 边界减以前 total，逐字段 reset 时把当前值作新 span。 | 精确 thread 绑定避免串入并发 subagent 文件，但不是 subagent 开销全覆盖证明；last_token_usage fallback 仅保留最后值，模型归到一个 key。 |
| Copilot | `copilot.go:60/75` fresh 可用 session.shutdown 总量，resume 则禁用该历史累计；优先 per-call usage，再 message output fallback。 | 现代 JSON 输出可不含这些事件（`:448` 告警）；不能从执行成功推导 usage 完整，且无费用来源标记。 |
| Cursor | `cursor.go:137/265/333/765` result 总量路径优先于 step；每次 result 对同 model 做加法，支持新旧字段名。 | 未见普通 Result 输出的累计基准/去重事件 ID；resumed session result 是否覆盖历史仍需版本绑定证据。 |
| Pi / OpenCode / OpenClaw | `pi.go:386` message usage 相加；`opencode.go:441` step_finish token 相加；`openclaw.go:509/530` stream 相加，无 stream 时用 final。 | 报告入口形态不同；OpenCode `:520/706` 读取 cost 只作“有产出”判断，未把该值折成 TokenUsage.CostUSDTicks。 |
| Kimi | `kimi.go:429/464/489` 注释记录 0.33.0 ACP 不含计费用量，改扫本地 wire；有 ACP token 则优先，否则 scan fallback。 | 文件扫描存在边界/完整性依赖，本轮未穷尽 wire 扫描器；不得由 context `used/size` 代替消耗 token。 |

这些是当前适配代码对输入的处理，不是本轮执行真实 CLI/Provider 后的验证。其它 backend 的专有事件与价格权威未逐个验证；不将同用 ACP client 等同相同 provider 计量保证。

### 5. Claim / retry / session 与上报失败

- 同 task 的 fresh fallback：`daemon.go:8005/8020/8415/9042`。两份非空 usage 按 model 相加；无 launch/call 身份，也无 cost 覆盖率。
- 具体反例：first=`100 tokens, cost缺失`，retry=`50 tokens, cost=30 ticks` → `150 tokens,30 ticks`。SQL 认为全行 costed，uncosted=0；实际未知的 first 未被保留。
- `reconcileFreshRetryResult:8430` 在 retryErr 非空时仅保留 firstUsage；虽然注释说每分支合并，不能据此推断启动失败/传输不明的 retry 没有花费。
- SQL `agent.sql:546/620` 系统 retry 新建 task，继承 issue/runtime、增加 attempt，保存 parent/retry lineage；原 task usage 与新 task usage 可各自保留。issue summary 会合计，不能再加父级 summary。
- `agent.sql:946` claim-finalize 失败以 task/runtime/dispatched_at/claim_generation CAS 重新入队；usage wire 没相同 generation，不能拒绝某一旧 claim 的 usage 并保留其归属。
- helper `handler/daemon.go:151` 校验现存 task、workspace 与 managed/runtime 权限，但 `ReportTaskUsage` 没 usage event receipt、claim CAS 或终态过滤。晚到只要该访问校验仍通过即可替换。
- provider/model 改名后上报会写另一行，不清理旧 key；没有跨 key 的“这次是纠正旧记录”关系，不能据此判断新开销还是重复归类。
- `daemon.go:5258` 使用外层 ctx 上报，普通执行取消后的结果仍有机会到达；daemon 关闭/崩溃/网络失败没有同等送达保证。
- `client.go:550/1151/1199` 无 usage 专用 retry/持久 outbox/原 key 查询，complete/fail 的 retry 在 `:547/586/1118`。不能将后者验证当作用量恢复证明。
- `handler/daemon.go:4708` 多行非事务 Upsert；第一个成功、第二个失败仍返回 OK，无 accepted/rejected 明细；client `:1210` 不检查成功正文。
- 新报告未带 cost 会把已有 cost 覆为 NULL；旧报告可向下覆盖 token；相同 payload 重发无业务去重记录，但刷新 updated_at 并重复触发 metrics。
- 进程退出/请求取消/任务 failed/cancelled 与“所有 usage 已确认且覆盖完整”是独立命题。现存链没有 unknown settlement 状态可供父预算直接消费。

### 6. SQL、hourly 与 API 的实际口径

- `032_task_usage.up.sql:1` 建立原行，四类 token 默认 0、created_at 默认 now、三元组 UNIQUE；其旧 FK/index 写法不作为 F3 新迁移先例，当前 CLAUDE 禁止新 FK 且要求 CONCURRENTLY。
- `task_usage.sql:1` 只有覆盖当前值；`:25/52` 通过现存 task.issue_id 读全部原行，未冻结预算根、父事项、原 project、claim 或实际发生时间。
- `101_task_usage_hourly_schema.up.sql:37` hourly grain=`UTC hour/workspace/runtime/agent/project/provider/model`；不是 task/call/event grain。
- `213_task_usage_authoritative_cost.up.sql:94/107` 用 usage.updated_at 找脏来源，用 created_at 决定目标小时；更正旧行重新计算旧小时，不生成独立冲正记录。
- `213:134` 对脏键重扫完整当前源行并 SUM；`:145/146` task_count 是 distinct task，event_count 是行数；`:179/195` 覆盖 bucket、删除空 bucket。
- `213:139` cost SUM(NULL)→0；uncosted 只涵盖 NULL-cost 行；老 hourly uncosted 列未重算时为 NULL，query `task_usage.sql:95` COALESCE 为该 bucket 全部 token。
- `102_task_usage_hourly_pipeline.up.sql:50/142/176/233` 为 task runtime/issue 改变、issue 删除/改 project、usage 删除排入旧新脏键；这些是可变归属修复，不是历史事件存证。
- `272:48/67` 事务 lock，watermark 每次最多前进一天，到 now−5m；scheduler `jobs_task_usage.go:36/58` 每5分钟尝试，拿不到内部锁也可成功且0行。
- 水位仅表明 DB 当前值扫描进度。慢提交/发生时间与接收时间差、上游未发/丢发不由水位证明；不作为 budget completeness watermark。
- `task_usage.sql:52` issue summary 无原行时 COALESCE 成0、task_count 仅含有 usage 的 task；它没有 eligible task 数、缺失 usage 数或父子递归计量边界。
- `handler/agent.go:741` 单 task usage保留每 model cost pointer；`task_usage_response_test.go:108/122/140` 明确无费用为 nil、无用量连 usage key 都省略。
- `handler/daemon.go:5326` GetIssueUsage 用当前 issue 权限后返回合计；dashboard `task_usage.sql:68/107` 从 hourly 按 viewer timezone/当前过滤读数。API 不返回原始帧、来源版本、逐调用覆盖率或更正链。
- `task_usage.sql:115` 已注明跨 bucket 累加 task_count 可重复。父预算不得合并“子 task行 + issue summary + hourly”三条路径，否则同消费多记。
- 只凭源码不能证明生产 DB 已应用全部 migration，也不能证明删除后所有链的实际保留时间；本轮没有访问 DB 验证。

### 7. 价格来源与版本：仅可作为候选估算

- 前端 `packages/views/runtimes/utils.ts:175/212` 的 MODEL_PRICING 是代码常量，单位 USD/百万 token，四档 input/output/cacheRead/cacheWrite。
- 注释列 Anthropic/OpenAI/DeepSeek/Moonshot/Zhipu/xAI 官方价格 URL，`:216` 明说不支持未来生效日期；不能把注释日期或当前代码提交当成一条 usage 已绑定的 price_version。
- `resolvePricing:427` 先查内置模型候选，再查 custom；会做 provider限定/日期后缀/模型别名/context tag 处理。匹配规则与命中 key 未随用量持久保存。
- `packages/core/runtimes/custom-pricing-store.ts:12/29/46` 为本地全局 Zustand 持久 store，四个 number，key 为模型字符串；没有 workspace 级审批、价格版本、生效日期或服务端账本绑定。
- `utils.ts:641/680` 输出 `ticks/1e10 + estimate(uncosted)`；缺价格只返回已有 ticks。缺 split 且有正 cost 时不估剩余 token，故上述部分费用会被隐去。
- `collectUnmappedModels:587` 可提示无法估价的模型，但这是页面诊断，不能保证 F3 API 不把未知显示为0。
- 服务器 `server/internal/metrics/pricing.go:14/23/225` 另有 float64 ModelPrice map 与 alias rules，无价版字段；`business.go:591/615/621` 当前费率估算，正 ticks 则覆盖总费用并按估算比例分摊 token 类别。
- metrics counter 每成功上报累加一次；不是 DB 原始行的幂等镜像，跨上报重放不能用它校验预算 exactly-once。
- F3 如复用费率知识，需冻结“费率表版本/命中SKU/来源/单位/舍入/生效范围/算法版本”，服务端可复算；不直接依赖 Views 的本地浮点计算或用户本机 custom store。
- JSON/API/UI 使用 number 的既有链不提供全 int64 范围精度保证；F3 应明确安全整数上限或十进制字符串编码并给溢出拒绝合同。

### 8. 已有静态断言与 F3 最小补证（提案）

现有 `handler/task_usage_response_test.go:21` 静态断言两 model 原值/nullable cost/无 usage 字段；它用直接 INSERT，未证明真实 producer→adapter→report 送达与权威。
`agent/acp_usage_test.go`、`codex_test.go:606/973/1005`、`copilot_test.go:673/709`、`daemon/daemon_test.go:1615` 是相关现存测试入口；仅作为静态查阅证据，未记本轮运行通过。

| 提案 | 可复用部分 | 首包必须补的最小证据 |
| --- | --- | --- |
| U1 来源信封 | 四类 token、ticks 单位、adapter parser | producer kind/version、model原值/解析值、event/claim/launch/span ID、payload digest、observed/occurred 时间、partial/complete，费用 quality 独立于值。 |
| U2 金额可信度 | 现有正 ticks、NULL cost 区分 | reported/estimated/unknown/known-zero 分开；Reasonix估算不能升级 actual；正cost+部分unknown、cost-only、token-only均有独立fixture。 |
| U3 幂等与更正 | 原 task/retry lineage | 同身份相同payload返回原收据；同身份不同payload拒绝；新revision显式更正，接受降低值而不静默当reset；乱序/重复不二次扣减。 |
| U4 累计边界 | Codex startTime基准、Copilot resume分支 | 每provider版本证明 cumulative还是delta，重置/重开新epoch；同claim多调用、内层fallback、跨claim/新task不串账，原始来源不可只留聚合。 |
| U5 可靠上报 | daemon final drain、现有HTTP身份验证 | durable original event spool/ACK/recovery，部分批失败不报整体成功；提交未知按原key恢复；取消与退出不自动释放未知支出。 |
| U6 归属与投影 | task.issue_id、retry_of_task_id/parent_task_id | 长期父事项根绑定单独冻结；父层只读归集不产生第二笔消费；移动/删除/重试不重归属逃逸，当前summary不充当根历史。 |
| U7 可复算估算 | 静态rate知识、model resolver | 服务端整数/有理数计算及固定版本；未知rate不折0；历史使用原价版，变价新版本不改旧计算。 |
| U8 证据分层 | 原行读API与hourly投影 | 原记录、budget accounting、展示聚合各自唯一消费边界；用手算fixture核对同预算根，不以调用同一helper的双边比较当oracle。 |

建议 observe 首包接纳“已知下界 + 未知执行/部分费用”并解释来源；先证明一个受控 provider 输入路径可重放与幂等，再扩大 provider 覆盖。当前数据只能做 legacy observation，不能回填成已证明完整的 actual。
enforce 的事务预留/释放由预算设计另定；本文件不把 usage pipeline 的最后一行/最后一次 HTTP200解释为可以释放的证明。

## Related specs and external references

- `.trellis/workflow.md` §1.2：研究入 task/research；`CLAUDE.md`：包边界、无新FK/并发索引、未运行不得声称通过。
- `.trellis/spec/server/platform-observability.md:5/20`：missing usage 是 unknown、token不是账单，多provider/model不能把 task重复计数。
- `.trellis/spec/server/task-api-capabilities.md`：当前 task/runtime/claim 权限核验；授权 scope 不等于真实 provider 计量完整。
- `.trellis/spec/guides/cross-layer-thinking-guide.md`：事件边界应有单一类型/解析/版本/来源，而非每个消费者自行猜测。
- 本文未外部联网验证费率；只记录源码引用的官方URL：`https://platform.claude.com/docs/en/about-claude/pricing`、`https://openai.com/api/pricing`、`https://docs.x.ai/developers/pricing`（完整列表在 utils.ts:179）。
- 版本证据限本仓库注释与parser：Codex rollout注释0.144.4（codex.go:3663）、metrics注释0.144.1、Kimi注释0.33.0、Reasonix schemaVersion=1。不是当前已安装CLI或真实提供方版本的证明。

## Caveats / Not Found

- 未证明各 provider 实际重复/累计/reset/费用权威，尤其 CLI 内建 subagents/helper 模型是否被父用量包含；不能同时累加父 summary 与子调用而声称完整。
- 未找到普通 TaskUsage wire/SQL 的 source/version、幂等事件、纠正关系、费用完整性/零价存在性、持久 usage ACK 或 price_version。
- 未穷尽所有 backend 专有 parser、Kimi wire 扫描和 collaboration 独立 usage 协议；它们不是普通 task_usage 已完整覆盖的证据。
- 未执行产品测试、SQL、Provider、真实任务、POLA-1、共享策略、发布/部署/Hosted CI。已有 tests 的名称/断言只代表代码中写了什么。
- migration定义与生产应用状态、慢提交跨水位的最终行为、权限撤销后合法旧usage的结算接纳规则仍需独立设计/授权后的确定性证据。
- researcher 只写本文件，不修改 code/spec/jsonl/task状态；相关 dirty 和提交祖先由 root 最终确认。

## SHA-256：本轮读取的关键文件

只读快照时间：`2026-11-13T10:38:56.450472+00:00`；从 .git/HEAD 与 ref 文本解析的 HEAD=`feb87aee75fa6b9dd051a4e35bf7df8b101edb75`，与 root 观察一致；未运行 git 命令。
规划文件可能被 root 并行增补，以下是快照时的工作树内容身份；不声称本轮全部读取发生在同一个不可变 cut。

| 路径 | SHA-256 |
| --- | --- |
| `AGENTS.md` | `403ec642d0625862bd042fa3293be4c46bb308b88159cbaa75559cbf1f389289` |
| `CLAUDE.md` | `ba958587589c26338bfb9547d745653a34239ee0524b8cb10a89c49056b382ed` |
| `.trellis/workflow.md` | `eb4508a4f6792f3d53705c1f9e7d21807dabd774543bd7b01f56372a704f6ad0` |
| `.trellis/tasks/11-12-agent-squad-followup-roadmap/prd.md` | `fe506c1b2752c5da6151754fbde03469cd93ec0a38a6935e2da88ab7eeb93820` |
| `.trellis/tasks/11-12-agent-squad-followup-roadmap/requirements.md` | `de66193eaa81c662525201b69104798904bb78a003e30af59be5112a1cf6bf2a` |
| `.omx/plans/agent-squad-next-f3-20261113.md` | `5cf9b8d4131c0403585295a89c7daefd595459d565bc6b40a0b629e3a85b8b87` |
| `.trellis/spec/server/platform-observability.md` | `5c088fa517ce5f221520a82ec3c099be8b9fe7308363547da3e57b84224e804e` |
| `.trellis/spec/server/task-api-capabilities.md` | `7daa1ea5610b6d3917b8e4816d2f89d09f4dae97b166bdd8e0f273ef5b9f4a07` |
| `server/pkg/agent/agent.go` | `d44ad98c4b7b6ac42a3c5039762fb4ebca50fdacf8f123f8d1bc74a0f4d4915e` |
| `server/pkg/agent/acp_usage.go` | `ab8c945101309d98f41eb5077574b790a86000d1bd62f72a450b169ea8afa57e` |
| `server/pkg/agent/hermes.go` | `b77d58ebfb9ed2df3d8e5cec42f403b2dd8f6f8f874e2a85914904e24e116115` |
| `server/pkg/agent/reasonix.go` | `a533cce69c41cbe757a6be3209e9f37847bcf07afe312faf3e685aef278fff32` |
| `server/pkg/agent/claude.go` | `027200a87a76dcb2c2c67a3543bb1591cd169b35609af2e3eeec2a31bd65dec3` |
| `server/pkg/agent/codex.go` | `77423e30b59f7e1fcd2d93532d4c5baa9040e4318b16102cb85e8c320af865f1` |
| `server/pkg/agent/copilot.go` | `8acc130d025529664019d9539868c47a0905158a8bda42e84b7a8b8b8943c03e` |
| `server/pkg/agent/cursor.go` | `dd178d2bc2e55a3f8c73f7fc78b970327aa4c0ed02e2cc8faadf70403233f03e` |
| `server/pkg/agent/pi.go` | `555b1bdcb551c31850506762728b9d803bbd1d3773654f563a539740127f68b9` |
| `server/pkg/agent/opencode.go` | `bc052e5e028bcde28aed1aaa1c97a762c85832316de9ddf3859601783d00db23` |
| `server/pkg/agent/openclaw.go` | `c37a85d29b96d6d2240abf45629237ae69678126660e9d36c755155e2ff2179c` |
| `server/pkg/agent/kimi.go` | `451aecdd59d7f45115c2fe9481b06f18fc2921243ca846fee763e9aa7df2124c` |
| `server/internal/daemon/daemon.go` | `27a914766f0cfebf0be1f66fa6a089a27506cf442fdf597fa0d1f4740fdc7bd9` |
| `server/internal/daemon/client.go` | `ba1b521622c9e6ad2461271ac30ea8517da59c871ad78128421fa0ca310e83ec` |
| `server/internal/daemon/types.go` | `5df0049ed6a19c2a6295b5c445352b35812e8e434eb875790403bdc50c54092d` |
| `server/internal/handler/daemon.go` | `c2773379f601e54fd71a66ccd1d3f5df63b6aa7506f8e11003fb7cfdbe34e157` |
| `server/internal/handler/daemon_rpc.go` | `5fb862e72fff347a5fa15b0c71954608b8467bb49404c4abd5c7796fa967da55` |
| `server/internal/handler/agent.go` | `fe379f7b47f49010deb815139b3a7d9ef743fd382c7d7f67211605656f9f40bf` |
| `server/internal/handler/task_usage_response_test.go` | `5d757c99c745226fed31209b3b8ee6c2094dc9df2b986ab791257a6a1fa35cda` |
| `server/internal/service/task.go` | `8ad09048e05873b9723045682352bcb85d309be90d8584631d9d2b83fe817481` |
| `server/pkg/db/queries/task_usage.sql` | `c5472f09e02310a4508ccd217952c41ec8f00d667a800cf6d4e472d159304ce7` |
| `server/pkg/db/queries/agent.sql` | `4d900ec193fb94e9b3c8f4835224ab64035872a85788bb2b42526cd0ea3999a1` |
| `server/internal/scheduler/jobs_task_usage.go` | `0336f45c784901be84833802f9b9102b7a8d10172e2d07f306c7e00442a4d7d3` |
| `server/migrations/032_task_usage.up.sql` | `bf72146798f8f9dc6cfaab987b6ed745cea4ce84ca7f725fbc18718e7efc2b5d` |
| `server/migrations/101_task_usage_hourly_schema.up.sql` | `090b8dc74d2cabb684fb81020ad2c0a31db5e613096552da89c7a814c2adfae4` |
| `server/migrations/102_task_usage_hourly_pipeline.up.sql` | `87f55e9198c62c5c677ac7f5e6e9e22cb8b34fbd686be2433e1cdfa051e1d2fe` |
| `server/migrations/213_task_usage_authoritative_cost.up.sql` | `ce0b9dd1004da032a9621687b64bc635a7b01a2e8e4e8732f198bf6bae144d04` |
| `server/migrations/272_rollup_task_usage_hourly_xact_lock.up.sql` | `7df30d90dcb935ea9e9543eded8e248b7694156cd21e406ba007e85c210188e9` |
| `server/internal/metrics/business.go` | `3b92084b3e0b2130ed8a4d7071891de89fe0dd05f356d526bc57af4d05eb1191` |
| `server/internal/metrics/pricing.go` | `8eb86e52a20d6976f39ee92ff2a6096b317f67a671ca21512b6972f454375b5b` |
| `packages/views/runtimes/utils.ts` | `878621da906e50c0e5da1c6d0623c3fb84e8d7382eca44566a5214d05a0d8418` |
| `packages/core/runtimes/custom-pricing-store.ts` | `b71f6cd71591edf63a25d8b8633a1954ebbd5f4e2c3fcbcfe166bd38cbee28a3` |
| `server/pkg/agent/acp_usage_test.go` | `e8a1830a4d042838bb47f466143a02c2b22ff3fce4632770bfc98401400a8515` |
