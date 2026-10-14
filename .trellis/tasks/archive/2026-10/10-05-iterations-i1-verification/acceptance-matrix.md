# I1 最终验收矩阵

2026-10-07，分支 `codex/projects-p1`，基于 `115b4cd28` 的未提交工作区。
29 条 I1 功能要求均有核验依据。UG/FCG 的最终判定由 `verification.md` 汇总；远端 CI、提交、推送、合并、部署均不冒称通过。原先“下一步 CG／客户端未开发”的交接已过时。

## 证据索引与源码范围

- **B**：`.omx/logs/i1-verification/full-check-accepted.log`。10,215 TS tests、15/15 lint/typecheck、全 Go race/vet、生产构建、249 E2E pass /49 skip。原会话10:38启动晚于最后五处修正；接手时其余6,232个快照文件一致。
- **H**：[迁移与历史补证](migration-recovery-evidence.md)及 JSON。新 migration2顶层/4子项、History15顶层/21 pass事件，race无失败/跳过；AC12/13真实写入增强。
- **U**：[完整 UI 审查](ui-final-audit.md)。最终受影响views100 tests、typecheck/lint；生产Web+真实Electron四项E2E在最终代码上4 pass /0 skip /0 retry，24.2s。最终截图/视觉判定由该文档引用。
- **M**：[移动兼容](mobile-final-compatibility.md)。196 tests+iOS脚本、typecheck、lint；只承诺类型／请求和受支持客户端提示，未做真机视觉。
- **P**：[性能原始样本](performance-evidence.json)及[普通写入样本](ordinary-write-evidence.json)。冻结阈值未调高，重新核算后仍通过。
- **FG/LG/HG/CG**：父任务和各子任务原有真实验收记录，保留而不重跑。
- [源码与运行证据](source-provenance.json)：保留全量基线与13项后续源／测试增量的分界，最终6,696个运行／测试文件无漂移。各轮统计相互重叠，不能相加成独立用例数。

下表省略测试函数的 `TestIteration` 共同前缀；[后端审计](backend-final-audit.md)给出每行完整函数、文件和实际断言。标为组合证据的行明确组合服务端不变量、真实路径及客户端接线，不伪称每条都有独立浏览器场景。

| ITR-AC | 验收行为 | 当前证据 | 结果／执行 |
| --- | --- | --- | --- |
| 01 | 启用确认时区，不创建周期或移动任务 | SettingsDefaultAndReleaseGate / EnableIterationSettingsValidatesCurrentRoleAndTimezone；U 中启用入口 | 已验证；B + U |
| 02 | member 不能改默认时区／禁用 | ProjectPlanningTimezoneContractAndHumanPermissions / ClosureDisableAdminLimitAndRollback | 已验证；B |
| 03 | 三个未来计划保留 planned | IterationListKeysetAndScopeBinding + LifecycleCreateReplayAndSavedTimezone；创建路径仅保存 planned（组合证据） | 已验证；B + LG |
| 04 | 并发开始仅一个 active | LifecycleCompetingStartsOnlyOneActive；真实唯一索引两连接 | 已验证；B + LG |
| 05 | 逾期只提醒，执行和生命周期不变 | IterationOverdueLocalDayAndFallback；跨本地午夜比较完整运行任务 | 已验证；B + CG |
| 06 | 待分拣等非正式任务跨入口拒绝 | IssueIterationAssignmentTerminalAdmissionAndHistory + LifecycleNoopAndNewActiveAdmission + iteration_confirmation_test.go | 已验证；B + LG |
| 07 | 安排／移出保留项目、负责人、状态与执行 | LifecycleManagementPreservesRunningExecutionAndProject + ClosureManagementPreservesRunningTask | 已验证；B + CG |
| 08 | 换期只有一个当前归属，双边日志 | IterationMembershipEventsFreezeSourceAndTarget + LifecycleBatchSwappedSources | 已验证；B |
| 09 | 拒绝补录到历史 | IssueIterationAssignmentTerminalAdmissionAndHistory；共享 source/target open-status 分支审查（组合证据） | 已验证；B + LG |
| 10 | 八项原始承诺不变，新增两项 | HistoryCanonicalThroughLifecycleAndIssueWriters：真实 start A–H，再 join I/J | 已验证；B + H |
| 11 | A–J：9/8/5、62.5%／50% | 同一真实 canonical fixture，原始完成4；pure stats 为公式边界 | 已验证；B + H |
| 12 | 移出后在别处完成不提高原始完成 | F 经真实 move 移出，再普通 HTTP done；D5/OD4、源事件数不变 | 已验证；H（本轮补齐） |
| 13 | 真实父子各计一次，说明非工作量 | B 的 parent_issue_id=A；original/scope 内各一次；UI count-not-workload 说明 | 已验证；H + U（本轮补齐） |
| 14 | 空／全取消有效完成率不适用 | StatisticsEmptyCancelledNoOpsAndReopen / StatisticsDeletionPreservesOriginalAndNullRatiosJSON | 已验证；B + HG |
| 15 | 只有未完成集合可结转，终态释放 | ClosureRejectsMissingAndTerminalMoves / CompletionAfterPreviewAndTerminalRelease；集合规则与实际去向组合 | 已验证；B + CG + U |
| 16 | 次数1→2，重试不再增长 | 真实 P95 循环每次断言 sample+1；LostCommitResponseRecoversOriginalOperation / RollbackReplayAndFrozenDigest（组合证据） | 已验证；P + B + CG |
| 17 | 目标取消后整体冲突，不部分移动 | ClosureTargetCancelRace + RealIssueAndProjectWriterRaces | 已验证；B + CG |
| 18 | 已提交响应丢失仍同一结果 | ClosureHTTPRecoversLostCommitResponse；原生 Electron 实际丢响应、原ID查询恢复 | 已验证；B + CG + U |
| 19 | 结束不停止／重启执行 | ClosureManagementPreservesRunningTask / DaemonCompletionBarrierDoesNotRestartExecution；真实 TaskService，fake 执行 fixture | 已验证；B + CG |
| 20 | 重开／删除／换项目不漂移历史 | ClosureRealPostClosureWritersPreserveSnapshot；真实P1删除、标签与优先级变更；浏览器历史对比 | 已验证；B + CG + H + U |
| 21 | 用过的空计划不能删除 | LifecycleUsedPlanCannotDeleteAndCancelReleasesAll；纯改名未用计划可以 delete/replay | 已验证；B + LG |
| 22 | 整体禁用：当前结束、未来取消、释放全部 | ClosureCancelAndDisable（103计划）；51计划/1,530任务完整禁用；真实UI交接后禁用 | 已验证；B + CG + P + U |
| 23 | 禁用冲突整体保持原状 | ClosureDisableAdminLimitAndRollback；真实 writer-first disable409、enabled保留 | 已验证；B + CG |
| 24 | 默认时区只影响新计划 | LifecycleCreateReplayAndSavedTimezone + P1共享时区API + create捕获当前时区（组合证据） | 已验证；B + LG |
| 25 | 纯日期一致、保存时区明确 | calendar/DST/午夜canonical；UI直接显示日期并按保存时区格式化实际时间；Shanghai 18:05Z→02:05 mounted证明 | 已验证；B + U |
| 26 | 旧请求编辑保留未知迭代字段 | IssueIterationOmittedFieldsPreserveExistingMembership；core response schemas；实际mobile序列化 | 已验证；B + M |
| 27 | 撤权后历史、通知与缓存不泄露 | 真实读/撤权竞态、outbox tombstones；core epoch+迟到响应保护、mounted清空；附件沿用既有授权边界 | 已验证；B + CG + U |
| 28 | 预览后done不能按旧事实结转 | ClosureRealIssueAndProjectWriterRaces 的 done→end/disable 双锁序；CompletionAfterPreviewAndTerminalRelease | 已验证；B + CG |
| 29 | 空基线不适用，后续新增单独计数 | LifecycleClockMidnightAndEmptyBaseline + FirstActiveJoinAfterPlannedLeaveAndStartedReset + null公式（组合证据） | 已验证；B + LG |

## 原始 PRD §§6–8 及额外体验要求

[UI审查](ui-final-audit.md)逐项列出全部界面要求及源码／断言，不把29行场景视为全部产品要求。重点复核了：

| 要求 | 最终实现与证据 |
| --- | --- |
| §6.1 导航／分组／历史搜索分页 | 当前和未来独立于历史分页，最早未来标签、空态、搜索／日期／状态／直链；超过50历史项回归 |
| §6.2 详情／五种筛选分组 | 标题完整字段、协调人／模式／复制链接；按真实状态、负责人、项目、优先级、标签对完整集合筛选分组；61项浏览器场景 |
| §6.2 指标／图表／历史 | 全期口径不随筛选变化；两种完成率、非工作量说明、同值表格；冻结摘要与实时对比独立 |
| §6.3 创建／详情／批量／项目／设置 | 显式当前归属与历史参与；精确批量集合；项目过滤、T1准入；设置补齐手动模式说明 |
| ITR-007/010/012 | 终态选择使用可读任务名；范围事件显示冻结任务身份；预览按保存时区显示业务时间；全部去向与执行说明保持完整 |
| §8 日期与状态限制 | 纯日期不漂移；锁定开始日／保存时区；历史元数据更正不改变快照；状态与动作逐项审查 |
| ITR-032/033/034 | 原ID恢复、403/qualified404撤权、409保留输入、键盘、长名、完整分页；mobile无完整编辑器但点击通知明确提示支持版本 |
| ITR-035 | 1,000项真实detail/preview/closure、51计划完整disable；无静默截断／N+1；超量明确413 |

## 迁移、兼容性和跳过项

真实550–566部分执行／INVALID索引恢复、无hook负控制、catalog与ledger幂等、部分空回退、used回退拒绝均已补证；原有P1共享时区、全部17步down及并发writer保护沿用B。演练只在新建独占库及私有schema进行，该库已确认所有权后删除。

兼容性由真实旧格式HTTP/Plugin请求、schema/wire fixtures、mobile请求序列化与受支持版本提示证明；没有安装历史Desktop/Mobile/CLI二进制。已有I1数据的数据库不能退回不记录I1事实的旧server。关闭开关不等于可以破坏性down。

[49个跳过项](skipped-tests.md)全部有具体测试和环境条件；I1跳过为0。这些独立密码／管理／AI provider／feed／其他Desktop／CLI fixture未运行，不能标PASS。远端CI不在本轮本地执行范围。

## 性能与保留证据限制

| 操作 | warm P95 | 冻结上限 |
| --- | ---: | ---: |
| 1,000项详情 | 57.992ms | 2,000ms |
| 完整结束预览 | 324.154ms | 2,000ms |
| 实际结束结转 | 1,099.838ms | 2,000ms |
| 1,530项禁用预览／提交 | 459.041 /957.511ms | 各3,000ms |

普通四写者关联任务中位吞吐426.3/s，P95 12.10ms；相对和绝对冻结门槛均复算通过。性能路径未受本轮UI／测试改动影响。它们是指定本机warm fixture结果，不是冷盘／生产网络／并发标签写SLA。

旧`/tmp` CG和普通写入原日志已过期：CG由保留的最新全量race重新覆盖；普通写入JSON中12组原始样本及阈值仍可复算。保留原摘要，但不声称本轮重新hash了不存在的文件。
