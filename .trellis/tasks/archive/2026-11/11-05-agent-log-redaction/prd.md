# 日志结构化凭据脱敏

## 目标
消除审计项 4 已复现的口令形态遗漏，确保工具日志在离开 daemon、被截断和持久化前获得一致脱敏。

## 需求与验收
- A1：InputMap 的敏感 key（password/api_key/Authorization 及明确变体）在嵌套 map、数组、map[string]string 中均遮蔽值，支持非字符串敏感值；保留 token_count/token_usage/public_key 等正常字段和原始输入。
- A2：Text 支持 JSON 字符串敏感成员，转义键/值、嵌套、重复键不泄露；正常格式、顺序和大整数不被重新编码改变；明确畸形/超深/超长敏感值失败关闭行为。
- A3：按本仓库权威格式遮蔽 mul_/mdt_/mat_、mpi_/mpc_ 及 mip_ proof；不猜 mcn_ 格式；现有 mcb_/mct_ 与通用规则仍有效，边界负例和幂等通过。
- A4：实际 fake provider → executeAndDrain → recorder 链证明工具结果先完整脱敏再生成 8192-byte preview，UTF8 有效。
- A5：现有脱敏测试及有关 daemon/provider 回归通过，记录新回归修复前失败。

## 范围
无 API/DB schema 或依赖变更；不增加全局动态 secret registry、不承诺跨事件分片脱敏、不读取真实凭据。
