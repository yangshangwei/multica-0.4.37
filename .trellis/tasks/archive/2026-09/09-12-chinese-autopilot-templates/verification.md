# 验收记录：内置自动化模板中文化

验收日期：2026-09-13（Asia/Shanghai）。

## 交付结果

全部 9 个内置模板的执行正文、中文名称和摘要已按业务重新编写；4 个汇总模板采用中文日期标题。缺陷分级明确严重程度到 `urgent/high/medium/low` 的映射，版本升为 2，其余模板保留版本 1。

中文输出模式说明改为「直接运行，不预先创建任务」，与巡检发现实质问题后按需建任务一致。创建、调度、派发和领取复用既有实现，没有增加依赖、数据表、迁移或运行分支。

## 逐项验收

| 要求 | 证据 | 结论 |
| --- | --- | --- |
| R1：自然中文的执行正文和交付格式 | 全部 9 份 PROMPT；`TestAutopilotTemplates_PromptsUseCanonicalChinese`；`research/reviews.md` 逐份对照原文的独立语义审查 | 通过 |
| R2：目录文案、中文日期标题与输出模式一致 | 注册表测试锁定 4 个中文 `{{date}}` 标题和 5 个空标题；真实 API 返回 9 个中文模板；宽屏、窄屏和生成任务截图 | 通过 |
| R3：预览→持久化→编辑→运行正常 | `TestAutopilotTemplateCreate_ChineseTemplatesDispatchVerbatim` 对 9 个模板实际创建、计划派发并从 daemon 领取；真实浏览器验证两种模式及编辑刷新；时区、来源、订阅、next_run 与原子性测试通过 | 通过 |
| R4：业务边界、阈值、去重和静默保持正确 | 注册表的顺序、模式、周期与巡检合同测试；独立审查逐一推演无发现、重复发现、不可用检查、空时段汇总等场景 | 通过 |
| R5：缺陷优先级与权限表述可执行 | `TestAutopilotTemplate_BugTriageUsesValidPriorities`；对照实际 priority 枚举；正文明确 observer 仅建议、写入失败继续处理且逐项报告未应用 | 通过 |
| R6：已有实例和接口兼容 | `TestAutopilotTemplateCreate_UserEditsRemainIndependent`；真实浏览器编辑后读取英文目录仍保留自定义正文；伪造模板字段不被采用；原请求边界 E2E 通过 | 通过 |
| R7：Trellis、检查与验收证据完整 | PRD、design、implement、上下文清单、两轮独立审查、本记录和下面的实际检查结果 | 通过 |

## 实际检查结果

| 检查 | 结果 |
| --- | --- |
| `pnpm test` | 682 个文件、8,040 项测试通过，5 个包任务全部成功 |
| `pnpm --filter @multica/views exec vitest run autopilots` | 最终文案更新后，16 个文件、486 项自动化测试通过 |
| `pnpm typecheck` | 最终工作树检查：9 个包任务全部成功 |
| `pnpm lint` | 最终工作树检查：6 个包任务全部成功，0 errors；另有 31 条非本任务引入的 warnings |
| `make test` | 独立数据库运行 race 检测；66 个含测试的 Go 包通过；无数据库不可用而跳过的假通过 |
| `go vet ./...`，以及最终 `go vet ./internal/handler ./internal/service ./internal/daemon` | 通过 |
| 模板 Go 回归（含 `-race`） | 9 模板真实创建、派发、领取，用户编辑保护，权限拒绝及事务回滚全部通过 |
| `e2e/autopilot-template-zh.spec.ts` | Chromium 真实中文 API 流程通过（1 passed，22.1s） |
| `e2e/autopilot-template.spec.ts` | 原有模板请求边界用例通过（1 passed，8.0s） |
| 新 E2E 独立 TypeScript、ESLint 检查 | 通过 |
| `git diff --check`、Trellis context validate | 通过 |
| visual-verdict | 最终 97/100，pass；1440px、390px 预览与生成任务页均已查看 |

检查日志保存在仓库的 `.omx/qa/chinese-autopilot-templates/checks/`。Go 测试使用本任务创建的独立数据库 `multica_autopilot_zh_20260913`，未将业务数据库用作测试套件的临时表空间。

全部数据库检查结束后，确认连接数为 0，再删除本任务创建的测试数据库及派生的临时环境文件。`checks/database-cleanup.json` 记录 `removed: true`；业务数据库保持正常运行。

## 浏览器与真实 API 证据

目录：`.omx/qa/chinese-autopilot-templates/browser/autopilot-template-zh-adop-937ec-rough-the-real-template-API-chromium/`。

- `workday-preview-1440.png`、`workday-preview-390.png`：实际中文模板预览，正文可滚动，无横向溢出。
- `workday-detail-1440.png`、`workday-edited-detail-1440.png`：真实创建和中文自定义编辑后刷新。
- `daily-review-dispatched-1440.png`、`daily-review-issue-1440.png`：实际派发结果、中文日期标题和中文任务正文。
- `real-api-verification.json`：9 个模板、原文复制、编辑结果、两类运行记录及生成任务。
- `workspace-cleanup.json`：仅本测试的随机工作区，已核对 `removed: true`。
- `.omx/state/chinese-autopilot-templates/ralph-progress.json`：最终视觉判定。

浏览器使用本 checkout 的 API `http://localhost:18572` 和 Web `http://localhost:13492`。已重建嵌入正文并恢复本地服务。

## 发现并处理的问题

1. 原缺陷模板将 `critical` 严重程度与 priority 混用。按实际 API 支持的 `urgent` 等值给出明确映射，并保留权限限制。
2. 原输出模式文案可能让人以为巡检绝不能创建任务，已精确为「不预先创建任务」。
3. 审查发现旧测试清理 helper 误以为订阅和规则版本会级联删除。已按自动化 ID 显式删除这两类记录；真实模板测试前后行数均为订阅 27、规则版本 165，确认没有增长。数据位于独立临时测试库。
4. 首次全量 Go 的 `TestGitEnv` 假定没有继承的 indexed Git 配置，本会话实际继承了 2 项。只从测试子进程中移除 `GIT_CONFIG_COUNT`、`GIT_CONFIG_KEY_n`、`GIT_CONFIG_VALUE_n` 后，环境测试与最终全量 race 均通过；没有修改全局 Git 配置或产品代码。
5. 一次重跑恰逢其他并行任务为 changelog 写入空实现；该独立任务完成实现后，其 race 测试及最终全量 Go 已通过。本任务未修改或回退那部分工作。
6. 浏览器验收处理了两个测试条件：富文本编辑器会保留所选段落的标题格式，测试先切回正文再输入；Next.js 冷编译需要等待路由实际就绪。没有为测试修改产品编辑器或导航行为。

## 验收边界

真实数据库、创建、调度派发、运行时领取和浏览器流程均已验证。运行时使用不连接真实进程或模型账号的隔离 fixture，未执行真实团队任务或将测试提醒发送给真实成员；不将排队/领取成功声称为真实模型完成业务工作的效果评测。提示词的业务质量通过独立语义审查与契约回归验证。

已有自动化保留工作区自己的配置快照，本次不批量改写旧实例。计划触发按保存的时区展开日期；不带 trigger ID 的手动运行沿用 UTC 日期规则。Lint 的 31 条既有警告不影响本次检查退出状态，未顺带修改无关文件。
