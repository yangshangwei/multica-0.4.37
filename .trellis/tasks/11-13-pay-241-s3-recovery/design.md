# S3 恢复设计

2026-11-15 规划补充已独立通过：`research/recovery-followup-v2-wire-review-r05.md/json` 为 PLANNING PASS，C01/C02 开放规划问题为零。阻塞恢复采用 `research/recovery-followup-block-contract-v2-20261114.md` 与已修正的 `research/recovery-followup-wire-supplement-r05.md`（SHA256 `d9934020f24751ee722bb0bf115a0e587a37a4ee6bb0465cf12e26fee90e76c5`）。仍需绑定最终已接受的 S2 source/锁合同后激活；规划通过不是已实现接口或 S3 验收。

## 已通过的阻塞与历史合同

- 单个 `information_repair` 请求在既有 workflow 事务内提交真实 blocked→非 blocked 状态、当前合法处理人、精确 superseded 关系/计数、事件和原键回执。后续 B2 是人另发的新请求；修复不自动创建 B2、Resume、派发或消费完成批准。
- 正常完整嵌套与 return 保留其精确链路。pending submission 存在时只允许精确提交/事件、当前可用 creator 和 in_review 出口；不可用则拒绝。正常返回的状态/fallback 来自原事务事实，不能改写为当前 catalog/handler 推断。
- 新受管 information request 的 reason、recipient、unblock_condition、estimated_resolution_at 必须完整。原 omitted 字段保留旧请求 hash；显式 null 与缺失分开。ETA 接受带时区的零至六位小数，持久化 UTC 微秒 instant，按最终 DB clock 校验新写；旧 ETA 过时不改变历史或调度执行。
- current actionability 与 complete/legacy/unavailable 历史记录分别表达。source-only authority 的 nullable participant/account epoch 使用真实 member 身份补充 fingerprint，不能制造 epoch。runtime v1、旧 snapshot/source bytes 保持其原合同。
- creation-options 可按显式 source/project 投影 not_selected/known policy observation；GET 观察不进入 creation POST/hash。真实创建事务解析 parent inheritance 的有效 project，再核当前政策；受管初始 blocked 继续用现有 status_unavailable 拒绝。
- 人类 history 使用真实 handoff `(created_at,id)` keyset、upper tuple、当前 revision/authority 和闭合 provenance。旧 event linkage 缺失明确为 null，仍入页及 repair 的完整 decimal denominator；不造 event ID。historical repair basis 仍须精确可验证原 event。
- history-only HMAC 使用实际缓存并通过验证的 server JWT signing key，拒绝公开开发默认值；真实重启轮换使旧 cursor 不再有效。新读事务显式五秒，编码 JSON 含 metadata/escaping/newline 不超过 65,536 字节，limit 为 1–50、cursor 不超过 2,048 字节；首个完整 item 超限返回 unavailable，不能漏行或伪造完整页。

复用既有workflow operation、原run/attempt/command、Ledger、Stop/proof、outbox与Query恢复。不会建立平行重试、通知或补读系统。具体源码与切口见research/recovery-gaps.md。

## 独立增量

- timeout-proof：真实受控执行deadline、API已提交后的ACK丢失、同目录异进程原账本恢复、明确新运行及旧失败留存；Web/Electron真实呈现。该子任务已完成实测并通过独立检查，2026-11-14结项归档。
- 并发/补读/通知：分别补priority与handler同revision两顺序、普通child显式expected_revision，55条跨页离线评论及实际WS重认证，人员/Agent混合提及与冻结接收人原事件去重。复用既有fixture，不把optional ordinary CAS称为全客户端强制版本。
- 阻塞结构：S2合同稳定后扩展既有information handoff，保留reason/recipient/previous status，增加解除条件与预计解除时间。受管任务进入blocked须完整四项，解除沿用原request身份、实际人/时间和已保存恢复状态；稳定业务责任人与本次阻塞处理人分别表达。旧未启用任务的兼容规则明确记录，不改成通用工作流引擎。

## 证据边界

Runtime outcome不等于评论或附件；各自计数和ID/hash独立校验。无权读取后仅允许已定义的content-free原回执，不能回放私有正文。WS是失效提示，补读完整性通过权威分页ID与版本证明，重复帧不是重复业务事件。真实模型、支付及100人性能分别由对应任务验证，不从本包推断。

DB按package/normal/race分离，浏览器另库，源码匹配API/production Web/Desktop。每次fixture使用正确host或namespace gate，并最终恢复原标志（epoch单调）。每个实现切口独立检查；所有S3验收满足才关闭S3。
