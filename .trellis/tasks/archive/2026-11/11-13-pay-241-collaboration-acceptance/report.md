# PAY-241 归档与人工核实交接

2026-11-20，用户明确决定：**“归档吧，剩下的任务我手工核实”**。据此将 PAY-241 父任务及仍开放的 S4、S5 转入行政归档，剩余核实工作交由用户接手。归档后的 `task.status=completed` 表示本轮代理工作结束；**`overall_acceptance_passed=false`**，完整业务验收和真实业务签字尚未完成。

本报告是归档后的当前交接入口。此前 PRD、设计、执行计划及研究中的“父任务保持开放”“不能归档”等表述保留为当时决定；本次用户指令仅更新任务处置方式，不修改原验收标准、历史证据或用例结果。

## 保留的验收结论

[24 条权威账本](test-matrix.csv)保持 **9 PASS / 11 PARTIAL / 4 NOT_RUN**，全部 CSV 字节保持不变。已有结果继续绑定各自源码、环境和接受范围，本轮没有重新运行产品测试，不能据归档推定当前 HEAD 再次通过。

- PASS：TC-02、05、08、13、15、16、21、22、23。
- PARTIAL：TC-01、03、04、06、07、12、14、17、18、19、20。
- NOT_RUN：TC-09、10、11、24。

[S1](../11-13-collaboration-agent-identity-baseline/report.md)、[S2](../11-13-pay-241-s2-stage-approvals/report.md)、[S3](../11-13-pay-241-s3-recovery/report.md)保持原已接受结论。[TC-06](research/tc06-independent-check-20261120.md)人工编辑与运行回复并发补证、[TC-12](research/tc12-independent-check-20261120.md)本地合成业务代码 A→B 补证保持接受；两者整例仍为 PARTIAL，不能代替真实支付仓库、远端 PR/CI 或外部执行证据。

## 交给用户核实的剩余事项

以下沿用[下一输入合同](research/next-input-contract-20261120.md)与[现有缺口](gaps.md)，不增加新的业务规则。详细阶段交接见 [S4 报告](../11-13-pay-241-s4-external-integrations/report.md)和 [S5 报告](../11-13-pay-241-s5-performance-acceptance/report.md)。

| 范围 | 人工核实前需明确或取得的最小事实 | 仍需保存的结果 |
| --- | --- | --- |
| TC-01 / TC-03 | PAY 完整填报清单或强制门禁的明确选择；若强制，列出字段、范围、检查时点与旧数据处理。确认 P1 对应 enum，以及分诊通知触发、收件人、内容、入口、送达与去重标准 | 按所选业务语义核对真实行为；已有完整正向创建、High 展示或单条 assigned 通知不自动核销这些条款 |
| TC-04 / TC-14 | 独立明确 Provider/model、账号范围、PAY 材料、有限次数与停止条件；质量评分和审核人，或允许的超时/失败注入方法。安装版 Main 若纳入范围，单独绑定 OS、可执行文件/ASAR/CLI 与匹配源码/API | 真实模型的依据、风险、置信度质量及失败恢复身份；受控 Provider、renderer/preload 和其他任务授权不替代该证据 |
| S4 E01 / TC-06、07及真实来源关联 | I02 指定仓库、连接、允许分支/PR、base/head/OID 和操作范围；I03 完整检查集、workflow/run/job/attempt 与制品 digest；相关 I09 实现者、评审者、QA 账号及实际角色 | 精确 head 与完整 CI/制品绑定；事件重投、旧 head 迟到、测试连接撤销/替换后的旧批准消费。只读 PR 观察只能支持部分身份绑定 |
| S4 E02–E04 / TC-09–11，以及 TC-12 真实修复 | I04–I06 支付/订单/履约沙箱与权威账本、回调身份、真实 45 秒延迟及消费失败恢复入口；TC-12 另绑定实际失败/修订版本和固定回归输入 | 正常支付、同回调三次、超时后成功、乱序及失败恢复的各域身份和结果；原失败与修复后的新结果分别保留 |
| S4 E05–E07 / TC-17–20 | I07 非生产执行器、精确制品/目标及原操作查询；I08 实际指标与故障恢复范围；I09 当前有权的 QA、发布及验收人 | QA 越权拒绝、人工批准后的实际发布/回滚、权威结果与完整交付审计。批准记录不等于外部执行成功 |
| S5 / TC-24 | 所需 S2–S4 变更稳定后的最终候选、匹配构建与新运行身份 | 100 独立协作者 × 100 事件的完整 10000 样本，以及 100 独立 run 的两类回写样本；保留失败、缺失与中止分母，并按既有双重 p95 和回写门槛判断 |
| 总验收与业务签字 | 原 24 例、缺陷处置，以及产品、研发、QA、项目经理的实际验收结论 | 原验收标准是否满足与真实签字；测试账号签字单列，不因行政归档生成业务通过结论 |

S4 **E01–E07 均为 NOT_RUN**。S5 旧 r02 仅为工具演练，最终 **TC-24 仍为 NOT_RUN**。普通评论机器借用 owner/admin 权限的问题继续由 `.trellis/tasks/11-20-comment-admin-human-authority` 独立承接。

人工核实后应新增带日期、操作者、源码/环境/外部身份和证据引用的结果记录，再按实际范围判断用例；不要改写旧失败、旧接受报告或把测试账号结论改称业务签字。本次归档未调用真实模型账号、支付系统或生产发布，也不赋予这些操作新的代理执行授权。

## 历史路径查阅规则

历史报告和 JSON 保留原始字节与哈希。遇到其中旧任务路径时，按下表将前缀映射到归档目录；后续文件名和子目录保持不变。

| 历史任务前缀 | 归档前缀 |
| --- | --- |
| `.trellis/tasks/11-13-pay-241-collaboration-acceptance/` | `.trellis/tasks/archive/2026-11/11-13-pay-241-collaboration-acceptance/` |
| `.trellis/tasks/11-13-pay-241-s4-external-integrations/` | `.trellis/tasks/archive/2026-11/11-13-pay-241-s4-external-integrations/` |
| `.trellis/tasks/11-13-pay-241-s5-performance-acceptance/` | `.trellis/tasks/archive/2026-11/11-13-pay-241-s5-performance-acceptance/` |
| `.trellis/tasks/11-13-collaboration-agent-identity-baseline/` | `.trellis/tasks/archive/2026-11/11-13-collaboration-agent-identity-baseline/` |
| `.trellis/tasks/11-13-pay-241-s2-stage-approvals/` | `.trellis/tasks/archive/2026-11/11-13-pay-241-s2-stage-approvals/` |
| `.trellis/tasks/11-13-pay-241-s3-recovery/` | `.trellis/tasks/archive/2026-11/11-13-pay-241-s3-recovery/` |
| `.trellis/tasks/11-13-pay-241-s3-timeout-proof/` | `.trellis/tasks/archive/2026-11/11-13-pay-241-s3-timeout-proof/` |

历史相对链接先按文档原位置解析，再应用上述映射；已经包含 `archive/2026-11/` 的路径不重复添加。源码、规范和 `docs/plans/` 路径仍按仓库根目录解析；仓库外私有证据的绝对路径保持原值。此规则便于查阅，不修改历史执行的来源绑定。

## 本次归档验证边界

本次仅处理任务交接文档、任务元数据和归档位置；没有产品、测试、依赖、迁移或通用规范变更，也没有新的可执行产品合同需要写入规范。文档检查、独立复核及 Git 收尾记录见[本轮记录](research/session-manual-handoff-20261120.md)。产品 lint/typecheck/build、TestMain、数据库、API、浏览器、模型和性能测试均未在本次归档中执行。
