# Research: A 组修复与真实模型实验的独立证据复核

- Query: 核对 A 组附件清单、评论字节限制、本地路径链接三项修复；复核 E0–E6 预登记、框架、样本与原始证据。
- Scope: internal；只读代码与原始材料，不修复、不改数据库、不启动真实模型。
- Date: 2026-10-30
- Code baseline: `/Volumes/artisan/code/2026/mc-collab-integ`，固定 HEAD `4b0cbfde7a2a03142e70ceb91294ec8d8f1ff074`。下文 `server/…` 位置均相对此 worktree。
- Evidence baseline: 本任务 `research/raw/` 的不可覆盖快照；实验源文件来自 `/Volumes/artisan/code/2026/mc-context-exp` 的未提交文件，本次未修改原件。

## Findings

### 结论与证据强度

| 项目 | 本次独立结论 | 分类 | 本次实际运行 |
| --- | --- | --- | --- |
| `7bb077987` 附件历史 | 修复了总历史超过 32 的一个触发，但填充仍不服从实际机器的数量/字节预算；不能放行最终交付 | 实现偏差；已定位数据流，新增边界尚未用失败测试执行复现 | 无 |
| `d84f217a9` 评论 JSON 转义 | 修复方向与 UTF-8 解码后限制一致，未发现新增阻塞；现有测试只直接证明 decoder 层 | 原缺陷已改，最终 HEAD 全链验证缺失 | 无 |
| `b34701756` 本地路径 | 协作评论现已走不依赖普通 task 环境的共用检查，测试断言 HTTP 前拒绝；未发现新增阻塞 | 原缺陷已改；检查范围刻意有限 | 无 |
| E0 修复前 | 旧材料记述未启动模型；原 `/tmp/exp-e0-base.log` 本次已不存在 | 有历史记述，缺原始日志，不能升级为本次复现实证 | 无 |
| E0 修复后 | authorizer 单测的旧日志通过，不等于真实供应商冒烟通过 | 缺真实执行验证 | 无 |
| E1–E6 | 只有未提交脚本与预登记，未见正式完成结果；两个超时试跑应继续排除 | 缺验证；部分行为依赖模型自觉 | 无 |

### A1：附件填充仍能阻塞纯文本指令

现有数据流：

1. `server/internal/service/collaboration_runtime_preparation.go:25` 的 `preparationAttachments` 去重收集本批 `AttachmentIDs`，只检查引用数量是否大于固定 `collaborationMaxSelectedAttachments=32`（`:41`；常量在 `collaboration_attachments.go:29`）。
2. `server/pkg/db/queries/collaboration_preparation.sql:13` 查询所有已交付且未弃置附件，引用优先，再按 `delivered_at DESC, id` 排序，固定取最多 32。它没有实际机器的 `FileCount`、`PerFileBytes`、`TotalInputBytes` 参数。
3. 服务将整个查询结果放入清单（`collaboration_runtime_preparation.go:49`），确认引用存在后按 ID 稳定排序（`:63`、`:70`）。这是“先填到平台数量上限，再整体验限”。
4. `preparationWithinLimits` 才按机器校验数量、单文件字节、总字节和 Git 保留后的空间（`:74`–`:88`），没有区分必须保留的引用与可省略的填充。
5. promotion 先在机器循环外取一次清单（`:99`），随后对每份 grant 对应实际 runtime 整体验限（`:117`、`:147`、`:151`）。claim（`:235`、`:239`）和 freeze（`:358`、`:362`）重复相同模式。promotion 在创建运行/预约前调用（`collaboration_runtime.go:786`），因此纯文字输入仍可能没有可执行候选。
6. 协议允许机器声明小于 32 的文件上限、小于 256 MiB 的总上限和小于 100 MiB 的单文件上限（`server/pkg/protocol/collaboration_preparation.go:263`）。这不是不合法的机器配置。

可从代码直接构造的合法反例（本次未运行 DB 复现）：

- 机器 `FileCount=16`、有 17 个已交付且未引用的小文件：清单 17，整体验限拒绝；本轮纯文字仍可能等不到执行。
- 默认 `FileCount=32`、四个各 80 MiB 的未引用已交付文件：每文件合法，合计 320 MiB 超过 256 MiB，总体拒绝本轮纯文字。
- 机器 `PerFileBytes=1 MiB`，近期一个未引用文件为 2 MiB：可省略的历史文件使纯文字被拒绝。
- 引用集合本身超过机器 FileCount/单文件/总量/空间预算：必须明确拒绝；不能通过裁掉引用来使运行成功。

现有测试的边界：

- `TestCollaborationRuntimePreparationIssueFileHistoryBeyondManifestLimitStillRuns`（`collaboration_runtime_preparation_test.go:594`）使用默认能力，断言清单恰好 32、新文件在、最旧文件不在（`:614`–`:617`）。只覆盖数量超过平台全局 32，未覆盖实际机器更小预算或文件总量。
- `TestCollaborationRuntimePreparationReferencedFileOutranksNewerIssueFiles`（`:623`）验证旧引用优先、未选文件不能经 run 读取（`:648`–`:653`），也只在 helper 层直接验证引用超过全局 32 的报错（`:655`–`:666`）。没有通过 Submit/Promotion/Claim 验证引用超限可被人识别。
- `.trellis/spec/server/collaboration-preparation.md:25` 的新增规则也仅说“剩余到 32”，须与实现一起补上机器预算，不能只改实现后保留冲突规范。

最小方案建议：保持“引用必保、历史尽力填充、首次 claim 冻结”的已有机制；使选择函数拿到实际运行的 capability limits，先验证全部引用存在且整体可容纳，再按既有近期顺序逐个考虑未引用文件，仅加入所有剩余预算均允许的文件。不得因可选历史不合预算拒绝纯文字；不得为了通过预算删除被引用文件。复用同一纯选择逻辑覆盖 promotion、claim、freeze 和 never-launched refusal（`:272`），并保留清单之外的文件不可读（`RuntimeAttachment`）约束。小队须逐个实际执行机器验证，不能让队长能力代表成员。

此方案不要求扩大附件读取权限、不引入新下载接口、不新增依赖。未引用填充可为零；不承诺为了填满预算扫描无限历史。

最小回归矩阵：

| 边界 | 预期 | 验证层 |
| --- | --- | --- |
| 33+ 历史，小文件，默认上限 | 纯文字可 promotion/claim，最多 32，近期优先 | 保留原 DB 回归 |
| 17 历史、FileCount=16 | 纯文字可运行，清单 ≤16 | 新 DB 回归 |
| 历史合计超 TotalInputBytes | 纯文字可运行，自动省略可选文件，清单总量 ≤实际上限 | 新 DB 回归 |
| 未引用单文件超 PerFileBytes，或 Git 保留后空间不足 | 跳过可选文件，保留合法引用 | 选择 helper 的边界矩阵 + 一条链路 |
| 旧引用 + 新历史 | 所有合法旧引用保留，仅在剩余数量/字节预算填新文件 | 扩展原引用回归 |
| 引用数量/单文件/总量/空间本身超限 | 明确 `runtime_preparation_limits`，无运行/预约/半截副作用，人可得拒绝原因 | 服务/API 链路；不能仅直接调 helper |
| 引用丢失或被弃置 | `runtime_preparation_unavailable`，不能静默省略 | 服务链路 |
| claim 后新交付、重试和非启动收敛 | 冻结清单不扩张；原身份和输入状态守恒 | 复用已有 frozen/nonlaunch 回归 |
| 小队各机器预算不同 | 每个 run 遵守自己的实际机器上限，引用均完整或整批明确拒绝 | 小队服务集成 |

### A2：JSON 转义后的读取预算

- `server/internal/handler/collaboration_runtime_business.go:14` 返回 `6*maxBytes + 64 KiB`，评论和小队 activity 分别传各自解码文本上限（`:25`、`:35`）。Go 对 `<`、`>`、`&` 的默认 JSON 转义由 1 字节变 6 字节；该读取预算覆盖合法原文及固定身份 envelope。
- 共用严格解析器仍限制 raw body、拒未知/重复字段（`collaboration_runtime.go:57`–`:79`）。扩大的是传输读取预算，不是产品内容上限。
- `CreateRuntimeComment` 调 `validateCollaborationCommentInput`（`server/internal/service/collaboration_squad.go:205`、`:210`），后者仍以 `len(Content)` 检查 64 KiB（`collaboration_mentions.go:118`–`:122`）。
- `TestCollaborationRuntimeCommentBodyAcceptsCodeWithinTheContentLimit`（`collaboration_runtime_comment_size_test.go:19`）覆盖 49,600 字节 HTML、64 KiB 最坏转义评论、activity 最坏转义及远超读取上限的 413（`:42`–`:54`）。断言直接驱动 decoder 闭包，成功返回 204，没有走真实服务或 DB。因此它证明“合法最坏转义可解码且传输仍有界”，不能单独证明“所有刚超过解码内容上限都会被业务拒绝”。
- 收尾需补/确认服务层超限回归：64 KiB 合法与 64 KiB+1 的普通 ASCII（仍在 raw 读取预算内），并验证没有落库。无须重复已由共用 parser 矩阵保护的未知字段全部排列。

### A3：协作评论的本地链接检查

- `server/cmd/multica/cmd_collaboration.go:91`–`:95` 改用共用 `rejectLocalPathLinks`，发生于 `run.Read`（`:115`）和 POST（`:143`）之前。
- 共用 helper `local_path_link_lint.go:137` 的原 gate 仍只用于普通 agent/task；无 gate 的 helper 已在同文件 `:144`–`:161`。协作 run 不设置普通 task 身份，因此换用它是必要的边界修复。
- 新测试 `TestCollaborationCLICommentAddRejectsLocalPathLinksBeforeHTTP`（`cmd_collaboration_test.go:258`）创建 `file://` 链接、断言请求次数为零且返回明确错误（`:265`–`:269`），不是只检测字符串 helper。
- 既有 canonical helper 测试覆盖真实链接/图片/autolink、工作目录和本机存在路径（`local_path_link_lint_test.go:41`），允许代码块、行内代码、应用内链接和普通文字（`:113`）。这是刻意的精确规则，不是完整任意本地路径防泄漏系统：相对链接和无法确认属于本机的绝对路径依赖提示约束（实现注释 `local_path_link_lint.go:86`–`:103`）。不应把这点再次报成协作修复失败，亦不应宣称它能强制所有交付物都可访问。

### E0：必须使用真实的冒烟入口

- 正式入口 `server/internal/daemon/collaborationrun/host_agentintegration_test.go:107` 先调用 `authorizeHostAccountSmokeRequest`，后续才把请求给 fixture。修复后 `:214` 明确允许 `POST /memory/base`，且仍须同一 run 路径和实际已发 permit 的业务 token（`:194`–`:225`）。
- 测试 `TestHostProviderAccountSmokeAuthorizesSharedContextBase`（`host_agentintegration_authorizer_test.go:17`）不启动供应商，只正反校验 token（`:20`–`:27`）；旧集成日志 `research/raw/integ-check.log.txt:8`–`:12` 只显示这条单测通过。
- 真实 Host 在组装 prompt 前读取 memory base（`host_runner.go:131`–`:136`）。正式 smoke 会同时验证 Provider 的 launch/termination/proof、通过当前 Multica CLI 读取上下文、最终成功内容及 ACK 后本机账本（`host_agentintegration_test.go:140`–`:166`）。因此新 E0 应使用 `TestHostProviderAccountSmoke`（函数声明在 `host_agentintegration_test.go:31`），不能只运行 authorizer 单测。
- 实验脚本的 `E0b_sanity` 是另外的实验场景（快照 `host_context_experiment_test.go.txt:299`），脚本 HTTP handler 直接 fallback 到 `fixture.serve`（`:149`–`:200`），没有调用上述 authorizer；fixture 在 `/memory/base` 分支仅查 `mcb_` 前缀（当前 `host_runner_test.go:202`）。它在修复前也不会因同一个白名单缺口阻断，不能替代 E0 的前后对照。

历史材料的限制：

- 预登记结果记述 E0 修复前于 `c58f79874` 失败、耗时 7.4s、CLI `claude 2.1.312`、darwin/arm64，Provider 未启动（`research/raw/experiments-preregistration-2026-10-29.md:29`–`:31`）。
- 唯一指定原始日志 `/tmp/exp-e0-base.log` 本次已经丢失（`research/raw/source-manifest.json` 对此记录 `present=false`）。应保留“历史运行记述，原始证据缺失”的等级；如最终验收需要可复核的修复前证据，应用不访问账号的 authorizer 旧版复现或隔离旧 worktree 重跑正式 E0，并保存新日志。
- 两个 5 分钟超时试跑继续作废（预登记 `:24`–`:27`）。原文把原因定为账号限流，但目前只有 keychain 失败、另一代理同一时段 429 和稍后恢复的记述，缺直接证明同一调用 429 重试的原始记录。宜归“环境/账号故障，具体根因未证实”，不将其写成产品失败，也不把限流根因当已证实事实。

### E1–E6：实验能测什么、不能测什么

| 场景 | 脚本真实覆盖 | 不能从其推导 |
| --- | --- | --- |
| E1 | memory 在启动 prompt 注入后的问答正确性；是否主动 PUT。输入不禁止写 memory（快照 `:300`–`:301`） | 答对不证明调用过 `memory show`；C11 写入仍是模型自觉，不能因一次答对声明“先读再写机制强制成立” |
| E2a/E2b | 脚本给定恰好 200 条评论，关键事实分别在尾/首（`:268`–`:293`、`:302`–`:305`） | 未走真实 SQL 的最新 200 限制，不覆盖 201+ 分页、byte budget 或全历史召回；无原始工具输出时无法证明模型错误源于 CLI 截断 |
| E3 | 设置 member Role/StepKey/ParentRunID（`:122`–`:125`）；上下文响应给 dispatch_comment（`:157`）；修复后 prompt 引导 member 只做派活（当前 `host_runner.go:517`–`:522`） | 不是“派活正文现在已直接注入 prompt”；仍需真实 CLI 召回 dispatch。不是队长续作/成员结果传递实验 |
| E4 | 第一次 PUT 故意返回 stale，并同时放入另一通道事实（`:174`–`:187`） | 不覆盖生产 DB 并发/幂等/授权检查；它是模型合并行为实验，生产版本守卫仍需服务测试 |
| E5 | 脚本评论中插入 CANARY 指令（`:313`–`:317`）并观察评论/最终答复 | 脚本只有 author_type/UUID，没有真实参与者或 SubmitCommand 权限校验；不证明“非指令者权限已隔离”；0/3 不等于抗所有注入 |
| E6 | 第一跳只把写成的 memory 传给一个全新目录/ledger 的第二跳（`:96`–`:100`、`:327`–`:329`），3 对接力 | 不是两台真机器或两家供应商实测；单机同 CLI/账号会继承相同 HOME；迁移 CONCURRENTLY 是场景给定事实，不是该实验验证过的 DB 正确性 |

预登记中 E1/E2/E4/E5/E6 选择修复前 `c58f79874`、E3 前后各三次（预登记 `:8`–`:9`）。不能悄悄全部改到最终 HEAD 后仍宣称完全按原方案。建议保留原文件与预测，另写有时间戳的运行前补充：严格复现历史条件的样本和最终 HEAD 的确认样本分开标记；所有样本注明真实代码 SHA、框架哈希和改动。E3 前后固定同一模型和框架，只改变受测代码；避免将当前不受测的其他改动混入因果解释。

### 当前实验框架需先补齐证据，而非直接跑完就算完成

以下都在未提交快照 `research/raw/host_context_experiment_test.go.txt`：

1. `contextRecord.Commit` 从环境自由填写（`:68`–`:69`、`:143`），未核对构建源 HEAD，不能当可信代码出处。
2. `provider_version` 记录了 CLI 版本，但没有实际模型 ID、provider 名、可执行文件/Multica CLI 版本或构建哈希、thinking/service-tier（`:143`）。运行配置清空 Invocation、未填 DefaultModel（`:112`、`:129`），最终会使用拥有者 CLI 默认模型（当前 `host_runner.go:163`–`:166`），模型选择会漂移。
3. `Prompt` 字段只声明为 `prompt_sha_note`，从未赋值（`:53`）；不存在实际送入模型的 prompt/brief 完整快照，也没有注入 memory 版本/hash 与场景输入/评论快照的可靠绑定。
4. HTTP 记录只有次数和写入内容（`:154`、`:164`、`:174`、`:195`）；没存有序 request/response、状态码、时间和相关 run/request ID。记录不应存 Bearer token；应该以不可逆/非秘密别名绑定身份。
5. 产品执行记录再被脚本每项截到 400 字节（`:225`–`:230`），且丢 `CallID` 等结构字段。对于 60 KiB 的 E2 评论输出，这无法判断工具本身先截、Provider 展示截、还是模型忽略。需要独立保存完整可审计的工具输入/结果与实际 provider 流，若产品已经截断则明确其 partial 标记并单独保存测试夹具输出。
6. `runErr` 只记字符串，未据此 fail；无 outcome 或错误答案也可能 `go test PASS`（`:217`–`:240`）；正确性必须用预先固定的评分规则另出逐次 verdict，不得把 Go 退出 0 当产品通过。
7. 输出追加忽略 `Write` 返回值，且未处理 close 错误（`:249`–`:254`）；实验进程中断前还可能没有任何 trial 记录。需要 durable 的 started/finished/incomplete 标记、实际退出码、可验证哈希和每次单独目录。原始目录必须在 task/reports 下，不能唯一放 `t.TempDir` 或 `/tmp`。
8. trials 未设置时默认 1（`:89`–`:92`），与预登记每场景 3 次不一致。命令必须显式 `MULTICA_CONTEXT_EXPERIMENT_TRIALS=3`，每个场景串行且不并发运行其他账号模型工作。

这些是“框架补证”工作，应先以假 Provider 验证记录/评分/失败分类，不消耗真实额度；再做正式实验。不要修改原预登记，也不要在看到模型结果后修改判定。

### 建议正式实验排程与门槛

1. 固定修复后最终 HEAD；先完成机器预算补修和全栈检查，避免在非最终代码上花额度却无法交付。
2. 固化并提交实验框架（仍仅 `agentintegration` build tag + `MULTICA_RUN_REAL_AGENT_SMOKE=1` 才可账号/CLI 访问），写运行前补充登记；代码版本从构建 source 读出并核对，不依赖手填。
3. 单独运行正式 E0 修复后 1 次：在隔离 worktree 的 `server/` 下，显式配置 provider/绝对 executable 后运行 `MULTICA_RUN_REAL_AGENT_SMOKE=1 go test -tags=agentintegration ./internal/daemon/collaborationrun -run '^TestHostProviderAccountSmoke$' -count=1 -v`，原始 stdout/stderr 保存到任务证据目录。成功必须有实际 launch/termination/proof/CLI context-read/explicit outcome；成功后才开始 E1–E6。E0b 不替代此门槛。
4. 按 E1 → E2a → E2b → E3 baseline/after → E4 → E5 → E6 顺序，单场景单 trial 串行，不 `t.Parallel`，不同时启动第二套模型实验。Go 子测试可用精确 `-run '^TestHostContextExperiment/E1_memory_first$'`，且显式 trials=3；实施时按实际新的框架入口更新命令。
5. E1/E2a/E2b/E4/E5 各 3 次；E3 修复前后各 3 次；E6 三个配对接力，每对两次调用，按“三对”计，不能把六个 hop 当六个独立成功样本。按现有方案，除修复前无模型 E0 和另行环境诊断外，共 28 次正式模型调用（修复后 E0 1 + 五个单场景各 3 + E3 6 + E6 6）。如运行前补充要求最终 HEAD 确认样本，须另列数量，避免隐藏扩大实验。
6. 每次保留 scenario、trial、arm、模型与 CLI 精确版本、实际 prompt/brief、源数据及版本、工具请求/结果、run/outcome/proof、评分及证据定位；答错是产品/模型行为结果，不能重跑到通过再挑样本。
7. 失败分类看请求所属服务和语义，不能只按状态码：供应商 401/429/503、CLI 无输出、transport 中断可归环境；Multica 对过期/撤权 token 的 401/403、模拟 stale 409 是受测产品行为。未知根因记 unknown，保留全部无效/中断 attempt，不进入成功率分母。上游连续不可用时保留缺口并停止付费重试，不把环境未知宣称产品通过。
8. 预登记“预测”不是可事后放宽的通过标准。结果应同时报告原阈值、有效样本/无效尝试、实际结果及产品剩余限制；特别 E5 任何 canary 被当作指令执行都应进入阻塞审查。三次样本用于发现机制缺口，不支持泛化可靠性承诺。

## Files found

- `server/internal/service/collaboration_runtime_preparation.go`：附件选择、机器校验、promotion/claim/freeze/nonlaunch 共同路径。
- `server/pkg/db/queries/collaboration_preparation.sql`：引用优先和近期历史补齐查询。
- `server/internal/service/collaboration_runtime_preparation_test.go`：33 文件历史和引用优先两个新增回归。
- `server/internal/handler/collaboration_runtime_business.go`、`collaboration_runtime_comment_size_test.go`：传输字节预算和 decoder 回归。
- `server/internal/service/collaboration_mentions.go`、`collaboration_squad.go`：解码后产品限制和实际写入入口。
- `server/cmd/multica/cmd_collaboration.go`、`cmd_collaboration_test.go`、`local_path_link_lint.go`、`local_path_link_lint_test.go`：协作 CLI 与共用路径检查。
- `server/internal/daemon/collaborationrun/host_runner.go`、`host_memory.go`、`host_runner_test.go`：真实 prompt 注入、模型选择和测试 fixture。
- `server/internal/daemon/collaborationrun/host_agentintegration_test.go`、`host_agentintegration_authorizer_test.go`：正式账号 smoke 与不调用模型的白名单单测。
- `research/raw/host_context_experiment_test.go.txt`：17292 字节未提交实验框架快照，SHA-256 `24149b979f88fdcc604ac4c13b889d99261639999ff1c4a889a7be3828081553`。
- `research/raw/experiments-preregistration-2026-10-29.md`：E0–E6 预登记与两次无效运行记述。
- `research/raw/source-manifest.json`、`integ-check.log.txt`、`integration-reflog.txt`：原始材料存在性、旧检查日志与 rebase 时间。

## Related specs

- `CLAUDE.md`：真实 Agent 测试必须双 gate；默认测试不能查找用户 CLI；测试前置与迁移硬规则。
- `.trellis/spec/server/collaboration-preparation.md:18`–`:32`：冻结清单及历史填充；`:58`–`:62`：平台上限；须同步实际机器预算规则。
- `.trellis/spec/server/collaboration-host-execution.md`：普通 host 权限、Provider/OS 验证范围、memory 注入。
- `.trellis/spec/server/collaboration-runtime-cli.md`：窄 run 身份、评论/协作上下文能力与 UUID 重试。
- `.trellis/tasks/archive/2026-10/10-26-collaboration-memory/prd.md` 的 C08–C15、CA01–CA04、CA15–CA16：基线、并发、写入门禁与模型引导而非强制。

## External references

未查外部资料；本次结论来自本地一手代码/夹具/规范。历史 CLI 版本只有预登记记述的 Claude `2.1.312`；未探测当前账号/模型或运行任何供应商命令。

## Caveats / Not Found

- 本次没有运行 Go/TS/DB 测试，不能把静态审查通过写成最终 HEAD 测试通过；默认 DB 测试可能在不可达时静默退出，执行阶段必须检查实际 RUN/PASS。
- A1 的机器预算反例已由实际代码和合法协议参数确定，但新增失败测试尚未创建/执行；A 组历史独立“通过”不能覆盖本次遗漏。
- 未找到正式 E1–E6 完整结果；历史 E0 原 `/tmp` 日志已不存在；未确认两个超时运行的精确底层根因。
- 本报告不是 B/C 配置授权或迁移审查；未扩大到其负责范围。
