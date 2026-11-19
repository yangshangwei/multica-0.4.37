# PAY-241 新会话交接

记录时间：2026-11-14 09:48Z。这是续接快照，不是S2或父任务验收报告。新会话仍须以最新文件、代码和进程核对结果为准。

## 1. 任务、授权与工作区

工作目录：`/Volumes/artisan/code/2026/multica-0.4.37`。

- 父任务：`.trellis/tasks/11-13-pay-241-collaboration-acceptance`。
- 现有子任务：`11-13-pay-241-s2-stage-approvals`、`11-13-pay-241-s3-recovery`、`11-13-pay-241-s4-external-integrations`、`11-13-pay-241-s5-performance-acceptance`。
- 沿用上述任务，不为续接新建业务任务。S1已归档；S3 timeout-proof子任务已归档。
- 用户已授权S2–S5规划、实现、测试、原生Trellis实施/独立检查、本地Lore提交，以及每个子任务自身验收通过后的归档。不要重复询问流程许可。
- 真实模型账号调用、真实资金操作和生产发布不在授权范围。缺外部条件时继续独立工作，保留NOT_RUN及具体缺项。
- 分支：`codex/cross-workspace-collaboration`。
- HEAD：`8e824b3035c4f17a9a9dfa0504f1294fbcc9b20b`。
- 核对时623项未提交改动，index为空；当前会话task pointer仍为S2。大量他任务差异不能覆盖或混入提交。
- 该HEAD是已完成的S5工具增量Lore提交，**不是S2提交**。本次S2产品代码和文档仍在工作区。

先读AGENTS.md、CLAUDE.md，按trellis-start/trellis-continue恢复，再读当前层规范。主会话协调、分配数据库、实际运行、更新规范、提交；产品/测试使用原生trellis-implement，独立检查使用trellis-check。服务端共享表、事务、CLI和完成门禁始终由单一实施负责人顺序推进，客户端可并行。

## 2. 旧代理与进程状态

旧服务端负责人为`/root/s2_server`；旧客户端负责人为`/root/s2_client`；检查为`/root/s2_server_check`、`/root/s2_client_check`；E2E编写为`/root/s2_e2e`。

交接时服务端实现代理和E2E检查代理均因Agent Router错误停止：`The requested item was created under a different Azure OpenAI resource`。这是代理运行错误，不是产品测试失败。没有重新派发它们。

- 服务端停止时尚未落实S2B-C01制品读取ACL修复，2c只完成冻结合同、序列化基线和DTO草稿，见下文。
- E2E检查代理在错误前已完整写入`joint-e2e-increment-2-check.{md,json}`并落实两项局部修正。主会话已核对最终5个文件、11份日志哈希和6个独立退出码文件，零差异。其余源提取检查退出码记在报告中。
- 未发现本任务仍在运行的Go/Vitest/受控Provider进程。六个已分配S2包数据库的只读连接审计均为0（排除审计连接自身）。
- 证据：仓库外`pay241-20261114-resume/new-session-db-audit.json`及`new-session-e2e-check-audit.json`。

旧agent ID仅供追溯。新会话先复核占用，再建立新的原生实施/检查代理恢复同一文件范围；不要依赖跨会话旧代理引用，也不要重复派发仍活跃的工作。

## 3. 第一优先：完成2b的独立检查修复

必须先读：

- `research/server-increment-2b-results.md`和JSON。
- **`research/server-increment-2b-check.md`和JSON。**
- `research/test-report-contract.md`。
- `research/wire-contract.md`，尤其8.7–8.9、13节。

2b已经实现command绑定subject、首次claim Git冻结、T1–T4、不可变报告/日志、QA批准/退回及QA→release/rework、CLI/builtin说明和宿主Git夹具。42个冻结server文件、45份日志/退出码已由主会话核对；独立重建为4889文件，vet/格式及窄协议/CLI检查通过。

实现报告中的最终命名测试：Stage108、关键race22、HTTP19、CLI root95/internal76、protocol336、harness21、brief2、cleanup1、QA hint6；另一个较早300项兼容回归单列，不能相加当验收案例。

**独立检查仍有S2B-C01/P1：T4列表/详情及QA可选报告引用只检查来源空间成员身份，遗漏R4已经执行的制品级读取权限。非制品发起人、非artifact manager的普通来源成员可能被R4拒绝，却能从T4读取制品绑定的subject/cases/logs。**

交接代码核对：`collaboration_test_report_view.go:testReportReadRun`仍只检查`SourceMember`；T4仍调用READ ONLY的`readCollaborationContent`。不能把此前“准备修复”的代理消息当已完成。

既定修复方向：

1. 复用现有`lockDeliveryArtifactInTx`的保留/当前制品读取授权边界，不新建制品权限体系。
2. 按报告原始冻结subject/candidate中的制品引用核查，不误用“当前候选/当前PR必须仍有效”来禁止有权用户读取旧失败报告。
3. 复用/抽取R4的repeatable-read且允许权限行锁的读取事务。不能直接在READ ONLY事务中调用M1锁查询。
4. 同样约束QA可选报告引用；不带私有引用的reason-only QA退回必须继续允许。
5. 用制品发起人/manager正例、普通来源成员拒绝、撤权、历史stale可读、T4 list/detail和可选QA引用做红→绿及race；保留失败并交独立复查。

2b冻结副本：`/Users/artisan/.multica/acceptance/pay241-20261114-resume/s2-server-inc2b-implementation-source`。它是42文件增量，重建须叠加报告列出的1b/2a及后续补充；不要静默从正在变化的工作区补历史依赖。

## 4. 2a与客户端已经完成的增量

2a已通过独立复查，原始及修复报告均保留：

- `server-increment-2a-results.*`、`server-increment-2a-supplemental-results.*`。
- `server-increment-2a-followup-results.*`、`server-increment-2a-followup-fixture-supplement.json`。
- **`server-increment-2a-source-fence-check.*`是A5最终成员锁修复结论。**

已修：16个S2索引清理注册；未授权R1 child digest显式null；原批准/原签字人最终DB时间到期检查；成员移除再加入不复活批准；A5来源成员行锁；GitHub旧回调、同时间不同head、未知check集合和派生链接竞争。A5空的外部scope仍允许；A5自身无保留JWT/PAT到期，A7 standing approval有有限到期。

客户端A5/3a、3b1、privacy+3b2已完成并检查：

- `client-increment-3a-recheck.*`关闭C01–C08。
- `client-increment-3b1-results.*`、`client-increment-3b1-check.*`：Core365/Views136；独立冻结schema173通过。
- **`client-increment-3b2-results.*`、`client-increment-3b2-check.*`**：原Core477/Views63；检查修复B01后Core70/Views64、types/lint通过，不能与原数相加。
- B01修复让最新R1来源拒绝同步隔离R4/T4私有缓存和迟到响应，并禁止旧证据保持C1/QA可用。实际来源workspace ID已成为内部`collaborationStageViewOptions`的必需参数；不是HTTP变更。
- C1用既有受保护mention-target的`can_submit_commands`，不放开全局直接命令`submit_command=false`门禁。
- 泛用GET404原请求行为、C08恢复和R38回滚型错误边界保持；R38失败不是持久失败回执。

主会话核对3b2的496个原始快照文件、34个最终范围文件、24份实现/10份检查日志，零差异。最新文件在工作区，复查副本为仓库外`s2-client3b2-checked-source`。客户端下一步是消费2c冻结合同，再做2d M1/activity，**尚未实现客户端2c**。

## 5. 2c已经冻结，修完2b后直接接续

阅读：

- `research/server-increment-2c-contract-preparation.*`。
- `research/server-increment-2c-contract-review.*`。
- **wire §8.10和§13.17**。
- **`research/server-increment-2c-control-contract.md`**。

交接核对SHA256：wire为`dabc19c5762e46577468e3ec436998b4837f28e9ba1996731b47b2d9ad3e8c14`；control contract为`ca3ee225e4a970fba2d70e389049f23f03f9967a01eb1882a812728bf2c6d1a4`。

2c目前只有合同、原请求序列化基线和部分Go DTO草稿（target_digest/rollback_observation及omitempty）。这些当前草稿尚不能继承冻结2b的编译结论，可能需要gofmt/编译收尾。没有829以后迁移；执行账本/完整release/rollback尚未实施。

不要重新讨论已经确认的角色隔离、人工手建child或2c技术取舍：

- A7 release绑定R4给出的`build.target_digest`；rollback绑定metric ID/digest、原成功部署及其立即前一制品，只发生在verification阶段。
- R4新增targets/operations/rollback_observations及fact的intent/target字段；旧响应缺字段表示unknown。native operation ID是有界字符串，不能当UUID。
- 未决执行用稳定的物理目标冲突键跨root/workspace/本地绑定串行，不能靠换集成/信任版本绕过。
- 真实内部consumer与原人类签字人分开验证；服务端同步拥有唯一调用入口，只有intent确定提交后才能进入一次。入口上限30秒且受原审批/证据/source proof更早到期约束；按最后DB时间加单调耗时判断。收尾上下文5秒。
- 进入前能证明零调用才可存internal not_started；提交/收尾不确定或进入后的异常保持unknown。历史lookup不能恢复调用权；不自动重试。
- A8在intent提交时消耗该批准，不能伪称撤销已取消外部执行；A9从选定当前有效release批准的唯一intent推导成功fact。
- 迟到认证结果只留历史，不续期、不重新背书旧部署、不重执行或自动advance。
- 同一native metric version不同canonical digest须持久标记ambiguous，旧anomaly及其批准不能继续消费；仅更新的有效原生观察可解除。
- 普通生产构造缺适配器时不可用。正例用test-only实际router/auth/background harness及私有控制管道注入受控来源，调用生产Prepare/Record；没有公开fixture路由或生产simulation开关，不seed人工批准/执行facts。该harness需要新的专用router/API数据库，不能借正常API或包测试库。
- 新源依赖最终须传到2d所有真实writer。新增可选字段不能改变旧code-review/QA原请求摘要；已有5个nullable字段的原键回执语义保持。

服务端继续单owner；客户端可以立即按已冻结8.10并行实现Core/Views/Web/Desktop共享接入。

## 6. E2E已编写与检查，实际场景仍为0次

最新阅读`joint-e2e-increment-2-results.*`与**`joint-e2e-increment-2-check.*`**。检查报告已完整落盘，虽然代理随后因路由错误停止。

覆盖编写：角色/建议/人工child/PR-CI/评审/原键恢复/撤销/成员移除重入；A05真实本地Git commit/bundle、裸Host不可用、typed C1→CLI subject/submit/receipt→T4、失败报告与失败执行、QA退回、显式新运行成功、保留旧失败。尚缺后续release/rollback/M1/admin完整场景。

5个相关文件：

- `e2e/collaboration-stage-approvals.spec.ts`
- `e2e/fixtures/collaboration-stage-approvals.ts`
- `e2e/fixtures/collaboration-host-runtime.ts`
- `e2e/fixtures/collaboration-host-provider.go`
- `e2e/fixtures/collaboration-host-provider_test.go`

独立检查已修Git/profile原始路径泄漏和beforeTrigger无deadline/缺失败证据；保留原失败。Provider race31、Host cleanup4、源提取7及类型/lint通过，Playwright只发现一个场景，测试体0。

实际运行约束：

- 用户已经决定完成2b–2d及客户端后统一跑完整A01–A12，不先盘点或分批冒充总验收。
- 使用最终源码新构建的API/CLI、production Web、匹配Desktop及支持新能力握手的**base Host provider**，不能混用closure provider；显式`COLLABORATION_STAGE_GIT`。
- source-only null artifact不证明构建；不能为通过release临时填假artifact。
- Git/代码/构建/输出/配置需新绑定；保留原失败和完整分母，输出与HOME/TMP放任务外部目录，不覆盖他人`.next`。
- 合成新账号邮箱桥接仅改匹配ID/用户名且原邮箱NULL的账号，成员关系仍走真实invite/accept/remove/rejoin。API明确清空`RESEND_API_KEY`、`SMTP_HOST`。
- 合成S2 API禁用真实GitHub刷新凭据（APP_ID/PRIVATE_KEY等），避免远程调用或覆盖合成CI；S4真实集成另验。
- 复用S5 owner-scoped心跳与10秒availability读取，150秒/5分钟/3分钟产品/夹具限制未改。
- 收尾停止自己拥有的进程，恢复分配时gate布尔原值，epoch只递增，保留DB/证据。

## 7. 数据库与私有工具

私有根：`/Users/artisan/.multica/acceptance/pay241-20261113-01`；续接证据根：`/Users/artisan/.multica/acceptance/pay241-20261114-resume`。

`run.mjs <configs/label.json> <command...>`先校验实际数据库身份，勿打印配置内容。

| 配置label | 许可 | 当前迁移 |
| --- | --- | --- |
| s2_impl_service | `./internal/service` normal | 828 |
| s2_impl_handler | `./internal/handler` normal | 828 |
| s2_race_service | `./internal/service -race` | 828 |
| s2_race_handler | `./internal/handler -race` | 828 |
| s2_2a_migrate_r01 | `./cmd/migrate`，有保留的合成2a候选 | 825 |
| s2_2a_ghsnapshot_r01 | `./internal/integrations/ghsnapshot`，42 named race通过 | 825 |

迁移由主会话做。新的migration先冻结DDL/sqlc/可编译状态，校验身份、无在用测试连接，再逐库应用；不让子代理自行切库/迁移。无FK，每个CONCURRENTLY index单独文件并注册失败重试清理，最高down guard在索引删除前拒绝保留历史。

最近迁移证据：`s2-inc2b-ddl-before.json`、`s2-inc2b-migrations.json`；二进制`s2-migrate-inc2b-828`；冻结826–828目录`s2-inc2b-migrations/migrations`。四库已核对索引结构/valid/ready/live/unique和无FK。

无DB纯测试：

- `run-pure-s2.mjs`：protocol、internal/cli、cmd/multica。
- `run-pure-s2-host.mjs`：`./internal/daemon -run '^TestCollaborationHostHarness'`及`./internal/daemon/collaborationrun -run '^TestHostBrief'`。

两个脚本都在续接证据根；各包外部HOME/TMP，明确Go缓存，不继承账号/DB环境。HostBrief会启动显式测试Provider，不能称为纯静态检查。其他DB包及真实API/harness须主会话新分配，不能随意借库。共享PG max_connections为100，不调高连接池/超时/清理规则掩盖失败。

## 8. S3、S4、S5和收尾

- S3复用已归档timeout-proof；余下附件幂等、迟到结果、55评论跨页补读、优先级/处理人并发、混合提及通知去重、中途撤权及阻塞四字段，等S2共享合同稳定后实施。
- S4已有`research/integration-readiness.*`。真实支付/履约沙箱、真实PR/CI、非生产发布/回滚目标/指标和审核人仍缺，E01–E07 NOT_RUN。不要把Cloud Stripe billing当PAY支付系统。
- S5工具修复和r02演练已提交`8e824b303`，31工具测试、79独立证据检查。100账号/10000传播/100次正式回写完整；pooled p95 702.579ms，业务回写130.694ms、终结724.774ms。来源是S2前`fb9f4c070...`，不是最终候选。r01失败与完整分母保留；r02资源/gate已恢复。**TC-24仍NOT_RUN，S5未归档。**最终候选需S2–S4稳定且无同机高负载构建时重跑。
- 父矩阵仍4 PARTIAL、20 NOT_RUN（21P0/3P1），不是代码完成比例。局部绿灯不能关闭父任务；实际业务验收和人签字缺项照实保留，测试账号签字明确标模拟。
- 全局migration cleanup检查仍有8个既有F2 report_snapshot缺项；S2的16项已修复。不要改全局断言隐藏失败，也不混入S2提交。
- 新规范`.trellis/spec/server/collaboration-stage-approvals.md`及server/Core索引行已写。server索引有他任务原有dirty hunk，提交只选本阶段部分。
- 用户允许本地Lore提交，禁止`git add -A`。只暂存明确属于本阶段的文件/hunk。自身验收通过后才归档子任务，不把规划/静态检查/模拟结果当完整业务通过。

新会话开场应简短报告核对后的真实进度、当前owner和最近可交付增量，然后直接修S2B-C01并接续2c→2d→联合验收→S3/S4/S5；无需询问“是否继续”。
