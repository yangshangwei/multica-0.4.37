# 技术设计

## 验收边界

本任务是证据收口任务，复用现有 template registry、squad instructions、lifecycle handoff handler、Playwright fixture 和 Trellis task metadata。不修改生产控制面，不创建新 Agent 类型、数据库表或凭据型 MCP。

## 验证层次

1. 静态层：核对 diagnostician roster、skill、权限、squad 路由和副本保护测试。
2. API/E2E 层：通过本地 API 和浏览器创建真实模板副本，记录实际测试结果。
3. 模型层：只在 `agentintegration` 与 `MULTICA_RUN_REAL_AGENT_SMOKE=1` 同时明确授权时执行；否则记录 blocker、替代证据和复现命令。
4. 任务层：读取父任务与子任务 JSON、PRD 和 evidence，输出状态矩阵，不擅自归档或完成其他任务。

## 失败语义

`pass` 只用于实际执行并通过的检查；`skipped` 用于环境缺失但不需要授权的测试；`blocked-by-authorisation` 用于真实模型、生产指标或凭据边界；`hold` 用于证据失败。所有状态都要注明来源文件或命令。
