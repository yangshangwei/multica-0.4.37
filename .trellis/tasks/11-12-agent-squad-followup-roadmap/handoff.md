# 后续能力交接

> 当前F2状态：最小首包已按本线程明确授权实现并通过确定性验收，见 [F2验收](research/f2-acceptance-20261113.md)。以下设计/待授权文字保留为历史；task仍in_progress，M1其他入口及F3/F4/F5、真实运行禁区不变。

本任务来源于用户批准的阶段结项，保留原路线，不表示全部路线已完成。

- [原 PRD](../archive/2026-11/11-05-agent-squad-rnd-improvements/prd.md)
- [原设计](../archive/2026-11/11-05-agent-squad-rnd-improvements/design.md)
- [归档评估](../archive/2026-11/11-05-agent-squad-rnd-improvements/research/archive-readiness-2026-11-12.md)
- [已交付阶段清单](../archive/2026-11/11-05-agent-squad-rnd-improvements/stage-closure-2026-11-12.md)

旧目录内的路径、源码哈希、审查与调用结果均属于其记录时间的历史事实。新实现必须使用当前源码身份；新验收结果不能改写旧失败。

## 2026-11-12 本轮规划交接

已按用户指定读取当前任务、阶段结项、完成门禁/提示 spec 和独立真实运行验收 PRD，并核对当前源码。开始时仍为 planning，design.md/implement.md 缺失，implement.jsonl/check.jsonl 都为 0 字节；无 active task pointer。本轮没有将 task.py start 当作选中任务操作。

当前首包是 M1-S1：普通单事项查看并选择当前证据 → 明确授权 → 提交完成 → 查询原操作和当前任务。详情见：

- [需求与验收](requirements.md)：16 条首包验收、其他入口的现行合同/待设计/后续顺序、M2 和五个 M3 独立包。
- [设计提案](design.md)：两个无业务写的准备接口、复用既有授权和 writer、两步明确 UI、CLI 五个子命令、原操作恢复。
- [实现计划](implement.md)：门禁、文件 ownership、顺序、验收主层、测试命令及回退。
- [后端核对](research/m1-backend-current-contract.md)、[客户端核对](research/m1-clients-current-contract.md)、[独立规划复核](research/m1-planning-review.md)。
- [续接状态](research/resume-check-2026-11-12.json)、[文档验证](research/m1-planning-validation.json)。两份 jsonl 已填真实 spec/research，上下文只覆盖首包和需保留的既有合同。

不要在实现时重新发明后端授权。已确认的主要缺口是任务范围证据发现、当前规范绑定/期限预览、共享 UI/CLI 的授权消费与未知恢复。准确字段是创建 header `Idempotency-Key`、body `operation_id`，普通 PUT 的 `completion`，创建回查 query `workspace_id`。原 GET `issued/not_checked` 不是当前可用证明；不能因查询404或未 consumed 自动重复写。

原操作创建未知查回原 issued 且未派发完成，可确认已签发并等待首次明确提交；完成/撤销未知查到 issued 则仍未知。已 consumed 后任务重开，保留历史收据并另读当前状态。候选列表必须遵守原 artifact authority，不从审批可见性推导制品可见性，分页游标也不能泄漏隐藏记录。

用户已明确批准 M1-S1 规划并启动现有任务。当前状态、文件 ownership、实现顺序与验收门槛以 implement.md 和各 lane 的 research 记录为准；所有操作继续保留其他工作区改动。后续入口和 F2–F5 需要各自设计，不能从本批准推导执行权。若实现中需实质改变已批准 wire、权限或范围，回到规划审阅。

真实 Provider、知识注入、发布、Hosted CI、部署和外部消息仍未授权；原授权、分系列预算、历史失败与停止条件继续由独立真实运行任务保留。账单对账和 Windows 范围不恢复。

## M1-S1 实现进展（2026-11-13）

普通单事项首包的后端准备 API、Core 原操作协调器、Web/Desktop 共享流程和 `issue completion` CLI 已实现。定向后端 handler/router normal/race 均零失败/跳过；Core 全量 4,546 项通过；M1 与受影响 Views/surface 定向 84 项通过；CLI 全包、vet/build 与内置 skill 合同测试通过。最终独立检查追加了状态/applicability、receipt 精确绑定和过期授权拒绝回归，详见 [最终检查](research/m1-final-check.md)。

实现和测试合同见 [implement.md](implement.md)；lane 证据分别见 [backend](research/m1-backend-resume.md)、[clients](research/m1-clients-resume.md)、[CLI](research/m1-cli-resume.md)。

## M1-S1 验收完成（2026-11-13）

两项验收缺口均已在源码提交 `463b201fa` 的 task-owned 环境中完成；相关验收记录后提交于 `c79dfe304`：

- 修复后完整 Views 套件 `pnpm --filter @multica/views test` 通过 602 files / 7,332 tests；日志见 [m1-views-full-suite-20261113.md](research/m1-views-full-suite-20261113.md)。
- Web production renderer 与真实 Electron renderer 均走通“候选证据 → 明确授权 → 提交 → 查询 receipt/当前状态”。两端都用新建 enforce project、固定 local-synthetic artifact receipt 和数据库观测 PR head；preview `200`、authorization `201`、bound PUT `200`、原 authorization 查询 `200`，receipt 为 `consumed`，当前任务为 `done` revision 2。键盘动作、Web 390x844、Desktop 680x900、截图和 CORS/daemon=0 证据见 [m1-renderer-evidence-20261113](research/m1-renderer-evidence-20261113/)。
- 截图 fixture 不使用 POLA-1，也未修改共享项目策略；真实 Provider、知识注入、发布、Hosted CI、部署和账单对账仍关闭。

历史的 off/observe Demo 检查和 POLA-1 的两条状态事件仍是历史事实，不作为本轮 renderer 证据。后续不得把共享 Demo 或 POLA-1 当作 UI fixture。

## F2 规划续接（2026-11-13，design only）

接续时 HEAD=c79dfe304、branch=codex/cross-workspace-collaboration、task.status=in_progress。当前 session 指针原为空，经 task.py validate 后恢复为本任务；这是恢复既有任务，不是批准 F2。初始 374 项并行未提交改动，本任务目录干净，已保留工作树/暂存区基线用于最终核对。

可审阅入口：

- [requirements.md](requirements.md)：F2-01–12 可观察结果，保留 M1-S2–S6/F3/F4/F5 后续矩阵。
- [design.md §8](design.md)：可重放最小输入与结果、版本/digest、采集 cut 与读取授权 cut、scope/time/coverage、两个 exact GET 与拒绝矩阵。
- [F2 research](research/f2-deterministic-report-snapshot-planning-20261113.md)：当前源码锚点、每 metric 权威/公式、双向只读 parity 方法、unknown 示例、A01–A16 手算/未来验证矩阵、D1–D6。
- [implement.md §9](implement.md)：文档工作与未完成规划门分开，未来验证顺序不代表可以执行。

主要修正：不能仅存来源 ID/digest；cursor 不等于 producer watermark；activity listener 不保证原状态写与历史原子一致；unknown cardinality 可以为 null，不能造0或1。stock_as_of 与 window events 分开。current 是只读已封存指针，显式 snapshot ID 才保证不变。采集时 source 不可读为安全 gap，读取 cut 前 Reporter 失权则 deny 无正文；新 GET-only 不取消既有当前报告 inert comment POST。原始冲突 header 目前由 Auth 覆写，并非一律拒绝；严格拒绝需新 ingress 设计。RR 建立后撤权不等于撤回在途字节。

仍需审阅/证据：D1 范围与时间，D2 全历史/报告 provenance，D3 保留/大小/删除，D4 GET/错误/header/分页，D5 采集主体/幂等/原子 seal，D6 coverage 与实际只读对照。不能把这些未决门标成已冻结或通过。当前仅设计，无 F2 产品测试或运行证据。

本轮不改产品代码、迁移、路由、共享策略，不调用 Provider，不操作 POLA-1；不新建/归档任务，不暂存或回退其他改动。M1-S1 批准不延伸到 F2；roadmap 继续 in_progress，其他 M1/F3/F4/F5 未完成项保留。真实 Provider、知识注入、发布、Hosted CI、部署和账单对账继续保持原边界。

### 本轮文档验证

- `task.py validate` 通过：implement.jsonl 11 条、check.jsonl 18 条，保留为 M1 历史上下文；`task.py current` 指向本任务。
- 44 个 Markdown 本地目标存在；F2-01–12 全部映射 A01–A16。task.json 可解析、in_progress、completedAt=null，F2 implementation_authorized=false。
- 只读独立审阅指出并已修正文档的两处问题：无项目内部空字符串与公共 null 的规范化；project 布尔/枚举/时间指标的 value_kind 和依赖组 coverage，不套 issue 分母。
- 与开始时基线核对：374 个既有 dirty path 状态/文件摘要、暂存区 diff 均未改变；M1 原批准/hash、design §1–7、implement §0–8、M1-S2–S6/F3/F4/F5 表格行保留。
- 本轮修改仅本任务 7 个已有规划文件；未暂存、未提交。`git diff --check -- <本任务目录>` 通过。未运行 lint/typecheck/产品测试；本轮没有代码变更，M1 的历史测试通过不计为 F2 证据。

## F2 实现启动（2026-11-13）

用户随后明确“F2 授权 ，请推进”。本批准独立于 M1-S1，覆盖 F2 实现与任务自有验证。前述 design-only/未授权文字属于上一轮历史状态；当前按 [F2 execution contract](research/f2-execution-contract-20261113.md) 收敛 D1–D6 并执行。来源不能证明的历史继续 unknown，不扩充真实 Provider、POLA-1、共享策略或其他 roadmap 包。task 保持 in_progress，不新建或归档。

## 本次续接交接：F2 设计与验收审阅

本次用户明确限定设计文档，上一节及已有F2代码/测试保留为并行工作，不作为本轮授权或通过证明。入口HEAD c79dfe304、同分支、task in_progress、session指针空，初始425个dirty项；task.py validate通过（implement 14/check 20条）后恢复原任务指针，状态不变。本轮写入范围仅7个已有规划/任务文件，M1产品与验收收据未修改。

可审阅入口为 [F2 research §8](research/f2-deterministic-report-snapshot-planning-20261113.md)、[design §10](design.md)、[需求细化](requirements.md) 和 [本轮文档计划](implement.md)。主要新增：

- 冻结reference/update/anchor当地日期与offset，使tzdb改变后仍能重算；区分保留事实、原结果摘要和来源完整性。
- formal_stock、business stock/window、project health范围分别说明；严格分类未知的实际可用性代价可见，I/X/U及未知尾部有数字oracle。
- 来源读取、producer历史、retention三个完整性维度；2,001条/整页/迟提交/terminal互转/移入出/分类缺失的确定性子场景。
- 固定HEAD旧规则、共同输入和双向ID差分作只读parity；无update时age与stale、health优先级有明确expected。
- Reporter仅exact当前报告/冻结项目GET；R在首次数据查询建立，raw header保留与验证分阶段；当前失权deny无正文，Auth/guard早退也no-store，公共manifest/envelope逐字段最小化。

D1–D6仍有产品选择和证据门，详见research §8.7；尤其as-of cohort与大量业务unknown是否满足首包、源/报告/项目删除及准入变化后的保留规则、公开字段、容量/分页与采集主体。本次未开展F2代码修复或产品验证，已有并行测试材料不登记为本次通过。

后续继续本任务，不创建/归档、不将roadmap标完成；M1-S2–S6/F3/F4/F5-A–E未完成项和真实运行限制保留。不要从本轮引导语、指针恢复、M1批准或并行执行文字推导本轮F2实施授权。

### 本次最终文档验证

- task.py validate通过：implement.jsonl 14条、check.jsonl 20条，均未改动；当前session指针为本任务，status=in_progress、completedAt=null、HEAD仍c79dfe304。
- 61个Markdown本地目标存在，代码围栏配对；F2-01–12/A01–A16及本次子场景映射存在，M1-S2–S6/F3/F4/F5-A–E仍保留。
- 以入口30,003个文件路径摘要核对，仅prd.md、design.md、requirements.md、implement.md、handoff.md、research/f2-deterministic-report-snapshot-planning-20261113.md、task.json这7个文件由本轮改变；其他既有产品/迁移/路由/测试及暂存区未变。去除本轮新增章节/状态说明可还原原任务文档与meta，保留并行授权记录、M1批准哈希和历史证据。
- 两个只读子代理复核来源/指标和Reporter权限；已补lead无效条件、in_progress/项目未终结的fixture前提、RR首次数据查询定义，以及report变为非正式/独立collaboration的待决条件和命名验收场景。未运行产品测试，不将静态审阅写成A01–A16通过。
- git diff --check限定本任务通过。未暂存、提交、新建或归档任务；未运行lint/typecheck/Go/Views/E2E，本轮仅修改文档。没有Provider、POLA-1或共享策略操作。

## 最小首包收敛续接（2026-11-13）

用户在同一线程依次确认“接受最小首包”“继续保留”“人工显式采集”。已立即记录PRD及task.meta.f2_minimum_scope_convergence。当前设计为：正式任务存量先行，业务分类/窗口历史缺证保持unknown；源删除/移出后最小事实随报告保留，当前读权限仍必需；当前有报告编辑权限的人显式采集，Reporter仅GET。完整历史和自动周期后置；这三项设计选择不授权本轮实施。

审阅 [最小首包合同](research/f2-minimum-contract-20261113.md)：包含公共字段白名单/内部来源、主要writer和局部历史矩阵、采集/原key/correction恢复、保留/撤回/当前资格、候选限额及实际候选测试的证据缺口。原F2 research仍是历史与完整A矩阵依据，不再反复将这三项选择列为用户未答。

下一步技术门：公共安全facts能复算指标、编辑权限事务准入、明确撤回时的内部定位/阻读/清除机制、report资格/移回规则、容量边界、真正同cut的只读parity及首次RR查询前后撤权证据。局部triage_action/iteration是有效来源，不笼统说无历史；它们仍不证明全部ordinary任务历史。

本次继续仅改规划文档；未修改并行F2代码/迁移/路由/测试、M1证据或JSONL。任务仍in_progress，不新建/归档、不提交，不调用Provider或操作POLA-1/共享策略。

### 最小首包文档验证

两份只读独立复核完成，已修正取消/断线不能自动重启采集的歧义，以及无项目时formal/business/window/health均not_applicable。公共投影、局部来源矩阵和具名测试入口通过静态核对；这不是产品运行通过。

task.py validate通过（implement 14条/check 20条），68个Markdown本地目标与围栏检查通过，task JSON有效且in_progress、completedAt=null，当前指针及HEAD c79dfe304不变。原meta、M1设计/计划/后续路线段落保持原样；git diff --check限定任务通过，新文档无尾部空白。

与本次30,123路径入口摘要比较，本会话仅改变prd/design/requirements/implement/handoff、F2 planning research、task.json并新增f2-minimum-contract，共8个本任务文档。源码、迁移、路由、候选测试、JSONL与暂存区未变。扫描期间观察到另一任务11-13-pay-241-collaboration-acceptance的11个文档发生并行变化，已单列为外部并行变化并保留，没有清理/回退或纳入本任务证据；因此不宣称整个工作副本未变。

本次未运行lint/typecheck/Go/Views/E2E/Provider，也未提交或执行任务生命周期完成操作。后续仍是F2设计技术门与证据，不将M1或本次三项产品选择扩成实施批准。

## F2 实现与验收交接

上述design-only记录之后，用户明确回答“授权”，允许F2代码/验证，并选择撤回“本报告全部快照”。已完成最小首包；当前入口为 [F2验收报告](research/f2-acceptance-20261113.md) 与 [可执行spec](../../spec/server/report-snapshots.md)。不要继续按旧内部Snapshot响应或“实施确认待返回”的历史草稿执行。

关键交付：人工capture与专用原意图receipt；PublicSnapshot独立重放（含project_reasons）；内部/公共/audit复合seal绑定；来源原生/内容版本与分页cut/EOF；逐指标unknown；exact Reporter/RR/read-fence；report-wide终止withdraw与原receipt恢复。新schema不加FK，独立CONCURRENTLY索引，owning root清理。旧draft无自动回填，源普通删除不撤回报告历史。

验证：handler normal/race各17根/30结果，router各6根/42结果；engine/health38/53、CLI487/1283、audit3/9、middleware7/17、builtin5/25和14项launcher纯测试通过；最终零fail/skip，Go build/vet通过。原handler测试前提失败及builtin行数失败均保留，修正与证据见报告。四个DB运行分别有独立分配/源码/命名结果，末尾仅进行了经builtin合同验证的文档移动。

本任务保持in_progress。M1-S2–S6/F3/F4/F5-A–E与独立真实运行任务不标完成；不新建/归档任务，不调用真实Provider，不操作POLA-1或共享策略，不发布/部署。其他并行collaboration comments、文档及归档变动不在本提交内。

最终限定审计通过：84项F2来源/工具摘要与最终文件一致，M1原批准及设计/验收区段保持原样；观察到的49项外部并行变化单列保留。提交按f2-commit-manifest的明确文件清单，server spec索引仅暂存F2单行，保留其余并行hunk。13份日志采用验证过的gzip副本提交，原始日志留在本地。未跟踪的共享verification-environment规范未被接管或暂存。
