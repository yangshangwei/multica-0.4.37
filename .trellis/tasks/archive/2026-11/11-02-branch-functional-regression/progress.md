# 测试进度

初轮本地测试与清理已完成（2026-11-02T01:53:18.361489+00:00）；后续条件补测继续推进。任务保持 `in_progress`，当前存在性能失败，不能按完整验收通过交付。

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

## 上批：实际 prepared 计划、统计对照及读取语义

- 固定候选7af9ab990不变，沿用既有任务。保存5,949个历史任务文件及全部已有改动；未重复439项E2E、DEB或macOS已通过子门，冻结P95未重跑，QA-PERF-001仍2171ms>2000ms FAIL。
- 原生子代理新库run1实际捕获6,000条规范SendBatch计划/参数，原流程与无统计cache-control均重复bitmap扫描/排序；明确任务内ANALYZE后，同prepared身份通过现有sequence索引Backward每条取1事件。单次诊断PASS，三次完整业务事务均回滚。见research/perf-cached-plan-diagnosis-1.md。
- 为分离旧缓存与统计影响，协调者在新库run2补充无统计DeallocateAll/reprepare控制，实际8,000条计划/参数均核验：新prepare仍扫描源3,003,000/目标1,001,000条并排序；ANALYZE后各1,000条、无Sort。prepared时间/计数重置和相同PID7356已记录；四次完整回滚一致。证明本夹具中旧缓存身份本身不足以解释该路径，不证明原P95尾样本根因、单列统计作用或缺索引。见research/perf-cached-reprepare-diagnosis-1.md。
- 读取语义与现有产品race回归：handler4顶层+6子例、service8顶层+8子例全部PASS；规范SQL复用生成typedScan，覆盖非空/空/缺行/既有target、首次真实Lock等待及RC后续语句新快照、服务端错误和Close/回滚，并执行真实撤权/CAS/快照失败/任务保留等现有回归。小型协议证据不等于未来完整优化实现等价，typeddecode/延迟Rows.Err及实际调度仍需随实现验证。见research/read-semantics-diagnosis-1.md。
- 子代理执行成功；协调者曾429（精确观察保留），中间消息空载荷不归为子代理执行失败。两次run2补充通知被子代理报告为空，协调者按已写计划接手；执行、自检、协调者复核及独立审阅分别记录，独立审阅及最终文档/清理补充已PASS WITH NOTES（research/cached-semantics-evidence-review-1.md/.json），无阻断性证据缺陷。最终独立文档审阅research/cached-final-coordination-review-1.md/.json也PASS WITH NOTES。派发入口evidence/next-cached-session-1/agent-dispatch.json。
- 最新清理evidence/next-cached-cleanup-2/run.json：本批4个新库及前5个专库连接均0，六旧端口关闭，无自有进程/容器/挂载/协议注册残留，multica://精确默认及同bundle集合不变。平台只读预检next-platform-preflight-2未发现新增真实执行条件，剩余门不改判；商业调用/远程派发均0。

## 上批：规范 SQL 计划和串行读取诊断

- 继续既有任务与授权，HEAD仍为固定候选7af9ab990；未新建任务。新专库multica_func7af9_next_sql_diag_1，单次TestIterationCanonicalSQLPlanDiagnostic通过；未执行冻结P95或重复439项/DEB/macOS升级。
- 三类规范批写各1,000条在原事务阶段进行EXPLAIN ANALYZE/BUFFERS/WAL并回滚。源leave执行时间合计651.403ms，target join221.459ms，issue CAS85.479ms；这些是插桩计划时间，不能直接当原批耗时。源/目标的最后事件子查询分别累计bitmap扫描3,003,000/1,001,000条事件再排序；CAS命中(workspace_id,id)索引，1,000次产生2,712,509字节WAL，未按单索引归因。
- 完整锁已持有处的相同3,000条读取ABBA对照：串行641.410/746.562ms，流水线29.665/36.465ms，全部逐条结果摘要与回滚业务指纹一致。支持继续研究读取传输方向；不证明首次加锁、非空任务、并发/新快照、typed Scan/no-row错误语义等价。
- 四表reltuples=-1、EXPLAIN文本与生产缓存语句不同、回滚保留物理WAL/版本/缓存影响。下一步先诊断同规范SQL的统计与缓存计划，再决定是否需要索引；不自动新增DDL或产品修复。原QA-PERF-001仍2171ms>2000ms FAIL。
- 现有DB-free规范SQL等价测试新跑1项PASS，不计原Go库存。结果入口research/perf-sql-plan-diagnosis-1.md、evidence/next-perf-sql-plan-1/，子代理429及协调者接手记录evidence/next-sql-session-1/agent-dispatch.json；独立续审已完成PASS WITH NOTES（research/sql-evidence-review-1.md/.json），全部6000计划和5900历史文件经独立核验；协调者分析与独立审阅分别保留。
- 只读平台条件预检完成，剩余门状态不变；无远程派发/商业调用。清理evidence/next-sql-cleanup-1/run.json：本批及前4个新专库连接均0，六个旧端口关闭，无自有进程/容器/挂载/协议注册残留，原默认路径及注册集合不变。

## 按推进计划继续执行的结果

上述统计/缓存及读取语义对照已完成。后续先读取本批根因边界/独立审阅及最新清理；统计缺失、缓存复用与重新准备已区分，原尾样本归因仍未知。最小可能产品范围和实现回归标准已写明，但当前不自动进入产品修复，也不重复诊断到绿。剩余平台按remaining-platform-handoff-1.md的真实条件推进，每批完成独立复核、记录更新及自有资源清理。完整新会话入口为session-prompt.md；本次交接整理未执行新测试。

- 计划已写入 .omx/plans/branch-functional-regression-next-20261102.md，并按顺序执行，不新建任务或重复审批。
- 性能已完成代码路径分析和一次受控分段诊断：真实1,000条结转2,396.832ms，七阶段/7,000条批写1,279.292ms，3,000次串行活跃任务/参与读取731.413ms；最慢源关系移除批623.604ms，连接池等待不显著。原P95 FAIL保留，诊断不计新通过身份。见 research/perf-code-diagnosis-1.md、perf-phase-probe-1.md。
- 已核实当前PostgreSQL是amd64镜像、Docker引擎是aarch64，与历史ARM64基线不同；该差异不直接证明超标原因。新诊断专库连接0，冻结测试和产品源码未变。
- macOS旧版→候选版文件安装替换和合成文件保留通过（24条断言）；随后实际运行两版immutable Main，真实home/userData、preload配置/偏好、daemon停止状态、配置文件与localStorage升级保留均通过。multica://默认路径仍为用户原安装，任务新增注册已注销。见 research/mac-runtime-upgrade-1.md。
- macOS真实登录/凭据迁移、自动更新安装、正式签名/公证仍开放；旧版dirty源码差异未重建，只对固定ZIP字节作结论。
- Linux原生桌面/Windows/iOS/签名/真实Provider已有准确执行入口与所缺条件：research/remaining-platform-handoff-1.md。未远程派发、未访问商业账号。
- 本批独立复核已完成PASS WITH NOTES，具体建议已处理：Windows基线条件说明修正；Mac首轮合成认证配置错误保留，新编号run2已增加两版实际密码表单可见性断言并通过。复核入口 research/next-evidence-review-1.md（含runtime2补充），清理见 evidence/next-cleanup-2/run.json：4个新库连接0、两轮Mac端口均关闭、无自有进程/协议注册残留。没有产品优化、依赖变更、提交或发布。

## 续验进展（2026-11-02）

- 性能专项已结束：`TestIterationI1P95` FAIL、`TestIterationI1DisableP95` PASS、`TestTriageQueueScaleBaseline` PASS，命令 exit 1，未重试或放宽阈值。
- 迭代结转 P95 为 **2,171ms > 2,000ms**，超出 171ms（8.55%）。保持失败；共享主机/数据库资源属于测量局限，尚未确认产品根因。
- 迭代禁用与 12,000 条任务（10,000 正式 + 2,000 待分诊）测试通过；该专项专库 `multica_func7af9_resume_perf_1` 结束后残留连接 0。原始结果见 `evidence/resume-handler-perf-1/`。
- 候选 ARM64 DEB 已构建并核验：`0.6.8-rc.20261102`，SHA-256 `44f3a4c5f74e91205bea4d398a81ed737efa49d9062a9f4bc4c2234c588fb01a`。旧版 `0.6.5~179~g1bfd169dc` → 新版 `0.6.8~rc.20261102` 的真实 `dpkg -i` 升级通过；独立新装、移除/重装、purge 通过，共 87 条检查通过。载荷、桌面入口、图标、协议声明、CLI 路径及合成配置/状态保留已核对。见 `research/resume-linux-deb-acceptance-1.md` 和 `evidence/resume-linux-deb-install-1/`。
- 续验计划中的 Go wire 导出/公开 skill 导入 2 项（race）和真实 Linux 条件 7 项全部通过；真实 Go 输出的 7 类响应经当前 TypeScript schemas 全部解析成功。分别见 `evidence/resume-handler-wire-live-1/`、`resume-linux-conditional-1/`、`resume-wire-schema-1/`。
- 本轮共补齐 12 个原跳过的 Go 身份，11 PASS / 1 FAIL；最新逐身份视图 **10,376 PASS / 1 FAIL / 6 SKIP**，子例 10,491 PASS。原 82 包成功历史与失败轮均保留，不冒充整包重跑或一次全命令成功。见 `research/go-resume-current-summary.json`。
- 原生 Linux 桌面会话、Windows、iOS、签名/公证和真实商业 Provider 仍需各自验收。容器包管理器通过也不能替代这些门。
- 原有 439 项 E2E 证据与初轮 Go 整包成功记录保留，不重复执行；本轮新性能 FAIL 单列，不能被旧整包通过覆盖。
- 两个 Trellis 子代理在开始工具工作前遇到 429，主会话接手；本轮复核属于协调者复核，不声称独立审阅。所有续验已结束；3 个新专库连接及新建容器/进程清理见 `evidence/resume-cleanup-1/run.json`。无产品改动或新依赖，未提交/发布。

## 初轮已完成记录

- 候选7af9ab9905c98c77bb091223ebe687c639475bb4，分支codex/cross-workspace-collaboration；本地main基线dd87e0b33c1d6a64d488f7ee9ecd0a65984a68ff。
- E2E439唯一身份全部有当前PASS证据，148spec。588条逐例记录含486PASS/13FAIL/89原始SKIP；原完整轮349/1/89、exit1保持原判。
- 非MobileTS13,082/1,063文件通过；Mobile196/28文件及类型/lint/wrapper通过、7warnings；seed专库13全过；静态/脚本和最终harness复核通过。
- Go82包均有成功完整包结果（各包最新派生）；含Redis/Linux补测顶层10,365PASS/18SKIP，子例10,491PASS。原5包失败保留，不能说一次全命令全绿。
- knowledgeintegration额外23顶层分别normal/race全部通过，无skip；数据库前缀setup失败完整保留。
- 当前生产Web/真实Electron、两Linuxnamespace、Host运行/小队/切换、知识部署OFFON实际输入、Mac.app完整Main、Linux容器目录Main均已实际通过。
- 独立审阅：规划、memory交互、handler在线fixture、installed-Main picker均PASS WITH NOTES，建议已应用。最终证据复核PASS WITH NOTES、无本地证据阻断，唯一文字计数建议已修正；见research/final-evidence-review.md。
- 只有测试/规范/任务材料改动，无产品代码或依赖；未提交/推送。完整清单见report.md。
- 清理：9个browser专库gate已核对/恢复，epoch不倒退；自有API/Web/renderer/HTTPfake Providers/Redis已停止，18920/13840/19860/16390无监听，无自有Host/packaged进程残留。旧默认未知服务未动；数据库/构件/日志保留，browser环境受24hTTL影响。
- 剩余完整发布验收：Windows原生/安装升级、iOS实际UI（缺simctl）、Linux安装器/升级及原生桌面、签名/公证、真实商业Provider/不同账号/模型质量；商业调用0。
- Go18skip逐项清单在research/go-current-summary.json；性能/外网/wire/pg_bigm、平台和已知未实现断言不记PASS。
- 新会话入口：session-prompt.md；不要重复本轮439测试、不要启动旧private launcher或挪用旧额度；按conditional-matrix.csv剩余轴补证。

## 新会话交接整理

- 当前进度、已完成诊断、剩余门和下一会话执行顺序汇总至research/session-handoff-1.md，完整引导语仍为session-prompt.md。
- 此次仅核对Git及已有最终审阅/哈希并更新交接文档；未执行新测试、数据库/应用操作或新的清理。上批清理9库连接0是其时点证据，新运行须重新核验。
- 固定候选、原2171msFAIL、31行矩阵状态与in_progress均保持；不新建任务、不重复要求已批准测试授权、不自动进入产品修复。
