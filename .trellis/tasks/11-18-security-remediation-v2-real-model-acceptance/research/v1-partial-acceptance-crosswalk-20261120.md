# v1 失败项与当前 v2 部分验收对照

父任务 AC10 的中间记录，未关闭 AC10。来源为子任务一 design 第 11 节、最终 implement 集成回执，以及本子任务冻结预期和真实平台记录。契约/脚本证据是已保存历史，本轮没有重新运行子任务一整套测试；真实模型结论只覆盖列出的样本。

| v1 问题 | 已合入的契约/平台脚本证据 | 官方 CLI 真实观察 | 当前判定与未覆盖 |
| --- | --- | --- | --- |
| 队长直接改代码、代写成员报告 | separation/reconciliation 契约；依赖和代码脚本按本人作者与 source_task 对账 | P1 停止并人工移交；O1 安全审查由受限 reviewer 本人交付；C1 队长仅派发可选分析师 | **样本内部分证明**。C1/D1 代码闭环未完成，不能推断队长在补丁流程中始终不越界 |
| 成功派发被记为 no_action | 模板指向系统协议；脚本派发回合核对活动 | C1 委派有 queued receipt，并记 action；O1 的活动随实际回合保存；P1 补充查询遇错记 failed，符合原冻结 protocol-consistent 条件 | **已观察到正确的局部记录**。P1 action-only 自动失败原件保留；没有以新运行刷 action。未覆盖所有失败、重派及 no_action 分支 |
| 诊断与实现角色交接未演练 | 代码脚本验证 verifier→implementer→verifier/QA 的本人交付；诊断角色仍未覆盖 | C1 仅 analyst queued，未开始 verifier/implementer 交接；D1 未运行 | **未覆盖真实交接质量**。不能把 bundle 脚本成功写成模型交接成功 |
| 审查报告与 diff、测试计数不符 | commit_binding/reconciliation/evidence 契约；代码与依赖脚本保存提交和测试 | O1 报告如实承认证据缺口，未声称独立文件/测试通过；P1 保持 unknown | **对代码审查对账仍未覆盖**。没有真实修复提交、同一测试红绿及 G2 完整对账 |
| 容器出站和角色文件隔离不足 | 部署边界文档；独立角色容器、managed 禁网、原 mat_ 通道及只读审查控制 | P1/O1 真实 transport 与 command 记录受当时绑定隔离约束；C1 仅 leader；代码角色当前仅无模型/脚本控制 | **已证明指定装置探针与有限真实回合**。不声称所有角色真实执行或通用沙箱安全 |
| 真实模型总门槛未通过 | 子任务一保留模型未整体验收说明 | P1 successor、O1 通过冻结要求；P1 首轮失败；C1 incomplete；D1 not-covered | **AC9/AC10 未完成**。original app-server SDK 未通过；CLI-direct 不替代生产适配器验收 |

漏洞验证座位的真实接单、复现测试、重跑和完整耗时均尚未形成闭环。C1 当前真实任务只有 leader 和未领取的 product-analyst，不能以 fake C1 的 14 次角色执行计入真实工作量，更不能据此满足两个发布周期的内置角色升格门槛（父任务 B7/K2）。

本轮不修改冻结模板、角色或 expected-actions。工具筛查和恢复前置见 [续做审计](resume-evidence-audit-20261120.md)；逐案 P1/O1 原始证据复核见 [独立历史审计](resume-p1-o1-audit-20261120.md)。
