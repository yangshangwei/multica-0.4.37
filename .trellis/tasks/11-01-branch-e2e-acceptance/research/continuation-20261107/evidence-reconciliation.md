# 2026-11-07 验收证据衔接复核

**旧版 15 项失败与 2 项阻塞均找到后续成功原始记录，相关修复已进入冻结候选；本复核没有执行当前候选测试，不构成当前分支验收通过。**

本次目标：`4c817dee9397a42ac903f419af7804b11d2d863e`。初次观察为 `514e89752`，准备期间分支新增了 `4431f89d6`（协作详情聚焦/操作弹框）与 `ba5b4bdd2`（辅助内容抽屉）；主会话已冻结独立工作树。最新抽屉任务仅声明原生交互和单元证据，明确未声明生产 Web 或完整 E2E 链路通过。

机器可读明细：[evidence-reconciliation.json](evidence-reconciliation.json)，包括原始用例键、标题映射、每项 raw JSON 的 SHA-256/时间/终态、修复 ancestry、当前 spec 行号及运行限制。

## 计数与历史边界

- 原 `coverage.csv` 是 437 项：421 PASS / 14 FAIL / 2 BLOCKED。ENV-1 在 `summary.json` 单列，原总报告因此为 438 项：421 PASS / 15 FAIL / 2 BLOCKED；这是两种库存表达，不是缺失一个已有 CSV 行。
- 按文件与完整标题匹配，逐项读取 11-04 core 的 17 个选中原始终态，均 `passed`、单次结果、`retry=0`；11-02 functional 的相同 17 项也逐项核对通过。没有仅凭汇总 PASS 接受。
- core 证据 16 项属于 `1a32ca36c`；Linux 完整 Main 一项属于 `1a32 + daemon FD 修复`、CLI SHA 前缀 `8ad7a9ac`。functional 证据属于 `7af9ab990` 与各自登记的测试适配，不是当前源码。
- 原失败、首次全量失败及后续成功分开保留。`1a32` 到新候选的 apps/packages/server/e2e/scripts 有 231 个变更文件，不能将历史通过移植为本轮通过。

## 原 17 项的当前入口

以下当前结果统一为 **待本轮执行**；“历史 PASS”仅指上述已核对的实际原始记录。两项 retained-attachment 标题由 Web/Electron 参数生成，不能通过源码整句搜索推定不存在。

| 原用例键 | 原状态 | 当前入口 | 历史结果 / 修复 |
| --- | --- | --- | --- |
| `3dccc2100422` | FAIL | `collaboration-account-desktop.spec.ts:402` — Desktop child links / records / restart | PASS；1ecabb964 已核对 |
| `c919808d1d26` | FAIL | `collaboration-account-desktop.spec.ts:544` — Desktop authors initial fields | PASS；1ecabb964 已核对 |
| `5a60ce5f71f0` | FAIL | `collaboration-boundaries-web.spec.ts:450` — Smaller gaps / exact recipient resolver stays read-only | PASS；1ecabb964 已核对 |
| `158a87f90c8f` | FAIL | `collaboration-delivery-web.spec.ts:23` — Web lost upload/create responses / preview revoke | PASS；1ecabb964 已核对 |
| `72c1ec89998d` | FAIL | `collaboration-delivery-web.spec.ts:194` — Web rejected creation resubmission | PASS；1ecabb964 已核对 |
| `187ee1881c81` | FAIL | `collaboration-editing-web.spec.ts:9` — Web source creation / edit / discussion | PASS；1ecabb964 已核对 |
| `860dfeffcf00` | BLOCKED | `collaboration-installed-clients.spec.ts:29` — cached v0.6.7 CLI refused unsafe controls | PASS；此前缺少保留 CLI 前置 已核对 |
| `8b009b887e43` | BLOCKED | `collaboration-installed-main.spec.ts:143` — Linux full Main / Web handoff / Resume | PASS；76836fa8d 已核对 |
| `535e317ad170` | FAIL | `collaboration-leave-web.spec.ts:14` — Web Leave / revoke / stop / rejoin | PASS；1ecabb964, 76836fa8d, 4431f89d6 已核对 |
| `4971c07b93b6` | FAIL | `collaboration-mention-journeys-desktop.spec.ts:102` — Desktop owner revoke / original-key replay | PASS；1ecabb964 已核对 |
| `8e9c882cd544` | FAIL | `collaboration-mention-journeys-web.spec.ts:44` — Web owner revoke / original-key replay | PASS；1ecabb964 已核对 |
| `cd80f1b61840` | FAIL | `collaboration-records-web.spec.ts:7` — Web child references / recorded pages | PASS；1ecabb964 已核对 |
| `f390d53c1162` | FAIL | `collaboration-retained-attachment.spec.ts（动态标题）` — Web typed creation / private upload / restart | PASS；1ecabb964, 76836fa8d 已核对 |
| `18e4a6485b4d` | FAIL | `collaboration-retained-attachment.spec.ts（动态标题）` — Electron typed creation / private upload / restart | PASS；1ecabb964, 76836fa8d 已核对 |
| `37ff666368df` | FAIL | `collaboration-retained-attachment.spec.ts:156` — Web + Electron retained file bytes / immutable AI input | PASS；1ecabb964, 76836fa8d 已核对 |
| `7b935d7ce8d5` | FAIL | `collaboration-source-surface.spec.ts:11` — Web + Electron source list / board / timeline | PASS；1ecabb964 已核对 |
| `QA-PRODUCT-001/ENV-1` | FAIL | `collaboration-runtime-handoff-web.spec.ts:9` — ENV-1 startup v0 / live latest v1 | PASS；1eb0cb33c, 9e7f18785, 1ecabb964 已核对 |

通用维护提交 `1ecabb964` 复用现有精确收件人选择器，只排除每次读取变化的 `author_authority.evaluated_at` 并验证其有效时间；其余业务事实完整比较。记录分页改为验证具体记录项、50+12 条 API 分页与唯一 ID，保留 handoff 对同一结果的合法显示。边界用例标题明确从“recipient lookup is exact-only”改为“the exact recipient resolver stays read-only”，原完整断言没有被删除。

ENV-1 产品修复 `1eb0cb33c` 已经 `9e7f18785` 集成；`memory.base_version` 与 `memory.version` 分开，UI 分别显示启动版本与最新版本。推荐运行当前正式 `collaboration-runtime-handoff-web.spec.ts`，而不是复用旧私有 template：现有测试同时证明 DB base0、API latest1/base0、空启动文案、Latest v1 链接及错误启动文案缺席。此用例仍是 seed retained run 后通过真实 API/UI 取证，不证明 Provider 实际摄取。

管理筛选单元门 `QA-TEST-001` 由 `9c056ad68` 固定 Date 到 2026-10-02，并保留真实 timers 与 31 天产品限制；每例结束恢复时钟。当前建议单独跑 `pnpm --filter @multica/views exec vitest run admin/executions/list-controls.test.tsx`。

## 最小本轮复验分组

每组均要求新候选来源绑定、生产 Web、实际认证开关、1 worker / 0 retry；选中标题见 JSON，整 spec 补验可提供相邻链路覆盖。

1. **Legacy Web / 原生 renderer（9 项）**：`collaboration-account-desktop` 两项、`collaboration-boundaries-web` Smaller gaps、`collaboration-delivery-web` 两项、`collaboration-editing-web` 一项、`collaboration-mention-journeys-{web,desktop}` 各一项、`collaboration-records-web` 一项。
2. **Password（6 项）**：`collaboration-leave-web`、`collaboration-retained-attachment` 三项、`collaboration-source-surface`、`collaboration-runtime-handoff-web` ENV-1。保留 same-source renderer、独立账户/专库及 `COLLABORATION_SOURCE_CLI` 等实际前置；避免前置 429 被解释成业务失败。
3. **安装/兼容条件（2 项）**：`collaboration-installed-clients` 需要原 v0.6.7 CLI 与当前服务；`collaboration-installed-main` 需要当前候选 Linux 完整包和受控运行镜像。不能用旧包或未配置导致的 SKIP 计作通过。
4. **最新 UI 增量**：至少补 `collaboration-memory-web`、`collaboration-workbench-web`、`collaboration-handoff-chain-web`，以及辅助抽屉里的附件、知识、参与者/活动入口。现有 disclosure helper 已适配新入口；旧快照成功不覆盖新抽屉。

以上只处理旧阻断和最新 UI 风险；完整分支验收还需要主会话的核心任务/权限/恢复/数据持久性覆盖，不能把 17 项小集写成整分支全绿。

## Runtime 9 / Knowledge 21 衔接

| 集合 | 已证实历史状态 | 集成 / 本轮含义 |
| --- | --- | --- |
| Runtime 9 | REC-1/2/3/4、ENV-1/2/4、RECALL-1/2 修复与独立审查已完成；9 条 canonical Go 入口均仍存在 | `4456cc65a`、`1eb0cb33c`、`5b9f66292`、`e1e6090eb`、`7627916c8` 与集成 `9e7f18785` 均是本候选祖先 |
| Runtime 后续确定性 | `1a32` quiet02 normal 62 PASS / race03 62 PASS，独立核验 912/915；password7 完整 7 项后续通过 | 旧 60/2 和 quiet02 race 重叠局限仍保留；不把真实 H3a 超时或模型理解归并为当前产品缺陷 |
| Knowledge 21 | #7 在 `eda0d68fb` 独立 PASS / 0 blockers，AC1–AC9 及 21 处置全部接受并归档；19 项有旧基线红证据，L7 未复现处置、L13 术语 parity 例外 | 集成 `6cb9fcb36` 与验收源码均为本候选祖先；旧报告“21 项未关闭”只适用于其旧冻结版本 |
| Knowledge 执行链路 | 共享 V16 和 CCG1 normal/race、CCG2 OFF/ON 已独立接受，证明受控 fake 输入/权限链路 | 不代表真实商业模型成功，也不替代本候选新运行 |

原 runtime repair-ledger 条目仍写 `implemented_awaiting_independent_review`，但同目录后续独审、集成提交与 11-05 当前确定性终审都已存在；需按时间和源码衔接，不能用旧条目覆盖后续终审，也不能用 ancestry 代替重跑。具体每条修复、测试路径及证据哈希见 JSON。

## 仍开放的边界

- 当前候选 E2E/静态/单元/必要集成门由执行负责人实际运行后才能关闭，本报告全部标为未运行。
- 原生 Windows / iOS / Linux desktop、签名公证、自动更新与真实认证升级、实际外部渠道仍需各自 owned runner/构件；容器包或 renderer 测试不能互相替代。
- 本任务商业 Provider 调用维持 0；其它任务 runtime 的 12/25、知识的 2/17 和停止规则不是本任务的授权。真实系列的未完成/403/超时不得写成这里已关闭。
- `1a32` 的 2155.025ms 性能例外仅属于该候选和样本；当前性能门如需执行必须保留原 2000ms 判据，没有通用放宽。

## 本复核验证

只新增本目录两个报告，未改产品、旧报告/CSV、配置、数据库或运行进程；没有 Provider 调用。已完成 JSON 可解析、17 项身份一一对应、17+17 个历史 raw 终态/零重试、全部修复 ancestry、9 个 runtime canonical test 存在检查。Lint/typecheck/当前测试未在本只读证据切片执行，交由主会话执行门记录。
