# S3 执行计划

当前客户端续接计划：`research/current-clients-plan-20261118.md`。主会话已重新指向现有S3，保存共享工作树逐文件归属/哈希基线，并确认已接受R04的5,058文件无漂移、ROOT生产路径匹配；四个测试差异和两条私有诊断缺失保留。先由唯一E2E owner补当前password/API、production Web和匹配Electron的字段并发/原回执组合，再做55评论分页与真实WS重认证，随后通知/上传/撤权和v2/r05阻塞四项。环境分配、执行、整合、矩阵与逐增量提交由主会话负责。

2026-11-18 续接：最终 S2 R33 已接受，来源见 `research/s3-source-binding-20261118.md`。本轮首增量为独立研究通过的两个新测试文件，复用既有真实 PostgreSQL 屏障与原键恢复；完整 S3 仍按下列余项验收。

- [x] 读取父矩阵、恢复规范与源码，拆出独立timeout-proof子增量。
- [x] 当前source service回归29项通过，无fail/skip（独立DB；附件、通知、records、outbox子合同）。
- [x] timeout-proof完成独立检查、本地Lore提交和归档；仅核销所覆盖子断言（TC14的受控超时/回执恢复/Stop竞争/显式重试；重复附件上传与真实模型超时未覆盖）。
- [x] 独立检查 v2 修复合同及 `research/recovery-followup-wire-supplement-r05.md`，C01/C02 规划问题已关闭；`research/recovery-followup-v2-wire-review-r05.md/json` 为 PLANNING PASS，原失败与未测范围保留。
- [x] 绑定最终已接受的 S2 R33 source/锁 helpers 和全范围验收报告；5,056 文件哈希/模式核对无漂移，既有相关 ROOT 字节匹配。最终引用与 v2/r05 review 同步到 implement/check jsonl 后激活既有 S3。
- [x] 评审首个 priority/handler 与普通关联 child 显式 CAS 测试边界，复用现有屏障与 testutil；见父 `research/s3-cas-intake-20261118.md`。
- [x] 首增量：`collaboration_recovery_cas_test.go` 两个 service 顶层和 `collaboration_child_recovery_cas_test.go` 一个 handler 顶层，四新专库完成 normal/race、格式/编译/vet和独立检查；保留原请求成功/拒绝及正常权威顺序。
- [ ] 在S2最终接口稳定后评审55评论补读、混合提及与撤权组合测试增量，再由trellis-implement实施。
- [ ] 评审并实施完整阻塞四项及旧请求恢复；与S2稳定责任/阶段启用协同，避免双方同时改handoff/workflow共享文件。
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
