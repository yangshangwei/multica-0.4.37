# 端到端与核心业务测试

后续状态：本报告的 TEST-002 焦点失败已在用户要求的下一轮修复。修复后创建表单 25/25、协作相关回归 284/284 通过；原始失败证据保留。详见 [焦点修复报告](../focus-repair-20261107/report.md)。其余首次测试结果继续按本报告记录。

测试日期：2026-11-07。按要求不创建 Trellis 任务，不修改或提交产品源码。首轮冻结基础 HEAD 为 `02d705921f81cccbb45069bb5398db5064cf77b6`，包含当时工作区未提交改动。期间另一会话提交到 `86d98be0414963fb3f3cba1a6c95310f7a1ed02a`；三个变化的协作组件另建生产快照并补测，保留两份源码身份，不把早期结果当作最终字节的完整重跑。

## 当前结果

| 范围 | 结果 |
| --- | --- |
| Core / Views / Web / Desktop 单元与组件回归 | 1,065 个文件、13,285 项全部通过，零跳过 |
| 类型检查 | 6 个目标全部通过 |
| lint | 5 个包全部 exit 0；31 条警告，零错误 |
| 生产 Web E2E：核心流程 | 38 / 38 通过 |
| 生产 Web E2E：恢复、聊天、AI 创建及自动化 | 58 / 58 通过 |
| 生产 Web E2E：项目、迭代、协作创建 | 62 / 64 通过；2 项定位器冲突 |
| 生产 Web E2E：协作收件人及账户工作台 | 3 / 3 通过 |
| 生产 Web E2E：密码注册、登录及恢复 | 14 / 14 通过 |
| E2E 初次有效运行合计 | **175 / 177 通过，2 失败，零跳过、零自动重试** |
| Go 后端 | 15 个包，按包名和测试名去重后 12,255 个命名节点通过，零单测断言失败，17 跳过 |
| Go race / 静态检查 | 6 次窄范围 `-race` 调用通过，零 race 警告；全仓 `go vet` 通过 |
| 最新协作源码 E2E 补验 | 5 / 5 通过，与当前三个组件源码哈希一致 |
| 最新协作源码组件补验 | 组合 44 / 45 通过；同一焦点失败在单文件 23 / 24 运行中复现 |
| Refresh 私有定位诊断 | 原断言保留，仅在私有副本将两处点击限定到内容区，3 / 3 通过；不覆盖原始失败 |

E2E 使用真正的 Chromium、`next build` / `next start`、Go API 和专用 PostgreSQL 数据库。覆盖登录和注销、工作区邀请与权限、任务创建/状态/批量操作/看板拖动、评论与通知、Agent 领取/执行/失败/重试/取消、聊天附件与防重、AI 草稿及失败保留、Autopilot、项目目标/健康/进度/生命周期、迭代规划，以及协作账户工作台。密码部署使用另一份专用数据库，启动时校验实际认证模式。

## 失败项

### TEST-001：两个权限撤销用例使用了不唯一的 Refresh 定位器

- `e2e/projects-p1-lifecycle.spec.ts:286`：P1-L14，真实工作区权限撤销后清除草稿。
- `e2e/projects-p1-lifecycle.spec.ts:324`：P1-L16，成员主动退出且没有 WebSocket 时，20 秒内请求应保持有界。
- 原因证据：`getByRole('button', { name: 'Refresh', exact: true })` 同时匹配内容区和侧栏两个按钮，Playwright 抛出 `strict mode violation`。随后 `finally` 关闭上下文，又产生 `page.waitForResponse: Target page, context or browser has been closed`；JSON 首条错误不能单独代表根因。
- 复现命令：在同配置的独立 legacy 环境运行 `pnpm exec playwright test e2e/projects-p1-lifecycle.spec.ts --workers=1 --retries=0 --grep 'P1-L14|P1-L16'`。
- 证据：`browser/run02-planning-and-collaboration/{run.log,results.json,summary.json,test-results/}`，含失败状态、截图和 trace。
- 结论：这是现有 E2E 定位器歧义；两条原用例未完成后续业务断言。另在私有诊断副本将恰好两处 Refresh 点击限定到 `data-testid=content`，其他断言保持原样，P1-L14 与两个 P1-L16 场景共 3 项全部通过，包括真实撤销、草稿清理及撤销后 20 秒请求上限。原用例仍为失败，未修改项目测试源码。
- 私有诊断证据：`browser/diagnostic-revocation-binding.json`、`browser/diagnostic-revocation.spec.ts`、`browser/revocation-diagnostic/{command.json,results.json,summary.json}`。

### TEST-002：协作创建表单存在未解决的顺序敏感焦点失败

- 位置：`packages/views/collaboration/creation-form.test.tsx:670`。
- 用例：`focuses a delayed known-source title after touch open without stealing a live control: true`。
- 期望：用户放在来源工作区选择框的焦点保持不变；实际：标题输入框获得焦点。
- 最新稳定字节的 3 文件组合重放为 44 通过、1 失败；只运行 creation-form 完整文件仍为 23 通过、1 失败。仅选该单例运行通过一次，因此不能用单例通过宣称失败已解决，也不能简单归因为跨文件并发。
- 首次补测还伴随 creation-form 的并发源码变化，已单独保留；上述组合和单文件复核绑定更新后的 `d4fee44d...` 字节。
- 证据：`browser/latest/{REPORT.md,verification.json,focused-replay-run02.json,focused-file-isolated-run01.json,focused-isolated-run01.json}`，每轮日志和命令均保留。未修复或放宽断言；文件内用例顺序、异步焦点隔离与真实交互之间的归因仍待进一步定位。

### RUN-001：service 原始完整命令超时

原 service 调用在 15 分钟上限退出 1，已通过 869 个顶层测试、2,213 个命名节点，无单项断言失败。此调用仍记录为失败。全新数据库续跑剩余 581 个顶层测试，初次 563 通过、18 项因 Redis 缺失跳过；18 项后来也在专属 Redis 下通过。按测试身份合并，精确覆盖声明清单的全部 1,450 个顶层测试，没有缺失。补跑证据不改写原超时记录。

后端总计 89 个原 Redis 缺失测试全部补跑通过；密码限流器的 48 并发请求精确准入 10 次。保留的 17 个跳过节点涉及平台限制、性能 opt-in、第三方集成和既有占位用例，逐项原因见 `backend/report.md`。

## 环境记录

首轮快照位于 Git 忽略目录，Tailwind 自动扫描得到零个源文件，造成无效样式、OTP 高度为零及弹层遮挡。该轮为 **环境失败**（22 / 37 通过、15 失败），完整保留在 `browser/core/` 和 `environment-run01.json`，不计入有效 E2E 汇总。

随后把相同源码快照迁至 Git 外，重新绑定全部 workspace 依赖，从零生产构建，并启用样式预检：Email 和 OTP 控件高度均为 32px。新数据库和构建的原始 E2E 结果独立存放在 `browser/run02-*`，没有修改断言或增加自动重试。

测试期间发现三个协作组件发生并发编辑，另有五张内嵌文档图片和 tutorial 文档 JSON 变化。早期记录为 `source-audit-current.json`，后续完整差异见 `browser/latest/source-audit.json`。三项业务源码已在 Git 外重新生产构建，Build ID 为 `VDQqjJElKD2ChUVY2IesP`；全部 22 条实际 workspace 依赖都在快照内部，Tailwind 扫描和生成样式预检通过。最后核对三个组件的快照与当前源码哈希相同，最新 E2E 5 项全通过；焦点单测失败如上保留。

所有本轮启动的浏览器验证 API、Web、确定性 provider 和专属 Redis 均在结束时停止，保留数据库与证据供复查。后端 23 个自有数据库在结束时连接为零；浏览器资源停止记录与数据库连接检查见 `browser/cleanup.json`。未停止其他会话的运行环境。

## 证据与覆盖边界

- 前端完整报告、命令、退出状态与逐用例清单：`frontend/REPORT.md`、`frontend/summary.json`、`frontend/*-inventory.json`。
- 后端原始命名测试事件、隔离数据库、命令和跳过原因：`backend/`。
- 后端按身份去重汇总及完整报告：`backend/aggregate.json`、`backend/report.md`。Go 工具链为 `go1.27.0 darwin/arm64`，PostgreSQL 为 17.11；这些是本机验证结果。
- 各 E2E 分组：`browser/<分组>/{command.json,results.json,summary.json,run.log,test-results/}`。
- 额外脚本回归：49 项中 48 通过、1 项数据库 opt-in 跳过，命令为 `node --test scripts/seed-rich-test-data.test.mjs scripts/check-collaboration-native-tests.test.mjs scripts/check-agent-critical-path.test.mjs`。
- Agent 浏览器回路使用真实 HTTP 协议的模拟 daemon；AI refinement 使用本机确定性 provider。未调用用户安装的真实 Agent CLI、未消耗模型账户额度，因此不能据此证明真实模型质量或外部 provider 可用性。
- 未执行 Electron 安装包、iOS 真机或全部可选部署组合。性能门槛、外部集成及原生 skip 以清单为准，不能把跳过记为通过。
- 产品文件无本轮修复或简化；本轮新增内容仅为隔离验证脚本、日志、截图、结果清单和本报告。

本轮未全绿：保留 2 项原始 E2E 定位失败、1 项最新组件焦点失败及原 service 超时调用。私有定位诊断、分库补跑和后续通过分别列出，均未擦除首次结果。
