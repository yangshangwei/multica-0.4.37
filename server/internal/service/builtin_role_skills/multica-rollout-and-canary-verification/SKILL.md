---
name: multica-rollout-and-canary-verification
description: "针对同一已批准发布产物核对基线、观察窗口、健康信号、阈值、结论和回滚结果；不把重建产物混入验证。"
user-invocable: false
metadata:
  category: operations
  icon: chart-line
---

# 发布后与 Canary 验证

## 输入

确认发布审批、不可变 artifact digest、环境、基线、观察窗口、信号来源、阈值、负责人和允许的回滚路径。产物标识、基线或窗口不明确时停止并标记 unknown。

## 检查

1. 记录发布前基线和将要观察的错误率、延迟、吞吐、队列、关键业务结果与依赖信号。
2. 在同一 artifact digest 上按固定窗口采样，标记数据缺口、告警、阈值越界和用户影响。
3. 输出 continue、hold、rollback recommendation 或 unknown，并说明每项证据；不要用重建或不同配置的产物替代。
4. 记录回滚是否获批、是否执行、执行结果和恢复信号；未获批时只给建议。

交付评论必须可被下游读取：`approved_digest` 与 `artifact_digest` 相同，且有 `baseline`、完整 `observation_window`、`signals`、`decision` 和 `rollback_outcome`。任一缺失或 digest 不一致只能标记 `unknown`，不能用重新构建的产物替代。

## 权限边界

读取生产指标必须已有授权且按脱敏范围执行。不得自行发布、暂停流量、回滚、修改阈值、通知客户或读取凭据；每个生产动作交给发布工程师并单独请求人工审批。发布后验证不替代 QA、安全审查或事故学习。
