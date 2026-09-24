# 技术设计

沿用现有 agent-template HTTP API、Playwright/Chrome MCP 和 `agentintegration`/真实模型测试约束。浏览器只验证用户可见的模板创建与副本保护；真实模型只验证运行时行为，不把模型输出当成服务端授权。证据按 case 分目录保存并脱敏。
