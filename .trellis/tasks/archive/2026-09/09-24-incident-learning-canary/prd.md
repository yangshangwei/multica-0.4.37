# 事故学习与发布后验证

## Goal

把事故记录转成可追踪的预防工作，并让每次发布都有一份基于同一产物的发布后健康结论。

## Requirements

- 新增 `multica-incident-learning`，输入脱敏时间线、影响信号、缓解动作、恢复证据和已知未知项。
- 事故学习必须区分事实、推断和未知；为每条预防措施记录负责人、验收信号、优先级和关联事故，不能把“没有证据”写成根因。
- 新增 `multica-rollout-and-canary-verification`，记录发布产物标识、基线、观察窗口、信号、阈值、结论、回滚结果和未观测项。
- 发布后验证只能针对已获审批的同一发布产物；生产读写、回滚和通知继续需要人工审批。
- 事故主流程在影响停止并确认后即可收尾；学习任务和长期修复必须独立链接，不阻塞止血。
- 对重复事故和未关闭预防任务给出可复核的关联提示，但不自动执行生产变更。

## Acceptance Criteria

- [x] 两个 skill 的输入、输出、停止条件、权限边界和未知证据处理写入模板并有单元测试。
- [x] 至少一个脱敏事故样例生成带 owner 和验收信号的预防任务；缺证据样例明确标记 unknown。
- [x] 至少一个发布样例记录 artifact digest、baseline、window、signals、decision 和 rollback outcome，且验证不会混用重建产物。
- [x] 重复事故与未关闭预防项可被发现并指向既有任务，不产生重复无主任务。
- [x] 与 incident/release squad 的路由、模板复制保护、本地化和文档保持一致。

## Notes

- Keep `prd.md` focused on requirements, constraints, and acceptance criteria.
- Lightweight tasks can remain PRD-only.
- For complex tasks, add `design.md` for technical design and `implement.md` for execution planning before `task.py start`.
