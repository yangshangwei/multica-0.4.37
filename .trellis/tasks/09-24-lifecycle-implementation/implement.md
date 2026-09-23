# 实施计划

1. 核对已有 RCA、角色、skill、模板测试和 workspace copy protection，保留真实限制证据。
2. 补齐 incident/release squad 的路由说明、后续阶段交接和输入输出契约，添加缺证据/顺序/同产物约束的回归测试。
3. 为事故学习、发布后验证、可靠性和 Agent 评测补充脱敏 fixture/演练文档，确保 `unknown`、审批和停止条件可观察。
4. 把五类交付治理能力落到既有角色和可组合 skill 路由；记录体验/迁移 workload gate，不在证据不足时新增 listed role。
5. 更新 server template spec、生命周期矩阵和 MCP 取舍文档；运行 Go、前端、类型检查和浏览器模板 E2E。
6. 完成两条闭环桌面演练：失败→RCA→修复→发布→观测→事故学习→防重发，以及 Agent 变更→评测→门禁→发布→漂移监控；真实模型测试只在显式授权时执行。
