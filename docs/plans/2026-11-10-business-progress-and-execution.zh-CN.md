# 六项业务核对与推进计划

核对日期：2026-11-10。代码基线：`b10da8fec9dab057f9830bd336369d42e27b8c85`，分支 `codex/cross-workspace-collaboration`。依据任务记录、原始运行账本、证据 SHA256、提交祖先关系和当前源码核对；未重新 fetch 或验证部署。共享工作区已有大量未提交资料，本轮沿用现有任务，不将它们一并提交。

## 真实进度

| 业务 | 已完成且有证据 | 尚未完成 | 下一步 |
| --- | --- | --- | --- |
| 跨账号共享协作 | 历史96/97；三个子任务归档；AC43四类场景中三类通过；独立模拟三账号闭环通过 | AC43跨环境小队：Claude队长被供应商拒绝，成员及续作没有运行。旧候选与当前HEAD的三个固定产品文件不同 | 重绑当前候选及供应商身份，再执行完整小队链；CLI直连成功不代替原生适配器 |
| 上下文交接父任务 | 实现和确定性修复已合入 | 首轮、runtime、知识三个真实验收门仍开着 | 汇总子任务证据，不先行归档 |
| 首轮修复闭环 | V16、password7、E3后续/E4通过；H5仅引用canary的独立回归已合入 | E5旧冻结基线t1是有效输出判据失败，非供应商故障；19次已用、8次未用，后续停止 | 单独评审E5失败处置；不篡改旧判据、不补跑、不借用其他预算 |
| 可靠交接与召回 | normal/race各62；E0a、H1/H2/H2b、H3a t1通过 | H3a t2五分钟内无最终结果，根因未确定；12/25已用、余13；后继未运行 | 复用已有13槽位续作提案，先准备继任入口审查；真实续作需明确的历史停止例外 |
| 知识沉淀 | 完整续作及历史输出保护已合入；完整fake普通/race各104，后续控制普通/race各119 | 两次E0b均403；2/17已用，K1/K2/K3未跑。另有CLI字节漂移：登记绑定165/166，当前文件SHA已变化 | 先解决版本/字节绑定，再取得合法原生接入证据；重绑源码、配置、数据库和窗口后跑剩余15槽 |
| 智能体与小队研发优化 | M0、任务最小权限、即时委派回执、关键路径CI及定向测试已合入；7个子任务归档 | 受控制品已通过规划复审、待实施；证据门禁、持久化回执、M2/M3未完成 | 先关闭受控制品设计审查，再实施M1；其余按依赖分包 |

“7/8子任务”不等于路线完成87.5%；M2/M3尚未拆成可验收子任务。代码合入也不等于真实模型或部署验收。

## 新发现与第一批推进

现有业务QA记录落后于文件和运行证据。S1–S3测试已存在，原 `b1-support-governance` 实际为S2/S3共 **0通过、2失败、0跳过、0重试**；S1没有运行证据。继续沿用 `11-09-collaboration-business-scenario-qa` 已批准范围，只修改本任务的新测试文件。

1. S2向严格的`/ai-policy`请求体多传`execution_view_version`，在权限校验前返回400；改为正确workflow字段并保留403断言。
2. S3存在两个“Resume target”按钮，原单目标helper没有限定资源；新增本任务专用目标定位，并联结点击与响应等待，避免清理时的page closed掩盖原错误。同类workflow请求一并核查。
3. 重新证明任务私有环境的API commit/PID、生产Web和renderer归属，冻结源码零漂移；首轮失败保留，以新run ID串行重跑S2/S3，再开知识配置跑S1。
4. 保存每场景业务终态、Provider实际运行、退出证明、附件/知识观察、清理及凭据扫描；产品缺陷如实记录，不修改产品以求全绿。

已在本轮重验私有环境归属。源码审计发现唯一差异是Next构建生成的`next-env.d.ts`路由类型路径；保留私有备份后恢复候选字节，再审 **27,607个文件、0漂移**。本轮采集器纯Python控制测试 **11/11通过**；这不是新的真实模型验收。

## 推进队列及退出条件

| 顺序 | 执行包 | 前置与交付证据 | 完成条件 |
| --- | --- | --- | --- |
| P0，现在 | 业务QA设施修正与S1–S3 | 已有独立环境/确定性Provider；新run ID，保留首次失败 | 每个场景运行结果与业务终态分列；报告、清理、源码审计、扫描齐全；产品失败进入问题清单 |
| P0，并行 | 受控制品规划收敛 | 消除批准前/后冻结、claim建行/改行、legacy状态三组矛盾；核对真实审批权限和查询泄漏边界 | 独立复核六类审查阻塞全部闭合，PRD/design/implement/manifests一致 |
| P1 | 受控制品M1实现 | 仅local-synthetic确定性tar；先冻结规格与pending审批，同事务创建未领取记录；unknown先落库再执行 | 私有库验证一次性claim、撤权/取消竞态、旧接口拒绝伪造、文件边界、未知结果不重跑、回滚拒删证据；独立检查通过 |
| P1 | 原生接入与知识版本绑定 | 不修改SDK身份绕过网关；既有403请求资料可供核查，发送另需授权；先离线确定原绑定二进制或显式版本迁移合同 | CLI/hash/config/源码一致，合法原生调用被接受，预算台账与窗口独审通过 |
| P1 | E5、H3a独立续作处置 | 原E5有效失败与H3a未完成保留；分别决定停止规则例外，不共享预算 | 经批准的继任登记、历史哈希、完整借记、首个新失败停止；未执行项绝不标通过 |
| P2 | M1证据门禁、持久化委派回执 | 复用现有身份、lifecycle、trigger_outcomes；独立PRD和验收矩阵 | 旧SHA/过期/缺失证据拒绝，回执可追溯且幂等；旧客户端兼容 |
| P3 | M2报告快照、总预算与调度 | M1身份/证据已闭合；先明确统计窗口、时区、价格及未知usage口径 | 确定性快照、并发预留不超发、重试不逃预算、取消/迟到结算幂等 |
| P4 | M3持续质量与治理 | M1自动证据及M2预算 | NFR故障样本、模型评测版本化、制品供应链、模板定制保留、知识失效可验证 |

不承诺缺少基线的工期；每包用上述退出条件推进。父任务仅在所有自身验收门关闭后完成。

## 证据入口

- [AC43历史真实报告](../../.trellis/tasks/10-10-cross-workspace-collaboration/research/ac43-20261105/report.md)
- [最新原生403与CLI对照](../../artifacts/qa/real-codex-20261109/report.md)
- [首轮E5记录](../../.trellis/tasks/10-30-collaboration-fix-closure/verification-2026-11-05.md)
- [H3a诊断](../../.trellis/tasks/10-30-collaboration-context-handoff-runtime/research/continuation-20261105/h3a-entry/failure-diagnosis.md)、[13槽位续作提案](../../.trellis/tasks/10-30-collaboration-context-handoff-runtime/research/continuation-20261105/continuation-proposal.md)
- [知识预算和顺序](../../.trellis/tasks/10-30-collaboration-knowledge-promotion/continuation-plan-2026-11-05.md)、[104次模拟证据](../../.trellis/tasks/10-30-collaboration-knowledge-promotion/research/codex-real-20261105-plan/continuation-verification.md)、[119控制证据](../../.trellis/tasks/archive/2026-11/11-08-merge-unmerged-worktrees/research/guard-tests-integration.md)
- [受控制品原始阻塞审查](../../.trellis/tasks/11-06-controlled-artifact-execution/plan-review.md)
- [业务QA首轮失败](../../artifacts/qa/collaboration-business-scenarios-20261109/b1-support-governance/playwright.json)

本轮执行结果随后追加到下方，历史失败文件保持原样。


## 本轮落地结果

- 新业务QA最终选定结果：S3/B3、S2/B5、S1/B6共3/3通过；历史主线复现4/4另列。12次受控Provider运行、12份host退出证明；保留四次失败尝试。见[完整报告](../../artifacts/qa/collaboration-business-scenarios-20261109/report.md)。这些是确定性Provider场景，不提升AC43、E5、H3a、知识真实模型验收状态。
- 仅修正两份本任务新测试：严格workflow请求字段、按目标定位、返工显式转交、抽屉关闭等待，并将知识权限拒绝收紧为精确403/code。定向TypeScript、ESLint和3项用例收集通过；没有产品代码改动。
- 自有API/Web/renderer已停止，三端口释放，知识off、执行和host闸门关闭，数据库/日志保留。最终源码27,607文件零漂移、六二进制原哈希一致。原扫描15项公开标识符误报保留，按精确公开上下文逐项复核后实际凭据命中0；不冒称原扫描器零命中。
- 受控制品design/implement已统一，补齐实际锁序、一次claim、未知结果、确定性打包、回滚先fence、工作区删除保留审批与回执。两轮独立复审后规划合同就绪，产品实现仍未开始。见[复审](../../.trellis/tasks/11-06-controlled-artifact-execution/plan-recheck-2026-11-10.md)。
- 六项业务继续保持in_progress；已同步知识当前入口与CLI漂移、研发M1下一步。未创建新总任务，未发送外部消息、未启动真实付费模型、未部署。

下一批优先实施已审查的M1受控制品子任务；真实验收线并行完成CLI绑定/合法网关准入及E5、H3a停止规则的独立处置。
