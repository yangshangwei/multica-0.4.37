# F2 最小首包合同与证据清单

> 当前状态：F2最小首包已获后续明确授权并完成实现/确定性验证，见 [验收报告](f2-acceptance-20261113.md)；本文各阶段草稿保留历史，现行合同以code-spec和验收记录为准。

状态：设计收敛，非实现批准。继续现有任务，保持 `in_progress`。源码基线为 `c79dfe304`，工作副本含并行 F2 草稿；本文件不表示它们已通过验收。前轮技术依据与 A01–A16 在 [F2 planning research](f2-deterministic-report-snapshot-planning-20261113.md)，本文件收敛其中的产品选择和执行前证据门。

## 1. 已确认的三项产品选择

| 项目 | 本线程用户回答 | 首包合同 |
| --- | --- | --- |
| 可用性范围 | “接受最小首包” | 先提供可重放的正式任务存量；业务分类/窗口历史无权威证据时明确 unknown。完整业务分类和全历史体系后置，不再作为首包前置 |
| 来源变化后的保留 | “继续保留” | 已获准冻结的最小事实随原报告保留；源任务删除/移出不改旧正文。读取仍受当前报告/项目权限约束；owning report/工作空间删除时清理，明确撤回内容时停止读取 |
| 采集主体 | “人工显式采集” | 当前有报告编辑权限的人显式发起；Reporter只GET读已封存结果，GET不生成/刷新；原请求恢复与新correction区分。自动周期采集后置 |

这些回答批准的是设计取舍。本轮仍不实现代码、迁移、路由，不运行产品测试或真实 Provider，不操作 POLA-1、共享项目策略，不新建/归档任务。M1-S1不重做，M1-S2–S6/F3/F4/F5-A–E未完成项不变。

## 2. 可交付结果与范围

采集 cut C 中，范围固定为 exact workspace/project 内仍存在的普通正式任务（admission=`not_required/accepted`，排除独立 collaboration）；无项目时只保留当前报告身份，formal_stock、business stock、window和project health这四类项目统计均为not_applicable，不返回0、不转而统计整个workspace。当前正式存量描述C，不承诺窗口内曾属于项目的全部任务，也不复原C前已移出/删除的任务。

| 结果族 | 首包可承诺内容 | 正常值/缺证据时的验收 |
| --- | --- | --- |
| formal_stock | total/completed/cancelled/known-open/unknown_status、closure_ratio；blocked/overdue/unassigned/in_review/risk_union/environment_unavailable | 来源齐全时exact；包含报告和未分类任务，scope必须明示。每个指标仅按自身依赖传播unknown；证实空集合才允许0 |
| project health | 项目到期/lead有效性、latest update、progress age、health及reasons，按同一formal输入和冻结日历 | 复用旧规则；与旧project-health总体不同时先做ID差分。无update的null+exact与读取失败的null+unknown不同 |
| business stock | 有正面业务分类的存量，可靠报告provenance自排除 | 对未分类普通任务保留U，不能因不是autopilot就计入业务；正式总数exact并不使业务总数exact |
| window | created/completed/cancelled/closed/reopened/net及期末状态 | 所需出生、类别、归属、因果、保留证明缺失时unknown。观察到若干activity不升级为完整窗口历史；不从当前status或updated_at倒推 |
| 扩展来源 | PR、deployment、产品改善、模型叙述 | 首包没有权威采集适配器的来源标not_collected；不得从任务完成、PR合并、执行成功推断已部署或改善 |

`sealed_partial`是有效且可重读的结果，不是采集失败的别名：formal_stock可exact而business/window为unknown。整体complete仅指版本化必需来源/指标均满足合同；无需为取得`complete=true`扩大首包历史建设。根权限失败、事务/cut失效、不可解析内容或整体尺寸超限仍不能以partial成功掩盖。

日历、语义和摘要沿用前轮合同：保存scope/分类规则、timezone来源、[start,end)、as_of、当地reference/update/anchor日期或原offset，以及可定位的语义/coverage/canonical规则版本。as_of是C的时间标签，不是“该时刻之前全部提交”的证明。重读使用冻结事实和原规则，不重新读取当前tzdb、状态目录或来源行。

## 3. Reporter 公共投影与内部重放材料

推荐另行冻结一个公共投影，不能直接把内部`reportsnapshot.Snapshot`整体返回。当前并行`service/report_snapshot.go:52–55`将该内部对象作为响应，属于需要在未来实现审阅中核对的差异，不在本轮修改。

| 事实组 | Reporter公共投影建议 | 内部材料/排除原因 |
| --- | --- | --- |
| 身份 | snapshot/workspace/report/project ID、capture时report revision、同report/project的correction_of | created_by、request_key、内部sequence、request/cycle摘要仅供采集/幂等/审计，不参与Reporter统计 |
| 协议与时间 | public_projection_version、公开语义/coverage/时间规则版本、sealed状态、明确scope、window、as_of、解析日期/offset、无机器路径的时间来源说明 | 原MVCC read_cut、内部schema/部署/构建路径不是公开统计事实；不得把内部cut hash当公开水位 |
| 普通任务事实 | 已获准cohort的ID/revision/project/status/category/admission/created_at/due_date、report/business/unknown分类及固定reason code | origin_id、autopilot/run链、原始details和分类内部证据默认内部；保留可信判定及公开规则版本，不暴露目录或自动化资源 |
| validity事实 | 每issue的assignment_valid/environment_unavailable，每project的lead_valid，均为true/false/null及对应依赖quality | assignee/lead/reference的原用户、agent、squad、runtime、roster ID内部。当前算法用它们作join键；公开安全布尔足够重算结果，null不替换为false |
| 项目/类别事实 | 项目ID/revision/status/due/进入进行中anchor及时间语义；latest_update_at；cohort实际用到的status/category映射 | 不发布整个workspace状态目录；关联主体名称、内容、调用权限、机器路径不必要 |
| 事件 | 首包生产窗口unknown时不需要公开原活动清单。未来确有exact/partial结果消费事件时，仅给已获准cohort的snapshot-local fact引用、issue、业务时间、已证明因果/类别/归属 | 原activity ID、actor、原始JSON、无计算用途的recorded time、私有来源定位默认内部；局部iteration/triage正文不自动公开 |
| source/coverage/gap | 固定source-kind/adapter语义版本，read/producer/retention完整性状态、安全原因与依赖；可公开总体的E/I/X/U、known_count/extent_unknown、贡献ID仅引用公共cohort | 原filters/identities/cursor、内部完整性摘要、隐藏source ID/数量/SQL错误不公开。存在但不能披露的水位标proof_visibility=internal，不能谎称不存在；当前activity可靠水位确实不可证明 |
| 数值及摘要 | typed metric值、quality、独立lower_bound、rational；公共facts/result/projection摘要 | 内部整体digest不作为公共校验值。公共摘要只能证明公开内容一致，不能声称验证了内部未披露清单 |

公共输入仍是事实，不得把最终数字包装成输入后声称“可重放”。例如每个任务的状态类别、日期、assignment validity及来源缺口是输入，overdue/risk_union等结果由冻结规则重新计算。公共投影能复算公开指标；底层身份/来源真实性和无损历史由内部manifest负责。内部证据必须实际保留最小内容/身份/版本，不能只保存散列。

公共投影应随seal同时冻结。无论由哪个当前有权Reporter读，同一snapshot/projection版本的公共正文和摘要不变；权限检查放在外层。如果未来披露规则不再允许旧投影，整份拒读或经独立明确操作产生新版本，不按每次读者动态删几行后继续声称原快照。

## 4. 当前主要 writer 的来源能力矩阵

以下是源码静态核对，范围是列出的writer家族；不是部署数据库历史审计或无遗漏证明。最小首包无需补齐这些全历史，只须据实保存适用来源及缺口。

| Writer / 来源 | 现有证据与源码锚点 | 首包边界/精确历史仍缺什么 |
| --- | --- | --- |
| 普通/子任务创建 | `service/issue.go:311,334,520,542`同事务创建当前issue；HTTP `handler/issue.go:3199`，source context `handler/source_context.go:746` | 当前行+DB created_at不证明创建时属于当前project/formal范围；精确created需birth时project/admission/time/source/version |
| 自动化报告创建 | `service/autopilot.go:790,838–854`原子origin与linked run；提交后才发事件 | 可证明特定受信内置report来源；缺链为unknown，不等于业务。冻结判定和链版本，后删自动化不改旧分类 |
| 旁路创建 | `handler/onboarding_shim.go:275,446`；`lifecycle_handoff_transaction.go:398–405,476` | 只审普通CreateIssue不足以宣称覆盖所有创建；它们没有因此获得统一birth历史证明 |
| 普通/批量更新 | `handler/issue.go:4096–4140,4402,5599`更新与适用iteration/完成收据同事务；`:4465–4486,5634`写后发事件 | 当前revision可靠；局部收据不补齐所有中间状态/归属/准入。批量逐项提交，不是全批全局顺序 |
| move/项目变化 | `handler/issue_move.go:117–149`最终进入UpdateIssue | 位置键不是因果序；当前项目写及iteration子集不构成全项目membership历史；保留as-of范围，不扩读移出任务 |
| 失败恢复/GitHub状态 | `service/task.go:5957–5962`、`handler/github.go:1926`使用`service/issue_status_write.go:116–135`事务helper | 有局部事务证据，仍非完整ordinary账本；不能把运行失败本身计为业务重开或PR合并计为部署 |
| Triage准入/拒绝/duplicate/reopen | `handler/triage_actions.go:269,479,509,537,549`原子保存实际写与triage_action before/after | 有局部准入历史，不能笼统记“无admission历史”；round、时间、类别、原request与保留需独立适配，accepted不能当created |
| CSV导入 | `handler/triage_import.go:443,473–478`及`triage.go:609–642`关联pending intake与导入行结果，不派发执行 | 可证明本地batch/row身份；DB创建时间、外部原始时间、首次accepted不同。未证明外部完整历史、其他import/backfill |
| 普通/批量删除 | `handler/issue.go:4914,4970–4977`、`iteration/record_delete.go:14–48`保存适用iteration删除事实 | 未关联iteration时没有这条历史；当前formal存量不计已删行，完整历史吞吐需要其他tombstone/顺序/保留证明 |
| 项目删除detach | `handler/project.go:970–992`为iteration子集记录离开项目 | 不覆盖所有ordinary归属历史；旧report/project快照当前读权限需独立判断 |
| 迁移/修复/手工写 | 已发现`513_triage_tables.up.sql:1`补默认not_required；运行恢复如上 | 未证明实际部署升级、旧导入、修复、外部直接SQL的完整历史。历史起点/缺口区间必须保留unknown，建设后置 |

横切依据：

- `iteration/record.go:113–115`对未关联iteration返回nil；`:19–28,135–188`在caller tx记录before/after、类别、project、sequence；`iteration/transaction.go:97–115`区分采样时间与单调事件时间。这是局部较强来源，不是全局提交水位，不能直接以iteration event kind代替F2生命周期公式。
- `activity_listeners.go:23–45,68–85`独立写创建/状态记录，失败只日志。created listener要求特定payload类型，自动化`autopilot.go:869`发布map；所以“发过created事件”不证明一定留有activity。public Timeline仍有2,000上限。
- 当前正常状态API只允许创建时定key/category（`issue_status.sql:73–95`）；archive保留类别语义。不能泛称普通编辑能改category；历史修复/导入仍无全量证明。
- 旧`001_init.up.sql:159`曾定义activity随issue删除cascade；iteration/triage/workspace清理另有边界（`workspace_delete.sql:353,537–563`）。这是历史源码，不是本轮允许新增FK/cascade，也不证明当前部署实际schema已核验。

## 5. 人工采集、封存与原请求恢复

候选接口仍为`POST /api/issues/{canonical_report_uuid}/report-snapshots`，只用于明确人工动作；不是本轮新增路由。签名、request key、窗口、expected report revision、correction_of等候选wire可参考并行execution contract，但本文件的产品范围与最小公开投影优先，不能照搬内部完整响应。

采集准入必须证明当前human凭据与普通报告编辑权限、当前member/生命周期、报告ordinary/formal资格及exact project；不能把“用户能读这条报告”替代编辑准入。复用普通writer的实际权限/fence合同，在采集事务内复核，不增加“拥有某角色名字即可采集”的旁路。`handler/issue.go:4149`的loader和`:3935`附近write fences可作核对入口；并行human capture的成员/credential检查仍需逐项证明与此编辑合同一致。

全部采集页、状态/项目/validity/update依赖在一个C内取得；内部manifest、公共投影、结果和seal/current选择同次原子发布。事务失效/整体尺寸超限不发布，单源可恢复失败只在同cut有效时保留gap。采集不修改issue/project/comment/policy，不签发完成授权、不触发模型或dispatch；快照持久化本身是唯一新增领域写。

| 意图/故障 | 确定行为 |
| --- | --- |
| 新人工采集 | 明确原request key、不可变输入及当前revision，成功仅产生一个seal；不得后台隐式生成key后反复POST |
| 相同key、相同绑定，原结果存在 | 当前采集主体有权时返回原收据和原public seal；不按新数据重采。不要求当前report revision仍等于capture revision |
| 相同key、不同输入/actor/report/workspace | 冲突，不泄露他人原结果；不能用旧key换project/窗口 |
| 同一cycle、新key、无correction | 冲突而非第二份root；cycle明确绑定report/project/period/timezone/UTC窗口。不同period口径不能伪装同一修正 |
| 显式correction | 新key+同cycle当前seal为correction_of；新cut、新ID，旧seal和gap不变。两人并发修正只能有一个匹配当前前驱 |
| 取消/context到期/已确认中止的断线 | 停止采集并回滚，不自动重启、不换key；提交是否已发生仍不明时进入下一行的unknown恢复规则 |
| 请求仍有效且已确认可重试的序列化竞争 | 仅在原人工请求仍有效时，可携原key整体重启事务；不续拼旧页，不把序列值当source水位。已取消的请求不能进入此分支 |
| 已提交但响应丢失/取消竞争 | 保持未知；经原key和原输入恢复原结果，不自动correction，不宣称取消已生效。Reporter GET无恢复写权限 |
| current已变化，读取旧ID/恢复旧key | 旧ID只取原封存内容；current是选择器而非重放身份。不能跟随alias返回后继 |

原key恢复具体传输（仅显式重交原POST或独立human只读恢复入口）是待落实的技术接口；首包建议显式重交原POST、由服务端幂等命中且重新验权，CLI网络层不自动重试。不得为方便Reporter增加by-request-key或capture能力。

## 6. 保留、撤权和当前资格矩阵

读R是新RR首次取得数据库快照的数据查询（可能为token/task查询）。所有current authority与实际返回数据同tx。Reporter只允许当前报告的两个exact GET；无query/HEAD/目录兜底，root失败先拒绝；成功和所有错误no-store、无正文/隐藏ID/hash/计数/CDN cookie泄漏。

| 变化 | 首包读取合同/建议 |
| --- | --- |
| source状态/内容/归属改变或普通删除 | 按用户决定保留原facts/public seal；当前report/project和reader准入通过时200，不查询live source回填 |
| report普通revision/status编辑 | 不改变capture事实；reader task本身仍有效则可读。report revision不是历史GET的CAS |
| report移到Q或project被删除后detach | 旧P capability当前失配403；当前Q/无项目合法reader不能读旧P seal，指定旧ID最小404。不得用current回退另一scope |
| report移回原P | 建议按当前有效capability/claim和保留状态重新判断；可读原P内容。实际撤销的token不复活，不承诺一次移动永久吊销所有旧token（若需要须另设epoch） |
| report变pending/rejected/duplicate或独立collaboration | 建议首包同一R返回403；恢复普通正式资格后重新按当前scope/reader判断。这是读取准入，不能改旧分类/指标 |
| owning report或workspace删除 | 清理所属内部/公共内容；操作与根删除原子。无正文，不借同名新report恢复；不按source issue ID误删其他report的合法历史 |
| 明确撤回内容/历史保留权 | 按用户确认停止返回受影响旧正文；不局部删行后继续使用旧digest。物化撤回/清除机制仍需技术设计，见下段 |
| reader撤权 | R前已提交撤权必须deny；R后不承诺收回在途字节，下次GET必须deny；不能将当前reader失权降级成source gap |

撤回机制是仍需落实的门：需要用**内部**来源关系定位受影响seal，在独立当前保留/可读状态中阻止正文释放，并定义正文删除与最小内部tombstone。它不修改sealed数值/摘要，也不能公开被撤回源的身份。owning report删除可沿现有根清理路径，但只删除一个source不会自动代表明确撤回保留权。尚未证明并行草稿支持这条独立撤回合同；未来实施前必须给出具体验证方式，不能用“从未请求撤回”跳过需求。

## 7. 候选限额与完整性边界

保留工程候选值：内部canonical seal总字节上限16MiB、候选任务10,000、活动100,000；这不是实测容量或共享项目政策。首包不开放公共facts分页，公共投影也须在有界响应内。内部仍需全序分页及EOF/limit+1证明；数组乱序、重复页不改变规范结果。

已知安全上限截断源时记录scan_limit、unknown extent和受影响metric；不得谎称EOF、把尾部计0或公开不可读数量。整体对象超字节上限/无法生成完整公共重放输入时拒绝封存，原current不变。版本不支持/内容损坏不自动换latest或当前live重算。后续需纯序列化边界样本、峰值内存/响应缓冲及多页事务时间证据；不能只把三项常数写进代码当作验收。

## 8. 现有候选测试与仍缺的证据

下面只核对测试源码存在和断言形状，**均未在本次运行**；“Production”字样的测试名不代表生产服务、真实Provider或实际部署通过。并行旧日志不归入本次验收。

| A场景 | 当前可研究的测试入口 | 本首包独立证据门 |
| --- | --- | --- |
| A01/A02/A06/A09 | `reportsnapshot/snapshot_test.go`的`TestSnapshotLifecycleAndHalfOpenWindow`、`TestSnapshotUnknownIsNotZeroOrFalse`、`TestSnapshotDigestsAndFrozenReplay`、`TestSnapshotDSTWindowAndSavedOffsets` | 公共投影变更后重新证明只用公开安全facts可复算；23/25h、历史update/anchor日期、partial/空集/缺源相互区分 |
| A03/A04/A05 | handler的`TestF2ReportSnapshotActivityScanBeyondTimelineCap`已构造2,101条；当前断言read_complete=true、producer_complete=false、watermark=nil | 多种页大小、恰好整页、同revision异内容、较早tuple迟commit、cut失效不能拼页的屏障；读取完整不升级producer历史 |
| A07/A08/A10 | handler的`TestF2ReportSnapshotCaptureReplayCorrectionAndFrozenSources`；pure的`TestSnapshotProductionHistoryAndClassificationRemainUnknown` | 保留用户选定strict unknown；移出/移回、report准入/类型变化、首次识别历史报告不改旧seal；真实源删除与明确撤回分开 |
| A11/A15 | handler的`TestF2ReportSnapshotSourceGapAndFailedSeal`、`TestF2ReportSnapshotConcurrentOriginalRequest`；CLI的`TestIssueReportSnapshotCaptureResponseLossNeverRetries` | GET commit故障零正文、取消与commit先后、same-cycle竞争修正、current跨scope不得fallback、未知schema/损坏/超限拒绝；POST故障不能代替GET故障 |
| A12 | `health_test.go`的`TestFreezeHealthPrivacyAndMetricParity`；projecthealth的`TestFrozenCalendarPreservesHealthAndSnapshotVersion` | 固定HEAD旧规则+手算，双向ID差分；不能两侧都调用已改helper自证；public validity投影保持健康语义 |
| A13/A14/A16 | router的`TestF2ReportSnapshotProductionReporterExactGetMatrix`、`TestF2ReportSnapshotProductionRevocationAndFreshClaim`、`TestF2ReportSnapshotProductionLegacyReviewerAndNoProject` | 当前主要是顺序撤权；须增加首次R查询前后屏障、runtime owner/originator/claim独立变化、no-store早退、逐公共字段sentinel、report资格变化。只检查两个digest字符串不等于字段最小化通过 |
| 原领域零副作用 | handler初步检查report revision/comment和seal数量；router检查inert comment合同 | GET成功/拒绝/超限/未知结果逐项核对issue/project/comment/policy、凭据、完成授权、队列/dispatch/Provider零新增；不要把既有inert comment测试动作记成GET副作用 |

验收证据分层：文档/源码检查只证明设计可定位；pure固定输入证明计算；task-owned DB验证C、持久化、来源缺口和保留；真实Auth/router证明R和最小权限。每次未来运行固定源码/数据库身份、exact测试清单、expected/observed与失败/skip，零Provider。完整历史/模型行为/发布部署验证不纳入最小首包。

## 9. D1–D6 收敛状态与下一步

| 门 | 已收敛 | 仍需技术设计/证据 |
| --- | --- | --- |
| D1 | 最小首包为C时正式存量；缺窗口历史不阻塞首包 | calendar provenance、普通范围与旧health总体差分、显式窗口/时区准入验证 |
| D2 | 用户接受strict分类/历史unknown；完整体系明确后置 | 已知局部iteration/triage不能被误删为“无历史”；collector来源清单/生产unknown的具名验证 |
| D3 | 用户决定保留源删除/移出后的最小事实；根删除清理、明确撤回停止读 | 公共字段映射、内部反向来源关系与撤回执行机制、根删除原子性、限额证据；report资格/移回行为按本文件建议纳入最终设计审阅 |
| D4 | exact report/frozen project两个GET、同R、当前失权无正文 | 新公共投影契约、全错误/headers/no-store矩阵、first-read屏障；不扩大目录/生成权限 |
| D5 | 用户决定人工显式采集；自动周期后置 | 与实际报告编辑权限相等的准入证明、原key/cycle/correction、原子seal/current、取消/未知提交及明确撤回的写边界 |
| D6 | 沿用A01–A16及前轮细化，纯fixture与生产source能力分开 | 按第8节补足named运行与同cut只读parity证据，当前没有本次产品通过记录 |

本轮可完成设计文档收敛，不将技术证据门写成通过。若后续要实施，必须先审阅本合同相对并行execution draft的公共DTO、采集编辑权限、撤回与当前report资格差异，再取得当时明确的实施指令；不使用M1-S1批准或本次三项设计选择越过原边界。

## 10. 下一步改动边界（实施确认待返回）

用户后续要求“请推进F2”。由于先前明确限制仅设计，本次已询问是否转入实现；答复前仅作只读核对与规划，未开始代码、迁移或产品测试。以下给出可直接审阅的具体边界，避免获准后重新发明整套引擎。

### 10.1 公共计算与持久格式

保留内部`Build(Input)/Validate(Snapshot)/Decode`及原摘要语义，不给内部字段加`json:"-"`来兼作脱敏。拟由同一engine owner提供`PublicInput/PublicSnapshot`及`ProjectPublic/ReplayPublic/ValidatePublic/DecodePublic`：

- `ProjectPublic`仅在采集时运行，先验证内部内容，再生成白名单事实、独立计算公共指标并与内部对应值比对；两份材料同时seal。
- 公共replay输入不能嵌入当前含creator/key/read_cut/origin/目录身份的内部struct。内部reference join先归约为safe validity，再复用无身份计算规则；不得伪造UUID或复制最终metric冒称重放。
- GET只返回已保存公共投影。拟在现有`snapshot_json`中保存版本化`{internal, public}`封存包；具体SQL/type调整由backend owner独占，先确认原字段摘要校验仍成立。旧内部-only草稿行明确unsupported，不在GET中临时生成投影；没有生产迁移/兼容事实就不能宣称旧行已升级。
- human POST单独返回capture receipt，绑定原request key、规范capture intent commitment、report与snapshot；Reporter GET不返回该receipt。CLI目前依赖内部`Snapshot.RequestKey`确认原意图，必须将校验迁到这个human-only receipt，不能直接删除校验。公共投影本身不携带key。

### 10.2 明确撤回的事务方案候选

最小候选是**精确report/snapshot范围的人工撤回**，而非全workspace来源搜索或通用治理系统。它只能停止明确目标报告内容的读取；不能声称一次撤回已覆盖同来源在其他报告中的全部引用。跨报告来源撤回仍需独立定位权限和范围设计。此范围细化尚未作为用户已确认选择登记。

撤回记录与seal事实分开，GET在其R同时检查当前可读状态。仅“插入tombstone + 锁report”不足以串行化：capture可能先建立RR快照，再等待未被修改的report锁，仍看不到刚提交的撤回。候选是很小的report/cycle控制记录，由capture/correction/withdraw共同更新；旧RR冲突需整次原请求重试，取消的请求不得重试。GET只读该控制，不新增业务写。

明确withdraw请求需当前human编辑权限、exact report/snapshot、原幂等key和预期seal绑定；成功后旧ID/current/recovery均不得释放撤回正文，不回退到更旧seal。原withdraw结果恢复只给最小收据。后续采集/修正是否允许再次纳入相同内容，需要明确控制记录的generation/重新准入规则，不能靠新key绕过撤回。owning report/workspace删除继续原子清理其材料。

此候选仍需技术审阅与明确撤回范围决定，不能以已有草稿存在tombstone/锁的方式预设通过；本轮不创建控制表或撤回路由。

### 10.3 文件责任与依赖

| 责任 | 拟修改边界 | 顺序 |
| --- | --- | --- |
| Engine | `server/internal/reportsnapshot/`的公共类型/解码/重放及测试，必要`compute.go/health.go`；若需旧健康纯规则接点，同owner处理`projecthealth/snapshot.go` | 先冻结Public API与human receipt wire；不改service/router/SQL/CLI |
| Backend | report snapshot service/collector/handler、窄middleware/guard、SQL及必要迁移、handler/router测试 | 单owner负责持久格式、撤回/当前authority及共享router/SQLc；不与engine/CLI争写 |
| CLI | `cmd_issue_report_snapshot.go`及fake-server测试、内置Reporter指导 | 公共DTO和human receipt稳定后接入；保留原请求精确校验与GET-only |
| Root/check | 环境/源码清单、精确包测试准入、独立检查、任务与spec记录 | 实施获准且schema稳定后才分配/迁移task-owned DB；正常/竞争测试各自固定身份，不复用旧环境当新验收 |

当前优先差距为：公共投影与原意图receipt、全部拒绝分支no-store、report普通/formal准入、明确撤回与并发控制、首次R查询前后权限屏障。原M1及其他路线不在本切片。已有`f2-owned-files.json`缺少后来新增部分测试/公共文件，未来准入前须按实际diff更新；manifest列出一个文件不等于其全部并行改动已归本会话所有。
