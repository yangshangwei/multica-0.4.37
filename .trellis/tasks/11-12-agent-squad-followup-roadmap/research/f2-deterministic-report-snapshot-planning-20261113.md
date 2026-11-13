# F2 确定性报告快照：源码核对与验收提案

> 当前状态：F2最小首包已获后续明确授权并完成实现/确定性验证，见 [验收报告](f2-acceptance-20261113.md)；本文各阶段草稿保留历史，现行合同以code-spec和验收记录为准。

> 本次续接仅审阅设计与验收。§1–7 保留前轮记录；当前工作副本已有并行 F2 实现及授权记录，其状态不能套用本轮。以 §8 的本轮源码身份、补充合同和证据门为准；本轮不实施、不运行产品测试、不新建或归档任务。

> 后续设计选择已收敛：用户确认最小首包、源变化后继续保留、人工显式采集。当前可审阅合同见 [f2-minimum-contract-20261113.md](f2-minimum-contract-20261113.md)；§8.7相应产品问题已得到回答，来源/权限/撤回/运行证据仍待验证，不推导本轮实施许可。

状态：design / acceptance review only，仍有待决项；不是 F2 实现批准，也不是产品测试通过记录。2026-11-13 接续同一任务，HEAD `c79dfe304`，分支 `codex/cross-workspace-collaboration`。开始时 task.json 为 `in_progress`、session task 指针为空；已恢复同一任务指针。开始时 374 项并行未提交改动，本任务目录干净。

本轮只更新任务规划文档，不修改源码、迁移、路由、共享策略或旧实例，不调用 Provider，不操作 POLA-1，不新建或归档任务。M1-S1 实现 `463b201fa` 与验收记录 `c79dfe304` 保持历史身份；Views 602 files / 7,332 tests、renderer 2/2 来自已保存证据，本轮未重跑。

## 1. 当前来源及局限

主技术合同见 [design.md §8](../design.md)，需求映射见 [requirements.md](../requirements.md)。下列源码锚点是本轮只读检查结果；归档设计与已有测试均不作为 F2 已交付证明。

| 来源 | 已确认事实 | F2 限制 |
| --- | --- | --- |
| archived `11-05-agent-squad-rnd-improvements/design.md`，后续目标/API/工作包表 | 冻结 scope/timezone/window/as_of、task/activity/PR/deployment 来源、coverage；先只读双向对照；模型只解释 | 设计方向，不是报告账本或权限批准 |
| archived `10-05-projects-p1-health/design.md`、`10-05-projects-p1-progress/design.md` | 实际项目关联 + `not_required/accepted` 正式任务；旧 done_count 为范围闭合；人工进展与系统统计分开 | F2 不改旧统计、人工验收合同 |
| `server/pkg/db/queries/project_health.sql:1`、`:18`；`server/internal/projecthealth/counts.go:8`、`:23` | 当前项目正式任务全量、当前状态类别（含归档类别）、最新 published_at；未知类别计入旧 open 并另列 unknown_status | 当前存量，不是窗口事件；没有报告自排除或历史项目归属证据 |
| `server/internal/projecthealth/snapshot.go:32`、`:123`、`:189`、`:222` | StatisticsSnapshot 为计算结果；正常 Compute 的 complete 只看 unknown_status；hash 覆盖排序后的输入、日期等 | hash 不等于来源清单、提交水位或持久 snapshot ID；不保存输入便不能独立重算 |
| `server/internal/projecthealth/collect.go:12`、`:20`、`:28`、`:81` | Collect 只用调用者 q；读取状态、任务、成员、agents、squads、runtime 和更新 | 内部目录读取不授予 Reporter 目录 API；只冻结关联主体的最小 validity/readiness 投影 |
| `server/internal/projecthealth/unavailable.go:12`、`:22` | 部分 SQL 失败生成 null counts / unavailable；断线、取消等不能形成可用 partial，事务须回滚 | 不能概括成现有代码忽略所有错误；F2 也不能在 aborted transaction 里继续封存 |
| `server/internal/handler/project_update.go:525`、`:669`、`:737` | 人工 update revision 保存系统 statistics_snapshot；读原统计，另检 live evidence availability | 不是独立来源 manifest；不为 parity 发布、更正 update 或双写状态 |
| `server/internal/handler/activity.go:47`、`:75`；`server/pkg/db/queries/activity.sql:1` | public Timeline 最多保留最新 2,000 activity，发 X-Timeline-Truncated；query 按 `(created_at,id)` 排序 | 翻页不能救回 cap 丢掉的旧行；tuple 是排序 cursor，不是全局提交序列 |
| `server/cmd/server/activity_listeners.go:68`；`server/internal/events/bus.go:57`；`server/internal/handler/issue.go:4465` | 普通 status activity 在同步 event listener 内另用 queries 写入，details 为 from/to，失败仅记日志；不是原 issue writer 的同一事务。issue publish 有 project_changed，该 listener 无对应项目历史记录分支 | 无全 writer 无损历史证明，无事件时刻类别/项目版本；同步 listener 仍有原写已提交而 activity 尚未记录/写失败的缺口 |
| `server/migrations/001_init.up.sql:156`；`server/pkg/db/queries/activity.sql:31` | activity.created_at 默认 now()，ID 可随机或由调用者提供 | 不是业务 effective_at / commit time；UUID、MAX(id/time)、xmin 不可充当持久全局水位 |
| `server/internal/iteration/record.go:113`、`:138`；`history_snapshot.go:24` | iteration 有同事务 facts/event/scope revision；未关联 iteration 不记录 | 可研究复用模式，不能宣称覆盖所有项目任务或把 iteration sequence 当全局水位 |
| `server/internal/service/builtin_role_skills/multica-progress-report/SKILL.md`、`builtin_agent_templates/progress-reporter/INSTRUCTIONS.md` | 当前 Reporter 项目内取数，无项目仅当前报告；窗口固定、报告自排除、去重、unknown、完成/取消/重开分列；当前报告 inert comment，不改状态 | 是语义 baseline，不是确定性服务端计算器；只读规则对照无需调用模型 |
| `server/internal/service/builtin_autopilot_templates/{daily,weekly}-progress-report/PROMPT.md`；archived `09-14-progress-reporter-daily-release/design.md` | 旧模板仍写工作空间兜底、触发时间窗口、允许 in_review；当前 role 为项目/执行开始时刻/不改状态 | 已知文案差异，以当前 capability 为权限权威。本轮不修模板、不升级已有实例 |
| `server/internal/auth/task_capability.go:21`、`:41`、`:76`、`:125` | server provenance progress-reporter v2+；frozen issue/project/runtime/claim；当前 principals/member/invoke 校验 | 名称、提示词、旧 producer 或 source link 不是授权 |
| `server/internal/handler/task_capability.go:34`、`:55`、`:96`、`:144`、`:225` | closed allowlist；RR READ ONLY 绑定 Queries/DB，缓冲响应至 commit；既有当前报告 comment POST 独立事务、inert；legacy NULL capability 对旧路由透传 | 新 F2 route 当前被拒；handler 必须显式 verified Reporter；新增 GET-only 不等于移除既有 comment POST |
| `server/internal/middleware/auth.go:134`；`server/pkg/db/queries/task_token.sql:6` | Auth 覆写 actor/workspace header；token 到期/删除失效 | 当前不是原始冲突 header 一律拒绝；若要求拒绝，须设计覆写前的 ingress 校验 |

## 2. 每项指标的权威与只读对照

以下均为待审阅 `cohort-as-of-v1` 提案。cohort 是采集 cut 的 exact project 正式任务集合，排除已证实报告；描述“采集时项目成员的本期记录”，不冒称“本期曾属于该项目的一切事件”。事件只消费此 cohort 内、证据证明属于 frozen project 的记录。事件发生时项目归属、状态类别或时间无法证明则相关指标 unknown，不能跨项目补抓正文。D1 保留这个范围选择的审阅。

| 指标 | 权威输入及提案公式 | 对照/边界 |
| --- | --- | --- |
| stock.total | cut 时 issue.project_id、正式 admission_status、去重 ID，扣除证实报告 | project-health formal scope；报告排除单列差异，不用默认分页数 |
| stock.completed/cancelled/open/unknown_status | 冻结 status + cut 类别；done、cancelled、已知非 terminal、未知四类互斥，完整集合上和为 total | 旧 open 含 unknown；F2 的已知 open 不混入未知。旧 done_count=completed+cancelled |
| stock.closure_ratio | (completed+cancelled)/total，保存整数分子/分母；total=0 为 null；类别缺失则 unknown | 同输入且完整时与 project-health 对照；不是完成率/产品成效 |
| blocked/overdue/unassigned/in_review/risk_union | 非 terminal 的 status、due_date、当地 reference date、关联主体 validity；risk_union 为 issue ID 并集 | 复用 Compute（due_date 严格早于 reference date）；未知类别/引用使相应指标 unknown |
| execution_environment_unavailable / lead_valid | 关联 member/agent/squad validity、runtime status 的 cut 投影 | 复用 Collect/Compute；不是调度容量或真实 Provider 可用性 |
| project_overdue/latest_update_at/progress_age_days/health | 冻结项目状态/到期日/lead、latest project_update.published_at、in_progress_since、timezone/reference date；沿用现阈值版本 | 不可只冻结 issues 而实时查更新；人工 health_judgment 不覆盖计算值 |
| window.created | cohort issue.created_at 在 [start,end)，有当时在项目的范围依据；每 issue 一次 | created_at 只证明创建时刻，单独不能证明当时项目 |
| window.completed/cancelled | 证明的非 done→done、非 cancelled→cancelled，各保存不同 issue IDs 和事件 refs | 两集合可重叠，二者和不必等于 closed；取消不是完成 |
| window.closed | 非 terminal→terminal（done/cancelled）的不同 issue IDs；反复关闭只计一次 | done↔cancelled 不新增 closed；不是当前 closed stock |
| window.reopened | terminal→已知非 terminal 的不同 issue IDs，保留全部转换事实 | 不撤销历史 closed/completed；end 状态另列 |
| window.net_created_minus_closed | created−closed；依赖均 exact 才输出 exact，否则 null/unknown | 不是 open 存量增量，移入/出、重开、删除、准入改变另列 |
| end 状态/进行中/阻塞原因/负责人 | end 前状态需 baseline+完整有序事件链；叙述引用固定 revision 的评论 excerpt | 只有 cut 当前值时写 state_as_of，不能冒充 end；updated_at 不倒推历史 |
| PR/deployment/产品结果 | 已有有权且版本固定的 source refs；产品结果还需 baseline、指标、观察窗和实测 | 无来源则 unknown/not_collected；不从 done/评论/PR merged 推断部署或改善，不访问远端补齐 |

未来对照证据门（本轮未执行）：

1. 在任务自有 fixture 的同一授权 RR cut，给旧 Compute 与候选 reducer 同一 project/timezone/reference date/input；两个实时 HTTP 请求不是同一 cut。
2. 第一组同 formal scope 比可比 stock；第二组加 F2 排除，双向 ID 差分 old_only/new_only/common，解释报告排除、unknown/open 和时间含义差异，不能仅对总数。
3. 对旧 update revision 只比当时 fixture 的可比字段；缺当时 manifest 的历史结果标 not_replayable，不拿现在数据冒称重放。不得创建/更正 update 来取统计。
4. role skill 用手算 fixture 做规则对照，旧模板差异列明；没有模型调用不记 Reporter 行为通过。
5. 保存每 metric 的 input IDs/hash、oracle/candidate、差异原因、coverage、源码/语义版本和零业务写证明；未解释的差异阻止验收。

## 3. 来源、分页与完整性审阅门

- 每源保存 kind/authority、schema/adapter version、实体 ID/native revision（若有）、最小事实 payload、digest、时间字段含义、cut、ordering、读取边界、coverage/gap。只有 ID/hash 而无保留的不可变内容，不是可重放来源。
- 必需物化 issue/status/project/admission/report-classification、关联 validity、更新时刻、事件 before/after/顺序依据。无 native revision 时用实际投影 content digest，并声明无法证明中间版本；不以 updated_at/xmin 冒充历史。评论只保留获准引用的最小 excerpt/version。
- read_cut、ordering_cursor、producer_watermark 分开。只有来源支持可靠单调提交/交付协议才设置 watermark，否则 null+watermark_unavailable；读完 DB 可见行也不证明 producer 无遗漏。
- 采集全部页使用同一 RR/q 和唯一全序 keyset；保存页边界/数量/hash，最终 manifest 按稳定身份排序且不依赖 page size。重复 identity/revision 去重，同 identity/revision 异内容是 conflict，不任取其一。
- 同时间戳+随机 ID 只能稳定展示，不能证明因果顺序；reopen/close 的 end 状态须有 revision/sequence 或可验证链，否则 unknown。
- 内部采集页与 sealed 快照对外分页分开。断线/换连接/新事务不能续拼旧 cut；重启整次采集形成新 cut，未 seal 的结果不出现在 current。
- 可恢复单源缺失且 cut/authority 有效可保留 partial+gap；SQL error 若已中止事务，须采用另行审阅的 savepoint 方案或终止采集，不能在 aborted tx 中封存。全 DB cut 丢失是 unavailable，不是成功 partial。
- 迟到是业务时间在窗口、但交付/提交在 cut 后。旧快照只含采集当时可知 gap；不能事后补写迟到 ID/unknown/digest。显式 correction 用新 ID/cut；原周期重试只恢复原结果。
- 缺历史、未审计 writer、截断、缺页、缺权、类别历史等各有 reason/affected metric/boundary；source 不可读只披露安全缺口，不能显示隐藏 ID/数量。replay 请求者失权则 deny，无正文，旧 gap 不改变。

## 4. Coverage 定义与可手算示例

按 metric 声明 value_kind、coverage_unit 与依赖。issue 计数指标的 unit 为去重 issue；project bool/enum/timestamp/scalar 指标的 unit 为版本化 required_dependency（命名的必要事实组），例如 project_overdue 需要项目状态/到期日、时区和参考日期，latest_update_at 需要完整 update 来源。source coverage 另以 row/event 为 unit。三类不混加，不用平均总分掩盖未知；project 指标不能套 issue 分母。

以下先定义 issue 计数指标。可完整枚举候选总体时：E=eligible（排除前候选）、I=included（该指标事实齐全，可能贡献 0）、X=excluded（有正面依据的报告/范围/准入排除）、U=unknown（候选存在但所需证据/分类未决）。E=I+X+U，后三者互斥；source 缺失不得进 X。

coverage-v1=I/(E−X)，保存整数 numerator/denominator；分母=0 为 null+not_applicable。E 未知则 ratio=null，另保存 observed_candidate_count。U 使用 {known_count, count: integer|null, extent_unknown, reasons}：确知一个缺失则 count=1；不知缺多少则 count=null、extent_unknown=true、gap 非空，不为强求 nonzero 而编造一个任务。unknown 不折算为 0。

每 metric 输出 value_kind 及类型匹配的 value（integer/rational/boolean/enum/timestamp/null）、quality（exact/partial/unknown/not_applicable）、verified refs、coverage refs。计数 partial 可单列“已核实至少 N 个”，未知总数仍为 null；净变化/比例不能从局部样本算 exact。stock 的已知 completed/cancelled/open 与 unknown_status 是证据分区，其和可等于已枚举 total；分类未决时不能把已核实 completed 数当成真正完成总数。

project-level coverage 的 E 是该 metric_schema 声明的必要依赖组数，I 为已验证组、X 仅为按合同证实不适用组、U 为未验证组，也保存命名清单与 I/(E−X)。查询失败不能进 X；任一必要组未知则 bool/health/timestamp 等 value=null+unknown，不给 false/clear。无更新行且已证明来源完整时 latest_update_at=null+exact，与读取失败的 null+unknown 区分。派生 metric 的覆盖列出所用 metric/source 依赖，不从其他指标的 issue 分母借比例。

complete 仅当所有声明的必需 source/metric 可证明、无影响它们的 gap；未纳入首包的扩展指标明确 not_collected。

| 示例 | 必需结果 |
| --- | --- |
| 5 候选，3 事实完整、1 报告、1 历史缺失 | E=5,I=3,X=1,U=1；coverage=3/4；窗口总数 unknown，保留已核实清单 |
| 读到 3 候选后尾页失败，无可靠 total | E=null,I=3,X=0,U.count=null,U.extent_unknown=true；ratio=null，gap 非空 |
| 当前报告+标题叫“日报”的普通业务任务 | current report ID 证明 X=1；同名业务正常参与，不能按标题排除 |
| 历史报告分类未决 | 该候选 U，不默认为业务或偷偷 X；分类来源待 D2 |
| 有项目且完整证明为空 | E=I=X=U=0；counts=0，coverage/closure_ratio=null；与读失败不同 |
| 无 frozen project | 只读当前报告；project metrics=not_applicable，不输出整个 workspace 为 0 |
| 项目字段/时区齐全，但 update 来源不可读 | project_overdue 可按自身完整依赖求值；latest_update_at/依赖它的 health 为 null+unknown，不能借 issue coverage=100% 宣称完整 |

## 5. 确定性验收矩阵（未来测试，未执行）

固定 W/P/R、窗口与 cut；A/B/D/R 均在 start 前创建且全程属于 P，状态类别证据完整，只有 R 是报告。手算 oracle：A 在窗口 todo→done→todo→done；B todo→cancelled；C 在 start 新建保持 todo；D 只在 end 进入 done；R 为报告。无其他缺口且均属 P 时，created={C}、completed={A}、cancelled={B}、closed={A,B}、reopened={A}、net=−1；D 的 end 事件排除。stock 另按 cut，可包含 D 已 done，不能改写窗口结果。

| ID / 需求 | 场景 | 可观察通过条件与主验证层 |
| --- | --- | --- |
| A01 / F2-01 | 输入乱序、不同 page size、重启重读 | 同规范输入 facts/result digest 一致；同 snapshot ID 保留 envelope/facts/gaps/digest。reducer+持久化 |
| A02 / F2-02 | start/end、UTC/Shanghai、DST 23/25h 自然日与滚动24h、时区后改 | start 包含/end 排除；UTC 边界不变；timezone/provenance/tzdb 或解析版本可复核。时间 fixture |
| A03 / F2-03 | 2,001+ activity、同 timestamp、多页重试、重复/冲突 | 无漏/重计，cap/冲突为 gap；page size 不改规范结果。source adapter/DB |
| A04 / F2-03 | 并发提交，旧事务 timestamp 很早但后 commit，采集中断线 | 单 cut 可见集合，max time/ID 不是完整证明；不拼旧页、不发布半成品。DB 屏障 |
| A05 / F2-04 | 迟到、producer 写失败/遗漏、retention 起点晚于窗口 | 旧快照不改；correction 新 ID/cut；缺历史 unknown；读完 DB 不冒充完整 producer |
| A06 / F2-05 | A 重复完成/重开、B 取消、done↔cancelled、同时间无因果序 | 按 §2 分列去重；terminal 互切不增加 closed；因果不明的 end 状态 unknown。手算/reducer |
| A07 / F2-05 | cut 前后移入/移出、项目重分配 | as-of cohort 差分明确；事件时归属不明为 gap；旧快照不随当前集合变、不跨项目抓历史 |
| A08 / F2-06 | R/历史报告/同名业务/分类缺失 | ID/provenance 决定 X，同名不误排；未知为 U，保留理由，不复用旧报告数值 |
| A09 / F2-07 | 缺权、尾页总数未知、真正空集合 | 按 §4 输出，隐藏 ID/数量不泄漏，U=null 不转0；布尔/枚举/时间指标按依赖组而非 issue 分母判断；事务失效整体 unavailable |
| A10 / F2-08 | seal 后 reopen/cancel/move/delete、类别/时区更改 | reader 当前获准时原 frozen payload 不变；live 状态单独鉴权，不能拼入旧正文 |
| A11 / F2-08 | payload 丢失/损坏、未知版本、到期 | unavailable/unsupported，不重算或换 latest；hash-only 不算可重放 |
| A12 / F2-09 | stock、旧 update、role skill 规则对照 | 双向 ID 差分/每 metric 权威解释齐全；无旧输入标 not_replayable，无 Provider |
| A13 / F2-10 | exact GET、无项目、其他报告/项目/snapshot、伪装/legacy | design §8.5 矩阵；scope sentinel 不在正文/错误/header/cursor 泄漏。真实 Auth/router |
| A14 / F2-10 | Auth 后 RR 前撤权、RR 后撤权、下一页撤权、commit 失败 | 前者 deny 无正文；后者按请求 cut 线性化不承诺收回在途字节；后续请求 deny；commit 失败无缓冲正文 |
| A15 / F2-11 | current 缺失/换后继、显式旧 ID、原周期重试/响应丢失 | current GET 不生成；旧 ID 不变；原 key 恢复同结果，correction 需新意图；生成部分待 D5 |
| A16 / F2-12 | GET 成功/拒绝，cursor 替换、header/query/path 歧义 | 不 mint token/签发或消费 completion，不写 issue/project/comment/policy，不触发 dispatch/Provider |

每行未来须有 expected/observed/证据位置，拒绝与运行失败分开。该矩阵及现有 M1 测试不代表 F2 测试已经实现或通过。

## 6. 仍需决策与证据

| ID | 当前建议及取舍 | 退出规划前需要 |
| --- | --- | --- |
| D1 范围/时间 | 建议 as-of cohort，stock_as_of 与 window events 分列；窗口由可信 report input 明确给出，不在触发/执行时间间静默切换 | 审阅 cohort/事件时项目、滚动/自然周期、end 状态需求；若要求整个项目本期吞吐/移出任务，先补历史与权限 |
| D2 历史/报告身份 | activity 为有边界的证据；iteration/收据只覆盖子集；current report ID 可自排除，历史报告需 server provenance | 全 writer/删除/导入/补录/类别变化矩阵、baseline/sequence/有效时间、报告分类与 retention 证据；不足则允许相关指标 unknown |
| D3 保留/冻结读 | 建议获准采集的最小 facts 依 report/project 当前授权继续可读；source 后移出/删除不改旧事实，live 单独验权 | 保留期限、大小、删除/撤回边界、未知 schema 恢复；不存无限正文或以 digest 代内容 |
| D4 GET/错误 | 建议两个 report-anchored GET，项目投影内嵌；无业务筛选 query 或新 project/list route | 精确 path/分页编码、401/403/404/503、冲突 header ingress；RR 线性化、fresh claim 读旧快照的规则 |
| D5 采集主体/发布 | GET 只读已 seal；Reporter 不获创建权；周期 key 与 correction 分开 | 谁采集、准入/限额/取消/幂等、seal/current pointer 原子性、故障回滚、schema/索引独立设计；本轮不指定可写新主体 |
| D6 口径/证据 | coverage-v1、每指标 unknown、旧模板差异单列 | 手算 oracle 与 A01–A16 实际结果、源码 hash/schema/adapter/semantics/时区版本、cut 证明；当前没有 F2 产品运行证据 |

方案可审阅，但尚未达到实现准入。D1–D6 解决并审阅通过后，仍需最终 F2 范围的独立明确实现批准。M1-S1 批准、in_progress、恢复指针与本引导语均不授予 F2 实现权；M1-S2–S6、F3、F4、F5-A–E 保持未完成。

## 7. 本轮重点源码身份（只读）

HEAD 为 c79dfe304；下表固定本轮读到的关键工作树内容，避免并行工作下仅凭 HEAD 推断文件身份。后续设计/实现前须重新核对。其他源码锚点见 §1。

| 文件 | SHA-256 | 相对 HEAD |
| --- | --- | --- |
| `server/internal/projecthealth/snapshot.go` | `1538c7c215c429eae99729c02186208a48a27b5491d02ad024d1a178a549fe40` | 未改动 |
| `server/internal/projecthealth/collect.go` | `52535e9b339d42cec52fc4b2cdcbf8e0713ac703e082e975ba1684945c7db0e5` | 未改动 |
| `server/internal/projecthealth/counts.go` | `b81766a294a3c610823638094ff21995a9bc47c6ebbfad5daf252d52f5ceb2cd` | 未改动 |
| `server/pkg/db/queries/project_health.sql` | `5ea624173fc3e7075e66b8c547c92325ea2b36a85eeb7914ca2bfa538a9dda98` | 未改动 |
| `server/pkg/db/queries/activity.sql` | `d7e86ec39eb5bdabe120471803cc22295bf3a67bf3c471041b231fd066056765` | 未改动 |
| `server/cmd/server/activity_listeners.go` | `0cf6daf0833ca4a7074dd6f4ae9eefcbd550b2d07e1336bc34a4efaa9d9d8213` | 未改动 |
| `server/internal/handler/project_update.go` | `45855914d21e8792a90e73274986dcdf887b81a68b3030c262d75c6c7c5fc70f` | 未改动 |
| `server/internal/auth/task_capability.go` | `7cd8a09b08224ee20584f97b0cacc7773e0649b22d8547c924d9b7749579f8c5` | 未改动 |
| `server/internal/handler/task_capability.go` | `15c014178de5a37ee657c37cb072456e3fe73327b5d41f1127da935945760a2c` | 未改动 |
| `server/internal/middleware/auth.go` | `18392905c77f59897c63b4bc2490b20c95389bf49c88748e934643ca9b6e5dfe` | 未改动 |
| `server/internal/service/builtin_role_skills/multica-progress-report/SKILL.md` | `b7025fa10806815553dbead431824c2659bf9501de57713b1dbc8c7a25834f81` | 未改动 |
| `server/internal/service/builtin_autopilot_templates/daily-progress-report/PROMPT.md` | `1b27dd74939630ab5d03bb89a4a02ffe6c9c6dc578b23d9937afa74d148bda5d` | 未改动 |
| `server/internal/service/builtin_autopilot_templates/weekly-progress-report/PROMPT.md` | `4dcad2bd16271b09c732ed1b31ac04e70d515a6b2fc6ae1ffecb421376b8ae9e` | 未改动 |

## 8. 本次续接设计复核（2026-11-13，设计文档范围）

### 8.1 基线、记录归属与授权

本次入口 HEAD=`c79dfe304`，branch=`codex/cross-workspace-collaboration`，task.status=`in_progress`，completedAt=null。初始 session 指针为空，已通过 `task.py validate` 后恢复同一任务；恢复指针不启动 F2 实施。初始 Git 状态有 425 个未提交项，包含本任务的规划、JSONL、F2 实现草稿及测试材料；并非 §1–7 那一轮的“374 项、任务目录干净”。

`f2-execution-contract-20261113.md` 和 task.json 已记录另一轮独立授权与实现进度。本轮保留这些材料，不改写其历史，不将它们或 M1-S1 批准当作本轮实施许可。下文的建议仍是可审阅合同；并行代码存在、测试文件存在或先前测试日志存在，都不等于本次已验证。

| 核对点 | HEAD c79dfe304 | 本次工作副本（只读观察） | 对设计的影响 |
| --- | --- | --- | --- |
| projecthealth | `snapshot.go:189` complete 只检 unknown_status；Compute 使用当前 tzdb；Collect 不返回输入 | dirty `snapshot.go:123–160` 新增 CalendarFacts/ComputeWithCalendar，`collect.go:33` 新增 CollectInput；complete 仍非通用来源证明 | 可以研究复用边界，不能把改动当已交付基线或独立重放清单 |
| report snapshot 路由 | closed TaskCapabilityGuard 没有 F2 allowlist | dirty `task_capability.go:79–83` 增加 exact Reporter GET；`router.go:2248–2250` 与未跟踪的 handler/service/middleware 提供候选接线 | “当前路由不允许”仅适用于 HEAD；本轮不增加、删除或验收这些路由 |
| 报告分类 | 没有独立 F2 collector | 未跟踪 `report_snapshot_collect.go:114–123` 仅当前报告/已有 seal/受信内置自动化链为 report，其余为 unknown | 必须说明可用性代价；不能把未证明是报告视为已证明是业务 |
| 指标集合 | project-health 当前正式范围统计 | 未跟踪 `reportsnapshot/compute.go:39–58` 同时定义 formal_stock、stock 和 window | 两种存量必须具名；业务排除不能偷换对照总体 |
| 旧来源 | project_update revision 的统计结果；Timeline 2,000 cap；activity 非原 writer 原子历史 | 对应旧来源限制仍在；新 collector 私有分页不修复 producer 漏记 | 物化 retained-visible rows 与证明全历史完整是两项验收 |

M1-S1 的 `463b201fa`、602 files / 7,332 tests 和 renderer 2/2 仅引用已保存收据；本次未重跑、不改变其验收结论。M1-S2–S6、F3/F4/F5-A–E 仍未完成。没有真实 Provider、POLA-1、共享项目策略、发布或部署操作。

### 8.2 冻结对象与可重放承诺

重读承诺分成三层：原 sealed 结果字节/摘要不变；支持该语义版本的纯计算器只用物化输入可重算同一结果；来源完整性另由 manifest/gaps 证明。摘要一致仅证明保存内容一致，不证明历史无遗漏或模型结论正确。普通 GET 可校验已存内容，但不重新采集或拿 live 数据补算。

除 §8.2（design.md）的 envelope 外，物化合同还须逐项明确：

| 事实组 | 最小保留内容与权威 | 不足时的处理 |
| --- | --- | --- |
| 范围/分类 | workspace/project/report 规范 ID、report/project revision、ordinary/formal admission 谓词版本、候选 ID 集合、每个 report/business/unknown 分类及正面来源依据 | 丢失/不可信 origin 或业务分类无证据为 unknown；标题、负责人、当前提示词不能决定分类 |
| 状态/成员历史 | cut 状态及类别映射；窗口事件的 before/after、effective/recorded time 区别、原生 revision/sequence、当时项目/准入证明、窗口前 baseline | 缺事件时间、因果序或归属则影响对应窗口/期末指标；当前类别/项目不能倒填历史 |
| 健康输入 | 项目状态/到期日/lead/进入进行中时刻及依据，任务状态/日期/关联主体最小 validity/environment 布尔事实，latest update 的 ID/revision/published_at 与集合读取证明 | max(published_at) 数值可供纯计算，但没有选中源身份/版本和读取证明不能充当完整来源清单 |
| 日历输入 | timezone 及其配置/请求来源、period_mode、anchor/来源、UTC 微秒 [start,end)、as_of；reference_date、latest_update_date、in_progress_since_date，或能原样重建这些日期的冻结 offset；时间规则版本/可识别 tzdb provenance | 只保存 IANA 名称或 start/end offset 不够；历史 update/anchor 的当地日期也可能因 tzdb 升级漂移。无法取得 tzdb 版本要明示，不伪造；已解析值仍须保存 |
| 外部/叙述来源 | PR、deployment、结果指标及评论若纳入，需已获准的最小版本化事实、独立 source adapter 和依赖定义 | 首包未采集标 not_collected；指标必需而读不到标 unknown。不得访问远端补齐、保存任意正文或推断已部署 |

每种 source 记录 kind、authority/adapter/schema version、实体身份与 native revision。没有 native revision 时明确 `content_revision` 只标识本次投影，不假装记录了中间版本。语义版本应定位冻结的规则产物/摘要；同时保存指标目录、scope/classification、时间、coverage、排序、canonical encoding 版本。仅写一个“v1”而无法找回规则不满足长期重放。

规范化对象键、唯一身份排序、UTC 微秒、null/空数组、整数精度、字符串编码和 rational 的表达；规定重放使用原始字符串字节还是特定 Unicode 规范，不在不同读取时临时归一化。相同身份+revision+payload 重复页去重；相同身份+revision 异内容为 conflict，不能静默最后写入胜出。facts/result 摘要不含页大小、随机 snapshot ID、GET 时钟；页收据属于不可变采集审计，可以另有摘要。旧 snapshot 整体摘要不得被 current 切换或新读权限时间改写。

### 8.3 来源读取、完整性与最小披露

每源分别声明 `read_complete`（本 cut 可见且获准集合是否遍历完成）、`producer_complete`（窗口历史是否有无损交付证明）、`retention_complete`（所需起点/baseline 是否仍保存）。不存在的 producer watermark 为 null，并列来源能力缺口；空表、EOF、MAX(time/id)、事务开始时间、sequence 分配号均不能自行证明完整提交水位。不同 source 的水位也不能比较大小后拼成“全局完整”。若未来有 producer 水位，必须注明 partition/epoch、提交/交付语义及截至何时无缺口的协议证明。

内部分页合同固定 cut C、scope/predicate/version、排序键方向、首末键、每页数量/内容摘要、终止依据及 limit+1 截断检测。分页完成后还要验证身份集合与页边界一致；活动 `(created_at,id)` 是遍历全序，并非状态变化的因果序。恰好命中页上限必须继续查 EOF，不能少读最后一页。达到安全上限必须记录未知尾部，不伪造 EOF。

迟到分为“业务发生于窗口内、C 后才提交/交付”及“C 前原 writer 已成功、历史 listener 遗漏”；两者都不能靠 `created_at <= as_of` 修复。旧快照只记录 C 当时已知缺口；后继补证/correction 使用新身份和 cut，保留 correction_of，不能修改旧 gap 或数值。连接/事务失效需整体重启，不能复用前几页；只有仍有效的同 cut、当前准入已通过且单源失败可恢复时，才可 seal partial。

采集阶段 source 不可读只保留安全的 gap（来源类别、原因、受影响指标、extent 是否未知），不返回隐藏 ID、数量、摘要或低熵内容 hash。report/project/reader 的根准入失败是整次 deny，不能封存为“成功的空项目”。重读阶段先复核当前 Reporter，再读取冻结事实；其权限已撤销则直接 deny，无快照正文，不能返回历史内容再附“权限未知”。

内部可重放清单与 Reporter 公共投影需要字段级白名单。project-health 内部收集了成员/运行时/小队事实，不因此获得目录 API 或这些 ID 的披露权限。对于分类依据，保留受信判定、规则版本及必要安全 provenance；origin_id、autopilot_run ID、producer/actor/runtime ID 也需逐字段审查。内部 hash 不能自动公开。若内部清单比公共投影丰富，分别标识内部完整性摘要与公共投影摘要；不得声称公共摘要验证了未公开内容。推荐冻结足以重算公开指标的安全布尔/日期/类别投影，并把需更高权限的来源证明留在内部审计。

### 8.4 逐指标权威和 coverage 精化

`formal_stock` 若保留，只表示当前普通正式任务存量，包括报告及分类 unknown 的记录；`stock` 表示有业务分类证据的去重存量；`window` 表示该 as-of cohort 中有当时项目/类别/时间证明的窗口事件。生产 collector 目前没有一般普通任务的正面 business 分类，故 `formal_stock.total` 可为 exact，而 `stock.total` 与窗口业务总数仍 unknown。这是有意暴露的证据缺口，不能换标题把它说成“业务进度已完整”。是否接受该首包可用性，仍是需要产品审阅的取舍。

§2 的每项指标权威继续有效，另补以下精确边界：

| 指标/对照 | 权威规则与验收细节 |
| --- | --- |
| formal_stock/stock 的 total、completed、cancelled、open、unknown_status、closure_ratio | actual project + admission + 类别；closed stock=completed+cancelled。旧 open 包含 unknown_status，F2 known open 不含。只有完整可比较集合才断言四类之和为 total；分类/尾部未知时不能对局部数值套等式 |
| blocked、overdue、unassigned、in_review、risk_union、execution_environment_unavailable | 当前 projecthealth.Compute 的 issue 集合与关联 validity 是规则权威；risk_union 按 ID 并集，不求和。未知引用不等于已证实“无负责人”；runtime 非就绪不等于真实 Provider 失败 |
| project_overdue、lead_valid | 各自项目状态/到期日/当前日、lead 引用有效性；缺依赖 null+unknown，lead 证实缺失或按冻结规则无效（如已归档 agent/squad、无效小队负责人）才可 false。完成/取消项目不因到期日而判项目逾期 |
| latest_update_at、progress_age_days | 完整 published update 源；无 update 时分别 null+exact，不能编成0。progress_age_days 只来自 latest update，不把 in_progress_since fallback 天数填进该字段 |
| health、reasons/stale_progress | 旧规则优先级：unknown 类别 unavailable → empty → risk → attention → clear。stale 在无 update 时可用 in_progress_since，按当地日期差 >=7。environment_unavailable/ended_project_open_issues 可出现在 reasons，却不独自将 health 升为 attention；禁止重写旧算法后自证 parity |
| window.created/completed/cancelled/closed/reopened/end_* / net | 按 §2 集合公式；stock_as_of 与 state_at_end 独立。done→cancelled 计 cancelled，cancelled→done 计 completed，均不新增 closed；缺 before 类别不凭 to 值断言重开/关闭 |
| PR/deployment/产品结果/人工判断 | 各自有权版本事实；人工 progress update 是其发布文本/判断的权威，F2 是已封存 scope/语义/cut 的统计权威，project-health 是当前健康视图权威；三者不互相覆盖 |

每 metric 的候选总体要先定单位与谓词。若 source query 已过滤到普通正式项目集合，E 只计该集合的排除前候选；未读取的跨项目/非正式/collaboration 记录不得编进 X 或暴露计数。范围谓词中的排除规则单列；只有实际安全可枚举且属于声明总体的候选，才进入 I/X/U。I 包括“已证实对该指标贡献0”的任务；X 是有正面依据的不适用/报告排除，不是谓词为 false，更不是读失败。每候选只能属 I、X、U 之一，多条 gap 不能重复增加 U。

已知总体时 E=I+X+U，coverage=I/(E−X)；未知尾部时 E/U.count/ratio=null，U.known_count 只计实际已知未决候选。所有必需依赖均需可判定，才输出 exact；`value=null` 与单列已核实下界分开，lower_bound=0 从来不等于 total=0。缺失 source 并不必然污染所有 metric：只沿显式 dependency 图传播；不依赖该源的指标保持原 quality。覆盖率不是准确率/置信度；多个指标的比例不能平均成 complete。完整已知空集的 count=0，比例无定义为 null；未知空返回不能沿用这项规则。

只读对照必须包含两步：先把旧 project-health 的完整 formal IDs 与候选 ordinary formal IDs 双向作差（旧 SQL 未排独立 collaboration，候选 SQL 已排）；再在共同的普通 formal 冻结输入上比较可比指标，最后单独应用报告排除。业务过滤后的 health 不能冒称原项目 health；需显式标统计 scope。使用 HEAD 的旧规则/手算 oracle 验证并行改过的 reducer，不能双方都调用修改后的 helper 形成循环证明。旧 update 没有原输入清单时仅可比其保存字段，标 not_replayable；不发布、更正或双写 update 取样。

### 8.5 Reporter 的精确读取合同与拒绝顺序

候选能力仍只有 GET `/api/issues/{report_uuid}/report-snapshots/current` 与 GET `/api/issues/{report_uuid}/report-snapshots/{snapshot_uuid}`。frozen-project 指该报告内的冻结项目投影，绝不表示新增 project/workspace 列表读取能力。首包建议不开放 query 或外部分页；内部采集分页必需。16 MiB/10,000/100,000 是并行执行稿中的候选限额，不是由本轮确认的容量证据。

验证 R 必须是本次请求中首次取得 PostgreSQL RR 数据快照的查询所建立的可见性边界，不是 BEGIN 或 SET TRANSACTION 的语句时间。token/task/runtime lookup、原始 header 与 verified identity 比较、实际 report/project、current 选择及 snapshot 正文都使用绑定同一 tx 的 q；guard 不能检完 A scope 后让 handler 从 pool 消费 B scope。原始 header 应在 Auth 覆写前保留，验证后在同一 R 比对；“Auth 前已根据 verified identity 拒绝”是不准确的顺序描述。

固定等式：snapshot.workspace=verified token.workspace=report.workspace；path.report=snapshot.report=capability.issue；snapshot.project=report.current_project=capability.project。OptionalUUID 统一 null/内部空值语义，非法非空值拒绝。snapshot ID、scope 和封存头通过后才释放内容；不能先返回 digest/数量/来源 ID 再检查。报告 revision 后续普通编辑不破坏历史读取；报告变项目则拒绝。producer claim 是 provenance，不要求等于合法 fresh reader 的新 claim；reader 的 live claim 必须仍精确有效。

当前报告若变为独立 collaboration 或不满足 formal 准入，建议同一R返回403、零正文；这项比单纯UUID/project等式更窄的读取条件须在D3/D4最终审阅，不得由工作副本某一入口的缺省行为决定。源任务在seal后转出formal集合只影响新采集；只要报告仍符合读取准入且保留合同有效，旧源事实不重算。

| 分类/场景 | 拟定 HTTP 行为与不可泄露内容 |
| --- | --- |
| valid verified Reporter + exact report/frozen project + supported sealed snapshot | 200；原封存内容，无新增采集/业务写 |
| 无项目且三方均无项目 | 200 仅 report scope；project metrics not_applicable，无 workspace 兜底 |
| 初始缺失/无效/过期 token | 401，通用错误；不含 snapshot/source 身份、digest、隐藏计数 |
| Auth 后、R 前失权；terminal task、claim/runtime/主体成员/invoke 不匹配；角色/legacy 冒用；其他 report/project | 403；不降级为 gap、partial、空成功或使用历史权限 |
| 在已获准 report 下 current 不存在、ID 不存在或指向其他 report 的 snapshot | 同形最小404；不能揭示错配对象存在。权限失败先处理 |
| 非规范/编码路径、额外段/重复斜杠、非 GET（含 HEAD）、任何 query、重复/冲突身份 header | capability 不放行；认证角色路由违规统一403，格式400仅限已完成身份/scope 验证的解析错误。不依赖 redirect 或全局放宽规则；精确 wire 状态待 D4 最终审阅 |
| R/cut/DB 不可用、commit 失败、支持版本外或内容完整性失败 | 最小 unavailable（建议503；unsupported 内部原因单列）；零快照正文，不回退 current/live |
| R 建立后撤权 | 当前请求仅承诺 R 的一致性，不承诺收回在途字节；下一次 GET/页必须拒绝。再次读同 RR 不能获得最新权限 |

所有成功和早期拒绝路径均 no-store，包括 Auth/guard 在 handler 之前退出的路径；不得发快照 ETag/Last-Modified、敏感 error details、Location 或可枚举 cursor。no-store 不能撤回已交付字节，也不能替代客户端清除失权后的受保护结果。DB/runtime 的既有内部可观测性可继续，但 GET 不写业务状态、评论、策略、凭据、完成授权、dispatch 或 Provider。

如果以后允许 public facts 分页，需另审唯一 GET 形状与 cursor：绑定 immutable snapshot/scope/projection-schema/manifest digest/order/last key；每页新的 R 当前验权；current 第一页解析后必须固定 snapshot ID，不能跟随 alias 跳转后继。超出限额且尚未批准分页时，明确 unavailable 或已定义的 partial，不能悄悄删去物化事实。

### 8.6 补充验收反例与证据门

保留 A01–A16。下表是其细化，不是产品通过记录；全部 observed=`not_run_this_turn`。未来结果须记录 case/test name、expected/observed、source/DB identity、cut 屏障与失败证据。

| 子场景 / 主需求 | 固定输入或动作 | 必需 expected |
| --- | --- | --- |
| A02-calendar / F2-02 | New York 自然日 2026-03-08、11-01；上海 2026-11-12；升级 tzdb/切换 workspace timezone 后重读 | 前两者 UTC 窗长23h/25h；上海 [11-11T16:00Z,11-12T16:00Z)；rolling24h 始终24h。旧 reference/update/anchor 当地日期和 health/age 不漂移；新建窗口受当前规则验证 |
| A03-boundary / F2-03 | 2,001 条活动，页大小1/37/500/2,000；再用恰好2,000条、重复页和同版本异内容 | 前者最终唯一身份2,001且 EOF 可证；恰好整页继续确认 EOF；重复不加计数，内容冲突拒绝/明确 gap，不选择赢家。结果与摘要不依赖页大小 |
| A04-cut / F2-03 | T0 较早分配 time/id，T1 在 C 首次查询后才提交；C 的翻页继续 | T1 在整个 C 均不可见，即使其 tuple 小于最后 cursor；新 cut 可见。旧 cut 无生产完整性证明仍标 unknown，不能以旧 max tuple 排除迟到 |
| A06-terminal / F2-05 | E 在 start 前 done，窗内 done→cancelled→done；其余使用 §5 A/B/C/D/R oracle | E∈completed∩cancelled，E∉closed/reopened；原 oracle created={C},closed={A,B},reopened={A},net=-1 不变；D 恰好 end 的事件只影响其后 stock，不进窗口 |
| A07-membership / F2-05 | 同一业务任务先 P→Q；另一个 Q→P，分别在 C 前后发生；历史类别/项目 proof 缺一项 | C 前移出者不在 as-of cohort，不能宣称完整项目吞吐；移入者只在有事件时项目证明的指标中参与；C 后改变仅影响新快照。期末状态缺完整链则 unknown |
| A08-taxonomy / F2-06 | R 当前报告、H 内置历史报告、B 标题含日报且有受信 business 证明、N 普通但无分类证明 | business total：E=4,I=1,X=2,U=1，coverage=1/2，value=null、已核实下界1；formal total=4 exact。生产没有 B 证明时不得靠标题或非自动化身份把 N 变为 B |
| A09-dependency / F2-07 | 5 候选中 I3/X1/U1；未知候选有两种 gap；另令 update 源失败；另设已证明空集 | U仍1、ratio=3/4；未知尾部改 E/U.count/ratio 为null。update 相关 age/health unknown，独立 project_overdue 不受污染。已证明空集 counts0，读取失败不是0 |
| A10-version / F2-08 | seal 后只改 report revision/源状态/源删除；fresh valid claim 读旧 seal；再移 report 到 Q | 同项目且保留权限成立时旧 facts/result 不变；原 producer claim 不限制 fresh reader。移 report 后 deny；源删除的保留规则不撤销当前读检查 |
| A12-health / F2-09 | in_progress项目无published update、in_progress_since距当地日期8天；未completed/cancelled的空项目且due已过；另一个依赖齐全且只有environment unavailable reason的非空项目 | 第一例age=null、stale=true；第二例health=empty且可带project_overdue reason；第三例environment reason本身不把clear改attention。与HEAD旧规则及手算比对，不以新helper自证 |
| A13-disclosure / F2-10 | 每字段放置 scope外 UUID/标题/actor/runtime/roster sentinel；正文、error、headers、digest、cursor 检查 | 未获准字段/计数/可反查摘要无泄漏；合法最小 validity 投影可保留。snapshot错配与不存在最小404同形；Auth/guard提前拒绝也no-store |
| A14-first-read / F2-10 | Auth通过，BEGIN后但该RR首次取得数据库快照的数据查询前提交撤权（该查询可能读取token/task/runtime，并非特指snapshot表）；另在R建立后撤权；commit故障 | 第一种deny且零正文；第二种仅按R决定本次、下次deny；commit故障不释放缓冲正文。runtime lookup与实际读取必须证明同tx |
| A14-report-kind / F2-10 | seal后report成为独立collaboration或formal准入被撤回；再用同project的fresh Reporter读取 | 按建议准入规则403且零正文；仅源任务发生准入变化时不重算旧seal。该产品取舍须在D3/D4签定，测试不得先把任一当前实现当oracle |
| A15-cancel / F2-11 | 采集期间取消/断线，提交前取消；提交成功响应丢失；current已被correction切换后旧ID/旧key恢复 | 前者未seal/current不推进；提交竞争未知不能宣称取消成功。已提交旧key仅在当前准入下恢复原结果，不重采；旧ID不跟current。采集/幂等机制是将来独立设计，不在本轮运行 |
| A16-errors / F2-12 | 成功、拒绝、超限、无快照、未知schema、HEAD/query/路径变体分别观察副作用 | Provider/dispatch/凭据/授权/issue/project/comment/policy写均为0；no-store全路径；无可读旧内容时不返回任何快照正文。产品验证另行批准后执行 |

### 8.7 剩余决策与证据（不得以并行代码存在代替）

| 门 | 当前可审阅建议/已有并行选择 | 尚需决策或独立证据 |
| --- | --- | --- |
| D1 | as-of ordinary formal cohort；stock与window分开；自然/滚动窗口显式 | 产品是否接受不包含C前移出任务的吞吐；事件时归属/准入历史的适用合同；请求时区与workspace健康时区不同时的双向对照；calendar/tzdb重放证据 |
| D2 | 严格正面报告/业务分类；缺历史保持unknown；formal_stock另列 | 是否接受首包大量business/window为unknown；如要求精确值，先设计受信business分类与全writer历史，而非默认普通即业务。逐writer/create/status/move/admission/delete/import/repair的来源矩阵、原子性/保留/序列证明 |
| D3 | 最小安全事实保留，源变化不回写；并行稿选report/workspace删除清理与有界大小 | 源删除/撤回、report移回旧project、project删除、合法清除的产品保留决定；当前report变pending/rejected或独立collaboration是否拒绝读取（建议拒绝，A14-report-kind）；字段级公开/内部manifest边界；限额实测和超限契约，不能仅列常数 |
| D4 | 两个exact GET、无query，当前RR权限；raw header保留后验证 | 真实Auth/router的角色/path/header/no-store/错误优先级矩阵；首条数据查询前后撤权、当前report普通/formal准入变化与commit failure证据；public分页如需要另审；不能从GET批准推出采集权 |
| D5 | GET绝不生成；并行稿有人类显式采集、cycle/request/correction区分 | 谁能采集及其准入是独立能力决定；seal/current/幂等/取消响应丢失的事务证明；报告级sequence只选current，不是源水位；本轮无迁移/路由/实现批准 |
| D6 | 逐metric权威、coverage与同cut只读parity | A01–A16含本节子场景的实际expected/observed；旧规则固定身份、双方ID差分、未解释差异为阻断；生产unknown边界与合成完整fixture分开；独立审阅及零业务写证据 |

本次交付是可审阅设计与验收条件，未将 D1–D6 标为本轮已批准或产品验收通过。后续实施须按当时有效的明确指令重新核对范围和源码；本轮到文档验证为止。

### 8.8 本次关键来源身份

以下为本次只读审阅的工作副本SHA-256。HEAD列用Git对象读取对照；未跟踪文件仅为并行草稿，不是交付证明。源码若改变，后续运行验收需重新固定身份。

| 文件 | 工作副本 SHA-256 | 与 HEAD c79dfe304 的关系 |
| --- | --- | --- |
| `server/internal/projecthealth/snapshot.go` | `5e6c52a1c72ea559ff89091d9057e7d24f5e7666dcb1bde1617459336f4b976f` | 并行修改 |
| `server/internal/projecthealth/collect.go` | `99cf573ae88e8c4a0eb65e135b41467eacdb718139e6a3eb01572636b3237b94` | 并行修改 |
| `server/internal/projecthealth/counts.go` | `b81766a294a3c610823638094ff21995a9bc47c6ebbfad5daf252d52f5ceb2cd` | 相同 |
| `server/pkg/db/queries/project_health.sql` | `5ea624173fc3e7075e66b8c547c92325ea2b36a85eeb7914ca2bfa538a9dda98` | 相同 |
| `server/internal/handler/project_update.go` | `45855914d21e8792a90e73274986dcdf887b81a68b3030c262d75c6c7c5fc70f` | 相同 |
| `server/internal/handler/activity.go` | `c70f4038e52876de4deb384faeae5b10abcdd8f9e2e7169988eea34a8faf25dc` | 相同 |
| `server/pkg/db/queries/activity.sql` | `d7e86ec39eb5bdabe120471803cc22295bf3a67bf3c471041b231fd066056765` | 相同 |
| `server/cmd/server/activity_listeners.go` | `0cf6daf0833ca4a7074dd6f4ae9eefcbd550b2d07e1336bc34a4efaa9d9d8213` | 相同 |
| `server/internal/auth/task_capability.go` | `7cd8a09b08224ee20584f97b0cacc7773e0649b22d8547c924d9b7749579f8c5` | 相同 |
| `server/internal/handler/task_capability.go` | `0a660c543f49762c8d84a356c402b1669ebe57db8d56fa15df60778bb45f7866` | 并行修改 |
| `server/internal/middleware/auth.go` | `18392905c77f59897c63b4bc2490b20c95389bf49c88748e934643ca9b6e5dfe` | 相同 |
| `server/cmd/server/router.go` | `f5c9a659eb47c6d7a64a33c3657d6431575dfc1a921d10778a131da25c94f028` | 并行修改 |
| `server/internal/service/builtin_role_skills/multica-progress-report/SKILL.md` | `9442a5c627626d1de552dbb5e91d9e6d169d20ec1e42bc3014c4def3962963b5` | 并行修改 |
| `server/internal/service/report_snapshot_collect.go` | `8770c00ef5f2543f3e5256166509f359c66126324b53f484711bb39a7e375bc6` | 未跟踪并行草稿 |
| `server/internal/handler/report_snapshot.go` | `65f99fce78d921dcdc1807c38bd7c4a5172a436fb8f5e9f53f64b54256f1161f` | 未跟踪并行草稿 |
| `server/internal/reportsnapshot/compute.go` | `0bcb799d0a9c443b9ae853a149365d13d0832135ca7ca0aba9fe5ba403236f94` | 未跟踪并行草稿 |
| `server/internal/reportsnapshot/time.go` | `65d77efce0413b4d3f6dd29a262ff2a27d29bf1f392d7588b18d0ad315a3abc5` | 未跟踪并行草稿 |
