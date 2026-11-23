# PAY-241 Continuation Implementation Plan

> 历史推进记录：以下状态与验收数字保留于文档所述日期。相关任务已迁入归档目录，引用已更新；当前进度以归档任务的验收矩阵和后续记录为准。

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 完成 S2 最后一个实际场景及独立验收，再按既有依赖推进 S3 恢复、S4 真实集成和 S5 总验收。

**Architecture:** 沿用现有任务树、当前权限、精确版本、原键恢复与事务完成门禁。复用已接受的生产 Web/Desktop 和 Go 产品；新测试源、运行配置及实际结果分别绑定，保留所有历史失败与业务回执。

**Tech Stack:** Go、PostgreSQL/pgx/sqlc、React/TypeScript、TanStack Query、生产 Next.js、Electron、Playwright。

---

## 当前进度：R28 预览读取并发回归

R27 的提交绑定与 UTC 到期时间修复已独立接受为 **ACCEPTED_R27_BINDING_UTC_NORMAL_RACE**。普通/竞态各2顶层/6叶/8命名PASS，0FAIL/SKIP；有效提交、M1 off、三个拒绝控制和实际密码管理员接管当前提交均通过，72关系/阶段/管理员/适配器无写入。两处局部修复与两份回归已按哈希接入ROOT，ROOT格式、service vet和编译全部通过。原提交RED、管理员RED、R01时区FAIL和未执行race保留。见 [R27独立接受](../../.trellis/tasks/archive/2026-11/11-13-pay-241-s2-stage-approvals/research/r27-preview-binding-utc-followup-independent-check-20261117.md)。

当前继续原S2，下一批先只写一个新的同层读取并发测试：实际standing-QA/release R4与完成预览两种顺序、同目标两预览、原writer两顺序/普通与marked阶段writer、账号/密码/制品manager/原QA issuer撤权，以及缺失提交绑定/真实退回阶段。库存为5顶层/13叶/18命名；源5050文件与三个fresh service库已准备，产品锁尚未修改。独立接受绑定增量后再取得该批真实RED，仅据已到达的错误限定纯读取修复。

R28真实RED已完整终结：**13叶10PASS/3FAIL，18命名13PASS/5FAIL（含两个父失败）**，0SKIP/缺失/额外/未终结/漂移，测试库连接0。三个读取共存叶分别到达真实root-collaboration UPDATE、artifact-child共享读和root-collaboration共享读的 `55P03`；10项writer/当前权限/绑定/真实退回控制通过。当前新GREEN副本仅批准预览、协作intent和stage-completion三个Go文件，复用已有read marker/共享访问与观察路径，原writer常量false和实际排他锁保持。原R27与602行新回归完全保留，fresh normal/race及独立接受尚待执行。

推进顺序：**R28真实RED → 保留writer默认的纯读修复 → fresh normal/race与独立接受 → 匹配Go产品/新八槽位/配置/日志 → 原四场景与A01–A12/正常恢复 → S3 → 真实S4 → 最终S5/父24条。** 最近完整四场景仍R25的1PASS/1FAIL/2NOT_RUN、retry0；父24条仍0PASS/9PARTIAL/15NOT_RUN。单元通过不能替代完整预览、两端或业务验收。

## 保留检查点：R27 完成预览回归

重新核对上个会话的封存结果：R26 候选读取增量已独立接受，普通与竞态各 **4 顶层 / 11 叶 / 15 命名 PASS，0 FAIL / SKIP**。两个产品文件和一份回归已接入 ROOT，当前字节与接受版本一致；ROOT service 编译与 vet 的实际 exit0 记录也已取得。原 RED 四个读取共存叶失败、原 writer 与五项当前权限控制通过的证据保留。见 [R26 独立检查](../../.trellis/tasks/archive/2026-11/11-13-pay-241-s2-stage-approvals/research/r26-candidates-read-green-independent-check-20261117.md)。

最近完整产品运行仍是 **R25：1 PASS / 1 FAIL / 2 NOT_RUN，retry0**；候选读取增量尚未重新运行原四场景。父账本保持 **0 PASS / 9 PARTIAL / 15 NOT_RUN**，不按单元测试数估算完成率。

R27 已取得并独立接受真实提交预览 RED：两个有效提交正例均返回 `verification/stage_submission_unbound`，三项当前版本/权限/凭据拒绝控制通过，5叶完整终结、0SKIP/漂移/残留连接，72关系与阶段/适配器状态无写入。原因是临时 guard 未初始化精确提交ID，尚未进入不可变提交查询。现在在新私有GREEN中仅修复该初始化，并在保留原产品的新副本补管理员接管当前提交的真实回归；额外纯读锁仍另行验证。见 [独立RED边界](../../.trellis/tasks/archive/2026-11/11-13-pay-241-s2-stage-approvals/research/r27-preview-submission-red-independent-check-20261117.md)。

本轮继续既有 S2，按以下顺序推进：

1. **已完成：**核对 R26 独立接受、三份 ROOT 字节、真实 ROOT 编译/vet 与旧结果；同步父子进度和计划。
2. **提交RED已完成，管理员回归进行中：**已分配新的私有源码与service专库，真实验证阶段、`SubmitAcceptance` 和借用事务复现了 `boundSubmission` 分支。另补实际密码管理员、独立签字人和失能发起人的当前提交预览；预览/R4两连接、同目标互读和原writer控制的下一批库存已研究。
3. **最小GREEN进行中：**仅初始化临时预览guard的精确提交ID，保留当前权限、提交/原操作/交接校验、版本、期限和writer默认锁。管理员同分支用独立真实RED验证。额外读锁须在绑定修复后另取真实双连接RED，分别归因、分别接受。
4. **独立接受：**同一回归来源在新库完成 normal/race、受影响的原读写控制、格式/编译/vet；候选增量与完整预览分别记录接受范围。
5. **S2 最终验收：**重建受影响的匹配 Go 产品，保留已接受的生产前端；新八槽位、配置、日志和来源绑定后执行原四场景一次。全部通过且 A01–A12、正常停机与历史恢复独立接受后，才关闭 S2。
6. **后续依赖：**S3 并发/CAS 与恢复 → S4 真实非生产 E01–E07 → S5 最终候选的 100 人、10,000 传播样本和 100 次写回 → 父24条及业务签字。I02–I09 外部输入仍缺失，相关真实用例保持开放。

主会话协调、分配环境与更新进度；原生 Trellis 实施/检查代理分别持有明确文件。本轮不创建新任务、不新增依赖、不提交 Git，保留旧数据库、失败、提交、回执和制品。

## 保留检查点：R26 续接

R25被动采集已独立接受并执行原四场景，但在第二项QA读取提前失败，最新结果 **1PASS/1FAIL/2NOT_RUN，retry0**，原结果`4e4b2718…`保留；没有到达M1采集点。API已正常停止，连接0、epoch48、43run结算/28artifact完整。

下一步改为[确定性候选读取并发回归](../../.trellis/tasks/archive/2026-11/11-13-pay-241-s2-stage-approvals/research/r25-m1-read-coexistence-test-plan-20261117.md)：新同层测试4顶层/11叶 → 明确空库RED → 根据真实错误做仅候选纯读修复 → freshGREEN/race/原writer控制及独立接受。三个RED/GREEN/race库已分配；初始只写新service测试，不提前改变产品锁。另用真实managed-submission回归处理preview的额外边界与boundSubmission初始化。然后恢复最终原四场景/A01–A12，后续S3–S5依赖不变。

## 保留检查点：R25 起点

本计划第1步的工具栏操作修复已独立通过，第2步的新来源/运行绑定与原四场景已完整执行一次。最新为 **R24：3PASS/1FAIL/0NOT_RUN，retry0**，原结果SHA256 `9ed5f3d54e38cd718fd5a97d84f1e13df4d749eb8bf309d5e3534742f42a84a3`。第四项已真实提交结果200，随后M1完成预览helper等待POST响应20秒超时；M1授权和最终完成未到达。

本次自有API/bridge已正常停止，harness终结PASS、连接0、gate恢复epoch44、39run结算/26artifact完整，全部来源/产物无漂移。R25研究已确认helper/Core/router路径一致；实际候选GET返回409，原诊断白名单未包含此GET且先前4096条已用完，未取得正文/SQLSTATE/holder。

当前最小写域只有新私有harness源的 `server/cmd/server/pay241_read_diagnostics_test.go`：白名单收窄为canonical UUID GET candidates，保留4096记录/1MiB正文Tee、pgx关联及0600sink。不改前端helper、生产锁/SQL/权限或原5/20/480秒；原R24前端/三Go产品复用，仅编译匹配testharness。独立接受后用新八槽位/配置/日志执行原完整四场景一次，取得实际拒绝证据后才决定产品修复。新239路径保留231旧路径，旧submission与失败不重放。见[R25原因报告](../../.trellis/tasks/archive/2026-11/11-13-pay-241-s2-stage-approvals/research/r25-m1-preview-diagnosis-20261117.md)。

父24条及S2 A01–A12/S3–S5保持开放。以下初始R24检查点和执行步骤保留，继续按未完成门槛推进。

## 保留检查点：R24 起点

2026-11-17，Asia/Shanghai。继续现有 `.trellis/tasks/archive/2026-11/11-13-pay-241-s2-stage-approvals`，不创建新任务。最新实际结果是 **R23：3 PASS、1 FAIL、0 NOT_RUN，全部 retry 0**，替代进度账本里尚未同步的 R22 1/1/2。R23 原结果 SHA256：`440d404d437fa34b1e808e56ffefa98bca30ea2768cb8970db3466f24db7a21f`。

| 阶段 | 当前事实 | 下一完成条件 |
| --- | --- | --- |
| S1 身份基线 | 已完成并归档 | 保留原范围 |
| S2 角色审批与门禁 | 服务端、共享客户端与 R23 回执解析已独立接受；前三个实际场景通过 | 修复第四场景操作路径，完整四场景通过，A01–A12 独立接受及资源恢复 |
| S3 执行与协作恢复 | 受控超时证明已归档，剩余恢复计划 v2/r05 已评审 | 最终 S2 绑定后执行并发字段/CAS、评论/WS、提及、上传与阻塞信息 |
| S4 外部集成 | I02–I09 实际非生产输入未登记，E01–E07 未运行 | 取得真实仓库/CI、支付/履约、回调、发布/回滚、指标及人员权限输入 |
| S5 性能与总验收 | 工具与 r02 演练已完成 | 最终 S2–S4 候选的正式采样及业务签字 |

父账本暂保持 **24 条：0 PASS、9 PARTIAL、15 NOT_RUN**。三个合成场景通过不自动变更父任务结论。

第四场景失败发生在原 20 秒 Submit result 点击：测试先打开不含该动作的 More actions 菜单，再点击菜单外的工具栏按钮，正常 modal 遮罩拦截了点击。尚未执行后面的正例提交、Web/Desktop M1 完成、原键恢复及管理退出。详见 [R24 原因与写域](../../.trellis/tasks/archive/2026-11/11-13-pay-241-s2-stage-approvals/research/r24-submit-result-selector-diagnosis-20261117.md)。

## 1. 同步账本与修复单处测试操作

**责任：**主会话同步账本；`trellis-implement` 负责单文件修复；独立 `trellis-check` 审查。

**文件：**

- `e2e/collaboration-stage-execution.spec.ts`
- S2 `research/progress-20261117.md`、`implement.md`、`task.json`
- 父任务 `session-progress.md`、`gaps.md`、`task.json`
- 新 R24 实施/独立检查报告

1. 重新解析 R23 原结果、reporter、第四项 HTTP 和真实失败日志，保留 3/1 与原期限。
2. 删除该处无条件 More actions 打开，直接点击 exact、唯一的工具栏 Submit result button；删除未使用的 import。
3. 保留后续所有字段、状态码、请求身份、批准、M1、两端恢复、R38 和退出断言；不使用 `.first()`、force 或增加超时。
4. 执行实际覆盖该根目录 E2E 文件的 lint、严格类型检查与四场景发现；独立复核单文件差分。

**门槛：**单文件改动通过检查；原 R23 是保留的真实 RED，新运行提供 GREEN。无需添加只复述选择器实现的单元测试。

## 2. 恢复运行环境并执行完整四场景

**责任：**主会话。独立环境检查代理只核证据与所有权。

**文件：**仓库外 `/Volumes/artisan/.multica-acceptance/pay241-r24-continuation-20261117-32uud2uo/` 的新运行配置、来源绑定、日志及结果；旧 R23 文件只读。

1. 记录旧 API/bridge PID 已不存在、64922 无监听和旧 socket 仍在的事实；不将它冒充正常停机收据。
2. 使用既有只读 SQL collector 核当前自有数据库身份、连接、gate、Host run、租约、预留、制品及全部保留历史；不修改旧队列或 unknown 意图。
3. 停机时分配八个全新 0700 staging 槽位，完整保留已有 188 路径，恢复 staging 根 0500。使用新 socket、ready、日志和输出目录，保留旧残留文件。
4. 复制已有冻结测试源，仅整合已检查的单文件修复并更新依赖根链接；保留已接受产品与原角色/配置。新来源绑定核对全部实际消费输入及产物。
5. 启动唯一匹配的 `stage-router.test` API/bridge，核生产 Web 64925、Desktop renderer 64926、原管理员登录及新 live-before。
6. 运行新绑定的原命令一次：

   ```text
   node22 <new-runtime>/run-original-four-scenarios-r01.mjs <new-four-case-permit.json> <sha256>
   ```

   实际 Playwright argv 保持 `test --config <bound-config> --workers=1 --retries=0 --max-failures=1`，原 action 20 秒和 case 480 秒不变。
7. 失败时保留完整四分母、失败位置与未运行项，按实际责任层继续诊断；不重放旧已成功的回滚请求或 sealed generation。

**门槛：**四个实际场景全部通过，并有对应的新 source/config/product/runtime 与原始结果绑定。

## 3. 关闭 S2 的证据门槛

**文件：**S2 A01–A12/W01–W18/Q01–Q12 地图、新 R24 运行后/停机/独立最终报告、父 `test-matrix.csv`。

1. 逐项对照新运行和已有同层普通/竞态证据，保留 W10 普通 CLI 独立拒绝记录，不累加重叠测试数。
2. 核真实提交、唯一授权与最终写入、Web GET404/原键恢复、Desktop 消费同一授权、R38 及项目退出拒绝。
3. 正常停止本次自有 API/bridge，核连接 0、原 gate 布尔恢复、epoch 单调、run 结算、无活动租约/未释放预留及制品历史完整。
4. 独立接受全部 A01–A12 后才归档 S2；父矩阵仅按每条完整要求和实际证据更新。

**门槛：**完整 S2 独立接受，剩余外部业务及父任务要求仍按各自证据保留。

## 4. 推进 S3 第一批恢复证明

**文件：**现有 S3 `{prd,design,implement}.md` 和上下文清单；service/handler 同层测试，具体新文件在激活时分配。

1. 绑定最终 S2 来源、锁 helper 与接受报告后激活现有 S3。
2. 先测试 priority 对 priority 两提交顺序、priority 对 transfer 两顺序、普通 child priority 对 assignee 两顺序及一个 omitted-revision 控制。
3. 使用真实事务提交屏障和两个 PostgreSQL backend。transfer 先提交时保留原处理人编辑的 403，不将其改成统一 409。
4. 有真实 RED 才扩大到产品修复；普通/竞态检查按 service/handler 独立自有库执行。
5. 之后按已审 v2/r05 推进 55 评论补读与 WS 重认证、混合提及去重、上传未知结果/迟到结果及四项阻塞信息，冻结公共 DTO 后再接 Core/Views。

**门槛：**所有剩余 S3 要求有独立接受证据。准备阶段参考 [S3/S4 最小切片](../../.trellis/tasks/archive/2026-11/11-13-pay-241-s2-stage-approvals/research/r23-s3-s4-next-slice-20261117.md)。

## 5. 真实 S4 与正式 S5

**文件：**现有 S4 intake/register、E01–E07；S5 harness/research；父24条账本。

1. 主会话更新 I01/I10 内部绑定，复用现有 intake 字段，列明 I02–I09 缺失的目标、读权、故障注入与恢复范围；不猜业务 API 或复制凭据。
2. 实际输入到位后执行 E01–E07；受控 adapter 与手填记录仅证明平台合同。
3. 最终 S2–S4 候选稳定后，用 100 个独立参与者采集 10,000 传播样本与 100 次独立 run 写回。
4. 独立重算传播 p95 ≤2 秒、写回 p95 ≤5 秒，保留缺失样本与所有失败；完成资源恢复及实际业务签字后核销父24条。

**门槛：**真实外部输入、最终性能和业务验收完整。缺输入时继续可独立开展的内部工作，相关真实案例保持开放。

## 执行约束

主会话协调、集成和更新进度；实现/检查按指定文件分工。保留他任务改动、旧数据库、失败、回执与全部制品，不新增依赖，不使用真实模型账户、资金或生产部署。本轮不提交 Git；全局最多两套数据库执行，PAY 只占一套。
