# F2 最小首包验收

结果：已完成本线程明确授权的F2最小首包实现与确定性验证。任务仍为`in_progress`；M1-S2–S6/F3/F4/F5-A–E继续待办，不归档roadmap。M1-S1实现/验收记录未重做。

## 交付行为

- 当前有权的人显式采集；scope/timezone/[start,end)/as_of、解析日历、语义版本、原生/内容来源身份、最小事实、结果和独立分页审计一起封存。
- formal_stock先行，business分类与历史不足保持unknown；已知单项缺口不丢失E/U分母，未知尾部不折0。来源错误只影响依赖指标，创建事实不依赖状态活动。project_reasons与健康规则一起从安全事实重放。
- Reporter仅exact当前报告/frozen project的current/by-ID GET。公共投影不包含creator/request key/read_cut、origin/目录/运行时/roster身份或私有审计；行级seal_digest绑定内部、公共和audit，拒绝自洽但错配的替换。
- 每次在同一RR读取当前身份/任务/claim/runtime/principals、report/project/qualification/control与实际内容；前置撤权拒绝无正文，commit失败不释放缓冲内容，所有分支no-store且不刷新wildcard CDN cookie。
- 人工capture receipt保留原key/规范意图绑定；原请求恢复不重采，correction是新意图。CLI本地拒绝mat_/mdt_/mcn_写请求，并独立校验公共投影和human-only收据。
- 用户选定的“本报告全部快照”撤回是终止状态：旧ID/current均不可读，该report不能再采集/修正，无restore；原withdrawal请求可恢复最小收据。共享control行实际更新，防止旧RR越过撤回。源普通删除/移出仍保留旧事实；owning report/workspace删除显式清理。

## 最终验证结果

| 范围 | 顶层用例 / 含子用例结果 | 结果与证据 |
| --- | --- | --- |
| handler normal | 17 / 30 | 全通过；`f2-test-environments/f2-handler-final-02.json` |
| handler race | 17 / 30 | 全通过；`f2-test-environments/f2-handler-race-01.json` |
| real Auth/router normal | 6 / 42 | 全通过；`f2-test-environments/f2-router-final-01.json` |
| real Auth/router race | 6 / 42 | 全通过；`f2-test-environments/f2-router-race-01.json` |
| engine/projecthealth race | 38 / 53 | 全通过；`f2-final-evidence-20261113/final-checks.json` |
| private source audit race | 3 / 9 | 全通过；`f2-final-evidence-20261113/pure-results.json` |
| middleware | 7 / 17 | 全通过；同上 |
| CLI全包（最终公共schema） | 487 / 1,283 | 全通过；`f2-final-evidence-20261113/final-checks.json` |
| builtin合同（修正后） | 5 / 25 | 全通过；`f2-final-evidence-20261113/builtin-fixed-result.json` |
| 测试启动器 | 14项纯Python回归 | 全通过；`f2-test-runner_test.py`，未连接DB |
| 受影响Go build/vet | 七个相关package | 全通过；`f2-final-evidence-20261113/final-checks.json` |
| schema与迁移 | 794–804 | 两表0个FK、8个有效唯一索引；新增索引均独立CONCURRENTLY；`f2-final-evidence-20261113/schema-check.json` |

上述最终运行零失败、零skip；不同运行不能相加为互不重复的产品用例数。DB四次各用新建、迁移完成、实际身份匹配的专属数据库；先前失败数据库与日志保留。CLI/纯测试使用私有HOME、无DATABASE_URL及agent CLI guard。Go为1.27.0 darwin/arm64，PG为17.11。

## A01–A16 对应证据

| 验收组 | 主要证明位置/命名用例 |
| --- | --- |
| A01/A02 不可变/日历 | public replay及internal legacy identity、DST/saved offsets、project reasons摘要；engine race日志 |
| A03/A04 分页/cut | `TestF2ReportSnapshotActivityScanBeyondTimelineCap`（2,101条）；`RetainedPagesKeepOneCutAndConfirmEOF`（2,000条+第三次EOF、较早tuple迟commit） |
| A05 缺历史/迟到 | 同cut/新correction对照；production history/classification unknown与source audit负例；不将read_complete当producer_complete |
| A06 生命周期 | pure lifecycle/half-open/terminal-switch、window birth独立证据；真实源能力不足仍unknown |
| A07/A08 范围/报告排除 | capture/replay/source冻结、report资格/项目first-read barrier；pure分类/报告身份与未知业务；来源proof和audit验证 |
| A09 coverage/来源缺失 | `KnownCandidateGapPreservesCoverageDenominator`、`SourceGapAndFailedSeal`、`ReportOnlyGapKeepsSourceUnknown`、`UpdateGapDoesNotDiscardProjectFacts` |
| A10/A11 重读/缺失/损坏 | `CaptureReplayCorrectionAndFrozenSources`、`RequiresBoundStoredPublicProjection`、`RowSealBindsPublicAndCollectionAudit`、`OwnedDeleteCleanup`及CLI严格解码 |
| A12 只读parity | health原规则golden/ID范围说明、safe-fact计算及project_reasons用例；没有发布或更正人工update来取样 |
| A13/A14 精确权限/竞争 | `PublicProjectionAndHumanReceipt`、`ProductionReporterExactGetMatrix`、`FirstReadAuthorityBarrier`、`ReadCommitFailureWithholdsPublicBody` |
| A15 原请求/取消/撤回 | concurrent original request/withdrawal、terminal recovery、both correction/withdraw commit orders、cancel不重启/live serialization才重启、CLI response-loss |
| A16 零业务/凭据副作用 | report revision/comment/queue相关断言、existing inert comment控制、middleware无cookie、CLI机器凭据零HTTP、拒绝与commit failure零正文 |

完整expected顶层清单为`f2-handler-expected-tests.json`及`f2-router-expected-tests.json`，每个原始JSONL/分配记录保存实际run/pass/fail/skip与源码摘要。

原始JSONL在当前工作副本保留；提交中采用同名`.jsonl.gz`，以减少审阅噪声。`f2-final-evidence-20261113/log-archive-manifest.json`列出原始/压缩路径及两份SHA-256；已逐个验证解压后逐字节一致，包括失败日志。需要读取时解压对应文件，不能将压缩文件本身的hash与原日志hash混用。

## 独立复核及保留的失败

独立trellis-check无遗留阻断，确认修复：公共/内部/audit错配、已知单项gap分母、created被activity gap污染、mcn_本地写拦截、遗漏project_reasons。额外纯回归6个顶层/10项通过；该复核未用编译冒充DB运行。

初次`f2-handler-final-01`只失败`SourceGapAndFailedSeal`。fixture仅含被正面排除的当前report；活动源确为ReadComplete=false/Count=null/gap，但业务集合可证实为空，metric Complete=true合理。修正测试前提并拆开源未知/指标完整断言，新增report-only反例；没有改生产语义求绿。原失败结果28 pass/1 fail保留。

初次builtin合同失败是working-on-issues body545行超过500。F2细节移到单层`references/report-snapshots.md`，主文保留入口，M1/其他指导不删改；最终body500行、5/25通过。原`builtin-contracts.jsonl`及失败结果保留。

## 源码身份与并行工作

验证时HEAD为c79dfe304，实际执行的是记录了哈希的F2工作副本；不能只用HEAD描述源码。四个成功DB运行内的83项来源/测试/工具摘要前后一致。之后仅移动了两份builtin Markdown中的F2详情并增加支持文档，已独立运行相关builtin合同；Go/SQL/测试/运行脚本未变化。

`f2-final-evidence-20261113/runtime-run-summary.json`明确列出这项文档差异，`final-source-sha256.json`记录最终84项清单。清单用于验证身份，不表示其中所有预存hunk都归本会话。另一个任务的collaboration comments及文档改动保留，不纳入F2提交或验收结论。

共享`verification-environment.md`在本工作副本原本是另一并行任务的未跟踪规范，本次只读取，没有纳入提交或接管它；既有task上下文引用在当前工作副本验证通过。该并行规范的独立提交仍由其owner负责。

## 限制与后续

- 真实activity无完整producer/事件时项目/类别历史，业务统计可能unknown；这是用户批准的首包，不宣称完整历史体系。
- 旧未带公共投影/control/seal绑定的草稿行明确不可用，没有自动升级或GET回填。
- 撤回只终止本报告内容的应用读取，不是跨报告来源治理或物理备份清除；RR后已交付字节不可回收。
- 已验证功能边界与有界解码/分页，不宣称生产容量SLO、压力基准或长期运行表现。
- 未调用真实Provider、操作POLA-1/共享项目策略、发布/部署/Hosted CI；未改前端，因此未重跑M1 Views/renderer或做新UI验收。
- M1-S2–S6/F3/F4/F5-A–E与独立真实运行任务仍待办；本roadmap不标完成或归档。
