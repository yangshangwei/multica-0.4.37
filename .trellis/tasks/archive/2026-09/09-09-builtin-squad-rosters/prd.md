# 内置小队第二批：把已有的 8 个角色组装成 6 支可一键上岗的小队

延续 `09-05-builtin-dev-agents`（第一批：8 个角色模板 + 2 支小队 + 四级自治 + 审批边界）。

## Goal

让下一次升级默认带上 6 支覆盖常规研发场景的小队模板，使一个工作区不必自己想清楚"哪些角色该在一起、谁在什么时候接手、什么情况必须叫人"就能直接用。

第一批已经把 8 个干活角色、7 个角色 Skill、自治等级与审批边界都做完了，但内置小队只用到其中 5 个角色 —— `security-reviewer`、`release-engineer`、`technical-writer` 有完整提示词、有配套技能、有自治等级，却没有任何小队引用它们。这一批的价值就是把已经付过成本的能力变现。

前提约束：**不改变小队的运行模型，也不改变任何既有 Agent、Skill、Squad、Task 与自动化的行为。**

## 已确认决策

1. **一个新的干活角色都不加。** 6 支小队的名册全部由既有 8 个 listed 角色填充，`TestAgentRoleTemplates_ListedRosterIsEight` 的 8 不动，智能体选择器不变。新增的只有 6 个 unlisted 的 coordinator lead 模板。
   理由：第一批明确拒绝按技术栈拆分角色，同一条原则在这里意味着"同一批角色 + 不同路由策略 = 不同小队"。区别应该落在 `INSTRUCTIONS.md`，不在名册。
2. **不引入并行。** Review Gate 天然适合用 `issue.stage` 扇出（审查和安全审查读同一份 diff、互不写文件），但这一批先与既有两支保持一致：一轮一个成员、串行。
   理由：并行扇出会同时改变小队的执行模型和内容两件事，失败时无法区分是策略写坏了还是并行错了。把它留给单独一次改动。
3. **不新增角色 Skill。** 每角色 1 个技能是有测试兜的硬上限；6 个新 lead 复用 `multica-requirement-clarification`，与既有两个 lead 一致。
4. **6 支小队全部是内容，不碰任何契约。** 零迁移、零 API 变化、零 schema 变化、零前端变化 —— 模板列表接口与 staffing 接口对小队数量无感，前端 modal 直接 map 服务端返回。
5. **`operator` 天花板必须显式写进文档。** Release 与 Incident 两支含 `release-engineer`，`SquadTemplate.MaxAutonomy()` 会把这两支的授权天花板抬到 `operator`，导致 coordinator 级的 agent 调用者装不了它们。这是既有机制的正确行为，但不写进文档就会被当 bug 报。

## Requirements

### R1 六支小队模板

| key | 名称 | leader | 名册（全部既有角色） |
|---|---|---|---|
| `review-gate` | Review Gate Squad | `review-gate-lead` | code-reviewer, security-reviewer, qa-engineer |
| `release` | Release Squad | `release-lead` | release-engineer, qa-engineer, technical-writer |
| `incident` | Incident Response Squad | `incident-lead` | product-analyst, implementer, release-engineer, qa-engineer |
| `maintenance` | Maintenance Squad | `maintenance-lead` | implementer, qa-engineer, security-reviewer |
| `discovery` | Discovery Squad | `discovery-lead` | product-analyst, architect |
| `docs` | Docs Squad | `docs-lead` | technical-writer, code-reviewer |

- 每支小队的 `INSTRUCTIONS.md`（leader-only 的路由策略）必须写明：路由表（缺什么 → 派给谁 → 何时算完）、排序约束、父 issue 状态规则、交接格式、何时停下叫人。
- 每支小队的每个座位都要有 en/zh/ko/ja 四语的 role note —— `TestSquadTemplates_RosterIsCoherent` 逐语种校验非空。
- 小队标题与描述四语齐全。
- 六支的路由策略必须彼此可区分：同一批角色下，区别只能来自策略。Incident 与 Bug Fix 用同样的角色但优先级相反（止血优先于定性，回滚优先于修复），这一点要在两份策略里读得出来。

### R2 六个 unlisted coordinator lead 模板

- `Listed: false`、`Autonomy: AutonomyCoordinator`、`RoleSkills: ["multica-requirement-clarification"]`、`MaxConcurrentTasks: 2`，与既有两个 lead 一致。
- 每份 `INSTRUCTIONS.md` 必须含 `## Not your job`、`## Definition of done`、`## Escalate to a human when`，以及 `## Responsibilities` / `## How you route` 之一，且不含 `{{` 占位符。
- 标题与描述四语齐全，有 avatar emoji。
- 每个 lead 都必须被某支小队作为 leader 引用，否则 `TestAgentRoleTemplates_UnlistedAreSquadLeaders` 会判定它不可达。

### R3 既有行为不变

- 8 个 listed 角色、它们的提示词、自治等级、技能绑定，一个字节不动。
- `feature-delivery` 与 `bug-fix` 两支的名册、策略、版本不动。
- 已经从角色模板创建过 Agent 的工作区：staffing 新小队时按 `template_key` 复用既有 Agent，不改它的指令与自治等级（既有 `resolveTemplateAgentInTx` 行为，本任务不改，但要在验收里确认）。

### R4 文档与内置 Skill 同步

- `apps/docs/content/docs/squads.{mdx,zh,ja,ko}.mdx`：把 "Two are built in" 的两行表扩到八行，并说明 Release / Incident 的 `operator` 天花板。
- `server/internal/service/builtin_skills/multica-squads/SKILL.md`：修正 "same agent in both built-in squads" 这类假定只有两支的措辞。按仓库规则，改动内置 Skill 描述的产品行为要同 PR 更新 `SKILL.md` 与 `references/*-source-map.md`。
- 无需重生成 `server/internal/docs/`：该 in-app docs bundle 尚未进 main（是 `feat/docs-in-app` 分支的在飞工作），本分支从 main 切出，目录不存在。

### R5 测试常量同步

- `builtin_agent_autonomy_test.go` 的 `TestSquadTemplates_RosterIsCoherent`：`want 2` → 8，`wantKeys` 补齐并固定产品顺序。
- `squad_template_test.go` 的 `TestListSquadTemplates_ReturnsBothPilots`：`want 2` → 8，函数名不再说 "BothPilots"。
- 新增一条断言：六支新小队的名册只使用 listed 角色，且不引入新的角色模板 key（防止后来者用"加个新角色"绕过第一批的八角色决策）。

## Constraints

- 不新增依赖，不新增迁移，不改 API schema，不改前端。
- 小队与角色的 `INSTRUCTIONS.md` 为英文（与仓库既有 agent-harness 文本一致）；仅选择器展示用的标题/描述/role note 四语。
- 模板 key 一旦定下不得重命名 —— 它记录在 `squad.template_key` 上，是已创建小队的来源凭证。
- 产品顺序即注册表顺序（选择器按给定顺序渲染），新六支排在既有两支之后。

## Acceptance Criteria

- [ ] `GET /api/squads/templates` 返回 8 支，顺序为 feature-delivery、bug-fix 及新增六支的产品顺序。
- [ ] 六支中的每一支都能一次 staffing 出完整花名册：leader 绑定为 `leader_id`、每个座位有 `squad_member` 行、`squad.instructions` 与模板路由策略逐字相同。
- [ ] 先 staffing `feature-delivery` 再 staffing 任意新小队时，重叠角色（如 Implementer、QA Engineer）被复用而非重建，`reused_agent_ids` 如实反映。
- [ ] Release 与 Incident 的 `MaxAutonomy()` 返回 `operator`；coordinator 级 agent 调用者 staffing 这两支被拒绝，且不留下任何 Agent 或 squad 行。
- [ ] 其余四支的 `MaxAutonomy()` 不高于 `contributor`，coordinator 级调用者可以装。
- [ ] 六个新 lead 均不出现在 `GET /api/agents/templates` 的列表里（unlisted），且自治等级为 coordinator。
- [ ] 六份小队策略均含父 issue 状态规则与"不标记 done"；六份 lead 提示词均含三个必需小节且无占位符。
- [ ] 全量 `server/internal/service` 与 `server/internal/handler` Go 测试通过（含 DB-backed）。
- [ ] 四语 `squads.*.mdx` 表已扩到八支并说明 operator 天花板；`multica-squads/SKILL.md` 与 source map 已同步。
- [ ] 既有两支小队的 staffing 行为与既有 8 个角色的提示词无任何 diff。

## Out of scope

- Review Gate 的 `issue.stage` 并行扇出（决策 2）。
- 小队嵌套（`squad_member.member_type` 增加 `'squad'`）、结构化交接物、小队级并发配额、路由指标聚合、副队长、小队记忆、按 issue 属性自动选队 —— 这些是上一轮探索里识别出的结构性缺口，各自需要数据模型改动，单独立项。
- 为新小队配套的 autopilot 预设（schedule / webhook 触发模板）。
- CLI 的 `squad create --from-template` 入口。
