# 最终跨链路执行清单（静态映射，未执行）

被测候选 `231a08063162acde63ebcabe2ef6a353f7a62bca`。映射时主分支文档提交为 `20f128dce`；server/e2e/apps/packages/scripts 与候选相同。此报告只读源码与既有证据，未运行任何测试、浏览器、数据库、服务、Provider 或外部操作。8 个步骤的精确 argv、静态预期数量、覆盖边界和 32 个源码哈希见 `crosschain-itinerary.json`；静态数量不能算实际执行结果。

## 顺序与最少重跑范围

先等当前完整 `make check` 结束。若同候选默认 legacy 报告已具名证明 `collaboration-memory-web.spec.ts` 的 2 项及 `collaboration-knowledge.spec.ts` 的 8 项非跳过通过，可以复用该结果，不必再跑相同范围；必须核对实际报告，不能从“全套退出0”推断 opt-in 套件运行过。

| 次序 | 现有入口 | 静态顶层数量 | 必须环境与范围 |
|---|---|---:|---|
| 1 | runtime-handoff Web/desktop、handoff-chain Web/desktop、offline-handoff、memory-web | 9；复用 memory2 时实际补跑7 | password；同源生产 Web/native renderer。ENV1、长评论及旧页召回、撤权后私读拒绝、partial/unknown记录投影、人类接手链、并行memory与原键恢复 |
| 2 | control-journey Web/desktop、live-revoke Web | 5 | password/managed；协议执行门显式开启、host门可继续关闭。两平台 partial Resume/未确认proof、grant撤权与排队输入保留 |
| 3 | host-runtime、host-switch、host-squad | 3 | password/managed；显式新建的 fake daemon/provider/CLI。实际本机子进程、Stop/Resume、owner确认Switch、capacity1 squad接力 |
| 4 | memory-runtime | 1 | 同 password host 环境；将 provider 选为专用 memory-provider。两个本机runtime/profile/workdir通过实际CLI读取/写入v1→v2→v3 |
| 5 | collaboration-knowledge | 8（7Web+1Electron） | **legacy限定**，知识执行默认false。人工精确批准、纠错、派生、来源purge、过期/撤回、原键恢复。若最终完整check已实际覆盖则复用 |
| 6 | `^TestKnowledgeServerDaemonComposite` / internal/daemon / knowledgeintegration | 14顶层，静态27具名事件 | 每包独立新库，fixture内部legacy、knowledge=true。真实HTTP/WS/daemon与测试进程的实际输入和拒绝边界 |
| 7 | `^TestCollaborationKnowledgeHostComposite` / cmd/server / knowledgeintegration | 8顶层，静态16具名事件 | 另一独立新库；真实router/DB/Runner/HostExecutor与fake进程，实际stdin/argv及后续撤权拒绝 |
| 8 | TestRK1FakeProof + evidence/format/marker/trace回归，normal后race | 以实际具名清单为准 | 最终安静窗口、每次新库、无真实Agent标志。H2b大结果tail、H3评论页、真实新接口与实际启动分账；RKI6首个不完整仍保留并阻止付费 |

选中必需用例均应 **0 skip**。默认 legacy 跳过 password/Host/安装版只能说明通道不适用，不能关闭对应门。保持 worker1、retries0及源码原时限。整库/迁移/全包由统筹分配；普通compile使用 `go build` 或 `go test -c`，不要 `go test -run '^$'`。

## 两个关键环境陷阱

1. `collaboration-knowledge.spec.ts:17` 的 beforeAll 调用 `legacyLaneMissing()`；password 模式会抛错，不是可换通道的skip。它把任务库 execution/host门置false、creation置true并恢复，不调用runtime。`knowledge_execution_enabled:false` 的附件只是该suite范围说明，不能当成读到实际部署flag的证明。
2. 两个 knowledgeintegration 夹具硬性要求历史命名**前缀**：普通 `multica_knowledge_composite_20261101_`（daemon文件:51），Host `multica_knowledge_host_composite_20261101_`（cmd/server文件:221）。现在仍可在其后加唯一20261102后缀分配新库；`collab_joint_*` 等其他安全名字也会被拒绝。不要为了日期改产品guard，不复用旧失败库。cmd/server 的 TestMain 即使加filter也先接库；daemon组合会验证实际 current_database。

每个Go包使用不同新迁移库，normal/race完整重跑也用新库。知识候选历史和all_sources扫描保留，不能清历史求绿。Go命令执行前由root明确核对DB身份；正文不含任何URL凭据。

## Host假进程入口

`hostRuntimeConfigured()` 在三个显式binary变量任一存在时返回true；一旦配置，`hostEnvironment()`会要求全部值、loopback URLs、非默认DB、password、deploymentUUID及绝对现存可执行文件。因此不能只设一个值让suite“自动补全”。需要：

- `COLLABORATION_HOST_DAEMON`：同候选 `go -C server test -c -o <任务绝对路径>/host-daemon.test ./internal/daemon`。`collaboration_host_router_test.go` 的 init 只接受专用 `__multica_collaboration_host_router_poller` 参数，先于测试主体运行。
- `COLLABORATION_HOST_PROVIDER`：用同候选 stdlib-only `e2e/fixtures/collaboration-host-provider.go` 构建；memory-runtime另用 `collaboration-memory-provider.go`。按fixture-kind实际校验，不能把普通Provider二进制用于memory测试。
- `COLLABORATION_HOST_CLI`：同候选构建并绑定版本/commit/hash的Multica CLI，不能指向用户安装的Agent CLI。
- `DATABASE_URL`、`COLLABORATION_E2E_API_URL`、`NEXT_PUBLIC_API_URL`、`PLAYWRIGHT_BASE_URL`、`MULTICA_DEPLOYMENT_ID`、`E2E_PASSWORD_AUTH=1`、`COLLABORATION_ELECTRON_RENDERER_URL`及同源source root，Web+renderer实际CORS/WS预检，managed installation配置开启。

Host fixture自身在行锁下打开并最后恢复 enabled/creation/host_execution三门，epoch递增；不要并行另一个门控fixture。control-journey/live-revoke则**不会**打开协议执行门，先前503就是这个差异，root需只在其隔离库设置。原生fake profile有空Provider账号home，fixture只把受控变量传给子进程。

## 知识开/关和注入应如何分账

人工UI8项和实际输入组合是分层证明，不能拼成一次UI→模型旅程。默认完整check不带 `knowledgeintegration`，不能替代步骤6/7。

普通组合覆盖 issue/comment/Chat/quick-create/autopilot、实际claim/prepare/authorize/observations、已批准内容及原作者、withdraw/purge/expiry/gate在首启动前拒绝且零物理启动、失回执及原namespace恢复；Host组合覆盖实际新handoff/知识绑定/stdin、独立物理启动授权、后续withdraw/purge/expiry/source_access拒绝、旧协商不泄露及private stderr。两者把对象上的 `KnowledgeContextEnabled` 直接置true；普通组合还在gate子用例及cleanup场景置false，Host在恢复场景置false。它们不是独立API进程启动时解析env开关的证明。

如父验收要求部署形态的ON/OFF，分别记录同候选任务API/DB配置：一套 `MULTICA_KNOWLEDGE_CONTEXT_ENABLED=false`，一套仅测试环境true；各自以真实admission/输入或拒绝为证据。生产默认仍false。只重跑同一个程序化fixture两次但改外部env，fixture仍覆盖为true，不能称“验证了部署两态”。

服务层开关/并发可复用最终完整check的具名证据：`TestKnowledgeGateOffSafeArgsKeepOrdinaryClaim`、`TestKnowledgeGateOnSafeUnlistedArgsUseBaseExecution`、`TestKnowledgeDisabledGatePreservesOriginalObservationCleanup`、`TestCollaborationKnowledgeRuntimeNegotiationCannotUpgradeOrDowngrade`、`TestCollaborationMemoryConcurrentWritersFromOneBaseCommitOnce`、`TestCollaborationMemoryConflictRefreshMergeRetainsBothFacts`。需要局部补跑时只能在另一个root分配的fresh service库中串行，保留TestMain与全来源历史边界。

## 仍需明确保留的覆盖缺口

- **CCG1 纠错后的实际新进程输入**：humanUI有correction；service有竞争批准/withdraw与精确版本；两份actual-input composite没有correction/supersedes流程。若父项要求这一条跨链路，现有组合不足以声称“纠错后的实际stdin已验证”。最小后续是reviewed fake组合：批准→首输入→批准纠错→新run只含新版本→withdraw→后续拒绝且无额外启动。此报告没有新增测试。
- **CCG2 部署知识两态**：上述程序化true/false不等于真实API的env接线证明，另见前节。
- **CCG3 大结果与大集合分页**：runtime-handoff UI夹具priorResult很短；RK1-H2b把决定性code放到4096B前缀外，经真正CLI source分块读，评分要求暴露证据，能覆盖大结果tail。H2不会产生超过32项results/causes页，不能据此宣称该集合分页覆盖。
- runtime-handoff UI直接种植已完成checkpoint/seal历史，证明投影与权限，非摄取/kill；Host原生旅程证明真实进程但不是安装版full Main。stop/revoke保留还应结合最终同候选Go recording/authority具名结果；RK1 RKI6单次Run后缺seal仍必须如实incomplete。

## 安装与真实模型不纳入这些通过数

`collaboration-host-installed.spec.ts` 有独立opt-in：明确native packaged executable或已存在本地image二选一，不隐式pull；要求ASAR、CLI、executable、source hash及安装目录/包来源。Windows仍需实际原生/PowerShell/安装升级证据；cached旧包或源码renderer不能替代。

RK1 fake只使用测试可执行文件，真实入口必须不设 `MULTICA_RUN_REAL_AGENT_SMOKE=1`；最终paid仍由launcher检查runnable、来源、预算和独占窗口。所有fake和本清单均不占runtime25；原冻结系列另余14不混用。K1–K3/E0b/安装真实账号/AC43尚未预算与验收，不在本静态清单中关闭。
