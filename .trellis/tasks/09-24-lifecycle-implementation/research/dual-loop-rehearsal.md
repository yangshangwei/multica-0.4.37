# 双闭环脱敏演练

## 研发交付闭环

输入：`artifact=sha256:demo-release-1`，发布前错误率 `0.2%`、p95 `180ms`；发布后同一 digest 的 15 分钟窗口错误率 `2.4%`、p95 `640ms`，队列深度从 `12` 增至 `410`。客户标识、凭据和原始日志均已删除。

1. Incident lead 先请求 `production_release` 人工审批并交给 release engineer 回滚/关闭开关；QA 用原始用户侧信号确认错误率恢复到 `0.3%`、p95 `210ms`。
2. Diagnostician 收到脱敏时间线、变更范围和失败样例，单独输出 `confirmed/suspected/unknown` RCA；事故主任务不等待该结论。
3. Reliability engineer 记录 SLO 影响、错误预算、队列恢复和未观测的依赖信号；观察窗口未结束前 decision 为 `unknown`，不把回滚提交当作恢复演练。
4. Incident-learning 关联事故任务，产出“补充队列告警（owner: runtime，验收：连续 7 天无漏报）”“增加回归测试（owner: implementer，验收：失败样例通过）”等预防任务；重复事故链接已有任务，不创建第二个无主任务。

## Agent 质量闭环

输入：`agent=v3`、`skill=multica-debugging@1`、`mcp=playwright@pinned`，候选版本相对 `v2` 只改变提示词。固定 case manifest 包含成功、工具超时、越权读取凭据、成本上限、延迟上限和工具版本漂移。

1. Agent Evaluator 保存脱敏输入摘要、工具轨迹、停止原因、成本和延迟；越权 case 必须停止并判为安全失败。
2. 结果表分别记录 correctness/safety/cost/latency/drift；缺少 v2 基线或样本不足的格子为 `unknown`，不能由成功样例补齐。
3. Review Gate 收齐 code/security/QA/evaluator 结论；Release Squad 只在门禁通过并有人批准后发布同一候选 artifact。
4. 发布观察窗发现工具版本漂移或成本超阈值时，输出 hold/rollback recommendation 和新增评测用例，不自行改模型、绑定、生产配置或发布状态。
