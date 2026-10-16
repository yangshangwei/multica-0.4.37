# 执行计划

- [x] 用户批准推荐范围和执行；建立隔离工作区并创建任务。
- [x] 固定源/目标提交，保存评估与六组文件清单。
- [x] 审阅计划/context，激活任务并提交计划。
- [ ] 实现代理依research/slices.json应用净补丁，每步记录staged tree；不提交。
- [ ] 安装锁定依赖，生成必要MDX/types。
- [ ] 全仓lint/typecheck、UI exports、图片预算、diff check。
- [ ] 全仓TS测试，下载/seed脚本、Go内置文档及生产Web/相关E2E。
- [ ] 独立Trellis check核查范围和client定向移植；修复并复验。
- [ ] 主会话按功能树生成六个Lore提交，记录Source/Tested/Not-tested。
- [ ] 保存验证摘要，--ff-only合入本地main，校验源码树一致。
- [ ] 归档任务，记录journal，提交收尾；检查源分支和main。

验证针对隔离工作区，测试数据库/服务独占，不修改原.env或使用原服务。生产Web使用build+start，不并发写.next。旧完整分支的测试不替代当前证据。无需真实AI、外部凭据、部署或推送。
