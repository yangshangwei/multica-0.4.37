# 执行计划：内置小队第二批

分支 `feat/builtin-squad-rosters`，从 main（`39683082a`）切出的 worktree
`.claude/worktrees/builtin-squad-rosters`。docs-bundle 不在 main，所以本任务不触碰
`server/internal/docs/`。

顺序原则：**先写测试期望（S1），再填内容（S2/S3），再接注册表（S4）。** 内容测试是这一批
唯一的自动化护栏，先让它红起来，才能证明后面的内容真的被检查了。

---

## S1 先改测试，让它红

`server/internal/service/builtin_agent_autonomy_test.go`

- `TestSquadTemplates_RosterIsCoherent`：`want 2` → 8，`wantKeys` 改为
  `feature-delivery, bug-fix, review-gate, discovery, docs, maintenance, release, incident`
  （产品顺序，见 design §3.2）。
- 新增 `TestSquadTemplates_UseOnlyListedRoles`：遍历每支的 `Members`，断言每个
  `TemplateKey` 都在第一批八角色集合内。这条防的是后来者用"加个新角色"绕过八角色决策。
- 新增 `TestSquadTemplates_MaxAutonomyMatchesRoster`：逐支断言天花板
  —— `release` / `incident` = `operator`，其余六支 ≤ `contributor`。

`server/internal/handler/squad_template_test.go`

- `TestListSquadTemplates_ReturnsBothPilots` → `TestListSquadTemplates_ReturnsTheWholeRoster`，
  `want 2` → 8。

跑一次确认全红（`go test ./internal/service/ -run 'TestSquadTemplate'`），S1 才算完。

## S2 六个 lead 提示词

新建 `server/internal/service/builtin_agent_templates/<lead>/INSTRUCTIONS.md` ×6。

硬性要求（`TestAgentRoleTemplates_InstructionsCarryTheContract`）：含
`## Not your job`、`## Definition of done`、`## Escalate to a human when`，以及
`## Responsibilities` / `## How you route` 之一，不含 `{{`。

写作判据：
- 长度对齐既有两个 lead（~2.5KB）。
- 不重复 `squadOperatingProtocolFor()` 已有的机制类内容（mention 语法、一轮一条评论、
  记 evaluation）—— 那是第三层。
- 每份必须有一条与近邻小队冲突的规则（design §2.1 表），写完逐条对照。

对照参考：`feature-delivery-lead/INSTRUCTIONS.md` 与 `bug-fix-lead/INSTRUCTIONS.md`。

## S3 六份小队路由策略

新建 `server/internal/service/builtin_squad_templates/<key>/INSTRUCTIONS.md` ×6。

硬性要求：含 `Parent issue status` 小节与 "Never mark it done"（大小写任一）。

结构对齐既有两份：路由表（缺什么 / 派给谁 / 何时算完）→ 排序约束 → 父 issue 状态
→ 交接格式 → 何时停下叫人。长度 ~1.5KB。

`release` 与 `incident` 两份必须写明生产动作走人工审批 —— 名册里有 `release-engineer`
（operator），策略不说清楚就等于把审批边界交给模型自己发挥。

## S4 两张注册表

`builtin_agent_templates_roster.go`：在既有两个 lead 之后追加 6 个条目。统一字段
`Listed: false` / `AutonomyCoordinator` / `MaxConcurrentTasks: 2` /
`RoleSkills: ["multica-requirement-clarification"]` / `Version: 1`，各含四语
Titles + Descriptions + emoji（design §3.1，已避开既有 10 个）。

`builtin_squad_templates.go`：追加 6 个 `SquadTemplate`，顺序即 design §3.2 的产品顺序。
每个座位的 `Roles` 四语齐全 —— `RosterIsCoherent` 逐语种校验非空，漏一个语种就红。

S4 结束时 S1 的测试应全绿。

## S5 handler 层测试补齐

- `review-gate` 的 staffing 用例：三个座位、无 operator、与 feature-delivery 有
  code-reviewer + qa-engineer 两个重叠角色 —— 一个用例同时覆盖 staffing 与复用。
- operator 门禁：在既有 autonomy 测试模式里加 case，断言 coordinator 级 agent 调用者
  staffing `release` 被拒且不留任何 Agent / squad 行。

DB-backed 测试按 [[run-db-tests-from-audit-container]] 跑：克隆 `audit_template` 到
独立库，不碰 5432 上人在用的 dev 库。`TestMain` 在 `DATABASE_URL` 不可达时静默 exit 0，
必须在 `-v` 输出里确认测试名真的执行了（[[db-backed-go-tests-skip-silently]]）。

## S6 文档与内置 Skill

- `apps/docs/content/docs/squads.{mdx,zh,ja,ko}.mdx`：两行表 → 八行，补一段说明
  Release / Incident 的 `operator` 天花板（谁装不了、为什么）。四语措辞按
  `apps/docs/content/docs/developers/conventions.zh.mdx` 的中文产品口径。
- `builtin_skills/multica-squads/SKILL.md`：修正 "same agent in both built-in squads"
  这类假定只有两支的措辞。
- `builtin_skills/multica-squads/references/squad-source-map.md`：如有数量或行为陈述，同步。

## 验证

```bash
(cd server && go vet ./internal/service/ ./internal/handler/)
(cd server && gofmt -l internal/service internal/handler)
(cd server && go test ./internal/service/ -count=1 -v)
# DB-backed，克隆库后：
(cd server && DATABASE_URL='...' go test ./internal/handler/ -run 'Squad' -count=1 -v)
```

前端与 TS 侧无改动，但 `squads.*.mdx` 属于 `apps/docs`，跑一次 `pnpm typecheck`
确认 mdx 未破坏文档站构建（worktree 无 node_modules，需先按需安装或跳过并说明）。

## 回滚点

每个 S 独立可回滚：S1 只改测试期望，S2/S3 只新增文件，S4 是纯追加，S6 是文档。
任一步失败不影响既有两支小队 —— 它们的注册表条目与 prose 全程不动。

---

## 执行结果（2026-09-09）

全部 S 已完成。改动 14 个既有文件 + 新建 12 份 prose，654 行新增。

### 实际交付

| 面 | 结果 |
|---|---|
| lead prompts | 6 份新建，`builtin_agent_templates/` 从 10 个目录到 16 个 |
| squad policies | 6 份新建，`builtin_squad_templates/` 从 2 个目录到 8 个 |
| 角色注册表 | +6 unlisted coordinator（review-gate/discovery/docs/maintenance/release/incident-lead），8 个 listed 角色一字节未动 |
| 小队注册表 | +6 条目，产品顺序 feature-delivery, bug-fix, review-gate, discovery, docs, maintenance, release, incident |
| 文档 | 四语 `squads.*.mdx` 表 2 行 → 8 行，各补 operator 天花板说明 |
| 内置 Skill | `multica-squads/SKILL.md` 列出八个 key 并修正"两支"措辞；source map 点名 release/incident |

### 计划外的三处修正

1. **两处源码注释假定只有两支小队**，改动后读起来是错的：
   - `handler/agent_template.go` 的 `lockRoleSkillMaterialization`（"staffing both built-in squads at once"）
   - `pkg/db/queries/agent.sql` 的 `GetAgentByWorkspaceAndTemplateKey`（"a team that staffs both built-in squads"）
   后者需要 `make sqlc` 重生成（注释会复制进 generated 文件）。重生成 diff 只含该注释，无 schema 漂移。
2. **一处文档锚点错误**：初稿 en 版指向 `#permissions-and-access`（应为自治等级小节），zh 版指向 `#自治等级`（不是真实标题，实际是 `## 自治等级与审批边界`）。四语锚点已逐个对照真实标题校验。
3. **一条我自己写错的断言**：`squad_template_test.go` 中断言 review-gate 的 `MaxAutonomy() == contributor`，实测为 `coordinator` —— `MaxAutonomy()` 遍历 `TemplateKeys()`，**包含 leader 座位**，而所有 leader 都是 coordinator。所以非 operator 小队的天花板永远是 coordinator，不可能是 contributor。该断言已删除（服务层 `TestSquadTemplates_MaxAutonomyMatchesRoster` 已是这条规则的唯一权威层，按仓库 one-canonical-layer 规则不在 handler 重跑）。

### 新增测试

服务层（无 DB）：
- `TestSquadTemplates_StaffOnlyTheEightWorkingRoles` —— 座位只能用 listed 角色，堵住"加个新角色填座位"绕过八角色决策
- `TestSquadTemplates_LeadersAreOneToOne` —— 每支小队独占一个 lead，且每个 unlisted lead 都被引用（不可达即失败）
- `TestSquadTemplates_MaxAutonomyMatchesRoster` —— 逐支钉死授权天花板，release/incident = operator，其余六支 = coordinator

handler 层（需 DB）：
- `TestCreateSquadFromTemplate_StaffsASecondBatchSquad` —— review-gate 一次覆盖新建座位（lead + security-reviewer）与跨小队复用（code-reviewer、qa-engineer 来自 feature-delivery）
- `TestCreateSquadFromTemplate_CoordinatorCannotStaffAnOperatorRoster`（release/incident 两个子用例）+ `_CoordinatorMayStaffAContributorRoster` —— operator 天花板的实际门禁行为，此前只有 e2e 覆盖

改名：`TestListSquadTemplates_ReturnsBothPilots` → `_ReturnsTheWholeRoster`。

### 验证结果

```
go vet ./...                                    exit 0
gofmt -l（我触及的 14 个 go 文件）                 全部已格式化
go test ./internal/service/ -count=1 -v          14 PASS
go test ./internal/handler/ -run Squad -v        全部 PASS（含 3 个新用例）
go test ./... -count=1（全量，DB-backed）          exit=0，64 ok + 9 no-test-files = 73 包，0 FAIL
```

DB 走 `multica-postgres-1` 容器里 `multica_audit_test`（已迁移到 454，0 workspace）克隆出的
`squad_rosters_check`，跑完已 DROP。未碰 5432 上人在用的 `multica_multica_0_4_37_492`。
`-v` 输出里确认了测试名真的执行（[[db-backed-go-tests-skip-silently]]）。

**注意**：`.trellis` 记录的 audit 容器 `multica-main-audit-yih5y70`（127.0.0.1:53656）已不存在，
本次改用 dev 容器内的 `multica_audit_test` 作模板库，凭据 `multica:multica@127.0.0.1:5432`。

一次全量运行中 `internal/daemon/execenv` FAIL（`TestResolveWindowsSandboxStateFailsClosed`
一类 Windows sandbox 用例）。判定为既有 flake，非本次改动引入：该包不 import
`internal/service` 或 `internal/handler`（已用 `go list -deps` 确认），单独跑三次均 PASS
（0.35s vs 全量并发下 65s），最终全量运行 exit=0 且无 FAIL。

### 未执行的验证

`pnpm typecheck`（含 apps/docs 文档站构建）—— worktree 无 node_modules。改用静态核查替代：
四语 diff 共 46 行新增，全为 markdown 表格与散文，无 JSX、无 import、无 `{` / `<`
（MDX 表达式语法字符），反引号成对闭合。不足以等同一次真实构建，合并前建议在有依赖的
环境跑一次 `pnpm --filter @multica/docs build`。

### 结论对照验收标准

prd.md 的 10 条验收标准中 9 条已由上述测试直接覆盖并通过。第 9 条（四语文档 + SKILL.md
同步）已完成但只经静态核查，未经文档站构建。
