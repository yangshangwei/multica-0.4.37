---
name: multica-incident-learning
description: "事故恢复后整理脱敏事实、未知项和预防任务；为每项措施指定负责人、验收信号和重复事故关联。"
user-invocable: false
metadata:
  category: quality
  icon: repeat
---

# 事故学习

## 输入

只接收事故结束后的脱敏时间线、影响信号、缓解动作、恢复确认、变更/依赖线索和已有后续任务。把每条内容分成事实、推断或 unknown；缺少日志、时间、影响范围或恢复信号时不能补猜。

## 方法

1. 按时间线核对发现、影响、缓解、恢复和观察结束。
2. 区分触发因素、放大因素、检测缺口和恢复缺口；不要把“人做错了”当成根因。
3. 为每项预防措施写 owner、优先级、验收信号、截止条件和关联事故；先查找未关闭或重复任务。
4. 将永久修复、回归测试、监控/runbook、演练和产品决策拆成可验证任务。

交接评论至少带 `source_issue`、`facts`、`inferences`、`unknowns`；每项 prevention task 带 `issue`、`owner`、`priority`、`acceptance_signal` 和 `related_incident`。已有任务或重复事故应引用原编号，而不是创建无主副本。

## 边界

事故主任务在影响停止并确认后即可收尾；学习与长期修复是独立后续任务，不阻塞止血。不要读取生产凭据、客户原始数据或未脱敏材料，不执行发布、回滚、通知或状态收尾。证据不足必须显式报告并请求人工补齐。
