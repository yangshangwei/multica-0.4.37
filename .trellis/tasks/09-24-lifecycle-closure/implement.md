# 实施计划

1. 读取现有 RCA/双闭环子任务证据，建立 requirement-to-evidence 表，标出“模板存在”和“闭环可运行”之间的差距。
2. 补 RCA、事故学习、发布后验证和 Agent 评测的脱敏 issue/comment fixture/交接验证；必要时只新增最小测试辅助，不新增生产控制面。
3. 统计体验验证与迁移审查工作量，形成 workload gate 结果；本轮证据不足以新增两个 listed role，保留路由建议和复评条件。
4. 验证五类治理能力的触发-产物-验证矩阵和小队组合，修正文档/skill/路由不一致。
5. 执行两条闭环演练，记录任务/评论/文件产物和 unknown/stop 条件。
6. 运行 Go service/handler 定向套件、前端 skill 测试、typecheck、Playwright；全量失败与既有 fixture 问题分离记录。
7. 更新父/子 Trellis 任务验收和研究证据，只有所有可授权检查完成后才结束本任务；真实模型 smoke 保留明确未测试证据。

已完成的本轮证据：`server/internal/service/testdata/lifecycle/lifecycle_handoff_fixtures.json` 与 `lifecycle_handoff_fixture_test.go` 覆盖成功交接、下游消费和五类失败路径；`research/workload-gate.md` 记录 v0.4.46/v0.4.47 的来源、计数、gate 结论和复评条件。
