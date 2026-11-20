## 2026-11-20 已执行切片：TC-06 人工编辑与运行回复并发

按[精确刺激研究](research/tc06-content-stimulus-gap-20261120.md)与[最小计划](../../../docs/plans/2026-11-20-pay-241-concurrent-content-proof.md)只新增一个service测试。完整r01普通/竞态各1顶层2叶3命名PASS，31组完整业务集合、原请求和冻结来源保持；gofmt/vet/compile通过，peer新HEAD两份无关测试时钟改动经来源桥接并另compile/vet。资源已收拢，最终独立检查和定向Lore提交推送见本轮记录。TC-06仍PARTIAL、父9PASS/11PARTIAL/4NOT_RUN；真实远端来源和普通评论管理员权限仍各归S4/独立任务。父任务不归档。

## 2026-11-20 已执行切片：TC-12 本地实际代码修订回归

按[最小补证计划](../../../docs/plans/2026-11-20-pay-241-code-revision-regression.md)新增单个service测试。一顶层，普通/竞态新r03各1PASS；真实本地Git A→business-only B、固定oracle/input/argv、原失败/QA退回、新候选/run/report/QA批准、12组A历史与原key保持。原r01/r02各0PASS1FAIL及source/config/raw保留，修正仅明确Resume/Stop夹具顺序；没有产品/依赖改动。生产report/stage与test-only managed preparation/consent/settlement分列，真实支付修复仍S4，TC-12保持PARTIAL。Main完成独立检查、资源/源核对、父记录及定向Lore提交推送，父任务不归档。

## 2026-11-20 已接受切片：TC-04 真实机器完成／授权拒绝

沿用父任务及[最小补证计划](../../../docs/plans/2026-11-20-pay-241-machine-completion-proof.md)。仅新增一个实际router测试文件，复用TC-13真实claim及M1精确制品/交付夹具；两顶层、九拒绝叶、每模式十一命名。generic/restricted mat先成功读取，JWT+Agent提示独立；合法人类同body成功证明拒绝不是无效请求。完整新r02普通/竞态各11命名通过并独立接受；r01门禁响应预期失败保留。TC-04推进PARTIAL，父9PASS/11PARTIAL/4NOT_RUN；真实模型分析质量未测。真实模型质量、允许的指定普通任务执行授权和未启用策略保持各自合同，不扩大为禁止所有Agent状态写入。Main负责新隔离源/配置/数据库、记录、定向Lore提交推送。

## 2026-11-20 已接受切片：TC-01 完整正向创建补证

按[TC-01执行计划](../../../docs/plans/2026-11-20-pay-241-creation-proof.md)补充真实Web/Desktop创建、初始字段/状态/actor/time和原键身份。预登记2旅程、6授权会话、2个实际UI创建根，零重试；仅新增2测试文件。完整正向例不能证明所有PAY业务字段全局强制，也不能映射P1=High；TC-01至多推进PARTIAL、TC-03保持PARTIAL，完整r02两端2/2、独立检查已接受正向切片，矩阵TC-01更新PARTIAL。复用字节核验过的TC-02产物，新专库/API/私有状态，保留其他会话改动。TC-02/13已提交推送；当前父9PASS/10PARTIAL/5NOT_RUN。本轮定向工作/记录提交推送，父任务保持开放。

## 2026-11-20 已接受切片：TC-02 验收标准同步和历史回看

TC-02完整r02两端2/2通过并独立接受；父矩阵9PASS/9PARTIAL/6NOT_RUN。TC-13已提交推送。沿用[TC-02执行计划](../../../docs/plans/2026-11-19-pay-241-requirements-history.md)，新增双端真实三会话测试，验证description V1→V2→V3实际编辑、同步及Activity完整历史与原回执。预登记2顶层、6会话、4次显式UI编辑、18次当前版本观察和6次Activity回看；生产Web与匹配Desktop renderer/preload，零重试。Main负责隔离环境和整合，原生实施/检查代理分别负责夹具和独立验收。实际结果及剩余范围见research/tc02-independent-check-20261119.md；本轮工作和记录定向提交推送。以下TC-13及更早段落保留历史，不作为当前待办。

## 2026-11-19 已完成切片：TC-13 精确覆盖请求

S1/S2/S3已归档；父矩阵7PASS/10PARTIAL/7NOT_RUN。按[QA决定保护计划](../../../docs/plans/2026-11-19-pay-241-qa-decision-protection.md)继续父任务补充验证。当前仅新增两个Go测试文件：service两个顶层覆盖有效return/approval后机器Decide/Revoke，生产router一个顶层覆盖实际签发mat凭据的正向读取及QA纠正后的拒绝；不调用真实模型，不改变产品行为。普通/竞态按package分别使用新专库，独立验收后再更新TC-13。

# 分阶段执行草案

最新交接：S2 `research/r33-full-s2-a01-a12-independent-check-20261118.md` 已完整接受；本次执行计划为 `docs/plans/2026-11-19-pay-241-continuation.md`，实时结果记入父 `session-progress.md`。现有S3已完成字段并发、55评论补读、Extras-A R05和阻塞R08，按子任务 `implement.md` 继续普通评论事件去重、双端订阅及Extras-B受控运行。下文2026-11-15 r05及S2实施顺序保留为历史草案，其等待字段不作为当前阻挡项。

历史阶段（2026-11-15 r05）：总验收开放，S1 已归档。S2 2b 制品权限、2c 发布/回滚及客户端 3d 已有独立增量检查，E2E4 已完成编写检查但实际四场景仍未验收。2d 第一批三项修复已在 green-r03 独立通过；本次恢复保留的第二批草稿，继续精确事务锁、调用者与原操作绑定、最终证据时限，并补齐 F06、退出和普通 writer 测试。指定测试库已重新核验身份及 864 账本；P2 保持不可用，第二批完整检查后再重建联调环境运行 A01–A12。S3 超时证明已归档，完整恢复开放；S4 真实接入未运行；S5 r02 仅为已检查的工具演练。父矩阵维持 4 PARTIAL、20 NOT_RUN，本轮不提交 Git。

## 0. 规划与基线

- [x] 创建规划任务，核对当前规范、源码和同日PAY-1演练。
- [x] 保留24用例、完整目标、旧证据分类与当前NOT_RUN状态。
- [x] 明确七类状态、in_review专用语义、人工验收权和职业角色差距。
- [x] 提出首包及后续独立包，记录真实支付/发布/模型的验证边界。
- [x] 用户选择S1范围；首包PRD/design/implement在子任务收敛。
- [x] 创建S1子任务并配置上下文；没有新增后续未定案子任务。
- [x] 用户已批准最终S1摘要，仅激活该子任务。总任务不因开始首包而宣称全量已交付。

## 1. 已确认首包 S1

依赖：子任务最终方案批准；当前工作树差异重新核对。以下为概要，子任务B01–B10及具体文件归属是本包实施依据。

1. 读取 `trellis-before-dev` 与各层规范，记录基准HEAD、dirty文件、验证环境、测试Provider和二进制来源。只分配明确文件范围，不覆盖并行任务。
2. 为Agent作者公开投影新增失败回归：人类author合同不变、具体Agent可信归因、不可读来源不泄露、旧响应回退和畸形数据。
3. 使用 `trellis-implement` 子代理完成服务端/API/schema/共享视图最小改动，主会话协调边界。客户端子代理必须知道其他代理也在工作，不回退他人代码。
4. 复用canonical三账号闭环，补具体Agent展示及完整身份关联断言；对现有权限/重试/冲突/断线测试复用已有测试，避免重复测试同一规则。
5. 在任务自有DB/API与production Web运行；Desktop共享代码运行定向检查，声称Electron实际验证前必须获得对应renderer证据。
6. `trellis-check` 独立检查当前完整差异及跨层协议；修复必要缺陷，更新受影响规范。真实业务缺口保留到各自包。
7. 输出首包测试结果、变更文件、兼容性及未测项；提交只包含本包文件并使用Lore意图/验证记录。未经授权不发布生产或关闭业务任务。

建议文件归属：服务端评论DTO/投影及其测试；`packages/core/collaboration` schema及测试；`packages/views/collaboration` 评论展示与测试；canonical E2E及相关规范。具体文件在实施前查明后分配，不预先扩大到整个模块。

## 2. 后续包顺序

- S2：角色隔离和人工手建规则已确认，wire合同第13节已冻结A5代码绑定与子孙准入。续接1b/A5后由单一服务端owner顺序完成2a证据/评审、2b测试报告/QA、2c发布/回滚事实、2d所有真实writer门禁；客户端按合同并行。全部门禁齐备前政策启用保持关闭。
- S3：独立补超时/取消/重复结果、事件补读/通知、并发及阻塞字段；优先覆盖现有未证实P0。
- S4：在S2合同稳定后接入真实支付沙箱、PR/CI、发布/回滚。真实部署由人工批准，测试工具不能冒充批准者。
- S5：心跳与100socket负载契约修复、r02完整演练已完成，本地提交`8e824b303`；r01失败与完整分母保留。S1–S4必要依赖通过后在最终候选重新运行100独立用户与回写延迟验收。演练不改变TC-24状态，真实模型耗时/质量与平台受控测试分列。
- 最终：汇总八项交付物，按完整24用例及未解决缺陷判断，不汇总成虚假的部分通过率。

阶段任务已建立：11-13-pay-241-s2-stage-approvals、11-13-pay-241-s3-recovery、11-13-pay-241-s4-external-integrations、11-13-pay-241-s5-performance-acceptance。S3 timeout-proof是独立测试证明增量，不代替整个S3；任何局部通过均不关闭父任务。

## 3. 验证命令候选

规划阶段没有运行以下命令；进入实施前核对脚本与真实测试名称。DB命令必须设置任务自有测试数据库并满足 `database-test-isolation.md` / `verification-environment.md`，禁止借用旧演练DB。

```bash
pnpm --filter @multica/core test collaboration/operation.test.ts collaboration/command.test.ts api/ws-client-account.test.ts collaboration/run-handoff-queries.test.ts
pnpm --filter @multica/views test collaboration/workflow.test.tsx collaboration/delivery-panel.test.tsx
pnpm typecheck
pnpm lint
```

后端从 `server/` 对 `internal/collaboration`、实际改动的 handler/service 包运行命中的测试和 `go vet`，有事务/并发改动时加对应 `-race`。不把 `-run` 当隔离手段，不默认触发真实Agent。

完成password API、production Web、独立部署/二进制绑定后，复用：

```bash
make env-exec ARGS="-- pnpm exec playwright test e2e/collaboration-multi-account-closure.spec.ts --workers=1 --retries=0"
```

全量测试按改动风险与仓库要求执行；记录实际命令、退出码、跳过原因和新旧失败归属。无新变化或疑点不重复扩测。

## 4. 各包完成证明

| 包 | 可关闭该包的证据 | 不允许的推论 |
| --- | --- | --- |
| S1 | 作者投影/兼容/权限回归、三会话当前版本证据、真实缺口报告 | 24用例全通过、支付修复、全部职业门禁已实现 |
| S2 | 审批矩阵正反路径，精确版本/撤权/回放，所有写入口无法绕过 | 已部署生产或业务支付已正确 |
| S3 | 故障注入原始失败与恢复结果、重复/丢失检测、竞争写入证据 | 真实模型可靠性和100用户性能已证明 |
| S4 | 沙箱支付/履约日志、仓库与构建身份、人工批准和实际部署/回滚证据 | 测试账号审批可代替正式产品签字 |
| S5/总验收 | 完整样本/性能分布、缺陷处置、完整24用例和角色验收结论 | 未测或部分验证可以折算为PASS |

## 5. 本轮验证

初次规划仅做结构/路径检查。续行在新DB运行service 50/handler 31个named基线；S3 timeout-proof增量已完成normal/race各27项、两次受控真实E2E与独立检查。真实Provider账号、支付和发布未执行；新证据逐项记入test-matrix.csv的current_evidence，开放项见gaps.md。
