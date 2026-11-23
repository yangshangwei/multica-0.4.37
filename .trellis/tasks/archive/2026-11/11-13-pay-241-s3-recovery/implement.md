# S3 执行计划

2026-11-19 晚续接：Agent 输出已提交 `89740ed14`；普通 UI CAS 代码 280 项及完整 GREEN R02 两端 2 顶层/6 子场景独立接受。21 个所属产品/测试/E2E 路径已精准整合，主树全量 typecheck/lint 与最终 R01–R09 核对进行中。TC-16 更新 PASS，父矩阵 **7/10/7**。当前顺序以[收尾推进计划](../../../../../docs/plans/2026-11-19-pay-241-s3-closeout.md)为准，以下旧检查点保留。

续接见[当前推进计划](../../../../../docs/plans/2026-11-19-pay-241-continuation.md)。字段并发、55 评论恢复、人工通知/上传/撤权和结构化阻塞/解除已执行。普通评论同事件去重已提交，普通订阅 R05、受控 Extras-B R03 均已独立接受；当前父矩阵 6 PASS /11 PARTIAL /7 NOT_RUN，TC-21/22 本轮通过。当前受控 L1/L2 已由原 ACK/改变及新增迟到拒绝、十二组核账/旧事实与零恢复启动完成映射。后续按[剩余验收映射](research/s3-remaining-acceptance-map-20261119.md)处理 Agent 输出附件及普通 UI 版本竞争，再完成 R09。下列首增量及 R01/R02 为历史检查点。

### 2026-11-19 输出附件续接检查表

- [x] 读取根约定、完整交接、jsonl → PRD → design → implement、当前计划与剩余映射；恢复现有任务指针。
- [x] 保存 HEAD `699d510b9`、空暂存区与 513 条既有 dirty entries 的逐路径哈希；其他 WIP 和原失败保持。
- [x] 定位缺失的 Agent 输出 writer，记录单文件 multipart/真实 Agent 归属、原 comment event 与 server-owned outcome 引用的最小设计。
- [x] 分配新私有 source/config/state 根并核对 5076 文件来源、六项测试差异、Git 外可信状态链与集群连接预算。
- [x] 唯一 server/runtime Trellis implement owner 已保留缺失入口 RED，并在私有副本实施输出 writer、迁移/SQL、原评论事务复用、outcome 引用及 CLI；保持每个原失败。
- [x] 主会话已为新 schema、service/handler/router/CLI 和 normal/race 分配具体专库与精确命令；独立检查接受每模式 82 顶层/203 命名、合计 406 命名 PASS，Core/Views 222 命名 PASS；O01–O09、原控制及 catalog/gate/连接核对完成。
- [x] 独立 Trellis check 已接受最小合同、代码/功能及完整输出 R03；43 个所属路径精确整合 ROOT，既有文件模式和其他 WIP 保持。六项既有测试 source/mode/presence 差异保留；ROOT 编译/type/lint 兼容检查另行核对。
- [x] 独立 E2E owner 复用实际受控 Provider/daemon/CLI，完整输出 R03 在 production Web/Desktop 通过 2 顶层/16 子场景并独立接受；主会话已核对构建/源/配置和所有自有进程、原 gate/零连接。
- [ ] 输出增量独立接受后再补普通 priority/assignee UI numeric revision 竞争，保留旧无版本 API 控制并按实际证据决定 TC-16。
- [ ] 最终 R01–R09 来源/配置/程序/客户端/资源与接受证据逐项核对；满足 S3 自身验收后才定向提交及归档，S4/S5 仍独立。

2026-11-19 输出运行 R01 保留为整轮未接受：两个顶层失败，子场景 5 PASS / 1 FAIL / 10 NOT_RUN。Web 的本轮构建配置遗漏 `/ws`，进入输出前未取得订阅；Desktop 的 Stop 原回执已由既有 schema 解析，重放原始 JSON 多出 `proposal_id: null`。三个实际 Provider、四个输出 CLI 调用、三个 Agent 文件及两个已记录结果留存，不能拼接为整轮通过。所有自有服务正常停止，原 gate 布尔值恢复、epoch 2→6，856 条迁移/663 个索引保持，连接归零。详情见 [续接检查点 R02](research/output-continuation-checkpoint-r02-20261119.md)。下一完整轮使用新来源、配置、专库与上传目录；Go/共享产品字节未变的通过证据继续复用。

随后输出 R02 的 16 子场景均通过，但两个顶层因末尾错误的 permit HTTP 计数保持失败；每个原运行在准备前/启动前两次授权，原 token/attempt 不变。仅修正严格的 5 原身份/10 HTTP 配对核账后，新来源/配置/专库的 [完整 R03](research/agent-output-runtime-r03-independent-check-20261119.md) 单轮通过并独立接受，阻挡项 0：10 Provider、12 输出 CLI、10 Agent 文件/20 引用、6 人工输入文件、4 已记录结果、0 恢复启动。16 加密对象和 26 总引用全部保留，原凭据 42 写拒绝/14 读拒绝核对；自有服务停止、gate 2→6、连接归零。R01/R02 原失败未改写。当前进入普通 UI numeric revision 竞争，完整 S3 及 R09 尚未关闭。

### 2026-11-19 本轮检查表

- [x] 复核父24例计数、R08后端产品三文件与ROOT一致；恢复已有S3指针，另保存当前工作树基线。
- [x] 落盘普通评论原事件去重的最小设计和本轮有序计划；已有来源研究、Extras-B readiness作为实施依据。
- [x] 新 `notification_comment_replay_test.go` 在独立新专库取得同原事件顺序/并发重投RED；同修正测试源码的完整RED保留23失败，旧28控制通过。
- [x] 专用inbox查询/稳定交付ID的最小修复通过新专库normal/race各35顶层/57命名、相关旧通知控制、格式/ROOT编译/vet/sqlc和独立检查；仅普通评论事件范围。
- [x] 新双端普通订阅spec/fixture通过静态检查及实际新库、生产Web/匹配Electron验收，按原收件人/通知/评论ID核账；R05 完整 2/2、零重试独立接受，阻挡项 0。
- [x] 完成受控daemon/Provider/CLI和实际R08客户端的来源/配置/运行绑定；新库单轮Extras-B R03 完整4顶层/10子场景、10Provider/12原输入、零重试独立接受，阻挡项 0。
- [x] 更新已覆盖的TC-14/20证据及定义场景TC-21/22 PASS、缺口、进度和spec，保留全部原失败；完整S3仍按R01–R09独立检查。

2026-11-18 续接：最终 S2 R33 已接受，来源见 `research/s3-source-binding-20261118.md`。本轮首增量为独立研究通过的两个新测试文件，复用既有真实 PostgreSQL 屏障与原键恢复；完整 S3 仍按下列余项验收。

- [x] 读取父矩阵、恢复规范与源码，拆出独立timeout-proof子增量。
- [x] 当前source service回归29项通过，无fail/skip（独立DB；附件、通知、records、outbox子合同）。
- [x] timeout-proof完成独立检查、本地Lore提交和归档；仅核销所覆盖子断言（TC14的受控超时/回执恢复/Stop竞争/显式重试；重复附件上传与真实模型超时未覆盖）。
- [x] 独立检查 v2 修复合同及 `research/recovery-followup-wire-supplement-r05.md`，C01/C02 规划问题已关闭；`research/recovery-followup-v2-wire-review-r05.md/json` 为 PLANNING PASS，原失败与未测范围保留。
- [x] 绑定最终已接受的 S2 R33 source/锁 helpers 和全范围验收报告；5,056 文件哈希/模式核对无漂移，既有相关 ROOT 字节匹配。最终引用与 v2/r05 review 同步到 implement/check jsonl 后激活既有 S3。
- [x] 评审首个 priority/handler 与普通关联 child 显式 CAS 测试边界，复用现有屏障与 testutil；见父 `research/s3-cas-intake-20261118.md`。
- [x] 首增量：`collaboration_recovery_cas_test.go` 两个 service 顶层和 `collaboration_child_recovery_cas_test.go` 一个 handler 顶层，四新专库完成 normal/race、格式/编译/vet和独立检查；保留原请求成功/拒绝及正常权威顺序。
- [x] 双端 55 评论补读、混合提及与人工撤权组合已实施；批次为 2/2，Extras-A 单轮 6/6 顶层、18/18 子场景。原失败保留，实际运行凭据撤权另验。
- [x] 阻塞四项、旧请求恢复及与阶段策略的衔接完成；[R08](research/block-runtime-r08-20261118.md)保留了双端、原回执和业务历史核对。
- [ ] 新DB验证Go normal/race、Core/View、production Web/Desktop；记录原请求、失败与完整批次，修复真实复现问题。
- [ ] 独立trellis-check核对S3全范围，更新父24例与缺口；完整S3通过才归档，不继承S1旧环境结论。

API、构建和SQL执行由主会话分配；实现子代理只用授予的具体数据库和package，不调用真实模型或真实资金，不改其他任务未提交文件。

## 本轮首增量的执行与证明

1. service owner 只新增 `server/internal/service/collaboration_recovery_cas_test.go`，实现 `TestCollaborationRecoveryPriorityConcurrentIdentity` 和 `TestCollaborationRecoveryPriorityTransferCommitOrders`。同键同 priority 收敛一次，同键异 priority 拒绝；不同键同 revision 精确核获胜字段、事件和原回执。
2. handler owner 只新增 `server/internal/handler/collaboration_child_recovery_cas_test.go`，实现 `TestCollaborationOrdinaryChildPriorityAssigneeCASCommitOrders`。真实关联 child 用 numeric `expected_revision`；两顺序各一次200/一次409，败方字段不变，fresh 请求应用剩余字段，父协作状态与执行保持。
3. 真实 commit/PID 屏障必须在任何失败路径可释放并收齐两个请求。转交先提交的旧非创建人 handler 修改是403 forbidden；优先级先提交的旧 revision 转交是409 stale_revision。失去写权后的旧结果用 `GetWorkflowOperation` 恢复，不重发写请求。
4. 主会话使用私有配置逐个运行下面的精确命令，service/handler及normal/race分库；配置先核实际库名和完整迁移，不继承账户凭证或API专用pool/publisher设置。normal/race的名称集合、每条run/terminal、零skip/失败/重复/缺失、源码/配置前后哈希与gate/连接恢复均入报告。

2026-11-18 实测补充：service normal/race 的7名各已通过；handler normal 的3名全通过但最终 invocation-target 清理计数为1，整轮不接受。独立零测试体 TestMain RED 复现同一遗漏。已写清理计划并将唯一 handler owner 扩到已有 `handler_test.go` 的退出清理函数；修正后单独封存R04、TestMain-only GREEN及两个fresh handler normal/race。原R03失败保留，未运行的原handler race不消费；不能在子测试里删除仍供其他测试使用的共享种子。详见 `research/testmain-cleanup-diagnosis-20261118.md`。

```text
go test -json -count=1 -run '^TestCollaborationRecoveryPriority(ConcurrentIdentity|TransferCommitOrders)$' ./internal/service
go test -json -race -count=1 -run '^TestCollaborationRecoveryPriority(ConcurrentIdentity|TransferCommitOrders)$' ./internal/service
go test -json -count=1 -run '^TestCollaborationOrdinaryChildPriorityAssigneeCASCommitOrders$' ./internal/handler
go test -json -race -count=1 -run '^TestCollaborationOrdinaryChildPriorityAssigneeCASCommitOrders$' ./internal/handler
```

每个命令只由绑定私有source/config的执行器进入。测试过滤不代表 TestMain 隔离。结果仅为 S3 R04 的 service/HTTP 增量；API/Web/Desktop 与全部 S3 验收完成前，不将 TC-16 判 PASS。

## 后续实施归属与验收顺序

1. 先在最终 S2 输入上由独立测试 owner 补 priority/handler 两提交顺序与普通 child 的显式 numeric CAS；authority-first 的 403 与 stale revision 的 409 分别验证，旧 optional revision 行为不改成全客户端强制。
2. 单一 server owner 接续 shared handoff/workflow/lifecycle/creation、原事务 SQL 和对应 handler。按通过的 V2-01–18 保留真实 prior status、directed 原键恢复、pending-safe return、required-write 原子回滚、精确 affected count及不可变 repair link；不得新建平行恢复 ledger。
3. 先冻结 public DTO/read/query，再让 Core/Views owner 实施共享 schema、known denial/current actionability、block/repair form和原请求恢复。history 预算、MAC/current key、nullable source-only authority与closed repair-only empty default必须在同层覆盖，DOM不重跑纯解析矩阵。
4. 主会话核对实际下一空闲迁移前缀、DDL/hash/无FK/独立CONCURRENTLY索引与retention down guard，分配normal/race包数据库；所有旧失败库和timeout-proof档案保留。
5. 单一 E2E owner 复用现有部署/host/native设施，组合55评论跨页和真实WS重认证、原键上传恢复、混合人员/Agent提及原事件去重、撤权及blocking/repair历史。runtime v1缺structured block元数据、未支持的run-local输出上传、真实模型/安装版Main范围分别记录，不以受控结果填PASS。
6. 新接入的source必须绑定匹配API/CLI、production Web和Desktop构建，独立check逐条核R01–R09与V2/wire、名称/完整分页分母/原失败/所有实际入口、无新调度和gate/process恢复。只在S3自身所需证据齐备后关闭；父24例仍逐条更新。

## 首增量当前结论

2026-11-18 `research/cas-independent-check-20261118.md/json` 已独立接受，阻挡项0。两新测试和共享 TestMain 五行退出清理已精准整合 ROOT；service/handler 普通与竞态每模式7/3命名通过，ROOT 编译/vet/格式和30类关联记录/连接恢复通过。TC-16仅PARTIAL；当前API/Web/Desktop组合及全S3验收仍开放，下一步55评论/WS批次补读与同版本客户端竞争。

## Current-client field acceptance R06 — 2026-11-18

The complete fresh password API/production Web/matching Electron four-case field run is independently accepted:4tops/8orderings, retry0, source/config/raw/persistent IDs and owned gate/PID/native closure reconciled. Original operations and both current-authority403/stale409 results remain exact through fresh authorized continuation. Ordinary numeric API CAS is separately labelled from actual optional-version UI writes. TC-16 remainsPARTIAL; parent24 stays2PASS/12PARTIAL/10NOT_RUN.

Existing notification/upload/revocation/proven-result contracts were revalidated on four separate normal/race databases: service13tops/18names each, handler3 each,42named executions;30business relations0, protected history retained, originalgate restored andconn0. This does not accept current extra/browser/runtime compositions. Firstfive client failures, native noauth transport probe and all prior failures remain retained. Next: separate fresh55-comment/realWS profile, extras and structured blocking/repair. Blockserver R01's first16DBtops/43names has3failingtopcontracts; candidate/source/DB/raw frozen before R02 repairs. FullS3 remainsinprogress and unarchived.

## 55-comment current-client batch accepted — 2026-11-18

Fresh R01 Web/Desktop batch passes2tops/2batches, retry0:55originals+2controls each,114persistentIDs/110batch4controls,128events/126operations, all original receipt keys retained. Each platform completes two actual50+7 pagesets and two physical reconnects with3authenticated sockets plus1retained failed offline attempt. Exact persisted microsecond ordering, UI thread order, edited/tombstone/reply versions, final state and zero observer mutations/run/command/dispatch are independently reconciled. Owned API75246/native exit and actualgateepoch2/853ledger/663indexes/211tablecounts/conn0 are checked; Web/renderer remain held. Report research/comment-batch-r01-independent-check-20261118.md/json. TC15PASS; current local parent24=3PASS/12PARTIAL/9NOT_RUN; TC16PARTIAL and fullS3open. Fieldincrementcommit237b172a6 is separate.

Next: Extras-A current notification/upload/human revoke (six tops/eighteen subcases), separately authorized actual runtime Extras-B, and full v2/r05 block/repair. BlockR02 service normal/race each43functionalnames passed but5orphaned test handoffs makeprofilesNOT_ACCEPTED; handlernormal's missingChiid fixture failure retained. NewR03 two-test-file cleanup/context repair precedes fresh verification. No archived/fullS3 claim.
