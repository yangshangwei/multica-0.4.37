# 技术设计：内置小队第二批

## 1. 这次改的是内容，不是机制

第一批已经把机制做完了。这一批要动的只有两张注册表和十二份 prose：

```
server/internal/service/
  builtin_agent_templates_roster.go        +6 unlisted coordinator lead 条目
  builtin_agent_templates/<lead>/INSTRUCTIONS.md   ×6 新建
  builtin_squad_templates.go               +6 SquadTemplate 条目
  builtin_squad_templates/<key>/INSTRUCTIONS.md    ×6 新建
```

不动的（逐一确认过）：

| 面 | 为什么不动 |
|---|---|
| 迁移 / DB | 小队模板不落库，`squad.template_key` 是 TEXT，无枚举约束 |
| API handler | `ListSquadTemplates` 遍历 `service.SquadTemplates()`，`CreateSquadFromTemplate` 按 key 查表，两者对数量无感 |
| `packages/core/api/schemas.ts` | `SquadTemplateListResponseSchema` 是 `z.array(...)`，无长度约束 |
| 前端 modal | `staff-squad-template.tsx` 直接 map 服务端返回，2 列网格 + `overflow-y-auto`，8 张卡无需改版 |
| 8 个 listed 角色 | 六支小队全部由既有角色填充 |
| 角色 Skill | 每角色 1 个技能是硬上限，新 lead 复用 `multica-requirement-clarification` |

## 2. 关键契约

### 2.1 小队之间的区别在策略，不在名册

`SquadTemplate` 的字段里，`Members` 决定名册，`Instructions()` 决定路由。这一批有三对小队名册高度重叠：

- Incident（analyst + implementer + release-engineer + qa）vs Bug Fix（analyst + implementer + qa）
- Maintenance（implementer + qa + security-reviewer）vs Review Gate（code-reviewer + security-reviewer + qa）
- Discovery（analyst + architect）⊂ Feature Delivery

这是有意的。区别写在 `INSTRUCTIONS.md` 里，而且必须是可执行的区别，不是语气差异：

| 小队 | 策略里必须体现的、与近邻不同的那一条 |
|---|---|
| Incident | 止血优先于定性；回滚优先于修复；复盘另开 issue。Bug Fix 恰好相反（先定性、不许猜、必须带回归测试） |
| Maintenance | 批次化：一次一个依赖/一个 CVE，不打包；变更面积是上限而不是产出目标 |
| Review Gate | 只读 diff，不产出改动；must-fix 回给原作者而不是绕过审查者自己改 |
| Discovery | 只产出结论，一行代码都不写；产物是可决策输入，交给 feature-delivery |
| Release | 生产动作必须走审批；回滚方案先于发布动作存在 |
| Docs | 文档跟随已合并的变更，不猜将来的变更 |

### 2.2 leader 提示词与小队策略是两层，不能写重

第一批定下的分层，这一批照抄：

- `builtin_agent_templates/<lead>/INSTRUCTIONS.md` → 复制进 `agent.instructions`。写这个角色的判断力：怎么选人、什么不是它的活、什么时候上抛。
- `builtin_squad_templates/<key>/INSTRUCTIONS.md` → 复制进 `squad.instructions`。写这个名册的路由表：缺什么派给谁、何时算完。
- `squadOperatingProtocolFor()`（`handler/squad_briefing.go`）→ 系统硬编码，claim 时注入。写机制：@mention 语法、一轮一条评论、派完就停、必须记 evaluation。**这一批不碰它。**

三层在 claim 时拼成一份 briefing。写内容时的判据：机制类的话（"用完整 mention markdown"、"记 squad activity"）已经在第三层了，前两层重复一遍只是加长提示词。

### 2.3 `operator` 天花板是既有机制，不是新增门禁

`SquadTemplate.MaxAutonomy()` 取名册里最高的自治等级，`CreateSquadFromTemplate` 用它调 `requireAgentMayGrantAutonomy`：

```go
// handler/squad_template.go:169
if maxAutonomy := template.MaxAutonomy(); maxAutonomy != "" {
    if !h.requireAgentMayGrantAutonomy(w, r, workspaceID, maxAutonomy) {
        return
    }
}
```

Release 与 Incident 含 `release-engineer`（`AutonomyOperator`），所以这两支的天花板是 `operator`。后果：

- coordinator 级的 agent 调用者可以装其余四支，装不了这两支 —— 一个 coordinator 不能凭空造出一个 operator，这正是第一批 R5 要的。
- 人类调用者不受 `requireAgentMayGrantAutonomy` 限制（它只对声明了等级的 agent actor 生效）。

**不改这个机制，只在文档里说清楚。** 没写的话，"为什么我的 lead 装不了发布小队"会被当 bug 报。

### 2.4 复用语义对新小队自动成立

`resolveTemplateAgentInTx` 按 `(workspace_id, template_key)` 查既有 Agent，命中则原样复用（含被改过的指令与被调低的自治等级）。六支新小队全部使用既有角色 key，所以一个已经装过 feature-delivery 的工作区再装 review-gate 时，Code Reviewer 和 QA Engineer 会被复用，只新建 `review-gate-lead` 和 `security-reviewer`。

这条不需要新代码，但需要在验收里实测 —— 它是"角色是共享的、小队是编排"这个产品主张的唯一证据。

## 3. 内容规格

### 3.1 六个 lead 模板

统一字段（与既有两个 lead 一致）：

```go
Listed:             false,
Autonomy:           AutonomyCoordinator,
MaxConcurrentTasks: 2,
RoleSkills:         []string{"multica-requirement-clarification"},
Version:            1,
```

| key | DefaultName | emoji |
|---|---|---|
| `review-gate-lead` | Review Gate Lead | 🚧 |
| `release-lead` | Release Lead | 📦 |
| `incident-lead` | Incident Lead | 🚨 |
| `maintenance-lead` | Maintenance Lead | 🧹 |
| `discovery-lead` | Discovery Lead | 🧭 → 与 feature-delivery-lead 的 🧭 冲突，改用 🔭 |
| `docs-lead` | Docs Lead | 📚 |

emoji 无唯一性约束，但重复会让选择器难认，逐个查过既有 10 个已用的：🔍 📐 🛠️ 🧪 🔬 🛡️ 🚦 📝 🧭 🚑。

### 3.2 六支小队条目

产品顺序（选择器按注册表顺序渲染）：既有两支在前，然后按"进来的工作有多常见"排：

```
feature-delivery, bug-fix,        // 既有
review-gate, discovery, docs,     // 轻、低权限、日常
maintenance,                      // 例行
release, incident                 // 含 operator，重
```

把两支 `operator` 的排在最后，让选择器里权限从低到高，误点的代价单调递增。

小队 emoji：`review-gate` 🚧、`discovery` 🔭、`docs` 📚、`maintenance` 🧹、`release` 📦、`incident` 🚨（与各自 lead 一致，选择器上小队卡与 leader 行看起来是一家）。

## 4. 测试策略

三层，与第一批一致：

**内容不变量（`internal/service`，无 DB，最快）** —— 这是这一批的主力：
- `TestSquadTemplates_RosterIsCoherent` 扩到 8：每个座位的模板存在、leader 是 unlisted coordinator、四语 role note 非空、策略含 `Parent issue status` 与 "Never mark it done"。
- 新增 `TestSquadTemplates_UseOnlyListedRoles`：六支新小队的座位只用 listed 角色，且不引入第一批八角色之外的 key。防止后来者靠"加个新角色"绕过八角色决策。
- 新增 `TestSquadTemplates_MaxAutonomyMatchesRoster`：逐支断言天花板，把 Release/Incident = operator 钉成契约而不是巧合。
- `TestAgentRoleTemplates_InstructionsCarryTheContract` / `_UnlistedAreSquadLeaders` / `_LocalizedCopyIsComplete` 自动覆盖 6 个新 lead，无需改 —— 它们遍历 `AllAgentRoleTemplates()`。

**HTTP 契约（`internal/handler`，需要 DB）**：
- `TestListSquadTemplates_ReturnsBothPilots` → 改名为 `_ReturnsTheWholeRoster`，`want 2` → 8。
- 新增一支新小队的 staffing 用例（选 `review-gate`：三个座位、无 operator、与 feature-delivery 有两个重叠角色，一次覆盖 staffing + 复用两件事）。
- Release/Incident 的 operator 门禁：既有 `agent_autonomy` 测试模式已存在，加一个 case 而不是新写一套。

DB-backed 测试按 [[run-db-tests-from-audit-container]] 跑：克隆 `audit_template` 到独立库，避免碰 5432 上人在用的 dev 库。注意 `TestMain` 在 `DATABASE_URL` 不可达时静默 exit 0，必须在 `-v` 输出里确认测试名真的跑了。

**E2E**：不新增。`e2e/agent-autonomy-gates.spec.ts` 只按 key 查 `feature-delivery`，不断言数量，八支不会让它失败。

## 5. 兼容性

- 已存在的小队行不受影响：`template_key` 只是来源记录，新增注册表条目不回溯改写任何行。
- 旧客户端（已安装的 desktop）拿到 8 支而不是 2 支：`SquadTemplateListResponseSchema` 是 `z.array().default([])` 且 `.loose()`，多出的条目正常解析；modal 是滚动网格，不会溢出。
- 反向：新客户端连旧后端只拿到 2 支，仍可用。

## 6. 取舍记录

1. **不做并行。** Review Gate 是天然的并行场景，但同时改执行模型和内容会让失败原因不可区分。串行版本先上，`issue.stage` 扇出单独立项。
2. **不加 autopilot 预设。** Maintenance（周度 schedule）和 Review Gate（PR webhook）都强烈指向自动化触发，但 autopilot 预设是另一套注册表和另一份权限模型，混进来会让这个任务从"内容"变成"内容 + 机制"。
3. **六支而不是三支。** 逐支写策略的边际成本远低于第一批（机制已在，角色已在），而少写的那几支会让"小队是编排层"这个主张缺证据 —— 只有两支时看起来像两个特例。
4. **Incident 与 Bug Fix 并存而不是合并。** 合并成一支带分支逻辑的策略，会让 leader 每轮先判断"这算事故还是缺陷"，判断错的代价是用错优先级。两支各自明确，选择成本交给创建时的人。

## 7. 已知风险

- **策略同质化。** 六份 prose 由同一次会话写出，最大的失败模式是读起来都一样、路由表只换了角色名。判据在 §2.1 那张表：每支必须有一条与近邻明确冲突的规则，写完逐条对照。
- **提示词膨胀。** 三层拼接后 leader 每轮读到的文本不短。lead 提示词控制在既有两个 lead 的长度量级（~2.5KB），策略控制在 ~1.5KB。
- **emoji 与已有角色重复** 会让选择器难认，§3.1 已逐个避开。
- **`operator` 天花板**：文档漏写就会被当 bug，R4 已列为验收项。
