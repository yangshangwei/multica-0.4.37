# P1 最终验收证据审计

最终核对日期：2026-10-06（Asia/Shanghai）。最终候选：`31534d8449caffc2cf4cda2a34c6a495ec58faf6`。首轮/补证基线为`d61757952`、`61ba556e4`；G1/G2/G3、RR-01—03、BR-01—03及workspace404合同已关闭。真实浏览器在61ba再次捕获RR02尾空格归一化边界，31534d844以忠实RED→GREEN补验修复，双端重建后最终8/8通过。**本轮P1功能、兼容、迁移与规定性能证据已闭合**；浏览器lane随后提交 [visual-verdict](browser-evidence/visual-verdict.json)，结论 pass、94/100；父代理核对了20份浏览器证据的manifest/hash与JSON完整性。本报告引用该视觉结论，不冒称审计者另做一次视觉审阅。

完整check session41190在61ba的前五阶段通过，但E2E 7/8令整条命令exit1；后续是31534d844的定向26测试/type/scoped lint、双端重建与完整8项E2E补验，**不是把那次完整check改写成exit0**。后端/mobile未在该修复中改变。最终E2E session40711的exit0由父执行者确认，本审计核对其完整JSON/日志及全部30条收敛原始样本。

方法：对照父 `prd.md`、`test-spec.md`、`api-contract.md`，读取真实测试 fixture/断言及产品读写路径，结合各 lane 的命令/结果与独立二审。测试名仅用于定位；例如取消-only 的纯 `Compute` 测试没有查询验收表，不能据此证明整个 API/UI 无验收。旧测试通过、当前源码和最终候选结果分别记录。

状态：**已证明**=该行本轮P1范围有实质断言及已执行记录，并已纳入最终补验。首轮G1/G2/G3与后续RR02边界均有关闭证据；AC-22/23只能写“P1现有部分已证明，I1真实分支待I1”，不得称包含未来迭代的整条验收通过。

## 证据索引

- F：`server/internal/handler/project_revision_test.go`、`project_timezone_test.go`、`project_delete_concurrency_test.go`；执行记录 `../10-05-projects-p1-foundation/verification.md`。最新删除故障测试已含 10 个阶段，旧报告“9”不是当前数量。
- A：`project_association_concurrency_test.go`、`server/internal/service/project_association_dispatch_test.go`；记录 foundation `association-verification.md`；原15个、BR-02补齐后17个真实handler入口双顺序、`pg_blocking_pids` 屏障、实际 runtime teardown。`backend-review-2.md` 的 BR-01/02 二审已关闭。
- H：`server/internal/projecthealth/health_test.go` 与 `handler/project_health_test.go`；health `verification.md`。读取了正式 6/2/2、unknown、RR、引用、验收排序与 C 游标具体断言。
- P：`project_update_test.go`、`project_update_concurrency_test.go`、`project_update_notifications_test.go`；progress `verification.md` 与 `evidence/review-fixes-race.jsonl`。包含实际进展/修订/请求/outbox/inbox 行与真实授权读取。
- C/V：`packages/core/api/project-p1-client.test.ts`、core `projects/*test*`、views `project-management.test.tsx`、`project-review-regressions.test.tsx`、`project-risk-pagination.test.tsx`；UI `review-fixes.md`、`pagination-c-verification.md`、独立 `frontend-review-3.md`（最新 APPROVE，RR全部关闭；第2轮报告保留历史失败）。
- MO/CLI：UI `mobile-verification.md`（最新显式 404 的 API/Query red→green，27 文件/185 测试与独立 mobile 三检查）；foundation `cli-verification.md`（真实 httptest 请求体，显式 revision、不自动 GET/重试）。
- E：`e2e/projects-p1.spec.ts`、desktop spec 与 `browser-verification.md`。最终`.omx/p1-browser-final-e2e.json`与日志证明31534d844的8/8、零skip/flaky；中间结果仅保留历史。最终视觉结论独立见 [visual-verdict](browser-evidence/visual-verdict.json)，不以E2E JSON代替视觉审阅。
- G/RR追加：`legacy-capability-verification.md`及对应真实响应JSON；`closure-acceptance-verification.md`及真实DB三种终态测试；UI `review-fixes-2.md`与`project-overview-lifecycle.test.tsx`；独立`frontend-review-3.md`；随后UI `preview-whitespace-verification.md`与31534d844差异补齐真实RR02尾空格边界。本轮读取实际断言、旧版响应、closure日志及最终E2E原始结果。
- M/O：`migration-verification.md` 及实际迁移测试；`performance-c-report.md`、`performance/results-c/{budget-assessment,completion-audit,current-full-scope-audit}.json` 与三档原始采样。迁移本人此前独立实际执行；性能本轮读取报告、预算结果及源指纹对照，未冒称重跑。

## 27 项 AC

| AC | 当前结论 | 已核对的实质证据／余项 |
|---|---|---|
| 01 原项目/五视图 | 已证明 | M 旧项目逐字段保存；既有 resources/execution-squad tests；E formal 场景逐个切 board/list/table/swimlane/gantt 并保存截图。最终E8/8已覆盖。 |
| 02 描述模板同源 | 已证明 | V `previews selected missing sections...` 明确只写 description+expected revision，预览不写；F CAS 实际存库；既有 `chat_project_context_test.go` claim 断言 ProjectDescription 与原项目资源。未新增 goal 镜像；最终E模板链通过。 |
| 03 选择/重复/取消 | 已证明 | core `goal-template.test.ts` 原有 Goal 时只追加 Scope，已有全部章节返回空；V 保留原文且预览无 update；最终E通过。RR-02/03已由忠实editor回归及独立三审关闭；31534d844最终真实E通过。 |
| 04 闭合100%不等于验收 | 已证明 | 新H `ClosedScopeDoesNotImplyAcceptanceOrChangeProjectState` 三种终态均closure1、两验收null、持久planned及零进展；新V overview lifecycle断言100%与两个“No acceptance recorded”、无Passed。G3关闭。 |
| 05 验收描述版本/摘要 | 已证明 | P `AcceptanceKeepsOriginalDescription` 真保存 v1、描述改 v2、更正仍原文/v1；H `AcceptanceSelectionUsesPublicationAndDescription` 较新旧版 partial 与当前版 failed 各选正确对象，更正不改排序/统计 version；最终E通过。 |
| 06 通过依据 | 已证明 | P `ValidationAndHumanGate` 对 passed/partial 空白依据422且项目无新进展；E 依据错误/补填/保存链已有中间通过。RR-02快速/延迟预览回归已独立通过；最终E通过。 |
| 07 N10/F6/C2/U2 | 已证明 | H `FormalGoldenScope` 实际 API 10/6/2/2 和旧 done_count=8；纯 Compute .8；V metrics actual/closure 分开；最终E6/2/2通过。 |
| 08 空正式集合 | 已证明（规则/API） | H golden 空集合 ratio=nil、项目延期单独 true；`BoundariesAndNoTaskSideEffects` 外空间不混入且 empty；V metrics 空集合无百分比。最终浏览器8项通过，详见补验记录。 |
| 09 全取消 | 已证明 | 新H all_cancelled真实DB断言N2/F0/C2/U0、closure1、两验收null、planned未变/零记录；新V全取消fixture仍显示尚无验收，不合成Passed。G3关闭。 |
| 10 非正式排除 | 已证明 | H 全 admission/actual 与 candidate 项目；T1 `BoundaryFormalCollectionsAndExplicitSearch` 保留授权显式搜索；最终E五视图候选排除通过。 |
| 11 接受不执行 | 已证明 | `TestTriageOrdinaryAcceptanceAdmitsProjectWithoutExecutionOrInheritance`、H 正式集合；E 普通接受 total+1、重放/执行计数链中间及最终31534d844均通过。 |
| 12 日期逾期 | 已证明 | H risk/date 实际 today 不逾期，纯 Compute due<D 且 terminal 排除；`TimezoneDayAndSemanticInputChanges`、DST 规则不依赖查看者时区。 |
| 13 风险交集去重 | 已证明 | H `RiskOverlapUnknownAndArchivedCategory`、真实 `RiskOverlapPaginationAndLiveContinuation` 逐项 counts/union 与风险ID分页；不是仅检查总数。 |
| 14 离线≠未分配 | 已证明 | `AssigneeLifecycleAndRuntimeAreSeparate` 与 `OfflineReferencesRemainAssigned`：引用有效与运行环境分开；private 可读/调用权限不混入 assignee 判定。 |
| 15 自定义/未知状态 | 已证明 | H archived category、unknown 保留 N/U、complete=false；原 custom terminal SQL 测试纳入独立执行。 |
| 16 精确下钻/个人过滤 | 已证明 | V exact risk 请求无个人过滤、parent/child 均呈现；H 当前RR suffix；C 分页5场景证明 sticky/失败保留/低ID从头重入；最终EC sticky/失败保留/从头刷新通过。 |
| 17 不完整不绿 | 已证明 | H 真缺 catalog 输入返回不完整而非0、精确风险503；V 先成功后失败移除 healthy 结论并禁用风险按钮；最终E503恢复/窄屏断言通过；视觉人工结论另档。 |
| 18 署名/发布/刷新 | 已证明 | P `CreateReplayAndImmutableCorrection`、`HistoryIdentityAndCursorScope` 实际作者/修订/离开或删除占位，不泄email；snapshot 检查非递归；最终E通过。 |
| 19 已提交丢响应重试 | 已证明 | P 同request/hash返回原 revision，即后来更正仍回原结果；并发同意图只一份；E `route.fetch` 真提交后abort，再相同payload重试是实质丢响应证明，中间及最终候选均通过。 |
| 20 更正历史 | 已证明 | P append-only、理由/expected_revision、原作者/首发不变、并发一胜一409；V与最终E历史展示通过。 |
| 21 完成警告非阻塞 | 已证明 | F 状态审计/no-op、原因 NULL 留痕；E U>0无验收仍允许完成，保存有理由并验证任务不变；最新UI属性恢复已随定向与最终E通过。 |
| 22 状态不改执行 | P1现有部分已证明；I1待I1 | F 实际多状态循环 issue 不变且无新queue；E synthetic running task 多状态后仍运行。真实当前迭代/结束快照在I1建fixture后验收。 |
| 23 删除保留任务/历史 | P1现有部分已证明；I1待I1 | F 实际 running task/issue保留、project_id清空、专属progress清理/automation暂停；A全writer双顺序；M全新表workspace清理和失败原子性；最终E删除链通过。I1历史不虚构。 |
| 24 删除权限/撤权 | 已证明（后端） | F owner/admin、member/task_token拒绝；A等待member fence重查；最新 production router/JWT 的操作403、真实leave后404证明前置middleware分类。 |
| 25 描述并发恢复 | 已证明 | F真实双连接一胜一409、缺token428、旧属性不覆描述；C顺序保存；V/E现有冲突恢复；RR-03当前props仍v1且GET失败时选择v2立即保持v2、后续写token8的回归已由独立三审通过；最终E冲突/server版本选择通过。 |
| 26 跨空间/撤权 | 已证明 | P跨项目/空间/失权/source脱敏；真实safe execution reader指定修订引用+当前private/chat授权；middleware404与core/mobile epoch测试。RR-01真实session teardown、旧请求/旧editor拒绝已独立通过；最终reader/browser通过。 |
| 27 mention与通知 | 已证明 | P真实member/agent mention、任务计数0、两worker/outbox稳定id、两锁序撤权；实跑mixed inbox string revision保留旧条目。最终E另一成员通知打开精确update及private reader403均通过。 |

## 14 项需求聚合

| 需求 | 结论与 AC/补充证据 |
|---|---|
| PRJ-001 | 已证明：AC01，身份/资源/小队旧行为有F/M及最终E证据。 |
| PRJ-002 | 已证明：AC02/03，原description同源与选择追加已证明，最新editor链最终E通过。 |
| PRJ-003 | 已证明：AC04—06/20；验收不可变/依据及闭合不自动验收组合均已取得P/H/V证明，G3关闭。 |
| PRJ-004 | 已证明统计/闭合验收独立规则；最终呈现已证明：AC07—09。`projects-page.tsx` progress排序使用未舍入 `progressOf`，渲染四舍五入没有用于排序。 |
| PRJ-005 | 已证明：AC10/11；T1 formal/实际关联和接受无执行有后端实证。 |
| PRJ-006 | 已证明：AC12—15，unknown/archived/日期/引用/父子并集均有canonical规则与DB测试。 |
| PRJ-007 | 已证明：AC16/17，ADR-05 C当前页与overview一致，跨页不宣称冻结快照。 |
| PRJ-008 | 已证明：AC18/27；人工真实身份/来源复核/零执行、RR02均已证明。 |
| PRJ-009 | 已证明：AC19/20；同意图、payload冲突、append-only/outbox已有真实并发证明。 |
| PRJ-010 | 已证明：AC21；旧无原因API仍可写且审计留NULL，UI恢复最终E通过。 |
| PRJ-011 | 后端与最终E已证明；I1保留：AC22/FR03。 |
| PRJ-012 | 后端与最终E已证明；I1保留：AC23/24、A/M/BR01—02实际修复回归。 |
| PRJ-013 | 已证明：AC17/25；lead失效H、资源/小队读取失败既有section回归、统计失败V；RR03已独立关闭，真实最终E通过。 |
| PRJ-014 | 已证明：MO独立185测试/CLI显式revision与旧router真实200/200/404/400已证明（G1关闭）；最新Web/Electron完整性8/8已证明。 |

## 18 项补充 FR

| FR | 结论 | 实质证据与剩余动作 |
|---|---|---|
| 01 字段边界 | 已证明 | P `ValidationAndHumanGate` 空白/10000与10001 emoji；`CharacterEvidenceAndMalformedInputBoundaries` 1000/1001理由、50/51证据、畸形JSON/伪造recipient/statistics；canonical中英与CRLF。不以JS UTF16长度替代codepoint。 |
| 02 日期 | 已证明 | F新编辑start>due422、历史坏日期仅改priority成功、null清空；既有 invalid400；V保留attempted date断言与相关最终E通过。 |
| 03 时区/七日 | 已证明 | F IANA/权限/settings列；H D-6/D-7、春秋DST/未发布migration；新V mounted timer跨UTC日发第2次Query并更新reference_date，focus恢复发第3次Query。G2关闭。 |
| 04 并发属性 | 已证明 | F/C有CAS、图标/priority不回写描述、no-op和旧无token属性兼容；CLI只发送明确给定版本。RR03最新忠实controlled-editor断言已独立通过；最终E通过。 |
| 05 RR快照 | 已证明 | H同caller事务二次Collect在另一连接更新后保持版本，下一事务才更新；P实际统计采集并落库、stale preview409、clock独变不冲突、历史快照不重写、unknown不能发布。 |
| 06 证据再授权 | 已证明 | P issue revision与execution digest、private/chat、allowlist/source/member fence后重试；safe reader真返回task/message并拒绝未引用/错父身份/失权。C严格身份parser；最终E真实读取对话框通过。 |
| 07 发布竞争 | 已证明 | P `ConcurrentRequestsAndCorrections`、`ConcurrentRequestIdentityAcrossProjects` 与真实leave后重放拒绝，不凭request_id绕过权限。 |
| 08 删除/撤权编辑 | 已证明 | RR01实际cleanup/new account网络/旧请求和旧editor隔离、RR02快慢preview、RR03 stale props/GET失败的v2选择均已独立三审通过；不是还未修的阻断项。 |
| 09 不完整/失效 | 已证明 | H真实输入失败/invalid lead与历史作者；V旧统计标stale/禁绿/可retry，资源小队query失败保持显式恢复，最终E故障恢复通过。 |
| 10 所有关联writer | 已证明（后端） | A真实handler锁探针排除FK假阳性，两个顺序；真实autopilot旁路/默认候选验证；BR02后scope-view迟到创建也拒绝。P删除/通知竞争不复活。 |
| 11 删除故障/历史 | 已证明（后端） | F当前10阶段故障事务全回滚；automation archived保持/其他paused、trigger禁用；A真实Runtime teardown与run_only死锁回归已关闭；I1历史另列。 |
| 12 通知恢复 | 已证明 | P两worker+existing inbox稳定id、rollback后无inbox、backoff cap/deadletter12、ctx取消、修订只补新recipient；删除/撤权实际屏障两锁序。 |
| 13 混合版本 | 已证明 | MO旧字段/能力/428/late response，CLI显式revision，当前JWT router操作403/scope404；旧binary有授权workspace/project两个200作为控制，再真实capability404/错误path400，G1关闭。 |
| 14 回退只读 | 已证明 | `FlagOffRetainsHistoryAndExactReplay` 保留原结果并拒绝新意图；`CapabilitiesReadOnlyRollbackKeepsCASAndTimezone`；M保留任何P1数据guard拒绝down。E能力限制是故障注入，不能替代真实旧router。 |
| 15 WS/日界/重连 | 已证明 | core事件分类/保护缓存、mobile真实flat keys与独立订阅/重连测试；最终E30次两Page due/assignee/admission实跑通过，总P95=161ms，各类P95≤281ms；这份DOM证明独立于C服务端负载。自然日界timer/focus→真实Query已补证G2；双Page原始30样本已逐条核对。 |
| 16 权限身份 | 已证明 | P真实owner/admin/member署名与task/cloud/legacy-agent拒绝；F人类admin时区；当前production route重新确认前置middleware。操作级403不撤整个空间，404指定code才撤权。 |
| 17 安全呈现 | 已证明 | 正文/快照为React文本节点；内部issue用workspace AppLink、execution用指定revision reader；外链HTTP(S)严格parser、noopener noreferrer；无外部fetch；P javascript拒绝/伪recipient拒绝。 |
| 18 零执行副作用 | 已证明 | F状态循环、P mention/preview/read计数、A删除后dispatch拒绝和普通pause行为保留；C性能completion-audit execution_tasks=0；E包括synthetic running与daemonStarts=0，最新整组8/8通过。 |

## 追加证据关闭表

| 项目 | 本轮核查证据与实际断言 | 结论 |
|---|---|---|
| G1真实旧router | `legacy-capability-result.json`包含binary SHA、health commit `0af59c5d5-performance-baseline`/PID60479/启动时间；workspace与project两个200且身份相符，再workspace capability404和错误path400；listener_stopped=true。初次401未被当unsupported。 | 关闭；无需再跑旧版或性能 |
| G2自然日界 | `project-overview-lifecycle.test.tsx`挂真实ProjectOverviewPanel+QueryClient，staleTime Infinity；23:59:30推进60000ms产生第二次API调用/显示D+1，随后仅改变Date+focus产生第三次调用/显示D+2。产品effect已有clearInterval/removeEventListener清理。执行记录在UI review-fixes-2的113条共享suite。 | 关闭；最终双Page30事件另有真实通过证据 |
| G3闭合与验收 | 读取真实Go三种终态fixture与断言及`.omx/p1-health-closure-acceptance.log`：14项health顶层race通过，新all_done/all_cancelled/mixed均通过，2.949s；V两fixture100%、两个尚无验收、无Passed。 | AC04/09关闭；不使用纯Compute冒充DB/UI证明 |
| RR01会话边界 | 实际clearClientSessionData调用resetProjectAccessSession；单调sessionGeneration不归零；新用户网络可访问，旧响应/乐观mutation/旧editor清理不能触碰新会话，copy-only文本清除。独立frontend-review-3复跑core22条。 | 关闭 |
| RR02预览flush（含尾空格追加） | 原快慢preview已三审通过，但真实61ba E2E仍发现getMarkdown raw与callback trim不一致。31534d844令预览/取消/删除捕获均匹配既有trim合同；新忠实mention尾空格RED→GREEN，真正内容变化仍使旧preview失效；26定向测试/type/lint及最终真实发布链通过。 | 最终关闭；保留61ba失败，不把早前三审扩大成已覆盖该边界 |
| RR03 server adoption | 当前props保持v1、GET失败，409带v2后选择立即呈现v2/删除拒绝稿，再编辑发送v2 revision；adoptedServer控制值留到canonical追上。独立三审确认实际editor同步行为。 | 关闭 |
| BR01—03 | 最新backend-review-2为APPROVE，真实Runtime teardown与17 writer/10清理阶段、实际router scope404及core消费均独立关闭；frontend-review-3确认core保留code并清内容。旧报告中“RR还待修”是时间早于第三轮的交接，不推翻新结论。 | 关闭 |
| 404错误合同文档 | 当前api-contract §1说明有workspace_access_denied的404直接失权、不能unsupported；§6分别列三种operation/source403、scope404、resource404/503；真实router测试要求改为非成员404及code。 | 首轮文档缺口关闭 |

## 最终阶段结果与来源

| 阶段 | 实际结果 | 本审计核对方式 |
|---|---|---|
| 61ba完整check前五阶段 | static 15/15；TS 10,052通过；69个Go包race通过、go vet、隔离API/production Web build通过 | 读取`.omx/p1-complete-check.log`；TS五包2635+62+6116+957+282=10052，逐行69个Go ok；脚本先race后vet，再启动API/Web |
| 同次check E2E | 7通过、1失败、0skip、0flaky；整命令**exit1** | `.omx/p1-complete-check-e2e.json`、完整日志末尾`Checks FAILED (exit 1)`；真实mention预览消失失败保留 |
| 31534d844修复补验 | 仅progress composer与忠实测试变化；26定向测试、views typecheck与scoped lint通过 | `preview-whitespace-verification.md`及commit实际diff；匹配raw imperative与normalized callback，不忽略真正编辑；后端/mobile未改变 |
| 最新API/Web | API PID91721、commit31534d844；Web PID92966、production、commit31534d844、build `7nHz90Ry9eikLfmkC-5ER` | 使用`.omx/p1-final-fixed-running.json`；名字近似的`p1-browser-final-running.json`仍是旧61ba，不能混用 |
| 最新Electron | build exit0，main/preload/renderer四产物SHA有记录，renderer `index-DNeAcGUl.js` | `.omx/p1-desktop-31534d-provenance.json`；真实Electron spec使用新产物通过 |
| 最终完整8项E2E | **8/8通过、0skip、0unexpected、0flaky，49.7s**；父执行session40711 exit0 | `.omx/p1-browser-final-e2e.json`全部8个result=passed/errors=[]，start2026-10-05T16:10:03.335Z；`.omx/p1-browser-final.log`一致 |
| 双Page30变更 | due/assignee/admission各10；总体P50 97ms、P95 161ms、max281ms；各类P95 281/139/135ms，全部≤5000ms | 读取final-results的`convergence-samples.json`，逐条两client值、slowest=max、before/after version不同；重新计算nearest-rank百分位与报告一致 |

最终8项逐条覆盖：真实Electron/中文历史/零daemon；目标模板与两人冲突/server版本采用；两Page30变更；6/2/2与T1普通接受/五视图/闭合无验收；C live cursor与sticky/从头刷新；真实丢响应发布重试/更正/验收/安全执行reader/真实inbox跳转；完成警告及管理员删除保留执行；capability/503恢复/中文窄屏键盘及hit-test。每项结果均来自完整最终spec运行，没有把单测数量合成为E2E通过数。

## 审计结论及父任务交接

本次27AC的**P1现有部分**、14需求及18补充FR有对应证据；此前明确缺口和真实浏览器追加发现均已关闭。AC22/23的未来I1分支仍未验收。可以结束本轮功能/兼容/性能审计；无需因最后仅UI归一化变更而重跑未改变的后端/mobile或已完成迁移/C负载。

父代理收尾补记：浏览器lane的最终 [visual-verdict](browser-evidence/visual-verdict.json) 为 pass、94/100，与最终E2E中的键盘/溢出/hit-test断言共同归档；父verification及六份task.json已同步。本轮P1没有剩余实施/验收阻断。历史完整check exit1、原A性能失败、RR/BR发现均保留。

## 性能、迁移与明确限制

C性能已经有实测证明：500项目/目标10000正式+300非正式，0/1/10变更每秒各≥600秒；非空后续页570/570、570/570、569/569严格前进，第二页30/30、31/31、31/31；预算判定PASS/failures=[]，源指纹与测量binary核对为真，execution_tasks=0。原A失败保留。另有最终双Page DOM30样本≤5秒通过，与服务端C负载分别证明，不能互相替代。

迁移已经有真实573历史up/P1-only down、旧行字节保留、252 guard/writer子场景、INVALID-index实际red→green、catalog与ledger恢复、工作空间清理原子性证据；不需为本审计重复破坏性演练。

交付限制：不声称包含未来I1分支的“27AC全部通过”、已生产发布或I1迭代历史通过。Redis未配置，依赖Redis的集成覆盖保持未测/跳过边界，69个Go包通过不表示每个可选外部服务场景均已执行。用户目标找寻90%/风险定位30秒没有实测用户基线；HTTP/双Page延迟不能代替找寻成功率。移动端范围为MO兼容，未做iOS真机/模拟器/IPA视觉验收；既有7项mobile lint警告与其它基线警告不虚报为零。
