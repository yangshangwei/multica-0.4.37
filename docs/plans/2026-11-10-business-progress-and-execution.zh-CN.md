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
- [受控制品原始阻塞审查](../../.trellis/tasks/archive/2026-11/11-06-controlled-artifact-execution/plan-review.md)
- [业务QA首轮失败](../../artifacts/qa/collaboration-business-scenarios-20261109/b1-support-governance/playwright.json)

本轮执行结果随后追加到下方，历史失败文件保持原样。


## 本轮落地结果

- 新业务QA最终选定结果：S3/B3、S2/B5、S1/B6共3/3通过；历史主线复现4/4另列。12次受控Provider运行、12份host退出证明；保留四次失败尝试。见[完整报告](../../artifacts/qa/collaboration-business-scenarios-20261109/report.md)。这些是确定性Provider场景，不提升AC43、E5、H3a、知识真实模型验收状态。
- 仅修正两份本任务新测试：严格workflow请求字段、按目标定位、返工显式转交、抽屉关闭等待，并将知识权限拒绝收紧为精确403/code。定向TypeScript、ESLint和3项用例收集通过；没有产品代码改动。
- 自有API/Web/renderer已停止，三端口释放，知识off、执行和host闸门关闭，数据库/日志保留。最终源码27,607文件零漂移、六二进制原哈希一致。原扫描15项公开标识符误报保留，按精确公开上下文逐项复核后实际凭据命中0；不冒称原扫描器零命中。
- 受控制品design/implement已统一，补齐实际锁序、一次claim、未知结果、确定性打包、回滚先fence、工作区删除保留审批与回执。两轮独立复审后规划合同就绪，产品实现仍未开始。见[复审](../../.trellis/tasks/archive/2026-11/11-06-controlled-artifact-execution/plan-recheck-2026-11-10.md)。
- 六项业务继续保持in_progress；已同步知识当前入口与CLI漂移、研发M1下一步。未创建新总任务，未发送外部消息、未启动真实付费模型、未部署。

下一批优先实施已审查的M1受控制品子任务；真实验收线并行完成CLI绑定/合法网关准入及E5、H3a停止规则的独立处置。


## 后续推进实绩：受控制品子任务已完成

`f5b51d7be`已提交并合入当前分支，受控制品子任务已独立验收归档。实现审批前冻结规格、当前权限复核、一次领取、确定性本地tar、未知结果不重跑、删除后审批回执留证。默认无配置不可执行；没有增加部署或真实模型能力。最终handler普通/race各84通过、router普通/race各23通过，macOS纯包普通/race及Linux各78通过，构建/vet/sqlc幂等和独审通过。详见[交付记录](../../.trellis/tasks/archive/2026-11/11-06-controlled-artifact-execution/delivery.md)。

下一阶段按以下可验收交付推进，父任务子项8/8归档不等于M1或整个研发路线完成：

1. **M1交付证据门禁**：以现有任务/项目lifecycle为入口，冻结policy版本、提交/制品摘要、证据生产者、结果和失效时间；先观察再提示/阻断。验收旧SHA绿灯不复用、缺失/过期/unknown不得当pass、撤权和并发变更不越过条件写入、旧客户端兼容。先确认现有lifecycle与artifact回执的接合点，再单独PRD/设计/实现。
2. **M1持久化委派回执**：复用trigger_outcomes，将目标身份、触发评论、排队/合并/延后/拒绝结果持久关联；验证重复请求、成员替换、离线/私有拒绝、队长重启后的原回执读取，不以mention文本冒充实际派发。与证据门禁共享身份但不建立第二套执行引擎。
3. **M2**：先确定报告scope/时区/[start,end)/水位和coverage，再实现只读确定性快照；总预算使用父根预留/结算/未知usage，验证重试不逃预算和并发不超发；最后接能力/负载调度。
4. **M3**：固定环境的NFR/恢复、版本化模型评测、制品供应链证据、保留定制的模板升级与知识失效。依赖M1证据和M2预算后逐包验收。

真实模型线继续独立：知识优先解决旧CLI字节绑定和合法网关准入；AC43重绑当前候选；E5/H3a分别保留原失败及明确停止例外要求。本次真实模型调用0，无新预算消耗。


## 下一批范围核对：证据观察（本增量已验收）

2026-11-10 从 a3b722a47 新建独立 worktree `mc-delivery-evidence-20261110`。现有完成写入分布在单/批量更新、创建、GitHub自动完成及独立协作流程，项目还没有证据policy存储。按既有“先观察再提示/阻断”顺序，子任务 `11-10-delivery-evidence-gate` 先实现只读、固定版本的证据观察API，不把部分入口拦截宣称为全局完成门禁。

本包pass必须同时匹配当前issue/project版本、选定提交/制品摘要、受控制品成功回执、当前生产者/审批权限及有效观察窗口；数据库未观察到关联PR head则unknown。数据库已观察head不是远程实时Git HEAD证明，调用者声明不是可信源码新鲜度。旧客户端完成行为保持；完整项目政策、持久审计及全部done路径的阻断在后续包关闭。

持久化委派回执、M2/M3顺序保持。真实模型预算与停止规则保持原状。

最终源码冻结后handler普通/race各140、真实路由各34通过，均0fail/skip；build/vet、技能合同21与runtime扫描1通过。原始设施/断言/竞态失败全部保留，未计作通过。专属容器停止，10库与日志保留，真实模型调用0。详见本子任务delivery与verification记录；完成只覆盖observe阶段，不能宣称项目完整交付门禁已实施。

观察增量实现提交 `45af89016`；[交付与剩余边界](../../.trellis/tasks/archive/2026-11/11-10-delivery-evidence-gate/delivery.md)、[独立审查](../../.trellis/tasks/archive/2026-11/11-10-delivery-evidence-gate/research/independent-review.md)。


## 后续推进：持久委派回执增量

从38d07b6ed建立独立worktree mc-durable-receipts-20261110，复用已有即时回执和实际派发，完成可选Idempotency-Key的无附件普通评论创建与恢复。保存显式target UUID、实际任务/执行者/覆盖小队、排队/合并/延后/拒绝；重启读取、同键重放及unknown不重派受到当前权限约束。用户明确选择暂时离线仍排队，并冻结当时观察，未绑定/不可用拒绝。

最终handler普通/race各342、真实认证路由各20、迁移各5通过，均0fail/skip；CLI98、指令/技能33、build/vet/sqlc幂等及36文件冻结通过。保留原失败及独立审查修复。无键/编辑/quick-action不纳入持久幂等，空targets不保证隐式补偿；queued/recorded不代表完成。参见[交付记录](../../.trellis/tasks/archive/2026-11/11-10-durable-delegation-receipts/delivery.md)。

下一包仍为M1项目政策版本与持久评估，再提示/阻断；必须覆盖单/批量更新、创建即完成、GitHub自动完成和独立协作流程。M2顺序保持快照→父总预算→能力/负载调度，M3持续治理；六项业务父任务不因本子项归档而完成。真实模型调用0，历史停止/预算不变，未推送/部署。


## 最新推进：项目政策版本与持久评估

在c3bc9abd1上以独立worktree完成项目off/observe政策追加修订、owner/admin管理及不可变持久评估。复用现有观察器与真实制品回执；同键恢复原结果并完整复核当前权限，政策/版本/提交/期限变化使适用性失效，unknown不自动升级。原观察API保持无账本，未接入任何done写入门禁；也没有skip豁免。

最终冻结26文件，handler普通/race各236、真实认证路由各80、实际迁移/注册各37通过，均0失败/跳过；技能/迁移静态/runtime合同23通过，全后端build/vet/sqlc幂等及格式检查通过。失败原始记录保留，细节见[交付记录](../../.trellis/tasks/archive/2026-11/11-10-delivery-policy-evaluations/delivery.md)。

下一包是提示与完整强制门禁：复核普通/批量更新、创建即完成、GitHub自动完成及独立协作所有入口，覆盖自定义done类别；政策转换、skip负责人/理由/权限、并发条件与旧客户端兼容须单独收敛。不能将本包历史pass/current作为可复用完成授权。M2保持确定性快照→父级总预算→能力/负载调度；M3保持持续NFR、模型评测、供应链、模板与知识治理。六项业务父任务不关闭，真实模型调用0、历史预算与停止规则不变。


## 最新推进：完成前证据提示API基础

从05de761de核对无后续提交后，新独立worktree交付human-only普通任务完成前提示，复用现有off/observe政策和原人类持久评估恢复。custom done与内置done同义；只有原pass/current提示matching，所有结果enforced/usable_for_completion均false。无key提醒，off/no-project/non-done不消费证据，撤权/丢失绑定/数据库错误明确拒绝。没有新观察器、执行器或迁移。

最终13文件冻结，handler普通/race各325、真实认证路由各103，零失败/跳过；合同50和build/vet/sqlc/format通过，原4次失败保留。专属PG0连接后停止，10库/日志保留。具体交付与限制见[提示包交付](../../.trellis/tasks/archive/2026-11/11-10-delivery-completion-hints/delivery.md)。

本包仅API提示基础，无UI/CLI自动展示、skip或任何完成写入阻断。全入口核查新增明确comment→child、move、批量逐项提交和R38管理员协作例外。后续完整强制门禁需先收敛原评估者与完成人/机器/跨账号证据、skip权限/绑定、创建done、done编辑/换项目及批量错误协议，在每个实际状态事务内复核。owner/admin政策管理已定；skip提问尚未收到答案，未据推荐自动实施。M2/M3顺序、六项业务状态及真实模型预算/停止规则保持；本批模型调用0，无推送/部署/消息。
