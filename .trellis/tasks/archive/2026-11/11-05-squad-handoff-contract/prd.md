# 小队阶段交接与指令一致性

## 目标
修复审计项 1 与 8 的已确认合同矛盾，保留人工验收和现有阶段事件语义。

## 需求与验收
- C1：assignment/comment/leader 的实际注入指令都明确交付 in_review 是等待验收，只有 done/cancelled 关闭子任务阶段；不得再承诺 in_review 会推进 barrier。
- C2：working-on-issues、squads 的 skill/source-map 与双语产品文档一致区分“同 issue 成员评论回传”和“子 issue 终态唤醒父任务”。不增加绕过验收的 @parent 指令。
- C3：Review Gate leader 与小队六席位和适用条件一致：代码、安全、QA、Agent评测、体验、迁移；缺关键证据 unknown/hold，修复回原作者、不能自行验收。
- C4：review-gate-lead 模板 v2→v3；小队 v3 保持；自治/默认并发/skills 不变；已有实例不自动覆盖。
- C5：后端、前端双语摘要与 docs 表准确；skill 行预算符合现有约束；新增生成/模板回归能在旧源码失败，在修改后通过。

## 非目标
不改 issue_child_done 状态逻辑、数据库、自动阶段推进、并行审查协议或跨账号协作状态机；这些属于后续独立工作流设计。
