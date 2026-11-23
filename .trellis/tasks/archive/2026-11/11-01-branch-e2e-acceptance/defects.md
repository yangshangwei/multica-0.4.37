# 缺陷与失败清单

结论：P0 验收未全通过。产品、测试、环境、稳定性分开记录；原日志均保留。

## QA-PRODUCT-001：启动 memory 归因错误

# Confirmed product defect: ENV-1

- ID: QA-PRODUCT-001 (existing runtime audit ENV-1). Priority: P0 acceptance item; severity: medium (S2), consistent with the existing audit.
- Target: 822c15785449b6c1cef699f6fd4d8e1f5483bd5e; production Chromium/Web 13930, password API 19010; dedicated task database.
- Prerequisite: retained supported run with immutable `memory_base_version=0`; authorized creator and external recipient. Run rows are seeded by the existing retained-history fixture.
- Reproduction: (1) run the isolated `memory-startup.spec.ts`; (2) create/transfer the shared task; (3) retain a supported run started at v0; (4) PUT shared memory with `expected_version=0` to produce v1; (5) sign in as recipient and open this run's Handoff context panel.
- Expected: latest live memory is identifiable as current context, while this run's startup attribution remains v0/empty. Never label v1 as the version this run started from.
- Actual: DB base remains 0; the live handoff response reports memory v1 and the UI renders the link `Started from v1`.
- Requests: authenticated `PUT /api/collaboration/issues/:id/memory`, `GET /api/collaboration/issues/:id/runs/:runId/handoff?execution_view_version=6`; no credentials are included in this report.
- Evidence: `evidence/ENV1/ENV-1-api-and-persisted-facts.json`, `evidence/ENV1/ENV-1-live-handoff.png`, `runs/ENV1-startup-memory-first/results.json`, original failure screenshot and error context.
- Impact: the UI attributes a later decision to the run's startup input, compromising human handoff and audit interpretation. This reproduction does not show that the actual Provider startup input changed.
- Recommendation: expose the immutable startup memory reference separately from live/current memory and bind the startup label to that retained field. Re-run this invariant after the pending integration repair, plus Web/native handoff and shared-memory conflict regression.
- State: OPEN on the frozen version. No product fix applied; pending other-worktree repair does not close this result.
- Coverage limit: real API, persistent base fact and production UI are verified; seeded history does not prove real process ingestion.

## QA-TEST-001：管理筛选测试固定日期过期

严重度：测试门禁 S2。前置：系统时间 2026-11-01/02；起始时间固定 2026-10-01、无 time_to。复现：在固定版本运行 `packages/views/admin/executions/list-controls.test.tsx`。预期：测试合法 queued 历史筛选时可导航；实际：范围超过31天，产品正确拒绝，测试在读取 push.mock.calls[0] 时报错。建议：固定时钟或明确合法时间窗口，保留产品范围校验。证据：`runs/typescript-first`、`runs/unit-admin-filter-reproduction/results.json`。本轮未修测试源码。

## 原库存仍失败的14个用例

共同前置：固定822c、生产Web、对应legacy/password专属配置，运行下列spec的原始用例。复现命令为 `pnpm exec playwright test e2e/<file> --workers=1 --retries=0`，环境见各run的execution.json/private_env_file；不得使用默认数据库。期望为原用例完整业务断言。实际为下表及所链接原始输出。

| 用例键 | Spec / 场景 | 实际结果与建议 | 首败/最终证据 |
| --- | --- | --- | --- |
| `3dccc2100422` | `collaboration-account-desktop.spec.ts` — Desktop child links, recorded pages and result notifications survive popouts and restart | 分页/作用域失败：首屏已有最终结果，断言预期0；需区分交接区与记录页，核对原请求游标及页内容后定性。 | `legacy-actual-collaboration-account-desktop/results.json` |
| `c919808d1d26` | `collaboration-account-desktop.spec.ts` — Desktop authors initial fields and preserves discussion through shared edit controls | 测试交互不兼容：旧文本框未按当前 PropertyPicker 打开；应复用现有收件人 helper，保留全部创建/附件/撤权断言。 | `legacy-actual-collaboration-account-desktop/results.json` |
| `5a60ce5f71f0` | `collaboration-boundaries-web.spec.ts` — Smaller gaps: title/date edits leave attributed records, recipient lookup is exact-only, a source member resolves and reacts without notifications | 未定位超时；保留现场，分别排查测试接线、客户端等待和API终态，不增加重试。 | `legacy-actual-collaboration-boundaries-web/results.json` |
| `158a87f90c8f` | `collaboration-delivery-web.spec.ts` — Web creates one private issue after lost upload/create responses and revokes an open preview through account WS | 测试交互不兼容：旧文本框未按当前 PropertyPicker 打开；应复用现有收件人 helper，保留全部创建/附件/撤权断言。 | `legacy-actual-collaboration-delivery-web/results.json` |
| `72c1ec89998d` | `collaboration-delivery-web.spec.ts` — Web explicitly resubmits rejected creation input without losing files or reviving the rejected key | 测试交互不兼容：旧文本框未按当前 PropertyPicker 打开；应复用现有收件人 helper，保留全部创建/附件/撤权断言。 | `legacy-actual-collaboration-delivery-web/results.json` |
| `187ee1881c81` | `collaboration-editing-web.spec.ts` — Web source creation, field edits and discussion share one authorized issue | 测试交互不兼容：旧文本框未按当前 PropertyPicker 打开；应复用现有收件人 helper，保留全部创建/附件/撤权断言。 | `legacy-actual-collaboration-editing-web/results.json` |
| `535e317ad170` | `collaboration-leave-web.spec.ts` — Web Leave issue refuses creator and handler; an owner's departure revokes every grant, cancels only their waiting input, stops running work, and rejoining restores nothing | 测试交互不兼容：旧文本框未按当前 PropertyPicker 打开；应复用现有收件人 helper，保留全部创建/附件/撤权断言。 | `namespace-collaboration-leave-web/results.json` |
| `4971c07b93b6` | `collaboration-mention-journeys-desktop.spec.ts` — Desktop owner revocation after preview refuses only A and original-key replay stays deduplicated | 测试快照不兼容：作者权限评估时间每次读取变化；需保留稳定业务事实全量比较，并单列验证评估时间有效性。 | `legacy-actual-collaboration-mention-journeys-desktop/results.json` |
| `8e9c882cd544` | `collaboration-mention-journeys-web.spec.ts` — Web owner revocation after preview refuses only A and original-key replay stays deduplicated | 测试快照不兼容：作者权限评估时间每次读取变化；需保留稳定业务事实全量比较，并单列验证评估时间有效性。 | `legacy-actual-collaboration-mention-journeys-web/results.json` |
| `cd80f1b61840` | `collaboration-records-web.spec.ts` — Web shares ordinary child references and retained run records through account notification links | 分页/作用域失败：首屏已有最终结果，断言预期0；需区分交接区与记录页，核对原请求游标及页内容后定性。 | `legacy-actual-collaboration-records-web/results.json` |
| `f390d53c1162` | `collaboration-retained-attachment.spec.ts` — Web typed human and owned AI creation stays atomic and idle from its first private upload through restart | 测试交互不兼容：旧文本框未按当前 PropertyPicker 打开；应复用现有收件人 helper，保留全部创建/附件/撤权断言。 | `password-actual-collaboration-retained-attachment/results.json` |
| `18e4a6485b4d` | `collaboration-retained-attachment.spec.ts` — Electron typed human and owned AI creation stays atomic and idle from its first private upload through restart | 测试交互不兼容：旧文本框未按当前 PropertyPicker 打开；应复用现有收件人 helper，保留全部创建/附件/撤权断言。 | `password-actual-collaboration-retained-attachment/results.json` |
| `37ff666368df` | `collaboration-retained-attachment.spec.ts` — Web and Electron retain exact comment file bytes and immutable AI input after display removal, deletion, refresh and cross-client restart | 测试快照不兼容：作者权限评估时间每次读取变化；需保留稳定业务事实全量比较，并单列验证评估时间有效性。 | `password-actual-collaboration-retained-attachment/results.json` |
| `7b935d7ce8d5` | `collaboration-source-surface.spec.ts` — Web and Electron source list, board, timeline and comments preserve external identity and scoped layout order | 测试交互不兼容：旧文本框未按当前 PropertyPicker 打开；应复用现有收件人 helper，保留全部创建/附件/撤权断言。 | `password-actual-collaboration-source-surface/results.json` |

每例完整预期/实际、调用位置和错误摘要见 `failures-final.json`，全部执行见 `executions.json`；这些失败尚不支持将其分类为已证实权限泄露。

## QA-STABILITY-001：首败后独立复验通过，原因未关闭

- Web 撤权/作者移除：首次真实评论 POST 预期201，实际503；同断言独立复验通过。未取得首个503响应体，不能确定根因或声称已修复。原证据 `namespace-collaboration-lifecycle-controls`，复验 `lifecycle-web-reproduction`。
- 优先级乐观显示/失败回滚、隐藏字段菜单、原生选择边界：首轮失败，独立复验均通过。原轮与 `priority-rollback-reproduction`、`hidden-fields-reproduction`、`selection-boundary-reproduction` 同存。不能由单次成功宣称稳定性关闭。

## 已定位的环境/测试前置失败

| 编号 | 原始问题 | 修正及结果 |
| --- | --- | --- |
| ENV-A | 固定 Chromium 未安装 | 安装仓库匹配浏览器；先用独立 Chrome 冒烟，随后正式 Chromium；首轮20启动阻塞保留 |
| ENV-B | 初始 Go 专库未迁移 | 原库保留，另建迁移库；后按包专库完成，不计原部分service通过 |
| ENV-C | host-v3准入开启时跑v2 namespace | API409 execution_mode_unsupported与现有cutover契约一致；分模式后重验，不当产品缺陷 |
| ENV-D | Docker DNS 首选IPv6 | 保存首轮，显式验证192.168.65.254后复验；未改断言 |
| ENV-E | MCP/发布目录未创建 | 创建任务专属目录；先API验证，再Web/native发布/目录用例通过 |
| ENV-F | 密码迁移窗口未配置 | 配置测试专属cutoff/deadline，Web/native迁移绑定通过 |
| ENV-G | HTTP假Provider未识别转义后的测试标记 | 保留v1，v2仅修fixture标记识别；原4失败保留，原断言复验通过；无供应商调用 |
| ENV-H | changelog feed路径保护及native locale | 使用该spec允许的任务专属live路径和中文系统locale，原流程通过；未绕过路径保护 |
| ENV-I | 测试收集/启动参数错误 | 3条收集错误、一次不支持的CLI选项等完整保留，不算业务执行 |

## 既有缺陷及版本边界

`baseline/fix-ancestry.json` 与任务 `research/review.md` 核对 runtime9/knowledge21项；H1 gate-OFF额外参数拒绝普通任务、H2旧workdir丢失导致后续知识绑定拒绝、H3超过128条history永久拒绝仍是冻结版本高风险。其余按原审计分类继承；本轮不冒称它们全部复现。其他worktree修复和统筹后续主干合流不能关闭本次旧版本结果。

缺条件的真实Provider、Windows和安装升级组合列为BLOCKED，不使用旧预算、旧构件或交叉编译替代。
