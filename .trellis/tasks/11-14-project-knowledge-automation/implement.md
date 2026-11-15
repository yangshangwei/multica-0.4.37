# 项目知识自动沉淀与默认启用：实施与推进计划

> 进度更新（2026-11-15）：任务保持 `in_progress`，沿用已批准的 `2026-11-14-v1`。私有候选 `4e0f7cbe...` 的 API／CLI／production Web／Desktop 完整构建和两条实际旅程均通过：2 PASS、0 FAIL／SKIP／RETRY，源码稳定。所属服务与资源审计通过；私有最终 AC1–AC12 独立接受、0 阻塞；共享 ROOT 预审0知识冲突，Core107／Views46测试、2个typecheck和34文件定向lint已通过。PAY正式CODE_ACCEPTED_R06，进入P2 r07；共享 ROOT 最终整合仍开放，`final_acceptance_verified=false`。依据见 `research/progress-2026-11-15.md`。

> 依赖 `prd.md` D1–D10 与 `design.md`。勾选表示该项实现已核对；未勾选项可能已有部分实现，不能据此推断整个阶段尚未开始。定向测试、当前源码复验和完整产品验收分别记录，P6 通过前不宣称全部 AC 完成。

## 1. 规划检查点

- [x] 用户同意将两个需求合并为一个任务进入规划。
- [x] 确定无项目跳过、正式验收与不阻塞业务、异常处理及人工历史边界。
- [x] 确定统一服务端 AI 模型、一次配置及费用归属。
- [x] 确定项目默认自动采纳，有权负责人/空间管理员可关闭或调整。
- [x] 确定重开立即暂停，重新验收后逐条复查、恢复或修订。
- [x] 完成默认值、来源/身份、模型契约与暂停/版本兼容研究。
- [x] 按最终结构重写 PRD，保留 R/D/AC 编号及证据引用。
- [x] 完成最终设计独立复核并处理四项发现，记录见 `research/planning-review.md`。
- [x] 校验最终上下文清单、引用、无未决产品问题与任务状态。
- [x] 用户于2026-11-14批准 `2026-11-14-v1` 实施；批准记录见 `research/implementation-approval.md`。

## 2. 顺序与文件责任

保持一个任务，在内部按下面阶段形成可审阅变更。后续Codex原生执行/检查子代理只处理明确分配的文件，主会话负责共享文件整合与最终验证；不得撤销其他会话现有改动。

### P0：实施前冻结与回归基线

- [x] 重新读取当前分支/工作区及本任务规划版本，核对已有其他任务对 lifecycle、router、协议和共享界面的改动。
- [x] 根据当前实现选定独立测试数据库/运行环境；不运行真实账户模型，不把生产环境借给测试。
- [x] 在后端/协议/Core的正确归属层先增加最小失败回归，覆盖新默认值、系统身份、delivery来源、重开有效性和v1/v2边界。见 `research/p0-check.md`；实际事务重开竞态仍在P2/P6。

责任：主会话协调，测试边界依照 server specs。P1–P5依赖P0。

### P1：持久模型、领域契约与严格版本读取

- [x] 添加项目策略、验收工作、append-only evaluation及有效性头；扩展候选human/automation来源和不可变约束。
- [x] 新增delivery精确来源，复用交付领域的accepted/SubmissionID读取，保持旧memory/comment/run_record。
- [x] 公共知识v2请求/响应、来源union、manifest有效性绑定、普通/协作runtime2；同时保留严格v1读者与原v1回执恢复。
- [x] 更新普通持久记录版本CHECK并按原版本读写；保持协作稳定slot可发现，不用新key隐藏旧私有历史。
- [x] 新关系无FK/cascade，索引CONCURRENTLY且一文件一语句；运行sqlc并更新删除/保留清单及兼容测试。核对范围以对应封存源码和迁移证据为准。

文件边界：`server/migrations/`（实施时分配最新编号）、`server/pkg/db/queries/collaboration_knowledge*.sql`及对应generated、`server/pkg/protocol/collaboration_knowledge*.go`、`server/pkg/protocol/task_knowledge.go`、服务端严格存储解码器。旧客户端测试必须在写新记录前通过；不能只改协议常量。

原知识origin索引与stage submission重复占用861，已仅将未发布知识索引迁移改名865，SQL字节不变；当前知识迁移为844–860及865。18个迁移、16个单语句CONCURRENTLY索引、无新增FK/cascade和sqlc141文件零漂移已核对，见 `research/p6-migration-contract-unique-prefix-ebc4f688.json`、`research/p6-sqlc-filecache-097c96ba.json`。历史 storage 9、downgrade 22、retention 3、protocol 451、retained-version service 25 项通过。删除清单遗漏已修复：补齐 `report_snapshot`/`report_snapshot_control` 分类，完整manifest及历史保留2/2通过，见 `research/p1-manifest-fixed-86431872.json`；新布局迁移569/569、知识/authority/mixed race428/428和testutil18/18均通过。最终ROOT产品与兼容验收仍归P6。

### P2：项目策略、正式验收工作与即时暂停

- [x] 落地默认策略和有权限的覆盖设置，revision/request幂等，显式关闭不被默认升级覆盖。
- [x] 正式accepted事务只写持久工作；唯一issue/accepted event身份。无项目/manual/关闭策略不启动模型工作。
- [x] 实现租约、阶段结果、uncertainty、有界DB退避及有权显式重试；不向原关闭业务队列派发。
- [x] 读source的事务锁内执行有效性校验；重开依赖既有epoch前进即刻使旧认证失效，并追加原source失效扫描。
- [x] 新prepare/authorize/续跑检查均应用自动有效性，不依赖异步目标hint；原人工决定语义保持。

文件边界：`server/internal/service/collaboration_lifecycle.go`、`server/internal/service/collaboration_delivery.go`、`server/internal/service/collaboration_knowledge_sources.go`、`server/internal/service/collaboration_knowledge_resolve.go`、新的策略/自动工作/validity领域文件及handler/router wiring。复用原lock order，不在source写事务增加target锁。

实现及定向证据已具备：policy/accepted intent 6、消费竞态 30、source isolation 2、daemon/host 138 项通过。消费来源文件清单仍为 30/30 匹配；最终整个服务端版本的行为复验继续由 P6 承担。

原HTTP整合失败已修复并保留：`UpdateProjectKnowledgePolicy` 与 `RetryCollaborationKnowledgeJob` 的拒绝投影不再丢失封存回执的版本、身份和revision，原非成功状态及原请求恢复8/8通过，见 `research/p6-sealed-refusal-fixed-f070f6fc.json`。此结论绑定该HTTP检查源码，不替代P6真实产品旅程。

### P3：服务端提炼、规则采纳与复查恢复

- [x] 在`server/pkg/llm`增加自有typed JSON result/usage薄接口，复用原参数协商，旧四类消费者行为不变；登记新增消费者。
- [x] 固定预算：0–3候选、完整来源≤8份/128KiB、完整目标≤16条/128KiB、每阶段总输入≤288KiB/wire2MiB、两个受限模型阶段、每API并发2。
- [x] 新增额度与完整重验复查分开：固定全部暂停自动候选清单，≤3条/批、最多6个复查批次加1个新增批次；每批独立持久身份、阶段预算和原回执。新增最多3条不截断应复查集合。
- [x] 精确引文/完整hash/版本/当前来源权限、schema、模型判定完整性、目标revision、策略和预算形成确定性门；模型不输出权限/真实ID/工具调用。
- [x] 完整对照不可读、超限或变化时待处理；精确重复跳过；semantic duplicate保留依据，conflict/uncertain转待处理。
- [x] 每条采纳结果、系统身份、evaluation、validity及target revision/hint原子落库；回执丢失恢复原结果，不重新生成。
- [x] 同一批可容纳集合及逐条结果在一个事务原子保存；失败整批回滚，不误报独立成功。自由文本模型理由/引用按该次请求全部输入来源闭包授权，不信任related_refs缩小权限；公开待处理仅用reason_code。
- [x] 重新验收后逐条复查：相同恢复新validity，变化按规则修订，冲突保持暂停；人工撤回/修订和再次重开优先。
- [x] 分开create/revalidate/revise意图；精确自身前驱不触发普通重复/修改冲突拦截，其他内容仍完整比较；新validity支持来源纳入prepare/replay/authorize的锁和可用性复查。
- [x] 暂停保留原approved容量，恢复/替代仍检查容量；旧manifest不被自动替换成新证明续跑。
- [x] 新补的后续验收与迟到对照两个回归已通过；published-lineage 上限的重试及后续人工决定覆盖已完成，当前完整 worker 矩阵27/27 GREEN。完整 service/race 与消费者门禁归 P6 继续核对。

文件边界：`server/pkg/llm/client.go`及outbound/usage测试、新的自动整理/规则服务、现有知识事务原语、`server/internal/service/task_knowledge*`、`server/internal/daemon/knowledge_context.go`和`server/internal/daemon/collaborationrun/host_knowledge.go`的v2消费适配。

历史 expanded worker 18、expired lease 1、model/rules 62、LLM 56 项通过，生产 worker 已在 `server/cmd/server/main.go` 启动接线。当前worker27/27及完整service race3840/3840已完成对应输入复验；模型语义、修复后产品旅程及最终ROOT整合仍由P6核销，不能从局部或私有通过推导全部AC完成。

### P4：Core、共享Web/Desktop界面及异常处理

- [x] Core增加v2 schema/版本协商、策略和工作Query/Mutation、原请求恢复；网络响应不cast。
- [x] 交付完成处展示整理/未配置/已保存/待处理/无复用内容；项目提供默认策略与覆盖控件，不增加验收必填项。
- [x] 知识详情展示human/automation、来源、规则和证据，暂停状态独立于approved历史；未知协议清楚显示不支持。
- [x] 待处理仅对当前可读来源且有目标权限的人开放，引用和通知不泄露其他受限来源；人工决定与后台迟到响应的UI保持一致。
- [ ] 新header覆盖CORS和安装版Desktop；新增来源链接沿现有任务/交付历史入口，不新增全局路由。

实现核对：Core/Views 历史定向测试分别为 113/156 项通过；P4 文件清单 37/38 个哈希一致，全部 23 个生产/翻译文件一致。发生变化的是增加拒绝重试用例的 Core 测试文件，该文件此次复验 6/6 通过。HTTP 拒绝回执已保留原非成功状态及封存身份，修复后 8/8 通过，策略失权恢复和两种工作拒绝恢复断言均已执行到，见 `research/p6-sealed-refusal-fixed-f070f6fc.json`。私有 P6 的实际 Web/Desktop 消费已通过；共享 ROOT 的 CORS 和平台证据需随最终源码联调重新绑定。

文件边界：`packages/core/collaboration/knowledge-*.ts`及项目策略相关Query、`packages/views/collaboration/knowledge-*`和交付详情、`packages/views/projects/`、两种locale。共享逻辑不落到app目录，平台接线分别验证。

### P5：默认值、文档、规格与可部署边界

- [x] 修改router与Compose默认值、`.env.example`；保持check脚本和既有部署显式关闭。新默认值在reader与自动化保护完成后一起交付。
- [x] 同步运行/升级说明、环境变量用户文档、模型资料消费者、帮助生成产物及相关builtin skill/source-map。
- [x] 更新server/core/views specs，保留旧任务的历史失败/未测事实。
- [x] 写清服务端/客户端/daemon版本顺序、新字段保留与关闭/回滚限制；不在本轮部署。

文件边界：`server/cmd/server/router.go`、`server/cmd/server/task_knowledge_config_test.go`、`docker-compose.selfhost.yml`、`.env.example`、相关docs/spec/generated帮助。P5依赖P1–P4。

配置实现及用户文档已具备，绑定源码的router及P5文档检查已登记。server/core/views规格与索引、运行/升级说明已经同步自动化及严格v1/v2契约；生成帮助只更新manifest和两页相关产物，157文件与源生成结果一致，见 `research/p5-generated-help-current.json`。八份P5独立审查、138本地链接及帮助核对已完成；修复后静态检查和最终ROOT配置/平台闭包仍在P6，既有报告不自动绑定后续变更。

### P6：完整验证与最终交付

2026-11-16检查点：新完整SOURCE已实体封存，wrapperv2的58项离线检查及独立准入已通过；整棵SOURCE独立closure尚待完成。旧TMP运行资产缺失、原因未知，历史私有4e最终AC与原失败证据继续保留。任务维持`in_progress`／P6，ROOT最终验收仍false；本检查点尚未创建新PG配置、environment、DB、allocation、build或runtime。最新顺序及证据见 `research/progress-2026-11-16.md`。

既有封存输入已完成前端lint/typecheck/test14,112PASS（Docs64缓存）、service race3840/0/0、handler5912/0/7、router1014/0/8、新布局迁移569/0/0、知识/authority/mixed428/0/0及testutil18/0/0；完整具名分母、skip原因及原失败见进度文件。Go build/vet、非DB和UI入口检查也有各自源码绑定，不能将这些报告视为最新ROOT全量通过。

P6私有 `f8a313ea125c357cc3a05f90770da33e15d6d0dc7d733854d7d533615eb339fc` 已完成API/CLI/production Web/Desktop构建、启动和live配置核对；两条实际产品旅程均在注册返回400 `invalid_username`，0skip/0retry，原报告/hash不变，四所属服务已stopped、cleanup0。见 `research/p6-original-password-registration-failure-e562808a.json`。随后已只修ROOT与私有detail fixture的共同password actor块：合法≤32字符用户名、真实PATCH `/api/me/onboarding` / POST `/api/me/onboarding/complete`，校验用户ID及 `onboarded_at`；见 `research/p6-password-account-onboarding-repair-4fa0adbd.json`。修复后private/ROOT定向noEmit各0错误、独立admission0阻塞、历史93c78b9a完整build／启动通过，其后两条旅程在缺少profile owner处失败；修复后4e完整两端PASS见本节最新进度，原失败保留。ROOT原有另一任务comment-menu import/locator和Views等差异全部保留，不宣称公共/私有四文件完全parity，ROOT整合仍open。

C3原prepare/authorize直接重放在指定冻结b91 Go图上normal2/race2逻辑叶PASS，见 `research/p6-c3-direct-replay-20261115-0f943220c767.json`；这不替代ROOT或后续build。两次独立AC检查仅为interim准入，均未给出final AC/P6验收，见 `research/p6-final-independent-ac-review-20261115-fa158b1b.json` 与 `research/p6-final-independent-ac-review-20261115-deployment-3ae86e4b.json`。其后私有4e最终独立AC已接受、0阻塞，见 `research/p6-independent-final-candidate-ac-review-20261115-a710db0a.json`。共享ROOT最终验收尚未完成，`final_acceptance_verified` 仍false。

- [ ] 执行下面的验证矩阵，修复真实失败并保留原始证据。
- [ ] 独立审查系统权限、事务竞态、旧读者和隐私投影；主会话核对实际测试清单及所有AC。
- [ ] 汇总已改文件、实际通过/失败/跳过、未覆盖的真实模型语义/平台范围和部署注意事项。
- [ ] 在获得相应后续授权后进入提交/交付；本规划不自行创建PR、推送或部署。

## 3. 验证矩阵

下列新测试名为实施目标，不代表已存在或已运行；可按仓库实际命名调整，但不能丢失行为覆盖。

| 场景/拟测试 | owning layer | AC |
| --- | --- | --- |
| `TestKnowledgeContextDefaultEnabled`：unset/empty/true/false/invalid及实例传播，Compose与隔离test false | router配置、shell静态fixture | AC1 |
| `TestKnowledgeAutomationAcceptedDelivery`：他人/自验收，真实accepted/source，后续实际输入收到版本 | service集成、受控daemon | AC2 |
| `TestKnowledgeAutomationSkipsNoProject`：零工作/候选/模型请求/workspace写入，手工功能保留 | service/handler | AC3 |
| `TestKnowledgeAutomationEventReplay`：非验收拒绝、重复通知、多worker、回执丢失、阶段成功后恢复 | service/worker | AC4 |
| `TestKnowledgeAutomationRules`：0–3、精确引用、重复键/未知ref、精确和语义重复、三种意图及自身前驱例外、冲突、不完整/失权对照、改目标revision | 纯规则及fake模型 | AC5 |
| human/system、来源、证据、未知用量/协议、暂停/待处理与原请求恢复；模型未标related_refs仍按全部输入来源闭包限制理由 | Core、Views、service投影 | AC6 |
| `TestKnowledgeAutomationNonblocking`：manual、未配置、超时、DB退避、生成uncertainty，不重放业务 | worker/LLM/handler | AC7,AC10 |
| `TestKnowledgeReopenFencesConsumption`：reopen与prepare/authorize两种提交先后，不依赖outbox，既有会话复查 | service事务、daemon | AC8a |
| `TestKnowledgeReacceptanceValidity`：原文恢复、修订替代、超过3条完整分批、跨批冲突、证明新支持来源失效、冲突/过期/失权继续暂停，再次重开和旧租约 | service事务 | AC8b |
| `TestKnowledgeHumanDecisionWins`：人工withdraw/correction与迟到自动结果两种提交顺序，人工批准/其他来源不受影响 | service事务 | AC8c |
| 16条/128KiB、暂停保留容量、部分可容纳集合原子结果、事务失败整批回滚、原子回执，无淘汰或截断 | 协议预算、service | AC9 |
| 未配置零出站、完整模型/usage presence、原参数协商与旧消费者不变、私有错误隔离 | pkg/llm及outbound契约 | AC10 |
| 默认策略、显式关闭、角色变化、CAS/请求身份、在途关闭、不回填历史 | handler/service、共享策略控件 | AC11 |
| v1 legacy/v2新形态、明确unsupported、旧回执、普通存储双读、协作固定slot、CORS、已曝光会话拒绝降级 | protocol/Core/service/daemon | AC12 |

至少一条Web和原生Desktop完整旅程覆盖：项目任务提交/验收→后台提炼→采纳→后续受控运行输入→重开暂停→重验恢复；另测无项目和纯人工零自动调用。构建Desktop不等于操作验收，fake provider不等于真实模型理解。

## 4. 命令与环境前提

实施时读取`Makefile`/各`package.json`作为命令源。先最窄相关测试，再按跨层风险完成 `pnpm typecheck`、`pnpm lint`、`pnpm test`、`make test` 和适当的静态/完整检查；需要全流水线时执行 `make check`，保留实际运行的包/测试清单。

Go编译可用 `go -C server build ./...`；执行前必须依照 `.trellis/spec/server/database-test-isolation.md` 和 `.trellis/spec/server/verification-environment.md` 分配并证明独立测试DB归属、迁移状态和配置。`go test -run '^$'` 会执行TestMain，不能用作编译验证。service/handler/router分别使用适当独立DB并避免全局fixture互相清理。

普通测试只用fake模型/测试专用可执行文件，并移除环境中的真实模型账号配置。真实账户、Provider质量和新平台矩阵需要单独明确预算与运行授权，不在普通检查里触发。完整UI测试使用该任务的生产模式Web、API和独立数据，不使用next dev冷编译造成的假失败。

## 5. 回滚与完成证据

关闭项目自动沉淀阻止新工作/采纳，不删除历史；关闭知识使用保留原会话限制及清理。新的持久形态只能由识别该版本的程序读写，不通过删除候选或换slot强制降级。

最终报告需要绑定实际源版本/工作区差异、配置、运行环境、具名测试与AC。规划校验只证明文档/引用/状态一致；不能提前勾选产品AC或宣称真实模型可靠。

## 6. 已登记推进与历史证据（2026-11-15）

1. **修复与静态核对已完成。** 合法 password 账号／真实 onboarding API 和已验证 profile owner marker 均已补齐。private／ROOT 定向 noEmit 各 0 错误；detail fixture lint 原 8／现 8，automation fixture 原 5／现 5，新增均 0，standalone lint 仍未通过。两轮独立变更准入为 0 阻塞，所有原始失败保留。
2. **私有完整构建与旅程已完成。** 候选 `4e0f7cbe36dadf1936c31a5dfc04656e7d6ac4594e5a45b9eac1e8e02f697191`、Web BUILD_ID `Suwn9SgGXHAkR5YxpSvU5`、`RUN/build-419ecea5cf21`／`start-eaaff8bb0f95`／`test-0440e9486493` 完整绑定。Web 与实际 Electron Desktop 两条旅程 2 PASS、0 FAIL／SKIP／RETRY、源码稳定；覆盖自动采纳、真实 daemon 精确正文与 v2 证明、重开暂停／旧续跑零新增 provider、同候选版本重验产生新 validity、新执行成功及无项目／纯人工零自动工作、候选、调用与工作空间自动写入。见 `research/p6-final-bound-journeys-and-cleanup-fc83cb34.json`。
3. **所属资源审计已完成。** 四常驻服务停止、cleanup 0；11 个记录的所属进程检查无残留，API／Web／renderer／model 及两个 daemon 健康端口共 6 个端口无监听。browser／C3 两库保留且连接各 0，原报告／日志／绑定和三个 before-stop 快照哈希不变。Redis 所属 container／network 已清理，无重复 teardown；见 `research/p6-final-owned-resource-audit-569cc14c.json`。
4. **私有最终 AC 已通过，当前入口为共享 ROOT 整合。** 原审查代理确认 AC1–AC12 全部私有候选行、C3的3,104生产输入映射及23份主证据哈希，0阻塞，见 `research/p6-independent-final-candidate-ac-review-20261115-a710db0a.json`。共享ROOT选定113对文件预审0知识冲突；Core107／Views46、2包typecheck／34文件lint PASS，见 `research/p6-root-knowledge-integration-preflight-a34fa9cc4e45ecca.json` 和 `research/p6-root-knowledge-frontend-verification-jOgYVN-c1c27d4b.json`。当前共享容器4文件67项PASS，见 `research/p6-root-knowledge-container-verification-bUtC74-dd29b635.json`；全Go build／vet已PASS（12,173输入稳定），见 `research/p6-root-knowledge-go-static-verification-20261115-7a3e544be95f.json`；当前ROOT service知识normal／race已独立最终接受，每模式194top／428named／365leaf PASS、0FAIL／SKIP／缺失／重复／未结束；当前4774源与配置稳定、两个所属DB连接0，见 `research/p6-root-knowledge-service-independent-v3-normal-race-20261115-87a4b72f8b7f.json`。v3解析器29项GREEN／PARSER-C1关闭，原normal recorderfailed及缓存复制RED保留，未重复执行Go；最终冻结／构建／运行依赖原PAY P2 r07的独立接受。两处 ROOT 窄补丁已用历史 before／after 完整哈希重建核验并交给 PAY 原负责人，见 `research/p6-root-narrow-fixture-binding-b7dc8f78.json`。detail actor 相同，整文件保留既存 comment-menu 差异；automation 全文相同。共享 ROOT 仍有并发 Go／SQL／frontend 变化，需单独冻结最终联调图并重新绑定受影响检查、构建与运行，私有 PASS 不关闭整个任务。
5. **PAY-241 按依赖推进。** 原 thread `01a1dde9-4bd6-7c00-9fa1-e29bf5ddb142` 继续承担实现与两个独立检查 lane；统一 test-only 4,968 输入、manifest `fffeec5a...`、真实 AST 116＝37＋79、四 fresh DB 配置已核对，normal 与 race 均已完成，各 116 个顶层／321 个 named PASS，0 FAIL／SKIP／缺失／额外／重复／未结束；当前两组 race raw 与原 checker 结果哈希已核验。见 `research/pay241-owner-s2-running-and-final-binding-callback-718237a5.json` 和 `research/pay241-current-normal-race-116-pass-observation-ef9fe50a.json`。原负责人已正式交付 CODE_ACCEPTED_R06／当前代码阻塞0，W01–W18／Q01–Q12代码契约已接受，见 `research/pay241-code-accepted-r06-bound-callback-16483c51.json`。历史r03及原V01／V02失败仍按原结果保留；当前P2=false，产品0／4，父24为4PARTIAL／20NOT_RUN。P2 r07已实际封存healthy-schema PUT503／expected200 RED（1 named FAIL、0 SKIP），19个专用profile已准备；最小3既有文件修复继续由原owner承担，GREEN候选源 `18b3f4bd…` 已绑定并获执行许可，18个未用profile的实际验证／独立P2接受待完成，随后依次为P2新码独立接受与真实schema negatives／readiness／启停／history及原键恢复→匹配五个 binary 与 production Web／Desktop／实际 API／WS→四产品场景／A01–A12 与 cleanup→S3→真实非生产 S4 E01–E07→正式 S5 性能／父 24。
6. **完成门槛。** 知识私有AC已有最终接受；仅在共享ROOT最终输入验收也有一致证据后更新任务完成状态。真实模型语义质量、安装版 Main／installer 和其他平台维持原有独立验收范围。


本轮新增的 password／onboarding 与 managed profile owner 可执行夹具契约已写入 Web E2E spec，见 `research/p6-password-managed-spec-update-42ee8838.json`；独立文档契约检查 PASS／0 阻塞，见 `research/p6-password-managed-spec-check-52067d2e.json`。审查补齐现有 `requires_password_change` 拒绝条件；独立夹具 lint 的原 8／5 个诊断仍保留。它记录现有生产前置条件，未改变产品接口或认证规则。

## 最终ROOT推进检查点（2026-11-16）

P2已取得 `P2_CODE_ACCEPTED_R09_SCOPED`／0阻塞／ready_for_ROOT_narrow_integration=true，源80abb746／4,968；18成功profile，10新命令＋8明确绑定复用，N／R具名集合相等，4native SQL controls／862恢复／历史与资源审计通过，0所属连接／PID。原R07／R08失败保留；Live／完整S2／四业务场景仍未接受。

ROOT三文件已窄整合，5,035保护路径0漂移、35peer产品差异／865／hooks／virtualoff保留；整合回执SHA b52fc31fd8a0e97e0bbbc76532a4dc2c5d63903aa9f43d00a7aa7b3240af5166。澄清ddb430dbfdf85ac518a8a6e4c2c79a9967aa5a753f423584d0c83263cd9b616a说明P2项目stage策略默认off未变，非知识自动策略的默认值。Go joint e4dbc77e／5,038与frontend315a7654／3,358仅分层证据。

旧TMP父目录在`/var`别名和`/private/var`规范路径均缺失，原mutable／runtime／sealed4e实体不可用，原因未知。原4e AC、旅程、清理、原失败及审批报告和哈希仍保留；后续进入NEW persistent STATE／allocation准入。

新完整ROOT已一次实体COPY为`frozen-source`和`mutable-source`：source_id `5dee3553b10a530ac841c58afe3ad2e737eaf7eab4bd72e3544b3d65c2edae33`，选定private HEAD `2f6989bfa5630ff2790d37a539c1522d2d29d094`，各10,408文件／32workspace links。ROOT3、RK3、Final5及stage activation三after-image已在两份目录核对，完整字节／权限／链接无漂移。post-COPY guard证明后续controller的代码、Markdown及workspace源码读取只使用private SOURCE；原始许可、patch和manifest已另行持久保留。见 `research/p6-physical-full-source-wrapper-v2-bindings-20261116-5dee3553b10a.json` 和 `research/p6-post-copy-sealed-source-independence-20261116-5dee3553b10a.json`。

NEW wrapperv2封闭PG URI的host／dbname query覆盖、编码／重复项、非法port和fragment；原7项、49 URI场景及2保留守卫共58项离线通过。独立审查`WRAPPER_V2_ACCEPTED_FOR_FRESH_RUNTIME_PREPARATION`／0阻塞／PW-PG1 closed，整棵SOURCE review仍待关闭。本检查点actual env／DB／build／runtime为0；主会话随后仅授权NEW mutable `.env`独占0600的PG-only配置与validate-only pin。实际SQL／createDB／allocation须等整棵SOURCE独立closure及主会话新handoff，source-ready和最终ROOT验收仍false。

PAY四产品场景的首项真实失败：HTTP400 `unknown execution_view_version`，后3项NOT_RUN。原失败保留，下一步由PAY owner修复私有`collaboration-multi-account-closure.ts`的`workflowAuthority`，review后再写ROOT fixture。该窗口已经释放，后续ROOT fixture／任务Markdown更新不会自动导入本次封存SOURCE。
