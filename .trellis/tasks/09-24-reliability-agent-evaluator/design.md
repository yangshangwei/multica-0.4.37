# 技术设计

增加两个 listed role template，各自只附一个 role skill：`reliability-engineer` -> `multica-reliability-engineering`，`agent-evaluator` -> `multica-agent-evaluation`。前者默认 observer/contributor 取决于现有模板约定，后者默认 observer；两者不获取 operator 权限。角色提示词定义独立交付物和与 QA/security/release/diagnostician 的交接，不按技术栈拆分。

评测 skill 采用版本化 case manifest（输入摘要、期望、禁行行为、工具/模型版本、阈值），报告记录正确性、安全性、成本、延迟和漂移。可靠性 skill 采用 SLI/SLO、错误预算、队列/容量、依赖降级、RPO/RTO 证据表。使用现有 agent/template API 和测试，不增加持久化模型。
