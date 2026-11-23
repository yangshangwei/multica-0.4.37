# S2 执行计划

## 当前验收：S2已独立接受（2026-11-18）

- [x] R29最小已消费授权读取、R31空claim预检、R33 M1策略专用等待均取得真实同层RED并独立接受GREEN；原写入口、当前权限和原stage-policy NOWAIT保持。
- [x] R33原四场景4PASS/0FAIL/0NOT_RUN，retry0；Web/Desktop原授权完成与回执恢复、策略off、管理员禁用/R38拒绝/恢复全部通过。
- [x] 当前来源/程序/CLI/制品/数据库历史与所属正常停机独立核对；连接0、gate恢复epoch90、76settled/62complete、租约及未释放预留0。
- [x] 完整12A/18W/12Q、原116service/99handler/54router及受影响R29/R31/R33、Core/Views/Web/Desktop证据链独立接受，产品阻挡项0。
- [x] PRD验收勾选、父24条2PASS/11PARTIAL/11NOT_RUN、当前缺口和进度同步；所有历史失败与已成功操作保留。
- [ ] 代码提交与任务归档。本次续接遵守不提交/不暂存约束；未把已接受代码称为已合并或已归档。
- [ ] S3恢复剩余范围、真实S4和最终S5继续按各自验收门槛推进。

最终报告：`research/r33-full-s2-a01-a12-independent-check-20261118.md/json`。
以下均为原检查点历史快照；其中未勾选项不代表当前S2仍未通过。

## 保留接续：R29 已消费授权回执读取（2026-11-18）

- [x] R27绑定/UTC与R28共享读取独立接受，联合26named及受影响原控制58named普通/race各PASS；ROOT整合、格式/编译/servicevet、五个匹配构建和普通CLI两项409零写入验证完成。
- [x] R28原四场景实际3PASS/1FAIL/0NOT_RUN、retry0。E04早期项目退出/删除探针执行；Web签发M1原授权，Desktop用同账号/原键/原请求体完成且已消费回执200；随后Web顺序恢复操作200/授权409在spec572停止。收尾/截图及管理员禁用接管恢复未到，原完成不重放。
- [x] 所属API/bridge正常停止，连接0、gate恢复epoch56、49run结算/35artifact完整；原292路径字节/权限/inode保留。R29新5051接受来源、三个service专库与8empty0700槽位已准备，root恢复0500，无新API/测试执行。
- [x] 仅新增私有558行 `delivery_completion_consumed_read_concurrency_test.go`：3top/10leaf/13named；真实同P签发/消费与已完成授权后登记72关系基线，R4/同grant/operation、实际Revoke及三权限控制保持原5秒/250ms。
- [x] 唯一新测试+5051原文件冻结、格式/编译/AST通过；真实RED命名9PASS/4FAIL、叶7PASS/3FAIL独立接受。三读叶源child/授权锁55P03，七控制PASS，十快照相同；保留夹具handoff/job/invalidation历史，不宣称全库零变化。原R28 holder仍未知。
- [x] 最小私有GREEN只改 `delivery_completion_authorization.go` 与 `delivery_completion_collaboration.go` 必要source-child读模式，共同校验主体和三个原constantfalse入口保持；558行测试与其他5050文件不变。实际格式/编译/servicevet PASS。
- [x] 同源fresh service新13+原63（7top/76named/54leaf）normal/race、directGet handler3top/6named/5leaf normal/race全部PASS并独立接受，零缺失/跳过/漂移/残留连接。
- [x] ROOT三路径哈希整合、格式/编译/servicevet PASS；五匹配程序构建PASS，当前来源/链接/产物与保留运行库/制品准备核对完成。
- [ ] 独立启动准备检查 → 唯一匹配API/实际前检/当前CLI负例 → 原四场景一次完整运行 → A01–A12/W/Q与正常停止/历史制品独立接受 → S3恢复 → 真实S4 → 最终S5。

主会话持有环境、编译/执行、集成和进度；既有Trellis实施/检查代理按明确文件归属推进。当前父24条0PASS/9PARTIAL/15NOT_RUN，完整S2开放，不新增任务/依赖或提交Git。当前计划在 `docs/plans/2026-11-18-pay-241-continuation.md`。以下均为保留的历史检查点。

## 保留接续：R28 预览纯读取并发

- [x] R27绑定/UTC增量独立接受，normal/race各2top/6leaf/8named PASS，原RED/时区FAIL/未用race保留；三文件ROOT哈希整合，格式/编译/servicevet PASS。
- [x] 新私有R02接受来源5050文件和RED/GREEN/race三个service库已准备，无产品修改/新Go执行。
- [x] 新602行测试冻结为5051source/5top13leaf18named；编译PASS，5050原文件不变；真实RED18命名13PASS/5FAIL，三个读取共存叶55P03，10个writer/权限/绑定控制PASS，无SKIP/缺失/漂移/残留连接。
- [ ] 据真实RED仅改已批准的submission纯读取分支，复用既有shared artifact/observer/access/run与stage-task fence，所有实际writer默认保持，R38/self/附件另守原边界；独立RED边界检查与新私有GREEN实施进行中。
- [ ] 同源fresh normal/race、受影响原writer控制与独立接受 → ROOT整合/匹配产品 → 原四场景/A01–A12/正常恢复 → S3–S5。

实施与检查按明确文件归属；主会话拥有环境/集成/账本。新库存及读锁边界见 `research/r27-m1-preview-regression-plan-20261117.md`。本轮不新增任务/依赖或提交Git，完整S2仍开放。

## 保留接续：R27 完成预览真实回归

- [x] R26 候选增量取得独立 `ACCEPTED_R26_CANDIDATE_ONLY_NORMAL_RACE`，normal/race 各4顶层/11叶/15命名 PASS，0FAIL/SKIP；两产品文件与674行回归已 ROOT 接入，当前哈希一致，ROOT 编译/vet 的实际 exit0 记录已核对。
- [x] 保留原 RED、R25 四场景1PASS/1FAIL/2NOT_RUN、retry0和正常停机历史；父24条仍0PASS/9PARTIAL/15NOT_RUN。
- [x] 新接受来源5049文件/1top5leaf6named被冻结；真实验证阶段/精确 `SubmitAcceptance` 的两个正例均复现 `stage_submission_unbound`，三拒绝控制通过，72关系和阶段/适配器无写入，独立RED边界接受。
- [ ] 在保留原产品的新副本补管理员当前提交的真实RED；新GREEN只初始化临时guard的精确boundSubmission，不改原提交/操作/交接校验或锁。
- [ ] 根据实际失败限定预览修改；分别验证 `boundSubmission` 与额外读锁，保留原 writer/当前权限/源身份/5秒预算，不扩大 R38 或授权写入。
- [ ] 同源 fresh normal/race、受影响读写控制和编译/vet通过，由独立检查接受预览范围，再接入ROOT。
- [ ] 匹配产品、新八槽位/来源/配置/日志 → 原四场景 → A01–A12与正常恢复独立接受 → S3；真实S4/最终S5和父24条保持各自门槛。

主会话持有环境与集成，`trellis-implement` 持有新同层测试和真实RED后明确批准的产品文件，`trellis-check` 独立审阅。当前完整S2未接受。本轮不提交Git。见 `docs/plans/2026-11-17-pay-241-continuation.md`。

## 保留接续：R26 真实读并发RED

R25原四场景因更早QA读取停止为1PASS/1FAIL/2NOT_RUN，未采到M1409。停止后epoch48/43run/28artifact/连接0保持；所有原失败保留。当前选择已封存的 `research/r25-m1-read-coexistence-test-plan-20261117.md/json` 确定性验证路径，不重复随机四场景。

1. 仅在新私有源增加 `server/internal/service/collaboration_completion_candidates_read_concurrency_test.go`，4顶层/11叶，用真实fixture/借用pgx.Tx、两个backendPID及R4/候选两种持锁顺序；原writer与当前权限负例、无写入快照保留。
2. 主会话已分配RED/GREEN/race三个明确空service库；测试源/库存冻结和命令许可后执行RED。原5秒读、默认10分钟包预算、普通pool、Source/credential/role均不放宽。
3. 真实RED且独立边界接受后，候选纯读修复初始限两个既有product文件，复用现有共享root/child/artifact读helper与仅纯读入口的marker；普通实际writer默认保持。不得只改root而遗漏内部child/principal/approval/execution排他锁。
4. 同测试源freshGREEN/race及受影响read/writer控制通过后独立接受。另行用真实managed-submission回归验证preview的boundSubmission初始化及额外读锁，不把候选增量当完整preview。
5. 最终匹配产品、原四场景、A01–A12和正常恢复通过后结束S2，再推进S3。完整父24条与真实S4/最终S5保持原门槛。

主会话拥有环境/产品整合；实施/检查各守明确文件。本轮不提交Git。以下保留历史接续。

## 保留接续：R25 M1 候选拒绝的被动采集

R24工具栏修复已独立通过，实际原四场景为3PASS/1FAIL/0NOT_RUN、retry0。第四项已提交结果200，但候选GET返回409，界面因此没有Artifact picker，随后completion-preview等待20秒失败。研究已确认helper/Core/router路径一致；原诊断白名单不包含该GET，且早先4096条容量已用完，原正文/SQLSTATE/holder未知。详见已封存 `research/r25-m1-preview-diagnosis-20261117.md/json`。

本轮最小行为差距仅是测试诊断未采到该拒绝。实施代理只在新私有源的 `server/cmd/server/pay241_read_diagnostics_test.go` 收窄 `pay241DiagnosticPath`：仅采 canonical UUID 的GET `/api/collaboration/issues/{id}/delivery-evidence/candidates`，不采R1/R2/R4/T4或preview POST。保留原4096记录/1MiB正文Tee、完整性/hash、0600独占私有sink、pgx原错误/请求身份关联。ROOT、前端helper、生产SQL/读写锁、当前权限和5/20/480秒均不改。

1. 对真实函数做AST/精确差分、capture-policy正反边界、gofmt及唯一许可的 `go test -c ./cmd/server` 匹配harness构建；不执行TestMain/DB/API。其余5046输入、三Go产品和全部前端复用已绑定字节。
2. 独立接受该测试诊断修改与新binary；主会话使用新八槽位239总路径、新日志/socket、已恢复epoch44/39run/26artifact的保留DB和匹配API。
3. 保留原R24E2E源码与全部断言，完整四场景一次执行；新的原生HTTP/PG记录用于判定409原因，不调整waiter或重放旧submission。失败保留；只在当前代码问题/验收证据齐备时推进S2接受。
4. 有真实产品缺陷再按同层真实回归做最小修复；随后新完整四场景、A01–A12与正常停机/历史独立接受。S3仍等最终S2。

R24正常停机/231制品路径/26tar已独立接受；新八个空槽位保留全部历史。父24条0PASS/9PARTIAL/15NOT_RUN不变，当前计划在 `docs/plans/2026-11-17-pay-241-continuation.md`。以下为历史接续。

## 保留接续：R24 Submit result 操作路径与最终验收

2026-11-17 重新核对最新原始记录：R23 四场景为 **3 PASS / 1 FAIL / 0 NOT_RUN，retry0**。前三项角色/评审/QA、发布/回滚、迟到部署已实际通过；第四项在原20秒点击中被测试自己打开的 More actions modal 菜单遮罩拦截，尚未提交正例结果或完成后续共享 M1/R38 流程。原结果 `440d404d…` 与所有历史失败保留，原因及单文件写域见 `research/r24-submit-result-selector-diagnosis-20261117.md`。

1. `trellis-implement` 仅改 `e2e/collaboration-stage-execution.spec.ts`：删该处无条件菜单打开及未使用 import，直接点击 exact 工具栏 Submit result button；全部后续断言和20秒/480秒期限保持。
2. 运行覆盖实际根目录 E2E 文件的 lint、严格类型与四场景发现，由独立代理审查；原 R23 是真实 RED，新实际完整运行是 GREEN，不添加只镜像选择器的测试。
3. 主会话记录旧 API/bridge 已不存在及旧 socket 残留。新只读快照已确认连接0、原门禁恢复epoch36、33run结算/19artifact完整；保留全部188路径与旧unknown意图，使用新八槽位/socket/log/source/config运行。
4. 完成唯一匹配 API、生产 Web/Desktop、已有管理员及 live-before 核对，以新绑定按 workers1/retries0/maxFailures1 执行原四场景一次，保留首失败与全部四分母。
5. 四项完整通过后核对 A01–A12/W01–W18/Q01–Q12、正常停止所属 API/bridge、门禁/资源/历史恢复和独立最终接受；之后才归档 S2 并激活现有 S3。

父24条当前仍0PASS/9PARTIAL/15NOT_RUN。主会话协调与更新账本，实施/检查代理各守指定文件；本轮不提交Git。当前推进计划：`docs/plans/2026-11-17-pay-241-continuation.md`。以下为历史接续记录。

## 保留接续：R23 条件版本令牌回执

2026-11-17 最新原四场景为 R22 **1PASS/1FAIL/2NOT_RUN**。第一场景通过，第二场景的回滚已成功持久化，但 `collaboration-stage-control.ts` 将合法 `calls.last.conditional_version = "1/after/<intent>"` 当作正整数验证。原请求61b保留且不重放；原 R20 持锁者仍未证明。

Trellis 实施代理仅在私有两文件源中补回归并修正该字段为既有 bounded UTF-8 `text(...,256)`；真实 RED14/2，GREEN16/0、strict 类型检查和实际两文件 lint 通过。独立检查已封存 ACCEPTED、阻挡项0；本次主会话按 ROOT 字节/权限守卫接入协议和 owning Node 回归，并同步新的隔离测试副本。接入后的16例、strict类型和实际两文件lint已独立通过；当前3359/1961/1676前端输入/链接/产物及5047Go输入/4程序已核对，可复用原生产Web/Desktop和R22Go产品。

主会话已停止 R22 自有 API/bridge，并取得实际停止后连接0、epoch28门禁恢复、27run结算/12artifact完整及145路径/12tar历史核对；独立恢复检查已封存ACCEPTED。本次已在停机时分配新八槽位、保留全部历史并恢复根0500；新配置/log/socket/来源已准备，before-API完整行与R22停机快照一致。下一步完成单一API/管理员/平台实际预检及before绑定，以原workers1/retries0/maxFailures1、断言和期限跑原四场景。W10普通CLI拒绝证据已独立接受，完整A09/A01–A12仍依据最终场景与证据门槛。最新记录在 `research/progress-20261117.md`。以下旧接续记录保留。

## 当前接续：R22 原四场景运行

R21 原回执读锁最小修复已独立接受并窄整合 ROOT；普通 6 条命名、竞态 9 条命名、service vet 与四个 Go 产品构建通过。上一会话停在实际 API/管理员/平台前检后，原四场景启动数 0。当前继续执行，不重复实施该修复或重跑已接受的代码控制。

R22 源码研究又定位到独立的只读入口：`EnsureCapabilities` 每轮先读能力 GET，匹配时不发 POST；`RuntimeCapabilities` 无持久写入，却通过原 managed password-session 路径拿排他用户锁。先在已绑定的 R21 私有源上仅新增同层双连接回归，证明同一 QA 账号的能力 GET 与实际 R4 两种读顺序，并保留真正注册 writer、账号和 binding 失效控制。三个新专库已准备；实际 RED 之后才允许该 GET 专用的最小共享账号读锁修复。原 R20 持锁者仍未证明。

1. 记录当前 ROOT 与已接受两文件的字节一致性、消费产品及保留平台来源；重新只读核对数据库、门禁、运行/制品/预留与历史，确认单一 PAY API 所有权。
2. 使用新配置路径、socket、ready、日志和被动诊断文件恢复匹配 API；保留已有数据库、管理员、staging generation 及所有原证据，核实本轮未用槽位。
3. 以新 before/来源绑定运行原完整四场景：workers1、retries0、maxFailures1，原 5/20/480 秒期限不变。新失败按实际层诊断，不自动重跑或放宽权限。
4. 补齐 W10 的匹配普通 CLI 拒绝收据和 A01–A12 独立证据；实际 after、门禁恢复及自有进程清理通过后才关闭 S2。
5. S2 最终接受后绑定已评审的 S3 v2/r05；S4 真实输入和 S5 最终候选门槛按父推进计划保持。

当前最新完整四场景仍为 R20 的 1PASS/1FAIL/2NOT_RUN，父24例为0PASS/9PARTIAL/15NOT_RUN；实际 R20 排他锁持有者未证明。主会话持有环境与进度，Trellis 子代理按指定文件实施/检查。

## 当前接续：R21 原回执读取与证据读取共存

R20共享sourceUnavailable显示修复、Views22/Core44、类型/lint及匹配Web/Desktop构建已独立接受。实际四例为1PASS/1FAIL/2NOT_RUN：第一例完整通过，第二例QA表单因最后R4共享用户锁55P03/HTTP503保持禁用，而T4报告仍有效。实际锁持有者尚未证明，不把后续静态候选当作该原失败根因。

1. 使用R18已接受canonical源的新私有5044文件副本，仅新增`collaboration_workflow_read_concurrency_test.go`。真实artifact-bound stage fixture取得原review批准/回执，再通过真实PostgreSQL屏障验证GetWorkflowOperation与R4两种读顺序应共存；5秒读取期限保持。
2. 保留真实DecideStage(return)排他用户写锁下R4的55P03/零投影控制，以及wrong_actor404、disabled_account403。所有SQL结果、授权、原回执、source身份来自现有fixture与实际代码，不注入假返回。
3. 冻结RED5045文件/4顶层6命名5叶库存，独立检查边界后在主会话分配的三个空red/green/race专库中先执行唯一RED。原代码预期两个读共存失败和三个控制PASS，全部原结果保留。
4. 只有RED实际证明后，限定GetWorkflowOperation读取入口复用原共享LockArtifactUsers；保留原authorizeWorkflowAccount、单actor身份/原键/历史scope和decode，普通writer的LockCollaborationUsers及其他回执GET不变。新修复先normal→race、相关原回执/撤权控制和scoped vet独立接受，再窄整合ROOT。
5. 当前R20真实锁持有者可另做有界被动记录，不能改权限、读隔离、自动重试或原5/20/480秒验收。最终仍需另一轮原四场景、A01–A12及W10匹配CLI负例，完整S2接受后再激活S3。

环境与执行许可由主会话持有，API已停止，PAY只运行一个DB套件；知识可独立运行另一套，合计最多两个。R20after独立通过、门禁恢复epoch24、历史队列/unknown/制品全部保留。计划入口为`docs/plans/2026-11-16-pay-241-continuation.md`。

## 当前接续：r16 候选读取 503

已恢复同一任务；正式四场景为 1 PASS / 1 FAIL / 2 NOT_RUN。原 QA 场景和两次制品打包的成功证据保留，第二场景的 reviewer 候选 GET 503 尚无正文/SQLSTATE，表单继续按读取错误锁定。

1. 独立核对原输出与父 24 例，只将已有实际评审/QA 证据的 TC-08、TC-12 更新为 PARTIAL。保留业务代码修改后回归和完整 A11/A12 缺口。
2. 在既有冻结源的新私有副本加入 test-only pgx 查询错误 tracer 和原 E02 当前 reviewer 的被动 GET 响应采集。先检查/编译和独立复核；不改业务请求、角色、权限、读隔离、pool、20 秒/480 秒、原点击或重试规则。
3. 在 API 停止时分配八个全新 generation，保留所有旧目录/文件/业务历史；恢复匹配 API、production Web/renderer 和原管理员，再以新绑定执行一次原 E02 诊断。该结果不计入正式四场景。
4. 依据实际响应与底层错误确定最小修复边界，先在同层取得真实失败回归，再修复并独立检查。保护来源成员/当前权限锁与所有最终写入门禁。
5. 修复来源绑定后，使用另一组新 generation 执行原完整四场景，再独立逐项核对 12A/18W/12Q。S2 尚未接受前不归档或开始 S3 全面实施。

主会话负责环境、进度、集成与最终证据；当前代码调查与诊断副本分别由唯一研究/实施代理负责。本轮继续不提交 Git、不新增依赖、不调用真实模型账户、资金或生产部署。

## 最新夹具修正接续（r13）

本轮已经实际通过修复后的代码评审流转、人工签字/撤权/原键/源权限子路径，并完成一个bareHost受控运行。第二个资源启用时，全AI execution区的同名Resume target选择器匹配两个合法按钮；仅夹具须按真实当前target/resourceId定位，不改产品控件、权限、scope或原20秒480秒期限。

新private helper先从当前认证页面GET严格解析任务，精确targetId查resource_id，沿现有资源行+按钮双has/count1模式点击。小队target.resource_id与leader.agentId不同须覆盖，不能直接选agentId或.first。五个新同层node回归加四个既有cleanup控制先取得6PASS/3FAIL，修正后9PASS；静态与独立检查后用同API/新suffix/新许可跑原四场景，旧r11场景及after纠正保留。

原gate false/true/false已恢复epoch2→4，task/queue/未释放预留/未settled运行0，staging及八slot0700为空；独立after允许保留同DB/API新轮，无需清历史或再次迁移。产品图3af与639回归保持，onlytest-source2路径形成新manifest。

## 当前修复边界与验证顺序（2026-11-16）

真实 r03 原 R4 响应证明当前阶段、候选、source、CI 均有效，缺 Move 控件由客户端未识别 wire 已规定的六个发布/回滚阻挡码引起。行为差距位于 Core R4 解码及拒绝回执状态表，Views 显式文案只负责可理解的拒绝原因。

- 最小产品修正为 `stage-schema.ts` 的 blocker/固定拒绝表、`stage-labels.ts` 的六条标签及英中 locale 六个拒绝键；同层两份 Core 回归和新的纯标签测试验证原 R4 组合、真正未知码、原键及双语显示。动作推导、权限、服务端 strict decoder、原 E2E 点击/断言/超时均保持。
- 先保存原代码的13条真实红回归，再作最小七路径绿修正；新完整相关范围636记录/37文件（最终标签语言拆分可能增加1条，按实际新分母记录），随后类型、lint与两端生产构建独立接受。保留新标签 test 的原类型/规则失败，机械修正不能放宽产品合同。
- 当前 ROOT 已有 c54 comment-list、三个 E2E fixture 和两个 locale 的 peer 变化。旧候选证据固定保留，新的最终前端副本须读当前 peer 字节并窄追加六个拒绝键，保护 `commentActions.more` 的新显示名与 focus/44px 变化；重建受影响两端，不覆盖整 locale 文件。
- 当前入口服务源独立e4/5038不变，四个新专库分别完整迁移853条且初始事实全0。独立 checker 使用许可 `4b7bc72a…` 完成只读准入后按 handler99 normal/race、router54 normal/race 串行实际测试，保留原默认期限和pool，不重复service116。
- 客户端与入口检查全部通过后，再重启匹配API/CLI/Host/productionWeb/renderer，以新来源绑定跑原四场景。原r01/r02失败及r03诊断独立保留，最后A01–A12和父矩阵按实际证据更新。

## 2026-11-16 r09 与首次实际运行

最新证据见 `research/progress-20261116.md`。以下旧时点记录保留，不能替代本轮来源绑定。

- P2 已独立接受为 `P2_CODE_ACCEPTED_R09_SCOPED`，阻挡项 0。三条已批准 ROOT 路径完成窄整合；入口可显式启用，项目政策仍默认关闭。原 HTTP 503、K06 两次错误测试假设与 READ ONLY recorder 失败均保留。
- 当前 5,038 文件服务层图已独立接受为 `JOINT_SERVICE_ACCEPTED_R09_SCOPED`；37+79 两段 normal/race 各 116 顶层、321 命名、263 叶 PASS，无失败或跳过。该结论限于绑定服务范围，完整 S2 仍须最终跨层证据复核。
- 生产前端 3,358 输入/1,961 链接已核对；5 项类型、2 项 lint、4 项测试命令、2 项构建均成功，619 个参数用例来自 36 个文件。匹配 server/CLI/router harness/Host/受控 provider 已构建，运行库实际 853 条迁移账本与源一致，865 索引有效；真实本地管理员初始化、改密、重新登录与 `/api/admin/me` 权限核验通过。
- 首次正式四场景 `four-scenarios-r01` 以 exit1 停止：E01 在参与者邀请准备收到 `400 invalid_request`，尚未打开 Web/Desktop 页或派发 Host；E02–E04 为 `NOT_RUN`。实际 0 PASS、1 FAIL、3 NOT_RUN，原 4 分母、请求、回执及清理记录保留，未自动重试。
- 原因是 E2E `workflowAuthority` 复用 Host 输入时带上工作流合同不接受的 `execution_view_version: 6`。仅修 `e2e/fixtures/collaboration-multi-account-closure.ts` 的字段选择，先新私有副本与独立检查，再绑定修正版重跑。服务端 strict decoder、原断言和超时保留。
- 知识任务已完成完整 ROOT 输入封存，允许本条 fixture 与 PAY 任务 MD/meta 更新；新的 fixture 属于封存后独立差分。五项 peer mechanical 改动仅登记为新副本输入，当前冻结运行源和旧报告不改写。

下一门槛：失败运行 after/资源核对 → 单点 fixture 修正及独立接受 → 修正版四场景 → A01–A12 与完整 writer/事务证据复核。父矩阵仍为 4 PARTIAL、20 NOT_RUN；S3 完整恢复、S4 真实接入、S5 最终候选验收仍开放。

## 2026-11-15 续接推进

本轮用户要求列出推进计划并继续现有 PAY-241；已重新绑定此 `in_progress` 子任务，不另建任务。完整依赖、验收门槛及分工见 `docs/plans/2026-11-15-pay-241-continuation.md`。本轮不执行 Git 提交或生产操作，保留既有授权与安全边界。

- 唯一服务端实施代理接续仓库外保留的 2d 草稿，按 checked 2c 到 2d 的最小差分整合；不重写客户端或 E2E 已通过的编写增量，不覆盖他任务共享文件。
- 发现保留的 2d DDL r02 将候选摘要约束修正为已有阶段候选使用的 bare hex64；旧测试库已经应用前一版同名 861/864，不能靠再次 `up` 证明新约束。旧库和失败证据保留，未改账本冒充匹配。
- 主会话从已独立检查的 2c 加两层明确 DDL 覆盖重建 4,949 文件冻结，所有哈希与文件清单匹配。重新构建 migrator，在专用空库完整迁移至 835 条账本、最后 864；结构与精确 ready 账本均成立，零新外键。
- 新建并核对 10 个隔离克隆，分离实施 service/handler/router normal/race、主会话 harness 与独立检查库；配置和 HOME/TMP 在仓库外，仅按明确包使用。当前 DDL 通过七项空库 down/up、半回退、不可变及保留历史拒绝控制；合成行不代表正式交付或部署。
- 已重新核对前端 3,127 输入文件、1,961 链接、1,586 构建产物及 23 外部追踪文件，零差异。该构建复用证明不等于产品联调；四个实际场景仍未执行，等待最终 2d 及匹配 CLI/Host 构建。
- S3 v2 恢复合同及 S4 外部输入作为当前 S2 后续依赖并行研究；不会提前激活或实现其他任务。总矩阵保持 4 PARTIAL、20 NOT_RUN。

私有证据根：`/Users/artisan/.multica/acceptance/pay241-20261115-continuation`。下一门槛：2d 全入口/事务测试与独立审查 → 匹配产品构建 → 四场景与 A01–A12。

状态：in_progress（2026-11-14激活）；产品规则已确认，最终技术规划复核通过（research/plan-review-final.md，阻挡项0）。用户已授权实现、本地提交与归档。

最新交接（2026-11-14 09:53Z）：用户转到新会话；完整状态见`research/new-session-handoff-20261114.md`。服务端与E2E检查代理因Agent Router资源引用错误停止，无本任务在途测试进程；六个S2包数据库连接审计为0。2b已有冻结实现与检查报告，但**S2B-C01/P1：T4/QA可选报告引用遗漏制品级读取ACL**尚未修复，是续接首要事项。2c合同已冻结，仅有DTO/序列化草稿，未进入执行账本深度实现。E2E第二批编写和独立局部修正已完成，实际场景执行0。不要把旧“实施中”摘要或代理报错本身当作最新产品完成/失败结论。

续接核对（2026-11-14 10:00Z）：新会话 `01a1d97d-7768-7f30-871f-87c43c99fb71` 已绑定现有 S2，无新建任务。HEAD 仍为 `8e824b303`，623 项未提交状态，index 为空；旧 PAY-241 主会话最后事件为交接完成，当前原生代理树无旧实施代理。六个包数据库连接均为 0，证据为仓库外 `new-session-db-audit-2.json`；初始工作区文件清单为 `resume2-workspace-manifest.json`（派发后、owner 加载上下文期间采样，不能替代各 owner 的编辑前字节）。唯一服务端 owner `/root/s2_server_resume` 先修 S2B-C01、冻结并独立复查后续接 2c→2d；客户端 `/root/s2_client_resume` 按已冻结 §8.10 并行完成 3c。实码另有旧会话未验证的制品 ACL 测试草稿与 runtime fixture setup 草稿，先保留原始字节再验证，不继承冻结 2b 的通过结论。联合 E2E 实际执行仍为 0。

最新增量（2026-11-14 10:34Z）：**S2B-C01/P1 已通过独立复查并关闭**，见 `research/server-increment-2b-acl-fix-results.{md,json}` 和 `research/server-increment-2b-acl-fix-check.{md,json}`。原始五项错误放行与夹具失败均保留；14 文件显式覆盖重建为 4,894 文件，独立 service23、ACL race13、handler44 及 vet/build/gofmt 通过。主会话核对实施/检查报告及原始日志哈希，并将历史制品权限、空页与 RR 权限锁合同写入 stage 规范。原服务端 owner 已接续 2c，先交 DDL/sqlc/可编译冻结再由主会话迁移。客户端 3c 的 16 文件、14 对日志/退出码已冻结并经主会话核对，Core400/Views54、类型/lint 通过；`/root/s2_client_check_resume` 正在独立检查。`/root/s2_e2e_resume` 接续缺失发布/回滚/完成门禁场景编写，实际运行仍等 2d 及客户端稳定。政策启用保持 false，父矩阵不变。

后续状态（2026-11-14，本次会话持续推进）：

- 客户端 3c 独立通过，C3C-C01（重复 intent/approval 的矛盾 fact）已修复；最终 Core403/Views54。3d 已完成 29 文件、2,532 文件隔离源码与 152 链接冻结，Core544/Views110、类型/lint 通过，独立检查正在修复已复现的 C3D-C01/C02（旧路由无 code 的 404/405 不应清来源权限；候选分页必须核请求 limit/cursor）。所有原始失败保留。
- 服务端 2c 已实现并冻结纯 S2 的 63 文件覆盖/4,938 文件完整源码，六包 normal/race 及静态检查通过；新增原 issuer 制品读取权限回归已红→绿。独立检查保留相同绿灯，并实证两项 P2：S2C-C01 兜底收尾重新起算 5 秒预算，S2C-C02 release 的传递批准读取状态未跟随原 reviewer 撤权。原执行消费已正确拒绝且零调用；唯一 server owner 正做两项窄修复，之后独立复查，尚未进入 2d 实施。见 `research/server-increment-2c-results.*`、`server-increment-2c-check.*`。
- 2c 迁移已由主会话应用到 8 个自有数据库，831 个账本版本至 843、13 新索引及实际 schema readiness 验证通过。专用迁移库空 down/up、843 及直接 829 的保留历史拒绝均通过，保留一条明确的合成迁移探针 intent。原八个 F2 清理注册失败不变。见 `research/server-increment-2c-migration-results.*`。
- 已冻结 F06 协作/R38候选与 completion-preview、安全 activity 合同：`research/server-increment-2d-client-contract.md`、wire13.18；接口实现仍随 2d，客户端可以据此并行。E2E3 主体独立通过，E3-C01 USTAR 数字字段夹具问题已修，11 纯测试/静态检查通过。原 E2E owner 在增量 4 改接真实共享 M1 表单、跨 Web/Desktop 的同原 grant/operation-ID 消费与恢复；实际四个场景仍均未运行。
- 私有 JSONL/socket 控制桥经独立 26 项纯测试检查通过，未运行实际 harness。root 后续 shutdown ACK 疑点被额外纯探针否证，桥代码未再改动。已在 `/Users/artisan/.multica/acceptance/pay241-s2-20261114-r01/allocation.json` 分配短私有 socket、固定 0500 staging 根及八个 0700 槽位、0700 output；未启动 API/浏览器。
- 新的并行任务 `11-14-project-knowledge-automation` 占用 844–859 及部分共享 server/Core/Views 文件。保留其代码与混合图失败；S2 基于已检查源加明确 owned overlay/hunk 隔离重建，不运行其迁移，不将其差异混入 S2 提交。2d 新迁移编号须再次查看最新占用。HEAD 仍以实际核对为准，主会话未提交。

最新运行级状态持续写在仓库外 `pay241-20261114-resume/resume2-coordinator-state.json`。这个状态记录不替代各增量冻结报告；2b 通过不关闭 S2，2c/2d、联合 A01–A12、S3/S4/S5仍按依赖继续。

续接更新（2026-11-14 14:28Z，以上各时点记录保留）：

- 服务端 2c 两项 P2 已由同一 owner 修复并独立关闭，见 `research/server-increment-2c-followup-check.{md,json}`。最终基线为 4,939 文件；独立 normal15/race15、vet/gofmt 通过。首次 followup race 的 `55P03 github_installation` 仍按未知原因保留；后续单例、完整49项及独立重跑通过不抹去原失败。
- 客户端 3d 已独立通过，C3D-C01/C02 修复保留；最终 Core550/Views112、类型/lint 通过。见 `research/client-increment-3d-check.{md,json}`。跨 source issuer/completion consumer 使用既有授权 API 与消费端引用表单；本增量没有新建独立 issuer 工作台。
- E2E4 最终编写独立通过，13项纯夹具、隔离类型检查、lint及4场景发现通过；2,552源码文件/638类型依赖核对无漂移。实际 API/Web/Desktop 场景仍为0。见 `research/joint-e2e-increment-4-check.{md,json}`。A09还须服务端 self/R38 成功、跨issuer/consumer、三种M1模式和最终时钟的真实writer证据。
- 服务端 owner `/root/s2_server_resume` 开始2d，先做提交候选持久绑定及同事务最终门禁，再项目逃逸/重开、F06/activity/readiness；政策启用仍false。并行任务迁移已占844–860，后续号码在创建前复核。
- root已核对HEAD未变、index为空，平台Web/Desktop及构建配置无本地差异；以HEAD平台文件加checked E2E4/Core/Views构建3,127文件的隔离前端副本，1,961依赖链接仅指向隔离workspace或现有pnpm包。生产Web构建进行中，尚无构建通过结论；Desktop CLI等2d冻结后构建。私有绑定在 `pay241-s2-20261114-r01/frontend-build-r01/input-binding.json`。
- `/root/s3_recovery_prepare` 只研究现有S3后续恢复与阻塞四字段，不修改共享writer；`/root/s2_final_matrix` 只核对最终A01–A12完整分母和真实入口证据，不提前运行或关闭验收。

## 1. 事实与规划

- [x] 加载 AGENTS/CLAUDE、Trellis 和父任务全部输入。
- [x] 记录初始 HEAD、149项 tracked/226项 untracked 差异，限定本阶段文件。
- [x] 原生研究代理核查服务端、客户端、测试环境和邻接任务责任。
- [x] 创建 S2 子任务，起草 PRD/design/implement，并集中询问关键产品规则。
- [x] 收到两次规则答复，收敛PRD与wire/data合同；保留人工手建，禁止自评且QA/发布分离。
- [x] 独立 `trellis-check` 做规划评审，处理阻挡项；配置真实 implement/check jsonl。
- [x] `task.py start` 激活实施；服务端首增量（wire-contract + 协议/策略/角色/建议家族/child准入）已派发原生trellis-implement。

## 2. 基线与环境

- [x] 新建 `/Users/artisan/.multica/acceptance/pay241-20261113-01`，与代码仓库分离。
- [x] 创建 `multica_pay241_20261113_01_s2_baseline_service` / `_handler`，实际数据库身份校验与迁移；配置仅留0600私有JSON。
- [x] 运行原协作交付、M1、当前授权、原键恢复基线；service 50/handler 31 named通过，无跳过，见research/baseline-results.json。
- [x] 为新用例及race分别分配service/handler四个独立数据库，记录允许的package/test；主会话已迁移至828，子代理不得切库、迁移或访问旧业务DB。
- [x] 本次续接另建 `s2_2c_router_r01`、`s2_2c_router_race_r01`、`s2_2c_harness_r01`、`s2_2c_migrate_r01` 四个独立配置/数据库，分别用于 router normal、router race、主会话控制的实际 router harness、迁移验证。使用已核对的 2b 冻结 DDL/二进制从空库迁移，816 个账本版本完整匹配至 828，17 个 S2 索引 valid/ready/live，未复用运行中 API 或包数据库。证据在仓库外 `s2-2c-verification-allocation.json`、`s2-inc2c-baseline-migrations.json`、`s2-inc2c-baseline-all-stage-index-audit.json`。当前仅完成基线准备，后续 2c DDL 仍须冻结后由主会话迁移；不代表已执行 router/harness 验收。

## 3. 实施文件归属与顺序

主会话只负责计划/规范、隔离环境、集成、证据、本地提交和归档。产品与测试由原生 `trellis-implement` 执行；其他会话的未提交改动不得回退。

| Lane | 独占文件/职责 | 依赖 |
| --- | --- | --- |
| 服务端 | 新 `server/internal/collaboration` 固定阶段类型/规则；新 `service/collaboration_stage*`；SQL/迁移/sqlc生成；`collaboration_workflow.go`、`collaboration_handoff.go`、`collaboration_lifecycle.go`、`delivery_completion*.go`、`issue.go` 必要事务接线；新 handler 与 router 接线；同层测试 | 最终阶段/角色与协议设计；共享事务文件由同一代理持有 |
| 客户端 | 新 `packages/core/collaboration/stage-*`；`api/client.ts`、`collaboration/index.ts`；新 shared Views 面板/表单与 detail 挂载；双语文案与同层测试 | 服务端 DTO 已冻结；不得改服务端或平台路由 |
| 联合验收 | stage canonical E2E、已有 closure 必要接线；fixture 环境与证据结构 | 后端/客户端集成可编译；不与前两lane共享产品文件 |

每一lane按行为红→绿并保存首个失败。报告CLI已按 `research/test-report-contract.md` 确定新增subject/submit/receipt，由服务端owner同步完成protocol、runtime/claim、HTTP/router、run-local guard/帮助及builtin说明。已有API/CLI完成路径必须被同一服务端门禁拒绝，不为证明拒绝复制一套完成工作流。

服务端首增量先交付wire-contract：逐动作列出design中的判别请求/最小确认、方法/路径、当前真人权限、版本/签字到期边界、成功与拒绝/进行中/404；项目政策单列原键修订恢复，issue动作复用原workflow操作。客户端以此冻结合同开始，不能提前按全部字段必填的统一回执编码。此接口交付由同一实现owner执行，不增加用户确认步骤；未完成所有真实writer门禁前不开放政策启用。

## 4. 检查顺序

1. 服务端定向单测、HTTP入口与事务回归；按design末尾实际入口表覆盖所有done与项目脱离/删除、人工child控制组、未批准机器拆解、角色撤权、同键/换键重放。包括child语义版本和正常状态变化分离、完成before→after、root重开/配置变化与enqueue/Start两个顺序。
2. 阶段服务/handler关键竞争用例 `-race`；迁移真实up/down/保留历史与独立并发索引检查。
3. Core协议/恢复、Views接线；`pnpm typecheck --concurrency=1`、`pnpm lint --concurrency=1`、相关 tests，`go vet` 和编译静态检查。
4. 冻结源码副本与依赖边；构建当前API/CLI、production Web、Desktop renderer/preload和受控Provider。浏览器库独立。
5. 三个人类测试会话验证可用/拒绝/版本变化/恢复；补管理员/独立code reviewer/release会话以覆盖角色分离。模拟人签字需注明。
6. 宿主fixture使用host gate；namespace fixture使用host=false；串行切换并按本次分配原值恢复，epochs保持单调。
7. `trellis-check` 独立完整差异/证据审阅。发现实质问题返回对应lane；只在新变化或未决问题要求时重跑扩测。

## 5. 汇总与提交

- [x] A01–A12逐条记录结果、命令、来源版本、环境、actor、原始证据及限制；R33独立接受。
- [x] 更新父矩阵/roles-and-states/缺口；保留原24例和旧历史列。S2局部断言不冒充整条PASS。
- [x] 更新确有变化的spec，避开其他任务规范；三份后端及一份E2E规范已定向提交。
- [x] 实际核验 API/Web/Desktop 与源码哈希，停止本阶段拥有的进程，保留DB/失败证据并恢复gate；沿用R33原始关闭证据。
- [x] 明确54路径分组Lore提交726c6fce2/2bd043c06/2e706092a；隔离候选检查通过并已推送，归档记录见report.md。
- [x] S3已完成并归档，后续转S4真实输入及S5；父任务维持开放，不能因S2关闭而结项。

回滚点：规划未完成不进入实施；实现红灯不关闭阶段；数据库保留审计，不用删除历史或重置他人文件回滚。真实资金、生产发布、真实模型账号均不在当前授权执行范围。

## 独立审阅的实施增量与必改接线

1. 协议/策略/角色/建议家族及全部child准入。未齐全部门禁的中间代码不可提供可启用政策入口。
2. typed证据/签字人代际、stage-only final guard、submit/done/project detach/delete接线；M1 off/observe/enforce矩阵与当前DB时间到期竞争。
3. Core/Views阶段操作、M1协作typed完成消费端（明确本包承接）、GET-only stage恢复与activity/通知投影；两端真实合成合同验证。

后两项已加入明确归属：服务端lane同时拥有completion preview/真实writer final guard与activity投影；客户端lane同时拥有普通/协作区分的M1 schema和stage_confirmation恢复、Query失效与双语通知。既有普通M1行为继续回归保护。

F01–F06的拟定技术收敛已写design，须独立复核；用户角色规则已确认；最后技术复核通过即激活S2实施。S3独立受控timeout测试子增量已经通过独立规划审查并实施，不代表S2通过。


## 2026-11-14 接管与推进决定

- 原协调会话 -25 的 S2 服务端/客户端 lane 于 00:48Z 因模型错误中止，未写 1b 报告。-45 于 01:53Z 接手，重派的两个 lane 在首个工具调用前遇到 429，未改动文件，随后 -45 退出。02:05Z 起由会话 -3a 负责 S2；起点 HEAD `fb9f4c070`，合同 sha256 `ced0cd66…`。
- 用户指令：服务端在合同第 13 节补上 A5 `code` 后放开 A5，然后跑 A01–A12。核对结果：第 13 节第 3–5 条已记录 `code`，但第 2 批（A6–A9、R4、T1–T4、10.2 完成/移出门禁、P2 启用）尚未提供服务，A03–A07、A09、A11 无法验收。用户选择**先实现第 2、3 批，再一次性跑完整 A01–A12**，不做中间状态盘点。
- 执行顺序：服务端 1b 收尾 ∥ 客户端放开 A5 → 服务端第 2 批（按 2a 证据/A6/R4/代码评审/A8、2b 测试报告与 QA、2c 发布/回滚与执行事实、2d 完成/移出门禁、P2 启用、activity 投影的顺序串行，由单一 owner 负责）∥ 客户端第 3 批（按冻结的第 8、9、10.2 节）→ 联合 E2E 与生产 Web/Desktop → A01–A12 → 独立 trellis-check。

本阶段实现owner划分：服务端一个trellis-implement持有全部server文件与迁移/sqlc/报告CLI/事务接线，先输出wire-contract供客户端并行；客户端一个trellis-implement持有core/views/locales共享接线与测试，不碰server。E2E在接口稳定后单独交接或由客户端owner增加；主会话负责DB分配、构建和实测。实际文件列表在agent实施结果中记录，任何超出已确认业务范围的必要变更先报告。

### 本次续接占用核对（2026-11-14 02:30Z）

- Codex会话 `01a1d7db-fa02-7b61-87a7-f9d4d4a22338` 绑定现有S2任务，未新建任务。起点HEAD `fb9f4c070`，同分支488项未提交改动；创建页面、文档等其他任务差异不纳入S2提交。
- 原协调 `dad148bc` 的1b/A5代理于02:21Z因额度错误停止，主会话随后被用户中断；两个新results文件均只是草稿。确认这一状态后才由原生 `trellis-implement` `/root/s2_server`、`/root/s2_client` 续接。服务端保持唯一owner，先完成1b报告与针对性验证，再顺序2a–2d；客户端先完成A5再第3批。
- S5原协调 `22c65976` 于02:23Z也因额度错误停止，未发现心跳补丁。r01恢复报告及原始样本保留，S5另行续接，不重复已有运行。
- 本次状态/源码采样记录位于仓库外 `/Users/artisan/.multica/acceptance/pay241-20261114-resume`。这次采样发生在派发之后，不替代此前 `research/initial-worktree.json` 的原始基线。
- 并行创建页会话随后独立提交 `482cd7f683739aca72050b6b09fdfe724742a898`，明确未含stage/Core/locales路径；本阶段源码仍是该HEAD加S2未提交覆盖，不能把它当S2已提交。
- 客户端A5收尾已完成并验证：`research/client-a5-release-results.{md,json}`，14个变更文件与9份命令日志哈希均由主会话复核；Core232/Views102通过、类型检查与stage lint通过。A5暂停解除，接续客户端3a。真实API/Web/Desktop及完整S2独立检查仍未进行。

### 2026-11-14 05:55Z 增量核对与接续

- HEAD为`8e824b303`，S5工具提交不包含S2产品代码；工作区仍有555项并行差异，当前index为空。当前任务指针仍为S2。
- 服务端1b、2a均有正式results。2a的45个文件、34份命令日志/退出码、4份主会话迁移记录已核对哈希，两项静态检查由主会话重跑通过。冻结副本与核对记录位于仓库外`pay241-20261114-resume/s2-2a-source`及`s2-2a-coordinator-hash-check.json`。独立`/root/s2_server_check`只读审阅此副本，不占用实施文件。
- 唯一服务端owner`/root/s2_server`接续2b；先冻结T4及create-only评论test_subject供客户端接入，再完成报告CLI、实际command/首次claim绑定及QA审批。所有迁移仍由主会话执行，政策启用保持false。
- 客户端3a独立复查关闭C01–C08；Core292、Views127及C08受影响30项通过，见`research/client-increment-3a-recheck.{md,json}`。`/root/s2_client`正在3b1，限于现有完成/管理/项目退出的阶段错误反馈；T4尚未冻结，不猜接口。
- 主会话另分配`multica_pay241_20261114_s2_ghsnapshot_r01`与`multica_pay241_20261114_s2_migrate_r01`，分别只供ghsnapshot、cmd/migrate验证；均已从空库迁移至825。新外部HOME/TMP通过Git-root拒绝探针。数据库测试与实际降级保护的结果另行记录，当前不宣称通过。
- A01–A12及production Web/匹配Desktop联合验收仍未执行。完整父矩阵状态保持4 PARTIAL、20 NOT_RUN；局部测试数不作为验收完成比例。
- 2a补充验证完成：ghsnapshot全包race为24个顶层/42个named通过；实际821–825迁移空库down/up通过，留存代码候选时在825拒绝且索引/账本不变。现有迁移注册检查失败，发现16个S2索引缺少重试cleanup注册，另8个F2缺项独立保留。见`research/server-increment-2a-supplemental-results.{md,json}`。
- 2a独立检查完成但尚未通过：S2A-C01(P1)迁移重试注册、S2A-C02(P2)私有child描述摘要字典推断均已交原server owner修复；V01为最终消费前审批到期的待复现竞争窗口，不能直接当第三个已证实缺陷。原2a报告保留，修复需新证据。见`research/server-increment-2a-check.{md,json}`。
- `/root/s2_e2e`仅新增阶段验收spec/fixture并做静态检查，复用现有Web/Electron测试设施；不启动API/浏览器，不提前运行A01–A12，不修改server/Core/Views。完整运行仍等待2d及客户端合同稳定。
- 服务端已冻结wire §§8.8/8.9及13.14–15（SHA256 `a71d71570cf1f337d0f8c299e77e307e6d37516dd516eaa6e0882548c5669520`）：T4分页/详情/用例/日志、create-only typed评论test_subject、source-only artifact_digest:null，以及source-denied child digest:null。客户端据此开始privacy+3b2。
- 客户端3b1报告完成：Core365/Views136通过、类型检查通过、lint零错误及项目页两条既有warning。主会话核对冻结副本482个文件、24个增量文件、19份日志/退出码及4个未改保护文件，均匹配；原生独立检查待空闲检查lane接续。见`research/client-increment-3b1-results.{md,json}`及仓库外`s2-client3b1-coordinator-hash-check.json`。
- V01后续已由实际service测试中的最终DB-time seam复现：原批准/原签字人到期两个分支仍推进，调用者到期控制组正确拒绝。首红日志`s2-server-inc2b-followups-red-1.log`保留；服务端owner继续修复及normal/race检查，不能据此修改旧2a检查时点的结论。

### 2026-11-14 迁移与E2E编写完成后的状态

- 2a三项修复已有独立的`research/server-increment-2a-followup-results.{md,json}`：阶段service90、关键race5、HTTP10、S2清理注册1项通过，无skip。主会话核对11个文件与6份日志/退出码，冻结到仓库外`s2-2a-followup-source`；原生独立复查仍待执行，不改旧2a报告。
- 2b的826–828迁移与sqlc已生成，全后端编译通过；主会话核对六份DDL并从各自825基线顺序应用到四个数据库，唯一command索引结构、valid/ready/live/unique及无FK均核实。证据：仓库外`s2-inc2b-ddl-before.json`、`s2-inc2b-migrations.json`。新的迁移binary为`s2-migrate-inc2b-828`；其执行只使用冻结的826–828文件目录。
- 客户端privacy+3b2进行中。测试评论使用既有受保护mention-targets的`can_submit_commands`和comment/v6/当前stage/review事实；不放开全局直接命令runtime门禁。`/root/s2_client_check`已对3b1冻结副本开始只读独立检查，缺失的纯依赖如需补充，必须单独记录来源/哈希，不能把当前3b2全包当历史3b1重跑。
- 首批E2E编写完成，两个新文件及11份检查日志由主会话核对；类型/lint/发现检查通过，测试体执行0。见`research/joint-e2e-increment-1-results.{md,json}`。3b2已改变其部分UI依赖，实跑必须重新绑定最终源码；不得沿用静态检查时的依赖哈希宣称联合验收。
- E2E仅为本次公开注册的精确新账号补合成`@example.invalid`邮箱（原邮箱必须NULL，匹配ID/用户名，事务内校验自有DB），其余邀请/移除/重入仍走真实API。主会话启动验收API时必须明确清空`RESEND_API_KEY`与`SMTP_HOST`，核对邮件服务选择本地日志分支；这不是公开邮箱设置流程的验收。

### 2a复查收尾、3b2检查与A05编写

- 2a后续复查补齐三份原先漏列的nullable-digest测试文件，保留首次独立重建失败并添加`server-increment-2a-followup-fixture-supplement.json`；重新构建/vet通过。复查随后发现A5只判断来源成员快照但未锁成员行，真实第二连接DELETE与确认可同时提交。服务端通过既有source-member动作allowlist修复，验证scope-first、revoke-first及empty external控制组，normal/race与独立复查均通过。最新结论在`research/server-increment-2a-source-fence-check.{md,json}`，旧报告不回写。
- 3b1独立检查通过（另跑173项冻结schema测试）。3b2独立检查修复B01：当前R1拒绝必须同步清除/隔离旧R4/T4，禁止旧私有证据维持C1/QA可用；保留草稿和原键。最终Core70/Views64及类型/lint通过；原477/63不与重跑相加。主会话核对496快照文件、34个最终范围文件、24份实现与10份检查日志，零差异。见`research/client-increment-3b2-check.{md,json}`。
- `/root/s2_e2e`编写A05第二批；授权最小扩展现有`collaboration-host-runtime.ts`和`collaboration-host-provider.go`及其测试，增加可选Git/test-report动作与默认成功不变的失败控制。同一服务端owner负责`collaboration_host_router_test.go`及相邻边界测试中的可选Git输入，使用既有HostConfig.Git，在capability注册前验证，ready仅输出安全摘要。所有原超时保持不变，普通fixture行为保持回归；不使用synthetic preparation rows证明正路径。
- 主会话已提供不带DB/账号环境的纯测试launcher：仓库外`run-pure-s2.mjs`用于protocol/CLI/command三包；另一个`run-pure-s2-host.mjs`仅允许`./internal/daemon -run ^TestCollaborationHostHarness`与`./internal/daemon/collaborationrun -run ^TestHostBrief`。各包独立外部HOME/TMP；HostBrief测试会启动显式测试Provider，不能称为只有静态检查。旧launcher字节保留，避免改变既有CLI证据。
- 2c技术准备已写`research/server-increment-2c-contract-preparation.{md,json}`，正在独立评审，尚未冻结或实施。主会话选择保留当前有效批准门禁：迟到事实可存历史，但不自动续期、重新调用或推进；不增加既有部署重新背书语义。实际router测试harness与生产默认无适配器的区别须保留。
- 已通过trellis-update-spec新增`.trellis/spec/server/collaboration-stage-approvals.md`并加server/Core索引链接，记录成员锁、最终DB时间、R1→私有缓存、迁移重试及证据清单要求。新规范结构/链接/空白检查通过；server索引含原有他任务差异，提交须只选本阶段新增hunk。
