# 核心功能与近期合入代码测试报告

**439项E2E库存均已有通过证据；后续性能专项新增1项失败，完整验收仍未通过。** 迭代结转 P95 为 2,171ms，超过冻结门槛 2,000ms；平台和真实商业 Provider 验收也仍有缺口。

初轮生成时间：2026-11-02T01:51:49.823793+00:00。初轮独立证据复核为 **PASS WITH NOTES**（`research/final-evidence-review.md`），范围仅限当时已执行项目，不覆盖后续性能失败或安装升级补测。

## 版本与环境

- 分支`codex/cross-workspace-collaboration`，产品候选`7af9ab9905c98c77bb091223ebe687c639475bb4`；本地main/origin-main/merge-base`dd87e0b33c1d6a64d488f7ee9ecd0a65984a68ff`。未fetch。整分支446提交，旧822c验收后128提交。
- macOS arm64，Node22.23.2、pnpm10.28.2、Go1.27.0；与CI Go1.26工具链差异保留。Web使用真实production build/start，Chromium与实际Electron分别有运行证据。
- 所有API/browser数据在任务专库；Go包使用新迁移模板的独立克隆库。API/DB/进程提交、Build ID、配置指纹、renderer/preload/CLI和harness补丁哈希可追溯。未复用最初身份不匹配的默认服务。

## 实际结果

| 检查 | 最终已覆盖范围的结果 | 证据 |
| --- | --- | --- |
| lint/typecheck/UI exports与脚本 | 原9段命令均exit0；后续Web类型、views lint、diff-check、11项Node控制器回归与11项macOS必需native checker通过 | `research/static-ts-result.json`；`evidence/final-harness-checks/` |
| 非Mobile TS | **13,082通过 / 1,063文件** | `research/static-ts-result.json` |
| Mobile | **196通过 / 28文件**；typecheck/lint/wrapper通过，lint7warnings | `research/mobile-result.json` |
| 数据种子集成 | **13通过 / 0跳过**，新专库补齐原条件skip | `evidence/seed-integration-1/` |
| 初轮Go build/vet与82包 | 每包均有完整成功历史；73包有具名测试、9包无具名测试；当时逐身份视图顶层**10,365通过 / 18跳过**，子例10,491通过。后续opt-in失败单列 | `research/go-current-summary.json` |
| 带标签知识集成 | **23个顶层用例分别normal/race通过**，共46顶层执行及42子例执行，无失败/跳过；独立于默认Go库存 | `research/knowledge-tagged-summary.json` |
| E2E库存 | **439 / 439唯一身份已有通过证据**，148个spec，无当前未解决的库存FAIL/NOT_RUN | `coverage.csv`；`evidence/e2e-summary.json` |
| 部署开关与Host | 启动知识OFF/ON分别验证真实任务配置→API→受控Host物理输入与授权事件；Host运行/Stop/Resume/小队/切换通过 | `evidence/host-off-1/`；`evidence/knowledge-on-1/` |
| 客户端 | 生产Web、真实Electron、macOS arm64.app完整Main，以及Linux容器内目录包完整Main通过 | `evidence/mac-packaged-main-1/`；`evidence/linux-full-main-4/` |
| 后续性能专项 | **2 PASS / 1 FAIL，exit 1**：迭代结转 P95 2,171ms > 2,000ms；迭代禁用与 12,000 条任务分诊通过。专库结束后连接 0 | `evidence/resume-handler-perf-1/` |
| 其余Go条件补测 | wire导出/公开skill导入2项（race）、真实Linux条件7项全部通过。合并12个原skip后最新顶层**10,376 PASS / 1 FAIL / 6 SKIP**，子例10,491 PASS；非整包重跑 | `research/go-resume-current-summary.json` |
| 真实Go→TypeScript契约 | 本轮导出的7类分诊响应经现有schemas与解析入口全部成功，7 PASS / 0 FAIL；独立于原单元和E2E库存 | `evidence/resume-wire-schema-1/` |
| Linux ARM64 DEB | 旧版→候选版实际dpkg升级、独立新装、移除/重装和purge通过；87条检查通过，合成用户配置/状态保留、CLI路径未覆盖 | `research/resume-linux-deb-acceptance-1.md`；`evidence/resume-linux-deb-install-1/` |

E2E共保存**588条逐例框架记录**：PASS486、FAIL13、原始跳过/NOT_RUN89。包含重跑，不是588个独立业务用例。所有自动重试为0，没有增加断言超时。439逐身份最终状态与原轮结果分别保存。

**原样完整E2E轮仍是349PASS/1FAIL/89SKIP、exit1；原Go82包首轮仍是5包失败。** 后续测试适配、前置修正与逐项补跑不会追认这些原轮为成功，也不等于单次`make check`整体全绿。

## 本批：Webhook 错误合同补证与 Mac updater 离线准备

- 本批计划research/readiness-batch-plan-1.md，继续既有任务。完整保存6,234个批前任务文件及36项已有工作区/旧任务文件；三树候选/源码/冻结身份保持。原生研究确认CLI未知字段丢失是明确未实现限制，不新增必须保留的合同、不重复已覆盖的已知字段矩阵，原skip保留。
- 协调者在任务专用admin库分配新库multica_func7af9_next_webhook_lookup_1，空模板/751迁移/连接余量60均核验，未访问默认库。新Webhook私有overlay一次guardedrace编译、一次执行，1顶层+4子例PASS、无skip/重试，见research/webhook-lookup-diagnostic-1.md/.json与evidence/next-webhook-lookup-1。
- 真实规范GetWebhookTriggerByToken及20目的Scan见证：服务端PrepareError→PgError42P01的prepare/解析故障、独立closedpool客户端获取故障均500；真实ErrNoRows404记坏凭据债务；绝对限流429在DB/body之前且RetryAfter60。4例body/Exec/Query/SendBatch0、三表指纹不变、sharedtestPool健康/默认search_path不变，所有独立pool关闭。错误例不证明SELECT执行或成功非空行解码，也不证明Webhook签名/交付/worker/Provider完整验收。
- 本诊断是新身份；原Go10376PASS/1FAIL/6SKIP与439E2E库存不变，未删除或改写原CLI/Webhookskip。QA-PERF-001仍2171ms>2000msFAIL；没有新的真实尾延迟症状，未重跑P95或统计/cache夹具及已通过子门。
- Mac updater离线准备5条命令通过，原生成latest-mac.yml字节、固定ZIP/DMG SHA512/size/SHA256、当前SDK6.8.3 parser/reference/channel/ZIP选择、两固定ASAR版本/IPC均核验。已生成manifest/feed/配置模板/未来步骤；placeholder未解析，无应用/updater/feed网络/账号操作，实际MAC-AUTOMATIC-UPDATER仍NOT_RUN。
- 新验证边界：manual check不受automaticUpdates=false阻断；下载缓存按Node HOME计算，须同时限定进程HOME与Electron接受测试home/userData；Squirrel动态代理/原生准备、安装和真实重启是分别取证的阶段，下载事件不替代安装/重启。已用trellis-update-spec追加两份验证规范并完整保留原内容；不改产品/SQL/索引/依赖/维护策略。
- 协调者复核已完成，独立代码/原始证据审阅已PASS WITH NOTES（research/readiness-code-evidence-review-1.md/.json），最终文档审阅已PASS WITH NOTES（research/readiness-final-coordination-review-1.md/.json）；最新本批清理evidence/next-readiness-cleanup-1/run.json：57个已登记任务库均连接0，包含新Webhook专库；六记录端口关闭，自有进程/容器/挂载/协议新增注册0，原精确默认路径及注册集合与批前相同。只读清理审计不采集计划/统计或执行测试。最终独立结论已分别记录；31行条件矩阵保持21PASS/1FAIL/6BLOCKED/3NOT_RUN，必需门开放，in_progress/noarchive/no commit。

## 上批：剩余门条件、身份与清理续验

- 已恢复当前会话指向既有任务，不新建任务。本批计划为 research/remaining-gates-batch-plan-1.md，原生平台执行切片与身份检查切片均完成；协调者复核分别落盘，最终独立文档/清理审阅已完成PASS WITH NOTES（research/remaining-final-coordination-review-1.md/.json），无阻断性证据问题。
- 三个工作树候选/源码及冻结测试身份均匹配。已核对6,113个批前任务文件、5,949个更早历史文件和36项已有工作区/旧任务文件；158次已审阅原始哈希比较（154个唯一构件）均匹配，439项库存不变。已有Next生成导入差异严格字节对照通过；初始私有检查器误分类失败与修正补充均保留，不计产品失败。
- 未发现新的真实症状环境证据；新平台预检、私有身份检查及资源审计均非延迟/计划症状。未再次执行缓存/统计夹具或P95，没有当前统计倒推历史状态。QA-PERF-001仍2171ms>2000ms FAIL。
- 按remaining-platform-handoff-1.md完成10条只读命令及4份固定分发构件身份核验；本地未出现真实Linux/Windows runner、完整Xcode/simctl或正式签名构件。真实认证迁移、自动更新和商业Provider条件仍缺，没有新增可执行门。31行矩阵保持21 PASS / 1 FAIL / 6 BLOCKED / 3 NOT_RUN。
- 最新清理 evidence/next-remaining-cleanup-1/run.json：56个已登记任务库均连接0（含上批9专库及浏览器/知识库），六记录端口关闭，自有进程/容器/挂载/协议注册残留0；原multica://精确默认路径及注册集合相同。数据库连接仅作只读清理审计，不采集统计/计划或执行测试。
- 子代理平台lookup错引用run.json已校正为build.json并保留拒绝观察；原生context显示截断后身份子代理完整读取落盘注入。当前两执行切片均无429或空消息观察，历史429/空消息记录保持。
- 本批仅新增任务/私有取证与更新协调文档；全量lint/typecheck、已通过439 E2E、ARM64 DEB和Mac子门不重跑。JSON/CSV/AST/哈希/diff检查及协调者复核通过。无产品SQL/索引/依赖/维护策略修改，无账号操作、商业调用、远程派发、提交或发布；必需门开放，任务保持in_progress。

## 已完成缓存计划与读取语义结果

新run1实际6,000计划及run2实际8,000计划通过backend-only auto_explain捕获，全部规范SQL/原参数、prepared计数/身份与相同后端已核验。无统计原流程、缓存复放、清缓存重prepare均重复扫描事件并排序；任务内ANALYZE后，现有sequence索引反向扫描每条取1事件，无Sort。run2源/目标notice计时合计原流程710.095/234.867ms、新prepare667.284/236.153ms、ANALYZE后16.908/11.184ms；这些插桩时间不是原P95或可相加的端点收益。没有缺索引证明，也未重建原失败尾样本。两个新库完整回滚分别3次/4次均保持被覆盖业务指纹，物理WAL/版本/缓存及部分统计元数据仍可能变化。

读取语义专项race为handler4顶层+6子例、service8顶层+8子例PASS。新增协议证据使用生成typedScan，验证非空/空/ErrNoRows/既有target、首次实际锁等待及RC后续语句新快照、服务端22012/Close/回滚；现有真实handler撤权、serviceCAS/最终快照失败/任务保留等回归通过。它们支持未来最小完整集准备/结果消费边界的回归基线，不能证明尚未实现的优化、typeddecode/延迟Rows.Err故障或全部调度等价。

详见research/perf-cached-plan-diagnosis-1.md、perf-cached-reprepare-diagnosis-1.md、read-semantics-diagnosis-1.md；协调者复核在evidence/next-cached-session-1/。两原生执行子代理成功；run2补充通知空载荷后协调者接手，协调者429观察保留；独立审阅research/cached-semantics-evidence-review-1.md/.json为PASS WITH NOTES，原始诊断及最终文档/清理补充无阻断证据缺陷；另有research/cached-final-coordination-review-1.md/.json最终文档独立审阅PASS WITH NOTES，不以自检代替。最新清理next-cached-cleanup-2验证9个专库连接0、六旧端口关闭、自有进程/挂载/容器/协议注册0及原协议精确路径/集合不变。平台next-platform-preflight-2条件未改变。未改产品SQL/索引/依赖，未重跑原P95及已通过验收门，无提交、发布、远程派发或商业调用。

## 上批 SQL 计划与传输诊断

一次规范SQL诊断PASS，另有1项DB-free规范批SQL等价测试PASS；均单列于诊断证据，不改变原Go/E2E库存或原P95FAIL。三类批写各1,000条在原事务阶段EXPLAIN并回滚，源leave/target join/issue CAS的计划执行时间合计651.403/221.459/85.479ms。源/目标最后事件查询各累计bitmap扫描3,003,000/1,001,000条事件并排序；CAS通过workspace+id索引匹配，WAL2,712,509bytes。没有逐索引成本证明或产品修复。

在完整锁已持有的回滚夹具中，相同3,000条规范读串行641.410/746.562ms，流水线29.665/36.465ms；全部逐条结果摘要相同。该对照支持读取传输为后续优化研究方向，但不覆盖首次锁、非空任务/已存在target、并发与fresh snapshot或typed Scan/no-row错误处理。EXPLAIN是独立语句，四表reltuples=-1且回滚不消除WAL/物理版本/缓存影响；下一步先区分统计与缓存计划，不直接新增索引，也不把两个诊断的耗时相减当预期收益。见research/perf-sql-plan-diagnosis-1.md。

Trellis执行子代理完成工具工作后429，初次复核子代理亦429，协调者保留原始输出并接手报告。独立续审已完成PASS WITH NOTES（research/sql-evidence-review-1.md/.json），6000份计划/参数/哈希与5900历史文件经独立核验，无阻断诊断证据缺陷；协调者与独立结论分别保留。本机平台条件只读检查完成，仍按research/remaining-platform-handoff-1.md交接，无远程/商业调用。evidence/next-sql-cleanup-1/run.json确认5个新专库连接0、六旧端口关闭，无自有进程/容器/挂载/协议注册残留，原协议默认路径和集合不变。产品和冻结测试哈希未变，完整lint/typecheck/439项未重复。

## 后续推进计划的执行结果

- 一次受控真实1,000条结转2,396.832ms：七批写入1,279.292ms、3,000次串行读731.413ms；单次归因诊断不计P95通过或Go原库存通过。源码和冻结用例未变，诊断专库连接0。见 research/perf-phase-probe-1.md。
- 当前PostgreSQL已核实是amd64镜像运行于aarch64引擎；此环境差异和一次归因都不能重建原两条尾延迟样本。本批已检查源leave/issue CAS/target join的规范SQL计划和已锁读取传输；后续先区分统计/缓存计划，原P95 FAIL保留，没有产品修复。
- macOS固定旧版ZIP→候选ZIP文件安装替换、完整文件清单和5个合成文件保留通过（24条断言、16条命令）。后续两版真实Main在同一私有安装路径启动，真实preload配置/偏好、隔离home/userData及localStorage保留通过，协议新增注册已清理且默认处理器精确路径与原值相同。见 research/mac-runtime-upgrade-1.md。
- macOS真实登录/凭据迁移、自动更新安装和正式签名门仍开放；旧版dirty构件只对固定字节作结论。平台恢复命令和构件限制见 research/remaining-platform-handoff-1.md。
- 本批已派发实际研究/执行及独立证据复核；原续验429记录保留。新审阅入口 research/next-evidence-review-1.md，清理入口 evidence/next-cleanup-2/run.json。

## 已实际覆盖的核心功能和近期修复

身份/验证码/密码/设备登录、退出/会话与首次改密；工作空间创建切换、邀请与成员角色、撤权和越权拒绝；任务创建编辑删除、状态/优先级回滚、评论/附件、幂等与重复发送；项目、进度、验收与迭代范围/历史；分诊、搜索、导航、聊天、skills/MCP和资源发布、自动化及平台管理。

跨账号协作覆盖创建、精确收件人、Transfer、补充、交付、验收、Leave、离线通知、深链与多会话。近期重点覆盖memory启动版本与live版本、并发编辑/恢复/清除、原键恢复、嵌套runtime清理后同库Leave、原生窗口焦点/弹出/重启、知识审批纠错/撤回/来源清除和物理输入、部署开关两态、实际两Linux namespace执行及Host小队/切换/停止恢复。

产品领域→测试入口/预期结果见`test-plan.md`；逐例标题→最终结果→原始run见`coverage.csv`。API集成、浏览器故障注入、合成历史与实际进程证据各自保留，不以测试文件存在推定行为通过。

## 发现、修复及简化

本轮修改均限于测试和验证规范，没有业务代码或新依赖：

| 文件 | 改动与目的 |
| --- | --- |
| `e2e/collaboration-memory-web.spec.ts` | 显式展开实际收起的历史，并在展开后再次验证关闭态不可Restore；清除与权限断言保留 |
| `server/internal/handler/daemon_test.go` | 五个预期在线fixture领取前显式建立当前心跳，保留生产150秒门槛、stale/offline负例和原断言 |
| `e2e/collaboration-installed-main.spec.ts` | 用现有精确UUID picker helper替代遗漏的旧输入交互；保留表单选中与全部工作流断言 |
| `e2e/fixtures/collaboration-installed-main.mjs`、`collaboration-recipient-picker.mjs` | plain Node packaged控制器使用等价小型helper，保持真实Main/IPC/认证与业务回执校验 |
| 两份测试规范 | 记录进程环境隔离、在线fixture前置、容器依赖解析和harness指纹要求 |

三组测试修复均经过独立审阅及相关完整流程复验。简化是复用既有收件人操作、在所属测试显式建立前置，未新增产品兼容层或重构业务。

此外修正了测试进程pool/HOME覆盖、条件分道门、错误fake Provider、容器模块链接/临时执行挂载和知识专项专库前缀。密码限流仍真实启用，三份run受到注册429影响后保留首败并等待自然窗口复验；没有重置服务来清空额度。详见`defects.md`。

历史因果边界：memory收起的具体触发未直接建立；旧handler失败时runtime行已清理，后续107.824秒日志不能重建原状态。本轮通过证明最终测试前置和交互可用，不将这些局限包装为已查明的产品根因。

## 尚未关闭的完整验收范围

- **性能门 FAIL**：`TestIterationI1P95` 的结转 P95 超标 171ms（8.55%）。保留原测量与失败，原尾样本根因仍未知；新夹具已建立统计缺失/重复扫描机制，不以该诊断或共享负载推测改判通过。原 82 包的成功历史不代表此 opt-in 用例通过。
- **Windows原生与安装升级、iOS实际UI**：当前没有Windows runner；本机缺完整Xcode/simctl。Mobile单元成功不能替代iOS界面或跨端语义验收。
- **Linux原生桌面会话、其他安装格式/架构及升级/回滚组合、签名/公证**：Linux ARM64 DEB已在隔离Debian12容器完成真实新装/升级/移除重装；该结果不覆盖窗口管理器、原生焦点/托盘、登录/IPC业务流程、自动更新或降级。macOS已实际完成旧版→候选Main启动、正常密码表单以及合成配置/localStorage保留；真实登录/凭据迁移、自动更新安装和正式签名仍开放。
- **真实商业Provider、不同账号、模型理解/质量/额度**：未获本轮账号/模型/调用上限授权，实际商业调用0。受控fake进程只证明输入、权限和执行链路。
- **Go剩余6个顶层skip**：pg_bigm缺失、两个Windows行为、CLI未知字段与Webhook数据库错误的未实现断言、当前目录无HTTP MCP recipe；见`research/go-resume-current-summary.json`。原18个skip中已实际执行12个，11 PASS / 1 FAIL；历史计数与原始日志不变。
- **版本组合限制**：包构建来源639bf6ef8，server/apps/packages三棵Git树与本候选完全相同、实际ASAR/CLI/主程序hash匹配，所以允许本次新运行；未复用旧结果。旧私有adapter-v5/RKI6 launcher仍未使用，不关闭其原审阅门或挪用付费额度。

这些不阻止交付本轮测试结果，但完整发布验收保持未完成。平台/配置轴见`conditional-matrix.csv`，后续会话按该表逐项补证。较早续验批的两个子代理工具工作前429及协调者复核记录保留；本次计划批已有独立审阅PASS WITH NOTES，首次Mac合成配置问题已在新编号运行修正并补充可见性断言，复核入口research/next-evidence-review-1.md。

## 清理、证据和交接

已核对9个browser环境数据库的执行门：enabled/host准入均关闭、creation恢复初值，epoch不倒退；password人为准备的namespace门已恢复。自有API/Web/renderer、两个HTTP fake Provider和独立Redis均已停止，18920/13840/19860/16390无监听，未发现自有Host/packaged进程残留，未停止默认或其他任务服务。证据`evidence/cleanup/run.json`。

保留数据库、隔离工作树、构件和原始日志用于复现。browser registry环境配置了24小时TTL，后续GC可能回收其数据库；Go显式专库没有registry TTL。不要把停止进程等同于永久保存环境。

任务材料：`test-plan.md`、`implement.md`、`coverage.csv`、`defects.md`、`conditional-matrix.csv`、`progress.md`、`session-prompt.md`。原始命令、退出码、JSON、截图和trace在`evidence/`，私有配置与runner在忽略目录`.omx/private/branch-functional-regression-20261102/`。所有续验已结束，4个新专库连接和两轮Mac实际运行进程/端口/协议清理见 evidence/next-cleanup-2/run.json；先前清理仍见 evidence/resume-cleanup-1/run.json；旧证据与439库存指纹保持不变。此次只新增任务材料和隔离验证runner，没有产品/依赖改动，未重复运行全量lint/typecheck。

Trellis任务保留in_progress用于剩余平台/真实账号门交接，不归档成“完整验收通过”。测试/规范改动尚未提交或推送。
