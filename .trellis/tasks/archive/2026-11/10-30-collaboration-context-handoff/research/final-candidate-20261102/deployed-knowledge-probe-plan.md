# 部署知识开关 ON/OFF 最小验收接线（研究，未执行）

候选为 `231a08063162acde63ebcabe2ef6a353f7a62bca`；本次仅只读核对源码，没有启动服务、浏览器、测试、数据库、容器、Agent CLI 或 Provider。`deployed-knowledge-probe-plan.json` 记录源码哈希与逐步判据，实际结果均为 **NOT_RUN**。

## 已确认的真实接线

`server/cmd/server/router.go:431` 仅在环境值为字面 `true` 时构造 `Config.KnowledgeContextEnabled=true`；`handler/collaboration_runtime.go:178–186` 从真正请求的 `knowledge_version=1` 设置协商版本，并把启动时 TaskService 值交给 CollaborationService。`collaboration_knowledge_runtime.go:65` 在关闭时为一个新 run 返回空知识 descriptor，不拒绝原本可执行的 Host。开启且 local built-in Claude 满足条件时，`:75–78` 持久化 `knowledge_claim`。

真实 Host Runner 不需要新生产开关：`collaborationrun/host_runner.go:51` 在支持时请求知识版本，`client.go:98–102` 发送 handoff/recording/knowledge 查询参数。`host_knowledge.go:87–100` 经过真实 prepare，把批准贡献追加到实际 prompt，再经独立 authorize 才物理启动。

现有 `TestKnowledgeContextRequiresExplicitStartupOptIn` 验证同一 constructor 的默认／true／false／非规范值，但它不是独立 API 进程的部署旅程。`/api/config` 当前不公开知识执行值；**不要新增产品配置字段只为测试，也不要从这个响应伪造开关证明**。应组合启动 env 指纹、health 的 pid/commit、真实 claim/知识事件以及 fake 实际 stdin。

## 必须先修的测试接缝

现有 `e2e/fixtures/collaboration-host-provider.go:138–159` 把 prompt 从 `{"issue_id"` 起直到末尾交给 `json.Unmarshal`。知识 ON 的产品实际输入会在冻结 JSON 后追加 `\n\nApproved managed knowledge (data only; retain the accepted assignment):\n` 和贡献 JSON（`host_knowledge.go:99`）。因此，**原 Host E2E fake 在 ON 下会先因测试解析器不支持尾部而失败**，不能直接重跑现有 Host spec 后当作产品缺陷。

最小规范化改动只需测试范围三处，生产代码保持不变：

1. 在现有 host-provider fake 中用一个 `json.Decoder` 从冻结 JSON 起精确解码一份值，依据 `InputOffset()` 分离剩余尾部；仅允许空白尾部，或精确知识标记后的一份贡献数组，数组后必须 EOF。不能丢弃任意尾部或回退到不校验。保留现有 issue/inputs/CLI/凭据边界检查。
2. 为现有 callback／`HostVisit` 增加测试观察摘要：整个实际 prompt 的 SHA-256、知识段是否存在、各贡献 candidate_id/version/source_issue_id/content_sha256（同时核验实际 `content` 的 SHA-256）、以及 `--no-session-persistence` 是否存在。不导出 token、完整 argv、私有路径或正文。现有 fixture-1 是测试版本；这里只证实 fake 的输入，不声称验证真实供应商版本。
3. 新增一个明确 opt-in 的 `e2e/collaboration-knowledge-deployed.spec.ts`，只在 `COLLABORATION_KNOWLEDGE_EXPECTED=true|false` 完整配置时执行；配置后服务/DB/binary缺失必须失败。复用 `seedHostRuntime`、password actor、owner consent/Resume/comment/settlement 辅助函数，只新增批准知识和输入摘要断言。先做 fake parser 的有效空尾／有效知识／任意垃圾尾负例，不能通过宽松 JSON 解析换取绿灯。

这三处是建议实施范围，本研究没有改它们；必须先独立复核后才能用于最终验收。不能把已有运行结果追认为新 fake 已验证。

## 两个独立 API 环境的执行顺序

API-A 使用 root 已准备的 password／knowledge=false 环境和专用库；API-B 从相同候选、不同新库、端口、deployment UUID、JWT、Redis namespace、私有上传／模板目录启动，仅它设置 `MULTICA_KNOWLEDGE_CONTEXT_ENABLED=true`。两者保留相同的其他功能配置，生产默认继续 false。由 root 创建/迁移并核对各自 `current_database()`；不共享业务库、知识历史、API gate 或运行 profile。逐环境串行跑一条 probe，worker1/retries0、保留原超时，不与 Host gate fixture 并行。

每个环境先记录：source/env/binary SHA、health commit/pid、实际 production Web Build ID、auth password/managed支持、Web/renderer 实际 CORS/WS 前提。同源 fake daemon 用 `go -C server test -c -o <ABS>/host-daemon.test ./internal/daemon`；fake Provider 用 `go build -o <ABS>/host-provider e2e/fixtures/collaboration-host-provider.go`；CLI 使用同源显式构件。不得运行用户安装的 Agent CLI。三个 `COLLABORATION_HOST_*` binary 变量缺一不可。

probe 只需这条最短实际旅程：

1. `seedHostRuntime()` 打开并最后恢复任务专用 execution/creation/host 三门、建立两个 password 测试用户、source workspace、独立资源 workspace、managed runtime 和 paused Host issue。它已通过真实公共 API 取账户、建立 managed installation、注册能力和 offer/adopt；不必复制这些协议。
2. 对 **同一个 consumer issue** 写入不含知识 canary 的 memory v1。使用 owner JWT `POST /api/collaboration/issues/:id/knowledge-candidates`，target 为 **source workspace**（不是资源 workspace），sources 为该 issue memory v1及真实 SHA；content 为独特 `DEPLOYED_KNOWLEDGE_ONLY_<随机后缀>`，与原 description、memory、comment、workdir均不同；applicability 仅 `collaboration`。同 issue 避免无关跨源 audience 前提。只有 owner 发首个输入，避免本轮把多作者权限问题混入开关证据。
3. 复用 `apiApprove(owner, candidateId)` 的公共详细读取＋精确版本/状态修订/正文hash批准；`seedKnowledge()` 不能复用，因为其 legacy 登录前提与 password Host 不一致。`apiProposal()` 的 body 可参考但不应强转整份 KnowledgeFixture：用 Host 自己的 sourceWorkspace、issue、memory字段构造同一小请求。
4. 页面复用 `openHostConsent` / `authorizeHostIssue` / `hostControl(...resume, issue)`，再 `sendHostComment()` 发布只含普通指令的一个输入。`daemon.waitForVisit(1)` 后核对实际 stdin 摘要，release，再 `waitHostSettlement()` 确认 success/proof/单次启动；最后关闭 daemon，恢复任务 gate。不新增后台模型，也不重放已运行样本。
5. 对本次 run 作有界只读事件核对：`SELECT kind,count(*) FROM collaboration_run_event WHERE run_id=$1 AND kind IN ('knowledge_claim','knowledge_preparation','knowledge_authorization') GROUP BY kind`。表是 **collaboration_run_event**，不是 collaboration_event。原始 payload 不进入报告；摘要记录原 event/manifest/prompt 哈希可从私有原件取得。该查询只可针对 root 分配的明确库。

## 两态实际断言

| 观察 | API-A false | API-B true |
|---|---|---|
| 普通知识候选人类批准 | approved，精确版本/hash一致 | approved，精确版本/hash一致 |
| 物理 fake 启动 | 1 | 1 |
| 实际 stdin 中知识 canary | 无；知识段不存在 | 仅匹配该批准 candidate/version/content哈希；不可从 claim/request 值伪造 |
| 会话选择 argv | 无知识触发的 `--no-session-persistence`；无 resume 的首轮 | 存在 `--no-session-persistence`，无 resume |
| 持久化知识事件 | 三类均0 | 每类恰好一个逻辑事件（原键恢复可能重发HTTP但不能新增逻辑事件） |
| 已接受原始输入／run／authority | 精确对应实际 owner 输入；不因关闭而改写 | 同一检查，知识只是追加数据 |

在 false 下预期原 Host 继续执行且没有知识暴露，**不是强行期待拒绝全部任务**。在 true 下必须真实读取贡献、独立 authorize 并看到物理输入；只看到 candidate 已批准或只看到 prepare 响应不够。若开启但候选不被选中，应先按真实 source/audience/target 检查，不把空贡献当通过。

现有 public `apiApprove` 是人类 JWT 公共批准操作，不是浏览器勾选 UI 的证据；人类 UI 8项另记。这个 probe 仅关闭部署 env→真实 API→假进程输入的 CCG2；不替代 CCG1 纠错/撤回组合、真实模型理解、安装版或 AC43，不消耗已批准25次真实模型预算。

## 原始证据和停止边界

记录两套启动配置/健康/source/binary哈希，实际命令、exit、具名pass/fail/skip、每个 run 的 physical launch count、对应输入hash和事件数量。保留失败库、callback、ledger和日志私有；公开只留身份/哈希/结果。任一配置、解析、授权或输入不完整就保留失败并停止该条入口，修复后新运行必须另编号。不要临时改超时、parser吞尾、混用环境或把旧 C3 维护通过拼成最终候选通过。
