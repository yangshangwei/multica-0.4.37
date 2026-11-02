# 知识 runtime 的真实验收登记

状态：知识真实场景未运行，知识预算为 0。原产品提交 `96cedf485` 经 `8cb04e329` 合入；21 项审计处置再经 `6cb9fcb36` 合入。当前实际被测产品为 `639bf6ef8`，第 4 候选 `64a9994ec` 保留相同产品指纹 `9f72fc214b8e81f6fc3c1969be5e3304b7f158f60a6f7305f1531d55574d80ee`。共享 v6 已于 2026-11-02 15:02:20 UTC 以 exit 2 结束，当前等待唯一负责人修正 outer-Go PUB_DIR 环境隔离并启动新的完整检查；以下登记不把准备或已有 fake 结果计为整体验收通过。

已有独立复核的 runtime [RK1](../10-30-collaboration-context-handoff-runtime/research/real-model-rk1/README.md) 框架及 normal/race 假执行证据；它目前只支持 runtime 场景，fixture 显式关闭知识。RK1-K1–K3 和普通 E0b 不在其可执行分派表中，不能通过打开登记标志或借用 runtime 预算运行。知识框架、固定 K3 入口表、单独预算及最终绑定仍需完成，见 [当前关闭条件核对](../11-01-collaboration-knowledge-audit-fixes/research/continuation-20261102/knowledge-closure-readiness.md)。生产知识开关继续默认 false。

人工页面、实际 fake 输入、模型理解各按自己的证据记录。原 runtime `0e1ee8e1e` 或旧 E0/E1 不能证明当前知识真实场景已经通过。

## 原系列继续保持原账本

原 E2a trial 2 的 upstream 502 和后续 E3 after trial 1 的 EPROTO/超时无效尝试均保留；当前下一入口为 **E3 after trial 2，剩余 14 次**，再按原登记进入 E4 → E5 → E6。原冻结源码、样本和账本由首轮闭环负责人管理。
场景、来源分组、Claude Opus 5-5、编号、三槽限制、首个不完整结果停止及原终止证明
规则保持不变。禁止替换样本、额外健康请求、切换模型或扩展超时。
原系列和新版本验收在独立时段运行，不与编码、迁移或数据库全套测试重叠。

## 当前三份预算独立

| 系列 | 当前登记 | 归属与条件 |
| --- | --- | --- |
| 原冻结 E0–E6 | 剩余 14 次 | 首轮闭环任务；保留原编号、无效样本及源码 |
| RK1 runtime | 批准上限 25，当前记录消耗 0 | runtime 任务；包含 H2b，H6 每轮两次启动；不涵盖知识或普通 E0b |
| 知识 RK1-K1–K3 | 批准 0，消耗 0 | K1 登记 4 次，K2 登记 2 次，拒绝路径 0 次；K3 逐入口固定后才能确定 N；普通 E0b 的 1 次另行登记 |

旧“最小 14 / 不含 K3 全集 29”是方案估算，不是知识预算授权。当前额度以各自账本为准，不跨系列转移。本页不修改任何全局预算、paid source_id、runnable 或启用状态。

## 当前下一阶段设计（未实施）

[知识专用框架设计](../11-01-collaboration-knowledge-audit-fixes/research/knowledge-real-model-preregistration-20261102/design.md) 和 [入口表](../11-01-collaboration-knowledge-audit-fixes/research/knowledge-real-model-preregistration-20261102/entry-matrix.json) 已按现有代码落盘。提案使用六个新增测试/driver 代码文件（五个 rk1k 测试文件、一个 tagged daemon driver），保留原 17 个 runtime 框架文件和原系列账本；launcher 是后续协调构件。知识专用 wrapper 区分真实 ordinary task/run 身份，薄 daemon driver 复用生产执行，不伪造 collaboration run ID。

建议固定 K1 四次、K2 两次、K3 八个生产者/角色样本，共 14 次知识调用；普通 E0b 的一次另计。这不是八个不同知识 API 或八条 UI 旅程，也不是已分配预算。K1 的四个消费者彼此独立，第四状态选择撤回，真实过期另列；K2 在仍有效的第三 claim/preparation 冻结后撤权，再验证零第三进程。撤权早于新 claim 时允许合法选择 fresh generation，不能强行记成全局拒绝。Root 生产者使用现有 comments+mention+target_versions API，不新增不存在的 commands endpoint。

后续先在独立环境实现并复核 fake-only 框架，确认普通 task-token CLI 读取、AI quick-create 关联/fence、真实来源撤权、全部物理启动与证据计数。公共/私有 source、CLI、配置与框架绑定完成后，才能申请精确的知识预算与独占执行窗口；当前保持分配 0、无知识真实调用。完整共享检查由原负责人单执行，本设计没有启动测试或 Provider。

## 新知识版本独立登记

1. 绑定最终合入 commit 与完整 source/overlay SHA-256；登记实际服务端、daemon、
   Provider CLI 版本、平台/安装方式、请求与观察模型、thinking/tier、账户非敏感标识。
   源码发生变化时，逐项重验受影响范围，不能只替换报告里的版本号。
2. 先为知识场景建立并独立复核可执行分派：当前 `rk1_scenarios_recall_test.go` 仅有 E0a/H1/H2/H2b/H3a/H3b/H5/H6，`newRK1Fixture` 关闭知识，launcher 仅采用 runtime addendum。新增知识能力须保留真实 router/claim/披露/授权/记录/实际输入链路，先完成无真实 Provider 的证明，再登记独立数据库、run/session/attempt、固定编号、调用上限和停止规则。当前没有为知识系列分配预算。
3. 首先证明真实 CLI 接受当前协商及 argv：普通启动具有可观察的原生 session，
   协作启动使用独立 run namespace 和 `--no-session-persistence`。不支持时保留拒绝，
   不删参数或转为不受保护的启动。
4. 固定“未批准→批准精确版本→下一输入→纠错/过期/撤回→下一输入”的材料和预期。
   保存实际工具/HTTP 结果及 Provider 输入；正确答案必须可由这些材料追溯。
5. 续作单独验证同一原生 session 的累计 exposure、来源撤权、丢响应后的原键恢复，
   以及首轮/后续物理启动拒绝。普通 issue/comment/Chat/quick-create/autopilot 与
   协作 root/member/continuation 的实际覆盖逐项登记，不从单个入口外推。K3 的原入口表为 issue、comment、Chat、quick-create、autopilot、协作 root、member、continuation；框架逐项接线并固定实际物理启动数后才能申请相应额度，不能先用可变 N 启动。
6. 先确认存在上游恢复依据，再开启独立窗口。第一个环境、框架或执行不完整结果
   立即停止后续轮次，保留失败与已消耗次数；不追加临时探测或自动重放业务输入。

人工页面、服务权限、实际 fake stdin 和真实模型理解分别报告。原系列用自己的账本；
新知识系列用自己的版本和样本。Windows、安装包组合、多账号/额度溯源继续归父任务
及跨平台交付矩阵，本页不声称已经覆盖。
