# 本轮问题与条件缺口

候选：`7af9ab9905c98c77bb091223ebe687c639475bb4`。产品、测试环境与平台前置分别记录。原始失败与复验分别保留，最新逐例结果见`coverage.csv`。

## QA-PERF-001：迭代结转 P95 超过冻结门槛（未解决）

- 状态：FAIL；性能门失败，尚未定位为具体产品缺陷。
- `TestIterationI1P95` 实测结转 P95 **2,171ms**，门槛 **2,000ms**，超出 171ms（8.55%）。专项命令 exit 1，其余两项禁用 P95、12,000 条任务分诊测试通过。
- 执行于固定候选的独立数据库，性能测量未启用 race、自动重试 0，没有调高门槛或修改断言。数据库使用共享 PostgreSQL 与主机资源；该局限不改变 FAIL 判定，也不证明产品根因。
- 原始 Go JSON、查询计划、样本、环境身份、命令与 SHA-256 均保留；专项完成后专库残留连接 0。
- 后续：在独立诊断/修复范围中分析已有样本、查询计划与结转调用路径；本测试任务不顺手修改业务代码，也不为获得绿色反复重跑。
- 证据：`evidence/resume-handler-perf-1/run.json`、`stdout.jsonl`、`triage-perf.json`、`database-operations.jsonl`。

### QA-PERF-001 诊断进展

一次真实1,000条结转2,396.832ms；七阶段批写1,279.292ms，3,000次串行读取731.413ms，源leave批623.604ms，pool获取与提交成本很小。当前PG为amd64镜像/ARM64引擎；不能将环境差异或单次诊断当作原P95根因或修复证据。保持FAIL，优先分析批写SQL计划与串行读边界。见 research/perf-phase-probe-1.md、evidence/next-perf-environment-1/run.json。

### QA-PERF-001 规范 SQL 与传输诊断（本批）

- 新专库/新文件overlay单次诊断PASS。源leave/target join各1,000条计划累计扫描3,003,000/1,001,000条事件并排序；issue CAS经(workspace_id,id)索引命中，计划执行合计85.479ms、WAL2,712,509bytes；22个issue索引的各自成本未测，不能认定新增索引或删索引为修复。
- 三类写计划执行合计分别651.403/221.459/85.479ms；EXPLAIN独立文本/规划时间/物理回滚影响与原1279ms批写不直接相减。四表reltuples=-1；下一诊断应区分统计/缓存计划与索引不足。
- 相同3,000规范读在已锁完整集处ABBA：串行641.410/746.562ms、pipeline29.665/36.465ms，逐条结果摘要一致。是受控传输方向证据，非产品替换；目标缺行、活跃任务为空、锁已持有，未证明并发/新快照/no-row控制流等价。
- 原冻结P95未重跑，仍2171ms>2000ms FAIL；无产品、SQL、索引、依赖改动。完整结论和局限：research/perf-sql-plan-diagnosis-1.md。协调者复核与子代理429记录另存；所有原失败和439项证据不变。

### QA-PERF-001 实际缓存/统计与语义诊断（最新）

- 实际规范preparedSendBatch计划run1共6,000、run2共8,000；无统计原流程/复用/DeallocateAll重新准备都重复bitmap扫描排序。新prepare源/目标仍3,003,000/1,001,000事件；ANALYZE后同新prepared身份用现有索引Backward各1,000条且无Sort。支持本夹具统计/重规划机制，旧缓存身份本身不足以解释，不证明原尾样本或缺索引；不得直接改DDL/产品SQL或嵌入ANALYZE。
- 新handler4+6/service8+8 race全部通过；typed非空/空/缺行/既有target、首次锁等待/RC新快照、错误Close/回滚与当前产品撤权/CAS/快照失败/任务保留证据增强。仍无未来优化实现，typeddecode/延迟Rows.Err/实际完整调度须随实现验证。
- 原2171ms>2000msFAIL和冻结标准保持；所有新增均诊断单列，不增加原Go/E2E库存。根因/最小未来范围/回归标准见research/perf-cached-reprepare-diagnosis-1.md与read-semantics-diagnosis-1.md。协调者与独立复核分别记录；清理next-cached-cleanup-2连接9库均0。

## QA-ACCEPTANCE-MAC-001：升级运行时子门已补齐，认证/更新/信任门仍开放

- 初次macOS文件验收因应用启动会注册宿主机协议而未启动，原BLOCKED记录保留。
- 后续记录当前精确默认处理器、限定任务副本的新增注册并清理，已实际运行旧版和新版Main；隔离home/userData、真实preload配置/偏好和合成本地存储升级保留通过。系统默认处理器精确路径保持原值，任务注册已注销。
- 这只解决运行时隔离前置和对应升级子门；旧版dirty源码差异未重建，真实认证/凭据迁移、自动更新和Developer ID/公证仍未验收。
- 证据：research/mac-runtime-upgrade-1.md、evidence/next-mac-runtime-1/run.json；剩余平台入口 research/remaining-platform-handoff-1.md。

## QA-TEST-MAC-001：合成认证配置缺少能力字段（新运行已复验）

- 首轮next-mac-runtime-1合成服务声明auth_mode=password却未返回password_auth_available=true；当前schema缺省false，登录页正确拒绝该组合。两张原截图均显示认证配置错误。
- 首轮Main版本/home/userData、preload配置偏好和localStorage保留断言仍有证据；该轮不计正常密码界面、真实登录或业务升级成功。
- next-mac-runtime-2只修正任务私有合成API响应，并增加两版真实用户名/密码输入和整个密码表单的可见性断言；全部通过。没有改产品、断言阈值或增加自动重试，首轮结果完整保留。
- 最新证据：evidence/next-mac-runtime-2/run.json及baseline.png、candidate.png；复核见research/next-evidence-review-1.md。真实登录/凭据迁移与自动更新仍开放。

## QA-ENV-001：测试进程的连接池覆盖干扰默认值用例（已复验）

- 分类：本轮协调环境设置；非已证实产品缺陷。
- 首败：`cmd/server` 的 `TestPoolSizing_DefaultsWhenNothingSet`、`TestPoolSizing_URLParamsHonoredWhenEnvUnset`、`TestPoolSizing_PartialURLParam`，实际 max=4 与预期默认值或 URL 值不符。
- 原因：协调者将 API 连接预算思路应用到了 Go 测试进程，注入 `DATABASE_MAX_CONNS=4` / `DATABASE_MIN_CONNS=0`。`server/cmd/server/dbstats_test.go:25` 仅在传入非空参数时写 env，空参数用例继承了这些覆盖。
- 处置：不改产品/测试断言。保留首败；新专库、移除这两个测试进程覆盖值后，专项7项通过；另一个新专库整包366 PASS / 7条件SKIP / 0 FAIL。运行API仍使用max10/min1。
- 证据：`evidence/go/06-cmd_server/`、`evidence/go-cmd-server-replay/targeted/`、`evidence/go-cmd-server-replay/full/`。

## QA-ENV-002：全局任务路径覆盖干扰 HOME 隔离用例（已复验）

- 分类：本轮协调环境设置；非已证实产品缺陷。
- 首败：CLI4项、daemon7项、execenv4项，涉及历史配置、daemon身份、Reasonix/DSH/Hermes路径与清理。
- 原因：协调者注入 `MULTICA_TASK_CONFIG_ROOT` / `MULTICA_TASK_WORKSPACES_ROOT`，优先于用例 `t.Setenv("HOME", t.TempDir())`。例如 `server/internal/cli/config_test.go:16`、`server/internal/daemon/identity_test.go:14`；Hermes实际路径仍落入任务全局go-config，精确事件见 `research/execenv-first-failure.json`。
- 处置：不改源码/断言。每包改用独立私有HOME并显式复用原Go缓存，移除路径和pool全局覆盖；原env全部保留。仅更改尚未开始的permit，实际run记录各自env SHA。已开始的4包在独立新运行中复验。
- 结果：CLI整包75 PASS / 1 SKIP；daemon整包897 PASS / 2 SKIP；execenv整包660 PASS / 0 SKIP；均exit0。初始失败仍保留，不能把首轮宣称为全绿。
- 证据：`research/go-environment-correction-v2.json`、`evidence/go/17-internal_cli/`、`evidence/go/20-internal_daemon/`、`evidence/go/22-internal_daemon_execenv/`、`evidence/go-home-replay/`。

## QA-BLOCKED-001：iOS 实际运行条件缺失

- 分类：平台前置；状态BLOCKED。
- 事实：本机开发目录为 `/Library/Developer/CommandLineTools`，`xcrun simctl`退出72，标准目录未找到完整Xcode。
- 已完成：Mobile类型/lint、196项单元、3类wrapper回归。未启动模拟器、构建iOS app或访问真实设备。
- 缺口：实际登录、导航、工作空间切换、通知计数、生命周期和跨端语义一致性。
- 证据：`research/mobile-result.json`、`evidence/mobile/ios-prerequisites.json`。

## QA-TEST-001：关闭后历史清除用例等待折叠内容（复验通过）

- 原样完整轮与单例均在等待`Purge v2`超时。截图/ARIA显示版本历史收起，具体触发未直接建立。
- 仅增加显式展开历史，并在历史已展开时再次断言关闭任务没有Restore操作；所有清除写、理由校验、版本/权限/窄屏断言保留。
- 独立审阅PASS WITH NOTES；关联10项和最终加强断言的2项均通过。原两次失败不追认为通过。
- 证据：`legacy-full-1`、`legacy-memory-replay-1`、`legacy-memory-harness-1/2`及`research/memory-harness-review.md`。

## QA-TEST-002：handler预期在线fixture未显式刷新心跳（整包复验通过）

- 原handler整包5个连续claim用例返回空task；另一新库原样5项单独执行全部通过。
- 五个用例均复用TestMain一次性初始化的runtime，而生产claim要求最近150秒心跳。修复只为这些预期在线fixture建立当前心跳，不改生产阈值/授权或stale/offline负例。
- 独立审阅后新库整包2863 PASS / 0 FAIL / 7 SKIP。根分支已同步仅测试改动。
- 历史因果局限：旧runtime行已被TestMain清理；新日志刷新前年龄107.824秒不能证明旧失败时的准确状态，也不能用包含编译的runner起始时间推算fixture年龄。
- 证据：`evidence/go/33-internal_handler`、`handler-claim-replay-1`、`handler-full-freshfixture-1`、`research/handler-heartbeat-review.md`。

## QA-TEST-003：完整Main用例遗漏当前收件人选择器（复验通过）

- 原Linux目录包完整Main用例在Web前置等待已不存在的`Exact recipient`输入。仅改测试交互为当前Recipient→精确UUID查询→选中账号，并保留表单选中状态和全部回执/版本/权限/工作流断言。
- Playwright侧复用既有TS helper，plain Node packaged控制器使用等价小型`.mjs` helper；新增文件哈希独立记录。未新增依赖、产品代码或超时/重试。
- 第二轮容器前置无法解析宿主绝对node_modules链接，保留模块错误；在任务ignored目录复制既有同版本Playwright三包，并核对容器内解析后复验。
- 第三轮及恢复选中状态断言的第四轮实际目录包完整Main通过，Mac原生.app完整Main也通过。目录运行不等于安装器或升级验收。
- 证据：`linux-full-main-1/2/3/4`、`mac-packaged-main-1`、`research/installed-picker-review.md`、`container-playwright-layout.json`。

## QA-ENV-003：条件分道执行门、专属Provider与自然限流（复验通过）

- namespace控制用例要求执行门打开、Host准入关闭；首次未准备该前置导致503/disabled投影。仅在专库按当前契约准备，记录原布尔和epoch，执行后恢复。
- memory-runtime首次误选普通host-provider而缺少memory_input观察，改为已经编译的专属memory-provider；随后memory→Leave在同API/DB且不重置gate条件下均通过。
- 密码批次三份run受到正常30次/5分钟注册限流影响，原setup失败保留；等待自然窗口后以新run执行未通过项。未重置服务或调整限流。
- 证据：`research/password-namespace-gate-preparation.json`、`evidence/password-batch/progress.json`、各`password-*`run与`evidence/environment/password-gate-restored.json`。

## QA-ENV-004：Linux测试tmpfs默认noexec（复验通过）

- Linux信号回收/标准输入补测初次无法执行测试创建的fake CLI，实际`/proc/mounts`证明任务tmpfs为noexec。
- 新运行仅将该容器的测试tmpfs配置为exec；网络仍关闭、rootfs只读、capabilities全部移除。3个顶层及3个子例通过。
- 证据：`evidence/linux-agent-kernel-1/2`。

## QA-ENV-005：知识标签专项要求固定的专库前缀（复验通过）

- 默认Go全量不编译`knowledgeintegration`；单独专项首次使用通用任务库名，被已有owned-prefix保护拒绝，业务尚未运行。
- 原4段拒绝保留。新建符合ordinary/Host各自前缀的4个专库，guard/断言不改；normal/race分别补跑，23个顶层用例分别normal/race通过，无失败/跳过，见`research/knowledge-tagged-summary.json`。
- 首轮错误不归为产品缺陷或代码通过；将最终实际专项结果单列在正式报告。

## 工具入口误用（已纠正）

- 原生测试结果checker首次缺少必需参数退出2，打印usage；随后以真实macOS各包最终JSON事件及OS参数调用，11个必需native case验证通过。原usage记录保留于`evidence/final-harness-checks`。

## 既有门禁与后续条件

- 历史 `822c15785` 与 `639bf6ef8` 的失败结论保留；本轮新证据单独归档。
- 旧私有adapter-v5/RKI6 launcher审阅缺口不由本任务规划审阅关闭。本轮使用新的直接命令与permit，不写旧clearance。
- Linux ARM64 DEB包管理器新装/升级/移除重装已实际通过；Linux原生桌面、其他格式/架构、Windows原生/安装升级及商业真实Provider账号/额度仍按真实前置登记，不能以共享代码、交叉编译或fake Provider结果代替。

## 本批续验记录：开放门与检查器观察

剩余门只读预检及身份检查没有新增可执行条件或产品缺陷。QA-PERF-001原2171ms>2000msFAIL、六BLOCKED及三NOT_RUN保持；证据为research/remaining-platform-conditions-1.md/.json、remaining-identity-symptom-1.md/.json。没有新的真实症状环境证据，不重复统计/cache诊断或P95。

本批初始私有身份检查器误将已保存的Next生成路由导入差异归为产品行为变化，原run.json失败保留；严格完整字节对照的verification.json已通过，无源码或验收结果修改。平台只读lookup错误文件路径的工具拒绝及修正另存setup-observation.json，不属于产品失败或测试授权拒绝。36项保护清单已包含既有未跟踪recipient helper；原记录保留。

最新清理next-remaining-cleanup-1为56个任务库连接0、六端口关闭、自有资源/协议注册0、原默认路径/注册集合不变。协调者复核与最终独立审阅分别完成；独立结论PASS WITH NOTES（research/remaining-final-coordination-review-1.md/.json）。未修改产品/SQL/索引/依赖/维护策略，无新增商业/账号/远程/发布操作。

## 本批：两处原skip的合同边界与新补证

CLI未知字段preservation是config.go与原skip明确记载的未实现能力，不能把future marker变成本候选须通过的保证。原skip不变，受支持的typed/map配置与任意未声明字段区别保留。

Webhook原DB错误skip不改写；既有db.New(DBTX)已提供可用seam。新私有身份TestTaskPrivateWebhookTokenLookupErrorDiagnostic在新专库一次race运行1+4PASS，补证prepare42P01/closed-pool→500、真实no-row→404/debt、429前置门/no-query与无body/写入。真实prepare错误不等于SELECT执行，closedpool是客户端故障；成功非空行解码和完整交付链仍未由本诊断证明。原Go10376PASS/1FAIL/6SKIP保持，新证据单列research/webhook-lookup-diagnostic-1.md/.json。

Mac自动更新已准备真实生成feed及源码/构件绑定模板，但无实际updater/feed/app执行。源代码表明manual check可在automaticUpdates=false时auto-download，且cache依赖Node HOME；Squirrel原生阶段/动态端口、安装与真实重启均需各自取证。实际更新/认证迁移/信任门仍开放，见research/mac-updater-readiness-1.md/.json。

本批未发现需自动修改产品的缺陷；原QA-PERF-001 2171msFAIL不改，未重复相同夹具或P95。两个只读研究lookup路径错误及纠正记录保留；无429/空通知导致的执行失败。独立代码/原始证据复核已PASS WITH NOTES（research/readiness-code-evidence-review-1.md/.json），最终文档复核已PASS WITH NOTES（research/readiness-final-coordination-review-1.md/.json）；最新本批清理evidence/next-readiness-cleanup-1/run.json：57个已登记任务库均连接0，包含新Webhook专库；六记录端口关闭，自有进程/容器/挂载/协议新增注册0，原精确默认路径及注册集合与批前相同。只读清理审计不采集计划/统计或执行测试。
