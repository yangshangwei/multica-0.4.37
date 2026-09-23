# 实施计划

1. 读取现有 handler/service 测试和 lifecycle fixture，建立“模板存在 / 运行时交接 / 外部信号接入”三列审计。
2. 先添加失败测试：RCA route contract、incident-learning dedup/owner contract、rollout evidence gate、agent evaluator case gate。
3. 实现无副作用的生命周期 contract validator，并在实际 issue/comment/task queue 测试中调用；不新增生产迁移。
4. 将 validator 和交接格式接入相关 role skill/squad instructions，明确 unknown/hold/人工审批停止条件。
5. 补充治理能力审计记录：契约兼容、威胁建模、供应链、产品结果、灾备分别指向既有角色和可验证产物；没有独立 workload 的不新增角色/MCP。
6. 运行定向 Go 测试、前端 typecheck/Playwright、Trellis validate 和 diff check；分离记录环境已有失败。
7. 完成前检查工作树，不覆盖上一轮改动或用户未跟踪文件，并更新本任务研究证据。
