# S3 执行计划

- [x] 读取父矩阵、恢复规范与源码，拆出独立timeout-proof子增量。
- [x] 当前source service回归29项通过，无fail/skip（独立DB；附件、通知、records、outbox子合同）。
- [x] timeout-proof完成独立检查、本地Lore提交和归档；仅核销所覆盖子断言（TC14的受控超时/回执恢复/Stop竞争/显式重试；重复附件上传与真实模型超时未覆盖）。
- [ ] 在S2最终接口稳定后评审priority/handler竞争、55评论补读、混合提及与撤权组合测试增量，再由trellis-implement实施。
- [ ] 评审并实施完整阻塞四项及旧请求恢复；与S2稳定责任/阶段启用协同，避免双方同时改handoff/workflow共享文件。
- [ ] 新DB验证Go normal/race、Core/View、production Web/Desktop；记录原请求、失败与完整批次，修复真实复现问题。
- [ ] 独立trellis-check核对S3全范围，更新父24例与缺口；完整S3通过才归档，不继承S1旧环境结论。

API、构建和SQL执行由主会话分配；实现子代理只用授予的具体数据库和package，不调用真实模型或真实资金，不改其他任务未提交文件。
