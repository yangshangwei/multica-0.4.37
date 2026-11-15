# PAY-241 开放缺口

原24例没有整体通过结论。历史PAY-1、S1、当前单元/协议测试与真实集成证据分开记录；test-matrix.csv为逐项当前账本。

| 缺口 | 当前事实 | 后续归属 |
| --- | --- | --- |
| G-S2-ROLE | 1b/2a及后续修复已独立通过，包括代码评审、原签字凭据/最终到期、成员移除再加入、A5成员锁与版本失效。2b的S2B-C01及2c执行/指标、后续S2C-C01/C02均已独立关闭；2d全writer门禁由同一owner继续。完整阶段验收未完成 | S2实现及检查 |
| G-S2-CONTRACT | 客户端3c/3d已独立通过，保留R1拒绝隔离、fact唯一性、旧路由404/405与候选分页修复；最终Core550/Views112。E2E4编写13纯测试/类型/lint及4场景发现通过，实际场景0。所有真实writer门禁接入前政策启用保持关闭；A01–A12及production Web/Desktop联合验收未完成 | S2实现及检查 |
| G-S2-FINAL-WRITERS | r06 完整代码检查已接受 W01–W18/Q01–Q12，normal/race 各116个顶层、321条命名、263个叶记录通过，无当前代码阻挡项。原 r03/r05 失败保留。r07 私有 GREEN 仅改策略入口及两份既有测试，正在18个未用专库验证真实 readiness、启停、缺项关闭/恢复和 M1 已消费历史。LIVE P2、四个实际 Web/Desktop 场景和 A01–A12 仍未接受 | S2策略启停及实际联合验收 |
| G-S2-REPORT-ACL | 已关闭：S2B-C01/P1修复复用原冻结制品的M1读取权限与允许权限锁的RR事务，在cursor/分页/空页前检查，QA可选引用同样受保护；有权历史读取及reason-only退回保留。独立4,894文件重建、service23/ACLrace13/handler44及静态检查通过，原失败和两种真实撤权顺序保留。见S2 research/server-increment-2b-acl-fix-check.md；局部关闭不等于S2验收通过 | S2增量已独立关闭 |
| G-S2-ISSUER-ACL | 已独立关闭：2c原代码评审人失去制品manager权限后，当前consumer不能替代原issuer权限消费批准；R4传递release/rollback状态同步失效。原始issuer-artifact-red-1与旧独立检查发现保留，见S2 research/server-increment-2c-followup-check.md。跨issuer完成消费的2d实际writer矩阵仍待验证 | S2 2c增量已独立关闭 |
| G-S3-TIMEOUT | 已由timeout-proof增量关闭受控范围：15秒执行deadline、ACK丢失后跨进程原回执恢复、Stop两种顺序、显式重试保留旧失败，T01–T06独立检查通过。仍开放：Agent输出附件上传幂等、迟到结果、真实模型超时、安装版Desktop Main | S3其余范围 |
| G-S3-RECOVERY | 批次补读、优先级/处理人指定字段竞争、混合提及通知完整组合矩阵尚未执行 | S3 |
| G-S3-BLOCK | 信息往返缺解除条件/预计时间必填；block责任人与S2稳定业务责任人不能混用 | S3与S2对齐 |
| G-S4-ENV | 未提供支付/履约沙箱、真实仓库及非生产发布/回滚目标。根配置不足不表示遍历所有账号；真实用例NOT_RUN | S4真实接入 |
| G-S4-EVIDENCE | 接入清单及2026-11-14补充说明已保存；S2 2a的GitHub旧回调、同时间不同head、未知check节点和派生链接本地修复已独立通过。VCS仍缺仓库/运行身份；真实E01仍需真实仓库重投/乱序证据。回滚指标、稳定物理目标冲突键及一次性执行合同已冻结，2c服务端实施中；真实E01–E07仍全部NOT_RUN | S2证据门禁 / S4真实来源 |
| G-S5-SAMPLE | r01在S2前源码上完成10000传播样本（pooled p95 783ms），回写71个正式run成功、第72个中止、28个NOT_RUN，完整分母100保留。心跳与100socket负载契约修复已通过独立检查（31项工具测试），r02新库/新目录完成100账号prepare、10000传播样本（pooled p95 702.579ms）及100次独立run回写（业务130.694ms、终结724.774ms），独立79项检查通过且资源已恢复。最终候选正式验收仍未运行，TC-24保持NOT_RUN | S5 |
| G-CHECK-BASELINE | S2的16个并发索引重试注册已修复并独立验证；全局迁移注册检查仍报8个既有F2 `report_snapshot*` 缺项。保留原失败及最新仅剩F2的结果，不改全局断言或将其混入S2提交；这不是24例业务验收的通过或失败计数 | 宽检查限制 / F2原范围 |
| G-FINAL | 产品/研发/QA/项目经理业务签字及全部P0/P1证据未齐，测试账号不能代替 | 父任务 |

基线：feb87aee7，独立service 50/handler 31个named测试通过，无skip，限于既有M1/交付/原请求和当前授权的选定子断言；不消除新增功能缺口或证明真实支付、CI、发布及模型质量。
