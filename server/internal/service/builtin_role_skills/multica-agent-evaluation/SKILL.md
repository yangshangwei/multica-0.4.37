---
name: multica-agent-evaluation
description: "用版本化评测用例验证 Agent、Skill 和 MCP 变更的正确性、安全性、成本、延迟和漂移。"
user-invocable: false
metadata:
  category: quality
  icon: bot
---

# 智能体评测

## 评测输入

每个 case 必须记录候选 Agent/Skill/MCP 版本、模型和工具版本、输入摘要、期望结果、禁止行为、权限、样本量、成本/延迟阈值和变更前基线。基线或关键工具证据缺失时写 `unknown`。

## 最小矩阵

- 正常成功和边界输入。
- 工具失败、超时、空结果和重复调用。
- 越权请求、凭据/生产数据请求和应停止的高风险动作。
- 正确性回归、成本或延迟超阈值、结果稳定性。
- 模型、提示词、Skill、MCP、权限或数据变化造成的漂移。

评测交接必须提供 baseline/candidate Agent 版本、Skill/MCP 版本和每个 case 的 `category`、脱敏 `trace`、`stop_reason`、`result`。正确性、工具失败、安全、成本、延迟和漂移各至少一个 case；缺 trace 或缺越权拒绝 case 时结论为 `unknown` 或 `hold`。

## 交付与边界

保存脱敏输入摘要、工具调用、停止原因、实际结果和判定。每项结论标记 confirmed、suspected 或 unknown，并附复现条件。评测不替代代码审查、安全审查、QA 回归或发布审批；不修改线上配置、不读取凭据、不执行发布/回滚。需要付费模型、真实客户数据或生产操作时先请求人工介入。
