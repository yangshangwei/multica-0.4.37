# E2E 分支覆盖扩展与验证：2026-10-05

当前 Playwright 清单为 **234 条 / 74 个 spec**，较本轮开始的 **180 条 / 68 个 spec** 新增 **54 条**。按文件、完整测试标题和 project 精确去重，234 条均取得本轮实际通过证据。没有把跳过、清单枚举或失败后自动重试算作通过。

新增部分包含 **50 条浏览器场景、4 条 HTTP 集成场景**。故障注入只改变指定请求的失败/延迟，重试和持久化验证使用本地真实 Go API 与 PostgreSQL；无真实模型执行。**这是一份业务场景覆盖账本，不是 100% 代码分支覆盖率报告。**

## 新增分支矩阵

| 模块 | 新增 | 关键分支 |
| --- | ---: | --- |
| [密码认证](../../e2e/password-branches.spec.ts) | 12 | 用户名和密码边界、姓名空白、重复账号、错误凭据、未知账号、注册响应丢失、网络/503/429恢复、重复提交 |
| [任务写入](../../e2e/issue-mutation-branches.spec.ts) | 7 | 状态/优先级回滚、删除失败重试和取消、创建草稿保留、评论草稿恢复、重复发送 |
| [工作区与权限](../../e2e/workspace-access-branches.spec.ts) | 10 | 邀请接受/拒绝/撤销/无效ID/错误收件人、跨工作区读写拒绝、实时撤权、删除取消与重试 |
| [自动化](../../e2e/autopilot-lifecycle-branches.spec.ts) | 9 | 必填项、非模板创建、手动任务改名、添加定时、暂停/恢复、失败回滚、删除取消/重试、触发器部分保存失败 |
| [聊天与附件](../../e2e/chat-recovery-branches.spec.ts) | 6 | 上传失败恢复、移除附件、会话创建失败、发送失败、并发提交、切换会话时迟到响应 |
| [分诊](../../e2e/triage-branches.spec.ts) | 10 | 禁用入口、暂停关闭限制、优先级/责任人规则、版本冲突、非法暂停时间、重新审核、请求幂等、禁止隐式派发 |

每条用例单独命名。失败恢复验证页面状态及真实 API 数据；涉及重复操作时检查记录、历史或请求数。分诊的三个 API 集成场景和工作区的一个 API 集成场景在标题中明确标记。未把原来的长用例拆分后计为新增。

## 本轮发现与修复

- **产品缺陷：跨域登录限流冷却失效。** 后端发送了 `Retry-After`，但 CORS 未暴露该响应头；浏览器读不到它，密码表单无法开始等待。`server/cmd/server/router.go` 增加一个暴露响应头；`cors_headers_test.go` 新增 429/503 回归，先失败后通过。浏览器另行确认 API 与页面确实跨域，并读取真实部署的 CORS 配置验证倒计时。
- **既有测试类型错误：** 修复 iframe 参数、onboarding token、工作区存在性、Desktop bridge、CLI 退出码共六处严格类型错误；采用明确类型和运行时检查，没有弱化业务断言。
- **新测试修正：** 路由必须先 fulfill/continue 再 unroute；定时器实际保存带 `TZ=UTC` 的 cron；撤销邀请会删除记录并返回 404；单智能体直接进入会话；AI 可在发送后改写会话标题，测试导航应使用稳定的会话 ID。修正均根据源码和真实浏览器证据。
- **测试环境修正：** 原任务 env 含重复的设备认证和 provider 配置，后面的旧值覆盖了新值，造成首轮 14 条失败。清除冲突后核验 `/api/config` 与 provider `/health`，42 条相关场景和修改过的 spec 全部复测通过。

## 验证证据

原始证据目录：`.gstack/qa-reports/2026-10-05-branch-expansion/`（本地忽略目录）。完整逐条历史见 `evidence-summary.json`；运行配置、构建 ID 与源码哈希见 `provenance-final.json`。最终状态按实际执行时间选择，早期失败仍保留。

| 检查 | 结果与边界 |
| --- | --- |
| Playwright 当前全部用例 | 234 条均有本轮零重试通过证据，0 最终失败、0 仅跳过、0 缺失；分环境多次运行合并，并非一次全量命令全绿 |
| 新增密码分支 | 12/12；强化跨域断言后冷却用例额外 1/1 |
| 新增其他分支 | 42/42 逐项通过；最后合并运行 41 通过、1 个动态标题定位失败，修正后聊天整文件 6/6 再次通过 |
| 设备登录历史跳过项 | 独立开启设备认证的 API 上实跑 1/1 |
| 既有管理员、密码迁移与原生专项 | 17/17，包含真实 Electron fixture；外部运行时按原用例约束模拟 |
| TypeScript 单元测试 | `turbo test --concurrency=1 -- --maxWorkers=2` 9956/9956，5 个包任务成功，其中 docs 使用有效缓存；未将 mobile 历史 123 条计入 |
| 全仓 typecheck / lint | 均成功，复用有效 Turbo 缓存；lint 保留 29 个既有 warnings，无 errors |
| E2E 严格 TypeScript | 对全部 `e2e/*.spec.ts` 单独执行 strict tsc，退出 0 |
| 后端改动所在包 | 独立新建并迁移数据库，`go test -race -p 2 -parallel 2 ./cmd/server` 成功（39.160s）；`go vet ./cmd/server` 成功；测试数据库已删除 |
| 差异检查 | `git diff --check` 成功 |

初始不限并发的 `pnpm test` 曾有一个设备认证单测超时并中止后续包；随后限制并发的完整 9956 条运行通过，没有增加测试超时。一次未显式指定数据库的后端扩大检查连到了旧 schema 并失败；改用新建、完整迁移且不被 API 使用的专用数据库后，整个改动包通过。未把这两次失败记作成功，也未声称本轮跑过整个 Go 仓库或单条 `make check`。

## 覆盖解释与剩余边界

| 范围 | 可支持的结论 | 仍不能推出的结论 |
| --- | --- | --- |
| 认证 / 会话 / 管理员 | 邮箱、设备、密码、迁移、撤权、跨域冷却与故障恢复已有实际场景 | 所有攻击序列、任意跨版本组合均已穷尽 |
| 任务 / 分诊 / 工作区 | 代表性读写、权限、冲突、幂等、取消与重试链路通过 | 全部状态 × 角色 × 平台笛卡尔积已由 E2E 枚举 |
| AI / Agent / MCP / 自动化 | 受控 provider、API、派发边界和保存规则通过 | 真实模型推理、第三方 MCP transport、生产 daemon 全链路通过 |
| 聊天 / 附件 | 提交前失败、重复提交、附件绑定与迟到响应保护通过 | 所有“服务端已保存但响应丢失”场景都具有幂等保障 |
| UI / 原生 | Chromium 与现有 Electron fixtures 有实际证据 | Firefox、WebKit、iOS 原生、真实桌面升级安装均通过 |
| 外部服务 | 原有 mock / 本地 fixture 覆盖 UI 接线及协议边界 | 真实 OAuth/Composio、邮件投递、支付、IM 外部服务已验收 |

纯函数/枚举/缓存状态矩阵继续由对应单测负责，例如 `core/issues/mutations.test.tsx`、`core/issues/batch.test.ts`、`core/chat/pending.test.ts`、定时器 cron 解析测试。E2E 保留有业务意义的贯穿链路，不重复铺设相同纯函数矩阵。

## 变更范围与复现

新增六个分支 spec；修正五个旧 spec 的类型检查；生产改动仅为暴露 `Retry-After`，并增加直接 CORS 回归。复用 `TestApiClient`，没有新增依赖或通用抽象。开始本轮前工作区已有的其他未提交改动保持原状。隔离的 API/Web 及模型 fixture 进程已停止，API 测试数据库和报告保留。本轮改动按认证修复与分支扩展分别提交；原有改动不包含在这两笔提交中。验证记录基于包含原有修复的完整工作区，并非仅应用本轮提交后的干净基线复测。

按照 `.trellis/spec/web/frontend/e2e-run-environment.md` 启动隔离的生产模式 Web/API。密码场景需要 `E2E_PASSWORD_AUTH=1` 和 password mode；其他新增场景使用关闭 device auth 的 legacy mode。导出匹配的 `DATABASE_URL`、`NEXT_PUBLIC_API_URL`、`PLAYWRIGHT_BASE_URL` 后运行：

```bash
pnpm exec playwright test e2e/password-branches.spec.ts --workers=1 --retries=0
pnpm exec playwright test e2e/triage-branches.spec.ts e2e/issue-mutation-branches.spec.ts e2e/workspace-access-branches.spec.ts e2e/autopilot-lifecycle-branches.spec.ts e2e/chat-recovery-branches.spec.ts --workers=1 --retries=0
pnpm exec tsc --noEmit --target ES2022 --module ESNext --moduleResolution bundler --esModuleInterop --skipLibCheck --strict e2e/*.spec.ts
```

密码注册服务存在真实的每 IP 30 次 / 5 分钟预算；一次新增密码套件约使用 17 次注册请求。连续多轮运行需要遵守该预算，不通过关闭限流掩盖问题。

## 新增用例逐条索引

### 密码认证（12 条）

| 测试标题 | 最终证据 |
| --- | --- |
| [password authentication branch recovery › invalid username preserves registration fields and can be corrected without restarting](../../e2e/password-branches.spec.ts#L49) | `password-verified.json`，passed |
| [password authentication branch recovery › five-character password error can be corrected to the supported six-character minimum](../../e2e/password-branches.spec.ts#L75) | `password-verified.json`，passed |
| [password authentication branch recovery › overlong pasted password is rejected and accepts the corrected 128-character boundary](../../e2e/password-branches.spec.ts#L98) | `password-verified.json`，passed |
| [password authentication branch recovery › whitespace-only display name reaches server validation and keeps credentials for correction](../../e2e/password-branches.spec.ts#L123) | `password-verified.json`，passed |
| [password authentication branch recovery › duplicate username after normalization leaves the original identity and password intact](../../e2e/password-branches.spec.ts#L146) | `password-verified.json`，passed |
| [password authentication branch recovery › incorrect password preserves the login form and a corrected retry restores the same identity](../../e2e/password-branches.spec.ts#L172) | `password-verified.json`，passed |
| [password authentication branch recovery › unknown account gives a generic error and can sign in after the account is created](../../e2e/password-branches.spec.ts#L196) | `password-verified.json`，passed |
| [password authentication branch recovery › lost registration response offers sign-in recovery for the account the server already created](../../e2e/password-branches.spec.ts#L218) | `password-verified.json`，passed |
| [password authentication branch recovery › network failure during login keeps credentials and succeeds after connection recovery](../../e2e/password-branches.spec.ts#L254) | `password-verified.json`，passed |
| [password authentication branch recovery › temporary authentication service failure preserves the form and permits a real retry](../../e2e/password-branches.spec.ts#L284) | `password-verified.json`，passed |
| [password authentication branch recovery › Retry-After disables login submissions until cooldown ends without discarding credentials](../../e2e/password-branches.spec.ts#L314) | `password-cors-verified.json`，passed |
| [password authentication branch recovery › pending registration locks the form and ignores repeated native submissions](../../e2e/password-branches.spec.ts#L358) | `password-verified.json`，passed |

### 任务写入（7 条）

| 测试标题 | 最终证据 |
| --- | --- |
| [failed status update rolls back the optimistic value before a successful retry](../../e2e/issue-mutation-branches.spec.ts#L154) | `branches-final.json`，passed |
| [failed priority update restores the existing priority and retry persists](../../e2e/issue-mutation-branches.spec.ts#L179) | `branches-final.json`，passed |
| [failed deletion keeps the confirmation and issue until a successful retry](../../e2e/issue-mutation-branches.spec.ts#L204) | `branches-final.json`，passed |
| [cancelling after a failed deletion leaves the issue readable after reload](../../e2e/issue-mutation-branches.spec.ts#L226) | `branches-final.json`，passed |
| [failed manual creation retains title and description and retry creates exactly one issue](../../e2e/issue-mutation-branches.spec.ts#L242) | `branches-final.json`，passed |
| [failed comment send preserves the draft across reload and retry posts exactly once](../../e2e/issue-mutation-branches.spec.ts#L277) | `branches-final.json`，passed |
| [repeated send shortcuts while a comment is pending create only one comment](../../e2e/issue-mutation-branches.spec.ts#L309) | `branches-final.json`，passed |

### 工作区与权限（10 条）

| 测试标题 | 最终证据 |
| --- | --- |
| [accepting an invitation joins exactly once and revisiting shows its accepted state](../../e2e/workspace-access-branches.spec.ts#L87) | `branches-final.json`，passed |
| [declining an invitation persists without membership and prevents later acceptance](../../e2e/workspace-access-branches.spec.ts#L105) | `branches-final.json`，passed |
| [revoking an invitation while its page is open rejects stale acceptance](../../e2e/workspace-access-branches.spec.ts#L121) | `branches-final.json`，passed |
| [a nonexistent invitation shows a recoverable empty state](../../e2e/workspace-access-branches.spec.ts#L145) | `branches-final.json`，passed |
| [another account cannot read accept or decline an invitation addressed to its owner](../../e2e/workspace-access-branches.spec.ts#L157) | `branches-final.json`，passed |
| [a nonmember cannot open or mutate another workspace and can recover to their own](../../e2e/workspace-access-branches.spec.ts#L173) | `branches-final.json`，passed |
| [[API integration] foreign issue identifiers cannot bypass the selected workspace on reads or writes](../../e2e/workspace-access-branches.spec.ts#L192) | `branches-final.json`，passed |
| [removing a member from an open workspace relocates the browser and revokes stale API access](../../e2e/workspace-access-branches.spec.ts#L203) | `branches-final.json`，passed |
| [cancelling workspace deletion preserves data and resets the typed confirmation](../../e2e/workspace-access-branches.spec.ts#L230) | `branches-final.json`，passed |
| [[browser fault injection] failed workspace deletion keeps the dialog and data until a successful retry](../../e2e/workspace-access-branches.spec.ts#L250) | `branches-final.json`，passed |

### 自动化（9 条）

| 测试标题 | 最终证据 |
| --- | --- |
| [autopilot lifecycle branches › focuses missing required fields and creates nothing until title and executor are supplied](../../e2e/autopilot-lifecycle-branches.spec.ts#L133) | `branches-final.json`，passed |
| [autopilot lifecycle branches › creates a non-template run-only autopilot with its default schedule and persists it on reload](../../e2e/autopilot-lifecycle-branches.spec.ts#L164) | `branches-final.json`，passed |
| [autopilot lifecycle branches › renames a manual autopilot without silently creating a schedule](../../e2e/autopilot-lifecycle-branches.spec.ts#L189) | `branches-final.json`，passed |
| [autopilot lifecycle branches › adds an explicitly requested default schedule to a manual autopilot exactly once](../../e2e/autopilot-lifecycle-branches.spec.ts#L207) | `branches-final.json`，passed |
| [autopilot lifecycle branches › persists pause and resume across reload and gates manual runs while paused](../../e2e/autopilot-lifecycle-branches.spec.ts#L228) | `branches-final.json`，passed |
| [autopilot lifecycle branches › rolls back a rejected pause and can pause successfully after retry](../../e2e/autopilot-lifecycle-branches.spec.ts#L249) | `branches-final.json`，passed |
| [autopilot lifecycle branches › cancels detail deletion without issuing a delete or losing the manual autopilot](../../e2e/autopilot-lifecycle-branches.spec.ts#L273) | `branches-final.json`，passed |
| [autopilot lifecycle branches › restores the list row after failed deletion and removes it after a real retry](../../e2e/autopilot-lifecycle-branches.spec.ts#L292) | `branches-final.json`，passed |
| [autopilot lifecycle branches › preserves a created autopilot when its schedule fails and repairs it without a duplicate](../../e2e/autopilot-lifecycle-branches.spec.ts#L324) | `branches-final.json`，passed |

### 聊天与附件（6 条）

| 测试标题 | 最终证据 |
| --- | --- |
| [upload rejection preserves text and reattaching binds exactly one file on send [fault injection]](../../e2e/chat-recovery-branches.spec.ts#L120) | `chat-final.json`，passed |
| [removing an uploaded attachment sends only the remaining text [real API]](../../e2e/chat-recovery-branches.spec.ts#L151) | `chat-final.json`，passed |
| [session creation rejection keeps the new draft and retries without empty navigation [fault injection]](../../e2e/chat-recovery-branches.spec.ts#L175) | `chat-final.json`，passed |
| [message rejection retains the draft and retry persists no duplicate [fault injection]](../../e2e/chat-recovery-branches.spec.ts#L201) | `chat-final.json`，passed |
| [repeated keyboard submit during a pending send creates only one message [latency injection]](../../e2e/chat-recovery-branches.spec.ts#L224) | `chat-final.json`，passed |
| [late send acceptance preserves the other session's draft and selection [latency injection]](../../e2e/chat-recovery-branches.spec.ts#L250) | `chat-final.json`，passed |

### 分诊（10 条）

| 测试标题 | 最终证据 |
| --- | --- |
| [Triage browser + real API branch coverage › disabled triage hides intake and rejects a direct submission without creating work](../../e2e/triage-branches.spec.ts#L132) | `branches-final.json`，passed |
| [Triage browser + real API branch coverage › a snoozed pending item prevents disabling triage in both settings and the API](../../e2e/triage-branches.spec.ts#L147) | `branches-final.json`，passed |
| [Triage browser + real API branch coverage › required priority preserves the acceptance draft until a valid priority is selected](../../e2e/triage-branches.spec.ts#L164) | `branches-final.json`，passed |
| [Triage browser + real API branch coverage › responsibility requires a reviewer and applies only to new intake without assigning execution](../../e2e/triage-branches.spec.ts#L186) | `branches-final.json`，passed |
| [Triage browser + real API branch coverage › a stale review draft retains its reason, refreshes the revision and commits exactly once](../../e2e/triage-branches.spec.ts#L205) | `branches-final.json`，passed |
| [Triage browser + real API branch coverage › a past snooze time keeps the item ready until a valid future time is confirmed](../../e2e/triage-branches.spec.ts#L233) | `branches-final.json`，passed |
| [Triage browser + real API branch coverage › a rejected item cannot reopen while disabled and uses current responsibility in the next round](../../e2e/triage-branches.spec.ts#L256) | `branches-final.json`，passed |
| [Triage API integration branch coverage › intake replay returns one identity and a changed payload cannot reuse its request ID](../../e2e/triage-branches.spec.ts#L297) | `branches-final.json`，passed |
| [Triage API integration branch coverage › decision replay preserves the original revision and history while rejecting a reused key with a new reason](../../e2e/triage-branches.spec.ts#L314) | `branches-final.json`，passed |
| [Triage API integration branch coverage › pending items reject rerun and ordinary assignment while retaining editable content](../../e2e/triage-branches.spec.ts#L332) | `branches-final.json`，passed |

## 全部 spec 账本

| Spec | 当前用例 | 实际通过 |
| --- | ---: | ---: |
| [admin-resource-publishing.spec.ts](../../e2e/admin-resource-publishing.spec.ts) | 3 | 3 |
| [agent-autonomy-gates.spec.ts](../../e2e/agent-autonomy-gates.spec.ts) | 25 | 25 |
| [agent-categories.spec.ts](../../e2e/agent-categories.spec.ts) | 1 | 1 |
| [agent-category-grouping.spec.ts](../../e2e/agent-category-grouping.spec.ts) | 1 | 1 |
| [agent-mcp.spec.ts](../../e2e/agent-mcp.spec.ts) | 2 | 2 |
| [agent-role-template.spec.ts](../../e2e/agent-role-template.spec.ts) | 3 | 3 |
| [agents-discovery.spec.ts](../../e2e/agents-discovery.spec.ts) | 1 | 1 |
| [auth-device.spec.ts](../../e2e/auth-device.spec.ts) | 1 | 1 |
| [auth-qa.spec.ts](../../e2e/auth-qa.spec.ts) | 1 | 1 |
| [auth.spec.ts](../../e2e/auth.spec.ts) | 4 | 4 |
| [autopilot-lifecycle-branches.spec.ts](../../e2e/autopilot-lifecycle-branches.spec.ts) | 9 | 9 |
| [autopilot-template-zh.spec.ts](../../e2e/autopilot-template-zh.spec.ts) | 1 | 1 |
| [autopilot-template.spec.ts](../../e2e/autopilot-template.spec.ts) | 2 | 2 |
| [changelog-desktop.spec.ts](../../e2e/changelog-desktop.spec.ts) | 1 | 1 |
| [changelog.spec.ts](../../e2e/changelog.spec.ts) | 2 | 2 |
| [chat-actor-picker.spec.ts](../../e2e/chat-actor-picker.spec.ts) | 3 | 3 |
| [chat-attachments.spec.ts](../../e2e/chat-attachments.spec.ts) | 1 | 1 |
| [chat-recovery-branches.spec.ts](../../e2e/chat-recovery-branches.spec.ts) | 6 | 6 |
| [comments.spec.ts](../../e2e/comments.spec.ts) | 2 | 2 |
| [desktop-settings.spec.ts](../../e2e/desktop-settings.spec.ts) | 1 | 1 |
| [iframe-scroll-bridge.spec.ts](../../e2e/iframe-scroll-bridge.spec.ts) | 6 | 6 |
| [issue-assist-flow.spec.ts](../../e2e/issue-assist-flow.spec.ts) | 8 | 8 |
| [issue-mutation-branches.spec.ts](../../e2e/issue-mutation-branches.spec.ts) | 7 | 7 |
| [issue-table.spec.ts](../../e2e/issue-table.spec.ts) | 4 | 4 |
| [issues.spec.ts](../../e2e/issues.spec.ts) | 7 | 7 |
| [lifecycle-handoff.spec.ts](../../e2e/lifecycle-handoff.spec.ts) | 3 | 3 |
| [localized-template-defaults.spec.ts](../../e2e/localized-template-defaults.spec.ts) | 5 | 5 |
| [main-supplemental-qa.spec.ts](../../e2e/main-supplemental-qa.spec.ts) | 3 | 3 |
| [mcp-desktop.spec.ts](../../e2e/mcp-desktop.spec.ts) | 1 | 1 |
| [mcp-market.spec.ts](../../e2e/mcp-market.spec.ts) | 5 | 5 |
| [missing-user-session.spec.ts](../../e2e/missing-user-session.spec.ts) | 1 | 1 |
| [navigation.spec.ts](../../e2e/navigation.spec.ts) | 3 | 3 |
| [onboarding-shell.spec.ts](../../e2e/onboarding-shell.spec.ts) | 2 | 2 |
| [onboarding-smoke.spec.ts](../../e2e/onboarding-smoke.spec.ts) | 3 | 3 |
| [password-binding-desktop.spec.ts](../../e2e/password-binding-desktop.spec.ts) | 1 | 1 |
| [password-branches.spec.ts](../../e2e/password-branches.spec.ts) | 12 | 12 |
| [password-migration.spec.ts](../../e2e/password-migration.spec.ts) | 2 | 2 |
| [password-registration.spec.ts](../../e2e/password-registration.spec.ts) | 2 | 2 |
| [password-server-switch.spec.ts](../../e2e/password-server-switch.spec.ts) | 1 | 1 |
| [password-session-persistence.spec.ts](../../e2e/password-session-persistence.spec.ts) | 1 | 1 |
| [platform-admin-accounts.spec.ts](../../e2e/platform-admin-accounts.spec.ts) | 1 | 1 |
| [platform-admin-controls.spec.ts](../../e2e/platform-admin-controls.spec.ts) | 1 | 1 |
| [platform-admin-executions.spec.ts](../../e2e/platform-admin-executions.spec.ts) | 1 | 1 |
| [platform-admin-installations-read.spec.ts](../../e2e/platform-admin-installations-read.spec.ts) | 1 | 1 |
| [platform-admin-observability.spec.ts](../../e2e/platform-admin-observability.spec.ts) | 1 | 1 |
| [platform-admin.spec.ts](../../e2e/platform-admin.spec.ts) | 1 | 1 |
| [plugin-surface-document.spec.ts](../../e2e/plugin-surface-document.spec.ts) | 2 | 2 |
| [plugin-surface-security.spec.ts](../../e2e/plugin-surface-security.spec.ts) | 3 | 3 |
| [progress-reporting.spec.ts](../../e2e/progress-reporting.spec.ts) | 3 | 3 |
| [project-squad-workspace.spec.ts](../../e2e/project-squad-workspace.spec.ts) | 1 | 1 |
| [property-icons.spec.ts](../../e2e/property-icons.spec.ts) | 1 | 1 |
| [quick-actions.spec.ts](../../e2e/quick-actions.spec.ts) | 1 | 1 |
| [quick-create-actor-picker-phase2.spec.ts](../../e2e/quick-create-actor-picker-phase2.spec.ts) | 4 | 4 |
| [quick-create-actor-picker.spec.ts](../../e2e/quick-create-actor-picker.spec.ts) | 8 | 8 |
| [retained-languages-desktop.spec.ts](../../e2e/retained-languages-desktop.spec.ts) | 2 | 2 |
| [retained-languages.spec.ts](../../e2e/retained-languages.spec.ts) | 3 | 3 |
| [settings-integration-catalog.spec.ts](../../e2e/settings-integration-catalog.spec.ts) | 2 | 2 |
| [settings-preferences.spec.ts](../../e2e/settings-preferences.spec.ts) | 2 | 2 |
| [settings.spec.ts](../../e2e/settings.spec.ts) | 2 | 2 |
| [skill-category-taxonomy.spec.ts](../../e2e/skill-category-taxonomy.spec.ts) | 1 | 1 |
| [skill-library-accessibility.spec.ts](../../e2e/skill-library-accessibility.spec.ts) | 1 | 1 |
| [skill-market.spec.ts](../../e2e/skill-market.spec.ts) | 1 | 1 |
| [skill-template-creation.spec.ts](../../e2e/skill-template-creation.spec.ts) | 1 | 1 |
| [squads-audit.spec.ts](../../e2e/squads-audit.spec.ts) | 1 | 1 |
| [squads-design.spec.ts](../../e2e/squads-design.spec.ts) | 1 | 1 |
| [task-lifecycle-qa.spec.ts](../../e2e/task-lifecycle-qa.spec.ts) | 1 | 1 |
| [triage-branches.spec.ts](../../e2e/triage-branches.spec.ts) | 10 | 10 |
| [triage-desktop.spec.ts](../../e2e/triage-desktop.spec.ts) | 1 | 1 |
| [triage.spec.ts](../../e2e/triage.spec.ts) | 6 | 6 |
| [upstream-backports-cli.spec.ts](../../e2e/upstream-backports-cli.spec.ts) | 3 | 3 |
| [upstream-selected-fixes.spec.ts](../../e2e/upstream-selected-fixes.spec.ts) | 5 | 5 |
| [workspace-access-branches.spec.ts](../../e2e/workspace-access-branches.spec.ts) | 10 | 10 |
| [workspace-defaults.spec.ts](../../e2e/workspace-defaults.spec.ts) | 3 | 3 |
| [workspace-name-series.spec.ts](../../e2e/workspace-name-series.spec.ts) | 2 | 2 |
