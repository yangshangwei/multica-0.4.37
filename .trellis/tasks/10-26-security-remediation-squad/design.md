# 技术设计：安全漏洞修复小队两阶段落地

## 1. 设计结论

2026-10-27 用户将当前续跑改为不调用真实运行时的模拟验证。新增任务内的确定性 provider double，通过现有 daemon 和开发 API 驱动真实队列、任务凭据和角色交接；执行环境只含模拟器与合成实验室，不含真实 agent CLI、模型凭据或模型出口。脚本预设的判断与补丁不作为模型能力证据。详细设计边界见 `pilot/simulation-plan-20261027.md`，不改变下述产品固化方案与其原有准入门槛。

先用普通工作区小队验证路由，再把验证过的路由固化成一个新的内置 `security-remediation` 小队模板。阶段一不改产品代码；阶段二只扩展现有模板注册表、角色提示、目录展示和测试，不引入新的小队实体、数据库表、审批模型或生产执行通道。

这个小队负责安全事项的分诊、非生产修复和证据闭环。生产发布、线上迁移、凭据访问、外部通知和不可逆操作继续交给现有发布/事故路径，由 Operator 在单项人工审批后执行。

## 2. 阶段一：工作区试点

### 2.1 试点编排

创建名为“安全修复小队（试运行）”的普通小队，复用现有角色模板。推荐花名册如下：

| 座位 | 现有角色 | 自治等级 | 什么时候派活 | 输出 |
|---|---|---:|---|---|
| Leader | `maintenance-lead` | Coordinator | 每轮判断当前缺口并只派一个下一步 | 路由理由、证据状态、是否升级 |
| 分诊 | `product-analyst` | Observer | 来源、影响资产、严重程度或验收条件不清 | 受影响范围、严重程度、验收条件 |
| 安全审查 | `security-reviewer` | Observer | 所有事项的第一道安全判断；修复后的高风险 diff 再审一次 | 适用性、威胁模型、供应链/依赖结论、发现与 `confirmed/suspected/unknown` |
| 根因诊断 | `diagnostician` | Contributor | 复现成立但原因未知，或升级/配置失败原因未知 | 最小复现、根因证据、修复方向 |
| 实现 | `implementer` | Contributor | 方案明确后，一次只修一个 CVE/依赖/代码漏洞 | 最小补丁、回归测试、验证命令 |
| 验证 | `qa-engineer` | Contributor | 实现完成后独立验证 | 原始失败用例、负面/安全测试、回归结果 |
| 兼容性 | `migration-reviewer` | Observer | 依赖升级会改变 schema、API、已安装客户端或数据 | 旧/新契约、兼容窗口、校验、恢复限制 |
| 最终正确性 | `code-reviewer` | Observer | 代码或配置修复完成后，尤其是跨层变更 | diff 正确性、失败路径、范围和复用审查 |

不把 `release-engineer` 放入试点花名册。它是 Operator，生产动作必须走现有发布/事故小队和人工审批。涉及 Agent、Skill、MCP 的安全事项按需 @`agent-evaluator`；涉及线上信号但不需要操作时按需 @`reliability-engineer`，两者都不加入基础花名册。

Leader 可以复用 `maintenance-lead` 的维护边界，但试点小队指令必须补充安全专用路由：代码漏洞进入“安全审查 → 诊断（必要时）→ 实现 → QA → 代码/安全复审”；不能因为使用了维护 leader 就把代码漏洞误归为普通依赖升级。

### 2.2 三类事项的路由

#### 组件/依赖漏洞

1. 分诊确认公告、受影响版本、实际依赖路径和部署范围。
2. 安全审查员判断当前仓库/部署是否受影响、是否已有可利用迹象、修复版本来源是否可信；只处理一个公告或一个依赖。
3. 若改动影响 schema/API/客户端，先交迁移审查员；否则跳过并记录理由。
4. 实现工程师做最小升级/补丁，保留锁文件和来源证据，补一项没有修复时会失败的回归/安全测试。
5. QA 独立验证受影响路径、边界和原始风险；安全审查员或代码审查员复核最终 diff。
6. 发现当前部署可利用、凭据/客户数据暴露或没有安全兼容版本时，停止例行路径，转事故/人工决策。

#### 程序代码漏洞

1. 安全审查员先读 diff、授权辅助函数、数据模型和既有安全规则，给出可达后果，不写 exploit。
2. 已有充分因果证据时直接交实现；原因或触发条件不清时交诊断工程师，先做最小复现。
3. 实现工程师只修改必要文件并补回归测试；不得顺手重构或扩大权限。
4. QA 从攻击者可达输入、权限拒绝、租户边界、错误路径和资源上限验证；代码审查员复核一般正确性，安全审查员复核安全结论。
5. 必须保留残余风险、未验证边界和是否需要人工策略决策。

#### 其他安全风险

- 配置、密钥暴露、供应链、构建产物或运行时边界：先由安全审查员标出信任边界和证据缺口；不读取真实 secret。
- Agent/Skill/MCP 的越权或漂移：按需交 `agent-evaluator`，要求版本化 case、脱敏 trace、越权拒绝和停止原因。
- 线上指标、恢复信号或容量缺口：按需交 `reliability-engineer`，只读脱敏信号；需要改变线上配置时转发布/事故路径。
- 当前生产已受影响的安全事项：先请求人工介入，再 @`incident`；事故小队先止血，永久修复另建后续任务。

### 2.3 统一交接字段

试点不新增数据库字段，使用 issue 描述和评论中的固定 Markdown 结构，保证人和不同角色都能读：

```text
## Security triage
- type: dependency | code | other
- source: <CVE / report / path / config>
- affected_surface: <component, endpoint, data, deployment>
- severity: <critical | high | medium | low | unknown>
- exploitability: <confirmed | suspected | unknown | not-applicable>
- evidence_state: <confirmed | suspected | unknown>
- production_impact: <yes | no | unknown>
- route: <maintenance | bug-fix | review-gate | incident | human>
- blocked_actions: <production / secret / external notice / destructive>
- next_owner: <role or person>
```

角色评论沿用各自已有输出格式，并引用这些字段；不要在评论中放凭据、客户原始数据或可复用攻击载荷。

### 2.4 运行时和权限边界

- 试点绑定专用非生产运行时、隔离工作目录和最小代码仓库范围；默认不提供生产凭据、云 CLI 或写入生产的 MCP。
- Agent Access 先保持“仅自己”或指定试点成员；需要团队分配时由人显式扩大，不能把“加入小队”当成授权。
- Leader 为 Coordinator，安全审查/分诊/复审为 Observer，实现/诊断/QA 为 Contributor；没有 Operator 成员。
- 任何生产发布、线上迁移、凭据读取/轮换、外部通知或不可逆删除都只产出计划并停下，转现有审批流程。

## 3. 阶段二：内置模板固化

### 3.1 注册表和角色

新增一个内置小队：

- `SquadTemplate.Key`: `security-remediation`
- `Version`: `1`
- 默认名称：`Security Remediation Squad` / `安全漏洞修复小队`
- 图标：盾牌类图标（沿用现有 template avatar 约定）
- Leader：新增未列出的 `security-remediation-lead`，`Coordinator`，并发上限 2
- 成员：`product-analyst`、`security-reviewer`、`diagnostician`、`implementer`、`qa-engineer`、`code-reviewer`、`migration-reviewer`
- 不新增列出的工作角色；不把 `release-engineer`、`incident` 的 Operator 能力带入模板
- 第一版不新增 role skill：复用 `multica-requirement-clarification`、`multica-security-review`、`multica-debugging`、`multica-test-report`、`multica-code-review` 和 `multica-migration-review`

Leader 的指令只负责安全事项的分流和停机边界；成员的既有指令继续负责各自的交付格式。模板版本只在路由、权限或交付契约发生实质变化时递增。

### 3.2 代码与文档边界

预期影响文件（阶段二实现时再确认实际路径）：

- `server/internal/service/builtin_agent_templates_roster.go`：注册未列出的 leader；保持每个内置 squad 一个 leader 的约束。
- `server/internal/service/builtin_agent_templates/security-remediation-lead/INSTRUCTIONS.md`：安全分流、停机和交接规则。
- `server/internal/service/builtin_squad_templates.go`：加入 roster、角色说明、版本、双语名称和描述。
- `server/internal/service/builtin_squad_templates/security-remediation/INSTRUCTIONS.md`：把阶段一验证过的路由写入 leader briefing。
- 相关 server template/autonomy/handler 测试：模板数量、语言、leader/member coherence、最大自治等级、Coordinator 可创建且不含 Operator。
- `packages/views/squads/components/builtin-squad-catalog.tsx` 及 `squads` 本地化：添加安全修复摘要；保留未知模板用后端 description 的回退。
- `apps/docs/content/docs/squads*.mdx`、`agents-create*.mdx` 及必要的生成文档：说明新模板、权限和事故交接。

不新增 DB migration，不改变 `SquadTemplateResponse`、`SquadTemplateSchema` 或 `POST /api/squads/from-template` 的形状。模板列表是数组，客户端按字符串 key 和 loose schema 处理新增项；旧客户端看不到新模板但应继续解析其他项。

### 3.3 运行和回滚

- 新模板通过现有 `GET /api/squads/templates` 和 `POST /api/squads/from-template` 暴露，创建仍是一个事务，复用已有角色副本，不覆盖工作区自定义指令。
- 发布顺序：先服务端注册与测试，再更新 UI/文档；在没有匹配后端版本前，客户端不主动发送新 `template_key`。
- 回滚优先采用前向修复。若尚未有人创建新模板，可回退注册表和 UI；若已有工作区小队，保留其普通 squad 行、成员和已复制指令，不删除数据，也不依赖模板再次存在才能运行。
- 任何生产部署仍由发布小队执行；本模板本身不改变审批或运行时沙箱。

## 4. 试点准入和固化门槛

阶段一完成后，只有满足以下条件才进入阶段二：

- 三类合成 issue 都完成了分诊、路由、修复/拒绝、独立验证和证据回读。
- 至少一个未知根因走过诊断分支，至少一个 schema/API/客户端影响走过迁移审查分支。
- 至少一个当前生产可利用/凭据或客户数据暴露情形被正确转入事故/人工路径，且没有执行生产动作。
- 试点没有发现 leader 指令与成员指令冲突；所有 `unknown`/`hold` 都有明确 owner 和下一步。
- 明确哪些条件必须转交 `review-gate`、`incident` 或 `release`，不再依赖个人记忆。

## 5. 关键取舍

### 采用两阶段而不是直接新增模板

先验证真实路由，避免把“维护、缺陷修复、门禁、事故”四套已有规则复制成互相冲突的安全模板。代价是阶段一需要人工创建小队并记录试点证据。

### 采用一个安全专用 leader，而不是复用维护 leader 作为内置模板

工作区试点可以复用 `maintenance-lead`，因为它能立即处理依赖安全公告；内置模板需要自己的 `security-remediation-lead`，以满足每个内置小队独立 leader、版本和文案的注册表约束，避免维护规则和代码漏洞规则长期混在一起。

### 不把发布/事故角色塞进修复小队

这样保持最小权限和清晰的人工审批边界。代价是严重事项需要一次交接，但这正是避免修复 agent 直接触及生产的安全控制。

### 不新增扫描器或漏洞数据库

当前目标是可靠路由和修复闭环；扫描来源可以作为 issue 输入。引入 feed/SBOM/SIEM 会带来供应链信任、凭据和运行成本，应另立任务。
