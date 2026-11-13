# S3 恢复设计

复用既有workflow operation、原run/attempt/command、Ledger、Stop/proof、outbox与Query恢复。不会建立平行重试、通知或补读系统。具体源码与切口见research/recovery-gaps.md。

## 独立增量

- timeout-proof：真实受控执行deadline、API已提交后的ACK丢失、同目录异进程原账本恢复、明确新运行及旧失败留存；Web/Electron真实呈现。该子任务已完成实测并通过独立检查，2026-11-14结项归档。
- 并发/补读/通知：分别补priority与handler同revision两顺序、普通child显式expected_revision，55条跨页离线评论及实际WS重认证，人员/Agent混合提及与冻结接收人原事件去重。复用既有fixture，不把optional ordinary CAS称为全客户端强制版本。
- 阻塞结构：S2合同稳定后扩展既有information handoff，保留reason/recipient/previous status，增加解除条件与预计解除时间。受管任务进入blocked须完整四项，解除沿用原request身份、实际人/时间和已保存恢复状态；稳定业务责任人与本次阻塞处理人分别表达。旧未启用任务的兼容规则明确记录，不改成通用工作流引擎。

## 证据边界

Runtime outcome不等于评论或附件；各自计数和ID/hash独立校验。无权读取后仅允许已定义的content-free原回执，不能回放私有正文。WS是失效提示，补读完整性通过权威分页ID与版本证明，重复帧不是重复业务事件。真实模型、支付及100人性能分别由对应任务验证，不从本包推断。

DB按package/normal/race分离，浏览器另库，源码匹配API/production Web/Desktop。每次fixture使用正确host或namespace gate，并最终恢复原标志（epoch单调）。每个实现切口独立检查；所有S3验收满足才关闭S3。
