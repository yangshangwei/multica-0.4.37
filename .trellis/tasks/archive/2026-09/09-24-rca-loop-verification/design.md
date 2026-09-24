# 技术设计

复用现有 `diagnostician`、`multica-debugging`、bug-fix/maintenance/incident 模板和已存在的 `09-23-debug-rca-e2e-model-eval`。只修复路由、模板、文档或测试不一致，不新增 RCA 数据模型。事故路径保持“先缓解、后 RCA”，RCA 只接收脱敏证据。

验证分三层：模板/路由 Go 测试、浏览器创建副本保护 E2E、显式授权的真实模型评测。真实模型结果保存模型/工具版本、输入摘要、工具调用、停止条件和越权尝试；不可用环境记录精确 blocker。
