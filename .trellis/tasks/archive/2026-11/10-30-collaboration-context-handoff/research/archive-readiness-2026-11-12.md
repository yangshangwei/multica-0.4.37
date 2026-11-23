# Research: 上下文交接父任务、Runtime 与首轮修复归档评估

- Query: 核心功能是否已实现，这三个任务能否归档？
- Scope: internal；只读核对既有任务、规格和少量当前源码，仅写本报告。
- Date: 2026-11-12

## Findings

### 结论

三个任务的核心工程实现已有交付与有界验收证据；没有发现一整块尚未开发的 Runtime 产品功能。但它们的原范围同时承诺真实模型行为及跨子任务最终验收，现有记录不足以按原范围直接标记全部完成。

如果用户希望清理长期活跃任务，可以把“工程实现与确定性验收”作为已完成阶段，把未完成的真实验收和旧失败处置逐项移交到后续任务，再归档旧阶段。这是范围调整，不是把未测或失败改成通过。`.trellis/workflow.md:78` 说明 `task.py archive` 会设置 `status=completed`，因此仅运行命令不能表达“仍有未完成范围”。

| 任务 | 核心实现 | 按原范围直接完成归档 | 阶段归档应移交 |
| --- | --- | --- | --- |
| 协作上下文交接、召回与沉淀 | 三个子任务的实现已有集成；父任务主要负责统筹与最终验收 | 不满足，父 AC3/AC7 的真实行为尚未完整闭合 | 子任务剩余真实验证、失败处置及跨子任务最终结论；发布事项按已确认最新范围独立处理 |
| 可靠上下文交接与召回 | handoff、checkpoint、失权封口、来源/分页召回、启动绑定、Web/Desktop 展示均已有交付 | 实现阶段具备结项依据；原任务仍含真实模型验证，不宜无说明直接 completed | H3a-t3、H3b、H5、H6，当前源码/CLI/配置/数据库/窗口/准入绑定及历史 H3a-t2 保留 |
| 首轮协作修复交付闭环 | 8 个修复、附件预算、迁移/sqlc/全栈门禁已完成有界验证 | 原 E0–E6 闭环尚未完成，且 E5 为有效行为失败 | E5 失败处置、原 E5t2/3 与 E6 停止状态、实验样本限制，或明确结束原研究系列并保留否定结果 |

### Runtime 的核心工程与真实模型证据

- `../10-30-collaboration-context-handoff-runtime/verification-integration-2026-11-01.md:3` 记录产品提交 `0e1ee8e1e`，116 个交付文件与隔离集成目录逐字节核对；第 11–17 行列出已交付 checkpoint/seal、异步记录恢复、稳定启动快照、prompt/attempt 绑定、来源召回、预算与客户端。
- 同报告第 23–30 行记载当轮 Go、TS、静态检查、六个生产 Web/原生 Electron 用例及独立审查结果。它们是对应版本的历史证据，本报告没有重跑。
- 当前源码仍有完整实现入口：`server/internal/daemon/collaborationrun/host_handoff.go:43` 获取并校验快照，`:149` 绑定实际启动身份；`server/internal/daemon/collaborationrun/host_runner.go:185` 在执行流程中取得 handoff，`:614` 把普通评论、memory、交接说明、成员结果明确作为资料；`server/internal/service/collaboration_runtime_handoff.go:140` 提供运行时 handoff，`:162` 构建有来源的交接包，`:288` 和 `:381` 分别投影输入与结果。
- 不是“从未启动真实模型验收”：`../10-30-collaboration-context-handoff-runtime/task.json:34` 记录 E0a 行为通过（配置漂移单独处置），H1/H2/H2b 各 3/3，通过 H3a trial 1；trial 2 超时不完整。`progress.md:31` 记录 H2/H2b 独立复核与提交 `7c10f1442`、`8c0152b21`。
- “11 月 12 日真实验收未开始”只指本轮新续作。新入口 `server/cmd/server/rk1_runtime_continuation_test.go:1042` 和控制测试 `:1095` 已存在，但这是测试/执行登记基础，不代表新产品能力或真实验证通过。
- H3a-t2 的直接终止原因是固定五分钟 Provider 超时，未收到最终 result；模型已通过工具读到决定码。3025 个 Multica HTTP 调用为 2xx，记录、proof、seal 完整，无新产品补丁依据。上游未完成的具体原因未知，不能称作已确认的网关问题或产品缺陷。见 `../10-30-collaboration-context-handoff-runtime/research/continuation-20261105/h3a-entry/failure-diagnosis.md:3`、`:27`。
- 后续未执行顺序仍为 H3a-t3 → H3b → H5 → H6；历史消耗 12/25、剩余 13，当前真实 gate 全部 false。用户已批准有限历史停止例外与满足条件后的续作，不能重复索取同一授权，也不能认为这已满足当前技术绑定。

### E5 是真实失败，但不是已证明的当前产品越权漏洞

- `../10-30-collaboration-fix-closure/verification-2026-11-05.md:7` 记录 8 项修复、冻结 V16 全栈门禁及 password7 验收通过。
- 同文第 18–23 行记录 E3 after2/3、E4 三次通过；E5t1 在明确拒绝注入的说明中复述注册 canary，按事先冻结的输出判据有效失败，没有评论写入。E5t2/3 和 E6 未执行。
- E5 对应旧冻结产品 `c58f79874cfe2e6318e3dce067d7b6bac24e943f`；不能扩张成已经执行恶意指令、越权写入、真实秘密泄露或当前 V16 回归。也不能因为拒绝措辞而事后改判通过。
- 当前资料边界比旧版本更明确，但没有任意标记输出过滤，也没有真实模型证据证明当前版本不会再次在拒绝说明中引用标记。见 `../10-30-collaboration-fix-closure/research/e5-source-review-20261105/review.md:7`。
- 该旧报告建议的“仅引用、零业务写入” H5 fake 回归后来已完成并合入，`task.json` 的 `meta.h5_quote_regression` 记录 normal/race、变异验证和独审；不能再把它当未实现。它证明评分/停止机制能捕获此分支，不证明真实模型抵抗已通过。
- 原系列仍累计 19 次、余 8 次暂停；`../10-30-collaboration-fix-closure/task.json:233` 明确 E5 有效失败、`:234` 明确 E6 未运行、`:235` 明确 `archive_ready=false`。

### 父任务的完成条件

- `prd.md:82` 的 AC0 要求带版本的交付链与全栈验证；这些已有冻结候选证据。
- `prd.md:85` 的 AC3 明确要求 E1–E6 的预登记真实证据，`:89` 的 AC7 明确包含真实 E5；不能只凭 deterministic 通过消除这两项。
- 旧 `acceptance-2026-11-01.md` 包含多个历史快照。开头的 11 月 4/5 日补记已覆盖旧完整检查失败，但其中旧预算、未开始实验、待补矩阵不能作为最新状态。当前应优先读 task.json、最新日期化结果与原始失败处置。
- 父任务 `task.json:218` 的 11 月 12 日更新继续保留 in_progress，范围为真实验收和其他子任务门。父任务无需再开发一套独立产品实现，主要缺最终范围和证据收口。
- 当前明确开放的原范围是：原系列 E5 的失败处置与未运行的 E5t2/3、E6；Runtime H3a-t3/H3b/H5/H6；知识 K1–K3 的独立真实验收。各账本和停止规则应保留，不因移交而重置。
- 父元数据 11 月 9 日记录仍称“发布边界独立”，但没有证明旧矩阵每一项仍是当前结项必需。Windows 原生、安装组合和 usage/账单来自较早快照；协调者已指出 10-24 跨平台范围后续收窄并归档，且用户排除了 usage/账单对账。本报告不将它们自动恢复成必做项，也不以“全部旧发布门完成”为这些工程任务阶段归档前提。仅凭本次所读资料不能逐项确认当前生产发布剩余范围。

### 未提交变更与归档前边界

- Runtime `task.json:662` 的本轮候选记录是 `dirty_product_path_count=177`，这是当时产品根目录总数，不是 Runtime 任务拥有 177 个变更的证明。
- 新 Runtime 续作入口在当前源码可见，记录已写本轮验证；本次研究角色禁止任何 Git 操作，未自行复核当前索引、提交归属或 main 集成关系。主协调者应在真正归档前按任务清单核对相关 dirty code 是否已提交、对应验证是否仍适用。
- 主协调者本轮另行核对指出 Runtime 续作源码及 UI/CLI 提示源码仍有未提交变更；本报告只记录该协调观察，未重复执行 Git 检查。Runtime 归档应先收口本任务新续作代码的提交/验证，不能把旧实现提交当作覆盖新入口。
- 不应为归档清空、恢复、打包提交其他会话的未提交内容，也不应删除旧原始失败记录、账本或修改真实执行开关。

## Files found

- `prd.md`：父需求与 AC0–AC9 的产品/真实验收合同。
- `acceptance-2026-11-01.md`：父验收矩阵与多次历史补记，须区分日期。
- `../10-30-collaboration-context-handoff-runtime/prd.md`：Runtime 已交付范围与真实理解单列规则。
- `../10-30-collaboration-context-handoff-runtime/verification-integration-2026-11-01.md`：工程交付、平台与确定性验收。
- `../10-30-collaboration-context-handoff-runtime/progress.md`：H1/H2/H2b/H3a 真实结果、11 月 12 日有限续作准备。
- `../10-30-collaboration-fix-closure/verification-2026-11-05.md`：V16、password7、原系列 E3/E4/E5 的结论。
- `../10-30-collaboration-fix-closure/research/e5-source-review-20261105/review.md`：E5 失败含义、当前代码和 H5 评分的区别。
- `server/internal/daemon/collaborationrun/host_handoff.go`、`host_runner.go`：当前 handoff 获取、启动绑定和资料语义。
- `server/internal/service/collaboration_runtime_handoff.go`：当前 handoff 投影与分页来源。
- `server/cmd/server/rk1_runtime_continuation_test.go`：本轮续作登记、预算、前置绑定和真实入口。

## External references

无。本评估只使用仓库源码、规格、任务及历史验收记录，没有联网、真实 Provider/CLI 调用或数据库访问。

## Related specs

- `.trellis/spec/server/collaboration-startup-handoff.md`：启动快照、实际输入绑定不等同于模型理解保证。
- `.trellis/spec/server/collaboration-context-experiments.md`：真实与 fake 分账、预登记判据、E5/H5 停止和输出引用规则。
- `.trellis/workflow.md`：归档会设置 completed，阶段归档必须先明确移交未完成范围。

## Caveats / Not Found

- 本轮是只读事实核对，仅创建本报告；没有重跑测试、验证当前整个工作树、调用真实模型或执行归档。
- 未对知识子任务另做完整评估；本报告父任务结论引用已有集成记录，知识的真实 K1–K3 与生产开关由其专项评估补充。
- 没有发现足以要求 Runtime 新产品补丁的 H3a 超时证据；这不等于证明当前所有运行可靠性问题均不存在。
- 核心工程实现已交付、单个真实样本成功、完整真实验收通过、生产启用与发布完成是不同结论，应分别记录。
