# 技术设计

两个 skill 均为证据整理和决策辅助，不直接执行生产操作。`multica-incident-learning` 消费事故完成后的脱敏材料，输出事实/推断/未知、预防任务、owner、验收信号、重复事故关联和需人工决定项。`multica-rollout-and-canary-verification` 消费已批准产物和发布前基线，输出观察窗口内的信号、阈值、结论和 rollback outcome；产物以 digest/不可变标识绑定，禁止对重建产物作结论。

接入 incident/release squad 的独立后续阶段，保留现有 human approval 和 workspace skill 复制语义。优先使用 issue/comment/link 现有 API，不增加表；若现有 API 无法表达证据，再提出最小后续设计。
