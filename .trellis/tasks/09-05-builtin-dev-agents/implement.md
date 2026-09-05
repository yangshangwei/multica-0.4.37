# 执行计划：内置研发智能体第一阶段

状态说明：S1–S8 已实现并验证通过（2026-09-05，分支 `feat/builtin-dev-agents`，尚未提交）。S9 的 E2E spec 已写但未执行，原因见该节。

每一步都是纯增量。历史智能体的 `autonomy_level` 为空，因此在任何一步之后，既有 Agent / Squad / 自动化的行为都不变。

## S1 数据模型与查询

- [x] 迁移 450：`agent` 加 `template_key` / `template_version` / `autonomy_level`（默认空/0/空）。
- [x] 迁移 451：`squad` 加 `template_key` / `template_version`。
- [x] 迁移 452：建 `agent_approval_request`，**无外键**，`risk_class` 与 `status` 用 CHECK 固定词表。
- [x] 迁移 453 / 454：两个 `CREATE INDEX CONCURRENTLY`，各自单语句文件，down 用 `DROP INDEX CONCURRENTLY`。
- [x] 把两个并发索引登记进 `cmd/migrate/main.go` 的 `concurrentIndexCleanups`。
- [x] `pkg/db/queries/agent.sql`：`CreateAgent` 加三列（`COALESCE(narg, 默认)`）、`UpdateAgent` 加 `autonomy_level`、新增 `GetAgentByWorkspaceAndTemplateKey`。
- [x] `pkg/db/queries/squad.sql`：新增 `CreateSquadFromTemplate`（与 `CreateSquad` 分开，因为 `instructions` 只在这条路径上写）。
- [x] `pkg/db/queries/agent_approval.sql`：状态跃迁守卫写在 SQL 的 `WHERE` 里。
- [x] `workspace_delete.sql` 的 `DeleteWorkspaceLeafData` 加 `agent_approval_request` 清理 CTE。
- [x] `make sqlc` 重新生成。

验证：`cd server && go run ./cmd/migrate up && go build ./...`

## S2 模板注册表与内容

- [x] `internal/service/builtin_agent_templates.go`：`AutonomyLevel` 四级、`AutonomyAtLeast`（未声明 → 全部放行）、`AgentRoleTemplate`、四语回退。
- [x] `builtin_agent_templates_roster.go`：8 个 `Listed: true` 模板 + 2 个 unlisted squad leader。
- [x] `builtin_agent_templates/<10>/INSTRUCTIONS.md`：每份含职责/不负责/先读什么/输出格式/完成标准/上抛条件。
- [x] `builtin_role_skills.go` + `builtin_role_skills/<7>/SKILL.md`：独立 embed，**不**进 `BuiltinSkills()`。
- [x] `builtin_squad_templates.go` + 2 份路由策略。
- [x] 测试 `builtin_agent_templates_test.go`：清单恰好 8 个且顺序固定、unlisted 必须被某个 squad 引用且为 Coordinator、自治默认映射（Operator 恰好 1 个）、每份指令含必需小节且无未替换占位符、每个 Skill 名存在且带描述与版本、每个磁盘上的 Skill 都已登记版本、四语齐全。

验证：`cd server && go test ./internal/service -run 'TestAgentRoleTemplate|TestRoleSkill' -count=1`

## S3 自治策略提示词

- [x] `builtin_agent_autonomy.go`：四级正文 + Operator 的审批协议 + 其余等级的「碰到高风险动作怎么办」。
- [x] `AutonomyBriefing("")` 返回空串（历史智能体提示词逐字节不变）。
- [x] `ApprovalRiskClasses` 与迁移 452 的 CHECK 同源。
- [x] `daemon.go` claim 处注入：agent 自身指令 → 自治策略 → squad briefing。
- [x] 测试：未声明等级无附加段、Operator 含协议与风险类、非 Operator 不得出现执行协议、风险类与迁移一致、协议里的 CLI 命令名与 `cmd_approval.go` 一致。

验证：`cd server && go test ./internal/service -run 'TestAutonomy|TestApprovalRisk' -count=1`

**评审门：** 复制语义 vs Mika 分层、以及「未声明等级全部放行」的兼容性规则在这里定稿。

## S4 从模板创建 Agent

- [x] 把 `CreateAgent` 正文抽成 `createAgentFromRequest(w, r, req, rawFields, provenance)`；`CreateAgent` 只负责解析 body。
- [x] `CreateAgentParams` 传入 provenance 三列。
- [x] `agent_template.go`：`ListAgentRoleTemplates`、`CreateAgentFromTemplate`、`materializeRoleSkills` / `materializeRoleSkillsInTx` / `lockRoleSkillMaterialization`。
- [x] `AgentResponse` 暴露 `template_key` / `template_version` / `autonomy_level`（`omitempty`）。
- [x] `UpdateAgentRequest` 加 `autonomy_level`（三态：省略保留、`""` 清空、具名设置），并对机器凭证拒绝。
- [x] 路由：`GET /api/agents/templates`、`POST /api/agents/from-template`。
- [x] 测试 `agent_template_test.go`：指令逐字复制、provenance 与并发上限、Access 默认 private、Skill 已改写时复用且内容不变且只有一份、同角色第二个 Agent 共用同一 Skill 行、未知/空/unlisted key 返回 400、缺 runtime 400、`POST /api/agents` 无法伪造 provenance、显式放开工作区 Access。

验证：`cd server && go test ./internal/handler -run 'TestListAgentRoleTemplates|TestCreateAgentFromTemplate|TestCreateAgent_Ignores' -count=1`

## S5 自治执行

- [x] `agent_autonomy.go`：`agentActorAutonomy` + `requireAgentAutonomy` + 面向模型的拒绝文案（说清等级、要求与替代做法）。
- [x] 执行点：`UpdateIssue`（status/assignee 出现时）、`CreateIssue` → Contributor；`CreateAutopilot`/`UpdateAutopilot`、`CreateSquad` → Coordinator。
- [x] 测试 `agent_autonomy_test.go`：Observer 改状态/负责人被拒且数据未变、Observer 仍可改描述、Contributor 可以、**未声明等级不受影响**、人类不受影响、Observer 不能建 issue 且计数不变、Contributor 不能建 squad/自动化、智能体不能自提权、人类可设置/清空/省略保留、非法值 400。
- [x] `agent_autonomy_claim_test.go`：策略段位置在自身指令之后、未写入行、Operator 拿到协议、未声明等级 claim 到的指令与自身文本完全相同。

验证：`cd server && go test ./internal/handler -run 'TestUpdateIssue_|TestCreateIssue_Observer|TestCreateSquad_Contributor|TestCreateAutopilot_Contributor|TestUpdateAgent_|TestClaim_' -count=1`

## S6 审批边界

- [x] `agent_approval.go`：申请（任何 actor）、列表（智能体只见自己）、读取、决定（人类，双重检查）、记录执行（Operator + 已批准）、取消；审计写 `activity_log`。
- [x] 路由 `/api/agent-approvals`，decision 挂 `RequireHumanActor`。
- [x] `cmd/multica/cmd_approval.go`：`request` / `list` / `get` / `executed` / `cancel`，**没有 approve**（服务端对机器凭证一律拒绝，提供该命令等于提供一个注定失败的入口）。
- [x] `multica-release-check` Skill 与 Operator 协议里写入具体命令。
- [x] 测试 `agent_approval_test.go` 14 项：pending 落库与审计、身份取自 token 而非 body、风险类/摘要校验、智能体不能决定（含自己的）、人类批准后可记录执行且三条审计齐全、二次决定 409、`decision: executed` 400、未批准/已拒绝无法记录执行、Contributor 持批准仍被拒、跨智能体读取被拒、队列可见性、非法状态过滤 400、取消后不可再决定。

验证：`cd server && go test ./internal/handler -run 'Approval|TestRecordExecution' -count=1`

**评审门：** 「只有人能批准」是整套机制的唯一支点，这一步定稿。

## S7 Squad 模板

- [x] `squad_template.go`：列表、`CreateSquadFromTemplate`、`provisionSquadTemplate`（单事务 + 双 advisory lock 固定顺序）、`resolveTemplateAgentInTx`（复用不重新应用模板）。
- [x] `parsePermissionInput` 显式传 `"private"` 兜底——它的零值是空 `permission_mode`，非 NULL，会穿过列默认值成为非法模式。
- [x] `SquadResponse` 暴露模板列。
- [x] 路由 `GET /api/squads/templates`、`POST /api/squads/from-template`。
- [x] 测试 `squad_template_test.go`：两个模板与 leader 为 Coordinator、整队一次拉起且 leader 只占一个座位、bug-fix 复用 feature-delivery 建的 Agent 且工作区只有一个 Implementer、被改写过的 Agent 不被覆盖、占名 409 且不留残留、输入校验、显式放开与默认 private。

验证：`cd server && go test ./internal/handler -run 'SquadTemplate|SquadFromTemplate' -count=1`

## S8 前端

- [x] `packages/core/types/agent-template.ts`：类型 + `isKnownAutonomyLevel` / `isApprovalActionable`（未知状态一律读作未批准）。
- [x] `api/schemas.ts` 新 schema，`Agent`/`Squad` 类型加可选模板字段，`SquadSchema` 加模板列。
- [x] `api/client.ts` 8 个方法，全部走 `parseWithFallback`。
- [x] `paths.ts` 加 `newAgentTemplate()`；`diagnostics/diagnostic-context.ts` 登记该路由（否则遥测把它归到 `*` 桶）。
- [x] `views/agents/create/template-create-agent-page.tsx` + `use-role-templates.ts` + `use-create-template-agent-submit.ts`。
- [x] `AgentConfigurationPanel` 加 `roleTemplate` 只读模式（指令与 Skills 是事实而非可编辑字段）。
- [x] 创建方式选择页加第三张卡；web 与 desktop 路由各加一条。
- [x] `views/modals/staff-squad-template.tsx` + squads 页面入口 + 模态注册。
- [x] 四语 locale：`agents.role_templates`、`modals.squad_templates`、`squads.page.template_button`。
- [x] 测试：core 17 项（谓词 + 畸形响应）、views 13 项（角色列表含未知等级仍可渲染、跳转带 `?template=`、squad 上下文透传、空/失败态；提交体只含人选字段、workspace Access 映射、导航前已写缓存、squad join 失败仍完成、无 runtime 不发请求）。

验证：`pnpm typecheck` / `pnpm lint` / `pnpm test`

## S9 文档、内置 Skill 与 E2E

- [x] `agents.mdx` × 4 语言：自治等级表 + 审批边界 + 「这是可审计边界不是沙箱」的 warning。
- [x] `agents-create.mdx` × 4 语言：第三个起点、8 个角色表、「从模板创建到底做了什么」。
- [x] `squads.mdx` × 4 语言：两个 Squad 模板、复用规则、占名冲突。
- [x] `multica-creating-agents/SKILL.md` + source map：新端点、新字段、自治执行、审批；并把该 Skill 的一致性测试里已经过时的「禁止提及模板」条目改成对真实端点的断言。
- [x] `multica-squads/SKILL.md` + source map：staffing 语义、复用规则、单事务与锁顺序。
- [x] `e2e/agent-role-template.spec.ts`：三张起点卡 → 角色列表 → 配置页只读指令 → 断言创建请求体 → 跳转到新 Agent。
- [x] **执行该 E2E**（2026-09-06，连续两次通过）。当初记的阻塞原因是误读，见 S10.4。
  命令：`pnpm exec playwright test e2e/agent-role-template.spec.ts`

## 全量验证结果（2026-09-05）

| 检查 | 结果 |
|---|---|
| `go build ./...` / `go vet ./...` / `gofmt` | 通过（`cmd_runtime_test.go` 的 gofmt 漂移为既有，未触碰） |
| `go test ./internal/... ./cmd/...`（DATABASE_URL 取自 `.env`，迁移已应用） | 50 包 ok；仅 `TestDeviceLoginWithoutSharedWorkspaceProvisionsIdentityOnly` 失败 |
| `pnpm typecheck` | 9/9 |
| `pnpm lint` | 0 error，26 warning 全在未触碰文件 |
| `pnpm test` | 416 文件 / 4982 项通过 |

那一个 Go 失败是环境残留，不是回归：它断言全库不存在 slug 为 `intranet` 的 workspace，而该行由
`09-01-intranet-deviceless-auth` 于 2026-09-01 创建；本分支未改动任何 auth/device 文件。
数据属于另一个任务，未删除。

## 回滚点

按 S 分段提交即可逐段 `git revert`。整体回滚顺序：
先 revert 应用层提交，再 `go run ./cmd/migrate down` 退掉 454 → 450（453/454 的 down 是
`DROP INDEX CONCURRENTLY`，必须单独执行）。

## 剩余工作

见 `prd.md` 的 Out of scope 与 `design.md` §9：Phase 4 的看板/评测集/灰度、per-role runtime、
模板升级差异对比、以及 S9 未执行的 E2E。

## S10 审核后补齐（2026-09-06）

来源：对已完成部分的一次代码审核。三项发现都用真实数据库探针确认过，不是推断。

### S10.1 自治门禁的两个绕过（已确认 → 已修）

- [x] `POST /api/issues/batch-update` 完全没有门禁。探针实测：observer 智能体传一个 issue id
      即把状态改成 `in_progress`，而同一动作走 `PUT /api/issues/{id}` 是 403。这条直接推翻了
      `prd.md` 里已勾选的验收标准。
- [x] `PUT /api/issues/{id}` 自己也漏了一种形状：门禁读的是解码后的指针，而写路径读的是
      `rawFields`。`{"assignee_type":null,"assignee_id":null}` 解成 nil 指针 → 门禁跳过 →
      取消分配成功（探针实测 200）。
- [x] 修法是一个共享谓词 `agent_autonomy.go` `requestTouchesIssueDirection(rawFields)`，
      按**字段是否出现**判断而不是指针是否非空，两个 handler 都调它。列表
      `issueDirectionFields` 只有一份，下一个写这些字段的端点抄不歪。
- [x] 回归测试 8 项（`agent_autonomy_test.go`）：batch 路由的 observer 拒状态 / 拒负责人 /
      拒显式 null、observer 仍可改文本、contributor 可以、未声明等级不受影响、人类不受影响，
      加上 `PUT` 路由的显式 null 一项。全部先在旧代码上失败过。

范围说明：`priority` / `parent_issue_id` 仍未挂门禁，`DeleteIssue` / `batch-delete` /
`DeleteSquad` / `DeleteAutopilot` / `TriggerAutopilot` / `RerunIssue` 也没有。PRD 的执行点表只
列了 status/assignee/create，本轮不扩大产品语义；提示词已改成不再声称服务端拦得住那些。

### S10.2 提示词与文档不再声称不存在的能力

- [x] `autonomyObserverBody` 原文写「服务端会替你拒绝这些」，涵盖了 priority / parent。改成
      点名服务端真正拒绝的三类（状态、负责人、建 issue，且写明覆盖每条路由），其余明说
      「服务端看不到，不做是你这半边的契约」。
- [x] `agents.mdx` × 4 语言：删掉「任何智能体的等级都可以在其设置中调高或调低」——设置页没有
      这个控件（`grep -rn autonomy packages/views` 只有创建前的两个 badge）。改成如实说明等级
      来自模板、只有人能改、设置页尚未提供该控件。
- [x] `multica-creating-agents/SKILL.md` + source map：写清门禁覆盖 batch 路由与显式 null，
      并新增一行钉住「提示词不得越权声称」。

### S10.3 人类审批入口（原本完全缺失）

服务端、CLI、四语文档都在，但**产品里没有任何地方能批准**：CLI 故意不做 `approve`，web/desktop
也没有队列页，`CountPendingAgentApprovalRequests` 生成了却没有任何非生成代码调用。唯一的 Operator
模板（Release Engineer）按设计提交申请后无人可批。

- [x] `packages/core/agent-approvals/`：`queries.ts`（key 带 `wsId`，focus 重取，不轮询）
      + `mutations.ts`（decide / cancel 都不做乐观更新——决定就是这个特性要产出的记录，
      而且服务端的 409 正是人必须看到的东西）。
- [x] `packages/core/types/agent-template.ts`：补 `APPROVAL_STATUSES`、`isKnownApprovalStatus`、
      `isKnownApprovalRiskClass`。只用于选文案，从不用于判断授权——测试专门钉住这条区分。
- [x] `packages/views/agents/approvals/`：队列页（待处理 / 全部两个筛选）+ 申请卡片
      （风险类与状态 badge、展开看计划、决定说明与执行结果）+ 二次确认对话框。
- [x] 批准永不是一键：确认框复述被授权的那一个动作，并写明这次批准不覆盖换参重试 / 下一步 /
      以后同一件事，附「这是授权记录不是沙箱」。撤回不带说明——没有决定可注解。
- [x] 未知状态 / 未知风险类原样渲染，按钮只看 `isApprovalPending`。
- [x] 入口：智能体页面头部加 `ApprovalQueueAction`，待处理 > 0 时带数量。用的正是
      `(workspace_id, status, created_at DESC)` 索引服务的那条查询，且与队列页共用缓存条目。
- [x] `paths.agentApprovals()` + diagnostics 路由登记 + 两个 consistency/bucket 测试的断言。
- [x] web 与 desktop 各一条路由（静态段，先于 `agents/:id` 匹配）。
- [x] 四语 locale `agents.approvals`（41 键 × 4，键集完全对齐）。
- [x] `agent-approvals-page.test.tsx` 14 项：谁申请的 / 哪一类动作、默认待处理筛选并按该状态
      请求、切到审计视图、未知状态与未知风险类仍渲染、非 pending 不给决定按钮、展开看计划、
      无计划明说、加载失败可重试、空队列有解释；以及批准需确认、确认框复述动作与"不覆盖"措辞、
      带说明发 approve、取消确认不发请求、reject 走自己的决定且空说明省略、撤回走 cancel 端点
      不走 decision、409 时把服务端原句给人看。
- [x] `agents-page.test.tsx` 的 `@tanstack/react-query` 整模块 mock 补 `queryOptions`
      （它整体替换了该模块，缺这个导出会让新加的头部查询在渲染时炸）。
- [x] `agents.mdx` × 4 语言：审批段落指向这个页面，并写明只有人能决定、智能体不能批自己的。

### S10 验证

| 检查 | 结果 |
|---|---|
| `go build ./...` / `go vet ./internal/... ./cmd/...` | 通过 |
| `gofmt -l` | 只有 `empty_claim_cache.go`、`cmd_runtime_test.go` 两个既有漂移，未触碰 |
| `go test ./internal/handler/`（单包） | 仅 `TestDeviceLoginWithoutSharedWorkspaceProvisionsIdentityOnly` 失败——09-01 留下的 `intranet` workspace 残留，与本分支无关 |
| `pnpm typecheck` | 9/9 |
| `pnpm lint` | 0 error；warning 全在未触碰文件 |
| `pnpm test` | 417 文件 / 4996 项通过 |

注意：`go test ./internal/service/ ./internal/handler/` 写在同一条命令里会让两个包并发打同一个
库，`TestCleanupSourceContextObjectIntentsBoundsAttemptsNotSuccesses` 与
`TestCommentSourceContextLifecycle` 互相干扰而假失败。分开跑或加 `-p 1` 即可。

### S10.4 E2E 已执行，并修掉它自己的一个 fixture bug

阻塞原因是误读：`/api/agent-approvals` 只存在于未提交代码里，而运行中的 API 对它返回 401
（不是 404），说明跑的就是当前工作树；`apps/web/.next` 下只有 `dev/`，Next 会按需编译。
`make status` 的 `commit` 字段是启动时的 git HEAD，不是编译进去的代码。

真跑起来之后发现 spec 自己有个从没被执行过的 bug：

- [x] mock 的 runtime 写了 `owner_id: null`。`isRuntimeUsableForUser` 对无主 runtime 一律返回
      false（public 也不例外——服务端需要一个 owner 才能签发 agent 的 task token，MUL-3292），
      于是 `draftReady` 永远是 false，页脚按钮永久 disabled。补上 owner 后整条流程通过。
- [x] 断言全部通过，包括请求体里没有 `autonomy_level`、没有 `instructions`。
- [x] 连续两次通过（50.6s / 17.8s）。

注意：冷启动的 `next dev` 会让这条 spec 贴近 60s 超时——第一次跑时 `agents/new` 与
`agents/[agentId]` 都要现场编译，`router.push` 在目标路由的 RSC 载荷到达前不会改
`window.location`，所以 `toHaveURL` 会先超时。服务器热起来后 18s 左右。CI 上如果这条不稳，
先怀疑编译时间而不是产品。

### S10 仍未做

- 自治等级的设置页控件（文档现在如实说没有）。
- 审批的实时事件：队列靠 focus 重取与 mutation 失效，不会自己动。
- `CountPendingAgentApprovalRequests` 仍是死代码（客户端从列表长度取数）。删它要跑 `make sqlc`。
