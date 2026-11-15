# S3 执行计划

- [x] 读取父矩阵、恢复规范与源码，拆出独立timeout-proof子增量。
- [x] 当前source service回归29项通过，无fail/skip（独立DB；附件、通知、records、outbox子合同）。
- [x] timeout-proof完成独立检查、本地Lore提交和归档；仅核销所覆盖子断言（TC14的受控超时/回执恢复/Stop竞争/显式重试；重复附件上传与真实模型超时未覆盖）。
- [x] 独立检查 v2 修复合同及 `research/recovery-followup-wire-supplement-r05.md`，C01/C02 规划问题已关闭；`research/recovery-followup-v2-wire-review-r05.md/json` 为 PLANNING PASS，原失败与未测范围保留。
- [ ] 绑定最终已接受的 S2 source/锁 helpers 和验收报告；当前 intermediate 或 r03 待验证候选不能替代此前置。同步该最终引用和本次规划 review 到 implement/check jsonl 后激活 S3。
- [ ] 在S2最终接口稳定后评审priority/handler竞争、55评论补读、混合提及与撤权组合测试增量，再由trellis-implement实施。
- [ ] 评审并实施完整阻塞四项及旧请求恢复；与S2稳定责任/阶段启用协同，避免双方同时改handoff/workflow共享文件。
- [ ] 新DB验证Go normal/race、Core/View、production Web/Desktop；记录原请求、失败与完整批次，修复真实复现问题。
- [ ] 独立trellis-check核对S3全范围，更新父24例与缺口；完整S3通过才归档，不继承S1旧环境结论。

API、构建和SQL执行由主会话分配；实现子代理只用授予的具体数据库和package，不调用真实模型或真实资金，不改其他任务未提交文件。

## 后续实施归属与验收顺序

1. 先在最终 S2 输入上由独立测试 owner 补 priority/handler 两提交顺序与普通 child 的显式 numeric CAS；authority-first 的 403 与 stale revision 的 409 分别验证，旧 optional revision 行为不改成全客户端强制。
2. 单一 server owner 接续 shared handoff/workflow/lifecycle/creation、原事务 SQL 和对应 handler。按通过的 V2-01–18 保留真实 prior status、directed 原键恢复、pending-safe return、required-write 原子回滚、精确 affected count及不可变 repair link；不得新建平行恢复 ledger。
3. 先冻结 public DTO/read/query，再让 Core/Views owner 实施共享 schema、known denial/current actionability、block/repair form和原请求恢复。history 预算、MAC/current key、nullable source-only authority与closed repair-only empty default必须在同层覆盖，DOM不重跑纯解析矩阵。
4. 主会话核对实际下一空闲迁移前缀、DDL/hash/无FK/独立CONCURRENTLY索引与retention down guard，分配normal/race包数据库；所有旧失败库和timeout-proof档案保留。
5. 单一 E2E owner 复用现有部署/host/native设施，组合55评论跨页和真实WS重认证、原键上传恢复、混合人员/Agent提及原事件去重、撤权及blocking/repair历史。runtime v1缺structured block元数据、未支持的run-local输出上传、真实模型/安装版Main范围分别记录，不以受控结果填PASS。
6. 新接入的source必须绑定匹配API/CLI、production Web和Desktop构建，独立check逐条核R01–R09与V2/wire、名称/完整分页分母/原失败/所有实际入口、无新调度和gate/process恢复。只在S3自身所需证据齐备后关闭；父24例仍逐条更新。
