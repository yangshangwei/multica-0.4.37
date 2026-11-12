# Research: design.md 队列与运行控制独立审阅

- Query: 审阅 D02、D06–D08、D11 的 FIFO、启动/终结 fence、撤权回执及部署恢复是否具有可执行契约。
- Scope: internal；有界设计审阅。主文件为 .trellis/tasks/10-10-cross-workspace-collaboration/design.md，以下简称 design.md。
- Date: 2026-10-12。
- 基线: 已读最新 R39（仅新建）；O11 未决；v1 不做服务端隐式失败重试；撤权后终结凭据不得追加正文/附件。没有修改设计、产品代码或其他研究文件。

## Findings

总体方向成立：每单永久 session、逐条作者/FIFO、未结 run 独立于 task 终态、最小终结凭据和不向旧二进制回退，已覆盖此前研究的主要问题。以下是实施前应补充的协议细节，不是声称已实现代码存在这些缺陷。

### Q01 — P1：无启动许可的占位 run 缺少明确结算路径

- 定位: design.md:46、119、123、129、131、135。
- 场景: promote 已创建 prepared run 和 engine task，Stop/撤权在 launch permit 前提交。session 已 paused；claim/permit 被暂停挡住，Resume 又要求没有未结 run。如果只写 stop intention 等 daemon 回执，这条从未收到执行的 run 无人结算，永久挡住恢复。若保留该 command 等待再执行，又与 command_id 唯一 run 约束、禁止复活终态发生冲突。
- 最小修订: 明确 promote 成功即把选中的 command 设为当前 in_flight；Stop 对这个当前 command 的无许可 run，在同一源事务撤销所有派发身份、写 never_launched_proven、结算为 stopped 并清屏障，只有尚未选中的 waiting 指令保留。移除作者/整单关闭则使用相应 cancelled 原因。若希望保留尚未启动的当前 command，须另给不违反唯一 run 约束的状态模型，不能两种语义混用。
- 同时规定 claim 构建失败、runtime 在派发前消失、能力检查失败等无许可路径的结算责任；有过许可或提交结果未知时不得套此快捷路径。
- 验证: 用事务 barrier 卡在 promote 后、permit 前，再 Stop/撤权；无需 daemon 领取即可得可信未启动终态，恢复可执行下一条，延迟派发不能启动旧 run。

### Q02 — P1：持久账本尚缺 fork 窗口与终结 seal 的单调状态机

- 定位: design.md:53、133、135、152。
- 场景: “fork 前写身份 + 本地锁”本身不能证明重启不重复 fork。若记录仍为 prepared，进程已 fork、PID 回写前崩溃，新 daemon 可能把 prepared 当未启动再 fork。另有内部 fallback 时，“全部尝试已回收”只是一张瞬时快照；如果证明生成后仍允许创建下一内部尝试，服务器会放行另一 command。
- 最小修订: 写清跨 daemon 进程共用的 OS 锁及账本状态：身份持久化 → nonce 已消费/可能启动 → 进程身份已记录 → 永久封闭启动 → 退出证明。使用私有原子写和 fsync；可能 fork 前必须 durable burn nonce。崩溃落在 burn 与进程身份记录之间时保守 unknown，不能因查不到 PID 重跑。
- 最终 proof 和 never_started 墓碑必须在同一 launch 锁内先 seal 整个 run，禁止所有后续内部尝试，再落盘/发送。每次内部尝试要有单调 attempt 身份；旧尝试 proof 不能代表整个 run 已封闭。账本完整但启动阶段不确定也需恢复规则，不能仅覆盖“账本丢失”。
- 依据: 现有 cancel_receipts.go:301 用已观察尝试计数形成证明；本次新增 supervisor 不可仅照搬计数。现有 proc_windows.go:110 还有 fail-open 降级，设计已正确要求共享启动拒绝，需纳入同一状态机。
- 验证: fake 子进程分别在 burn 后/fork 后/PID 落盘前/最后尝试结束后退出 daemon；重复 claim、另一 daemon 实例和迟到 fallback 均不得新增 fork。

### Q03 — P1：新 purpose 的 admission 谓词需显式连接来源 issue 和执行 installation

- 定位: design.md:30、131、164、177。
- 场景: 新 engine task 的 issue_id 为空，旧 lock_issue_execution 不会替它锁定或检查来源 issue。D06 列出开放状态类别，但没有明确正式 admission_status；D08 写“获取 installation admission 前置锁”，锁本身也不等于拒绝 stopped installation。独立 claim 路由不能假定普通 ClaimAgentTask 的 SQL 条件仍自动生效。
- 最小修订: 定义唯一 CanAdmitCollaborationRun 谓词：source issue 存在且 admission_status 为 not_required/accepted；新建初始化为正式任务，禁止旧 triage 更改。新领取/首次 permit 必须满足当前 installation admission=accepting、runtime 绑定/所有者/健康条件，或明确合法的非托管分支。installation 停止期间，原 run 可按已有许可恢复观测/终结，但不可把旧“恢复已接纳执行”例外解释为新的启动许可。
- 该谓词在 source/执行空间的锁内重查，不能只靠预读和注册 capability。补一张每个入口使用的实际锁 helper/NOWAIT 对象表；termination proof 必须单列为无需当前 user/runtime authority 的历史身份路径，teardown 须在既有 runtime/agent 锁前完成源会话收尾，或采用明确的 NOWAIT 回滚协议，不能持旧资源锁再等待源 session。
- 依据: server/migrations/535_triage_execution_fence.up.sql:12；server/pkg/db/queries/agent.sql:819；server/internal/service/managed_runtime.go:123；runtime_teardown.go:62。现有规范要求 admission 覆盖所有 enqueue/claim/start；空 issue_id 的新载体需要显式补接。
- 验证: source admission 非正式/缺失、installation stopped、runtime 重绑定/删除与 permit 竞争；同一测试证明旧恢复入口不能绕过新谓词，并证明反向 teardown 锁竞争能有界回滚。

### Q04 — P1：维护冻结不能只逐个 paused，恢复请求会重新开放调度

- 定位: design.md:123、230、232、233。
- 场景: 关闭创建后逐个暂停 session，仍有权限的客户端同时 Resume，或对先前 paused 的 session 执行 Resume；D06 当前允许它再次 active。因而 D11 的“持久暂停各 session”无法单独形成可靠的部署冻结，恢复版本后可能直接派发该 session。
- 最小修订: 明确维护期间有所有可写节点共同遵守的持久 admission gate，优先于 session.active；冻结时先关该 gate，再暂停会话。Resume、promote、claim、finalize、permit 均检查；Stop、proof、历史读取和审计保持可用。解除维护只恢复接受新显式控制，不自动把 session 改 active。也可采用明确停止全部执行 writer 的维护方案，但须写清对仍运行 API 的 Resume 拒绝与再启动规则。
- 验证: 在冻结扫描期间持续并发 Resume/入队/claim，确认没有新许可，随后恢复部署也不自动恢复旧暂停会话。

### Q05 — P2：退出 proof 与业务 outcome 乱序时的结算规则未闭合

- 定位: design.md:47、53、127、137、154。
- 场景: 最小退出 proof 先到，而业务 complete/fail 回传丢失或已被撤权拒绝；或者已记录 failure 但进程迟迟未退出。现在“proof 只结算 run”和“失败在证明后记 failed”未说明 command 如何定态、何时显示业务失败，以及是否等待永远不能上传的正文。
- 最小修订: 分开持久 business_outcome 与 execution proof；业务失败可先记录/展示，settled_at 只能由可信 proof 驱动。为 proof-before-outcome、重复/迟到 outcome 定义确定的 reducer 和原因优先级（Stop/关闭/撤权不可被 late success 覆盖）。如果缺少 outcome，按明确的无内容结算原因标 incomplete/failed，或定义有界等待，不能无限阻塞已确认结束的 run，也不能把 exit 自动当成功。
- proof 可携带允许表中的最小终结枚举/尝试边界，但不能趁机加入正文、附件或任意结果；日志缺失独立记 incomplete。
- 验证: proof/result 两种到达顺序、撤权夹在两者之间、failure 先到但 proof 永不到；后续 FIFO 与停止状态均不被错误释放或覆盖。

### 已覆盖、不建议重新扩大范围

- R39 只新建已在 design.md:32、230 明确，不再要求本轮实施存量 quiesce 迁移。
- v1 无服务端隐式失败重试与 command_id 唯一一致；不要恢复旧 MaybeRetryFailedTask 路径。
- 终结 token 与业务 token 分离、撤权后只保留已记录内容、清理 engine task 不删源屏障方向正确。
- 普通任务保持原语义；新协作 purpose 需完整排除旧 orphan/retry/quick-create selectors。
- O11 的管理员接手分支没有授权，不实现附带转交；本审阅不改变此边界。

### 文件与规范依据

- design.md：本轮设计全文（236 行），核心定位见各条。
- research/shared-queue-runtime-design.md：此前源码研究；本次不重复全量扫描。
- server/internal/service/managed_runtime.go、runtime_teardown.go、managed_installation.go：定点核对现有锁入口。
- .trellis/spec/server/triage.md：所有执行边界的持久 admission。
- .trellis/spec/server/managed-installations.md：原绑定、代次、退出证明、锁序与旧权限不得降级。
- CLAUDE.md、.trellis/workflow.md：当前仅规划、无新 FK、并发索引、禁止未经授权运行真实 CLI。

## Caveats / Not Found

- 本文件为独立设计审阅，未改 design.md，也未运行产品、数据库或真实进程测试；所有验证项均为待执行场景。
- 没有发现需要放宽 R17 或扩展 R39 的理由。上述缺口应由主会话补为协议/事务契约，再进入实施评审。
- 非 managed daemon 的 installation/auth epoch 来源与认证兼容矩阵由授权审阅者负责，已协调避免重复；不能默认 NULL/0 代表已验证身份。
- 外部参考：未访问网络；使用既有仓库证据，不声称新的平台/供应商能力已获验证。

## 修订复核：2026-10-12

仅复核 Q01–Q05；依据 research/execution-contract.md E01–E06、design.md D06/D08/D11 和 implement.md P4/P5/P8。原发现保留为审阅历史，以下为当前结论。

- Q01 已关闭：E03 定义 promote 后即为当前 in_flight；从未许可时服务器原子撤派发、结算并清屏障，不等待 daemon；有过许可/未知提交不走快捷路径。
- Q02 已关闭：E04 明确跨进程锁、fsync、fork 前 nonce burn、崩溃 unknown、独立 attempt 许可及永久 seal 后才形成最终 proof。
- Q03 已关闭：E01 显式检查 source 正式 admission、installation accepting、当前资源与部署 gate；E02 分列 writer 锁序、teardown NOWAIT 和不依赖当前权限的历史 proof 路径。
- Q04 已关闭：execution-contract.md:15、40、103 明确所有 Resume/active 写入持 gate FOR SHARE 至提交；:105 要求请求比较 expected_execution_epoch，freeze/enable 均推进 epoch，迟到旧意图拒绝、旧 key 只重放收据。:109 禁止解除 gate 自动恢复。design.md:191、228 和 implement.md:117 已同步；协议 :151 包含读 gate→active 提交竞争及重新 enable 后旧请求测试。
- Q05 已关闭：E05 分开 outcome/forced disposition/proof；结果先到不清屏障，proof 先到无结果以 outcome_unconfirmed 结算，强制停止优先，未上传内容仅标不完整。

上述五项在设计层未发现遗留直接矛盾。此结论不表示实现/测试通过，也不覆盖全量架构或其他审阅项；未运行测试、未修改主文档。O11 仍未决，不授权管理员附带转交。
