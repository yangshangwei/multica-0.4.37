# 技术设计：内置研发智能体第一阶段

## 1. 边界与所有权

| 关注点 | 落点 | 理由 |
|---|---|---|
| 模板内容（指令原文） | `server/internal/service/builtin_agent_templates/<key>/INSTRUCTIONS.md`，`go:embed` | 提示词是可评审的散文，放 .md 才能像散文一样评审 |
| 模板元数据（版本、自治等级、默认 Skill、四语标题） | `builtin_agent_templates_roster.go` 里的 Go 结构体切片 | 元数据要编译期安全；顺序由产品决定，所以是切片不是 map |
| 角色 Skills | `builtin_role_skills/<name>/SKILL.md`，独立 embed | 与 `builtin_skills`（发给所有人）物理隔离，避免误加入常驻集合 |
| Squad 模板 | `builtin_squad_templates.go` + `<key>/INSTRUCTIONS.md` | 同上；squad 指令即 leader 的路由策略 |
| 自治策略提示词 | `builtin_agent_autonomy.go` | 与服务端执行的规则同源同仓，测试钉住两者一致 |
| 自治执行 | `internal/handler/agent_autonomy.go` + 各 handler 调用点 | 智能体走同一套 HTTP API，API 边界是唯一能真正执行的位置 |
| 审批 | `internal/handler/agent_approval.go` + `agent_approval_request` 表 | 需要状态与审计，无法用 activity_log 表达「待批准」 |
| 智能体入口 | `server/cmd/multica/cmd_approval.go` | 没有命令，被写进指令的协议就是不可执行的 |

## 2. 关键契约

### 2.1 模板 → Agent 是复制

```
INSTRUCTIONS.md (binary)  ──copy at create──▶  agent.instructions (workspace-owned)
                                               agent.template_key / template_version
```

与 Mika 完全相反的取舍，并在代码注释中写明原因：

- Mika：文本留在二进制里，每次 claim 分层拼接 → 可热更新，但工作区不能改。
- 模板：文本复制到行上 → 工作区可改，发布永不覆盖，代价是升级需要显式的差异动作（本阶段只记录版本）。

### 2.2 provenance 是服务端决定

`CreateAgentRequest` 不含 `template_key` / `autonomy_level`。做法是把 `CreateAgent` 的正文抽成
`createAgentFromRequest(w, r, req, rawFields, provenance)`，模板流用同一函数并额外传 provenance。

- 收益：runtime 归属、thinking/tier 校验、名称冲突 409、invocation permission 全部免费复用，且只有一处需要维护。
- `rawFields` 由模板流合成（`max_concurrent_tasks` 必须"存在"，否则会被全局默认值替换）。

### 2.3 角色 Skill materialize：只增不改

`materializeRoleSkillsInTx` 按名称 reuse-or-create，**代码里没有 update 路径**——这不是纪律，是结构上的不可能。

- `config.origin.type = "builtin_role_skill"`，刻意不被 `refreshableOriginSource` 识别 → 「从来源更新」不会尝试重新下载。
- 并发：两个模板可能引用同一个角色 Skill，而事务内的唯一键冲突无法恢复（整个事务被污染）。因此用工作区级 advisory lock（`lockRoleSkillMaterialization`）串行化，而不是靠重试。
- 锁顺序固定：先 (workspace, template) 再 workspace-wide role-skill，避免两个并发 staffing 死锁。

### 2.4 Squad staffing 是单事务

一支缺了 Implementer 的 squad 比失败更糟，因此不能循环调用 HTTP 创建端点（每次一个事务）。
`provisionSquadTemplate` 在一个事务里完成：advisory lock → 缺失角色 Agent → squad 行 → 全部成员行。

- 顺序上 leader 必须最先建（`SquadTemplate.TemplateKeys()` 保证），因为 squad 行需要 `leader_id`。
- 复用既有 Agent 时**不重新应用模板**：工作区可能改过指令、调低过等级、换过 runtime，创建第二支队伍不构成撤销这些的同意。
- 非模板 Agent 占名 → `agentNameConflictError` → 409，交给人决定。

### 2.5 自治等级：两处同时生效，一个来源

```
service.AutonomyBriefing(level)  ──claim time──▶  提示词（工作区删不掉）
service.AutonomyAtLeast(have,want) ──API──▶  handler 拒绝越权请求
```

兼容性规则是本设计里最重要的一条：**未识别或为空的等级对任何要求都返回 true**。

- 该列对所有历史行默认为空；若默认拒绝，升级会直接压死正在工作的工作区。
- 同理 `AutonomyBriefing("")` 返回空串 → 历史智能体 claim 到的指令逐字节不变。

执行点（刻意很窄，每个都能说出对应的验收条件）：

| 位置 | 要求等级 | 对应 PRD |
|---|---|---|
| `UpdateIssue`（status/assignee 出现时） | Contributor | Observer 不能改状态 |
| `CreateIssue` | Contributor | Observer 不能建工作项 |
| `CreateAutopilot` / `UpdateAutopilot` | Coordinator | Contributor 不能建常驻自动化 |
| `CreateSquad` | Coordinator | 路由是协调决策 |
| `RecordAgentApprovalExecution` | Operator | Contributor 不能操作生产 |

自提权路径：agent 的 task token 携带 owner 的 user id，会通过 `canManageAgent`。因此在
`UpdateAgent` 里对 `autonomy_level` 单独检查 `isMachineCredentialActor` 并拒绝；该端点的其余部分行为不变。

### 2.6 审批状态机

```
pending ──approve(human)──▶ approved ──executed(operator)──▶ executed
   │                            │
   ├──reject(human)──▶ rejected └──cancel──▶ cancelled
   └──cancel──▶ cancelled
```

状态跃迁的守卫写在 SQL 的 `WHERE` 里，不在 Go 里：

- `DecideAgentApprovalRequest ... WHERE status = 'pending'` → 第二个评审者更新 0 行 → 409，不会覆盖已有决定。
- `MarkAgentApprovalRequestExecuted ... WHERE status = 'approved'` → 重试或第二个 runtime 无法把被拒申请推进到 executed。

「只有人能批准」检查两遍（路由中间件 `RequireHumanActor` + handler 内 `isMachineCredentialActor`），因为整套机制就靠这一道门。

## 3. 数据模型

迁移 450–454，全部纯增量：

| # | 内容 | 说明 |
|---|---|---|
| 450 | `agent` 加 `template_key TEXT ''`、`template_version INT 0`、`autonomy_level TEXT ''` | 空 = 无策略声明 = 历史行 |
| 451 | `squad` 加 `template_key`、`template_version` | |
| 452 | 建 `agent_approval_request` | **无外键**（仓库规则）；`risk_class`/`status` 用 CHECK 约束固定词表 |
| 453 | `idx_agent_approval_request_workspace (workspace_id, status, created_at DESC)` | `CREATE INDEX CONCURRENTLY`，单语句单文件 |
| 454 | `idx_agent_approval_request_agent (agent_id, created_at DESC)` | 同上；智能体每轮都会查自己的申请 |

两个并发索引都登记进 `cmd/migrate/main.go` 的 `concurrentIndexCleanups`——否则一次被中断的迁移会留下
INVALID 索引并被记为成功（`TestEveryConcurrentUpBuildHasCleanup` 会挡住漏登记）。

`agent_approval_request` 没有外键，所以 workspace 拆除必须显式清理：在
`DeleteWorkspaceLeafData` 里加了 CTE，并登记进 `workspaceDeletionManifest`。

## 4. API 表面

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/api/agents/templates` | 与工作区无关（随二进制发布）；`?language=` 只影响标题/描述 |
| POST | `/api/agents/from-template` | 只接受人选择的字段；不接受 unlisted 的 leader 模板 |
| GET | `/api/squads/templates` | 含花名册与路由策略原文 |
| POST | `/api/squads/from-template` | 返回 `created_agent_ids` / `reused_agent_ids` |
| GET/POST | `/api/agent-approvals` | 智能体只看自己的；人看全工作区 |
| GET | `/api/agent-approvals/{id}` | 智能体读自己的决定结果 |
| POST | `/api/agent-approvals/{id}/decision` | 带 `RequireHumanActor` |
| POST | `/api/agent-approvals/{id}/execution` | Operator + 已批准 |
| POST | `/api/agent-approvals/{id}/cancel` | 申请者本人或可管理它的人 |

`/api/agents/templates` 与 `/from-template` 是静态段，chi 会优先于 `/{id}` 匹配（`/mika` 已依赖同一行为）。

## 5. 前端契约

- 新类型集中在 `packages/core/types/agent-template.ts`；`autonomy_level` 与 `status` 在**边界上保持 `string`**，未知值必须能渲染。
- `isKnownAutonomyLevel` 把「缺失」与「未知」一视同仁为「未声明」——UI 不得宣称一个后端并未执行的限制。
- `isApprovalActionable` 只对 `"approved"` 返回 true：任何不认识的状态都读作未批准。
- 模板列表、squad 模板、staffing 结果、审批都走 `parseWithFallback` + 畸形响应测试。`Agent` 自身仍无 schema（仓库既有缺口），`createAgentFromTemplate` 与既有 `createAgent` 保持一致。

## 6. UI 形态

- 创建页第三个起点「使用模板」；一条路由 `/{ws}/agents/new/template` 承载两步，选中的角色走 `?template=`（返回角色列表就是去掉该参数）。
- 第二步复用 `AgentConfigurationPanel`——两种流程产出同一种对象，界面不该长得像两个功能。为它加一个附加属性 `roleTemplate`：指令与 Skills 改为只读事实，并说明创建后可编辑。
- Squad 用模态框（`staff-squad-template`），创建前展示完整花名册，成功后按 created/reused 分别报数——「你本来就有一个 Implementer，它也在这支队伍里」是点按钮的人必须知道的事。

## 7. 兼容性

| 对象 | 影响 |
|---|---|
| 历史 Agent | 无。`autonomy_level` 为空 → 不受门禁约束，claim 到的指令不变 |
| 历史 Squad | 无。模板列为空，`CreateSquad` 路径未改（`instructions` 仍只由 update 写入） |
| 装机的桌面客户端 | 新字段全部 optional 且带默认值；旧客户端忽略即可 |
| 旧后端 + 新客户端 | 模板列表返回 404 → 查询失败 → 选择器显示"无法加载"，其余流程不受影响 |
| 迁移回滚 | 450/451 的 down 删列；452 drop 表；453/454 `DROP INDEX CONCURRENTLY` |

## 8. 取舍记录

1. **为什么不用 Mika 式分层？** 见 2.1：与「升级不覆盖工作区修改」直接冲突。
2. **为什么角色 Skill 不进 `BuiltinSkills()`？** 那会给全平台每次执行加 7 份文档。计划书的风险表把「提示词变长导致质量下降」列为首要风险。
3. **为什么 squad leader 是两个 unlisted 模板，而不是复用 Product Analyst？** PA 的指令写着「不改状态、不派活」，与注入的 Squad Operating Protocol 直接矛盾。给 leader 一份自己的协调者指令更诚实；不进选择器是因为没有花名册的 leader 无人可带。
4. **为什么审批只做到「记录 + 门禁」？** 服务端无法拦截 daemon 主机上的 shell。诚实的边界是可审计的那部分，文档如实写明，而不是把它包装成沙箱。
5. **为什么 Squad staffing 的 Access 默认 private？** 与从空白创建一致。staffing 不应隐式放开谁能运行某个东西；要全队可派活就显式选工作区。

## 9. 已知风险

- 审批边界不是权限隔离（见 8.4）。真正让生产凭据不可达的手段是不要把它放进开发 runtime，文档已这么写。
- 模板对所有工作区可见（Phase 4 的灰度未做）。
- Squad staffing 目前整队共用一个 runtime。
- 模板升级只保证「记录了版本、不覆盖」，差异对比与手动升级动作尚未实现。
